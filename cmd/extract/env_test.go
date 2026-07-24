package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingValuesWithoutOverridingEnvironment(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".env")
	if err := os.WriteFile(path, []byte("OPENAI_API_KEY=file-key\nOPENAI_TEXT_MODEL=file-model\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_TEXT_MODEL", "shell-model")
	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != "file-key" {
		t.Fatalf("API key = %q", got)
	}
	if got := os.Getenv("OPENAI_TEXT_MODEL"); got != "shell-model" {
		t.Fatalf("model = %q, want shell value", got)
	}
}
