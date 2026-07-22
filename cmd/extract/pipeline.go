package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type extractionBatch struct {
	pages []Page
	ocr   []OCRPage
}

type candidateEvaluation struct {
	normalized NormalizedCandidate
	issues     []ValidationIssue
}

var requiredCrossPageSpans = map[string][]int{
	"exo-armor":             {7, 8, 9},
	"ring of elven lords":   {22, 23, 24},
	"war drum of the horde": {29, 30, 31},
	"amulet of encasement":  {38, 39},
}

// RunExtraction batches OCR extraction, deduplicates overlapping results, and
// applies at most one image recovery plus one reconciliation request per
// invalid candidate. It never makes recovery calls for already-valid records.
func RunExtraction(ctx context.Context, ai AIExtractor, pages []Page, ocr []OCRPage, cfg Config) (result ExtractionResult) {
	result = ExtractionResult{
		Accepted:           make([]NormalizedCandidate, 0),
		Review:             make([]NormalizedCandidate, 0),
		Failed:             make([]ExtractionFailure, 0),
		CompletenessIssues: make([]ValidationIssue, 0),
	}
	metrics, hasMetrics := ai.(AICallMetrics)
	var startingCalls AICallCounts
	if hasMetrics {
		startingCalls = metrics.CallCounts()
		defer func() {
			applyActualAICallCounts(&result, startingCalls, metrics.CallCounts())
		}()
	}
	if ai == nil {
		result.CompletenessIssues = append(result.CompletenessIssues, pipelineIssue("nil_ai_extractor", "AI extractor is required", false))
		return
	}
	resolvedConfig, configIssues := resolvePipelineConfig(cfg)
	if len(configIssues) > 0 {
		result.CompletenessIssues = append(result.CompletenessIssues, configIssues...)
		return
	}

	pageIndex := pageByNumber(pages)
	batches := extractionBatches(pages, ocr, resolvedConfig)
	rawBatches := make([][]RawCandidate, 0, len(batches))
	for _, batch := range batches {
		candidates, err := ai.Extract(ctx, ExtractRequest{OCR: batch.ocr, Mode: "extract"})
		result.TextCalls++
		result.APICalls++
		if err != nil {
			result.CompletenessIssues = append(result.CompletenessIssues, pipelineIssue("text_extraction_failed", fmt.Sprintf("text extraction for pages %s failed: %v", pageNumbers(batch.pages), err), false))
			continue
		}
		rawBatches = append(rawBatches, candidates)
	}

	merged, mergeIssues := MergeCandidates(rawBatches)
	for _, candidate := range merged {
		evaluation := evaluateCandidate(candidate, pageIndex, mergeIssues)
		if len(evaluation.issues) == 0 {
			addCandidateToBucket(&result, evaluation.normalized)
			continue
		}

		final, failure, calls := recoverCandidate(ctx, ai, candidate, evaluation.issues, pageIndex, ocr, resolvedConfig)
		result.ImageCalls += calls.image
		result.ReconciliationCalls += calls.reconciliation
		result.APICalls += calls.image + calls.reconciliation
		if failure != nil {
			result.Failed = append(result.Failed, *failure)
			continue
		}
		addCandidateToBucket(&result, final)
	}

	if len(resolvedConfig.SelectedPages) == 0 {
		result.CompletenessIssues = append(result.CompletenessIssues, completenessIssues(pages, result, resolvedConfig)...)
	}
	return
}

type recoveryCalls struct {
	image          int
	reconciliation int
}

func recoverCandidate(ctx context.Context, ai AIExtractor, original RawCandidate, initialIssues []ValidationIssue, pageIndex map[int]Page, ocr []OCRPage, cfg Config) (NormalizedCandidate, *ExtractionFailure, recoveryCalls) {
	calls := recoveryCalls{}
	selectedImages := recoveryPages(original.SourcePages, pageIndex)
	selectedOCR := ocrForPages(ocr, selectedImages)
	current := original
	issues := append([]ValidationIssue(nil), initialIssues...)

	if semanticRetryLimit(cfg) > 0 {
		response, err := ai.Extract(ctx, ExtractRequest{
			OCR:    selectedOCR,
			Images: selectedImages,
			Prior:  []RawCandidate{current},
			Issues: issues,
			Mode:   "recover",
		})
		calls.image++
		if err != nil {
			return NormalizedCandidate{}, recoveryFailure(current, "image", issues, err), calls
		}
		candidate, failure := recoveredCandidate(current, response, "image", issues)
		if failure != nil {
			return NormalizedCandidate{}, failure, calls
		}
		current = candidate
		evaluation := evaluateCandidate(current, pageIndex, nil)
		if len(evaluation.issues) == 0 {
			return evaluation.normalized, nil, calls
		}
		issues = evaluation.issues
		selectedImages = recoveryPages(current.SourcePages, pageIndex)
		selectedOCR = ocrForPages(ocr, selectedImages)
	}

	if reconciliationLimit(cfg) > 0 {
		response, err := ai.Extract(ctx, ExtractRequest{
			OCR:    selectedOCR,
			Images: selectedImages,
			Prior:  []RawCandidate{current},
			Issues: issues,
			Mode:   "reconcile",
		})
		calls.reconciliation++
		if err != nil {
			return NormalizedCandidate{}, recoveryFailure(current, "reconciliation", issues, err), calls
		}
		candidate, failure := recoveredCandidate(current, response, "reconciliation", issues)
		if failure != nil {
			return NormalizedCandidate{}, failure, calls
		}
		current = candidate
		evaluation := evaluateCandidate(current, pageIndex, nil)
		if len(evaluation.issues) == 0 {
			return evaluation.normalized, nil, calls
		}
		return NormalizedCandidate{}, &ExtractionFailure{Candidate: current, Issues: evaluation.issues, Stage: "reconciliation"}, calls
	}

	return NormalizedCandidate{}, &ExtractionFailure{Candidate: current, Issues: issues, Stage: "validation"}, calls
}

func recoveredCandidate(original RawCandidate, response []RawCandidate, stage string, issues []ValidationIssue) (RawCandidate, *ExtractionFailure) {
	if len(response) != 1 {
		issue := pipelineIssue("recovery_candidate_count", fmt.Sprintf("%s recovery for %q returned %d candidates, want exactly 1", stage, original.Name, len(response)), false)
		return RawCandidate{}, &ExtractionFailure{Candidate: original, Issues: append(append([]ValidationIssue(nil), issues...), issue), Stage: stage}
	}
	return response[0], nil
}

func recoveryFailure(candidate RawCandidate, stage string, issues []ValidationIssue, err error) *ExtractionFailure {
	issue := pipelineIssue(stage+"_request_failed", fmt.Sprintf("%s recovery for %q failed: %v", stage, candidate.Name, err), false)
	return &ExtractionFailure{Candidate: candidate, Issues: append(append([]ValidationIssue(nil), issues...), issue), Stage: stage}
}

func addCandidateToBucket(result *ExtractionResult, candidate NormalizedCandidate) {
	if candidate.NeedsReview {
		result.Review = append(result.Review, candidate)
		return
	}
	result.Accepted = append(result.Accepted, candidate)
}

func evaluateCandidate(candidate RawCandidate, pageIndex map[int]Page, mergeIssues []ValidationIssue) candidateEvaluation {
	normalized, issues := Normalize(candidate)
	issues = append(issues, Validate(normalized, highestPageNumber(pageIndex))...)
	issues = append(issues, unavailableSourcePageIssues(candidate.SourcePages, pageIndex)...)
	if candidate.Continuation {
		issues = append(issues, pipelineIssue("unresolved_continuation", fmt.Sprintf("candidate %q remains marked as a continuation", candidate.Name), true))
	}
	issues = append(issues, mergeIssuesForCandidate(candidate, mergeIssues)...)
	return candidateEvaluation{normalized: normalized, issues: issues}
}

func extractionBatches(pages []Page, ocr []OCRPage, cfg Config) []extractionBatch {
	sortedPages := append([]Page(nil), pages...)
	sort.Slice(sortedPages, func(i, j int) bool { return sortedPages[i].Number < sortedPages[j].Number })
	batchSize := cfg.BatchSize
	overlap := cfg.Overlap
	step := batchSize - overlap
	ocrIndex := make(map[int]OCRPage, len(ocr))
	for _, page := range ocr {
		ocrIndex[page.Number] = page
	}
	batches := make([]extractionBatch, 0)
	for start := 0; start < len(sortedPages); start += step {
		end := start + batchSize
		if end > len(sortedPages) {
			end = len(sortedPages)
		}
		batchPages := append([]Page(nil), sortedPages[start:end]...)
		batchOCR := make([]OCRPage, 0, len(batchPages))
		for _, page := range batchPages {
			if extracted, found := ocrIndex[page.Number]; found {
				batchOCR = append(batchOCR, extracted)
			}
		}
		batches = append(batches, extractionBatch{pages: batchPages, ocr: batchOCR})
		if end == len(sortedPages) {
			break
		}
	}
	return batches
}

func recoveryPages(sourcePages []int, pageIndex map[int]Page) []Page {
	requested := make(map[int]bool, len(sourcePages)+2)
	for _, page := range sourcePages {
		requested[page] = true
		if _, found := pageIndex[page-1]; found {
			requested[page-1] = true
		}
		if _, found := pageIndex[page+1]; found {
			requested[page+1] = true
		}
	}
	selected := make([]Page, 0, len(requested))
	for page := range requested {
		if image, found := pageIndex[page]; found {
			selected = append(selected, image)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Number < selected[j].Number })
	return selected
}

func ocrForPages(ocr []OCRPage, pages []Page) []OCRPage {
	wanted := make(map[int]bool, len(pages))
	for _, page := range pages {
		wanted[page.Number] = true
	}
	selected := make([]OCRPage, 0, len(pages))
	for _, page := range ocr {
		if wanted[page.Number] {
			selected = append(selected, page)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Number < selected[j].Number })
	return selected
}

func pageByNumber(pages []Page) map[int]Page {
	index := make(map[int]Page, len(pages))
	for _, page := range pages {
		index[page.Number] = page
	}
	return index
}

func highestPageNumber(pages map[int]Page) int {
	highest := 0
	for number := range pages {
		if number > highest {
			highest = number
		}
	}
	return highest
}

func unavailableSourcePageIssues(sourcePages []int, pageIndex map[int]Page) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	for _, page := range sourcePages {
		if _, found := pageIndex[page]; !found {
			issues = append(issues, pipelineIssue("source_page_not_loaded", fmt.Sprintf("source page %d was not loaded for this run", page), false))
		}
	}
	return issues
}

func mergeIssuesForCandidate(candidate RawCandidate, mergeIssues []ValidationIssue) []ValidationIssue {
	needle := fmt.Sprintf("candidate %q", candidate.Name)
	issues := make([]ValidationIssue, 0)
	for _, issue := range mergeIssues {
		if strings.Contains(issue.Message, needle) {
			issues = append(issues, issue)
		}
	}
	return issues
}

func completenessIssues(pages []Page, result ExtractionResult, cfg Config) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	expectedPages := cfg.ExpectedPages
	if expectedPages < 1 {
		expectedPages = DefaultConfig().ExpectedPages
	}
	if len(pages) != expectedPages {
		issues = append(issues, pipelineIssue("unexpected_page_count", fmt.Sprintf("full run loaded %d pages, want %d", len(pages), expectedPages), false))
	}
	expectedItems := cfg.ExpectedItems
	if expectedItems < 1 {
		expectedItems = DefaultConfig().ExpectedItems
	}
	accounted := len(result.Accepted) + len(result.Review) + len(result.Failed)
	if accounted != expectedItems {
		issues = append(issues, pipelineIssue("unexpected_accounted_candidate_count", fmt.Sprintf("full run accounted for %d candidates, want %d", accounted, expectedItems), false))
	}
	allCandidates := make([]RawCandidate, 0, accounted)
	for _, candidate := range result.Accepted {
		allCandidates = append(allCandidates, candidate.Raw)
	}
	for _, candidate := range result.Review {
		allCandidates = append(allCandidates, candidate.Raw)
	}
	for _, failure := range result.Failed {
		allCandidates = append(allCandidates, failure.Candidate)
	}
	for name, span := range requiredCrossPageSpans {
		if !hasSourceSpan(allCandidates, name, span) {
			issues = append(issues, pipelineIssue("missing_cross_page_span", fmt.Sprintf("full run is missing %s across pages %s", name, pageNumbersFromInts(span)), false))
		}
	}
	return issues
}

func hasSourceSpan(candidates []RawCandidate, name string, want []int) bool {
	for _, candidate := range candidates {
		if normalizeIdentity(candidate.Name) != name {
			continue
		}
		if equalPageNumbers(candidate.SourcePages, want) {
			return true
		}
	}
	return false
}

func equalPageNumbers(first, second []int) bool {
	first = uniqueSortedPages(first)
	second = uniqueSortedPages(second)
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

func semanticRetryLimit(cfg Config) int {
	if cfg.MaxSemanticRetries > 0 {
		return 1
	}
	return 0
}

func reconciliationLimit(cfg Config) int {
	if cfg.MaxReconciliationRequests > 0 {
		return 1
	}
	return 0
}

func resolvePipelineConfig(cfg Config) (Config, []ValidationIssue) {
	defaults := DefaultConfig()
	issues := make([]ValidationIssue, 0)
	defaultValue := func(field string, value *int, fallback int) {
		switch {
		case *value == 0:
			*value = fallback
		case *value < 0:
			issues = append(issues, pipelineIssue("invalid_config", fmt.Sprintf("%s must not be negative", field), false))
		}
	}
	defaultValue("batch_size", &cfg.BatchSize, defaults.BatchSize)
	defaultValue("overlap", &cfg.Overlap, defaults.Overlap)
	defaultValue("max_semantic_retries", &cfg.MaxSemanticRetries, defaults.MaxSemanticRetries)
	defaultValue("max_reconciliation_requests", &cfg.MaxReconciliationRequests, defaults.MaxReconciliationRequests)
	defaultValue("expected_pages", &cfg.ExpectedPages, defaults.ExpectedPages)
	defaultValue("expected_items", &cfg.ExpectedItems, defaults.ExpectedItems)
	if cfg.BatchSize > 0 && cfg.Overlap >= cfg.BatchSize {
		issues = append(issues, pipelineIssue("invalid_config", "overlap must be smaller than batch_size", false))
	}
	return cfg, issues
}

func applyActualAICallCounts(result *ExtractionResult, before, after AICallCounts) {
	result.TextCalls = nonNegativeDelta(after.TextCalls, before.TextCalls)
	result.ImageCalls = nonNegativeDelta(after.ImageCalls, before.ImageCalls)
	result.ReconciliationCalls = nonNegativeDelta(after.ReconciliationCalls, before.ReconciliationCalls)
	result.APICalls = nonNegativeDelta(after.APICalls, before.APICalls)
}

func nonNegativeDelta(after, before int) int {
	if after <= before {
		return 0
	}
	return after - before
}

func pipelineIssue(code, message string, recoverable bool) ValidationIssue {
	return ValidationIssue{Code: code, Message: message, Recoverable: recoverable}
}

func pageNumbers(pages []Page) string {
	numbers := make([]int, 0, len(pages))
	for _, page := range pages {
		numbers = append(numbers, page.Number)
	}
	return pageNumbersFromInts(numbers)
}

func pageNumbersFromInts(pages []int) string {
	parts := make([]string, 0, len(pages))
	for _, page := range pages {
		parts = append(parts, fmt.Sprint(page))
	}
	return strings.Join(parts, ",")
}
