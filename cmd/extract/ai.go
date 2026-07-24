package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExtractRequest struct {
	OCR    []OCRPage
	Images []Page
	Prior  []RawCandidate
	Issues []ValidationIssue
	Mode   string
}

type AIExtractor interface {
	Extract(context.Context, ExtractRequest) ([]RawCandidate, error)
}

const anthropicCandidateToolName = "submit_catalog_candidates"

// AnthropicExtractor adapts the Anthropic Messages API tool-use response to
// the pipeline's existing strict RawCandidate contract.
type AnthropicExtractor struct {
	httpClient  *http.Client
	apiURL      string
	apiKey      string
	textModel   string
	visionModel string
	maxAttempts int
}

func NewAnthropicExtractor(httpClient *http.Client, apiURL, apiKey, textModel, visionModel string, maxAttempts int) *AnthropicExtractor {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &AnthropicExtractor{
		httpClient: httpClient, apiURL: anthropicMessagesURL(apiURL), apiKey: apiKey,
		textModel: textModel, visionModel: visionModel, maxAttempts: maxAttempts,
	}
}

func (e *AnthropicExtractor) Extract(ctx context.Context, request ExtractRequest) ([]RawCandidate, error) {
	candidates, _, err := e.ExtractWithMetrics(ctx, request)
	return candidates, err
}

func (e *AnthropicExtractor) ExtractWithMetrics(ctx context.Context, request ExtractRequest) ([]RawCandidate, AICallCounts, error) {
	counts := AICallCounts{}
	payload, err := e.requestPayload(request)
	if err != nil {
		return nil, counts, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, counts, fmt.Errorf("marshal Anthropic request: %w", err)
	}
	for attempt := 0; attempt < e.maxAttempts; attempt++ {
		response, err := e.doRequest(ctx, body, request.Mode, &counts)
		if err != nil {
			return nil, counts, err
		}
		if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			candidates, err := parseAnthropicResponse(response.Body)
			response.Body.Close()
			return candidates, counts, err
		}
		status := response.StatusCode
		detail := apiErrorDetail(response.Body, e.apiKey)
		response.Body.Close()
		if !isRetryableStatus(status) || attempt == e.maxAttempts-1 {
			return nil, counts, fmt.Errorf("Anthropic Messages API returned HTTP %d%s", status, detail)
		}
		if err := waitForRetry(ctx, attempt); err != nil {
			return nil, counts, err
		}
	}
	return nil, counts, fmt.Errorf("Anthropic Messages API attempts exhausted")
}

func (e *AnthropicExtractor) doRequest(ctx context.Context, body []byte, mode string, counts *AICallCounts) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Anthropic request: %w", err)
	}
	request.Header.Set("x-api-key", e.apiKey)
	request.Header.Set("anthropic-version", "2023-06-01")
	request.Header.Set("Content-Type", "application/json")
	recordAIAttempt(counts, mode)
	response, err := e.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send Anthropic request: %w", err)
	}
	return response, nil
}

func (e *AnthropicExtractor) requestPayload(request ExtractRequest) (map[string]any, error) {
	model := e.textModel
	if len(request.Images) > 0 {
		model = e.visionModel
	}
	content := []map[string]any{{"type": "text", "text": requestText(request)}}
	for _, page := range request.Images {
		dataURL, err := imageDataURL(page.ImagePath)
		if err != nil {
			return nil, err
		}
		parts := strings.SplitN(dataURL, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("encode page image")
		}
		mediaType := strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")
		content = append(content, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mediaType, "data": parts[1]}})
	}
	return map[string]any{
		"model": model, "max_tokens": 8192, "system": extractionSystemPrompt,
		"messages":    []map[string]any{{"role": "user", "content": content}},
		"tools":       []map[string]any{{"name": anthropicCandidateToolName, "description": "Return the extracted catalog candidates as structured data.", "input_schema": candidateSchema()}},
		"tool_choice": map[string]any{"type": "tool", "name": anthropicCandidateToolName},
	}, nil
}

func anthropicMessagesURL(apiURL string) string {
	apiURL = strings.TrimRight(apiURL, "/")
	if strings.HasSuffix(apiURL, "/v1/messages") {
		return apiURL
	}
	return apiURL + "/v1/messages"
}

func parseAnthropicResponse(body io.Reader) ([]RawCandidate, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Anthropic response: %w", err)
	}
	if len(data) > maxResponseBytes {
		return nil, fmt.Errorf("Anthropic response too large")
	}
	var response struct {
		Content []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode Anthropic response: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, fmt.Errorf("decode Anthropic response: %w", err)
	}
	for _, content := range response.Content {
		if content.Type == "tool_use" && content.Name == anthropicCandidateToolName {
			return decodeStructuredCandidates(content.Input)
		}
	}
	return nil, fmt.Errorf("Anthropic response has no %s tool_use", anthropicCandidateToolName)
}

func apiErrorDetail(body io.Reader, secret string) string {
	data, err := io.ReadAll(io.LimitReader(body, 4097))
	if err != nil || len(data) == 0 {
		return ""
	}
	message := strings.TrimSpace(string(data[:min(len(data), 4096)]))
	if message == "" {
		return ""
	}
	return ": " + safeError(fmt.Errorf("%s", message), secret).Error()
}

type OpenAIExtractor struct {
	httpClient  *http.Client
	apiURL      string
	apiKey      string
	textModel   string
	visionModel string
	maxAttempts int
}

const defaultOpenAIRequestTimeout = 90 * time.Second

func newOpenAIHTTPClient() *http.Client {
	return &http.Client{Timeout: defaultOpenAIRequestTimeout}
}

func NewOpenAIExtractor(httpClient *http.Client, apiURL, apiKey, textModel, visionModel string, maxAttempts int) *OpenAIExtractor {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return &OpenAIExtractor{
		httpClient:  httpClient,
		apiURL:      responseURL(apiURL),
		apiKey:      apiKey,
		textModel:   textModel,
		visionModel: visionModel,
		maxAttempts: maxAttempts,
	}
}

func (e *OpenAIExtractor) Extract(ctx context.Context, request ExtractRequest) ([]RawCandidate, error) {
	candidates, _, err := e.ExtractWithMetrics(ctx, request)
	return candidates, err
}

// ExtractWithMetrics returns candidates and the HTTP request attempts made for
// this invocation only. Retry attempts are included in the returned snapshot.
func (e *OpenAIExtractor) ExtractWithMetrics(ctx context.Context, request ExtractRequest) ([]RawCandidate, AICallCounts, error) {
	counts := AICallCounts{}
	payload, err := e.requestPayload(request)
	if err != nil {
		return nil, counts, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, counts, fmt.Errorf("marshal Responses request: %w", err)
	}

	for attempt := 0; attempt < e.maxAttempts; attempt++ {
		started := time.Now()
		response, err := e.doRequest(ctx, body, request.Mode, &counts)
		elapsed := time.Since(started).Round(time.Millisecond)
		if err != nil {
			return nil, counts, fmt.Errorf("Responses API request failed after %s: %w", elapsed, err)
		}
		if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			candidates, err := parseResponse(response.Body)
			response.Body.Close()
			return candidates, counts, err
		}
		status := response.StatusCode
		detail := apiErrorDetail(response.Body, e.apiKey)
		response.Body.Close()
		if !isRetryableStatus(status) || attempt == e.maxAttempts-1 {
			return nil, counts, fmt.Errorf("Responses API returned HTTP %d after %s%s", status, elapsed, detail)
		}
		if err := waitForRetry(ctx, attempt); err != nil {
			return nil, counts, err
		}
	}

	return nil, counts, fmt.Errorf("Responses API attempts exhausted")
}

func (e *OpenAIExtractor) doRequest(ctx context.Context, body []byte, mode string, counts *AICallCounts) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Responses request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+e.apiKey)
	request.Header.Set("Content-Type", "application/json")

	recordAIAttempt(counts, mode)
	response, err := e.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send Responses request: %w", err)
	}
	return response, nil
}

func recordAIAttempt(counts *AICallCounts, mode string) {
	counts.APICalls++
	switch mode {
	case "recover":
		counts.ImageCalls++
	case "reconcile":
		counts.ReconciliationCalls++
	default:
		counts.TextCalls++
	}
}

func (e *OpenAIExtractor) requestPayload(request ExtractRequest) (map[string]any, error) {
	model := e.textModel
	if len(request.Images) > 0 {
		model = e.visionModel
	}
	content := []map[string]any{{
		"type": "input_text",
		"text": requestText(request),
	}}
	for _, page := range request.Images {
		imageURL, err := imageDataURL(page.ImagePath)
		if err != nil {
			return nil, err
		}
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": imageURL,
		})
	}

	return map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "system",
				"content": []map[string]any{{
					"type": "input_text",
					"text": extractionSystemPrompt,
				}},
			},
			{"role": "user", "content": content},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "catalog_candidates",
				"strict": true,
				"schema": candidateSchema(),
			},
		},
	}, nil
}

func responseURL(apiURL string) string {
	apiURL = strings.TrimRight(apiURL, "/")
	if strings.HasSuffix(apiURL, "/v1/responses") {
		return apiURL
	}
	return apiURL + "/v1/responses"
}

func imageDataURL(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read page image: %w", err)
	}
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func requestText(request ExtractRequest) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Mode: %s\n", request.Mode)
	if (request.Mode == "recover" || request.Mode == "reconcile") && len(request.Prior) == 1 {
		target := request.Prior[0]
		fmt.Fprintf(&builder, "Recovery target: %s\n", target.Name)
		fmt.Fprintf(&builder, "Return exactly one candidate: the corrected replacement for %s.\n", target.Name)
		builder.WriteString("Do not return neighboring or additional catalog items.\n")
	}
	for _, page := range request.OCR {
		fmt.Fprintf(&builder, "\nOCR page %d:\n%s\n", page.Number, page.Text)
	}
	if len(request.Prior) > 0 {
		prior, _ := json.Marshal(request.Prior)
		fmt.Fprintf(&builder, "\nPrior candidates for reconciliation: %s\n", prior)
	}
	if len(request.Issues) > 0 {
		issues, _ := json.Marshal(request.Issues)
		fmt.Fprintf(&builder, "\nValidation issues: %s\n", issues)
	}
	return builder.String()
}

func candidateSchema() map[string]any {
	stringOrNull := func() map[string]any {
		return map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}}
	}
	integerOrNull := func() map[string]any {
		return map[string]any{"anyOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "null"}}}
	}
	effect := objectSchema(map[string]any{
		"category_raw": map[string]any{"type": "string"},
		"description":  map[string]any{"type": "string"},
	}, "category_raw", "description")
	limitation := objectSchema(map[string]any{
		"effect_index": integerOrNull(),
		"description":  map[string]any{"type": "string"},
	}, "effect_index", "description")
	candidate := objectSchema(map[string]any{
		"name":                    map[string]any{"type": "string"},
		"source_pages":            map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
		"source_item_type_raw":    map[string]any{"type": "string"},
		"source_item_subtype_raw": stringOrNull(),
		"rarity_raw":              map[string]any{"type": "string"},
		"usage_mode_raw":          map[string]any{"type": "string"},
		"wear_slot_raw":           stringOrNull(),
		"requires_attunement":     map[string]any{"type": "boolean"},
		"attunement_requirement":  stringOrNull(),
		"raw_description":         map[string]any{"type": "string"},
		"effects":                 map[string]any{"type": "array", "items": effect},
		"limitations":             map[string]any{"type": "array", "items": limitation},
		"confidence":              map[string]any{"type": "number"},
		"review_reasons":          map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"continuation":            map[string]any{"type": "boolean"},
	},
		"name", "source_pages", "source_item_type_raw", "source_item_subtype_raw", "rarity_raw",
		"usage_mode_raw", "wear_slot_raw", "requires_attunement", "attunement_requirement",
		"raw_description", "effects", "limitations", "confidence", "review_reasons", "continuation",
	)
	return objectSchema(map[string]any{
		"candidates": map[string]any{"type": "array", "items": candidate},
	}, "candidates")
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	fields := make([]any, len(required))
	for index, field := range required {
		fields[index] = field
	}
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             fields,
		"additionalProperties": false,
	}
}

func parseResponse(body io.Reader) ([]RawCandidate, error) {
	bodyBytes, err := io.ReadAll(io.LimitReader(body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Responses response: %w", err)
	}
	if len(bodyBytes) > maxResponseBytes {
		return nil, fmt.Errorf("Responses API response too large")
	}

	var response struct {
		Output []struct {
			Content []struct {
				Type    string `json:"type"`
				Text    string `json:"text"`
				Refusal string `json:"refusal"`
			} `json:"content"`
		} `json:"output"`
	}
	decoder := json.NewDecoder(bytes.NewReader(bodyBytes))
	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf("decode Responses response: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, fmt.Errorf("decode Responses response: %w", err)
	}

	var outputText string
	for _, output := range response.Output {
		for _, content := range output.Content {
			switch content.Type {
			case "refusal":
				return nil, fmt.Errorf("Responses API refusal: %s", content.Refusal)
			case "output_text":
				if outputText != "" {
					return nil, fmt.Errorf("Responses API returned multiple output_text parts")
				}
				outputText = content.Text
			}
		}
	}
	if strings.TrimSpace(outputText) == "" {
		return nil, fmt.Errorf("Responses API response has no output_text")
	}

	return decodeStructuredCandidates([]byte(outputText))
}

func decodeStructuredCandidates(data []byte) ([]RawCandidate, error) {
	root, err := decodeObject(data)
	if err != nil {
		return nil, fmt.Errorf("decode structured output: %w", err)
	}
	for field := range root {
		if field != "candidates" {
			return nil, fmt.Errorf("decode structured output: unknown field %q", field)
		}
	}
	rawCandidates, ok := root["candidates"]
	if !ok {
		return nil, fmt.Errorf("decode structured output: missing required candidates")
	}

	candidatesJSON, err := decodeRequiredArray(rawCandidates, "candidates")
	if err != nil {
		return nil, fmt.Errorf("decode structured output candidates: %w", err)
	}
	candidates := make([]RawCandidate, 0, len(candidatesJSON))
	for _, rawCandidate := range candidatesJSON {
		candidate, err := decodeCandidate(rawCandidate)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func decodeCandidate(data json.RawMessage) (RawCandidate, error) {
	fields, err := decodeObject(data)
	if err != nil {
		return RawCandidate{}, fmt.Errorf("decode candidate: %w", err)
	}
	if err := requireFields(fields, "candidate", rawCandidateFields...); err != nil {
		return RawCandidate{}, err
	}
	if _, err := decodeRequiredArray(fields["source_pages"], "candidate.source_pages"); err != nil {
		return RawCandidate{}, err
	}
	if err := requireArrayObjectFields(fields["effects"], "candidate.effects", "effect", rawEffectFields...); err != nil {
		return RawCandidate{}, err
	}
	if err := requireArrayObjectFields(fields["limitations"], "candidate.limitations", "limitation", rawLimitationFields...); err != nil {
		return RawCandidate{}, err
	}
	if _, err := decodeRequiredArray(fields["review_reasons"], "candidate.review_reasons"); err != nil {
		return RawCandidate{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var candidate RawCandidate
	if err := decoder.Decode(&candidate); err != nil {
		return RawCandidate{}, fmt.Errorf("decode structured output: %w", err)
	}
	if err := requireEOF(decoder); err != nil {
		return RawCandidate{}, fmt.Errorf("decode structured output: %w", err)
	}
	return candidate, nil
}

func decodeObject(data []byte) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := decodeJSON(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected object")
	}
	return fields, nil
}

func requireArrayObjectFields(data json.RawMessage, arrayName, kind string, required ...string) error {
	objects, err := decodeRequiredArray(data, arrayName)
	if err != nil {
		return err
	}
	for _, object := range objects {
		fields, err := decodeObject(object)
		if err != nil {
			return fmt.Errorf("decode %s: %w", kind, err)
		}
		if err := requireFields(fields, kind, required...); err != nil {
			return err
		}
	}
	return nil
}

func decodeRequiredArray(data json.RawMessage, name string) ([]json.RawMessage, error) {
	var values []json.RawMessage
	if err := decodeJSON(data, &values); err != nil {
		return nil, fmt.Errorf("decode %s array: %w", name, err)
	}
	if values == nil {
		return nil, fmt.Errorf("%s array must not be null", name)
	}
	return values, nil
}

func requireFields(fields map[string]json.RawMessage, kind string, required ...string) error {
	for _, field := range required {
		if _, ok := fields[field]; !ok {
			return fmt.Errorf("missing required %s.%s", kind, field)
		}
	}
	return nil
}

func decodeJSON(data []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	return requireEOF(decoder)
}

func requireEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("trailing JSON data")
	}
	return fmt.Errorf("trailing JSON data: %w", err)
}

var rawCandidateFields = []string{
	"name", "source_pages", "source_item_type_raw", "source_item_subtype_raw", "rarity_raw",
	"usage_mode_raw", "wear_slot_raw", "requires_attunement", "attunement_requirement",
	"raw_description", "effects", "limitations", "confidence", "review_reasons", "continuation",
}

var rawEffectFields = []string{"category_raw", "description"}

var rawLimitationFields = []string{"effect_index", "description"}

const maxResponseBytes = 10 << 20

func isRetryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func waitForRetry(ctx context.Context, attempt int) error {
	delay := 250 * time.Millisecond
	if attempt > 0 {
		delay = 750 * time.Millisecond
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

const extractionSystemPrompt = `You extract catalog items from supplied source evidence. Identify semantic item boundaries and preserve source-grounded descriptions. Emit complete candidates or set continuation=true when the source continues an item. Use only source evidence; never invent facts. Source page numbers must come from supplied page labels. Effect indexes are zero-based.

The following output fields are closed vocabularies. Emit only the allowed value, never a prose label, ability name, cooldown, or synonym outside this list:
- source_item_type_raw: exactly one of Wondrous item, Weapon, Armor, Potion, Ring.
- rarity_raw: exactly one of common, uncommon, rare, very rare, legendary, artifact, varies.
- usage_mode_raw: exactly one of worn, held, portable, consumed, worn armor, worn weapon, held armor, held weapon. Choose how the item is used, not what an ability does. Never put cooldowns, actions, durations, charges, or ability descriptions in usage_mode_raw.
- wear_slot_raw: null or exactly one of head, neck, torso, outerwear, hands, feet, finger. Armor is torso; a helm is head; a cloak is outerwear; a ring is finger. A non-worn item must use null.
- category_raw: exactly one of offensive, defensive, utility. Create one Effect per independently understandable ability and assign its primary purpose: offensive harms/attacks/controls enemies, defensive protects/resists/prevents harm, utility covers movement/senses/spells/other support. Do not emit labels such as movement, flight, armor, spellcasting, or luck manipulation.

Keep limitations, cooldowns, charges, durations, activation actions, and exceptions in limitations or effect descriptions, never in closed-vocabulary fields. Preserve uncertain source details in raw_description and add a review reason rather than replacing them with invented facts.

Mode recover or reconcile: return exactly one candidate. It must be the corrected replacement for the named Recovery target in the user content. Do not return page neighbors, split fragments, or any additional catalog items.`
