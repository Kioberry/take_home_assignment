// Command extract runs the PDF-to-ontology extraction pipeline.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"oddities/database"
	"oddities/database/generated"
	"oddities/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

const pdfPath = "data/items_combined.pdf"

type Dependencies struct {
	Validate           func(context.Context, Config) error
	OpenArtifacts      func(string, string) (*ArtifactStore, error)
	Render             func(context.Context, Config, string) ([]Page, error)
	OCR                func(context.Context, []Page) ([]OCRPage, error)
	Extract            func(context.Context, []Page, []OCRPage, Config) ExtractionResult
	Connect            func(context.Context) (*pgxpool.Pool, error)
	ApplySchema        func(context.Context, *pgxpool.Pool) error
	EnsureEmptyCatalog func(context.Context, *pgxpool.Pool) error
	Persist            func(context.Context, *pgxpool.Pool, NormalizedCandidate) error
}

func main() {
	if err := loadDotEnv(".env"); err != nil {
		fatal("configuration: %v", err)
	}
	cfg, err := parseConfig(os.Args[1:], os.Getenv)
	if err != nil {
		fatal("configuration: %v", err)
	}
	if err := run(context.Background(), cfg, defaultDependencies()); err != nil {
		fatal("extraction failed: %v", err)
	}
}

func parseConfig(args []string, getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg := DefaultConfig()
	cfg.PDFPath = pdfPath
	cfg.RunRoot = "tmp/extraction"
	cfg.APIURL = "https://api.openai.com"
	if value := strings.TrimSpace(getenv("OPENAI_API_URL")); value != "" {
		cfg.APIURL = value
	}
	if value := strings.TrimSpace(getenv("OPENAI_TEXT_MODEL")); value != "" {
		cfg.TextModel = value
	}
	if value := strings.TrimSpace(getenv("OPENAI_VISION_MODEL")); value != "" {
		cfg.VisionModel = value
	}
	cfg.APIKey = strings.TrimSpace(getenv("OPENAI_API_KEY"))

	flags := flag.NewFlagSet("extract", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var pageRange, runID, resumeRun string
	flags.StringVar(&cfg.PDFPath, "pdf", cfg.PDFPath, "source PDF path")
	flags.StringVar(&cfg.RunRoot, "run-root", cfg.RunRoot, "artifact root directory")
	flags.StringVar(&runID, "run-id", "", "new run identifier")
	flags.StringVar(&resumeRun, "resume-run", "", "resume an existing artifact run")
	flags.StringVar(&pageRange, "pages", "", "selected page or page range, for example 1-5")
	flags.BoolVar(&cfg.DryRun, "dry-run", false, "write artifacts without connecting to the database")
	flags.IntVar(&cfg.BatchSize, "batch-size", cfg.BatchSize, "AI extraction batch size")
	flags.IntVar(&cfg.Overlap, "overlap", cfg.Overlap, "overlap between extraction batches")
	flags.IntVar(&cfg.ExpectedPages, "expected-pages", cfg.ExpectedPages, "expected pages for a full run")
	flags.IntVar(&cfg.ExpectedItems, "expected-items", cfg.ExpectedItems, "expected items for a full run")
	flags.StringVar(&cfg.APIURL, "api-url", cfg.APIURL, "OpenAI-compatible API base URL")
	flags.StringVar(&cfg.TextModel, "text-model", cfg.TextModel, "text extraction model")
	flags.StringVar(&cfg.VisionModel, "vision-model", cfg.VisionModel, "vision recovery model")
	flags.IntVar(&cfg.MaxTextAttempts, "max-text-attempts", cfg.MaxTextAttempts, "maximum API attempts per request")
	flags.IntVar(&cfg.MaxSemanticRetries, "max-semantic-retries", cfg.MaxSemanticRetries, "maximum image recovery requests")
	flags.IntVar(&cfg.MaxReconciliationRequests, "max-reconciliation-requests", cfg.MaxReconciliationRequests, "maximum reconciliation requests")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if flags.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if runID != "" && resumeRun != "" {
		return Config{}, errors.New("--run-id and --resume-run cannot be used together")
	}
	if resumeRun != "" {
		cfg.Resume = true
		cfg.RunID = resumeRun
	} else if runID != "" {
		cfg.RunID = runID
	} else {
		cfg.RunID = time.Now().UTC().Format("20060102T150405.000000000Z")
	}
	if pageRange != "" {
		pages, err := ParsePageRange(pageRange, cfg.ExpectedPages)
		if err != nil {
			return Config{}, err
		}
		cfg.SelectedPages = pages
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("API key is required: set OPENAI_API_KEY in the environment or .env")
	}
	if cfg.BatchSize < 1 {
		return Config{}, fmt.Errorf("batch size must be at least 1, got %d", cfg.BatchSize)
	}
	if cfg.Overlap < 0 {
		return Config{}, fmt.Errorf("overlap must not be negative, got %d", cfg.Overlap)
	}
	if cfg.Overlap >= cfg.BatchSize {
		return Config{}, fmt.Errorf("overlap must be smaller than batch size (%d >= %d)", cfg.Overlap, cfg.BatchSize)
	}
	if cfg.ExpectedPages < 1 || cfg.ExpectedItems < 1 || cfg.MaxTextAttempts < 1 || cfg.MaxSemanticRetries < 0 || cfg.MaxReconciliationRequests < 0 {
		return Config{}, errors.New("expected counts and retry limits are invalid")
	}
	return cfg, nil
}

func run(ctx context.Context, cfg Config, deps Dependencies) error {
	if err := validateDependencies(deps); err != nil {
		return err
	}
	if err := deps.Validate(ctx, cfg); err != nil {
		return safeError(err, cfg.APIKey)
	}
	store, err := deps.OpenArtifacts(cfg.RunRoot, cfg.RunID)
	if err != nil {
		return safeError(fmt.Errorf("create artifact run: %w", err), cfg.APIKey)
	}

	var pages []Page
	var ocr []OCRPage
	var result ExtractionResult
	if cfg.Resume {
		pages, ocr, result, err = loadResumeArtifacts(store, &cfg)
		if err != nil {
			return safeError(err, cfg.APIKey)
		}
	} else {
		pages, err = deps.Render(ctx, cfg, filepath.Join(store.Root(), "pages"))
		if err != nil {
			return safeError(fmt.Errorf("render pages: %w", err), cfg.APIKey)
		}
		ocr, err = deps.OCR(ctx, pages)
		if err != nil {
			return safeError(fmt.Errorf("OCR pages: %w", err), cfg.APIKey)
		}
		result = deps.Extract(ctx, pages, ocr, cfg)
	}
	result = redactExtractionResult(result, cfg.APIKey)

	report := newRunReport(cfg, pages, result)
	report.Resumed = cfg.Resume
	if err := writePreDBArtifacts(store, ocr, result, &report); err != nil {
		return safeError(fmt.Errorf("write pre-database artifacts: %w", err), cfg.APIKey)
	}
	fmt.Fprint(os.Stdout, formatRunSummary(
		report,
		result,
		filepath.Join(store.Root(), "review.json"),
		filepath.Join(store.Root(), "report.json"),
	))
	if cfg.DryRun {
		return runResultError(report)
	}

	pool, err := deps.Connect(ctx)
	if err != nil {
		return terminalReportError(store, &report, "connect", fmt.Errorf("connect database: %w", err), cfg.APIKey)
	}
	if pool != nil {
		defer pool.Close()
	}
	if err := deps.ApplySchema(ctx, pool); err != nil {
		return terminalReportError(store, &report, "schema", fmt.Errorf("apply database schema: %w", err), cfg.APIKey)
	}
	if err := deps.EnsureEmptyCatalog(ctx, pool); err != nil {
		return terminalReportError(store, &report, "empty_guard", fmt.Errorf("database guard: %w", err), cfg.APIKey)
	}

	candidates := append([]NormalizedCandidate(nil), result.Accepted...)
	candidates = append(candidates, result.Review...)
	for _, candidate := range candidates {
		if err := deps.Persist(ctx, pool, candidate); err != nil {
			report.Failures = append(report.Failures, RunFailure{
				CandidateName: candidate.Raw.Name,
				CandidateKey:  CandidateKey(candidate.Raw),
				Stage:         "persistence",
				Error:         safeError(err, cfg.APIKey).Error(),
			})
			continue
		}
		report.InsertedCount++
	}
	report.FailureCount = len(report.Failures)
	if err := store.WriteJSON("report.json", report); err != nil {
		return safeError(fmt.Errorf("write final report: %w", err), cfg.APIKey)
	}
	return runResultError(report)
}

func validateDependencies(deps Dependencies) error {
	missing := make([]string, 0)
	if deps.Validate == nil {
		missing = append(missing, "validate dependency is required")
	}
	if deps.OpenArtifacts == nil {
		missing = append(missing, "artifact dependency is required")
	}
	if deps.Render == nil {
		missing = append(missing, "render dependency is required")
	}
	if deps.OCR == nil {
		missing = append(missing, "OCR dependency is required")
	}
	if deps.Extract == nil {
		missing = append(missing, "extraction dependency is required")
	}
	if deps.Connect == nil {
		missing = append(missing, "connect dependency is required")
	}
	if deps.ApplySchema == nil {
		missing = append(missing, "schema dependency is required")
	}
	if deps.EnsureEmptyCatalog == nil {
		missing = append(missing, "empty catalog guard dependency is required")
	}
	if deps.Persist == nil {
		missing = append(missing, "persistence dependency is required")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing injected dependencies: %s", strings.Join(missing, ", "))
	}
	return nil
}

func terminalReportError(store *ArtifactStore, report *RunReport, stage string, err error, secret string) error {
	safe := safeError(err, secret)
	report.Failures = append(report.Failures, RunFailure{Stage: stage, Error: safe.Error()})
	report.FailureCount = len(report.Failures)
	if writeErr := store.WriteJSON("report.json", *report); writeErr != nil {
		return fmt.Errorf("%s; write terminal report: %v", safe, safeError(writeErr, secret))
	}
	return fmt.Errorf("%s: %s", stage, safe)
}

func defaultDependencies() Dependencies {
	runner := execCommandRunner{}
	return Dependencies{
		Validate: func(_ context.Context, cfg Config) error {
			if _, err := os.Stat(cfg.PDFPath); err != nil {
				return fmt.Errorf("open %s: %w", cfg.PDFPath, err)
			}
			for _, binary := range []string{"pdftoppm", "tesseract"} {
				if _, err := exec.LookPath(binary); err != nil {
					return fmt.Errorf("required binary %q is unavailable: %w", binary, err)
				}
			}
			return nil
		},
		OpenArtifacts: NewArtifactStore,
		Render: func(ctx context.Context, cfg Config, outputDir string) ([]Page, error) {
			return RenderPages(ctx, runner, cfg.PDFPath, outputDir, cfg.SelectedPages)
		},
		OCR: func(ctx context.Context, pages []Page) ([]OCRPage, error) {
			return OCRPages(ctx, runner, pages)
		},
		Extract: func(ctx context.Context, pages []Page, ocr []OCRPage, cfg Config) ExtractionResult {
			ai := NewOpenAIExtractor(newOpenAIHTTPClient(), cfg.APIURL, cfg.APIKey, cfg.TextModel, cfg.VisionModel, cfg.MaxTextAttempts)
			return RunExtraction(ctx, ai, pages, ocr, cfg)
		},
		Connect: db.Connect,
		ApplySchema: func(ctx context.Context, pool *pgxpool.Pool) error {
			return db.Apply(ctx, pool, database.Schema())
		},
		EnsureEmptyCatalog: func(ctx context.Context, pool *pgxpool.Pool) error {
			return EnsureEmptyCatalog(ctx, generated.New(pool))
		},
		Persist: func(ctx context.Context, pool *pgxpool.Pool, candidate NormalizedCandidate) error {
			_, err := PersistCandidate(ctx, pool, candidate)
			return err
		},
	}
}

func runResultError(report RunReport) error {
	if report.FailureCount == 0 && len(report.CompletenessIssues) == 0 {
		return nil
	}
	return fmt.Errorf("extraction completed with %d failure(s) and %d completeness issue(s)", report.FailureCount, len(report.CompletenessIssues))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
