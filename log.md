# Engineering Log

This log records the reasoning and verification evidence for Reznar's Arcane
Oddities. Historical timestamps are Git author dates unless the entry names a
more precise run ID or artifact timestamp. Every new activity uses an actual
local timestamp; when an old entry has no defensible exact time it is marked
`exact time unavailable` rather than guessed. Fresh verification entries record
the command or API endpoint, outcome, artifact/run ID, and database scope.

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

### 2026-07-23T21:51:10+08:00 — Claude provider migration and live-verification status

- Local configuration: `.env` is Git-ignored and provides Anthropic model and credential configuration. The extractor's default provider was migrated from the OpenAI Responses API to the Anthropic Messages API with tool-use structured output.
- Local verification: focused Claude-provider and `.env` tests, `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available.
- Live verification: **Not verified / blocked.** Pages 7–9 dry-run attempts returned a model-not-found response for the configured model. The owner reported that usable paid API access for both providers is not currently available; do not make further provider calls until credentials, billing, and an available model are confirmed.
- Evidence limit: no successful paid extraction, image-recovery request, full 39-page/80-record dry-run, or fresh-database persistence run has been performed. Local `tmp/extraction/` artifacts are non-authoritative failed-attempt evidence and are not committed.

### 2026-07-24T08:05:49+08:00 — Default provider restored to OpenAI

- Decision: restore the default extraction provider to OpenAI Responses API after the owner configured a local `OPENAI_API_KEY` and selected `gpt-5.6-luna` for routine OCR batches plus `gpt-5.6-terra` for targeted image recovery.
- Reasoning: the OpenAI provider already has strict structured-output, retry, image-input, and regression coverage. The prior Claude path returned a model-not-found response and remains historical, not the active runtime route.
- Verification status: configuration and local provider tests are rerun after this change. No live OpenAI extraction is claimed until the bounded pages 7-9 dry-run completes with the owner's explicit document-sharing authorization.

### 2026-07-24T08:19:50+08:00 — OpenAI bounded smoke attempt

- Command: `go run ./cmd/extract --dry-run --pages 7-9` with the owner's explicit authorization to send those pages' OCR content to OpenAI.
- Result: **Interrupted / unverified.** The process produced no response or pre-database artifacts after approximately four minutes and was interrupted locally to stop the waiting request. Run directory `20260724T001950.744964000Z` exists but contains no artifacts or report.
- Evidence limit: this attempt does not establish a provider, extraction, or catalog-quality failure. Before another paid attempt, add a bounded HTTP timeout and record a terminal diagnostic outcome.

### 2026-07-24T08:33:32+08:00 — Project progress-recording rule

- Completed: added project-level `AGENT.md` with the instruction that every completed step is recorded in this engineering log.

### 2026-07-24T08:33:32+08:00 — OpenAI timeout and HTTP diagnostics

- Diagnosis: the configured OpenAI key and network path were verified with `GET /v1/models` (HTTP 200 in 1.232 seconds); `gpt-5.6-luna` and `gpt-5.6-terra` were available to the key. The interrupted smoke attempt was therefore not a key or basic connectivity failure.
- Completed: production OpenAI requests now use a dedicated client with a 90-second total timeout. Transport errors include elapsed time, and non-2xx responses include HTTP status, elapsed time, and a safely redacted, truncated server response detail.
- RED evidence: focused timeout tests initially failed because `newOpenAIHTTPClient` and `defaultOpenAIRequestTimeout` did not exist.
- GREEN evidence: `go test ./cmd/extract -run 'Test(OpenAIExtractorDoesNotRetryPermanentHTTPError|NewOpenAIHTTPClientUsesBoundedTimeout|OpenAIExtractorReportsTransportTimeoutWithDuration)' -count=1` passed.
- Full verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available.

### 2026-07-24T10:33:31+08:00 — OpenAI bounded smoke test, pages 7-9

- Command: `go run ./cmd/extract --dry-run --pages 7-9` with explicit authorization to send those pages' OCR content to OpenAI.
- Run ID: `20260724T023331.394467000Z`.
- Result: **Failed with extraction-quality evidence, not timeout or key failure.** The run completed one text extraction and four targeted image-recovery calls (`api_calls=5`) within the configured request timeout. It did not connect to or write PostgreSQL.
- Observed outcome: four candidates were extracted, but none normalized or entered review. The model emitted descriptive phrases in closed enum fields (`usage_mode`, `wear_slot`, and `effect_category`) instead of the required canonical values. Each targeted recovery also returned multiple candidates where the recovery contract requires exactly one candidate.
- Scope: Exo-Armor correctly appeared as a pages 7-9 candidate, so the cross-page merge path was exercised; it still failed normalization/recovery because its structured enum values were invalid.
- Next action: treat this as a provider-prompt/response-contract defect. Create a separate fix plan and do not claim the smoke gate passed.

### 2026-07-24T10:48:17+08:00 — Extraction prompt contract hardening

- Root cause: the prompt asked for source-grounded extraction but did not state the closed ontology vocabulary or the recovery response cardinality. The JSON Schema constrained these fields to strings but could not prevent semantic labels such as `movement`, `helm`, or cooldown text.
- Completed: the system prompt now enumerates allowed source type, rarity, usage mode, wear slot, and effect-category values; explains the armor/helm/cloak/ring mappings; prohibits ability/cooldown text in `usage_mode_raw`; and defines primary-purpose effect classification.
- Completed: recovery and reconciliation request text now names the target candidate and requires exactly one corrected replacement, prohibiting page-neighbor or additional item responses.
- RED evidence: the new prompt-contract tests failed because the original prompt omitted the closed vocabulary and recovery-target instructions.
- GREEN evidence: `go test ./cmd/extract -run 'Test(ExtractionSystemPromptDefinesClosedOntologyVocabulary|RequestTextMakesRecoveryTargetAndCardinalityExplicit|OpenAIExtractor)' -count=1` passed.
- Full verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available.

### 2026-07-24T11:07:12+08:00 — OpenAI bounded smoke test passed, pages 7-9

- Command: `go run ./cmd/extract --dry-run --pages 7-9` with explicit authorization to send those pages' OCR content to OpenAI.
- Run ID: `20260724T030712.308626000Z`.
- Result: **Passed.** The dry run completed one OpenAI text call, produced four candidates and four normalized records, with zero failures and zero database writes. No image recovery or reconciliation call was needed.
- Evidence: Armor of Retribution and Eagle Eye Helm were accepted. Exo-Armor correctly spans pages 7-9 and was retained with `needs_review=true` for OCR/source ambiguities. Helmofillomen was retained with `needs_review=true` for name/spelling uncertainty. Both review decisions preserve source evidence instead of fabricating missing facts.

### 2026-07-24T11:17:47+08:00 — CLI review summary

- Completed: successful extraction runs now print a concise stdout review summary whenever candidates require human review. Each line includes the candidate name, source pages, review reasons, and the durable `review.json` path, so a headless run can be triaged without opening artifacts manually.
- RED evidence: `TestFormatReviewSummary*` initially failed because no formatter existed.
- GREEN evidence: the focused formatter test passed, followed by `go vet ./...`, `go test ./... -count=1`, and `git diff --check` on macOS with local PostgreSQL available.

### 2026-07-24T11:19:11+08:00 — Full OpenAI dry run

- Command: `go run ./cmd/extract --dry-run` with the owner's authorization to send the full PDF OCR content to OpenAI. It did not connect to or write PostgreSQL.
- Run ID: `20260724T031911.919212000Z`.
- Extraction result: 39 rendered pages, 94 normalized candidates, 25 review candidates, zero extraction failures, ten text calls, eleven targeted image-recovery calls, and no request retries.
- Gate result: **failed / not eligible for persistence.** The completeness gate expected exactly 80 accounted records and reported `unexpected_accounted_candidate_count`: 94 candidates. The run correctly exited nonzero even though normalization itself had no failures.
- Evidence: `tmp/extraction/20260724T031911.919212000Z/report.json` and `review.json`. Four cross-page records cannot yet be claimed as a complete gate pass; the duplicate/overlap accounting defect must be diagnosed and fixed before a new paid full run or database replay.

### 2026-07-24T12:09:28+08:00 — Full-run count discrepancy root cause

- Diagnosis: the 94 reported candidates contain 14 duplicate overlap extractions, not 14 additional PDF items. Every duplicate is on a configured batch-boundary page (5, 9, 17, 25, 33, or 37); twelve duplicate pairs have identical names and two differ only by casing (`DANTHAG'S RAZOR`/`Danthag's Razor` and `DARKSTAR MACE`/`Darkstar Mace`).
- Evidence: case-insensitive grouping of `normalized.json` yields exactly 80 unique source names, matching the visual PDF audit. The count gate is failing because `CandidateKey` includes a raw-description hash, so independently generated descriptions for the same item in overlapping batches do not match and are retained as distinct candidates.
- Follow-up: replace the current generic review-only stdout output with an anomaly summary that groups duplicate candidates by normalized name and source pages, separately lists missing required spans and extraction failures, and points to their evidence. Do not rerun the paid full extraction until the merge identity and anomaly reporting are fixed and verified.

### 2026-07-24T12:21:10+08:00 — Overlap-deduplication design

- Decision: no prompt change is needed for the 94-versus-80 discrepancy. The model's differing but source-grounded descriptions across overlapping batches are expected; the local merge identity must tolerate them.
- Design: merge candidates with the same normalized name when their source pages overlap or touch, retain non-touching same-named records separately, and preserve scalar disagreements as reviewable merge conflicts. Replace verbose review stdout with an anomaly-first summary plus artifact paths.
- Scope: implementation and verification use the existing local run artifacts and unit tests; no new API call, database write, or database reset is authorized.

### 2026-07-24T13:46:06+08:00 — Overlap-deduplication implementation

- Completed: `MergeCandidates` now merges candidates with the same normalized name when their source pages touch, even when model wording differs or the name casing differs. Non-touching same-named entries remain separate.
- Completed: description merging retains the more complete source text when one alternate response already contains the other, preventing duplicated sentence fragments.
- RED evidence: same-page alternate-description and case-only-name tests initially produced two candidates.
- GREEN evidence: the focused overlap, non-touching-name, cross-page, and scalar-conflict tests passed after the merge change.

### 2026-07-24T13:47:04+08:00 — Anomaly-first CLI summary

- Completed: CLI output now puts completeness issues and extraction failures under `Extraction anomalies` before any human-review information. Ordinary OCR/source review candidates are summarized only as a count with a `review.json` path; their per-item reasons no longer flood stdout.
- Completed: every run also prints its `report.json` path, so the terminal points to the durable machine-readable evidence.
- RED evidence: `TestFormatRunSummaryPrioritizesAnomaliesOverOrdinaryReview` initially failed because the formatter did not exist.
- GREEN evidence: focused run-summary and previous review-summary tests passed after wiring the new formatter into `run`.

### 2026-07-24T13:48:20+08:00 — Offline overlap-fix verification

- Full verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available. The sandbox-only run was unable to bind `httptest` or access `localhost:5433`; the host run supplied the valid result.
- Saved-artifact evidence: `tmp/extraction/20260724T031911.919212000Z/normalized.json` contains 94 candidates, 80 case-insensitive normalized-name groups, and 14 duplicate groups. This confirms the recorded source inventory count and the overlap-duplicate diagnosis without making another API call.
- Scope limit: the old artifacts are immutable evidence and do not retroactively rerun the corrected merge code. The next full API dry run remains required to produce a fresh 80-candidate pipeline report before any database persistence.

### 2026-07-24T13:50:29+08:00 — Full OpenAI dry run after overlap fix

- Command: `go run ./cmd/extract --dry-run` with the owner's authorization to submit the full PDF OCR content to OpenAI. It did not connect to or write PostgreSQL.
- Run ID: `20260724T055029.102717000Z`.
- Result: **failed / not eligible for persistence.** The run rendered 39 pages and accounted for 82 candidates: 80 normalized, 11 marked for manual review, and two failures. It made ten text requests and six image-recovery requests, with no request retries.
- Remaining duplicate source identities: page 5 contains `QUATERMASTER'S CHEST` and `Quartermaster's Chest`; page 9 contains `Helm of Ill Omen` and `Helmofillomen`. They are alternate OCR/title renderings of two source entries, but do not share the same normalized name and so were not merged.
- Recovery failure evidence: `Exo-Armor` and `AXE OF ENEMY ATTUNEMENT` had recoverable `merge_conflict` issues, then each image-recovery request hit the 90-second Responses API timeout. Merge conflicts should become review evidence without requiring image recovery.
- CLI evidence: the anomaly-first summary correctly surfaced the count gate and the two failed candidates ahead of the 11-item review count. It did not include failure issue messages because `RunFailure.Error` is empty for extraction failures; the next formatter change must render `RunFailure.Issues` when no terminal error string exists.

### 2026-07-24T17:05:08+08:00 — Merge-conflict review routing

- Completed: candidates whose only issues are local `merge_conflict` records now bypass image recovery and enter manual review with the conflict messages preserved as review reasons.
- RED evidence: the focused routing test observed two AI calls because a merge conflict triggered image recovery.
- GREEN evidence: the same test now observes only the original text extraction and one review candidate; the existing invalid-candidate image-recovery test also remains green.

### 2026-07-24T17:30:01+08:00 — Structured review routing contract

- Completed: the extraction-only JSON contract now requires `review_kind`: `none`, `visual_ambiguity`, or `source_ambiguity`. This is not a database or ontology schema change.
- Completed: prompt instructions constrain `visual_ambiguity` to OCR facts that an original page image can resolve, while incomplete/narrative/semantic source evidence is `source_ambiguity` and remains manual review.
- GREEN evidence: OpenAI provider schema tests and the visual/source routing test passed locally with `httptest`; no external API request was made.

### 2026-07-24T17:57:11+08:00 — Full OpenAI dry run with structured review routing

- Local verification: `go vet ./...`, `go test ./... -count=1`, and `git diff --check` passed on macOS with local PostgreSQL available. The sandbox-only test run could not bind `httptest` or reach `localhost:5433`; the host run supplied the valid full-suite result.
- Command: `go run ./cmd/extract --dry-run --run-id 20260724T-debug-dryrun` with the owner's authorization to submit the complete PDF to OpenAI. It did not connect to or write PostgreSQL.
- Run ID: `20260724T-debug-dryrun`.
- Result: **passed / eligible for later persistence after human review.** The run rendered 39 pages, produced and normalized exactly 80 candidates, had zero extraction failures, and made ten text plus two image-recovery requests with zero retries. The full report is `tmp/extraction/20260724T-debug-dryrun/report.json`.
- Review routing evidence: five candidates require review. Two are model-classified `source_ambiguity` (`EXO-ARMOR`, `Amulet of Undead Control`); two are locally generated merge conflicts (`WAR PICK OF ARMOR PIERCING`, `UNIVERSAL SCROLL`) and correctly bypassed image recovery; `War Drum of the Horde` has a source-incompleteness reason but the model returned `review_kind=none`, so it remained in review through the existing reason-based guard. The latter is prompt-classification evidence to refine, not a run failure.

### 2026-07-24T18:04:12+08:00 — Manual review evidence audit

- Completed: visually audited the five review candidates against the rendered source pages; no API call or database write was made.
- Resolutions: `EXO-ARMOR` is `Armor (plate), artifact (requires attunement)`, gains +4 Strength and +4 Dexterity, and has the good-alignment attunement restriction; `WAR PICK OF ARMOR PIERCING` is a held weapon; `Amulet of Undead Control` is complete and its eight-hour reuse sentence ends with “after its use”; `War Drum of the Horde` has its complete mechanical effects on pages 30–31; `UNIVERSAL SCROLL` requires attunement by a bard, cleric, druid, sorcerer, warlock, or wizard.
- Diagnosis: all five review entries are recoverable from the original PDF. Four are merge/OCR disagreements and the War Drum review reason is a model-classification error. No source detail needs to be invented or accepted as permanently ambiguous.

### 2026-07-24T19:56:11+08:00 — Review-kind consistency guard

- Completed: the prompt now requires `review_kind=none` if and only if `review_reasons` is empty. A model response that nevertheless supplies a reason with `none` is deterministically reclassified as `source_ambiguity`, avoiding an accidental image-recovery request while preserving manual-review routing.
- RED/GREEN evidence: `TestNormalizeClassifiesReasonWithNoneKindAsSourceAmbiguity` failed when the raw response retained `none`, then passed after the normalization guard.
- Full verification: `go vet ./...` and `go test ./... -count=1` passed on macOS with local PostgreSQL available.

### 2026-07-24T21:14:28+08:00 — Recovery event reporting

- Completed: each targeted image-recovery or reconciliation request now appends a compact `recovery_events` entry to the run report. An event records the candidate name and source pages, triggering validation issues, stage (`image` or `reconciliation`), and outcome (`succeeded`, `unresolved`, or `failed`); it does not persist page images or OCR text.
- RED/GREEN evidence: `TestReportIncludesSuccessfulImageRecoveryEvent` initially found no `recovery_events` field, then passed after the recovery pipeline propagated its event data into `RunReport`.
- Full verification: `go vet ./...` and `go test ./... -count=1` passed on macOS with local PostgreSQL available.

### 2026-07-24T21:22:32+08:00 — Targeted live review-routing verification

- Command: `go run ./cmd/extract --dry-run --pages 7-9 --run-id 20260724T-exo-reviewkind` with the owner's authorization to send pages 7–9 to OpenAI. It completed with four normalized candidates, one review, zero failures, one text call, and one image-recovery call. Its report records `HELMOFILLOMEN` with an `image` recovery event, `visual_ambiguity` trigger, and `succeeded` outcome.
- Exo result: `EXO-ARMOR` is correctly normalized as `artifact` and no longer has the invalid `review_kind=none` plus reasons combination. However, the model still labels the readable +4 ability-score increase as `source_ambiguity`; visual audit shows that page 8 contains the value. This is an unresolved prompt-routing false positive, not an artifact or recovery-event failure.
- Command: `go run ./cmd/extract --dry-run --pages 29-31 --run-id 20260724T-wardrum-reviewkind` with the owner's authorization to send pages 29–31 to OpenAI. It completed with one normalized candidate, zero review candidates, zero failures, one text call, and no recovery calls. `War Drum of the Horde` now has complete mechanics, `review_kind=none`, and no review reasons.

### 2026-07-24T21:28:46+08:00 — Exo +4 routing diagnosis

- Root cause: visual inspection of PDF page 8 shows “increases to both your Strength and Dexterity ability scores by 4”, but the corresponding local Tesseract artifact ends at “scores by” and omits the `4`. The first extraction request receives OCR text only, so the model cannot extract the missing number from its initial input.
- Routing defect: the model correctly recognized that its supplied text lacked the value, but incorrectly emitted `source_ambiguity`. The missing value is an OCR-local, visually recoverable numeric gap, so it should be `visual_ambiguity` and trigger one targeted image-recovery request. No source ambiguity or database issue is involved.

### 2026-07-24T21:43:39+08:00 — Generalized visual-routing prompt

- Completed: replaced the narrow title/number vocabulary with a source-of-uncertainty rule. The prompt now routes any OCR or page-layout artifact — including truncated or misrecognized words, symbols, values, qualifiers, conditions, or other fields — to `visual_ambiguity` when the original page can resolve it. `source_ambiguity` is reserved for uncertainty that remains after direct page inspection.
- RED/GREEN evidence: `TestExtractionSystemPromptDefinesClosedOntologyVocabulary` first failed because the prompt lacked the OCR/page-layout criterion, then passed after the prompt update.
- Full verification: `go vet ./...` and `go test ./... -count=1` passed on macOS with local PostgreSQL available. No API call was made after this prompt-only change.

### 2026-07-24T21:51:28+08:00 — Exo visual-recovery verification

- Command: `go run ./cmd/extract --dry-run --pages 7-9 --run-id 20260724T-exo-visual-routing` with the owner's authorization to send pages 7–9 to OpenAI. It completed with four normalized candidates, zero review candidates, zero failures, one text call, and two image-recovery calls.
- Result: the generalized prompt correctly classified `Exo-Armor` as visual ambiguity and the report records an `image` event with `succeeded` outcome. The recovered record has `review_kind=none`, no review reasons, and preserves the source fact that both Strength and Dexterity ability scores increase by 4. The separate Helm of Ill Omen image recovery also succeeded.

### 2026-07-24T22:46:29+08:00 — Formal-run quality gate

- Root cause: the former control flow checked extraction failures and completeness issues only for `--dry-run`. A formal run could create its pre-database artifacts and then connect to PostgreSQL before reporting a failed quality result.
- Completed: immediately after writing the pre-database artifacts and stdout summary, `run` now evaluates `runResultError(report)`. Any extraction failure or completeness issue returns before database connection, schema application, empty-catalog validation, or persistence. A clean formal run continues to persist both accepted records and review records with `needs_review=true`.
- Verification: `TestRunBlocksDatabaseBeforePersistenceWhenQualityGateFails` and `TestRunReportsExtractionFailureWithoutDatabasePhase` passed; `go test ./cmd/extract -count=1`, `gofmt -d cmd/extract/main.go cmd/extract/main_test.go`, and `git diff --check` passed.
- Scope: this was local control-flow verification only. No external API call, full dry run, PostgreSQL connection, or database write was made.

### 2026-07-24T22:51:40+08:00 — Full OpenAI dry run after formal-run gate

- Command: `go run ./cmd/extract --dry-run` with explicit authorization to send the full PDF OCR content to OpenAI. Run ID: `20260724T145140.123149000Z`.
- Result: **failed / not eligible for persistence.** The run rendered 39 pages, accounted for 82 candidates, normalized 74 records, retained 9 review records, and recorded 8 extraction failures. It made 10 text and 8 image requests, with no retries. `inserted_count` remained zero because this was a dry run.
- Failure evidence: every targeted image-recovery request timed out after the configured 90 seconds while awaiting OpenAI response headers. The affected candidates were `QUATERMASTER'S CHEST`, `Quartermaster's Chest`, `HELMOFILLOMEN`, `Helm of Ill Omen`, `DANTHAG'S RAZOR`, `SWORD OF THE YOUXIA`, `Mask of Ill Luck`, and `Pouch of False Coins`.
- Count evidence: two OCR title-variant pairs remained distinct candidates on the same source pages: `QUATERMASTER'S CHEST`/`Quartermaster's Chest` on page 5 and `HELMOFILLOMEN`/`Helm of Ill Omen` on page 9. The exact-name overlap merge does not yet cover these title variants.
- Next action: do not run the formal database write. Diagnose the image-request timeout separately and add a bounded title-variant identity strategy before another paid full run.

### 2026-07-24T23:49:44+08:00 — GLM-OCR page-9 isolated validation

- Purpose: test whether replacing the local Tesseract OCR stage can prevent the known page-9 title-variant duplicate, without rerunning the full pipeline, OpenAI, or PostgreSQL.
- Input: the existing rendered `page-009.png` (about 2.69 MB), whose source heading is `HELM OF ILL OMEN`. The local Tesseract artifact had emitted `HELMOFILLOMEN`.
- SDK diagnosis: the official `glmocr` SDK (0.1.5) read `ZHIPU_API_KEY` correctly but failed before API response with Python OpenSSL `UNEXPECTED_EOF_WHILE_READING`; the environment proxy also fails TLS to the Zhipu domain. A no-key direct `curl` request reached the endpoint and received expected HTTP 401, while direct Python `requests` still failed TLS. The supported HTTP endpoint is therefore usable from this machine via direct `curl`, but the SDK is not.
- Command/result: one direct `curl` request to `POST /api/paas/v4/layout_parsing` with model `glm-ocr`, the page PNG encoded as a data URI, and `ZHIPU_API_KEY` from ignored `.env` returned HTTP 200 in 8.294 seconds. Usage was 2,505 prompt + 563 completion = 3,068 tokens. No OpenAI call or database connection/write occurred.
- OCR evidence: GLM-OCR returned `## HELM OF ILL OMEN`, marked it as `paragraph_title`, and supplied a title bounding box. Visual inspection confirms this exactly matches the source image. This isolates Tesseract title merging as a root cause of the page-9 duplicate; it does not yet prove full-document replacement quality.
- Artifacts: `tmp/extraction/glmocr-smoke/page-009-direct-http.json`, `page-009-glm.md`, and `page-009-comparison.md`.
- Next action: run the same bounded GLM-OCR comparison on the other known title-variant page, page 5 (`QUATERMASTER'S CHEST`/`Quartermaster's Chest`). Only if both cases pass should the Tesseract-to-GLM integration be designed and implemented.

### 2026-07-24T23:53:45+08:00 — GLM-OCR page-5 isolated validation

- Command/result: with explicit authorization, one direct `curl` request sent the existing rendered `page-005.png` to `POST /api/paas/v4/layout_parsing` with model `glm-ocr`. It returned HTTP 200 in 6.717 seconds, using 1,444 prompt + 347 completion = 1,791 tokens. No OpenAI call or database connection/write occurred.
- OCR evidence: the source image reads `QUATERMASTER'S CHEST`. GLM-OCR returned exactly `## QUATERMASTER'S CHEST` and marked it `paragraph_title`. The local Tesseract text preserved the spelling but prefixed the heading with OCR noise (`is \"`); the previous `Quartermaster's Chest` candidate was a downstream title variation rather than the source spelling.
- Conclusion: both known title-variant duplicate samples now pass the GLM-OCR test. The evidence supports replacing the local Tesseract stage with GLM-OCR output, while keeping the current structured extraction, normalization, review, quality gate, and database layers unchanged. This remains two-page evidence, not a completed full-document migration or dry run.
- Artifacts: `tmp/extraction/glmocr-smoke/page-005-direct-http.json`, `page-005-glm.md`, and `page-005-comparison.md`.
- Next action: design the smallest provider adapter that writes GLM-OCR page Markdown into the existing `[]OCRPage` contract, add offline fixtures/tests, then run one fresh full GLM-backed dry run before any formal database run.

### 2026-07-24T23:59:56+08:00 — Log timestamp audit and workflow hardening

- Completed: strengthened project-level `AGENT.md` so each completed activity must immediately add an auditable `log.md` entry with an ISO-8601 local timestamp, command/interface, outcome, artifact/run ID, and database scope.
- Historical repair: entries that had only a date were backfilled from either the cited Git author date or a timestamp-bearing run ID. The two new GLM OCR entries use their response-artifact modification times. No timestamp was invented from memory or conversation order.
- Verification: `git diff --check` passed after the documentation rewrite. Future entries must use this rule before work continues.

### 2026-07-25T00:00:20+08:00 — GLM-OCR full-document run started

- Scope: a separate sequential OCR-only run over the existing 39 rendered PNG pages. Each request uses the verified direct `curl` route to Zhipu `POST /api/paas/v4/layout_parsing` with model `glm-ocr`; responses and Markdown are durable artifacts.
- Boundaries: no OpenAI request, structured extraction, PostgreSQL connection, schema operation, or persistence is part of this run. Per-page errors are retained and do not stop later pages.

### 2026-07-25T00:08:33+08:00 — GLM-OCR full-document run completed

- Result: all 39 unique rendered pages returned HTTP 200 and yielded non-empty Markdown. The combined artifact `ocr.json` has page numbers 1–39 exactly once and is compatible with the pipeline's existing `[]OCRPage` data contract.
- Artifacts: `tmp/extraction/20260724T160020.000000000Z/glm-ocr/manifest.tsv` records per-page start/end times, HTTP metrics, and outcomes; `page-001.md` through `page-039.md` retain Markdown; `ocr.json` is the canonical combined OCR artifact.
- Incident: the execution harness detached the initial sequential loop while it was still running. A recovery loop overlapped it on pages 11–12, producing two successful attempts for each page. The original loop was identified as PID 96360 and terminated before further overlap. No page failed; only the successful first/last artifact is used in `ocr.json`. The duplicate requests are retained in the manifest and must be included in any cost review.
- Verification: `ocr_pages=39`, `unique_success_pages=39`, `failed_attempts=0`, and the combined Markdown is about 156 KB. No OpenAI request or database operation occurred.
- Next action: make one bounded GLM text-model chat-completions probe against the GLM OCR Markdown. The current OpenAI Responses extractor is not directly interchangeable because it uses `/v1/responses` and strict server JSON Schema, while Zhipu documents `/chat/completions` plus `json_object`; do not switch the formal Provider without a dedicated adapter and tests.

### 2026-07-25T00:09:32+08:00 — GLM text-provider probe blocked pending payload authorization

- Prepared local-only input: `text-probe-pages-001-005.md` combines 7,226 bytes of GLM-OCR Markdown for pages 1–5. The intended bounded call is `POST /api/paas/v4/chat/completions`, model `glm-4.6`, `response_format={"type":"json_object"}`, requesting only page/title pairs.
- Boundary: the execution safety control rejected sending this OCR-derived text because authorization so far explicitly covered page images to GLM-OCR, not pages 1–5 Markdown to the separate GLM text endpoint. No HTTP request was sent and no token usage occurred.
- Required before retry: explicit authorization for this exact payload and endpoint. A successful probe would establish only endpoint/key/JSON-mode compatibility, not equivalence with the existing strict-schema OpenAI extractor or approval for a provider replacement.

### 2026-07-25T00:18:25+08:00 — Pre-persistence local verification

- Commands: `docker compose ps`, `go run ./cmd/verify`, `go vet ./...`, and `go test ./... -count=1`.
- Result: Passed. `oddities-postgres-1` was healthy on localhost:5433; the database verifier reported `postgres is ready`; vet completed successfully; and the full Go suite passed (`cmd/extract`, `database`, and `stormland`).
- Scope: no AI-provider request, schema change, or catalog write was performed. The next step is a fresh-database artifact replay from the previously successful 39-page/80-record run.

### 2026-07-25T00:19:00+08:00 — Dedicated database reset for artifact replay

- Command: `docker compose down -v` in the assignment worktree.
- Result: Completed. Docker removed only the `oddities-postgres-1` container, `oddities_pgdata` volume, and `oddities_default` network defined by this project's compose file.
- Scope: this intentionally deleted the locally reproducible assignment database before the README-required fresh-database replay; no API request or catalog extraction was performed.

### 2026-07-25T00:19:28+08:00 — Fresh database ready

- Commands: `docker compose up -d` and `go run ./cmd/verify`.
- Result: Passed. The `oddities` compose network and `oddities_pgdata` volume were recreated, `oddities-postgres-1` started, and the verifier reported `postgres is ready`.
- Scope: the fresh database has not yet received schema objects or catalog records; no API request was made.

### 2026-07-25T00:19:55+08:00 — Successful full-catalog artifact persistence

- Command: `go run ./cmd/extract --resume-run 20260724T-debug-dryrun`.
- Input evidence: run `20260724T-debug-dryrun` had 39 pages, 80 normalized candidates, five review candidates, zero extraction failures, and zero API retries. Resume validation completed before database work.
- Result: Passed. The fresh database received the schema and all accepted plus review candidates; stdout reported five manual-review candidates and durable `review.json` / `report.json` artifact paths. The command exited successfully.
- Scope: this replay made no OCR or AI-provider request. It intentionally populated only the dedicated local assignment database rebuilt at 00:19:00; it is not a cross-run/idempotent production import.

### 2026-07-25T00:20:20+08:00 — Post-persistence SQL verification correction

- Command attempt: a read-only `psql` count query through `docker compose exec`.
- Result: the query used nonexistent plural table names (`magic_items`, `effects`, `limitations`) and PostgreSQL rejected it before returning any count. The schema declares singular `magic_item`, `effect`, and `limitation` tables.
- Scope: no database data was modified. A corrected read-only count query follows.

### 2026-07-25T00:21:36+08:00 — Post-persistence database verification and evaluator evidence update

- Command: corrected read-only `docker compose exec -T postgres psql` counts against `magic_item`, `effect`, and `limitation`.
- Result: Passed. The database contains 80 magic items, 219 effects, 155 limitations, and five `needs_review` items, matching the successful artifact replay report's `inserted_count=80`.
- Documentation: updated `EVALUATION.md` to state the final 39-page/80-record dry run and fresh-database replay evidence, the AI/deterministic-validation/human-review responsibility split, and the later live-run limitation without presenting it as a failed persisted catalog.
- Scope: read-only verification plus evaluator-facing documentation; no API call or additional catalog write occurred.

### 2026-07-25T00:26:11+08:00 — Final evaluator narrative consolidated

- Documentation: replaced the branch-local uppercase `EVALUATION.md` with the `astrid` branch's `evaluation.md` as the canonical evaluator narrative, preserving its design-process, ontology, and pipeline explanation.
- Completed: appended only verified final evidence: the successful 39-page/80-candidate dry run, five explicit human-review records, and fresh-database persistence counts (80 items, 219 effects, 155 limitations). No failed dry-run status is presented as the final result.
- Scope: documentation-only normalization for case-safe integration with `astrid`; no API request or database operation occurred.
