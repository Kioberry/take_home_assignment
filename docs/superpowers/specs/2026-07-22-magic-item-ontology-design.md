# Magic Item Ontology Design

Date: 2026-07-22

## 1. Goal and scope

The catalog ontology should make magic items easy to add, traverse, and compare. The initial design focuses on the two dimensions Reznar identifies in the README:

1. How an item is used and, for worn items, which equipment slot it occupies.
2. What kinds of magical effects and usage limitations the item has.

This document covers only the ontology. AI extraction, confidence scoring, retries, human review, source-page tracking, and other pipeline behavior will be designed separately.

The first pass deliberately stays coarse. When the PDF provides repeated evidence that the current model cannot express an important distinction, the proposed extension must be reviewed before the ontology changes.

## 2. Confirmed ontology

### 2.1 Object model

The ontology has three object types. Each object inherits the provided `object` base table and therefore receives an `id`, `created_at`, and `updated_at`.

```text
MagicItem 1 --- N Effect
MagicItem 1 --- N Limitation
Effect    1 --- N Limitation (optional from the Limitation side)
```

An item may have multiple effects and multiple limitations. Each effect has exactly one broad category. A multi-purpose item is represented by multiple effect records rather than one effect with multiple categories.

### 2.2 MagicItem

| Field | Meaning |
|---|---|
| `name` | The item's catalog display name. It must not be blank and is not unique. Name normalization, source identity, and extraction idempotency are pipeline concerns. |
| `source_item_type` | The canonical top-level type stated by the source. |
| `source_item_subtype` | Optional normalized source text for a parenthetical subtype such as `dagger`, `plate`, or `any axe`. |
| `rarity` | The item's canonical rarity. |
| `usage_mode` | Whether the item is worn, held, portable, or consumed. |
| `wear_slot` | The equipment slot occupied by a worn item. It is null for held, portable, and consumed items. |
| `requires_attunement` | Whether a user must attune to the item. |
| `attunement_requirement` | Optional source-derived text describing who may attune when a special requirement exists. |
| `raw_description` | The extracted source description retained without semantic rewriting. It must not be blank. |
| `needs_review` | Whether the structured interpretation requires review. The workflow that sets and resolves this flag is deferred to the extraction-pipeline design. |

The database enforces these consistency rules:

- A worn item must have a wear slot.
- A held, portable, or consumed item must not have a wear slot.
- An item that does not require attunement must not have an attunement requirement.

### 2.3 Effect

| Field | Meaning |
|---|---|
| `item_id` | Foreign key to the MagicItem that provides the effect. |
| `category` | Exactly one broad effect category. |
| `description` | Source-grounded description of the individual effect. It must not be blank. |

Deleting a MagicItem also deletes its dependent Effect records.

### 2.4 Limitation

| Field | Meaning |
|---|---|
| `item_id` | Foreign key to the MagicItem constrained by the limitation. |
| `effect_id` | Optional foreign key to the individual Effect constrained by the limitation. Null means the limitation applies to the item as a whole. |
| `description` | Source-grounded description of the usage limitation. It must not be blank. |

The database must ensure that a referenced Effect belongs to the same MagicItem as the Limitation. Deleting a MagicItem also deletes its dependent Limitation records.

### 2.5 Canonical vocabularies

```text
rarity:
  common
  uncommon
  rare
  very_rare
  legendary
  artifact
  varies

source_item_type:
  wondrous_item
  weapon
  armor
  potion
  ring

usage_mode:
  worn
  held
  portable
  consumed

wear_slot:
  head
  neck
  torso
  outerwear
  hands
  feet
  finger

effect_category:
  offensive
  defensive
  utility
```

`usage_mode` and `wear_slot` represent a conceptual hierarchy with two database fields:

```text
usage_mode
|-- worn
|   |-- head
|   |-- neck
|   |-- torso
|   |-- outerwear
|   |-- hands
|   |-- feet
|   `-- finger
|-- held
|-- portable
`-- consumed
```

Slots model equipment conflicts: two worn items conflict only when they occupy the same slot. For example, a ring and hat do not conflict, gloves and a ring do not conflict, and armor can be worn with a cloak because `torso` and `outerwear` are separate slots.

Every independently understandable ability is represented as a separate Effect and assigned exactly one broad category. Whole source paragraphs must not be stored as a single Effect when they contain multiple independent abilities. If an ability could plausibly fit more than one category, extraction uses its primary purpose and marks uncertain cases for review.

Detailed effect kinds such as `attack_bonus`, `resistance`, or `spellcasting` are not part of the vocabulary. The complete PDF shows that these concepts overlap frequently and would require a larger, less stable classification system without improving the README's requested offensive, defensive, and utility analysis.

## 3. Complete PDF audit evidence

The source PDF was treated as a scanned document because its embedded text layer contained almost no catalog content. All pages were rendered and visually inspected; OCR was used only as supporting evidence and corrected against page layout.

| Measure | Result |
|---|---|
| PDF pages | 39 |
| Pages rendered | 39/39 |
| Pages visually inspected | 39/39 |
| Catalog items identified | 80 |
| Unresolved item boundaries | 0 |

Four records cross PDF page boundaries:

1. Exo-Armor, PDF pages 7-9.
2. Ring of Elven Lords, PDF pages 22-24.
3. War Drum of the Horde, PDF pages 29-31.
4. Amulet of Encasement, PDF pages 38-39.

Page references are audit metadata and are not part of the business ontology.

## 4. Pending validation and change record

This section records the evidence-driven decisions made after the complete PDF audit. Approved ontology changes are incorporated into Section 2.

| Topic | PDF evidence | Current handling | Decision | Status |
|---|---|---|---|---|
| Creature applicability | Fiend and undead recur, but many targets such as bronze dragon, vampire, construct, medusa, elf, and humanoid occur only once or twice; relationships include damage, protection, control, healing, and favor | Preserve the information in Effect or Limitation descriptions | Do not add a CreatureTarget object because the observed vocabulary and relationships are not stable enough | Approved: no schema change |
| Environment applicability | Examples include wooded terrain, underwater use, sunshine and soil, and planar references, but they serve different semantic roles | Preserve the information in Effect or Limitation descriptions | Do not add an EnvironmentTarget object | Approved: no schema change |
| Detailed effect kinds | Repeated concepts include attack bonus, AC bonus, resistance, immunity, movement, spellcasting, healing, and transformation, with frequent overlap | Split independent abilities into separate Effects and assign one broad category | Keep only `offensive`, `defensive`, and `utility` | Approved: extraction rule clarified |
| Source item type | The complete PDF uses five top-level labels: Wondrous item, Weapon, Armor, Potion, and Ring; Weapon and Armor often include parenthetical subtypes | Store a five-value canonical type and optional subtype text | Add `source_item_type` and `source_item_subtype` to MagicItem | Approved: ontology change |
| Gold price | The PDF has no catalog sale prices; `50 gp` is a Quartermaster's Chest ability limit and a historical sale appears only in narrative | Preserve those values in the applicable description | Do not add a gold-price field or domain | Approved: no schema change |
| Limitation attachment | Repeated effect-specific limits include cooldowns, charges, and consequences of activating an individual ability | Keep `item_id`; use optional `effect_id` when the limitation applies to one Effect | Add an optional Effect relationship with same-item integrity | Approved: ontology change |
| Variable rarity | Pouch of False Coins is explicitly labeled `Wondrous item, varies` | Preserve the explicit source rarity | Add `varies` to rarity | Approved: ontology change |
| Consumed usage | Four potion or elixir records are activated by drinking and do not fit worn, held, or reusable portable use | Represent consumption directly | Add `consumed` to usage_mode | Approved: ontology change |
| Wear slots | All explicitly worn records fit the seven confirmed slots; Backpack of Holding does not require wearing to function | Treat the backpack as portable; retain the README-supported hands slot for future gloves | Keep the existing seven slots | Approved: no schema change |
| Name uniqueness | The 80-item inventory has no duplicate names, but that does not prove future catalogs prohibit duplicate display names | Defer normalization, deduplication, and stable source identity to the extraction pipeline | Keep `name` non-unique | Approved: no schema change |

For each future proposed change, record:

1. A real example from the PDF.
2. Why the confirmed ontology cannot represent it adequately.
3. The smallest schema or vocabulary change that resolves it.
4. The relationships and existing data affected.
5. The cost of leaving the ontology unchanged.

An isolated detail does not automatically justify a new object type or enum. Information without stable analytical value remains in the source-grounded description.
