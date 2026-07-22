-- requires: foundation, types

create table magic_item (
  name                   text not null,
  source_item_type       source_item_type not null,
  source_item_subtype    text,
  rarity                 rarity not null,
  usage_mode             usage_mode not null,
  wear_slot              wear_slot,
  requires_attunement    boolean not null default false,
  attunement_requirement text,
  raw_description        text not null,
  needs_review            boolean not null default false,
  primary key (id),
  constraint magic_item_name_present check (name ~ '[^[:space:]]'),
  constraint magic_item_subtype_present check (source_item_subtype is null or source_item_subtype ~ '[^[:space:]]'),
  constraint magic_item_description_present check (raw_description ~ '[^[:space:]]'),
  constraint magic_item_attunement_present check (attunement_requirement is null or attunement_requirement ~ '[^[:space:]]'),
  constraint magic_item_wear_slot_consistency check (
    (usage_mode = 'worn' and wear_slot is not null)
    or (usage_mode <> 'worn' and wear_slot is null)
  ),
  constraint magic_item_attunement_consistency check (
    requires_attunement or attunement_requirement is null
  )
) inherits (object);

comment on table magic_item is 'A cataloged magic item sold by Reznar.';
comment on column magic_item.name is 'Catalog display name; duplicate names remain possible.';
comment on column magic_item.source_item_type is 'Canonical top-level type stated by the source.';
comment on column magic_item.source_item_subtype is 'Optional normalized subtype text from the source, such as dagger or plate.';
comment on column magic_item.rarity is 'Canonical rarity of the item.';
comment on column magic_item.usage_mode is 'Whether the item is worn, held, portable, or consumed.';
comment on column magic_item.wear_slot is 'Equipment slot occupied by a worn item; null for other usage modes.';
comment on column magic_item.requires_attunement is 'Whether a user must attune to use the item.';
comment on column magic_item.attunement_requirement is 'Optional source-derived restriction on who may attune.';
comment on column magic_item.raw_description is 'Nonblank source description retained without semantic rewriting.';
comment on column magic_item.needs_review is 'Whether the structured interpretation needs human review.';

create trigger touch before update on magic_item for each row execute function touch();
