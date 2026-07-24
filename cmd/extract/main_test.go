package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestParseConfigDefaultsFlagsAndEnvironment(t *testing.T) {
	getenv := func(name string) string {
		return map[string]string{
			"OPENAI_API_KEY":      "test-key",
			"OPENAI_TEXT_MODEL":   "text-override",
			"OPENAI_VISION_MODEL": "vision-override",
		}[name]
	}

	cfg, err := parseConfig([]string{"--dry-run", "--pages", "1-5", "--resume-run", "prior-run"}, getenv)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.DryRun || !cfg.Resume || cfg.RunID != "prior-run" {
		t.Fatalf("mode/run ID = dry:%t resume:%t id:%q, want dry resume prior-run", cfg.DryRun, cfg.Resume, cfg.RunID)
	}
	if !reflect.DeepEqual(cfg.SelectedPages, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("selected pages = %#v, want 1-5", cfg.SelectedPages)
	}
	if cfg.BatchSize != 5 || cfg.Overlap != 1 || cfg.ExpectedPages != 39 || cfg.ExpectedItems != 80 {
		t.Fatalf("defaults = %#v, want Task 8 defaults", cfg)
	}
	if cfg.TextModel != "text-override" || cfg.VisionModel != "vision-override" {
		t.Fatalf("models = %q/%q, want environment overrides", cfg.TextModel, cfg.VisionModel)
	}
	if cfg.PDFPath != pdfPath || cfg.RunRoot != "tmp/extraction" || cfg.APIURL != "https://api.openai.com" {
		t.Fatalf("paths/API URL = %#v, want default CLI locations", cfg)
	}
}

func TestParseConfigRejectsInvalidOrConflictingInput(t *testing.T) {
	getenv := func(name string) string {
		if name == "OPENAI_API_KEY" {
			return "test-key"
		}
		return ""
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "run ID conflicts with resume", args: []string{"--run-id", "new", "--resume-run", "old"}, want: "cannot be used together"},
		{name: "zero batch", args: []string{"--batch-size", "0"}, want: "batch"},
		{name: "negative overlap", args: []string{"--overlap", "-1"}, want: "overlap"},
		{name: "overlap meets batch", args: []string{"--batch-size", "4", "--overlap", "4"}, want: "smaller"},
		{name: "invalid pages", args: []string{"--pages", "5-3"}, want: "page range"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseConfig(test.args, getenv)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.want)) {
				t.Fatalf("parseConfig(%#v) error = %v, want %q", test.args, err, test.want)
			}
		})
	}

	_, err := parseConfig(nil, func(string) string { return "" })
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "api key") {
		t.Fatalf("missing API key error = %v, want actionable API key error", err)
	}
}

func TestRunDryRunWritesArtifactsAndNeverTouchesDatabase(t *testing.T) {
	cfg := testConfig(t)
	cfg.DryRun = true
	cfg.APIKey = "dry-run-secret"
	result := testExtractionResult(t, "Dry Run Item")
	deps := testDependencies(t, result)
	databaseCalls := 0
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		databaseCalls++
		return nil, errors.New("dry-run must not connect")
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
		databaseCalls++
		return errors.New("dry-run must not apply schema")
	}
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
		databaseCalls++
		return errors.New("dry-run must not guard a database")
	}
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error {
		databaseCalls++
		return errors.New("dry-run must not persist")
	}

	if err := run(context.Background(), cfg, deps); err != nil {
		t.Fatalf("run dry-run: %v", err)
	}
	if databaseCalls != 0 {
		t.Fatalf("database calls = %d, want none for dry-run", databaseCalls)
	}

	for _, name := range []string{"ocr.json", "raw_candidates.json", "normalized.json", "review.json", "report.json"} {
		path := filepath.Join(cfg.RunRoot, cfg.RunID, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("artifact %s: %v", name, err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(contents), cfg.APIKey) {
			t.Fatalf("artifact %s contains API key", name)
		}
	}

	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	var report RunReport
	if err := store.ReadJSON("report.json", &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if report.PageCount != 1 || report.CandidateCount != 1 || report.NormalizedCount != 1 {
		t.Fatalf("report counts = %#v, want one page/candidate/normalized record", report)
	}
	if report.TextCalls != 2 || report.ImageCalls != 1 || report.ReconciliationCalls != 1 || report.APICalls != 4 {
		t.Fatalf("report API calls = %#v, want preserved extraction counts", report)
	}
	if report.DryRun != true || report.InsertedCount != 0 || report.FailureCount != 0 {
		t.Fatalf("report dry-run state = %#v, want no inserts or failures", report)
	}
}

func TestResumeReusesArtifactsSkipsExternalCallsAndRequiresFreshDatabaseLifecycle(t *testing.T) {
	cfg := testConfig(t)
	result := testExtractionResult(t, "Resumed Item")
	writeResumeArtifacts(t, cfg, result)
	cfg.Resume = true

	deps := testDependencies(t, ExtractionResult{})
	unexpectedExternalCalls := 0
	deps.Render = func(context.Context, Config, string) ([]Page, error) {
		unexpectedExternalCalls++
		return nil, errors.New("resume must not render")
	}
	deps.OCR = func(context.Context, []Page) ([]OCRPage, error) {
		unexpectedExternalCalls++
		return nil, errors.New("resume must not OCR")
	}
	deps.Extract = func(context.Context, []Page, []OCRPage, Config) ExtractionResult {
		unexpectedExternalCalls++
		return ExtractionResult{}
	}
	var calls []string
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		calls = append(calls, "connect")
		return nil, nil
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "apply")
		return nil
	}
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "guard")
		return nil
	}
	deps.Persist = func(_ context.Context, _ *pgxpool.Pool, candidate NormalizedCandidate) error {
		calls = append(calls, "persist:"+candidate.Raw.Name)
		return nil
	}

	if err := run(context.Background(), cfg, deps); err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if unexpectedExternalCalls != 0 {
		t.Fatalf("resume external calls = %d, want none", unexpectedExternalCalls)
	}
	if want := []string{"connect", "apply", "guard", "persist:Resumed Item"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("database lifecycle = %#v, want %#v", calls, want)
	}

	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	var report RunReport
	if err := store.ReadJSON("report.json", &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !report.Resumed || report.DatabaseResumeSupported {
		t.Fatalf("resume report = %#v, want artifact resume with database resume unsupported", report)
	}
}

func TestResumeRejectsIncompatiblePagesBeforeDatabaseWork(t *testing.T) {
	cfg := testConfig(t)
	writeResumeArtifacts(t, cfg, testExtractionResult(t, "Resume Item"))
	cfg.Resume = true
	cfg.SelectedPages = []int{2}
	deps := testDependencies(t, ExtractionResult{})
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		return nil, errors.New("database must not be reached")
	}

	err := run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "selected pages") {
		t.Fatalf("resume compatibility error = %v, want selected-pages mismatch", err)
	}
}

func TestResumeRejectsIncompatiblePDFBeforeDatabaseWork(t *testing.T) {
	cfg := testConfig(t)
	writeResumeArtifacts(t, cfg, testExtractionResult(t, "Resume Item"))
	cfg.Resume = true
	cfg.PDFPath = "other-catalog.pdf"
	deps := testDependencies(t, ExtractionResult{})
	databaseCalls := 0
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		databaseCalls++
		return nil, errors.New("database must not be reached")
	}

	err := run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "pdf") {
		t.Fatalf("resume compatibility error = %v, want PDF mismatch", err)
	}
	if databaseCalls != 0 {
		t.Fatalf("database calls = %d, want none after incompatible resume", databaseCalls)
	}
}

func TestResumeUsesSavedSelectedPagesWhenTheFlagIsOmitted(t *testing.T) {
	cfg := testConfig(t)
	writeResumeArtifacts(t, cfg, testExtractionResult(t, "Resume Item"))
	cfg.Resume = true
	cfg.SelectedPages = nil
	deps := testDependencies(t, ExtractionResult{})
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) { return nil, nil }
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error { return nil }

	if err := run(context.Background(), cfg, deps); err != nil {
		t.Fatalf("resume run: %v", err)
	}
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	var report RunReport
	if err := store.ReadJSON("report.json", &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !reflect.DeepEqual(report.SelectedPages, []int{1}) {
		t.Fatalf("resumed report pages = %#v, want saved selection [1]", report.SelectedPages)
	}
}

func TestResumeRejectsRawArtifactCountMismatchBeforeDatabaseWork(t *testing.T) {
	cfg := testConfig(t)
	result := testExtractionResult(t, "Resume Item")
	writeResumeArtifacts(t, cfg, result)
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	extra := validRawCandidate()
	extra.Name = "Unexpected Raw Candidate"
	if err := store.WriteJSON("raw_candidates.json", []RawCandidate{result.Accepted[0].Raw, extra}); err != nil {
		t.Fatalf("write incompatible raw artifact: %v", err)
	}
	cfg.Resume = true
	deps := testDependencies(t, ExtractionResult{})
	databaseCalls := 0
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		databaseCalls++
		return nil, errors.New("database must not be reached")
	}

	err = run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "raw candidates") {
		t.Fatalf("resume integrity error = %v, want raw artifact mismatch", err)
	}
	if databaseCalls != 0 {
		t.Fatalf("database calls = %d, want none after incompatible artifacts", databaseCalls)
	}
}

func TestRunContinuesAfterPersistenceFailureAndReportsIt(t *testing.T) {
	cfg := testConfig(t)
	first := testExtractionResult(t, "First").Accepted[0]
	second := testExtractionResult(t, "Second").Accepted[0]
	result := ExtractionResult{Accepted: []NormalizedCandidate{first, second}}
	deps := testDependencies(t, result)
	var calls []string
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		calls = append(calls, "connect")
		return nil, nil
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "apply")
		return nil
	}
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "guard")
		return nil
	}
	deps.Persist = func(_ context.Context, _ *pgxpool.Pool, candidate NormalizedCandidate) error {
		calls = append(calls, "persist:"+candidate.Raw.Name)
		if candidate.Raw.Name == "First" {
			return fmt.Errorf("persistence rejected %s", cfg.APIKey)
		}
		return nil
	}

	err := run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "failure") {
		t.Fatalf("run error = %v, want reported persistence failure", err)
	}
	if want := []string{"connect", "apply", "guard", "persist:First", "persist:Second"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("call order = %#v, want %#v", calls, want)
	}

	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	var report RunReport
	if err := store.ReadJSON("report.json", &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if report.InsertedCount != 1 || report.FailureCount != 1 || len(report.Failures) != 1 {
		t.Fatalf("persistence report = %#v, want one insert and one failure", report)
	}
	if report.Failures[0].CandidateName != "First" || report.Failures[0].Stage != "persistence" {
		t.Fatalf("failure = %#v, want First persistence failure", report.Failures[0])
	}
	contents, err := os.ReadFile(filepath.Join(cfg.RunRoot, cfg.RunID, "report.json"))
	if err != nil {
		t.Fatalf("read report bytes: %v", err)
	}
	if strings.Contains(string(contents), cfg.APIKey) {
		t.Fatal("report contains API key from a dependency error")
	}
}

func TestRunPersistsReviewCandidatesAfterTheEmptyCatalogGuard(t *testing.T) {
	cfg := testConfig(t)
	review := testExtractionResult(t, "Review Item").Accepted[0]
	review.NeedsReview = true
	result := ExtractionResult{Review: []NormalizedCandidate{review}}
	deps := testDependencies(t, result)
	var calls []string
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		calls = append(calls, "connect")
		return nil, nil
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "apply")
		return nil
	}
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "guard")
		return nil
	}
	deps.Persist = func(_ context.Context, _ *pgxpool.Pool, candidate NormalizedCandidate) error {
		calls = append(calls, "persist:"+candidate.Raw.Name)
		if !candidate.NeedsReview {
			t.Fatal("persisted review candidate lost NeedsReview")
		}
		return nil
	}

	if err := run(context.Background(), cfg, deps); err != nil {
		t.Fatalf("run review candidate: %v", err)
	}
	if want := []string{"connect", "apply", "guard", "persist:Review Item"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("call order = %#v, want %#v", calls, want)
	}
}

func TestRunReportsExtractionFailureWithoutDatabasePhase(t *testing.T) {
	cfg := testConfig(t)
	accepted := testExtractionResult(t, "Persisted Item").Accepted[0]
	failed := validRawCandidate()
	failed.Name = "Failed Item"
	result := ExtractionResult{
		Accepted: []NormalizedCandidate{accepted},
		Failed: []ExtractionFailure{{
			Candidate: failed,
			Issues:    []ValidationIssue{{Code: "missing_core", Message: "missing source content"}},
			Stage:     "validation",
		}},
	}
	deps := testDependencies(t, result)
	databaseCalls := 0
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		databaseCalls++
		return nil, errors.New("database must not connect after extraction failure")
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
		databaseCalls++
		return errors.New("schema must not apply after extraction failure")
	}
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
		databaseCalls++
		return errors.New("database guard must not run after extraction failure")
	}
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error {
		databaseCalls++
		return errors.New("persistence must not run after extraction failure")
	}

	if err := run(context.Background(), cfg, deps); err == nil {
		t.Fatal("run succeeded despite a failed candidate")
	}
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	var report RunReport
	if err := store.ReadJSON("report.json", &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if databaseCalls != 0 {
		t.Fatalf("database calls = %d, want none after extraction failure", databaseCalls)
	}
	if report.FailureCount != 1 || len(report.Failures) != 1 || report.InsertedCount != 0 {
		t.Fatalf("report = %#v, want one extraction failure and no persisted items", report)
	}
}

func TestRunRequiresInjectedDatabaseDependencies(t *testing.T) {
	cfg := testConfig(t)
	deps := testDependencies(t, testExtractionResult(t, "Injected Item"))
	deps.Connect = nil
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error { return nil }

	err := run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "connect dependency") {
		t.Fatalf("run error = %v, want missing injected connect dependency", err)
	}
}

func TestRunRejectsIncompleteDependenciesBeforeAnySideEffect(t *testing.T) {
	cfg := testConfig(t)
	deps := testDependencies(t, testExtractionResult(t, "Incomplete Item"))
	var calls []string
	deps.Validate = func(context.Context, Config) error {
		calls = append(calls, "validate")
		return nil
	}
	deps.OpenArtifacts = func(string, string) (*ArtifactStore, error) {
		calls = append(calls, "artifacts")
		return nil, errors.New("artifacts must not open")
	}
	deps.Render = func(context.Context, Config, string) ([]Page, error) {
		calls = append(calls, "render")
		return nil, errors.New("render must not run")
	}
	deps.OCR = nil
	deps.Extract = func(context.Context, []Page, []OCRPage, Config) ExtractionResult {
		calls = append(calls, "extract")
		return ExtractionResult{}
	}
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		calls = append(calls, "connect")
		return nil, errors.New("database must not connect")
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "schema")
		return nil
	}
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
		calls = append(calls, "guard")
		return nil
	}
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error {
		calls = append(calls, "persist")
		return nil
	}

	err := run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "ocr dependency") {
		t.Fatalf("run error = %v, want actionable missing OCR dependency", err)
	}
	if len(calls) != 0 {
		t.Fatalf("dependency side effects = %#v, want none", calls)
	}
	entries, err := os.ReadDir(cfg.RunRoot)
	if err != nil {
		t.Fatalf("read artifact root: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("artifact root entries = %#v, want none", entries)
	}
}

func TestResumeRejectsStaleReviewCandidateBeforeDatabaseWork(t *testing.T) {
	cfg := testConfig(t)
	result := testExtractionResult(t, "Current Item")
	writeResumeArtifacts(t, cfg, result)
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	stale := testExtractionResult(t, "Stale Review Item").Accepted[0]
	if err := store.WriteJSON("review.json", ReviewArtifact{Candidates: []NormalizedCandidate{stale}}); err != nil {
		t.Fatalf("write stale review artifact: %v", err)
	}
	cfg.Resume = true
	databaseCalls := 0
	deps := testDependencies(t, ExtractionResult{})
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		databaseCalls++
		return nil, errors.New("database must not be reached")
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error { return nil }

	err = run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "review") {
		t.Fatalf("resume integrity error = %v, want stale review rejection", err)
	}
	if databaseCalls != 0 {
		t.Fatalf("database calls = %d, want none after stale review", databaseCalls)
	}
}

func TestResumeRejectsOrphanReportFailureForAcceptedCandidateBeforeDatabaseWork(t *testing.T) {
	cfg := testConfig(t)
	result := testExtractionResult(t, "Accepted Item")
	writeResumeArtifacts(t, cfg, result)
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	var report RunReport
	if err := store.ReadJSON("report.json", &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	key := CandidateKey(result.Accepted[0].Raw)
	report.Failures = []RunFailure{
		{CandidateName: result.Accepted[0].Raw.Name, CandidateKey: key, Stage: "validation"},
		{CandidateName: result.Accepted[0].Raw.Name, CandidateKey: key, Stage: "validation"},
	}
	report.FailureCount = len(report.Failures)
	if err := store.WriteJSON("report.json", report); err != nil {
		t.Fatalf("write corrupt report: %v", err)
	}
	cfg.Resume = true
	databaseCalls := 0
	deps := testDependencies(t, ExtractionResult{})
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		databaseCalls++
		return nil, errors.New("database must not be reached")
	}

	err = run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "report failure") {
		t.Fatalf("resume integrity error = %v, want orphan/duplicate report failure rejection", err)
	}
	if databaseCalls != 0 {
		t.Fatalf("database calls = %d, want none after incompatible report failures", databaseCalls)
	}
}

func TestRunWritesTerminalReportForDatabaseLifecycleFailures(t *testing.T) {
	for _, test := range []struct {
		name  string
		stage string
		setup func(*Dependencies, string)
	}{
		{
			name:  "connect",
			stage: "connect",
			setup: func(deps *Dependencies, secret string) {
				deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
					return nil, fmt.Errorf("connection failed with %s", secret)
				}
			},
		},
		{
			name:  "schema",
			stage: "schema",
			setup: func(deps *Dependencies, secret string) {
				deps.Connect = func(context.Context) (*pgxpool.Pool, error) { return nil, nil }
				deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
					return fmt.Errorf("schema failed with %s", secret)
				}
			},
		},
		{
			name:  "empty guard",
			stage: "empty_guard",
			setup: func(deps *Dependencies, secret string) {
				deps.Connect = func(context.Context) (*pgxpool.Pool, error) { return nil, nil }
				deps.ApplySchema = func(context.Context, *pgxpool.Pool) error { return nil }
				deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
					return fmt.Errorf("guard failed with %s", secret)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.APIKey = "database-secret"
			deps := testDependencies(t, testExtractionResult(t, "Terminal Item"))
			deps.ApplySchema = func(context.Context, *pgxpool.Pool) error { return nil }
			deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error { return nil }
			deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error { return nil }
			test.setup(&deps, cfg.APIKey)

			err := run(context.Background(), cfg, deps)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.stage) {
				t.Fatalf("run error = %v, want %s failure", err, test.stage)
			}
			if strings.Contains(err.Error(), cfg.APIKey) {
				t.Fatalf("run error contains API key: %v", err)
			}

			store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
			if err != nil {
				t.Fatalf("NewArtifactStore: %v", err)
			}
			var report RunReport
			if err := store.ReadJSON("report.json", &report); err != nil {
				t.Fatalf("read terminal report: %v", err)
			}
			if report.FailureCount != 1 || len(report.Failures) != 1 {
				t.Fatalf("terminal report = %#v, want one failure", report)
			}
			if report.Failures[0].Stage != test.stage || report.Failures[0].Error == "" {
				t.Fatalf("terminal failure = %#v, want stage and message", report.Failures[0])
			}
			if strings.Contains(report.Failures[0].Error, cfg.APIKey) {
				t.Fatalf("terminal report contains API key: %#v", report.Failures[0])
			}
		})
	}
}

func TestRunReturnsNonzeroForCandidateAndCompletenessFailures(t *testing.T) {
	cfg := testConfig(t)
	cfg.DryRun = true
	failed := validRawCandidate()
	failed.Name = "Unresolved"
	result := ExtractionResult{
		Failed: []ExtractionFailure{{
			Candidate: failed,
			Issues:    []ValidationIssue{{Code: "missing_core", Message: "missing source content"}},
			Stage:     "validation",
		}},
		CompletenessIssues: []ValidationIssue{{Code: "unexpected_accounted_candidate_count", Message: "incomplete full run"}},
	}

	err := run(context.Background(), cfg, testDependencies(t, result))
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "failure") {
		t.Fatalf("run error = %v, want nonzero candidate/completeness failure", err)
	}
}

func TestRunBlocksDatabaseBeforePersistenceWhenQualityGateFails(t *testing.T) {
	cfg := testConfig(t)
	failed := validRawCandidate()
	failed.Name = "Unresolved"
	result := ExtractionResult{
		Failed: []ExtractionFailure{{
			Candidate: failed,
			Issues:    []ValidationIssue{{Code: "missing_core", Message: "missing source content"}},
			Stage:     "validation",
		}},
		CompletenessIssues: []ValidationIssue{{Code: "unexpected_accounted_candidate_count", Message: "incomplete full run"}},
	}
	deps := testDependencies(t, result)
	databaseCalls := 0
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) {
		databaseCalls++
		return nil, errors.New("quality-gated run must not connect")
	}
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error {
		databaseCalls++
		return errors.New("quality-gated run must not apply schema")
	}
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error {
		databaseCalls++
		return errors.New("quality-gated run must not guard database")
	}
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error {
		databaseCalls++
		return errors.New("quality-gated run must not persist")
	}

	err := run(context.Background(), cfg, deps)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "failure") {
		t.Fatalf("run error = %v, want quality-gate failure", err)
	}
	if databaseCalls != 0 {
		t.Fatalf("database calls = %d, want none before quality gate passes", databaseCalls)
	}
}

func TestRunRedactsAPIKeyFromExtractionFailureArtifacts(t *testing.T) {
	cfg := testConfig(t)
	cfg.DryRun = true
	cfg.APIKey = "extraction-secret"
	failed := validRawCandidate()
	failed.Name = "Secret Failure"
	result := ExtractionResult{Failed: []ExtractionFailure{{
		Candidate: failed,
		Issues:    []ValidationIssue{{Code: "provider_failure", Message: "provider rejected extraction-secret"}},
		Stage:     "extract",
	}}}

	if err := run(context.Background(), cfg, testDependencies(t, result)); err == nil {
		t.Fatal("run succeeded despite a failed candidate")
	}
	for _, name := range []string{"review.json", "report.json"} {
		contents, err := os.ReadFile(filepath.Join(cfg.RunRoot, cfg.RunID, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(contents), cfg.APIKey) {
			t.Fatalf("%s contains the API key", name)
		}
	}
}

func TestRunCountsExtractionFailuresOnceAfterDatabaseLifecycle(t *testing.T) {
	cfg := testConfig(t)
	failed := validRawCandidate()
	failed.Name = "Unresolved"
	result := ExtractionResult{Failed: []ExtractionFailure{{
		Candidate: failed,
		Issues:    []ValidationIssue{{Code: "missing_core", Message: "missing source content"}},
		Stage:     "validation",
	}}}
	deps := testDependencies(t, result)
	deps.Connect = func(context.Context) (*pgxpool.Pool, error) { return nil, nil }
	deps.ApplySchema = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.EnsureEmptyCatalog = func(context.Context, *pgxpool.Pool) error { return nil }
	deps.Persist = func(context.Context, *pgxpool.Pool, NormalizedCandidate) error { return nil }

	if err := run(context.Background(), cfg, deps); err == nil {
		t.Fatal("run succeeded, want extraction failure")
	}
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	var report RunReport
	if err := store.ReadJSON("report.json", &report); err != nil {
		t.Fatalf("read report: %v", err)
	}
	if report.FailureCount != 1 || len(report.Failures) != 1 {
		t.Fatalf("failure accounting = %#v, want one extraction failure", report)
	}
}

func testConfig(t *testing.T) Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.PDFPath = "catalog.pdf"
	cfg.RunID = "run-1"
	cfg.RunRoot = t.TempDir()
	cfg.SelectedPages = []int{1}
	cfg.APIURL = "https://api.openai.com"
	cfg.APIKey = "test-key"
	return cfg
}

func testExtractionResult(t *testing.T, name string) ExtractionResult {
	t.Helper()
	candidate := validRawCandidate()
	candidate.Name = name
	candidate.SourcePages = []int{1}
	normalized, issues := Normalize(candidate)
	if len(issues) != 0 {
		t.Fatalf("Normalize fixture: %#v", issues)
	}
	return ExtractionResult{
		Accepted:            []NormalizedCandidate{normalized},
		LogicalRequests:     3,
		TextCalls:           2,
		ImageCalls:          1,
		ReconciliationCalls: 1,
		APICalls:            4,
	}
}

func testDependencies(t *testing.T, result ExtractionResult) Dependencies {
	t.Helper()
	return Dependencies{
		Validate:      func(context.Context, Config) error { return nil },
		OpenArtifacts: NewArtifactStore,
		Render: func(context.Context, Config, string) ([]Page, error) {
			return []Page{{Number: 1, ImagePath: "page-001.png"}}, nil
		},
		OCR: func(context.Context, []Page) ([]OCRPage, error) {
			return []OCRPage{{Number: 1, Text: "OCR fixture"}}, nil
		},
		Extract: func(context.Context, []Page, []OCRPage, Config) ExtractionResult { return result },
		Connect: func(context.Context) (*pgxpool.Pool, error) { return nil, nil },
		ApplySchema: func(context.Context, *pgxpool.Pool) error {
			return nil
		},
		EnsureEmptyCatalog: func(context.Context, *pgxpool.Pool) error {
			return nil
		},
		Persist: func(context.Context, *pgxpool.Pool, NormalizedCandidate) error {
			return nil
		},
	}
}

func writeResumeArtifacts(t *testing.T, cfg Config, result ExtractionResult) {
	t.Helper()
	store, err := NewArtifactStore(cfg.RunRoot, cfg.RunID)
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	raw := make([]RawCandidate, 0, len(result.Accepted))
	normalized := make([]NormalizedCandidate, 0, len(result.Accepted))
	for _, candidate := range result.Accepted {
		raw = append(raw, candidate.Raw)
		normalized = append(normalized, candidate)
	}
	report := RunReport{
		RunID:                   cfg.RunID,
		PDFPath:                 cfg.PDFPath,
		SelectedPages:           append([]int(nil), cfg.SelectedPages...),
		PageCount:               1,
		CandidateCount:          len(raw),
		NormalizedCount:         len(normalized),
		TextCalls:               result.TextCalls,
		ImageCalls:              result.ImageCalls,
		ReconciliationCalls:     result.ReconciliationCalls,
		APICalls:                result.APICalls,
		LogicalRequests:         result.LogicalRequests,
		DatabaseResumeSupported: false,
	}
	if err := store.WriteJSON("ocr.json", []OCRPage{{Number: 1, Text: "saved OCR"}}); err != nil {
		t.Fatalf("write OCR: %v", err)
	}
	if err := store.WriteJSON("raw_candidates.json", raw); err != nil {
		t.Fatalf("write raw candidates: %v", err)
	}
	if err := store.WriteJSON("normalized.json", normalized); err != nil {
		t.Fatalf("write normalized candidates: %v", err)
	}
	if err := store.WriteJSON("review.json", ReviewArtifact{}); err != nil {
		t.Fatalf("write review: %v", err)
	}
	if err := store.WriteJSON("report.json", report); err != nil {
		t.Fatalf("write report: %v", err)
	}
}
