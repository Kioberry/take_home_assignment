# Evaluation notes

## 0. Design process

I used an evidence-driven refinement loop:

1. Draft a minimal ontology from Reznar's stated needs: equipment conflicts,
   effects, and limitations.
2. Audit the complete PDF against that draft.
3. For each mismatch, assess whether the model could represent it, whether the
   distinction recurred, and whether it enabled a useful catalog query.
4. Make only the smallest justified changes; finalize the specification before
   implementing the SQL schema.

This added source item type, variable rarity, consumed items, and
effect-specific limitations. It rejected creature, environment, and detailed
effect-type objects: the catalog did not supply stable enough vocabulary or
query value for them.

```text
Initial design
  -> Full-PDF audit
  -> Evidence-backed proposals
  -> Final ontology specification
  -> SQL schema and generated types
```

## 1. Ontology design

I designed the ontology around the questions Reznar needs to ask: where an
item is used, what it does, and what constrains its use.

### Sensible entity types

The model has three entity types:

- `MagicItem` represents a cataloged item and its identity-level facts: name,
  rarity, source item type, attunement, and usage.
- `Effect` represents one independently understandable ability. A multi-purpose
  item has multiple Effects rather than one mixed-capability paragraph.
- `Limitation` represents a restriction, cost, cooldown, charge limit, or
  condition on an item or one of its effects.

Wear location is a controlled `wear_slot` value on `MagicItem`, rather than an
entity: the assignment needs it to identify equipment conflicts. Its values are
`head`, `neck`, `torso`, `outerwear`, `hands`, `feet`, and `finger`.

I deliberately did not model every creature, environment, or detailed effect
subtype mentioned in the PDF. Those terms are too sparse or inconsistent for a
stable vocabulary, so they remain in source-grounded descriptions.

```text
PDF item
|
|-- independent, repeatable, queryable
|   |-- MagicItem
|   |-- Effect
|   `-- Limitation
|
|-- stable, finite vocabulary
|   |-- rarity
|   |-- source item type
|   |-- usage mode
|   `-- wear slot
|
`-- open or inconsistent source language
    |-- subtype
    |-- effect description
    `-- raw description
```

### Value types that encode meaning

Stable catalog vocabularies are Postgres enums: rarity, source item type, usage
mode, wear slot, and effect category. This makes invalid values
unrepresentable and gives generated Go code the same vocabulary.

Variable source language remains `text`: item subtype, attunement requirement,
raw description, effect description, and limitation description.
`raw_description` is preserved without semantic rewriting so extracted
structure remains auditable.

Nullability means "not applicable," not "unknown." `wear_slot` is required for
worn items and null for held, portable, and consumed items; a database
constraint enforces both directions. Likewise, an attunement requirement can
exist only when an item requires attunement.

`source_item_type` preserves the source catalog's mutually exclusive top-level
labels. It describes what the source calls an item, while `usage_mode` describes
how the item is used; the two dimensions remain separate.

```text
Is the vocabulary stable and finite?
|
|-- Yes -> enum
|          rarity, usage_mode, wear_slot
|
`-- No
    |-- Source language that must be preserved? -> text
    |   raw_description, subtype, descriptions
    |
    `-- Applicable only in a specific state? -> nullable field + constraint
        wear_slot is required only for worn items
```

### Honest relationships

```text
                    +-----------+
                    | MagicItem |
                    +-----------+
                     1       1
                     |       |
                   0..N    0..N
                     |       |
              +--------+  +------------+
              | Effect |  | Limitation |
              +--------+  +------------+
                   ^           |
                   |           | 0..1
                   `-----------'
```

Each Effect and Limitation belongs to exactly one MagicItem. A Limitation may
also reference one Effect from that same item, distinguishing an item-wide
restriction from an effect-specific one. The composite foreign key
`(effect_id, item_id)` prevents cross-item references. Deleting an item
cascades to its dependent Effects and Limitations, matching their lifecycle.

## 2. Pipeline quality

The pipeline design uses AI for semantic extraction and deterministic code for
data quality.

It first produces page-aware OCR text in batches. AI identifies item
boundaries, effects, and limitations through structured JSON; it preserves raw
source text and reports uncertainty instead of guessing. Code then normalizes
known variants into canonical values and validates ontology rules.

Invalid or incomplete records receive a targeted image-based retry. Unresolved
records are marked for review or reported as failures, never silently inserted.
This keeps routine extraction efficient while reserving expensive multimodal
calls for ambiguous pages. Completeness is checked against page, item, and
cross-page-record counts.

```text
PDF pages
  -> OCR with page numbers
  -> AI extraction: semantic boundaries and meaning
  -> Deterministic code: normalize and validate
       | valid                         | invalid or incomplete
       v                               v
   Persist                      Targeted image retry
                                      |
                                      `-> normalize and validate again
                                             |
                                             `-> review or failure report
```

> AI interprets the source; deterministic code controls data quality and
> escalation.
