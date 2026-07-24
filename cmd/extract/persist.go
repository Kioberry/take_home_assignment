package main

import (
	"context"
	"fmt"

	"oddities/database/generated"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureEmptyCatalog rejects a run when the catalog already contains records.
// The current ontology has no durable source identity for cross-run deduplication.
func EnsureEmptyCatalog(ctx context.Context, queries *generated.Queries) error {
	count, err := queries.CountMagicItems(ctx)
	if err != nil {
		return fmt.Errorf("count magic items: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("catalog is non-empty: %d magic items already exist", count)
	}
	return nil
}

// PersistCandidate stores one normalized candidate and all of its child records
// atomically. A failed child insert or effect index mapping rolls back the item.
func PersistCandidate(ctx context.Context, pool *pgxpool.Pool, candidate NormalizedCandidate) (pgtype.UUID, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("persist item %q: begin transaction: %w", candidate.Raw.Name, err)
	}
	defer tx.Rollback(ctx)

	queries := generated.New(tx)
	itemID, err := queries.InsertMagicItem(ctx, generated.InsertMagicItemParams{
		Name:                  candidate.Raw.Name,
		SourceItemType:        candidate.SourceItemType,
		SourceItemSubtype:     candidate.Raw.SourceItemSubtypeRaw,
		Rarity:                candidate.Rarity,
		UsageMode:             candidate.UsageMode,
		WearSlot:              candidate.WearSlot,
		RequiresAttunement:    candidate.Raw.RequiresAttunement,
		AttunementRequirement: candidate.Raw.AttunementRequirement,
		RawDescription:        candidate.Raw.RawDescription,
		NeedsReview:           candidate.NeedsReview,
	})
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("persist item %q: insert magic item: %w", candidate.Raw.Name, err)
	}

	effectIDs := make([]pgtype.UUID, len(candidate.Effects))
	for index, effect := range candidate.Effects {
		effectID, err := queries.InsertEffect(ctx, generated.InsertEffectParams{
			ItemID:      itemID,
			Category:    effect.Category,
			Description: effect.Description,
		})
		if err != nil {
			return pgtype.UUID{}, fmt.Errorf("persist item %q: insert effect %d: %w", candidate.Raw.Name, index, err)
		}
		effectIDs[index] = effectID
	}

	for index, limitation := range candidate.Raw.Limitations {
		effectID := pgtype.UUID{}
		if limitation.EffectIndex != nil {
			effectIndex := *limitation.EffectIndex
			if effectIndex < 0 || effectIndex >= len(effectIDs) {
				return pgtype.UUID{}, fmt.Errorf("persist item %q: limitation %d references effect index %d outside 0..%d", candidate.Raw.Name, index, effectIndex, len(effectIDs)-1)
			}
			effectID = effectIDs[effectIndex]
		}

		if _, err := queries.InsertLimitation(ctx, generated.InsertLimitationParams{
			ItemID:      itemID,
			EffectID:    effectID,
			Description: limitation.Description,
		}); err != nil {
			return pgtype.UUID{}, fmt.Errorf("persist item %q: insert limitation %d: %w", candidate.Raw.Name, index, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return pgtype.UUID{}, fmt.Errorf("persist item %q: commit transaction: %w", candidate.Raw.Name, err)
	}
	return itemID, nil
}
