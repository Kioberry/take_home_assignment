-- Add your named queries here, e.g.:
--
--   -- name: InsertItem :one
--   insert into item (name, rarity) values ($1, $2) returning *;
--
-- Then run `sqlc generate` (from this directory) to produce typed Go in
-- ../generated. See ../../stormland/query for a worked example.

-- name: InsertMagicItem :one
insert into magic_item (
  name, source_item_type, source_item_subtype, rarity, usage_mode, wear_slot,
  requires_attunement, attunement_requirement, raw_description, needs_review
) values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
returning id;

-- name: InsertEffect :one
insert into effect (item_id, category, description)
values ($1, $2, $3)
returning id;

-- name: EffectsForMagicItem :many
select id, category, description
from effect
where item_id = $1
order by id;

-- name: InsertLimitation :one
insert into limitation (item_id, effect_id, description)
values ($1, $2, $3)
returning id;

-- name: LimitationsForMagicItem :many
select id, effect_id, description
from limitation
where item_id = $1
order by id;
