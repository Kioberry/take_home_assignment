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
