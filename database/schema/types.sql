-- requires: foundation

create type rarity as enum (
  'common', 'uncommon', 'rare', 'very_rare', 'legendary', 'artifact', 'varies'
);
comment on type rarity is 'Canonical rarity of a magic item.';

create type source_item_type as enum (
  'wondrous_item', 'weapon', 'armor', 'potion', 'ring'
);
comment on type source_item_type is 'Canonical top-level item type stated by the source catalog.';

create type usage_mode as enum ('worn', 'held', 'portable', 'consumed');
comment on type usage_mode is 'How a user activates or carries a magic item.';

create type wear_slot as enum (
  'head', 'neck', 'torso', 'outerwear', 'hands', 'feet', 'finger'
);
comment on type wear_slot is 'Equipment slot occupied by a worn magic item.';

create type effect_category as enum ('offensive', 'defensive', 'utility');
comment on type effect_category is 'Broad primary analytical category for an independent effect.';
