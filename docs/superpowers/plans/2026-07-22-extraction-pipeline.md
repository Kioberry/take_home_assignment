# AI Extraction Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `cmd/extract` into an AI-driven pipeline that extracts all 80 PDF records, normalizes them into the approved ontology, and inserts accepted records through generated sqlc queries.

**Architecture:** Render and OCR the PDF locally, then send overlapping OCR batches to an AI provider behind a Go interface. Deterministic Go code merges, normalizes, and validates candidates; targeted page-image requests recover uncertain records before each accepted item is persisted in its own transaction and all run artifacts are written to disk.

**Tech Stack:** Go 1.26, standard library HTTP/JSON/process execution, Poppler `pdftoppm`, Tesseract OCR, OpenAI Responses API with Structured Outputs, PostgreSQL 18, pgx/v5, sqlc.

## Global Constraints

- Extraction must be AI-driven; OCR is supporting input and must not become a hand-written semantic parser.
- Preserve source-grounded descriptions and never fabricate fields to satisfy the database.
- Use only generated sqlc methods for application writes; no raw SQL or handwritten DAO in `cmd/extract`.
- Default to `OPENAI_TEXT_MODEL=gpt-5.6-luna` for routine OCR batches and `OPENAI_VISION_MODEL=gpt-5.6-terra` for targeted recovery; both remain configurable.
- Bound text retries to 2 transport attempts and semantic recovery to one image retry plus one reconciliation request.
- Refuse database writes when `magic_item` is non-empty; `--dry-run` never connects to or mutates PostgreSQL.
- Runtime deduplication is in-run only; do not add extraction provenance to the ontology in this version.
- A full run expects 39 pages and accounts for 80 records, including the four cross-page records named in the design.
- Do not stage or modify `.agents/`, `skills-lock.json`, or unrelated user changes.

---

## File map

- `cmd/extract/main.go`: CLI parsing, dependency wiring, orchestration, exit status.
- `cmd/extract/model.go`: raw, normalized, page, error, and run data contracts.
- `cmd/extract/artifacts.go`: atomic JSON artifact reads/writes and resume loading.
- `cmd/extract/pdf.go`: Poppler rendering and page selection.
- `cmd/extract/ocr.go`: Tesseract invocation and OCR cache production.
- `cmd/extract/ai.go`: AI interface, prompts, JSON Schema, OpenAI Responses client.
- `cmd/extract/merge.go`: continuation merge and in-run deduplication.
- `cmd/extract/normalize.go`: deterministic canonical vocabulary mappings.
- `cmd/extract/validate.go`: candidate invariants, completeness checks, retry reasons.
- `cmd/extract/pipeline.go`: batch extraction and bounded multimodal recovery policy.
- `cmd/extract/persist.go`: non-empty guard and per-item pgx/sqlc transaction.
- `cmd/extract/report.go`: review/failure records and final accounting.
- `cmd/extract/*_test.go`: focused unit and database integration tests.
- `database/query/queries.sql`: generated read query used by the non-empty guard.
- `database/generated/queries.sql.go`: regenerated sqlc output; never edit by hand.
- `SETUP.md`: extraction dependencies, environment, and commands.

### Task 1: Core contracts, configuration, and artifacts

**Files:**
- Create: `cmd/extract/model.go`
- Create: `cmd/extract/artifacts.go`
- Create: `cmd/extract/artifacts_test.go`

**Interfaces:**
- Produces: `Config`, `Page`, `OCRPage`, `RawCandidate`, `NormalizedCandidate`, `ValidationIssue`, `RunState`, `ArtifactStore`, `NewArtifactStore`, `WriteJSON`, and `ReadJSON`.

- [ ] **Step 1: Write failing artifact tests**

Test that `WriteJSON("ocr.json", value)` creates valid indented JSON, a second write atomically replaces it, and `ReadJSON` restores the exact page numbers/text. Test that `NewArtifactStore(root, "../escape")` rejects path traversal and that a missing resume artifact returns an error containing its path.

- [ ] **Step 2: Verify the tests fail**

Run: `go test ./cmd/extract -run 'TestArtifact' -v`

Expected: FAIL because `ArtifactStore` is undefined.

- [ ] **Step 3: Define exact contracts**

Add JSON-tagged types using these signatures:

```go
type Page struct { Number int; ImagePath string }
type OCRPage struct { Number int `json:"number"`; Text string `json:"text"` }
type RawEffect struct { CategoryRaw string `json:"category_raw"`; Description string `json:"description"` }
type RawLimitation struct { EffectIndex *int `json:"effect_index"`; Description string `json:"description"` }
type RawCandidate struct {
    Name string `json:"name"`; SourcePages []int `json:"source_pages"`
    SourceItemTypeRaw string `json:"source_item_type_raw"`; SourceItemSubtypeRaw *string `json:"source_item_subtype_raw"`
    RarityRaw string `json:"rarity_raw"`; UsageModeRaw string `json:"usage_mode_raw"`; WearSlotRaw *string `json:"wear_slot_raw"`
    RequiresAttunement bool `json:"requires_attunement"`; AttunementRequirement *string `json:"attunement_requirement"`
    RawDescription string `json:"raw_description"`; Effects []RawEffect `json:"effects"`; Limitations []RawLimitation `json:"limitations"`
    Confidence float64 `json:"confidence"`; ReviewReasons []string `json:"review_reasons"`; Continuation bool `json:"continuation"`
}
type NormalizedCandidate struct {
    Raw RawCandidate `json:"raw"`; SourceItemType generated.SourceItemType `json:"source_item_type"`; Rarity generated.Rarity `json:"rarity"`
    UsageMode generated.UsageMode `json:"usage_mode"`; WearSlot *generated.WearSlot `json:"wear_slot"`
    Effects []NormalizedEffect `json:"effects"`; NeedsReview bool `json:"needs_review"`; ReviewReasons []string `json:"review_reasons"`
}
type ValidationIssue struct { Code string `json:"code"`; Message string `json:"message"`; Recoverable bool `json:"recoverable"` }
```

`Config` contains PDF path, run ID/root, selected pages, dry-run/resume flags, batch size default 5, overlap default 1, expected pages 39, expected items 80, API URL/key/models, and retry limits. Keep secrets out of JSON artifacts.

- [ ] **Step 4: Implement atomic artifacts**

`WriteJSON` must create the run directory with mode `0755`, encode into `<name>.tmp` with indentation, close it, and rename it to the final file. `ReadJSON` uses `json.Decoder.DisallowUnknownFields()` so stale or malformed resume data fails loudly.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w cmd/extract/model.go cmd/extract/artifacts.go cmd/extract/artifacts_test.go && go test ./cmd/extract -run 'TestArtifact' -v`

Expected: PASS.

Commit: `git add cmd/extract/model.go cmd/extract/artifacts.go cmd/extract/artifacts_test.go && git commit -m "feat: add extraction contracts and artifacts"`

### Task 2: PDF rendering and local OCR

**Files:**
- Create: `cmd/extract/pdf.go`
- Create: `cmd/extract/ocr.go`
- Create: `cmd/extract/pdf_test.go`
- Create: `cmd/extract/ocr_test.go`

**Interfaces:**
- Consumes: `Page`, `OCRPage`, `ArtifactStore`.
- Produces: `CommandRunner.Run(ctx, name, args...)`, `ParsePageRange(string, max int) ([]int, error)`, `RenderPages(ctx, runner, pdfPath, outputDir string, selected []int) ([]Page, error)`, and `OCRPages(ctx, runner, []Page) ([]OCRPage, error)`.

- [ ] **Step 1: Write failing command-boundary tests**

Use a fake `CommandRunner` to assert rendering invokes `pdftoppm -png -r 200` and selected-page runs include `-f N -l N`; assert OCR invokes `tesseract <image> stdout -l eng --psm 6`. Cover `--pages 1-5`, `3`, `5-3`, `0`, and values over 39.

- [ ] **Step 2: Verify failure**

Run: `go test ./cmd/extract -run 'Test(ParsePageRange|RenderPages|OCRPages)' -v`

Expected: FAIL because rendering/OCR functions are undefined.

- [ ] **Step 3: Implement process execution without semantic parsing**

Use `exec.CommandContext`; include stderr in wrapped errors. Render images as `page-001.png`, check every expected output exists, sort numerically, and preserve PDF page numbers. OCR only trims trailing whitespace and rejects blank OCR output; it must not infer item fields or boundaries.

- [ ] **Step 4: Verify and commit**

Run: `gofmt -w cmd/extract/pdf.go cmd/extract/ocr.go cmd/extract/pdf_test.go cmd/extract/ocr_test.go && go test ./cmd/extract -run 'Test(ParsePageRange|RenderPages|OCRPages)' -v`

Expected: PASS.

Commit: `git add cmd/extract/pdf.go cmd/extract/ocr.go cmd/extract/pdf_test.go cmd/extract/ocr_test.go && git commit -m "feat: render and OCR catalog pages"`

### Task 3: AI extraction provider and strict structured output

**Files:**
- Create: `cmd/extract/ai.go`
- Create: `cmd/extract/ai_test.go`

**Interfaces:**
- Consumes: `OCRPage`, `Page`, `RawCandidate`, `ValidationIssue`.
- Produces:

```go
type ExtractRequest struct { OCR []OCRPage; Images []Page; Prior []RawCandidate; Issues []ValidationIssue; Mode string }
type AIExtractor interface { Extract(context.Context, ExtractRequest) ([]RawCandidate, error) }
func NewOpenAIExtractor(httpClient *http.Client, apiURL, apiKey, textModel, visionModel string, maxAttempts int) *OpenAIExtractor
```

- [ ] **Step 1: Write failing HTTP contract tests**

Use `httptest.Server` to verify `POST /v1/responses`, bearer authentication, text model selection without images, vision model selection with images, base64 `input_image` data URLs, and `text.format={type:"json_schema", strict:true}`. Return a fixture Responses payload and assert candidates decode. Test 429 then success, permanent 400, refusal, missing output text, and unknown candidate fields.

- [ ] **Step 2: Verify failure**

Run: `go test ./cmd/extract -run 'TestOpenAIExtractor' -v`

Expected: FAIL because `NewOpenAIExtractor` is undefined.

- [ ] **Step 3: Implement the prompt and JSON Schema**

The system prompt must state: identify semantic item boundaries; preserve descriptions; emit complete candidates or `continuation=true`; use only source evidence; return null/review reason when uncertain; index effects from zero; page numbers must come from supplied page labels. The root schema is `{ "candidates": [...] }`, sets `additionalProperties:false` on every object, requires every field in `RawCandidate`, and allows nullable subtype, wear slot, attunement requirement, and limitation effect index.

- [ ] **Step 4: Implement bounded Responses API transport**

Use standard-library `net/http`; retry only 408, 429, and 5xx with context-aware backoff of 250ms then 750ms. Parse output text from `output[].content[]` where `type == "output_text"`; report refusals and malformed responses without attempting to repair JSON locally. Do not log the API key or full source text.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w cmd/extract/ai.go cmd/extract/ai_test.go && go test ./cmd/extract -run 'TestOpenAIExtractor' -v`

Expected: PASS.

Commit: `git add cmd/extract/ai.go cmd/extract/ai_test.go && git commit -m "feat: add structured AI extraction provider"`

### Task 4: Cross-page merge and in-run deduplication

**Files:**
- Create: `cmd/extract/merge.go`
- Create: `cmd/extract/merge_test.go`

**Interfaces:**
- Consumes/produces: `[]RawCandidate`.
- Produces: `MergeCandidates(batches [][]RawCandidate) (merged []RawCandidate, conflicts []ValidationIssue)` and `CandidateKey(RawCandidate) string`.

- [ ] **Step 1: Write failing merge tests**

Fixtures must cover: exact overlap duplicate removed; same name but different pages retained; adjacent continuation fragments merged in page order; effects/limitations retained once; conflicting non-empty rarity values emit a recoverable conflict; and the four known cross-page names produce page sets `7-9`, `22-24`, `29-31`, `38-39`.

- [ ] **Step 2: Verify failure**

Run: `go test ./cmd/extract -run 'Test(MergeCandidates|CandidateKey)' -v`

Expected: FAIL because merge functions are undefined.

- [ ] **Step 3: Implement deterministic merge rules**

Normalize identity only for comparison with lowercase, collapsed whitespace, and punctuation trimming. Dedup key is normalized name + sorted pages + SHA-256 of whitespace-normalized raw description. Merge only when names match, pages overlap/are adjacent, and at least one fragment is marked continuation; union pages/effects/limitations stably, concatenate non-overlapping description text, take minimum confidence, and union review reasons. Never silently choose between conflicting non-empty scalar fields.

- [ ] **Step 4: Verify and commit**

Run: `gofmt -w cmd/extract/merge.go cmd/extract/merge_test.go && go test ./cmd/extract -run 'Test(MergeCandidates|CandidateKey)' -v`

Expected: PASS.

Commit: `git add cmd/extract/merge.go cmd/extract/merge_test.go && git commit -m "feat: merge and deduplicate extracted records"`

### Task 5: Canonical normalization and validation

**Files:**
- Create: `cmd/extract/normalize.go`
- Create: `cmd/extract/normalize_test.go`
- Create: `cmd/extract/validate.go`
- Create: `cmd/extract/validate_test.go`

**Interfaces:**
- Consumes: `RawCandidate` and generated ontology enum types.
- Produces: `Normalize(RawCandidate) (NormalizedCandidate, []ValidationIssue)` and `Validate(NormalizedCandidate, pageLimit int) []ValidationIssue`.

- [ ] **Step 1: Write failing table-driven normalization tests**

Cover every generated enum value plus aliases: `Very Rare -> very_rare`, `Wondrous Item -> wondrous_item`, `cloak -> worn/outerwear`, `ring -> worn/finger`, armor/weapon held or worn only when source wording supports it, and unknown/ambiguous input returns a recoverable issue instead of a guessed enum.

- [ ] **Step 2: Write failing invariant tests**

Cover blank name/description, invalid confidence outside `[0,1]`, non-contiguous/out-of-range pages, worn without slot, non-worn with slot, unattuned with requirement, blank effect/limitation descriptions, and limitation indexes outside the effect slice. Assert descriptions are byte-for-byte unchanged.

- [ ] **Step 3: Verify failure**

Run: `go test ./cmd/extract -run 'Test(Normalize|Validate)' -v`

Expected: FAIL because normalization and validation are undefined.

- [ ] **Step 4: Implement minimal explicit alias maps and invariants**

Maps return `(value, found)` and never use a generic fallback. Unknown core enums produce recoverable issues; impossible structure produces non-recoverable issues. `Normalize` carries AI review reasons and sets `NeedsReview` when a legal value is source-supported but confidence is below `0.85` or review reasons remain.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w cmd/extract/normalize.go cmd/extract/normalize_test.go cmd/extract/validate.go cmd/extract/validate_test.go && go test ./cmd/extract -run 'Test(Normalize|Validate)' -v`

Expected: PASS.

Commit: `git add cmd/extract/normalize.go cmd/extract/normalize_test.go cmd/extract/validate.go cmd/extract/validate_test.go && git commit -m "feat: normalize and validate ontology records"`

### Task 6: Adaptive extraction and multimodal recovery

**Files:**
- Create: `cmd/extract/pipeline.go`
- Create: `cmd/extract/pipeline_test.go`

**Interfaces:**
- Consumes: `AIExtractor`, merge, normalization, validation, pages/OCR.
- Produces: `RunExtraction(ctx context.Context, ai AIExtractor, pages []Page, ocr []OCRPage, cfg Config) ExtractionResult`.

- [ ] **Step 1: Write failing orchestration tests with a scripted fake AI**

Assert five-page batches overlap by one page; valid candidates make one text call only; invalid candidates receive one image call containing source and adjacent pages; a still-invalid candidate receives exactly one reconciliation call containing OCR, images, prior candidates, and issues; and a third invalid result becomes a precise failure without another API call.

- [ ] **Step 2: Add accounting tests**

Assert all candidates end in exactly one bucket: accepted, review, or failed. Assert partial `--pages` runs skip the global 39/80 check, while full runs emit completeness failures for wrong page count, wrong accounted total, or missing known cross-page spans.

- [ ] **Step 3: Verify failure**

Run: `go test ./cmd/extract -run 'TestRunExtraction' -v`

Expected: FAIL because `RunExtraction` is undefined.

- [ ] **Step 4: Implement bounded orchestration**

Create overlapping batches, call text extraction, merge/dedup, normalize/validate, and recover only affected candidates. Select image pages from candidate pages plus one adjacent page on each valid side. Re-run normalization/validation after every AI result. Mark accepted uncertain records `NeedsReview=true`; never accept candidates missing defensible core identity, boundaries, description, or legal enum values.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w cmd/extract/pipeline.go cmd/extract/pipeline_test.go && go test ./cmd/extract -run 'TestRunExtraction' -v`

Expected: PASS.

Commit: `git add cmd/extract/pipeline.go cmd/extract/pipeline_test.go && git commit -m "feat: add adaptive extraction recovery"`

### Task 7: Generated non-empty guard and transactional persistence

**Files:**
- Modify: `database/query/queries.sql`
- Regenerate: `database/generated/queries.sql.go`
- Create: `cmd/extract/persist.go`
- Create: `cmd/extract/persist_test.go`

**Interfaces:**
- Consumes: `NormalizedCandidate`, `pgxpool.Pool`, generated insert methods.
- Produces: generated `CountMagicItems(context.Context) (int64, error)`, `EnsureEmptyCatalog(context.Context, *generated.Queries) error`, and `PersistCandidate(context.Context, *pgxpool.Pool, NormalizedCandidate) (pgtype.UUID, error)`.

- [ ] **Step 1: Add the generated read query**

Append:

```sql
-- name: CountMagicItems :one
select count(*) from magic_item;
```

Run: `(cd database && sqlc generate)`

Expected: `database/generated/queries.sql.go` contains `func (q *Queries) CountMagicItems`.

- [ ] **Step 2: Write failing live-database tests**

Use the existing compose database test pattern to reset/apply the schema. Test successful item/effect/item-wide limitation/effect-linked limitation inserts; test non-empty guard rejection; and force a limitation failure with an invalid effect index to prove the item and all effects roll back. All verification SQL stays in tests only.

- [ ] **Step 3: Verify failure**

Run: `go test ./cmd/extract -run 'Test(PersistCandidate|EnsureEmptyCatalog)' -v`

Expected: FAIL because persistence functions are undefined.

- [ ] **Step 4: Implement one transaction per item**

Begin with `pool.Begin`, create `generated.New(tx)`, call `InsertMagicItem`, collect returned effect UUIDs in slice order, map nullable limitation indexes to UUID pointers, insert limitations, then commit. Defer rollback. Wrap errors with item name and stage; never construct SQL strings.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w cmd/extract/persist.go cmd/extract/persist_test.go && go test ./cmd/extract -run 'Test(PersistCandidate|EnsureEmptyCatalog)' -v`

Expected: PASS against running compose PostgreSQL.

Commit: `git add database/query/queries.sql database/generated/queries.sql.go cmd/extract/persist.go cmd/extract/persist_test.go && git commit -m "feat: persist extracted records transactionally"`

### Task 8: CLI, resume behavior, reports, and end-to-end orchestration

**Files:**
- Modify: `cmd/extract/main.go`
- Create: `cmd/extract/report.go`
- Create: `cmd/extract/main_test.go`
- Create: `cmd/extract/report_test.go`

**Interfaces:**
- Consumes: all previous tasks.
- Produces: `parseConfig(args []string, getenv func(string) string) (Config, error)`, `run(ctx context.Context, cfg Config, deps Dependencies) error`, and final `RunReport` JSON.

- [ ] **Step 1: Write failing CLI tests**

Test defaults, `--dry-run`, `--pages 1-5`, `--resume-run ID`, conflicting flags, invalid batch/overlap, missing API key, and model env overrides. Assert dry-run dependencies never call DB connect/apply/persist.

- [ ] **Step 2: Write failing report/resume tests**

Assert `ocr.json`, `raw_candidates.json`, `normalized.json`, `review.json`, and `report.json` are written; report fields include page/candidate/normalized/retry/review/failure/insert/API-call counts. Resume must load valid OCR/raw artifacts, skip completed external calls, reject incompatible selected pages, and state that DB-level resume is unsupported.

- [ ] **Step 3: Verify failure**

Run: `go test ./cmd/extract -run 'Test(ParseConfig|RunDryRun|Report|Resume)' -v`

Expected: FAIL because orchestration/report functions are undefined.

- [ ] **Step 4: Replace scaffold with dependency-injected orchestration**

Order operations as: parse config → validate binaries/files → create/load run → render/OCR → AI pipeline → write pre-DB artifacts → if dry-run finish → connect/apply schema only for fresh empty DB → non-empty guard → persist accepted records independently → write final report. Continue after one item transaction fails and record it; return non-zero when any candidate failed or full-run completeness fails.

- [ ] **Step 5: Verify and commit**

Run: `gofmt -w cmd/extract/main.go cmd/extract/report.go cmd/extract/main_test.go cmd/extract/report_test.go && go test ./cmd/extract -run 'Test(ParseConfig|RunDryRun|Report|Resume)' -v`

Expected: PASS.

Commit: `git add cmd/extract/main.go cmd/extract/report.go cmd/extract/main_test.go cmd/extract/report_test.go && git commit -m "feat: wire extraction CLI and reporting"`

### Task 9: Documentation and full verification

**Files:**
- Modify: `SETUP.md`
- Modify only if tests reveal contract drift: files created in Tasks 1-8.

**Interfaces:**
- Produces: reproducible evaluator instructions and verified extraction evidence.

- [ ] **Step 1: Document prerequisites and cost controls**

Add installation checks for `pdftoppm` and `tesseract`, required `OPENAI_API_KEY`, optional text/vision model envs, default adaptive model roles, `--dry-run`, `--pages`, `--resume-run`, artifact locations, fresh-database requirement, and the exact full-run command. Explain that image retries are targeted to avoid paying vision cost for every page.

- [ ] **Step 2: Run static and unit verification**

Run: `gofmt -w cmd/extract/*.go && go vet ./... && go test ./...`

Expected: all commands exit 0.

- [ ] **Step 3: Run a bounded live smoke test before spending on the full PDF**

Run: `go run ./cmd/extract --dry-run --pages 7-9`

Expected: artifacts are created, Exo-Armor is represented across pages 7-9, no database writes occur, and API call counts remain within configured bounds.

- [ ] **Step 4: Run the full dry-run quality gate**

Run: `go run ./cmd/extract --dry-run`

Expected: 39 pages, exactly 80 accounted records, all four known cross-page records have complete spans, zero unresolved core failures, and `report.json` lists API/retry/review counts.

- [ ] **Step 5: Reset and verify database persistence**

Read the run ID printed by Step 4 (for example `20260722T120000Z`), then run: `docker compose down -v && docker compose up -d && go run ./cmd/extract --resume-run 20260722T120000Z && go test ./...`

Expected: 80 items are inserted or every non-inserted item has a precise reported persistence failure; for assignment completion, resolve failures and repeat on a fresh database until 80 insertions succeed and all tests pass.

- [ ] **Step 6: Inspect scope and commit documentation/final refinements**

Run: `git status --short && git diff --check && git diff --stat`

Expected: only extraction pipeline, generated query, and `SETUP.md` changes are present; `.agents/` and `skills-lock.json` are unstaged.

Commit: `git add SETUP.md cmd/extract database/query/queries.sql database/generated/queries.sql.go && git commit -m "docs: document extraction pipeline workflow"`

## Final acceptance checklist

- [ ] `go vet ./...` and `go test ./...` pass.
- [ ] No test except explicit persistence integration tests requires network or an AI API.
- [ ] Full report shows 39 rendered pages and 80 accounted/inserted items.
- [ ] Exo-Armor, Ring of Elven Lords, War Drum of the Horde, and Amulet of Encasement retain their complete page spans.
- [ ] Invalid OCR candidates use targeted images and at most one reconciliation request.
- [ ] Every accepted record uses canonical generated enum types and preserves raw descriptions.
- [ ] Every application write goes through generated sqlc methods inside a per-item transaction.
- [ ] A non-empty catalog is rejected and dry-run performs no DB operation.
- [ ] Review and failure artifacts contain explicit reasons without API keys or fabricated values.
