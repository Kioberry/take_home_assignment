package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactWriteAndReadJSON(t *testing.T) {
	root := t.TempDir()
	store, err := NewArtifactStore(root, "run-1")
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}

	first := []OCRPage{{Number: 1, Text: "first"}}
	if err := store.WriteJSON("ocr.json", first); err != nil {
		t.Fatalf("first WriteJSON: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, "run-1", "ocr.json"))
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if !strings.Contains(string(raw), "\n  ") {
		t.Fatalf("artifact is not indented JSON: %s", raw)
	}
	var decodedJSON []OCRPage
	if err := json.Unmarshal(raw, &decodedJSON); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}

	second := []OCRPage{{Number: 7, Text: "replacement"}}
	if err := store.WriteJSON("ocr.json", second); err != nil {
		t.Fatalf("second WriteJSON: %v", err)
	}
	var restored []OCRPage
	if err := store.ReadJSON("ocr.json", &restored); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if len(restored) != 1 || restored[0] != second[0] {
		t.Fatalf("restored %#v, want %#v", restored, second)
	}
}

func TestArtifactStoreRejectsPathTraversal(t *testing.T) {
	if _, err := NewArtifactStore(t.TempDir(), "../escape"); err == nil {
		t.Fatal("NewArtifactStore accepted path traversal")
	}
}

func TestArtifactReadMissingResumeArtifactIncludesPath(t *testing.T) {
	store, err := NewArtifactStore(t.TempDir(), "run-1")
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}

	var pages []OCRPage
	err = store.ReadJSON("resume.json", &pages)
	if err == nil {
		t.Fatal("ReadJSON succeeded for missing artifact")
	}
	wantPath := filepath.Join(store.Root(), "resume.json")
	if !strings.Contains(err.Error(), wantPath) {
		t.Fatalf("error %q does not contain artifact path %q", err, wantPath)
	}
}
