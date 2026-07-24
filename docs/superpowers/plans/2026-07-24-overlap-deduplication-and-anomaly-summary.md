# Overlap Deduplication and Anomaly Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Collapse duplicate overlapping-batch extractions and make CLI output identify blocking anomalies before ordinary human review.

**Architecture:** Keep `CandidateKey` as a strict artifact identity. Extend `MergeCandidates` with a separate source-overlap identity based on normalized name plus touching page ranges, then retain existing deterministic field union and conflict recording. Replace the review-only formatter with a report-aware anomaly formatter.

**Tech Stack:** Go 1.26, standard library testing, jq for local ignored-artifact inspection.

## Global Constraints

- Do not change the OpenAI prompt or make an API call.
- Do not connect to, reset, or write the database.
- Preserve non-touching same-named source items as separate candidates.
- Preserve scalar conflicts as recoverable review evidence.
- Add a `log.md` entry for each completed implementation and verification step.

---

### Task 1: Merge overlapping alternate extractions

**Files:**
- Modify: `cmd/extract/merge.go`
- Test: `cmd/extract/merge_test.go`

**Interfaces:**
- Consumes: `RawCandidate`, `normalizeIdentity`, `pagesTouch`, and `mergeCandidateInto`.
- Produces: `canMergeCandidates(group candidateMergeGroup, candidate RawCandidate) bool` that merges equal normalized names on touching pages regardless of description hash.

- [ ] Write a failing test for same-page same-name candidates with different descriptions, plus a case-only name variant. Retain the existing non-touching-page test as the exclusion test.
- [ ] Run `env GOCACHE=/private/tmp/mulholland-task9-gocache go test ./cmd/extract -run 'TestMergeCandidatesMergesSamePageAlternateDescriptions|TestMergeCandidatesRetainsSameNameOnDifferentPages' -count=1`; expect the new case to fail with two candidates.
- [ ] Change `canMergeCandidates` to match normalized names with touching page sets without requiring `Continuation`.
- [ ] Re-run focused tests and commit `fix: merge overlapping extraction variants`.

### Task 2: Print anomaly-first CLI summary

**Files:**
- Modify: `cmd/extract/report.go`
- Modify: `cmd/extract/main.go`
- Test: `cmd/extract/report_test.go`

**Interfaces:**
- Consumes: `RunReport`, `ExtractionResult`, and artifact paths.
- Produces: `formatRunSummary(report RunReport, result ExtractionResult, reviewPath, reportPath string) string`.

- [ ] Write a failing formatter test with one completeness issue and one ordinary OCR review; assert anomaly precedes manual-review count, ordinary review text is absent, and both artifact paths are present.
- [ ] Run `env GOCACHE=/private/tmp/mulholland-task9-gocache go test ./cmd/extract -run TestFormatRunSummary -count=1`; expect compilation failure because the formatter is absent.
- [ ] Implement the formatter: list completeness issues and extraction failures under `Extraction anomalies`; print only review count and `review.json` path; always print `report.json` path. Wire it after pre-database artifact writing.
- [ ] Re-run focused tests and commit `feat: summarize extraction anomalies`.

### Task 3: Offline verification evidence

**Files:**
- Modify: `log.md`

**Interfaces:**
- Consumes: `tmp/extraction/20260724T031911.919212000Z/normalized.json`.
- Produces: logged proof that the saved run collapses to 80 normalized-name/page-overlap candidates.

- [ ] Run `go vet ./...`, `go test ./... -count=1`, and `git diff --check` with `GOCACHE=/private/tmp/mulholland-task9-gocache`.
- [ ] Run an offline grouping check on saved `normalized.json`, record the exact result in `log.md`, and commit `test: verify overlap deduplication evidence`.
