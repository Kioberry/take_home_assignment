package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropicExtractorSendsToolUseRequestAndDecodesCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			t.Fatalf("request = %s %s, want POST /v1/messages", r.Method, r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Fatalf("anthropic-version = %q", r.Header.Get("anthropic-version"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["model"] != "claude-test" {
			t.Fatalf("model = %#v", payload["model"])
		}
		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("tools = %#v", payload["tools"])
		}
		tool := tools[0].(map[string]any)
		if tool["name"] != anthropicCandidateToolName {
			t.Fatalf("tool name = %#v", tool["name"])
		}
		if _, ok := tool["input_schema"].(map[string]any); !ok {
			t.Fatalf("input_schema = %#v", tool["input_schema"])
		}
		io.WriteString(w, `{"content":[{"type":"tool_use","name":"submit_catalog_candidates","input":{"candidates":[]}}]}`)
	}))
	defer server.Close()

	extractor := NewAnthropicExtractor(server.Client(), server.URL, "test-key", "claude-test", "claude-vision", 1)
	candidates, err := extractor.Extract(context.Background(), ExtractRequest{Mode: "extract"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("candidates = %#v, want empty", candidates)
	}
}

func TestAnthropicExtractorReportsMissingToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"content":[{"type":"text","text":"no structured output"}]}`)
	}))
	defer server.Close()

	extractor := NewAnthropicExtractor(server.Client(), server.URL, "test-key", "claude-test", "claude-vision", 1)
	_, err := extractor.Extract(context.Background(), ExtractRequest{Mode: "extract"})
	if err == nil || !strings.Contains(err.Error(), "tool_use") {
		t.Fatalf("Extract error = %v, want missing tool_use", err)
	}
}

func TestAnthropicExtractorIncludesSafeAPIErrorDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"type":"error","error":{"message":"unknown endpoint"}}`)
	}))
	defer server.Close()

	extractor := NewAnthropicExtractor(server.Client(), server.URL, "test-key", "claude-test", "claude-vision", 1)
	_, err := extractor.Extract(context.Background(), ExtractRequest{Mode: "extract"})
	if err == nil || !strings.Contains(err.Error(), "unknown endpoint") {
		t.Fatalf("Extract error = %v, want server error detail", err)
	}
}
