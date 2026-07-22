package database

import (
	"context"
	"testing"

	"oddities/db"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func provisionOntology(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	if _, err := pool.Exec(ctx, "drop schema public cascade; create schema public"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if err := db.Apply(ctx, pool, Schema()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return pool, ctx
}

func TestOntologyVocabulary(t *testing.T) {
	pool, ctx := provisionOntology(t)

	var count int
	if err := pool.QueryRow(ctx, `
		select count(*)
		from pg_type
		where typname in ('rarity', 'source_item_type', 'usage_mode', 'wear_slot', 'effect_category')
	`).Scan(&count); err != nil {
		t.Fatalf("query vocabulary: %v", err)
	}
	if count != 5 {
		t.Fatalf("want five ontology enum types, got %d", count)
	}
}

func TestMagicItemConstraints(t *testing.T) {
	pool, ctx := provisionOntology(t)
	valid := `insert into magic_item
		(name, source_item_type, rarity, usage_mode, wear_slot, raw_description)
		values ('Cloak', 'wondrous_item', 'rare', 'worn', 'outerwear', 'A useful cloak')`
	if _, err := pool.Exec(ctx, valid); err != nil {
		t.Fatalf("valid worn item: %v", err)
	}
	for name, query := range map[string]string{
		"blank name":                 `insert into magic_item (name, source_item_type, rarity, usage_mode, raw_description) values ('  ', 'weapon', 'common', 'held', 'A weapon')`,
		"blank description":          `insert into magic_item (name, source_item_type, rarity, usage_mode, raw_description) values ('Sword', 'weapon', 'common', 'held', E'\n\t')`,
		"worn without slot":          `insert into magic_item (name, source_item_type, rarity, usage_mode, raw_description) values ('Hat', 'wondrous_item', 'common', 'worn', 'A hat')`,
		"held with slot":             `insert into magic_item (name, source_item_type, rarity, usage_mode, wear_slot, raw_description) values ('Sword', 'weapon', 'common', 'held', 'hands', 'A sword')`,
		"unattuned with requirement": `insert into magic_item (name, source_item_type, rarity, usage_mode, attunement_requirement, raw_description) values ('Ring', 'ring', 'rare', 'worn', 'an elf', 'A ring')`,
	} {
		if _, err := pool.Exec(ctx, query); err == nil {
			t.Errorf("%s: want constraint error", name)
		}
	}
}

func TestLimitationIntegrity(t *testing.T) {
	pool, ctx := provisionOntology(t)
	itemA := insertTestItem(t, pool, ctx, "Item A")
	itemB := insertTestItem(t, pool, ctx, "Item B")
	var effectA, effectB pgtype.UUID
	if err := pool.QueryRow(ctx, `insert into effect (item_id, category, description) values ($1, 'offensive', 'A effect') returning id`, itemA).Scan(&effectA); err != nil {
		t.Fatalf("effect A: %v", err)
	}
	if err := pool.QueryRow(ctx, `insert into effect (item_id, category, description) values ($1, 'utility', 'B effect') returning id`, itemB).Scan(&effectB); err != nil {
		t.Fatalf("effect B: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into limitation (item_id, description) values ($1, 'whole item limit')`, itemA); err != nil {
		t.Fatalf("item-wide limitation: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into limitation (item_id, effect_id, description) values ($1, $2, 'same item limit')`, itemA, effectA); err != nil {
		t.Fatalf("same-item limitation: %v", err)
	}
	if _, err := pool.Exec(ctx, `insert into limitation (item_id, effect_id, description) values ($1, $2, 'cross item limit')`, itemA, effectB); err == nil {
		t.Fatal("cross-item effect reference was accepted")
	}
	if _, err := pool.Exec(ctx, `insert into limitation (item_id, description) values ($1, 'blank')`, itemB); err != nil {
		t.Fatalf("second item limitation: %v", err)
	}
	if _, err := pool.Exec(ctx, `delete from magic_item where id = $1`, itemA); err != nil {
		t.Fatalf("delete item A: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `select count(*) from effect where item_id = $1`, itemA).Scan(&remaining); err != nil {
		t.Fatalf("count effects after cascade: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("want cascaded effects removed, got %d", remaining)
	}
}

func insertTestItem(t *testing.T, pool *pgxpool.Pool, ctx context.Context, name string) pgtype.UUID {
	t.Helper()
	var id pgtype.UUID
	err := pool.QueryRow(ctx, `
		insert into magic_item (name, source_item_type, rarity, usage_mode, raw_description)
		values ($1, 'wondrous_item', 'common', 'portable', 'A test item') returning id`, name).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s: %v", name, err)
	}
	return id
}
