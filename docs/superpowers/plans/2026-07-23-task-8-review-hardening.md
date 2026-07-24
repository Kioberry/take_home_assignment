# Task 8 Review Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Harden the Task 8 extraction CLI so injected dependencies fail before side effects, resume artifacts are identity-consistent, retry counts are accurate, and post-artifact database failures produce terminal reports.

**Architecture:** Keep `run` as the orchestration boundary. Validate every injected function before invoking any dependency, validate the complete persisted artifact graph with `CandidateKey` before connecting to the database, and centralize terminal report failure recording. Add logical API request accounting to the extraction result so report retry counts are computed as a non-negative difference.

**Tech Stack:** Go, standard library JSON/SHA-256 helpers, pgxpool dependency injection, Go tests, gofmt, go vet.

## Global Constraints

- Do not redo Tasks 1–7.
- Do not touch `.agents/skills-lock` or unrelated files.
- Use strict TDD: each regression test must fail before its production fix.
- Redact API keys in terminal report errors.
- Verify focused/full `cmd/extract` tests, `go vet`, and `git diff --check`.

### Task 1: Fail-fast dependency validation

**Files:**
- Modify: `cmd/extract/main.go`
- Test: `cmd/extract/main_test.go`

- [ ] Write a test with one nil injected dependency and counters on validation, artifact, render/OCR/extract, and database seams; assert no counter runs and the error names the missing dependency.
- [ ] Run the focused test and observe failure because current validation reaches earlier seams or reports a later-path error.
- [ ] Add one deterministic validator that checks all required `Dependencies` fields before calling any dependency; keep `main` passing `defaultDependencies()`.
- [ ] Run the focused test and then the existing dependency tests.

### Task 2: Resume candidate identity graph validation

**Files:**
- Modify: `cmd/extract/report.go`, `cmd/extract/model.go`, `cmd/extract/pipeline.go`
- Test: `cmd/extract/main_test.go`, `cmd/extract/report_test.go`

- [ ] Write a corrupt/stale `review.json` fixture test where the review candidate is uncounted or has a different `CandidateKey`; assert failure before `Connect`.
- [ ] Run the focused test and observe current loader accepting/replacing normalized review candidates.
- [ ] Validate report counts plus unique CandidateKeys and exact set membership across raw, normalized, review candidates, extraction failures, and report failures; classify normalized candidates from the validated review set only.
- [ ] Persist a candidate key on report failures so report identity checks are exact, then run the focused resume tests.

### Task 3: Retry count accounting

**Files:**
- Modify: `cmd/extract/model.go`, `cmd/extract/pipeline.go`, `cmd/extract/report.go`
- Test: `cmd/extract/pipeline_test.go`, `cmd/extract/report_test.go`

- [ ] Write tests asserting logical request count and `RunReport.RetryCount = max(APICalls-LogicalRequests, 0)`, including a non-negative clamp.
- [ ] Run focused tests and observe missing fields/incorrect count.
- [ ] Count one logical request for every extraction/recovery/reconciliation invocation, carry it through resume artifacts, and compute/write the report retry count.
- [ ] Run focused extraction/report tests.

### Task 4: Terminal report after database lifecycle failures

**Files:**
- Modify: `cmd/extract/main.go`, `cmd/extract/report.go`
- Test: `cmd/extract/main_test.go`

- [ ] Write a table-driven test for connect, schema, and empty-guard failures; assert returned error is redacted and report has the terminal stage/message and nonzero failure count.
- [ ] Run the focused test and observe missing/unchanged successful report.
- [ ] Centralize terminal failure append, redaction, failure-count update, and report write before each lifecycle return.
- [ ] Run the focused test and all existing persistence/report tests.

### Task 5: Verification and evidence

**Files:**
- Modify: `.superpowers/sdd/task-8-report.md`
- Add: `docs/superpowers/plans/2026-07-23-task-8-review-hardening.md`

- [ ] Run gofmt.
- [ ] Run focused `cmd/extract` tests, full `cmd/extract` tests, `go vet ./...`, and `git diff --check`.
- [ ] Append complete RED/GREEN commands, outcomes, changed behavior, SHA, and concerns to the Task 8 report.
- [ ] Stage only the Task 8 hardening files and create exactly one commit with message `fix: harden CLI resume and failure reporting`.
