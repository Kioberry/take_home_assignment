# Evaluation Evidence and Task 9 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a reproducible, evidence-backed assessment package for the ontology and extraction pipeline, then complete Task 9 verification without overstating unrun results.

**Architecture:** Keep code and evaluation evidence separate. `EVALUATION.md` is the short reviewer entry point; `log.md` is an append-only, traceable decision and verification record. Runtime artifacts remain local under `tmp/extraction/`.

**Tech Stack:** Markdown, Git, Go, sqlc, Docker/PostgreSQL, Poppler, Tesseract, OpenAI-compatible Responses API.

## Global Constraints

- Do not modify ontology or pipeline implementation unless verification exposes a concrete defect; document it and create a separate fix plan first.
- Do not invent timestamps, test results, API calls, item counts, or database inserts.
- Historical log events use the matching Git commit's ISO-8601 author timestamp. Document-only events use their document date and omit a time.
- Do not commit API keys or `tmp/extraction/` artifacts.
- Preserve `.agents/` and `skills-lock.json`.

---

## File map

- Create: `EVALUATION.md` — reviewer entry point: requirement mapping, design rationale, verification evidence, reproduction, and limits.
- Create: `log.md` — chronological decisions, commits, command outcomes, and risks.
- Modify: `SETUP.md` only when live verification proves command/instruction drift.
- Runtime only: `tmp/extraction/<run-id>/`.

### Task 1: Build the traceable engineering log

**Files:**
- Create: `log.md`
- Read-only inputs: Git log; README; SETUP; existing specs, plans, and Task 8 report.

**Produces:** A truthful history whose decisions and pass claims cite an existing SHA or document.

- [ ] **Step 1: Capture commit timestamps and current scope**

Run:

```powershell
git log --format='%H%n%aI%n%s%n%b%n---' --reverse
git status --short
```

Expected: every implementation commit has a usable ISO-8601 author timestamp.

- [ ] **Step 2: Create the log**

Create `log.md` with:
- An opening statement that Git author dates are used for historical timestamps.
- A chronological ontology entry linking its design spec and implementation SHA(s).
- A pipeline entry linking its design spec and Task 1–8 SHA range.
- A Task 8 hardening entry linking `babab34`, `e545f05`, and `.superpowers/sdd/task-8-report.md`.
- A “Fresh verification record” section with one new heading per command run: local timestamp, exact command, exit code, concise result, artifact run ID if any, and pass/fail/blocked status.

Use the actual Git timestamps and exact SHA values found in Step 1. Historical test-pass claims must cite the Task 8 report or captured command output.

- [ ] **Step 3: Check honesty and formatting**

Run:

```powershell
rg -n '\[.*timestamp.*\]|<.*SHA|TBD|TODO' log.md
git diff --check
```

Expected: no template markers and no whitespace error.

- [ ] **Step 4: Commit**

```powershell
git add log.md
git commit -m "docs: add engineering decision log"
```

### Task 2: Build the reviewer entry point

**Files:**
- Create: `EVALUATION.md`
- Consumes: `log.md`, README, SETUP, specs, plans, source SQL, and tests.

**Produces:** A five-minute navigation document for both scored dimensions.

- [ ] **Step 1: Write ontology-design evidence**

Add `## Ontology design` with:
- entity graph: MagicItem to Effects and Limitations; optional Effect-to-Limitation attachment;
- all five closed vocabularies and why they encode stable meanings;
- enforced invariants: nonblank source text, wearing/slot and attunement consistency, cascade deletion, and same-item limitation integrity;
- explicit non-decisions (creature/environment targets, detailed effect kinds, price, source identity) linked to the ontology design evidence table;
- links to schema files, generated queries, and `database/ontology_test.go`.

- [ ] **Step 2: Write pipeline-quality evidence**

Add `## Extraction pipeline quality` with:
- PDF → render/OCR → strict AI extraction → merge/dedupe → normalize/validate → targeted image recovery → per-item sqlc transaction → report;
- bounded retries/reconciliation, no fabricated values, 39-page/80-record and cross-page gates;
- dry-run no-DB guarantee, empty-catalog guard, resume graph validation, terminal reports, and redaction;
- links to test files by subsystem.

- [ ] **Step 3: Write reproducibility, evidence, and limits**

Add:
- `## Reproduce the verification`: exact static, bounded dry-run, full dry-run, reset, and resume commands from SETUP.
- `## Evidence status`: a table with gate, expected evidence, latest actual status, and `log.md` link. Before fresh execution, use “Not rerun in the current verification cycle”.
- `## Known limits`: local binaries/API key/cost requirements, local-only artifacts, and intentionally unsupported durable cross-run idempotency.

- [ ] **Step 4: Check links and scope**

Run:

```powershell
rg -n 'TBD|TODO|coming soon|will be added' EVALUATION.md
git diff --check
git status --short
```

Expected: no placeholders; only intended Markdown files differ.

- [ ] **Step 5: Commit**

```powershell
git add EVALUATION.md
git commit -m "docs: add evaluation evidence guide"
```

### Task 3: Execute Task 9 static/unit gate

**Files:**
- Modify: `log.md` and `EVALUATION.md` only with observed results.

**Produces:** Fresh static/unit evidence or a precise blocker.

- [ ] **Step 1: Capture prerequisites**

Run:

```powershell
go version
sqlc version
docker --version
Get-Command pdftoppm, tesseract -ErrorAction SilentlyContinue | Select-Object Name, Source
```

Expected: tool versions or absence are recorded in the log.

- [ ] **Step 2: Run quality gates**

Run:

```powershell
gofmt -d .
go vet ./...
go test ./... -count=1
git diff --check
```

Expected: no format diff; vet, tests, and diff check exit 0. On a failure, record concise output and mark the matching evidence row Failed; do not start paid live extraction.

- [ ] **Step 3: Record results and commit**

Add command, exit status, output summary, and gate status to `log.md`; link it from `EVALUATION.md`.

```powershell
git add log.md EVALUATION.md
git commit -m "test: record static verification evidence"
```

### Task 4: Execute Task 9 bounded live smoke gate

**Files:**
- Modify: `log.md` and `EVALUATION.md` only with observed results.
- Runtime-only: `tmp/extraction/<run-id>/`.

**Produces:** Local, cost-bounded pages 7–9 evidence.

- [ ] **Step 1: Verify prerequisites without exposing a secret**

Run:

```powershell
docker compose ps
go run ./cmd/verify
if ([string]::IsNullOrWhiteSpace($env:OPENAI_API_KEY)) { throw 'OPENAI_API_KEY is not set' }
Get-Command pdftoppm, tesseract | Select-Object Name, Source
```

Expected: database ready, key present but not printed, both binaries present. Otherwise log Blocked and do not run extraction.

- [ ] **Step 2: Run and inspect the bounded dry-run**

Run:

```powershell
go run ./cmd/extract --dry-run --pages 7-9
Get-Content "tmp/extraction/<run-id>/report.json" -Raw
```

Expected: no DB write, run artifacts, selected pages 7–9, no API key, and an Exo-Armor span. Record the emitted run ID and report summary; do not add artifacts to Git.

- [ ] **Step 3: Record results and commit**

Mark smoke Passed, Failed, or Blocked only from actual output, then:

```powershell
git add log.md EVALUATION.md
git commit -m "test: record extraction smoke evidence"
```

### Task 5: Execute full extraction and persistence gates

**Files:**
- Modify: `log.md`, `EVALUATION.md`, and only if proven necessary `SETUP.md`.
- Runtime-only: artifacts and the dedicated Compose database.

**Produces:** Full 39-page/80-record and fresh-database persistence evidence, or a concrete defect report.

- [ ] **Step 1: Run full dry-run and inspect its report**

Run:

```powershell
go run ./cmd/extract --dry-run
Get-Content "tmp/extraction/<run-id>/report.json" -Raw
Get-Content "tmp/extraction/<run-id>/review.json" -Raw
```

Expected: 39 pages, 80 accounted records, four named cross-page spans, consistent report/review counts, and no secret. If it fails, log it and stop before resetting the database.

- [ ] **Step 2: Reset the dedicated assignment database and persist**

Run:

```powershell
docker compose down -v
docker compose up -d
go run ./cmd/verify
go run ./cmd/extract --resume-run <run-id>
go test ./... -count=1
```

Expected: artifacts are reused, and assignment completion requires 80 inserts with zero unresolved extraction/persistence failures. This reset is limited to the project's local Compose database.

- [ ] **Step 3: Record results, scope-check, and commit**

Update every evidence row from observed output, then run:

```powershell
git diff --check
git status --short
git add EVALUATION.md log.md SETUP.md
git commit -m "docs: record full extraction verification"
```

Expected: no whitespace errors; only intended documentation is staged. If full verification fails, do not make a completion claim: create a separate defect plan.

## Final acceptance checklist

- [ ] Reviewer entry maps ontology and pipeline quality to concrete code, tests, and evidence.
- [ ] Engineering log has verifiable historical timestamps and fresh outcomes.
- [ ] Static, bounded smoke, full dry-run, and persistence gates are each marked from actual results.
- [ ] No secret or local artifact is committed.
