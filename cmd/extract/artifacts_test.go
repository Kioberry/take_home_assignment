package main

import (
	"encoding/json"
	"io"
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
	oldFile, err := os.Open(filepath.Join(root, "run-1", "ocr.json"))
	if err != nil {
		t.Fatalf("open old artifact: %v", err)
	}
	defer oldFile.Close()

	second := []OCRPage{{Number: 7, Text: "replacement"}}
	if err := store.WriteJSON("ocr.json", second); err != nil {
		t.Fatalf("second WriteJSON: %v", err)
	}
	oldBytes, err := os.ReadFile(filepath.Join(root, "run-1", "ocr.json.tmp"))
	if err == nil {
		t.Fatalf("temporary artifact remains: %s", oldBytes)
	}
	oldOpenBytes, err := io.ReadAll(oldFile)
	if err != nil {
		t.Fatalf("read old artifact descriptor: %v", err)
	}
	if string(oldOpenBytes) != string(raw) {
		t.Fatalf("old descriptor saw %q, want %q", oldOpenBytes, raw)
	}
	newPathBytes, err := os.ReadFile(filepath.Join(root, "run-1", "ocr.json"))
	if err != nil {
		t.Fatalf("reopen replacement artifact: %v", err)
	}
	if string(newPathBytes) == string(raw) || !strings.Contains(string(newPathBytes), "replacement") {
		t.Fatalf("reopened path saw %q, want replacement bytes", newPathBytes)
	}
	var restored []OCRPage
	if err := store.ReadJSON("ocr.json", &restored); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if len(restored) != 1 || restored[0] != second[0] {
		t.Fatalf("restored %#v, want %#v", restored, second)
	}
}

func TestArtifactWriteRecreatesMissingRunDirectory(t *testing.T) {
	root := t.TempDir()
	store, err := NewArtifactStore(root, "run-1")
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}
	if err := os.RemoveAll(store.Root()); err != nil {
		t.Fatalf("remove run directory: %v", err)
	}

	if err := store.WriteJSON("ocr.json", []OCRPage{{Number: 1, Text: "recreated"}}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	info, err := os.Stat(store.Root())
	if err != nil {
		t.Fatalf("stat recreated run directory: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0755); got != want {
		t.Fatalf("run directory mode %o, want %o", got, want)
	}
}

func TestArtifactWriteCleansTemporaryFileAfterEncodeFailure(t *testing.T) {
	store, err := NewArtifactStore(t.TempDir(), "run-1")
	if err != nil {
		t.Fatalf("NewArtifactStore: %v", err)
	}

	err = store.WriteJSON("broken.json", make(chan int))
	if err == nil {
		t.Fatal("WriteJSON succeeded for unsupported JSON value")
	}
	if _, statErr := os.Stat(filepath.Join(store.Root(), "broken.json.tmp")); !os.IsNotExist(statErr) {
		t.Fatalf("temporary artifact stat error = %v, want not exist", statErr)
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
