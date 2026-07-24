package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportWritesPreDBArtifactsAndResumeSafeAccounting(t *testing.T) {
	cfg := testConfig(t)
	cfg.APIKey = "artifact-secret"
	result := testExtractionResult(t, "Artifact Item")
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	report := newRunReport(cfg, []Page{{Number: 1, ImagePath: "page-001.png"}}, result)

	if err := writePreDBArtifacts(store, []OCRPage{{Number: 1, Text: "saved OCR"}}, result, &report); err != nil {
		t.Fatalf("writePreDBArtifacts: %v", err)
	}

	for _, name := range []string{"ocr.json", "raw_candidates.json", "normalized.json", "review.json", "report.json"} {
		path := filepath.Join(store.Root(), name)
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(contents), cfg.APIKey) {
			t.Fatalf("%s contains API key", name)
		}
	}

	var raw []RawCandidate
	if err := store.ReadJSON("raw_candidates.json", &raw); err != nil {
		t.Fatalf("read raw candidates: %v", err)
	}
	if len(raw) != 1 || raw[0].Name != "Artifact Item" {
		t.Fatalf("raw candidates = %#v, want Artifact Item", raw)
	}
	var persisted RunReport
	if err := store.ReadJSON("report.json", &persisted); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if persisted.PageCount != 1 || persisted.CandidateCount != 1 || persisted.NormalizedCount != 1 || persisted.ReviewCount != 0 || persisted.FailureCount != 0 {
		t.Fatalf("report accounting = %#v, want one clean candidate", persisted)
	}
	if persisted.TextCalls != 2 || persisted.ImageCalls != 1 || persisted.ReconciliationCalls != 1 || persisted.APICalls != 4 {
		t.Fatalf("report call accounting = %#v, want all API counts", persisted)
	}
	if persisted.DatabaseResumeSupported {
		t.Fatal("report claims unsupported database resume is available")
	}
}

func TestReportStatesDatabaseResumeIsUnsupported(t *testing.T) {
	report := newRunReport(testConfig(t), nil, ExtractionResult{})
	if report.DatabaseResumeSupported {
		t.Fatal("database resume support = true, want false")
	}
	if !strings.Contains(strings.ToLower(report.DatabaseResumeNote), "fresh") || !strings.Contains(strings.ToLower(report.DatabaseResumeNote), "database") {
		t.Fatalf("database resume note = %q, want fresh-database limitation", report.DatabaseResumeNote)
	}
}

func TestReportWritesNonNegativeRetryCountFromActualAttempts(t *testing.T) {
	cfg := testConfig(t)
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}

	result := ExtractionResult{APICalls: 4, LogicalRequests: 3}
	report := newRunReport(cfg, nil, result)
	if report.RetryCount != 1 {
		t.Fatalf("retry count = %d, want one retry", report.RetryCount)
	}
	if err := writePreDBArtifacts(store, nil, result, &report); err != nil {
		t.Fatalf("writePreDBArtifacts: %v", err)
	}
	var persisted RunReport
	if err := store.ReadJSON("report.json", &persisted); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if persisted.RetryCount != 1 || persisted.LogicalRequests != 3 {
		t.Fatalf("persisted retry accounting = %#v, want one retry from three logical requests", persisted)
	}

	clamped := newRunReport(cfg, nil, ExtractionResult{APICalls: 1, LogicalRequests: 3})
	if clamped.RetryCount != 0 {
		t.Fatalf("retry count = %d, want non-negative zero", clamped.RetryCount)
	}
}

func TestFormatReviewSummaryListsCandidatesReasonsAndArtifact(t *testing.T) {
	summary := formatReviewSummary("tmp/extraction/run-123/review.json", []NormalizedCandidate{{
		Raw: RawCandidate{Name: "Exo-Armor", SourcePages: []int{7, 8, 9}},
		ReviewReasons: []string{
			"OCR stat bonus is unclear",
			"source lore needs confirmation",
		},
	}})

	for _, want := range []string{
		"Review required: 1 candidate(s)",
		"Exo-Armor (pages 7,8,9)",
		"OCR stat bonus is unclear; source lore needs confirmation",
		"Review details: tmp/extraction/run-123/review.json",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary = %q, want %q", summary, want)
		}
	}
}

func TestFormatReviewSummaryOmitsOutputWithoutCandidates(t *testing.T) {
	if summary := formatReviewSummary("tmp/extraction/run-123/review.json", nil); summary != "" {
		t.Fatalf("summary = %q, want empty", summary)
	}
}
