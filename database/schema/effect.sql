-- requires: foundation, types, magic_item

create table effect (
  item_id     uuid not null references magic_item (id) on delete cascade,
  category    effect_category not null,
  description text not null,
  primary key (id),
  unique (id, item_id),
  constraint effect_description_present check (description ~ '[^[:space:]]')
) inherits (object);

comment on table effect is 'An independently understandable magical ability provided by one MagicItem.';
comment on column effect.item_id is 'The MagicItem that provides this effect.';
comment on column effect.category is 'Exactly one broad primary category for the effect.';
comment on column effect.description is 'Source-grounded description of this individual effect.';

create trigger touch before update on effect for each row execute function touch();
create index effect_item_idx on effect (item_id);
