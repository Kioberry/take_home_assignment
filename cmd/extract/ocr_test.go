package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestOCRPagesUsesTesseractCommandBoundary(t *testing.T) {
	runner := &fakeCommandRunner{stdout: []byte("name\n\n")}
	got, err := OCRPages(context.Background(), runner, []Page{{Number: 7, ImagePath: "/tmp/page-007.png"}})
	if err != nil {
		t.Fatalf("OCRPages: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(runner.calls))
	}
	wantArgs := []string{"/tmp/page-007.png", "stdout", "-l", "eng", "--psm", "6"}
	if !reflect.DeepEqual(runner.calls[0].args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", runner.calls[0].args, wantArgs)
	}
	if got[0] != (OCRPage{Number: 7, Text: "name"}) {
		t.Fatalf("OCR result = %#v, want trimmed text", got)
	}
}

func TestOCRPagesRejectsBlankOutput(t *testing.T) {
	runner := &fakeCommandRunner{stdout: []byte(" \n\t")}
	_, err := OCRPages(context.Background(), runner, []Page{{Number: 1, ImagePath: "page.png"}})
	if err == nil || !strings.Contains(err.Error(), "blank") {
		t.Fatalf("error = %v, want blank OCR error", err)
	}
}

func TestOCRPagesIncludesStderrInCommandError(t *testing.T) {
	runner := &fakeCommandRunner{stderr: []byte("tesseract failed"), run: func(string, []string) error {
		return errors.New("exit status 1")
	}}
	_, err := OCRPages(context.Background(), runner, []Page{{Number: 1, ImagePath: "page.png"}})
	if err == nil || !strings.Contains(err.Error(), "tesseract failed") {
		t.Fatalf("error = %v, want stderr", err)
	}
}
