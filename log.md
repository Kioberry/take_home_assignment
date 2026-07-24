# Engineering Log

This log records the reasoning and verification evidence for Reznar's Arcane
Oddities. Historical timestamps are the Git author dates of the cited commits.
Where a decision is documented without a corresponding implementation commit,
the entry uses the document date only. Fresh verification entries record the
actual local time, command, exit status, and artifact run ID.

## Historical record

### 2026-07-22T00:18:23+08:00 — Initial ontology design

- Decision: model a catalog around MagicItem, Effect, and Limitation. An effect
  belongs to one item; a limitation belongs to one item and can optionally
  attach to one effect.
- Reasoning: this graph captures the assignment's requested item usage, broad
  magical effects, and usage constraints while remaining traversable in
  PostgreSQL.
- Evidence: commit 6d8bde9 (docs: define initial magic item ontology) and
  docs/superpowers/specs/2026-07-22-magic-item-ontology-design.md.

### 2026-07-22T20:25:04+08:00 — Ontology refined from complete PDF audit

- Decision: keep five source item types, seven wear slots, three broad effect
  categories, varies rarity, and consumed usage mode. Keep item names
  non-unique. Do not add creature/environment target entities, detailed effect
  kinds, gold price, source-page, or durable source-record identity.
- Reasoning: the full 39-page audit found 80 records but did not establish a
  stable vocabulary or relationship semantics for the rejected concepts. Those
  details remain in source-grounded descriptions instead of becoming misleading
  database structure.
- Evidence: commit 7e2734b (docs: revise ontology from complete PDF audit) and
  the decision table in docs/superpowers/specs/2026-07-22-magic-item-ontology-design.md.

### 2026-07-22T21:39:15+08:00 — AI extraction pipeline designed

- Decision: use local PDF rendering and OCR as source input, an AI provider as
  the semantic extractor, deterministic Go normalization/validation, targeted
  image recovery, and generated sqlc methods for per-item persistence.
- Reasoning: this separates uncertain interpretation of the scanned source from
  canonical vocabulary enforcement and transactionally safe writes. It also
  bounds expensive image requests and records review/failure evidence rather
  than fabricating missing fields.
- Evidence: commit 023b584 (docs: design AI extraction pipeline) and
  docs/superpowers/specs/2026-07-22-extraction-pipeline-design.md.

### 2026-07-22T21:44:21+08:00 — Declarative ontology implemented

- Decision: provision the ontology in dependency order and generate typed Go
  from named SQL queries.
- Implemented controls: closed PostgreSQL enums; nonblank source text; wear
  slot and attunement consistency; cascading item ownership; and a composite
  foreign key preventing a limitation from referring to an effect on a
  different item.
- Evidence: commit 7d6ec8c (finished ontology building and designing for
  extraction pipeline); docs/superpowers/plans/2026-07-22-ontology-sql.md;
  database/schema; and database/ontology_test.go.
- Verification status: this log does not infer a current pass from the commit;
  fresh static and database verification is scheduled in Task 9.

### 2026-07-22T21:57:00+08:00 — Extraction implementation plan recorded

- Decision: implement the pipeline in nine independently testable tasks:
  contracts/artifacts, render/OCR, provider contract, merge/deduplication,
  normalization/validation, adaptive recovery, persistence, CLI/reporting, and
  final verification.
- Evidence: commit 3d1a559 and
  docs/superpowers/plans/2026-07-22-extraction-pipeline.md.

### 2026-07-22T22:01:49+08:00 to 2026-07-22T23:17:23+08:00 — Extraction foundations implemented

- Implemented: atomic artifacts and strict resume decoding (041f0fbc,
  b387ce2); selected-page rendering and OCR command boundaries (3da6c35,
  70dc72a); strict OpenAI Responses structured output and response validation
  (c724bfc, 16d7876, 7978c1e); deterministic continuation merge and
  deduplication (ae102a8, b2d66d7); explicit canonical normalization and
  structural validation (7f525ed).
- Reasoning: isolate the provider boundary, preserve original descriptions,
  avoid guessed enum values, and make source-page/cross-page behavior
  deterministic and unit-testable.
- Evidence: the cited commits and the corresponding Task 1–5 sections in
  docs/superpowers/plans/2026-07-22-extraction-pipeline.md.
- Verification status: focused tests were specified and added with each task;
  a fresh suite run is still pending in this verification cycle.

### 2026-07-22T23:31:45+08:00 to 2026-07-23T00:14:46+08:00 — Recovery, persistence, and test isolation implemented

- Implemented: bounded image recovery/reconciliation and completeness
  accounting (ffa8e3c, c9cf9e6, d60faa7, 312e4ba); per-item generated sqlc
  transactions and non-empty catalog guard (4d6ed6a); isolation for persistence
  database schemas (a7f23d4).
- Reasoning: a problematic candidate is retried only with relevant page images;
  unresolved records become explicit failures. A single item's write failure
  rolls back that item without blocking later records.
- Evidence: the cited commits; cmd/extract/pipeline.go; cmd/extract/persist.go;
  and their tests.
- Verification status: the Task 9 live gates will establish current 39-page,
  80-record, and persistence evidence.

### 2026-07-23T00:37:37+08:00 to 2026-07-23T01:16:31+08:00 — CLI/reporting and resume hardening implemented

- Implemented: CLI/environment configuration, dependency-injected orchestration,
  dry-run behavior, artifact resume, pre-database reports, independent
  persistence continuation, and terminal database-lifecycle reporting
  (87d0e7d, 83676ad, babab34, e545f05).
- Hardening decisions: validate all injected dependencies before side effects;
  validate the complete candidate identity graph before connecting; compute
  retries from actual attempts; redact API keys in durable outputs.
- Evidence: the cited commits;
  docs/superpowers/plans/2026-07-23-task-8-review-hardening.md; and
  .superpowers/sdd/task-8-report.md.
- Historical verification evidence: the Task 8 report records focused tests,
  a full cmd/extract test suite, go vet ./..., and git diff --check as passing
  at that stage. It also explicitly records that no live PDF/binary/OpenAI
  smoke test was run.

### 2026-07-23T01:16:17+08:00 — Operational workflow documented

- Decision: document local prerequisites, model configuration, dry-run,
  full-run, artifact-resume, fresh-database, and 39-page/80-record quality
  gate behavior.
- Evidence: commit e689b0e and SETUP.md.

## Fresh verification record

No command has been rerun in the current Task 9 verification cycle yet. Each
subsequent entry will include the local timestamp, exact command, exit code,
concise result, artifact run ID when applicable, and status of Passed, Failed,
or Blocked.

### 2026-07-23T16:57:38.1585665+08:00 — Static and unit verification attempt

- Commands: semantic gofmt comparison against temporary copies of all 36 tracked
  Go files; `go vet ./...`; `go test ./... -count=1`; and `git diff --check`.
- Format result: Passed. The repository has `core.autocrlf=true`, so direct
  `gofmt -d` reports CRLF-to-LF-only diffs for every Go file. Comparing the
  normalized source with gofmt output found no semantic format changes.
- Vet result: Passed when rerun outside the restricted sandbox; the initial
  sandbox attempt could not access the local Go build cache.
- Test result: Failed. Artifact replacement cannot rename an existing open
  file on Windows (`Access is denied`), and Windows reports directory mode
  `0777` rather than the Unix-only expectation `0755`. Database integration
  tests also failed because PostgreSQL was not listening on localhost:5433.
- Docker status: Blocked. `docker compose ps` could not reach the Docker Desktop
  Linux engine named pipe, so PostgreSQL cannot be started until Docker Desktop
  is running.
- Next action: treat the artifact failures as a cross-platform defect requiring
  a separate fix plan; do not run the paid PDF smoke test while the static/unit
  gate is failed and Docker is unavailable.

### 2026-07-23T17:14:26.7120280+08:00 — Portable unit-test subset

- Command: `go test ./cmd/extract -count=1 -skip 'TestArtifact|TestPersistCandidate|TestEnsureEmptyCatalog'`; followed by `go test ./cmd/verify ./db ./database/generated ./stormland/generated -count=1`; then `git diff --check`.
- Result: Passed. The cmd/extract non-artifact/non-database test subset passed in 2.145 seconds. Packages without tests built successfully, and the whitespace check passed.
- Scope: this confirms the provider contract, PDF/OCR command boundaries, merge/deduplication, normalization/validation, adaptive recovery, and dependency-injected CLI/report paths. It does not replace the blocked artifact or database integration gates.

### 2026-07-23 — Claude provider migration and live-verification status

- Local configuration: `.env` is Git-ignored and provides Anthropic model and credential configuration. The extractor's default provider was migrated from the OpenAI Responses API to the Anthropic Messages API with tool-use structured output.
- Local verification: focused Claude-provider and `.env` tests, `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available.
- Live verification: **Not verified / blocked.** Pages 7–9 dry-run attempts returned a model-not-found response for the configured model. The owner reported that usable paid API access for both providers is not currently available; do not make further provider calls until credentials, billing, and an available model are confirmed.
- Evidence limit: no successful paid extraction, image-recovery request, full 39-page/80-record dry-run, or fresh-database persistence run has been performed. Local `tmp/extraction/` artifacts are non-authoritative failed-attempt evidence and are not committed.

### 2026-07-24 — Default provider restored to OpenAI

- Decision: restore the default extraction provider to OpenAI Responses API after the owner configured a local `OPENAI_API_KEY` and selected `gpt-5.6-luna` for routine OCR batches plus `gpt-5.6-terra` for targeted image recovery.
- Reasoning: the OpenAI provider already has strict structured-output, retry, image-input, and regression coverage. The prior Claude path returned a model-not-found response and remains historical, not the active runtime route.
- Verification status: configuration and local provider tests are rerun after this change. No live OpenAI extraction is claimed until the bounded pages 7-9 dry-run completes with the owner's explicit document-sharing authorization.

### 2026-07-24 — OpenAI bounded smoke attempt

- Command: `go run ./cmd/extract --dry-run --pages 7-9` with the owner's explicit authorization to send those pages' OCR content to OpenAI.
- Result: **Interrupted / unverified.** The process produced no response or pre-database artifacts after approximately four minutes and was interrupted locally to stop the waiting request. Run directory `20260724T001950.744964000Z` exists but contains no artifacts or report.
- Evidence limit: this attempt does not establish a provider, extraction, or catalog-quality failure. Before another paid attempt, add a bounded HTTP timeout and record a terminal diagnostic outcome.

### 2026-07-24 — Project progress-recording rule

- Completed: added project-level `AGENT.md` with the instruction that every completed step is recorded in this engineering log.

### 2026-07-24 — OpenAI timeout and HTTP diagnostics

- Diagnosis: the configured OpenAI key and network path were verified with `GET /v1/models` (HTTP 200 in 1.232 seconds); `gpt-5.6-luna` and `gpt-5.6-terra` were available to the key. The interrupted smoke attempt was therefore not a key or basic connectivity failure.
- Completed: production OpenAI requests now use a dedicated client with a 90-second total timeout. Transport errors include elapsed time, and non-2xx responses include HTTP status, elapsed time, and a safely redacted, truncated server response detail.
- RED evidence: focused timeout tests initially failed because `newOpenAIHTTPClient` and `defaultOpenAIRequestTimeout` did not exist.
- GREEN evidence: `go test ./cmd/extract -run 'Test(OpenAIExtractorDoesNotRetryPermanentHTTPError|NewOpenAIHTTPClientUsesBoundedTimeout|OpenAIExtractorReportsTransportTimeoutWithDuration)' -count=1` passed.
- Full verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available.

### 2026-07-24 — OpenAI bounded smoke test, pages 7-9

- Command: `go run ./cmd/extract --dry-run --pages 7-9` with explicit authorization to send those pages' OCR content to OpenAI.
- Run ID: `20260724T023331.394467000Z`.
- Result: **Failed with extraction-quality evidence, not timeout or key failure.** The run completed one text extraction and four targeted image-recovery calls (`api_calls=5`) within the configured request timeout. It did not connect to or write PostgreSQL.
- Observed outcome: four candidates were extracted, but none normalized or entered review. The model emitted descriptive phrases in closed enum fields (`usage_mode`, `wear_slot`, and `effect_category`) instead of the required canonical values. Each targeted recovery also returned multiple candidates where the recovery contract requires exactly one candidate.
- Scope: Exo-Armor correctly appeared as a pages 7-9 candidate, so the cross-page merge path was exercised; it still failed normalization/recovery because its structured enum values were invalid.
- Next action: treat this as a provider-prompt/response-contract defect. Create a separate fix plan and do not claim the smoke gate passed.

### 2026-07-24 — Extraction prompt contract hardening

- Root cause: the prompt asked for source-grounded extraction but did not state the closed ontology vocabulary or the recovery response cardinality. The JSON Schema constrained these fields to strings but could not prevent semantic labels such as `movement`, `helm`, or cooldown text.
- Completed: the system prompt now enumerates allowed source type, rarity, usage mode, wear slot, and effect-category values; explains the armor/helm/cloak/ring mappings; prohibits ability/cooldown text in `usage_mode_raw`; and defines primary-purpose effect classification.
- Completed: recovery and reconciliation request text now names the target candidate and requires exactly one corrected replacement, prohibiting page-neighbor or additional item responses.
- RED evidence: the new prompt-contract tests failed because the original prompt omitted the closed vocabulary and recovery-target instructions.
- GREEN evidence: `go test ./cmd/extract -run 'Test(ExtractionSystemPromptDefinesClosedOntologyVocabulary|RequestTextMakesRecoveryTargetAndCardinalityExplicit|OpenAIExtractor)' -count=1` passed.
- Full verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available.

### 2026-07-24 — OpenAI bounded smoke test passed, pages 7-9

- Command: `go run ./cmd/extract --dry-run --pages 7-9` with explicit authorization to send those pages' OCR content to OpenAI.
- Run ID: `20260724T030712.308626000Z`.
- Result: **Passed.** The dry run completed one OpenAI text call, produced four candidates and four normalized records, with zero failures and zero database writes. No image recovery or reconciliation call was needed.
- Evidence: Armor of Retribution and Eagle Eye Helm were accepted. Exo-Armor correctly spans pages 7-9 and was retained with `needs_review=true` for OCR/source ambiguities. Helmofillomen was retained with `needs_review=true` for name/spelling uncertainty. Both review decisions preserve source evidence instead of fabricating missing facts.

### 2026-07-24 — CLI review summary

- Completed: successful extraction runs now print a concise stdout review summary whenever candidates require human review. Each line includes the candidate name, source pages, review reasons, and the durable `review.json` path, so a headless run can be triaged without opening artifacts manually.
- RED evidence: `TestFormatReviewSummary*` initially failed because no formatter existed.
- GREEN evidence: the focused formatter test passed, followed by `go vet ./...`, `go test ./... -count=1`, and `git diff --check` on macOS with local PostgreSQL available.

### 2026-07-24 — Full OpenAI dry run

- Command: `go run ./cmd/extract --dry-run` with the owner's authorization to send the full PDF OCR content to OpenAI. It did not connect to or write PostgreSQL.
- Run ID: `20260724T031911.919212000Z`.
- Extraction result: 39 rendered pages, 94 normalized candidates, 25 review candidates, zero extraction failures, ten text calls, eleven targeted image-recovery calls, and no request retries.
- Gate result: **failed / not eligible for persistence.** The completeness gate expected exactly 80 accounted records and reported `unexpected_accounted_candidate_count`: 94 candidates. The run correctly exited nonzero even though normalization itself had no failures.
- Evidence: `tmp/extraction/20260724T031911.919212000Z/report.json` and `review.json`. Four cross-page records cannot yet be claimed as a complete gate pass; the duplicate/overlap accounting defect must be diagnosed and fixed before a new paid full run or database replay.

### 2026-07-24 — Full-run count discrepancy root cause

- Diagnosis: the 94 reported candidates contain 14 duplicate overlap extractions, not 14 additional PDF items. Every duplicate is on a configured batch-boundary page (5, 9, 17, 25, 33, or 37); twelve duplicate pairs have identical names and two differ only by casing (`DANTHAG'S RAZOR`/`Danthag's Razor` and `DARKSTAR MACE`/`Darkstar Mace`).
- Evidence: case-insensitive grouping of `normalized.json` yields exactly 80 unique source names, matching the visual PDF audit. The count gate is failing because `CandidateKey` includes a raw-description hash, so independently generated descriptions for the same item in overlapping batches do not match and are retained as distinct candidates.
- Follow-up: replace the current generic review-only stdout output with an anomaly summary that groups duplicate candidates by normalized name and source pages, separately lists missing required spans and extraction failures, and points to their evidence. Do not rerun the paid full extraction until the merge identity and anomaly reporting are fixed and verified.

### 2026-07-24 — Overlap-deduplication design

- Decision: no prompt change is needed for the 94-versus-80 discrepancy. The model's differing but source-grounded descriptions across overlapping batches are expected; the local merge identity must tolerate them.
- Design: merge candidates with the same normalized name when their source pages overlap or touch, retain non-touching same-named records separately, and preserve scalar disagreements as reviewable merge conflicts. Replace verbose review stdout with an anomaly-first summary plus artifact paths.
- Scope: implementation and verification use the existing local run artifacts and unit tests; no new API call, database write, or database reset is authorized.

### 2026-07-24 — Overlap-deduplication implementation

- Completed: `MergeCandidates` now merges candidates with the same normalized name when their source pages touch, even when model wording differs or the name casing differs. Non-touching same-named entries remain separate.
- Completed: description merging retains the more complete source text when one alternate response already contains the other, preventing duplicated sentence fragments.
- RED evidence: same-page alternate-description and case-only-name tests initially produced two candidates.
- GREEN evidence: the focused overlap, non-touching-name, cross-page, and scalar-conflict tests passed after the merge change.

### 2026-07-24 — Anomaly-first CLI summary

- Completed: CLI output now puts completeness issues and extraction failures under `Extraction anomalies` before any human-review information. Ordinary OCR/source review candidates are summarized only as a count with a `review.json` path; their per-item reasons no longer flood stdout.
- Completed: every run also prints its `report.json` path, so the terminal points to the durable machine-readable evidence.
- RED evidence: `TestFormatRunSummaryPrioritizesAnomaliesOverOrdinaryReview` initially failed because the formatter did not exist.
- GREEN evidence: focused run-summary and previous review-summary tests passed after wiring the new formatter into `run`.

### 2026-07-24 — Offline overlap-fix verification

- Full verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available. The sandbox-only run was unable to bind `httptest` or access `localhost:5433`; the host run supplied the valid result.
- Saved-artifact evidence: `tmp/extraction/20260724T031911.919212000Z/normalized.json` contains 94 candidates, 80 case-insensitive normalized-name groups, and 14 duplicate groups. This confirms the recorded source inventory count and the overlap-duplicate diagnosis without making another API call.
- Scope limit: the old artifacts are immutable evidence and do not retroactively rerun the corrected merge code. The next full API dry run remains required to produce a fresh 80-candidate pipeline report before any database persistence.

### 2026-07-24 — Full OpenAI dry run after overlap fix

- Command: `go run ./cmd/extract --dry-run` with the owner's authorization to submit the full PDF OCR content to OpenAI. It did not connect to or write PostgreSQL.
- Run ID: `20260724T055029.102717000Z`.
- Result: **failed / not eligible for persistence.** The run rendered 39 pages and accounted for 82 candidates: 80 normalized, 11 marked for manual review, and two failures. It made ten text requests and six image-recovery requests, with no request retries.
- Remaining duplicate source identities: page 5 contains `QUATERMASTER'S CHEST` and `Quartermaster's Chest`; page 9 contains `Helm of Ill Omen` and `Helmofillomen`. They are alternate OCR/title renderings of two source entries, but do not share the same normalized name and so were not merged.
- Recovery failure evidence: `Exo-Armor` and `AXE OF ENEMY ATTUNEMENT` had recoverable `merge_conflict` issues, then each image-recovery request hit the 90-second Responses API timeout. Merge conflicts should become review evidence without requiring image recovery.
- CLI evidence: the anomaly-first summary correctly surfaced the count gate and the two failed candidates ahead of the 11-item review count. It did not include failure issue messages because `RunFailure.Error` is empty for extraction failures; the next formatter change must render `RunFailure.Issues` when no terminal error string exists.

### 2026-07-24 — Merge-conflict review routing

- Completed: candidates whose only issues are local `merge_conflict` records now bypass image recovery and enter manual review with the conflict messages preserved as review reasons.
- RED evidence: the focused routing test observed two AI calls because a merge conflict triggered image recovery.
- GREEN evidence: the same test now observes only the original text extraction and one review candidate; the existing invalid-candidate image-recovery test also remains green.

### 2026-07-24 — Structured review routing contract

- Completed: the extraction-only JSON contract now requires `review_kind`: `none`, `visual_ambiguity`, or `source_ambiguity`. This is not a database or ontology schema change.
- Completed: prompt instructions constrain `visual_ambiguity` to OCR facts that an original page image can resolve, while incomplete/narrative/semantic source evidence is `source_ambiguity` and remains manual review.
- GREEN evidence: OpenAI provider schema tests and the visual/source routing test passed locally with `httptest`; no external API request was made.

### 2026-07-24 — Full OpenAI dry run with structured review routing

- Local verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available. The sandbox-only test run could not bind `httptest` or reach `localhost:5433`; the host run supplied the valid full-suite result.
- Command: `go run ./cmd/extract --dry-run --run-id 20260724T-debug-dryrun` with the owner's authorization to submit the complete PDF to OpenAI. It did not connect to or write PostgreSQL.
- Run ID: `20260724T-debug-dryrun`.
- Result: **passed / eligible for later persistence after human review.** The run rendered 39 pages, produced and normalized exactly 80 candidates, had zero extraction failures, and made ten text plus two image-recovery requests with zero retries. The full report is `tmp/extraction/20260724T-debug-dryrun/report.json`.
- Review routing evidence: five candidates require review. Two are model-classified `source_ambiguity` (`EXO-ARMOR`, `Amulet of Undead Control`); two are locally generated merge conflicts (`WAR PICK OF ARMOR PIERCING`, `UNIVERSAL SCROLL`) and correctly bypassed image recovery; `War Drum of the Horde` has a source-incompleteness reason but the model returned `review_kind=none`, so it remained in review through the existing reason-based guard. The latter is prompt-classification evidence to refine, not a run failure.

### 2026-07-24 — Manual review evidence audit

- Completed: visually audited the five review candidates against the rendered source pages; no API call or database write was made.
- Resolutions: `EXO-ARMOR` is `Armor (plate), artifact (requires attunement)`, gains +4 Strength and +4 Dexterity, and has the good-alignment attunement restriction; `WAR PICK OF ARMOR PIERCING` is a held weapon; `Amulet of Undead Control` is complete and its eight-hour reuse sentence ends with “after its use”; `War Drum of the Horde` has its complete mechanical effects on pages 30–31; `UNIVERSAL SCROLL` requires attunement by a bard, cleric, druid, sorcerer, warlock, or wizard.
- Diagnosis: all five review entries are recoverable from the original PDF. Four are merge/OCR disagreements and the War Drum review reason is a model-classification error. No source detail needs to be invented or accepted as permanently ambiguous.

### 2026-07-24 — Review-kind consistency guard

- Completed: the prompt now requires `review_kind=none` if and only if `review_reasons` is empty. A model response that nevertheless supplies a reason with `none` is deterministically reclassified as `source_ambiguity`, avoiding an accidental image-recovery request while preserving manual-review routing.
- RED/GREEN evidence: `TestNormalizeClassifiesReasonWithNoneKindAsSourceAmbiguity` failed when the raw response retained `none`, then passed after the normalization guard.
- Full verification: `go vet ./...` and `go test ./... -count=1` passed on macOS with local PostgreSQL available.
