# Overlap Deduplication and Anomaly Summary Design

## Problem

The full OpenAI dry run `20260724T031911.919212000Z` produced 94 normalized
candidates while the visually audited PDF inventory contains 80 items.
Case-insensitive name grouping proves the source inventory is still exactly
80 names: fourteen extra candidates are duplicate extractions from overlapping
batch pages. The current `CandidateKey` includes a raw-description hash, so
the model's harmless wording differences prevent a merge.

The current CLI review summary also lists every `needs_review` item. It does
not distinguish source/OCR uncertainty from a pipeline anomaly that blocks
persistence, so it does not explain a failed completeness gate.

## Goals

- Merge alternate extraction renderings of the same source item from an
  overlapping page.
- Preserve incompatible scalar values as review evidence rather than silently
  discarding either source claim.
- Keep same-named items on separate, non-overlapping source pages distinct.
- Print actionable pipeline anomalies before ordinary review information.
- Verify the change entirely against unit tests and the saved local run
  artifacts; do not make another paid API call for this change.

## Merge identity

`CandidateKey` remains the strict artifact identity and continues to include
the description hash. `MergeCandidates` gains a separate overlap-identity
decision for raw candidates:

1. Names are compared using `normalizeIdentity`.
2. Source-page sets must overlap or touch.
3. If both conditions hold, the candidates merge even when descriptions
   differ and neither candidate is a continuation fragment.

The existing continuation merge remains necessary for multi-page entries and
continues to union their page sets and descriptions. A same-named item whose
page sets do not touch remains separate.

When scalar fields conflict, the deterministic first value remains the
normalized value and the existing recoverable `merge_conflict` issue marks the
merged record for review. Effects, limitations, descriptions, confidence, and
review reasons continue to use the existing union/merge behavior.

## CLI anomaly summary

Add a formatter that receives `RunReport`, extraction failures, and the paths
to durable artifacts. Its output order is:

1. `Extraction anomalies` with each completeness issue and extraction failure.
2. `Manual review` count with the `review.json` path; do not print all normal
   OCR/source review reasons to stdout.
3. `Run report` path.

After the corrected merge, normal full runs should have no duplicate-count
anomaly. If an anomaly occurs, its report/review artifacts remain the detailed
evidence source.

## Tests and acceptance criteria

- A same-page, same-normalized-name candidate pair with different descriptions
  merges to one candidate.
- Case-only name variants on the same page merge to one candidate.
- Same-normalized-name candidates on non-touching pages remain separate.
- Conflicting scalar fields on an overlap merge produce a reviewable conflict.
- CLI output includes completeness/failure anomalies before the review count,
  excludes individual normal review lines, and includes both artifact paths.
- Full local Go tests, vet, and whitespace checks pass.
- Replaying the saved `20260724T031911.919212000Z` artifacts through an
  offline diagnostic confirms the 94 candidates collapse to 80. This does not
  authorize a database replay or a fresh paid full extraction.
