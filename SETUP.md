# Setup

## Prerequisites

- Go 1.24+
- Docker (for the local Postgres)
- [`sqlc`](https://docs.sqlc.dev/en/latest/overview/install.html) — `brew install sqlc`

## 1. Start Postgres

```bash
docker compose up -d
```

One pinned PostgreSQL 18 (its built-in `uuidv7()` backs the `object` base class),
reachable at `postgres://postgres:postgres@localhost:5433/postgres`. Override with
`DATABASE_URL` if you need to. Reset the database at any time with:

```bash
docker compose down -v && docker compose up -d
```

## 2. Verify

```bash
go run ./cmd/verify
```

You should see `postgres is ready`.

## 3. Extraction pipeline

The extractor is AI-driven: local OCR supplies page text, the model identifies
items and fields, Go normalizes values into the ontology enums, and generated
sqlc queries write accepted records. Do not put an API key in source files,
JSON artifacts, or commits.

Additional local prerequisite:

- Poppler (`pdftoppm`) for page rendering — `brew install poppler`
- Tesseract for OCR — `brew install tesseract`

Configure OpenAI in the local `.env` file (already ignored by Git):

```dotenv
OPENAI_API_KEY="..."
OPENAI_TEXT_MODEL="gpt-5.6-luna"    # optional routine extraction model
OPENAI_VISION_MODEL="gpt-5.6-terra" # optional image-recovery model
```

Shell environment variables take precedence over `.env`. The extractor uses
OpenAI's Responses API and strict JSON Schema structured output; do not put the
API key in source files, JSON artifacts, or commits.

Run modes:

```bash
go run ./cmd/extract --dry-run                 # extract and write artifacts; no DB calls
go run ./cmd/extract --dry-run --pages 7-9    # bounded smoke run
go run ./cmd/extract                           # full extraction and persistence
go run ./cmd/extract --resume-run RUN_ID       # reuse saved artifacts on a fresh DB
```

Each run writes `tmp/extraction/RUN_ID/ocr.json`, `raw_candidates.json`,
`normalized.json`, `review.json`, and `report.json`. Image recovery is targeted
to invalid or incomplete candidates; routine records use text extraction only.
The command retries transient provider responses within bounded limits and
records actual API attempts and semantic recovery counts in the report.

The catalog has no durable source-record identity yet. Therefore a non-empty
database is rejected, and `--resume-run` does not resume database writes into a
partially populated catalog. Reset only a dedicated local assignment database
before a full persistence run:

```bash
docker compose down -v && docker compose up -d
go run ./cmd/verify
go run ./cmd/extract --resume-run RUN_ID
```

The full-run quality gate expects 39 pages and 80 accounted records, including
the cross-page records Exo-Armor (7–9), Ring of Elven Lords (22–24), War Drum
of the Horde (29–31), and Amulet of Encasement (38–39). A partial `--pages`
run intentionally skips the global 39/80 check. Candidates that remain
structurally incomplete after image and reconciliation recovery are reported
as failures and are not inserted.

## 4. Your work

- **`database/schema/*.sql`** — define your ontology here, on top of the provided
  `foundation.sql`. List each new file in `database/sqlc.yaml` (in dependency
  order) and give it a `-- requires:` header so provisioning applies it after its
  parents.
- **Generate typed Go** after each schema change, from the `database/` directory:
  ```bash
  cd database && sqlc generate
  ```
  Output lands in `database/generated/`.
- **`cmd/extract`** — the extraction pipeline and CLI.
- **`stormland/`** — a complete worked example in a different domain (commercial
  real-estate leases). Read it as your reference for the whole loop, then delete
  it if you like. Its live end-to-end test runs against the compose database:
  ```bash
  go test ./stormland/
  ```

## Layout

```
compose.yaml            local Postgres (port 5433)
db/                     connection pool + declarative-schema provisioner
database/
  schema/foundation.sql base `object` class + touch trigger (provided)
  schema/               your entity types go here
  query/                your named queries for sqlc
  generated/            sqlc output (typed Go)
  sqlc.yaml
stormland/              worked example: schema -> sqlc -> normalized insert
cmd/verify              "postgres is ready" check
cmd/extract             your extraction pipeline (scaffold)
data/items_combined.pdf the source catalog
```
