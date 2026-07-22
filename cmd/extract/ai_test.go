package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenAIExtractorSendsStrictStructuredTextRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertResponsesRequest(t, r, "text-model", false)
		writeResponse(t, w, successfulResponse(candidateJSON(t)))
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 1)
	candidates, err := extractor.Extract(context.Background(), ExtractRequest{
		OCR:    []OCRPage{{Number: 7, Text: "Arcane Compass\nRare wondrous item"}},
		Prior:  []RawCandidate{{Name: "Earlier item"}},
		Issues: []ValidationIssue{{Code: "overlap", Message: "Page 6 overlaps page 7", Recoverable: true}},
		Mode:   "extract",
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Name != "Arcane Compass" {
		t.Fatalf("candidates = %#v, want decoded Arcane Compass", candidates)
	}
}

func TestOpenAIExtractorUsesVisionModelAndDataURLs(t *testing.T) {
	imagePath := filepath.Join(t.TempDir(), "page-008.png")
	image := []byte{0x89, 0x50, 0x4e, 0x47}
	if err := os.WriteFile(imagePath, image, 0600); err != nil {
		t.Fatalf("write image fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertResponsesRequest(t, r, "vision-model", true)
		writeResponse(t, w, successfulResponse(candidateJSON(t)))
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 1)
	_, err := extractor.Extract(context.Background(), ExtractRequest{
		OCR:    []OCRPage{{Number: 8, Text: "Arcane Compass"}},
		Images: []Page{{Number: 8, ImagePath: imagePath}},
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
}

func TestOpenAIExtractorRetriesRateLimitThenSucceeds(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		writeResponse(t, w, successfulResponse(candidateJSON(t)))
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 2)
	candidates, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
	if err != nil {
		t.Fatalf("Extract after retry: %v", err)
	}
	if calls != 2 || len(candidates) != 1 {
		t.Fatalf("calls=%d candidates=%#v, want 2 calls and one candidate", calls, candidates)
	}
}

func TestOpenAIExtractorDoesNotRetryPermanentHTTPError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 3)
	_, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Fatalf("error = %v, want HTTP 400 error", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want no retry", calls)
	}
}

func TestOpenAIExtractorReportsRefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeResponse(t, w, map[string]any{
			"output": []any{map[string]any{
				"type":    "message",
				"content": []any{map[string]any{"type": "refusal", "refusal": "I cannot process that."}},
			}},
		})
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 1)
	_, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "refusal") {
		t.Fatalf("error = %v, want refusal error", err)
	}
}

func TestOpenAIExtractorRejectsMissingOutputText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeResponse(t, w, map[string]any{"output": []any{map[string]any{
			"type":    "message",
			"content": []any{map[string]any{"type": "reasoning", "summary": "hidden"}},
		}}})
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 1)
	_, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
	if err == nil || !strings.Contains(err.Error(), "output_text") {
		t.Fatalf("error = %v, want missing output_text error", err)
	}
}

func TestOpenAIExtractorRejectsUnknownCandidateFields(t *testing.T) {
	candidate := candidateJSON(t)
	candidate["unexpected"] = true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeResponse(t, w, successfulResponse(candidate))
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 1)
	_, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown-field error", err)
	}
}

func TestOpenAIExtractorRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		candidate func() map[string]any
		want      string
	}{
		{
			name: "candidate field",
			candidate: func() map[string]any {
				return map[string]any{"name": "Arcane Compass"}
			},
			want: "candidate.source_pages",
		},
		{
			name: "effect field",
			candidate: func() map[string]any {
				candidate := candidateJSON(t)
				candidate["effects"] = []any{map[string]any{"description": "Points north."}}
				return candidate
			},
			want: "effect.category_raw",
		},
		{
			name: "limitation field",
			candidate: func() map[string]any {
				candidate := candidateJSON(t)
				candidate["limitations"] = []any{map[string]any{"effect_index": nil}}
				return candidate
			},
			want: "limitation.description",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeResponse(t, w, successfulResponse(test.candidate()))
			}))
			defer server.Close()

			extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 1)
			_, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want missing %s", err, test.want)
			}
		})
	}
}

func TestOpenAIExtractorRejectsConcatenatedResponsesJSON(t *testing.T) {
	response := mustJSON(successfulResponse(candidateJSON(t)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(response + response)); err != nil {
			t.Fatalf("write concatenated response: %v", err)
		}
	}))
	defer server.Close()

	extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 1)
	_, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
	if err == nil || !strings.Contains(err.Error(), "trailing JSON") {
		t.Fatalf("error = %v, want trailing JSON error", err)
	}
}

func TestOpenAIExtractorRetriesRequestTimeoutAndServerError(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					http.Error(w, http.StatusText(status), status)
					return
				}
				writeResponse(t, w, successfulResponse(candidateJSON(t)))
			}))
			defer server.Close()

			extractor := NewOpenAIExtractor(server.Client(), server.URL, "test-key", "text-model", "vision-model", 2)
			candidates, err := extractor.Extract(context.Background(), ExtractRequest{OCR: []OCRPage{{Number: 1, Text: "item"}}})
			if err != nil {
				t.Fatalf("Extract after HTTP %d: %v", status, err)
			}
			if calls != 2 || len(candidates) != 1 {
				t.Fatalf("calls=%d candidates=%#v, want 2 calls and one candidate", calls, candidates)
			}
		})
	}
}

func assertResponsesRequest(t *testing.T, r *http.Request, wantModel string, wantImage bool) {
	t.Helper()
	if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
		t.Fatalf("request = %s %s, want POST /v1/responses", r.Method, r.URL.Path)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("Authorization = %q, want bearer token", got)
	}

	var request map[string]any
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if got := request["model"]; got != wantModel {
		t.Fatalf("model = %#v, want %q", got, wantModel)
	}
	format := request["text"].(map[string]any)["format"].(map[string]any)
	if format["type"] != "json_schema" || format["strict"] != true {
		t.Fatalf("text.format = %#v, want strict json_schema", format)
	}
	schema := format["schema"].(map[string]any)
	assertClosedObjects(t, schema)
	candidates := schema["properties"].(map[string]any)["candidates"].(map[string]any)
	candidate := candidates["items"].(map[string]any)
	assertRequiredFields(t, candidate, []string{
		"name", "source_pages", "source_item_type_raw", "source_item_subtype_raw", "rarity_raw",
		"usage_mode_raw", "wear_slot_raw", "requires_attunement", "attunement_requirement",
		"raw_description", "effects", "limitations", "confidence", "review_reasons", "continuation",
	})
	assertNullable(t, candidate["properties"].(map[string]any)["source_item_subtype_raw"].(map[string]any), "string")
	assertNullable(t, candidate["properties"].(map[string]any)["wear_slot_raw"].(map[string]any), "string")
	assertNullable(t, candidate["properties"].(map[string]any)["attunement_requirement"].(map[string]any), "string")
	limitation := candidate["properties"].(map[string]any)["limitations"].(map[string]any)["items"].(map[string]any)
	assertNullable(t, limitation["properties"].(map[string]any)["effect_index"].(map[string]any), "integer")

	input := request["input"].([]any)
	content := input[len(input)-1].(map[string]any)["content"].([]any)
	images := 0
	for _, part := range content {
		part := part.(map[string]any)
		if part["type"] != "input_image" {
			continue
		}
		images++
		url := part["image_url"].(string)
		if !strings.HasPrefix(url, "data:image/png;base64,") {
			t.Fatalf("image URL = %q, want PNG data URL", url)
		}
		if _, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, "data:image/png;base64,")); err != nil {
			t.Fatalf("data URL base64: %v", err)
		}
	}
	if (images > 0) != wantImage {
		t.Fatalf("input images=%d, want image=%v", images, wantImage)
	}
}

func assertClosedObjects(t *testing.T, schema map[string]any) {
	t.Helper()
	if schema["type"] == "object" && schema["additionalProperties"] != false {
		t.Fatalf("object schema is not closed: %#v", schema)
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for _, property := range properties {
			if propertySchema, ok := property.(map[string]any); ok {
				assertClosedObjects(t, propertySchema)
			}
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		assertClosedObjects(t, items)
	}
	if variants, ok := schema["anyOf"].([]any); ok {
		for _, variant := range variants {
			if variantSchema, ok := variant.(map[string]any); ok {
				assertClosedObjects(t, variantSchema)
			}
		}
	}
}

func assertRequiredFields(t *testing.T, schema map[string]any, want []string) {
	t.Helper()
	required, ok := schema["required"].([]any)
	if !ok || len(required) != len(want) {
		t.Fatalf("required = %#v, want %#v", schema["required"], want)
	}
	for _, field := range want {
		found := false
		for _, got := range required {
			found = found || got == field
		}
		if !found {
			t.Fatalf("required = %#v, missing %q", required, field)
		}
	}
}

func assertNullable(t *testing.T, schema map[string]any, valueType string) {
	t.Helper()
	variants, ok := schema["anyOf"].([]any)
	if !ok || len(variants) != 2 {
		t.Fatalf("nullable schema = %#v, want two anyOf variants", schema)
	}
	seen := map[string]bool{}
	for _, variant := range variants {
		if item, ok := variant.(map[string]any); ok {
			if kind, ok := item["type"].(string); ok {
				seen[kind] = true
			}
		}
	}
	if !seen[valueType] || !seen["null"] {
		t.Fatalf("nullable schema = %#v, want %q or null", schema, valueType)
	}
}

func successfulResponse(candidate map[string]any) map[string]any {
	return map[string]any{
		"output": []any{
			map[string]any{"type": "reasoning", "summary": []any{}},
			map[string]any{"type": "message", "content": []any{
				map[string]any{"type": "output_text", "text": mustJSON(candidateWrapper(candidate))},
			}},
		},
	}
}

func candidateWrapper(candidate map[string]any) map[string]any {
	return map[string]any{"candidates": []any{candidate}}
}

func candidateJSON(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"name":                    "Arcane Compass",
		"source_pages":            []any{7},
		"source_item_type_raw":    "wondrous item",
		"source_item_subtype_raw": nil,
		"rarity_raw":              "rare",
		"usage_mode_raw":          "passive",
		"wear_slot_raw":           nil,
		"requires_attunement":     false,
		"attunement_requirement":  nil,
		"raw_description":         "Points toward a named destination.",
		"effects": []any{map[string]any{
			"category_raw": "navigation",
			"description":  "Points toward a named destination.",
		}},
		"limitations": []any{map[string]any{
			"effect_index": nil,
			"description":  "Requires a destination name.",
		}},
		"confidence":     0.92,
		"review_reasons": []any{},
		"continuation":   false,
	}
}

func mustJSON(value any) string {
	bytes, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal fixture: %v", err))
	}
	return string(bytes)
}

func writeResponse(t *testing.T, w http.ResponseWriter, response map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
