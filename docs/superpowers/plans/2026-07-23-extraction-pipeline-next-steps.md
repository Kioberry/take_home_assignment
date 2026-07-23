# Extraction Pipeline Next Steps

> Cross-machine handoff. Current branch: `extraction-pipeline`.

## Current state

The implementation is complete through:

```text
go vet ./...
go test ./... -count=1
```

Both pass with local PostgreSQL and loopback access. Completed components include
PDF rendering/OCR, AI structured extraction, image recovery, cross-page merge,
normalization/validation, sqlc persistence, CLI/resume/reporting, and isolated
integration tests.

The branch is pushed to `origin` as `extraction-pipeline`. Do not merge it into
`astrid` until the live smoke checks below are reviewed.

## Next machine

```bash
git clone https://github.com/Kioberry/take_home_assignment.git
cd take_home_assignment
git fetch origin
git checkout -b extraction-pipeline --track origin/extraction-pipeline
brew install poppler tesseract sqlc
docker compose up -d
go run ./cmd/verify
```

Set the API key only in the shell; never commit it:

```bash
export OPENAI_API_KEY="..."
export OPENAI_TEXT_MODEL="gpt-5.6-luna"
export OPENAI_VISION_MODEL="gpt-5.6-terra"
```

## Verification sequence

First run:

```bash
go vet ./...
go test ./... -count=1
git diff --check
```

Then run the bounded real smoke test. It uses PDF rendering, OCR, and the API,
but does not connect to or modify PostgreSQL:

```bash
go run ./cmd/extract --dry-run --pages 7-9
```

Inspect `tmp/extraction/<RUN_ID>/` and verify Exo-Armor spans pages 7–9, no API
key appears in artifacts, and image/reconciliation calls are targeted.

Run the complete dry-run:

```bash
go run ./cmd/extract --dry-run
```

It must account for 39 pages and 80 records, including:

- Exo-Armor: pages 7–9
- Ring of Elven Lords: pages 22–24
- War Drum of the Horde: pages 29–31
- Amulet of Encasement: pages 38–39

Record the printed run ID and inspect `report.json`, `review.json`, and
`normalized.json`. Do not add handwritten PDF parsing rules when a candidate
fails; use the recorded validation reason and retry path.

## Fresh-database persistence

Only use the dedicated assignment database:

```bash
docker compose down -v
docker compose up -d
go run ./cmd/verify
go run ./cmd/extract --resume-run <FULL_DRY_RUN_ID>
```

Replace `<FULL_DRY_RUN_ID>` with the actual full dry-run ID. `--resume-run`
reuses extraction artifacts but requires a fresh database; the ontology has no
durable source-record identity yet. If an item fails, inspect the final report,
fix the pipeline without fabricating data, reset the database, and replay.

## Final acceptance

```bash
go test ./... -count=1
git diff --check
git status --short
```

Require 80 successful insertions or precise failures for every exception, all
four cross-page records complete, canonical enum values only, generated sqlc
writes only, and no API key in repository files or artifacts.

Known blocker on the previous machine: `pdftoppm` was missing and
`OPENAI_API_KEY` was unset, so no real API/PDF smoke run was performed there.
