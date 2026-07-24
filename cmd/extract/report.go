package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type RunFailure struct {
	CandidateName string            `json:"candidate_name,omitempty"`
	CandidateKey  string            `json:"candidate_key,omitempty"`
	Stage         string            `json:"stage"`
	Error         string            `json:"error"`
	Issues        []ValidationIssue `json:"issues,omitempty"`
}

type RunReport struct {
	RunID                   string            `json:"run_id"`
	PDFPath                 string            `json:"pdf_path"`
	SelectedPages           []int             `json:"selected_pages"`
	DryRun                  bool              `json:"dry_run"`
	Resumed                 bool              `json:"resumed"`
	DatabaseResumeSupported bool              `json:"database_resume_supported"`
	DatabaseResumeNote      string            `json:"database_resume_note"`
	PageCount               int               `json:"page_count"`
	CandidateCount          int               `json:"candidate_count"`
	NormalizedCount         int               `json:"normalized_count"`
	ReviewCount             int               `json:"review_count"`
	FailureCount            int               `json:"failure_count"`
	InsertedCount           int               `json:"inserted_count"`
	TextCalls               int               `json:"text_calls"`
	ImageCalls              int               `json:"image_calls"`
	ReconciliationCalls     int               `json:"reconciliation_calls"`
	APICalls                int               `json:"api_calls"`
	LogicalRequests         int               `json:"logical_requests"`
	RetryCount              int               `json:"retry_count"`
	Failures                []RunFailure      `json:"failures,omitempty"`
	CompletenessIssues      []ValidationIssue `json:"completeness_issues,omitempty"`
}

type ReviewArtifact struct {
	Candidates []NormalizedCandidate `json:"candidates,omitempty"`
	Failures   []ExtractionFailure   `json:"failures,omitempty"`
}

func newRunReport(cfg Config, pages []Page, result ExtractionResult) RunReport {
	return RunReport{
		RunID:                   cfg.RunID,
		PDFPath:                 cfg.PDFPath,
		SelectedPages:           append([]int(nil), cfg.SelectedPages...),
		DryRun:                  cfg.DryRun,
		DatabaseResumeSupported: false,
		DatabaseResumeNote:      "Artifact resume replays into a fresh database; database-level resume and idempotency are unsupported.",
		PageCount:               len(pages),
		CandidateCount:          len(result.Accepted) + len(result.Review) + len(result.Failed),
		NormalizedCount:         len(result.Accepted) + len(result.Review),
		ReviewCount:             len(result.Review),
		FailureCount:            len(result.Failed),
		TextCalls:               result.TextCalls,
		ImageCalls:              result.ImageCalls,
		ReconciliationCalls:     result.ReconciliationCalls,
		APICalls:                result.APICalls,
		LogicalRequests:         result.LogicalRequests,
		RetryCount:              retryCount(result.APICalls, result.LogicalRequests),
		Failures:                failuresFromExtraction(result.Failed),
		CompletenessIssues:      append([]ValidationIssue(nil), result.CompletenessIssues...),
	}
}

func failuresFromExtraction(failures []ExtractionFailure) []RunFailure {
	result := make([]RunFailure, 0, len(failures))
	for _, failure := range failures {
		result = append(result, RunFailure{
			CandidateName: failure.Candidate.Name,
			CandidateKey:  CandidateKey(failure.Candidate),
			Stage:         failure.Stage,
			Issues:        append([]ValidationIssue(nil), failure.Issues...),
		})
	}
	return result
}

func retryCount(apiCalls, logicalRequests int) int {
	if apiCalls <= logicalRequests {
		return 0
	}
	return apiCalls - logicalRequests
}

// formatReviewSummary prints the durable review artifact location alongside the
// specific candidates that require a human decision. It is intentionally
// concise so a non-GUI CLI run can be triaged from stdout.
func formatReviewSummary(reviewPath string, candidates []NormalizedCandidate) string {
	if len(candidates) == 0 {
		return ""
	}

	var summary strings.Builder
	fmt.Fprintf(&summary, "Review required: %d candidate(s)\n", len(candidates))
	for _, candidate := range candidates {
		fmt.Fprintf(
			&summary,
			"- %s (pages %s): %s\n",
			candidate.Raw.Name,
			pageNumbersFromInts(candidate.Raw.SourcePages),
			strings.Join(candidate.ReviewReasons, "; "),
		)
	}
	fmt.Fprintf(&summary, "Review details: %s\n", reviewPath)
	return summary.String()
}

// formatRunSummary keeps blocking extraction anomalies separate from ordinary
// source-review work so a headless CLI run immediately explains why it cannot
// proceed to persistence.
func formatRunSummary(report RunReport, result ExtractionResult, reviewPath, reportPath string) string {
	var summary strings.Builder
	if len(report.CompletenessIssues) > 0 || len(report.Failures) > 0 {
		summary.WriteString("Extraction anomalies:\n")
		for _, issue := range report.CompletenessIssues {
			fmt.Fprintf(&summary, "- %s: %s\n", issue.Code, issue.Message)
		}
		for _, failure := range report.Failures {
			fmt.Fprintf(&summary, "- %s (%s): %s\n", failure.CandidateName, failure.Stage, failure.Error)
		}
	}
	if len(result.Review) > 0 {
		fmt.Fprintf(&summary, "Manual review: %d candidate(s)\n", len(result.Review))
		fmt.Fprintf(&summary, "Review details: %s\n", reviewPath)
	}
	fmt.Fprintf(&summary, "Run report: %s\n", reportPath)
	return summary.String()
}

// redactExtractionResult ensures configuration secrets cannot cross an
// extraction, retry, or reporting boundary into durable artifacts.
func redactExtractionResult(result ExtractionResult, secret string) ExtractionResult {
	if secret == "" {
		return result
	}
	result.Accepted = redactNormalizedCandidates(result.Accepted, secret)
	result.Review = redactNormalizedCandidates(result.Review, secret)
	for index := range result.Failed {
		result.Failed[index].Candidate = redactRawCandidate(result.Failed[index].Candidate, secret)
		result.Failed[index].Issues = redactValidationIssues(result.Failed[index].Issues, secret)
	}
	result.CompletenessIssues = redactValidationIssues(result.CompletenessIssues, secret)
	return result
}

func redactNormalizedCandidates(candidates []NormalizedCandidate, secret string) []NormalizedCandidate {
	redacted := append([]NormalizedCandidate(nil), candidates...)
	for index := range redacted {
		redacted[index].Raw = redactRawCandidate(redacted[index].Raw, secret)
		redacted[index].ReviewReasons = redactStrings(redacted[index].ReviewReasons, secret)
		for effectIndex := range redacted[index].Effects {
			redacted[index].Effects[effectIndex].Description = redactString(redacted[index].Effects[effectIndex].Description, secret)
		}
	}
	return redacted
}

func redactRawCandidate(candidate RawCandidate, secret string) RawCandidate {
	candidate.Name = redactString(candidate.Name, secret)
	candidate.SourceItemTypeRaw = redactString(candidate.SourceItemTypeRaw, secret)
	if candidate.SourceItemSubtypeRaw != nil {
		value := redactString(*candidate.SourceItemSubtypeRaw, secret)
		candidate.SourceItemSubtypeRaw = &value
	}
	candidate.RarityRaw = redactString(candidate.RarityRaw, secret)
	candidate.UsageModeRaw = redactString(candidate.UsageModeRaw, secret)
	if candidate.WearSlotRaw != nil {
		value := redactString(*candidate.WearSlotRaw, secret)
		candidate.WearSlotRaw = &value
	}
	if candidate.AttunementRequirement != nil {
		value := redactString(*candidate.AttunementRequirement, secret)
		candidate.AttunementRequirement = &value
	}
	candidate.RawDescription = redactString(candidate.RawDescription, secret)
	candidate.ReviewReasons = redactStrings(candidate.ReviewReasons, secret)
	candidate.Effects = append([]RawEffect(nil), candidate.Effects...)
	for index := range candidate.Effects {
		candidate.Effects[index].CategoryRaw = redactString(candidate.Effects[index].CategoryRaw, secret)
		candidate.Effects[index].Description = redactString(candidate.Effects[index].Description, secret)
	}
	candidate.Limitations = append([]RawLimitation(nil), candidate.Limitations...)
	for index := range candidate.Limitations {
		candidate.Limitations[index].Description = redactString(candidate.Limitations[index].Description, secret)
	}
	return candidate
}

func redactValidationIssues(issues []ValidationIssue, secret string) []ValidationIssue {
	redacted := append([]ValidationIssue(nil), issues...)
	for index := range redacted {
		redacted[index].Code = redactString(redacted[index].Code, secret)
		redacted[index].Message = redactString(redacted[index].Message, secret)
	}
	return redacted
}

func redactStrings(values []string, secret string) []string {
	redacted := append([]string(nil), values...)
	for index := range redacted {
		redacted[index] = redactString(redacted[index], secret)
	}
	return redacted
}

func redactString(value, secret string) string {
	if secret == "" {
		return value
	}
	return strings.ReplaceAll(value, secret, "[REDACTED]")
}

func writePreDBArtifacts(store *ArtifactStore, ocr []OCRPage, result ExtractionResult, report *RunReport) error {
	if store == nil || report == nil {
		return fmt.Errorf("artifact store and report are required")
	}
	raw := make([]RawCandidate, 0, report.CandidateCount)
	for _, candidate := range result.Accepted {
		raw = append(raw, candidate.Raw)
	}
	for _, candidate := range result.Review {
		raw = append(raw, candidate.Raw)
	}
	for _, failure := range result.Failed {
		raw = append(raw, failure.Candidate)
	}
	normalized := append([]NormalizedCandidate(nil), result.Accepted...)
	normalized = append(normalized, result.Review...)
	artifacts := []struct {
		name  string
		value any
	}{
		{name: "ocr.json", value: ocr},
		{name: "raw_candidates.json", value: raw},
		{name: "normalized.json", value: normalized},
		{name: "review.json", value: ReviewArtifact{Candidates: append([]NormalizedCandidate(nil), result.Review...), Failures: append([]ExtractionFailure(nil), result.Failed...)}},
	}
	for _, artifact := range artifacts {
		if err := store.WriteJSON(artifact.name, artifact.value); err != nil {
			return err
		}
	}
	return store.WriteJSON("report.json", *report)
}

func loadResumeArtifacts(store *ArtifactStore, cfg *Config) ([]Page, []OCRPage, ExtractionResult, error) {
	if store == nil {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume artifact store is required")
	}
	var ocr []OCRPage
	var raw []RawCandidate
	var normalized []NormalizedCandidate
	var review ReviewArtifact
	var previous RunReport
	for _, artifact := range []struct {
		name  string
		value any
	}{
		{"ocr.json", &ocr},
		{"raw_candidates.json", &raw},
		{"normalized.json", &normalized},
		{"review.json", &review},
		{"report.json", &previous},
	} {
		if err := store.ReadJSON(artifact.name, artifact.value); err != nil {
			return nil, nil, ExtractionResult{}, fmt.Errorf("load resume artifact %s: %w", artifact.name, err)
		}
	}
	if previous.RunID != "" && previous.RunID != cfg.RunID {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume artifact run ID %q does not match %q", previous.RunID, cfg.RunID)
	}
	if previous.PDFPath == "" || filepath.Clean(previous.PDFPath) != filepath.Clean(cfg.PDFPath) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume PDF path %q does not match %q", previous.PDFPath, cfg.PDFPath)
	}
	if len(cfg.SelectedPages) == 0 {
		cfg.SelectedPages = append([]int(nil), previous.SelectedPages...)
	} else if !samePages(cfg.SelectedPages, previous.SelectedPages) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume selected pages %#v do not match artifact selected pages %#v", cfg.SelectedPages, previous.SelectedPages)
	}
	if previous.CandidateCount != len(raw) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume raw candidates count %d does not match report count %d", len(raw), previous.CandidateCount)
	}
	if previous.NormalizedCount != len(normalized) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume normalized candidates count %d does not match report count %d", len(normalized), previous.NormalizedCount)
	}
	if previous.ReviewCount != len(review.Candidates) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume review candidates count %d does not match report count %d", len(review.Candidates), previous.ReviewCount)
	}
	if previous.FailureCount != len(previous.Failures) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume report failures count %d does not match report count %d", len(previous.Failures), previous.FailureCount)
	}
	if err := validateResumeCandidateGraph(raw, normalized, review, previous); err != nil {
		return nil, nil, ExtractionResult{}, err
	}
	result := ExtractionResult{
		Accepted:            make([]NormalizedCandidate, 0, len(normalized)),
		Review:              make([]NormalizedCandidate, 0, len(review.Candidates)),
		Failed:              append([]ExtractionFailure(nil), review.Failures...),
		LogicalRequests:     previous.LogicalRequests,
		TextCalls:           previous.TextCalls,
		ImageCalls:          previous.ImageCalls,
		ReconciliationCalls: previous.ReconciliationCalls,
		APICalls:            previous.APICalls,
		CompletenessIssues:  append([]ValidationIssue(nil), previous.CompletenessIssues...),
	}
	reviewKeys := make(map[string]struct{}, len(review.Candidates))
	for _, candidate := range review.Candidates {
		reviewKeys[CandidateKey(candidate.Raw)] = struct{}{}
	}
	for _, candidate := range normalized {
		if _, needsReview := reviewKeys[CandidateKey(candidate.Raw)]; needsReview {
			result.Review = append(result.Review, candidate)
		} else {
			result.Accepted = append(result.Accepted, candidate)
		}
	}
	pages := make([]Page, 0, len(ocr))
	for _, page := range ocr {
		pages = append(pages, Page{Number: page.Number, ImagePath: filepath.Join(store.Root(), "pages", fmt.Sprintf("page-%03d.png", page.Number))})
	}
	if previous.PageCount > len(pages) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume OCR has %d pages, report requires %d", len(pages), previous.PageCount)
	}
	return pages, ocr, result, nil
}

func validateResumeCandidateGraph(raw []RawCandidate, normalized []NormalizedCandidate, review ReviewArtifact, report RunReport) error {
	rawKeys, err := rawCandidateKeys(raw, "raw candidates")
	if err != nil {
		return err
	}
	normalizedKeys, err := normalizedCandidateKeys(normalized, "normalized candidates")
	if err != nil {
		return err
	}
	reviewKeys, err := normalizedCandidateKeys(review.Candidates, "review candidates")
	if err != nil {
		return err
	}
	failureKeys, err := extractionFailureKeys(review.Failures)
	if err != nil {
		return err
	}
	normalizedKeySet := make(map[string]struct{}, len(normalizedKeys))
	for key := range normalizedKeys {
		normalizedKeySet[key] = struct{}{}
	}
	if err := requireKeySetSubset(normalizedKeySet, rawKeys, "normalized", "raw"); err != nil {
		return err
	}
	if err := requireKeySetMatches(rawKeys, unionKeySets(normalizedKeySet, failureKeys), "raw", "normalized and extraction failure"); err != nil {
		return err
	}
	for key := range reviewKeys {
		if _, ok := normalizedKeys[key]; !ok {
			return fmt.Errorf("resume review candidate %q is not represented in normalized candidates", key)
		}
	}
	for key, candidate := range normalizedKeys {
		_, listedForReview := reviewKeys[key]
		if candidate.NeedsReview != listedForReview {
			return fmt.Errorf("resume review candidate %q has inconsistent NeedsReview state", key)
		}
	}
	for _, failure := range review.Failures {
		key := CandidateKey(failure.Candidate)
		if _, ok := rawKeys[key]; !ok {
			return fmt.Errorf("resume extraction failure candidate %q is not represented in raw candidates", key)
		}
	}
	reportedExtractionStages := make(map[string]string, len(report.Failures))
	reportedFailureKeys := make(map[string]struct{}, len(report.Failures))
	for _, failure := range report.Failures {
		if failure.CandidateKey == "" {
			if isDatabaseLifecycleStage(failure.Stage) {
				continue
			}
			return fmt.Errorf("resume report failure at stage %q is missing candidate identity", failure.Stage)
		}
		if _, ok := rawKeys[failure.CandidateKey]; !ok {
			return fmt.Errorf("resume report failure candidate %q is not represented in raw candidates", failure.CandidateKey)
		}
		if _, duplicate := reportedFailureKeys[failure.CandidateKey]; duplicate {
			return fmt.Errorf("resume report failures contain duplicate candidate identity %q", failure.CandidateKey)
		}
		reportedFailureKeys[failure.CandidateKey] = struct{}{}
		if failure.CandidateKey != "" && !isDatabaseLifecycleStage(failure.Stage) && failure.Stage != "persistence" {
			reportedExtractionStages[failure.CandidateKey] = failure.Stage
		}
	}
	if err := requireKeySetMatches(failureKeys, stringKeySet(reportedExtractionStages), "extraction failure artifact", "report extraction failure"); err != nil {
		return fmt.Errorf("resume report failure identities are inconsistent: %w", err)
	}
	for key := range failureKeys {
		stage := reportedExtractionStages[key]
		for _, failure := range review.Failures {
			if CandidateKey(failure.Candidate) == key && failure.Stage != stage {
				return fmt.Errorf("resume extraction failure candidate %q has stage %q in report, want %q", key, stage, failure.Stage)
			}
		}
	}
	return nil
}

func rawCandidateKeys(candidates []RawCandidate, label string) (map[string]struct{}, error) {
	keys := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		key := CandidateKey(candidate)
		if _, exists := keys[key]; exists {
			return nil, fmt.Errorf("resume %s contain duplicate candidate identity %q", label, key)
		}
		keys[key] = struct{}{}
	}
	return keys, nil
}

func normalizedCandidateKeys(candidates []NormalizedCandidate, label string) (map[string]NormalizedCandidate, error) {
	keys := make(map[string]NormalizedCandidate, len(candidates))
	for _, candidate := range candidates {
		key := CandidateKey(candidate.Raw)
		if _, exists := keys[key]; exists {
			return nil, fmt.Errorf("resume %s contain duplicate candidate identity %q", label, key)
		}
		keys[key] = candidate
	}
	return keys, nil
}

func extractionFailureKeys(failures []ExtractionFailure) (map[string]struct{}, error) {
	keys := make(map[string]struct{}, len(failures))
	for _, failure := range failures {
		key := CandidateKey(failure.Candidate)
		if _, exists := keys[key]; exists {
			return nil, fmt.Errorf("resume extraction failures contain duplicate candidate identity %q", key)
		}
		keys[key] = struct{}{}
	}
	return keys, nil
}

func unionKeySets(first, second map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(first)+len(second))
	for key := range first {
		result[key] = struct{}{}
	}
	for key := range second {
		result[key] = struct{}{}
	}
	return result
}

func requireKeySetMatches(want, got map[string]struct{}, wantLabel, gotLabel string) error {
	if len(want) != len(got) {
		return fmt.Errorf("resume %s and %s candidate counts are inconsistent", wantLabel, gotLabel)
	}
	for key := range want {
		if _, ok := got[key]; !ok {
			return fmt.Errorf("resume %s candidate %q is missing from %s", wantLabel, key, gotLabel)
		}
	}
	return nil
}

func requireKeySetSubset(subset, superset map[string]struct{}, subsetLabel, supersetLabel string) error {
	for key := range subset {
		if _, ok := superset[key]; !ok {
			return fmt.Errorf("resume %s candidate %q is missing from %s", subsetLabel, key, supersetLabel)
		}
	}
	return nil
}

func stringKeySet(values map[string]string) map[string]struct{} {
	keys := make(map[string]struct{}, len(values))
	for key := range values {
		keys[key] = struct{}{}
	}
	return keys
}

func isDatabaseLifecycleStage(stage string) bool {
	switch stage {
	case "connect", "schema", "empty_guard":
		return true
	default:
		return false
	}
}

func samePages(first, second []int) bool {
	if len(first) == 0 {
		return true
	}
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func safeError(err error, secret string) error {
	if err == nil {
		return nil
	}
	if secret == "" {
		return err
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), secret, "[REDACTED]"))
}
