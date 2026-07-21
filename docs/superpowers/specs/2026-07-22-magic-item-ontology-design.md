# Magic Item Ontology Design

Date: 2026-07-22

## 1. Goal and scope

The catalog ontology should make magic items easy to add, traverse, and compare. The initial design focuses on the two dimensions Reznar identifies in the README:

1. How an item is used and, for worn items, which equipment slot it occupies.
2. What kinds of magical effects and usage limitations the item has.

This document covers only the first-pass ontology. AI extraction, confidence scoring, retries, human review, source-page tracking, and other pipeline behavior will be designed separately.

The first pass deliberately stays coarse. When the PDF provides repeated evidence that the current model cannot express an important distinction, the proposed extension must be reviewed before the ontology changes.

## 2. Confirmed ontology

### 2.1 Object model

The ontology has three object types. Each object inherits the provided `object` base table and therefore receives an `id`, `created_at`, and `updated_at`.

```text
MagicItem 1 --- N Effect
MagicItem 1 --- N Limitation
```

An item may have multiple effects and multiple limitations. Each effect has exactly one broad category. A multi-purpose item is represented by multiple effect records rather than one effect with multiple categories.

### 2.2 MagicItem

| Field | Meaning |
|---|---|
| `name` | The item's catalog name. It must not be blank. It is not initially unique because duplicate-name behavior has not yet been checked against the complete PDF. |
| `rarity` | The item's canonical rarity. |
| `usage_mode` | Whether the item is worn, held, or portable. |
| `wear_slot` | The equipment slot occupied by a worn item. It is null for held and portable items. |
| `requires_attunement` | Whether a user must attune to the item. |
| `attunement_requirement` | Optional source-derived text describing who may attune when a special requirement exists. |
| `raw_description` | The extracted source description retained without semantic rewriting. It must not be blank. |
| `needs_review` | Whether the structured interpretation requires review. The workflow that sets and resolves this flag is deferred to the extraction-pipeline design. |

The database enforces these consistency rules:

- A worn item must have a wear slot.
- A held or portable item must not have a wear slot.
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
| `description` | Source-grounded description of the usage limitation. It must not be blank. |

Limitations initially belong to the item as a whole. If the PDF repeatedly shows that limitations need to attach to individual effects, that relationship will be proposed as an ontology change. Deleting a MagicItem also deletes its dependent Limitation records.

### 2.5 Canonical vocabularies

```text
rarity:
  common
  uncommon
  rare
  very_rare
  legendary
  artifact

usage_mode:
  worn
  held
  portable

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
`-- portable
```

Slots model equipment conflicts: two worn items conflict only when they occupy the same slot. For example, a ring and hat do not conflict, gloves and a ring do not conflict, and armor can be worn with a cloak because `torso` and `outerwear` are separate slots.

Detailed effect kinds are not part of the initial vocabulary. The three broad categories are the stable first pass; finer kinds require PDF evidence and review.

## 3. Pending validation and change record

This section is the single backlog for evidence-driven ontology changes. A topic remains pending until PDF evidence demonstrates whether the current design is sufficient. An approved change is incorporated into Section 2.

| Topic | PDF evidence | Current handling | Possible change | Status |
|---|---|---|---|---|
| Creature applicability | Complete catalog not yet reviewed for relationship patterns | Preserve the information in descriptions | Relate creature concepts to MagicItem, Effect, or both | Pending |
| Environment applicability | Complete catalog not yet reviewed for relationship patterns | Preserve the information in descriptions | Relate environment concepts to MagicItem, Effect, or both | Pending |
| Detailed effect kinds | Frequency and stable vocabulary not yet measured | Use `offensive`, `defensive`, or `utility` plus description | Add a small mixed core vocabulary with an `other` fallback | Pending |
| Source item type | Complete set of source labels not yet measured | Preserve source wording in `raw_description` | Add a canonical item-type field or object | Pending |
| Gold price | Presence and format in the source have not yet been verified | Do not model it yet | Add a constrained gold-price domain if supported by the PDF | Pending |
| Limitation attachment | Initial model attaches limitations to MagicItem | Preserve effect-specific detail in the limitation description | Allow a limitation to relate to an individual Effect | Pending |

For each proposed change, record:

1. A real example from the PDF.
2. Why the confirmed ontology cannot represent it adequately.
3. The smallest schema or vocabulary change that resolves it.
4. The relationships and existing data affected.
5. The cost of leaving the ontology unchanged.

An isolated detail does not automatically justify a new object type or enum. Information without stable analytical value remains in the source-grounded description.
