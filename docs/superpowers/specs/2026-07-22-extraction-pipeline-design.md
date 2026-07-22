# Extraction Pipeline Design

Date: 2026-07-22

## 1. Goal and scope

Implement `cmd/extract` as an AI-driven Go pipeline that reads
`data/items_combined.pdf`, extracts all 80 catalog items, normalizes them into
the approved ontology vocabulary, and persists them through generated sqlc
queries.

The ontology is already designed and provisioned. This work does not redesign
the schema unless later evidence proves that durable extraction source identity
must become ontology data.

## 2. Architecture

The pipeline uses a quality-first adaptive path:

```text
PDF -> local page rendering and OCR
    -> batched text AI extraction
    -> merge and in-run deduplication
    -> normalize and validate
    -> targeted multimodal retry when needed
    -> sqlc transaction per item
    -> extraction report
```

Local OCR keeps routine extraction economical. It is supporting input rather
than the semantic parser: the AI model identifies item boundaries, fields,
effects, and limitations. Original page images are used when OCR-based output
is incomplete, inconsistent, or invalid.

## 3. Extraction flow

1. Render all PDF pages and run local OCR while preserving page numbers.
2. Send overlapping batches of approximately four to six OCR pages to the text
   model. The exact batch size is configurable.
3. Ask the model for strict structured output containing complete items or
   explicitly marked continuation fragments.
4. Merge fragments across overlapping batches using normalized name, adjacent
   source pages, continuation state, and description continuity.
5. Deduplicate repeated candidates within the current run using normalized
   name, source pages, and raw-description hash.
6. Normalize raw vocabulary values and validate ontology consistency.
7. For invalid, incomplete, or conflicting candidates, retry against the
   relevant page images. Include adjacent pages when the record may cross a
   boundary.
8. If the first image retry remains inconsistent, perform one reconciliation
   request using the images, OCR text, prior candidates, and validation errors.
9. Persist accepted candidates through generated sqlc methods.
10. Write an auditable report for accepted, reviewed, and failed candidates.

Known completeness checks are 39 pages, 80 items, and these four cross-page
records: Exo-Armor (7-9), Ring of Elven Lords (22-24), War Drum of the Horde
(29-31), and Amulet of Encasement (38-39). These facts verify extraction; they
do not hard-code item content.

## 4. AI output contract

Each candidate uses this logical shape:

```json
{
  "name": "",
  "source_pages": [],
  "source_item_type_raw": "",
  "source_item_subtype_raw": null,
  "rarity_raw": "",
  "usage_mode_raw": "",
  "wear_slot_raw": null,
  "requires_attunement": false,
  "attunement_requirement": null,
  "raw_description": "",
  "effects": [
    {"category_raw": "", "description": ""}
  ],
  "limitations": [
    {"effect_index": null, "description": ""}
  ],
  "confidence": 0.0,
  "review_reasons": []
}
```

The model must preserve source-grounded descriptions, return null plus a review
reason when uncertain, and never invent unsupported values. A null limitation
`effect_index` means the limitation applies to the item as a whole.

## 5. Normalization and validation

Deterministic Go mappings convert case, spacing, and known aliases into the
existing enum vocabulary. Examples include `Very Rare -> very_rare`,
`Wondrous Item -> wondrous_item`, and `cloak -> worn/outerwear`.

Normalization never creates new enum values. Ambiguous values trigger an image
retry. After retries, a source-supported legal interpretation may be persisted
with `needs_review=true`; a candidate without a defensible legal value fails.

Validation checks:

- non-blank item name and raw description;
- valid canonical enum values;
- worn items have a wear slot and other usage modes do not;
- unattuned items have no attunement requirement;
- effect and limitation descriptions are non-blank;
- limitation effect indexes refer to an effect on the same candidate;
- source pages are valid and contiguous;
- apparent truncation, continuation, or extraction conflicts are resolved.

Descriptions are not semantically rewritten during normalization.

## 6. Recovery and review policy

An invalid or incomplete OCR extraction does not immediately fail. It triggers
a targeted multimodal retry for the candidate's source pages, with the exact
validation errors included in the request. One reconciliation request follows
if needed.

After recovery:

- valid and complete candidates are inserted normally;
- valid candidates with limited semantic uncertainty are inserted with
  `needs_review=true` and explicit review reasons in the run report;
- candidates still missing core identity, boundaries, or source content are not
  inserted and receive a precise failure report;
- no field is fabricated merely to satisfy a database constraint.

## 7. Persistence and runtime idempotency

Each item is written in its own database transaction:

```text
Begin -> InsertMagicItem -> InsertEffect x N -> InsertLimitation x N -> Commit
```

Any failure rolls back the whole item without blocking unrelated items.
Limitation effect indexes are resolved to IDs returned by the effect inserts.
Only generated sqlc queries are used.

The first version guarantees deduplication within one run, not durable
cross-run idempotency. Because the ontology has no source identity and item
names are intentionally non-unique, the command refuses to populate a non-empty
catalog by default. The report states that interrupted database runs require a
fresh database before retrying.

A later ontology refinement may add an explicit source document or source
record identity. It must not overload `source_item_type`, which describes the
catalog's source type rather than extraction provenance.

## 8. CLI and artifacts

Supported execution modes:

```text
go run ./cmd/extract
go run ./cmd/extract --dry-run
go run ./cmd/extract --pages 1-5
go run ./cmd/extract --resume-run <run-id>
```

Configuration uses `DATABASE_URL`, an AI API key, and configurable text and
vision model names. API retry counts and batch sizes are bounded and
configurable. `--resume-run` reuses OCR and AI artifacts; it does not provide
database-level cross-run idempotency.

Each run writes:

```text
tmp/extraction/<run-id>/
  ocr.json
  raw_candidates.json
  normalized.json
  review.json
  report.json
```

The final report includes page count, candidate count, normalized count,
multimodal retries, review count, failures, inserted items, and API call counts.
A complete run accounts for all 80 expected items as inserted or explicitly
failed.

## 9. Code boundaries

```text
cmd/extract/
  main.go       CLI and orchestration
  model.go      candidate structures
  pdf.go        page rendering
  ocr.go        local OCR
  ai.go         provider and structured extraction
  merge.go      fragment merge and in-run deduplication
  normalize.go  canonical vocabulary mapping
  validate.go   structural and completeness checks
  persist.go    sqlc transaction writes
  report.go     run artifacts and summary
```

Each unit has one responsibility and is independently testable. AI access is
behind an interface so normalization, validation, merge, retry orchestration,
and persistence tests do not call an external API.

## 10. Verification strategy

Tests cover enum aliases, ontology consistency, cross-page fragment merging,
overlap deduplication, retry triggers, transaction rollback, dry-run behavior,
and report accounting. Live verification confirms 39 rendered pages, 80
accounted-for records, complete extraction of the four known cross-page items,
and successful database writes through generated queries.

