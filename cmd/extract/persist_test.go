package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"oddities/database"
	"oddities/database/generated"
	"oddities/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	persistenceSchemaSequence atomic.Uint64
	persistenceSchemaPattern  = regexp.MustCompile(`^extract_persistence_[0-9]+_[0-9]+$`)
)

func TestPersistCandidateInsertsEffectsAndBothLimitationScopes(t *testing.T) {
	t.Parallel()
	pool, ctx := provisionPersistence(t)
	candidate := persistenceCandidate(t)

	itemID, err := PersistCandidate(ctx, pool, candidate)
	if err != nil {
		t.Fatalf("persist candidate: %v", err)
	}
	if !itemID.Valid {
		t.Fatal("persisted item ID is invalid")
	}

	var name string
	if err := pool.QueryRow(ctx, `select name from magic_item where id = $1`, itemID).Scan(&name); err != nil {
		t.Fatalf("read persisted item: %v", err)
	}
	if name != candidate.Raw.Name {
		t.Fatalf("item name = %q, want %q", name, candidate.Raw.Name)
	}

	var effects int
	if err := pool.QueryRow(ctx, `select count(*) from effect where item_id = $1`, itemID).Scan(&effects); err != nil {
		t.Fatalf("count effects: %v", err)
	}
	if effects != len(candidate.Effects) {
		t.Fatalf("effect count = %d, want %d", effects, len(candidate.Effects))
	}

	var itemWideEffectID pgtype.UUID
	if err := pool.QueryRow(ctx, `select effect_id from limitation where item_id = $1 and description = $2`, itemID, "Cannot be used in daylight.").Scan(&itemWideEffectID); err != nil {
		t.Fatalf("read item-wide limitation: %v", err)
	}
	if itemWideEffectID.Valid {
		t.Fatalf("item-wide limitation effect ID = %v, want NULL", itemWideEffectID)
	}

	var mappedToIndexedEffect bool
	if err := pool.QueryRow(ctx, `
		select l.effect_id = e.id
		from limitation l
		join effect e on e.item_id = l.item_id and e.description = $2
		where l.item_id = $1 and l.description = $3
	`, itemID, candidate.Effects[1].Description, "Emits a burst only once each dawn.").Scan(&mappedToIndexedEffect); err != nil {
		t.Fatalf("read effect-linked limitation: %v", err)
	}
	if !mappedToIndexedEffect {
		t.Fatal("effect-linked limitation was not mapped to its indexed effect ID")
	}
}

func TestEnsureEmptyCatalogRejectsNonEmptyCatalog(t *testing.T) {
	t.Parallel()
	pool, ctx := provisionPersistence(t)
	queries := generated.New(pool)
	if err := EnsureEmptyCatalog(ctx, queries); err != nil {
		t.Fatalf("empty catalog rejected: %v", err)
	}

	if _, err := PersistCandidate(ctx, pool, persistenceCandidate(t)); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	if err := EnsureEmptyCatalog(ctx, queries); err == nil {
		t.Fatal("non-empty catalog accepted")
	} else if !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("guard error = %q, want non-empty detail", err)
	}
}

func TestPersistCandidateRollsBackWhenLimitationIndexIsInvalid(t *testing.T) {
	t.Parallel()
	pool, ctx := provisionPersistence(t)
	candidate := persistenceCandidate(t)
	invalidIndex := len(candidate.Effects)
	candidate.Raw.Limitations = append(candidate.Raw.Limitations, RawLimitation{
		EffectIndex: &invalidIndex,
		Description: "References a missing effect.",
	})

	if _, err := PersistCandidate(ctx, pool, candidate); err == nil {
		t.Fatal("persisted candidate with invalid limitation effect index")
	} else if !strings.Contains(err.Error(), candidate.Raw.Name) {
		t.Fatalf("persist error = %q, want item name", err)
	}

	for _, table := range []string{"magic_item", "effect", "limitation"} {
		var count int
		if err := pool.QueryRow(ctx, "select count(*) from "+table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s count after rollback = %d, want 0", table, count)
		}
	}
}

func TestPersistenceSchemaIdentifierIsUniqueAndSafe(t *testing.T) {
	t.Parallel()
	first := newPersistenceSchemaName()
	second := newPersistenceSchemaName()
	if first == second {
		t.Fatalf("schema names match: %q", first)
	}

	quoted, err := persistenceSchemaIdentifier(first)
	if err != nil {
		t.Fatalf("quote generated schema: %v", err)
	}
	if want := (pgx.Identifier{first}).Sanitize(); quoted != want {
		t.Fatalf("quoted schema = %q, want %q", quoted, want)
	}

	if _, err := persistenceSchemaIdentifier(`test"; drop schema public cascade; --`); err == nil {
		t.Fatal("unsafe schema identifier was accepted")
	}
	if _, err := persistenceSchemaIdentifier("public"); err == nil {
		t.Fatal("public schema identifier was accepted")
	}
}

func provisionPersistence(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(db.URL())
	if err != nil {
		t.Fatalf("parse database URL: %v", err)
	}
	admin, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}
	t.Cleanup(admin.Close)

	schema := newPersistenceSchemaName()
	quotedSchema, err := persistenceSchemaIdentifier(schema)
	if err != nil {
		t.Fatalf("validate schema name: %v", err)
	}
	if _, err := admin.Exec(ctx, "create schema "+quotedSchema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}

	testConfig := config.Copy()
	testConfig.ConnConfig.RuntimeParams["search_path"] = quotedSchema
	pool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		if _, dropErr := admin.Exec(ctx, "drop schema "+quotedSchema+" cascade"); dropErr != nil {
			t.Errorf("drop test schema after pool setup failure: %v", dropErr)
		}
		t.Fatalf("connect isolated test pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "drop schema "+quotedSchema+" cascade"); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	if err := db.Apply(ctx, pool, database.Schema()); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	return pool, ctx
}

func newPersistenceSchemaName() string {
	return fmt.Sprintf("extract_persistence_%d_%d", os.Getpid(), persistenceSchemaSequence.Add(1))
}

func persistenceSchemaIdentifier(schema string) (string, error) {
	if !persistenceSchemaPattern.MatchString(schema) {
		return "", fmt.Errorf("unsafe persistence schema name %q", schema)
	}
	return (pgx.Identifier{schema}).Sanitize(), nil
}

func persistenceCandidate(t *testing.T) NormalizedCandidate {
	t.Helper()
	candidate := validNormalizedCandidate(t)
	candidate.Raw.Name = "Dawnfire Compass"
	candidate.Raw.Effects = append(candidate.Raw.Effects, RawEffect{
		CategoryRaw: "offensive",
		Description: "Emits a burst of dawnfire.",
	})
	candidate.Effects = append(candidate.Effects, NormalizedEffect{
		Category:    generated.EffectCategoryOffensive,
		Description: "Emits a burst of dawnfire.",
	})
	linkedEffectIndex := 1
	candidate.Raw.Limitations = []RawLimitation{
		{Description: "Cannot be used in daylight."},
		{EffectIndex: &linkedEffectIndex, Description: "Emits a burst only once each dawn."},
	}
	return candidate
}
