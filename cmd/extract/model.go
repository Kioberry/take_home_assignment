package main

import "oddities/database/generated"

type Config struct {
	PDFPath                   string `json:"pdf_path"`
	RunID                     string `json:"run_id"`
	RunRoot                   string `json:"run_root"`
	SelectedPages             []int  `json:"selected_pages"`
	DryRun                    bool   `json:"dry_run"`
	Resume                    bool   `json:"resume"`
	BatchSize                 int    `json:"batch_size"`
	Overlap                   int    `json:"overlap"`
	ExpectedPages             int    `json:"expected_pages"`
	ExpectedItems             int    `json:"expected_items"`
	APIURL                    string `json:"api_url"`
	APIKey                    string `json:"-"`
	TextModel                 string `json:"text_model"`
	VisionModel               string `json:"vision_model"`
	MaxTextAttempts           int    `json:"max_text_attempts"`
	MaxSemanticRetries        int    `json:"max_semantic_retries"`
	MaxReconciliationRequests int    `json:"max_reconciliation_requests"`
}

func DefaultConfig() Config {
	return Config{
		BatchSize:                 5,
		Overlap:                   1,
		ExpectedPages:             39,
		ExpectedItems:             80,
		TextModel:                 "gpt-5.6-luna",
		VisionModel:               "gpt-5.6-terra",
		MaxTextAttempts:           2,
		MaxSemanticRetries:        1,
		MaxReconciliationRequests: 1,
	}
}

type Page struct {
	Number    int
	ImagePath string
}

type OCRPage struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

type RawEffect struct {
	CategoryRaw string `json:"category_raw"`
	Description string `json:"description"`
}

type RawLimitation struct {
	EffectIndex *int   `json:"effect_index"`
	Description string `json:"description"`
}

type RawCandidate struct {
	Name                  string          `json:"name"`
	SourcePages           []int           `json:"source_pages"`
	SourceItemTypeRaw     string          `json:"source_item_type_raw"`
	SourceItemSubtypeRaw  *string         `json:"source_item_subtype_raw"`
	RarityRaw             string          `json:"rarity_raw"`
	UsageModeRaw          string          `json:"usage_mode_raw"`
	WearSlotRaw           *string         `json:"wear_slot_raw"`
	RequiresAttunement    bool            `json:"requires_attunement"`
	AttunementRequirement *string         `json:"attunement_requirement"`
	RawDescription        string          `json:"raw_description"`
	Effects               []RawEffect     `json:"effects"`
	Limitations           []RawLimitation `json:"limitations"`
	Confidence            float64         `json:"confidence"`
	ReviewReasons         []string        `json:"review_reasons"`
	Continuation          bool            `json:"continuation"`
}

type NormalizedEffect struct {
	Category    generated.EffectCategory `json:"category"`
	Description string                   `json:"description"`
}

type NormalizedCandidate struct {
	Raw            RawCandidate             `json:"raw"`
	SourceItemType generated.SourceItemType `json:"source_item_type"`
	Rarity         generated.Rarity         `json:"rarity"`
	UsageMode      generated.UsageMode      `json:"usage_mode"`
	WearSlot       *generated.WearSlot      `json:"wear_slot"`
	Effects        []NormalizedEffect       `json:"effects"`
	NeedsReview    bool                     `json:"needs_review"`
	ReviewReasons  []string                 `json:"review_reasons"`
}

type ValidationIssue struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

// ExtractionFailure keeps a source candidate out of the accepted and review
// buckets together with the final recovery stage and concrete reasons.
type ExtractionFailure struct {
	Candidate RawCandidate      `json:"candidate"`
	Issues    []ValidationIssue `json:"issues"`
	Stage     string            `json:"stage"`
}

// ExtractionResult is the bounded-recovery outcome consumed by later
// persistence and reporting steps. Each extracted candidate belongs to exactly
// one of Accepted, Review, or Failed. Call counts are retained for run reports.
type ExtractionResult struct {
	Accepted            []NormalizedCandidate `json:"accepted"`
	Review              []NormalizedCandidate `json:"review"`
	Failed              []ExtractionFailure   `json:"failed"`
	CompletenessIssues  []ValidationIssue     `json:"completeness_issues"`
	TextCalls           int                   `json:"text_calls"`
	ImageCalls          int                   `json:"image_calls"`
	ReconciliationCalls int                   `json:"reconciliation_calls"`
	APICalls            int                   `json:"api_calls"`
}

type RunState struct {
	RunID      string                `json:"run_id"`
	Pages      []Page                `json:"pages"`
	OCR        []OCRPage             `json:"ocr"`
	Candidates []RawCandidate        `json:"candidates"`
	Normalized []NormalizedCandidate `json:"normalized"`
	Issues     []ValidationIssue     `json:"issues"`
}
