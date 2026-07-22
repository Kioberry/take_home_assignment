package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type RunFailure struct {
	CandidateName string            `json:"candidate_name,omitempty"`
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
		Failures:                failuresFromExtraction(result.Failed),
		CompletenessIssues:      append([]ValidationIssue(nil), result.CompletenessIssues...),
	}
}

func failuresFromExtraction(failures []ExtractionFailure) []RunFailure {
	result := make([]RunFailure, 0, len(failures))
	for _, failure := range failures {
		result = append(result, RunFailure{
			CandidateName: failure.Candidate.Name,
			Stage:         failure.Stage,
			Issues:        append([]ValidationIssue(nil), failure.Issues...),
		})
	}
	return result
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
	if len(raw) != previous.CandidateCount {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume raw candidates count %d does not match report count %d", len(raw), previous.CandidateCount)
	}
	if len(normalized) != previous.NormalizedCount {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume normalized candidates count %d does not match report count %d", len(normalized), previous.NormalizedCount)
	}
	if len(raw) != len(normalized)+len(review.Failures) {
		return nil, nil, ExtractionResult{}, fmt.Errorf("resume raw candidates are incompatible with normalized and failed artifacts")
	}
	result := ExtractionResult{
		Accepted:            make([]NormalizedCandidate, 0, len(normalized)),
		Review:              make([]NormalizedCandidate, 0, len(review.Candidates)),
		Failed:              append([]ExtractionFailure(nil), review.Failures...),
		TextCalls:           previous.TextCalls,
		ImageCalls:          previous.ImageCalls,
		ReconciliationCalls: previous.ReconciliationCalls,
		APICalls:            previous.APICalls,
		CompletenessIssues:  append([]ValidationIssue(nil), previous.CompletenessIssues...),
	}
	for _, candidate := range normalized {
		if candidate.NeedsReview {
			result.Review = append(result.Review, candidate)
		} else {
			result.Accepted = append(result.Accepted, candidate)
		}
	}
	if len(review.Candidates) > 0 {
		result.Review = append([]NormalizedCandidate(nil), review.Candidates...)
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
