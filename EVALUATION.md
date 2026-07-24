# Evaluation Evidence

This document separates verified local behavior from gates that require a
usable paid model account. It does not treat unit tests as proof that a live
catalog extraction succeeded.

## Ontology design

The catalog graph is `MagicItem -> Effect` and `MagicItem -> Limitation`, with
an optional `Limitation -> Effect` attachment. The schema uses the five closed
vocabularies documented in the [ontology design](docs/superpowers/specs/2026-07-22-magic-item-ontology-design.md): rarity, source item type, usage mode,
wear slot, and effect category.

The database enforces nonblank source text, wearing/slot consistency,
attunement consistency, cascade deletion, and same-item limitation integrity.
See [schema files](database/schema), generated [sqlc queries](database/generated/queries.sql.go), and [ontology tests](database/ontology_test.go).

Creature/environment targets, detailed effect kinds, sale price, and durable
source-record identity remain source-grounded text rather than unsupported
ontology structure; the decision evidence is in the ontology design's PDF
audit table.

## Extraction pipeline quality

The pipeline is:

```text
PDF -> render/OCR -> OpenAI structured extraction -> merge/deduplicate
    -> normalize/validate -> targeted image recovery -> sqlc transaction
    -> artifacts and terminal report
```

The default provider uses OpenAI Responses API strict JSON Schema output. OpenAI
text and image recovery models are configured locally through ignored `.env`
variables; shell variables take precedence. The default path uses bounded
transport retries, does not fabricate missing ontology values, records review
and failure evidence, and redacts the API key from durable artifacts.

Relevant coverage includes [provider tests](cmd/extract/anthropic_test.go),
[pipeline tests](cmd/extract/pipeline_test.go), [report tests](cmd/extract/report_test.go),
and [persistence tests](cmd/extract/persist_test.go). Full runs require 39
pages, 80 accounted records, and complete Exo-Armor (7-9), Ring of Elven Lords
(22-24), War Drum of the Horde (29-31), and Amulet of Encasement (38-39)
spans.

## Reproduce verification

Start the dedicated local database and run the local gates:

```bash
docker compose up -d
go run ./cmd/verify
go vet ./...
go test ./... -count=1
git diff --check
```

With a billed OpenAI API key and an account-available model configured in
`.env`, run the bounded smoke test:

```bash
go run ./cmd/extract --dry-run --pages 7-9
```

Then run the full dry-run and inspect its run artifacts:

```bash
go run ./cmd/extract --dry-run
```

Only after a successful full dry-run, reset the dedicated assignment database
and replay the artifacts into it:

```bash
docker compose down -v
docker compose up -d
go run ./cmd/verify
go run ./cmd/extract --resume-run RUN_ID
go test ./... -count=1
```

## Evidence status

| Gate | Latest actual status | Evidence |
| --- | --- | --- |
| Static checks and full local Go suite | Passed on macOS | `go vet ./...`, `go test ./... -count=1`, and `git diff --check`; see [engineering log](log.md) |
| OpenAI Responses API request contract | Passed locally | Mocked `cmd/extract` provider tests |
| Bounded pages 7-9 live smoke | Passed | Run `20260724T030712.308626000Z`: four normalized records, zero failures, no database writes; see `log.md` |
| Full 39-page/80-record dry-run | Failed quality gate | Run `20260724T055029.102717000Z`: 39 pages, 82 accounted candidates, 11 review candidates, and two image-recovery timeouts. Two same-page OCR title variants remain unmerged; see `log.md`. |
| Fresh-database persistence replay | Not run | Requires successful full dry-run |

## Known limits

- A usable paid OpenAI API key and an account-available model are required for
  real extraction; no successful live run is currently claimed.
- `tmp/extraction/` artifacts are local and ignored by Git.
- The pipeline intentionally rejects a nonempty catalog because it has no
  durable cross-run source identity; `--resume-run` is a fresh-database replay,
  not database-level idempotency.
