package main

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type scriptedAI struct {
	t         *testing.T
	responses [][]RawCandidate
	requests  []ExtractRequest
}

func (ai *scriptedAI) Extract(_ context.Context, request ExtractRequest) ([]RawCandidate, error) {
	ai.t.Helper()
	ai.requests = append(ai.requests, cloneRequest(request))
	if len(ai.responses) == 0 {
		return nil, fmt.Errorf("unexpected AI call: %#v", request)
	}
	response := ai.responses[0]
	ai.responses = ai.responses[1:]
	return response, nil
}

func TestRunExtractionBatchesOCRWithOnePageOverlapAndDoesNotRecoverValidCandidates(t *testing.T) {
	first := pipelineCandidate("First", 1)
	second := pipelineCandidate("Second", 6)
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{first}, {second}}}

	result := RunExtraction(context.Background(), ai, pipelinePages(1, 9), pipelineOCR(1, 9), Config{
		SelectedPages: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
		BatchSize:     5,
		Overlap:       1,
	})

	if len(ai.requests) != 2 {
		t.Fatalf("AI calls = %d, want 2", len(ai.requests))
	}
	assertRequestPages(t, ai.requests[0].OCR, []int{1, 2, 3, 4, 5})
	assertRequestPages(t, ai.requests[1].OCR, []int{5, 6, 7, 8, 9})
	for index, request := range ai.requests {
		if request.Mode != "extract" || len(request.Images) != 0 || len(request.Prior) != 0 || len(request.Issues) != 0 {
			t.Fatalf("request %d = %#v, want text-only extraction", index, request)
		}
	}
	if len(result.Accepted) != 2 || len(result.Review) != 0 || len(result.Failed) != 0 {
		t.Fatalf("buckets = accepted:%d review:%d failed:%d, want 2/0/0", len(result.Accepted), len(result.Review), len(result.Failed))
	}
	if result.TextCalls != 2 || result.ImageCalls != 0 || result.ReconciliationCalls != 0 || result.APICalls != 2 {
		t.Fatalf("call accounting = %#v, want 2 text calls only", result)
	}
}

func TestRunExtractionAppliesDefaultBatchAndOverlapForZeroConfig(t *testing.T) {
	first := pipelineCandidate("First", 1)
	second := pipelineCandidate("Second", 6)
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{first}, {second}}}

	_ = RunExtraction(context.Background(), ai, pipelinePages(1, 9), pipelineOCR(1, 9), Config{})

	if len(ai.requests) != 2 {
		t.Fatalf("AI calls = %d, want 2 default-sized batches", len(ai.requests))
	}
	assertRequestPages(t, ai.requests[0].OCR, []int{1, 2, 3, 4, 5})
	assertRequestPages(t, ai.requests[1].OCR, []int{5, 6, 7, 8, 9})
}

func TestRunExtractionAppliesDefaultRecoveryLimitsForZeroConfig(t *testing.T) {
	textInvalid := pipelineCandidate("Unclear", 1)
	textInvalid.RarityRaw = "mythic"
	imageInvalid := pipelineCandidate("Unclear", 1)
	imageInvalid.RarityRaw = "unknown"
	reconciledInvalid := pipelineCandidate("Unclear", 1)
	reconciledInvalid.RarityRaw = "unresolved"
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{textInvalid}, {imageInvalid}, {reconciledInvalid}}}

	result := RunExtraction(context.Background(), ai, pipelinePages(1, 1), pipelineOCR(1, 1), Config{SelectedPages: []int{1}})

	if len(ai.requests) != 3 || result.TextCalls != 1 || result.ImageCalls != 1 || result.ReconciliationCalls != 1 || result.APICalls != 3 {
		t.Fatalf("default recovery accounting = requests:%d result:%#v, want one text, image, and reconciliation call", len(ai.requests), result)
	}
}

func TestRunExtractionRecordsNegativeConfigurationInsteadOfDefaulting(t *testing.T) {
	ai := &scriptedAI{t: t}
	result := RunExtraction(context.Background(), ai, pipelinePages(1, 1), pipelineOCR(1, 1), Config{BatchSize: -1})

	if !hasIssueCode(result.CompletenessIssues, "invalid_config") {
		t.Fatalf("configuration issues = %#v, want invalid_config", result.CompletenessIssues)
	}
	if len(ai.requests) != 0 {
		t.Fatalf("AI calls = %d, want none for invalid config", len(ai.requests))
	}
}

func TestRunExtractionRecoversOnlyInvalidCandidateWithSourceAndAdjacentImages(t *testing.T) {
	invalid := pipelineCandidate("Blurred", 2)
	invalid.RarityRaw = "mythic"
	fixed := pipelineCandidate("Blurred", 2)
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{invalid}, {fixed}}}

	result := RunExtraction(context.Background(), ai, pipelinePages(1, 4), pipelineOCR(1, 4), Config{
		SelectedPages:      []int{1, 2, 3, 4},
		BatchSize:          5,
		Overlap:            1,
		MaxSemanticRetries: 1,
	})

	if len(ai.requests) != 2 {
		t.Fatalf("AI calls = %d, want text and one image recovery", len(ai.requests))
	}
	imageRequest := ai.requests[1]
	if imageRequest.Mode != "recover" {
		t.Fatalf("image request mode = %q, want recover", imageRequest.Mode)
	}
	assertRequestPages(t, imageRequest.Images, []int{1, 2, 3})
	if len(imageRequest.Prior) != 1 || imageRequest.Prior[0].Name != "Blurred" || len(imageRequest.Issues) == 0 {
		t.Fatalf("image request = %#v, want only invalid candidate and its issues", imageRequest)
	}
	if len(result.Accepted) != 1 || len(result.Review) != 0 || len(result.Failed) != 0 {
		t.Fatalf("buckets = accepted:%d review:%d failed:%d, want 1/0/0", len(result.Accepted), len(result.Review), len(result.Failed))
	}
	if result.TextCalls != 1 || result.ImageCalls != 1 || result.ReconciliationCalls != 0 || result.APICalls != 2 {
		t.Fatalf("call accounting = %#v, want one text and one image call", result)
	}
}

func TestRunExtractionRoutesMergeConflictToReviewWithoutImageRecovery(t *testing.T) {
	first := pipelineCandidate("Conflicted", 1)
	first.RarityRaw = "rare"
	second := pipelineCandidate("conflicted", 1)
	second.RarityRaw = "legendary"
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{first, second}}}

	result := RunExtraction(context.Background(), ai, pipelinePages(1, 1), pipelineOCR(1, 1), Config{
		SelectedPages:      []int{1},
		BatchSize:          5,
		Overlap:            1,
		MaxSemanticRetries: 1,
	})

	if len(ai.requests) != 1 {
		t.Fatalf("AI calls = %d, want text extraction only", len(ai.requests))
	}
	if len(result.Accepted) != 0 || len(result.Review) != 1 || len(result.Failed) != 0 {
		t.Fatalf("buckets = accepted:%d review:%d failed:%d, want 0/1/0", len(result.Accepted), len(result.Review), len(result.Failed))
	}
	if !strings.Contains(strings.Join(result.Review[0].ReviewReasons, "\n"), "conflicting rarity_raw") {
		t.Fatalf("review reasons = %#v, want merge conflict", result.Review[0].ReviewReasons)
	}
}

func TestRunExtractionReconcilesOnceThenFailsWithoutFourthAPICall(t *testing.T) {
	textInvalid := pipelineCandidate("Unclear", 2)
	textInvalid.RarityRaw = "mythic"
	imageInvalid := pipelineCandidate("Unclear", 2)
	imageInvalid.RarityRaw = "unknown"
	reconciledInvalid := pipelineCandidate("Unclear", 2)
	reconciledInvalid.RarityRaw = "unresolved"
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{textInvalid}, {imageInvalid}, {reconciledInvalid}}}

	result := RunExtraction(context.Background(), ai, pipelinePages(1, 4), pipelineOCR(1, 4), Config{
		SelectedPages:             []int{1, 2, 3, 4},
		BatchSize:                 5,
		Overlap:                   1,
		MaxSemanticRetries:        1,
		MaxReconciliationRequests: 1,
	})

	if len(ai.requests) != 3 {
		t.Fatalf("AI calls = %d, want text, image, and one reconciliation call", len(ai.requests))
	}
	reconcile := ai.requests[2]
	if reconcile.Mode != "reconcile" || len(reconcile.OCR) == 0 || len(reconcile.Images) == 0 || len(reconcile.Prior) != 1 || len(reconcile.Issues) == 0 {
		t.Fatalf("reconciliation request = %#v, want OCR, images, prior candidate, and issues", reconcile)
	}
	if len(result.Accepted) != 0 || len(result.Review) != 0 || len(result.Failed) != 1 {
		t.Fatalf("buckets = accepted:%d review:%d failed:%d, want 0/0/1", len(result.Accepted), len(result.Review), len(result.Failed))
	}
	if got := result.Failed[0]; got.Candidate.Name != "Unclear" || got.Stage != "reconciliation" || len(got.Issues) == 0 {
		t.Fatalf("failure = %#v, want precise reconciliation failure for Unclear", got)
	}
	if result.TextCalls != 1 || result.ImageCalls != 1 || result.ReconciliationCalls != 1 || result.APICalls != 3 {
		t.Fatalf("call accounting = %#v, want exactly three calls", result)
	}
	if len(ai.responses) != 0 {
		t.Fatalf("unused scripted responses = %d, pipeline made an unexpected number of calls", len(ai.responses))
	}
}

func TestRunExtractionReconciliationUsesLatestCandidateSourcePages(t *testing.T) {
	textInvalid := pipelineCandidate("Shifted", 2)
	textInvalid.RarityRaw = "mythic"
	imageInvalid := pipelineCandidate("Shifted", 3)
	imageInvalid.RarityRaw = "unknown"
	reconciled := pipelineCandidate("Shifted", 3)
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{textInvalid}, {imageInvalid}, {reconciled}}}

	result := RunExtraction(context.Background(), ai, pipelinePages(1, 4), pipelineOCR(1, 4), Config{
		SelectedPages:             []int{1, 2, 3, 4},
		BatchSize:                 5,
		Overlap:                   1,
		MaxSemanticRetries:        1,
		MaxReconciliationRequests: 1,
	})

	if len(result.Accepted) != 1 || len(result.Failed) != 0 {
		t.Fatalf("buckets = accepted:%d failed:%d, want 1/0", len(result.Accepted), len(result.Failed))
	}
	if len(ai.requests) != 3 {
		t.Fatalf("AI calls = %d, want 3", len(ai.requests))
	}
	assertRequestPages(t, ai.requests[1].Images, []int{1, 2, 3})
	assertRequestPages(t, ai.requests[2].Images, []int{2, 3, 4})
}

func TestRunExtractionAccountsForEveryCandidateInExactlyOneBucket(t *testing.T) {
	accepted := pipelineCandidate("Accepted", 1)
	review := pipelineCandidate("Review", 1)
	review.Confidence = 0.5
	invalid := pipelineCandidate("Failed", 1)
	invalid.RarityRaw = "mythic"
	stillInvalid := pipelineCandidate("Failed", 1)
	stillInvalid.RarityRaw = "unknown"
	finalInvalid := pipelineCandidate("Failed", 1)
	finalInvalid.RarityRaw = "unresolved"
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{accepted, review, invalid}, {stillInvalid}, {finalInvalid}}}

	result := RunExtraction(context.Background(), ai, pipelinePages(1, 1), pipelineOCR(1, 1), Config{
		SelectedPages:             []int{1},
		BatchSize:                 5,
		Overlap:                   1,
		MaxSemanticRetries:        1,
		MaxReconciliationRequests: 1,
	})

	if len(result.Accepted) != 1 || len(result.Review) != 1 || len(result.Failed) != 1 {
		t.Fatalf("buckets = accepted:%d review:%d failed:%d, want 1/1/1", len(result.Accepted), len(result.Review), len(result.Failed))
	}
	names := []string{result.Accepted[0].Raw.Name, result.Review[0].Raw.Name, result.Failed[0].Candidate.Name}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"Accepted", "Failed", "Review"}) {
		t.Fatalf("bucket names = %#v, want each original candidate exactly once", names)
	}
}

func TestRunExtractionSkipsGlobalCompletenessForPartialPages(t *testing.T) {
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{pipelineCandidate("Partial", 1)}}}
	result := RunExtraction(context.Background(), ai, pipelinePages(1, 1), pipelineOCR(1, 1), Config{
		SelectedPages: []int{1},
		BatchSize:     5,
		Overlap:       1,
		ExpectedPages: 39,
		ExpectedItems: 80,
	})

	if len(result.CompletenessIssues) != 0 {
		t.Fatalf("partial run completeness issues = %#v, want none", result.CompletenessIssues)
	}
}

func TestRunExtractionReportsFullRunCompletenessFailures(t *testing.T) {
	ai := &scriptedAI{t: t, responses: [][]RawCandidate{{}}}
	result := RunExtraction(context.Background(), ai, pipelinePages(1, 1), pipelineOCR(1, 1), Config{
		BatchSize:     5,
		Overlap:       1,
		ExpectedPages: 39,
		ExpectedItems: 80,
	})

	for _, code := range []string{"unexpected_page_count", "unexpected_accounted_candidate_count", "missing_cross_page_span"} {
		if !hasIssueCode(result.CompletenessIssues, code) {
			t.Fatalf("completeness issues = %#v, want %q", result.CompletenessIssues, code)
		}
	}
}

func TestRunExtractionUsesProviderAttemptMetricsWhenAvailable(t *testing.T) {
	provider := &meteredScriptedAI{
		scriptedAI: scriptedAI{t: t, responses: [][]RawCandidate{{pipelineCandidate("Retried", 1)}}},
		metrics:    []AICallCounts{{TextCalls: 2, APICalls: 2}},
	}

	result := RunExtraction(context.Background(), provider, pipelinePages(1, 1), pipelineOCR(1, 1), Config{SelectedPages: []int{1}})

	if result.TextCalls != 2 || result.ImageCalls != 0 || result.ReconciliationCalls != 0 || result.APICalls != 2 {
		t.Fatalf("provider attempt accounting = %#v, want two actual text attempts", result)
	}
	if result.LogicalRequests != 1 {
		t.Fatalf("logical request accounting = %d, want one request", result.LogicalRequests)
	}
}

func TestRunExtractionPerCallMetricsDoNotCrossContaminateConcurrentRuns(t *testing.T) {
	provider := &concurrentMetricsAI{
		entered: make(chan struct{}, 2),
		release: make(chan struct{}),
		responses: map[string]RawCandidate{
			"first":  pipelineCandidate("First", 1),
			"second": pipelineCandidate("Second", 1),
		},
		metrics: map[string]AICallCounts{
			"first":  {TextCalls: 2, APICalls: 2},
			"second": {TextCalls: 3, APICalls: 3},
		},
	}
	firstResult := make(chan ExtractionResult, 1)
	secondResult := make(chan ExtractionResult, 1)
	go func() {
		firstResult <- RunExtraction(context.Background(), provider, pipelinePages(1, 1), []OCRPage{{Number: 1, Text: "first"}}, Config{SelectedPages: []int{1}})
	}()
	go func() {
		secondResult <- RunExtraction(context.Background(), provider, pipelinePages(1, 1), []OCRPage{{Number: 1, Text: "second"}}, Config{SelectedPages: []int{1}})
	}()
	<-provider.entered
	<-provider.entered
	close(provider.release)

	first := <-firstResult
	second := <-secondResult
	if first.TextCalls != 2 || first.APICalls != 2 {
		t.Fatalf("first run counts = %#v, want its two attempts only", first)
	}
	if second.TextCalls != 3 || second.APICalls != 3 {
		t.Fatalf("second run counts = %#v, want its three attempts only", second)
	}
}

func pipelineCandidate(name string, page int) RawCandidate {
	candidate := validRawCandidate()
	candidate.Name = name
	candidate.SourcePages = []int{page}
	return candidate
}

func pipelinePages(first, last int) []Page {
	pages := make([]Page, 0, last-first+1)
	for page := first; page <= last; page++ {
		pages = append(pages, Page{Number: page, ImagePath: fmt.Sprintf("page-%d.png", page)})
	}
	return pages
}

func pipelineOCR(first, last int) []OCRPage {
	ocr := make([]OCRPage, 0, last-first+1)
	for page := first; page <= last; page++ {
		ocr = append(ocr, OCRPage{Number: page, Text: fmt.Sprintf("OCR page %d", page)})
	}
	return ocr
}

func assertRequestPages[T interface{ Page | OCRPage }](t *testing.T, values []T, want []int) {
	t.Helper()
	got := make([]int, 0, len(values))
	for _, value := range values {
		switch page := any(value).(type) {
		case Page:
			got = append(got, page.Number)
		case OCRPage:
			got = append(got, page.Number)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("page numbers = %#v, want %#v", got, want)
	}
}

func cloneRequest(request ExtractRequest) ExtractRequest {
	return ExtractRequest{
		OCR:    append([]OCRPage(nil), request.OCR...),
		Images: append([]Page(nil), request.Images...),
		Prior:  append([]RawCandidate(nil), request.Prior...),
		Issues: append([]ValidationIssue(nil), request.Issues...),
		Mode:   request.Mode,
	}
}

func hasIssueCode(issues []ValidationIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

type meteredScriptedAI struct {
	scriptedAI
	metrics []AICallCounts
}

func (ai *meteredScriptedAI) ExtractWithMetrics(ctx context.Context, request ExtractRequest) ([]RawCandidate, AICallCounts, error) {
	candidates, err := ai.scriptedAI.Extract(ctx, request)
	if len(ai.metrics) == 0 {
		return candidates, AICallCounts{}, err
	}
	metrics := ai.metrics[0]
	ai.metrics = ai.metrics[1:]
	return candidates, metrics, err
}

type concurrentMetricsAI struct {
	entered   chan struct{}
	release   chan struct{}
	responses map[string]RawCandidate
	metrics   map[string]AICallCounts
}

func (ai *concurrentMetricsAI) Extract(ctx context.Context, request ExtractRequest) ([]RawCandidate, error) {
	candidates, _, err := ai.ExtractWithMetrics(ctx, request)
	return candidates, err
}

func (ai *concurrentMetricsAI) ExtractWithMetrics(_ context.Context, request ExtractRequest) ([]RawCandidate, AICallCounts, error) {
	if len(request.OCR) != 1 {
		return nil, AICallCounts{}, fmt.Errorf("OCR pages = %d, want 1", len(request.OCR))
	}
	key := request.OCR[0].Text
	ai.entered <- struct{}{}
	<-ai.release
	candidate, found := ai.responses[key]
	if !found {
		return nil, AICallCounts{}, fmt.Errorf("unexpected request %q", key)
	}
	return []RawCandidate{candidate}, ai.metrics[key], nil
}
