-- requires: foundation, types, magic_item, effect

create table limitation (
  item_id     uuid not null references magic_item (id) on delete cascade,
  effect_id   uuid,
  description text not null,
  primary key (id),
  constraint limitation_description_present check (description ~ '[^[:space:]]'),
  constraint limitation_effect_same_item_fk
    foreign key (effect_id, item_id)
    references effect (id, item_id)
    on delete cascade
) inherits (object);

comment on table limitation is 'A usage limitation applying to an item or one of its effects.';
comment on column limitation.item_id is 'The MagicItem constrained by this limitation.';
comment on column limitation.effect_id is 'Optional constrained Effect; null means the limitation applies to the whole item.';
comment on column limitation.description is 'Source-grounded description of the usage limitation.';

create trigger touch before update on limitation for each row execute function touch();
create index limitation_item_idx on limitation (item_id);
create index limitation_effect_idx on limitation (effect_id);
