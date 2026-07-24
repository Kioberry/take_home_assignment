# Task 8 Report: CLI, Resume, Reports, and Dependency-Injected Orchestration

## Status

Complete. `cmd/extract` now parses CLI/environment configuration, orchestrates
fresh and artifact-resume runs through injected side-effect boundaries, writes
durable pre-database artifacts and a secret-safe final report, and returns a
non-zero error for candidate or full-run completeness failures.

## Files

- Modified `cmd/extract/main.go`
  - thin `main`, CLI parsing, runtime wiring, and dependency-injected `run`
  - dry-run exits before connect/schema/guard/persist
  - fresh database lifecycle applies schema, runs the non-empty guard, then
    persists accepted and review candidates independently
  - missing injected dependencies fail explicitly instead of silently falling
    back to real services
- Added `cmd/extract/report.go`
  - `RunReport`, failure/review artifacts, pre-DB artifact writes, and resume
    loading/compatibility checks
  - API-key redaction across report and artifact data
- Added `cmd/extract/main_test.go`
  - CLI, dry-run, resume, artifact compatibility, persistence continuation,
    completeness, and dependency-injection coverage using fakes only
- Added `cmd/extract/report_test.go`
  - artifact and report accounting coverage

## RED

The initial Task 8 test command failed before implementation because
`parseConfig`, `run`, `RunReport`, and `Dependencies` were undefined.

Additional RED checks captured resume PDF/page/raw-artifact incompatibility,
review-candidate persistence, API-key leakage in failure artifacts, and the
unsafe real-dependency fallback when an injected DB seam was omitted.

## GREEN

```sh
gofmt -w cmd/extract/main.go cmd/extract/report.go cmd/extract/main_test.go cmd/extract/report_test.go
GOCACHE=/private/tmp/mulholland-go-cache go test ./cmd/extract -run 'Test(ParseConfig|RunDryRun|Report|Resume)' -v
GOCACHE=/private/tmp/mulholland-go-cache go vet ./...
GOCACHE=/private/tmp/mulholland-go-cache go test ./... -count=1
```

All commands passed. The full suite used only the existing local httptest and
isolated PostgreSQL integration boundaries; no external AI API was called.

## SHA

- `87d0e7 feat: wire extraction CLI and reporting`
- `83676ad fix: require injected extraction dependencies`
- `babab34 fix: harden CLI resume and failure reporting`
- `e545f05 fix: close resume artifact graph gap` (final fix)

## Self-review

- `main` is limited to parsing, production dependency construction, and exit
  handling; `run` is directly testable with fakes.
- Dry-run writes `ocr.json`, `raw_candidates.json`, `normalized.json`,
  `review.json`, and `report.json` before returning without any DB operation.
- Resume checks run ID, PDF path, selected pages, and artifact counts, reuses
  saved OCR/normalization results, and explicitly states that database-level
  resume/idempotency is unsupported.
- Schema application and generated non-empty guard happen before per-item
  writes; one persistence error is recorded while later accepted/review items
  continue.
- Reports and artifacts omit `Config.APIKey`; failure and candidate strings are
  redacted before durable output.
- Only Task 8 files were staged. `.agents/` and `skills-lock.json` were not
  modified.

## Concerns

- A live OpenAI/PDF-binary smoke test was intentionally not run: unit coverage
  uses dependency injection and no API key or paid external request was used.
- Non-dry resume remains intentionally a fresh-database replay workflow; an
  existing catalog is rejected rather than treated as idempotent.

## Review hardening addendum (2026-07-23)

### Scope and root-cause evidence

The review target was the isolated `extraction-pipeline` worktree at the
pre-fix `babab34` Task 8 hardening commit. The final graph-gap fix is
`e545f05`. The worktree was clean before edits. The follow-up only checked missing
dependencies at the point where each path needed them: `run` could already
call the injected validator and artifact opener, and the original resume
loader only compared array lengths before replacing normalized review records
with whatever `review.json` contained. Database lifecycle errors returned after
the pre-DB report was written, leaving that report apparently successful.

### RED evidence

The new regression tests were added before the implementation changes:

```sh
GOCACHE=/private/tmp/mulholland-go-cache go test ./cmd/extract -run 'TestRunRejectsIncompleteDependenciesBeforeAnySideEffect|TestResumeRejectsStaleReviewCandidateBeforeDatabaseWork|TestRunWritesTerminalReportForDatabaseLifecycleFailures|TestRunExtractionUsesProviderAttemptMetricsWhenAvailable|TestReportWritesNonNegativeRetryCountFromActualAttempts' -count=1 -v
```

The first RED run failed to compile because `ExtractionResult.LogicalRequests`,
`RunReport.LogicalRequests`, and `RunReport.RetryCount` did not exist. During
the first implementation run, the incomplete-dependency regression reached the
artifact seam because a typed nil function was hidden inside an `any` check;
the lifecycle fixtures also exposed that fail-fast validation had to be
completed before those paths could execute. Those observations led to the
direct nil checks and complete fixtures shown in the final diff. The stale
review test was then exercised against the identity graph before the final
focused GREEN run. No production fix was claimed from the initial failing
outputs.

### GREEN evidence

The focused review suite passed after the minimal fixes:

```text
ok   oddities/cmd/extract  0.734s
```

The broader Task 8-focused suite passed:

```text
env GOCACHE=/private/tmp/mulholland-go-cache go test ./cmd/extract -run 'Test(ParseConfig|RunDryRun|Resume|RunRequiresInjected|RunRejectsIncomplete|RunWritesTerminal|RunExtractionUsesProvider|Report)' -count=1 -v
PASS
ok   oddities/cmd/extract  4.139s
```

The full package suite passed outside the sandbox so its `httptest` servers
could bind local ports:

```text
env GOCACHE=/private/tmp/mulholland-go-cache go test ./cmd/extract -count=1
ok   oddities/cmd/extract  2.239s
```

Static and whitespace checks passed:

```text
env GOCACHE=/private/tmp/mulholland-go-cache go vet ./...
git diff --check
```

The sandbox-only full test attempt failed before assertions because macOS
`httptest.NewServer` could not bind `[::1]:0`; the same full command passed with
the requested controlled escalation. No external AI request was made.

### Implemented protections

- `run` now validates every injected dependency before calling validation,
  filesystem/artifact, binary, extraction, or database seams. `main` continues
  to pass the complete `defaultDependencies()` value explicitly.
- Resume loading now validates report counts and the complete candidate graph.
  Raw, normalized, review, extraction-failure, and report candidate identities
  use `CandidateKey`; duplicates, missing members, stale review records,
  inconsistent `NeedsReview` state, orphan/duplicate report extraction
  failures, missing report failures, and stage mismatches are rejected before
  database connection.
- `ExtractionResult` records logical API requests. `RunReport` persists both
  `logical_requests` and `retry_count`, where retry count is
  `max(api_calls-logical_requests, 0)`. Provider attempt metrics and resume
  reports preserve this accounting.
- Connect, schema, and empty-catalog-guard failures append a redacted terminal
  failure with stage and message, update `failure_count`, and rewrite
  `report.json` before returning. The regression test verifies the report is
  not falsely successful.

### Changed files and boundary check

Changed Task 8 review-hardening files:

- `cmd/extract/main.go`
- `cmd/extract/model.go`
- `cmd/extract/pipeline.go`
- `cmd/extract/report.go`
- `cmd/extract/main_test.go`
- `cmd/extract/pipeline_test.go`
- `cmd/extract/report_test.go`
- `docs/superpowers/plans/2026-07-23-task-8-review-hardening.md`
- this report

`.agents/skills-lock` and all unrelated Task 1–7 files remain untouched.

### Remaining concerns

- No live PDF/binary/OpenAI smoke test was run; all new orchestration coverage
  uses injected seams and the existing local test boundaries.
- Resume still intentionally replays into a fresh empty database; this change
  hardens artifact integrity and failure reporting without adding database
  idempotency.
- The final fix is committed as `e545f05`; no live PDF/binary/OpenAI smoke test
  was run.
