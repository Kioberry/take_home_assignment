package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type commandCall struct {
	name string
	args []string
}

type fakeCommandRunner struct {
	calls  []commandCall
	stdout []byte
	stderr []byte
	run    func(name string, args []string) error
}

func (f *fakeCommandRunner) Run(_ context.Context, name string, args ...string) ([]byte, []byte, error) {
	f.calls = append(f.calls, commandCall{name: name, args: append([]string(nil), args...)})
	if f.run != nil {
		if err := f.run(name, args); err != nil {
			return nil, f.stderr, err
		}
	}
	return f.stdout, f.stderr, nil
}

func TestParsePageRange(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{name: "range", input: "1-5", want: []int{1, 2, 3, 4, 5}},
		{name: "single", input: "3", want: []int{3}},
		{name: "reversed", input: "5-3", wantErr: true},
		{name: "zero", input: "0", wantErr: true},
		{name: "over maximum", input: "40", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePageRange(tt.input, 39)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParsePageRange(%q) succeeded, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePageRange(%q): %v", tt.input, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParsePageRange(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderPagesUsesSelectedPageCommandBoundary(t *testing.T) {
	outputDir := t.TempDir()
	runner := &fakeCommandRunner{run: func(name string, args []string) error {
		if name != "pdftoppm" {
			t.Fatalf("command = %q, want pdftoppm", name)
		}
		page := args[4]
		path := filepath.Join(args[len(args)-1] + "-" + page + ".png")
		return os.WriteFile(path, []byte("image"), 0644)
	}}

	got, err := RenderPages(context.Background(), runner, "catalog.pdf", outputDir, []int{5, 3})
	if err != nil {
		t.Fatalf("RenderPages: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(runner.calls))
	}
	if got[0].Number != 3 || got[1].Number != 5 {
		t.Fatalf("pages = %#v, want page numbers 3, 5", got)
	}
	for _, page := range got {
		if filepath.Base(page.ImagePath) != fmt.Sprintf("page-%03d.png", page.Number) {
			t.Fatalf("image path = %q, want canonical name", page.ImagePath)
		}
	}
	for _, call := range runner.calls {
		if !containsArgs(call.args, "-png") || !containsArgs(call.args, "-r", "200") {
			t.Fatalf("call args = %#v, want -png -r 200", call.args)
		}
		if !containsArgs(call.args, "-f", call.args[4]) || !containsArgs(call.args, "-l", call.args[4]) {
			t.Fatalf("call args = %#v, want selected page bounds", call.args)
		}
	}
}

func TestRenderPagesAllPagesNormalizesVariablePaddingAndCleansTempFiles(t *testing.T) {
	outputDir := t.TempDir()
	runner := &fakeCommandRunner{run: func(name string, args []string) error {
		if name != "pdftoppm" {
			t.Fatalf("command = %q, want pdftoppm", name)
		}
		prefix := args[len(args)-1]
		for _, suffix := range []string{"12", "003", "5"} {
			if err := os.WriteFile(prefix+"-"+suffix+".png", []byte("image"), 0644); err != nil {
				return err
			}
		}
		return nil
	}}

	got, err := RenderPages(context.Background(), runner, "catalog.pdf", outputDir, nil)
	if err != nil {
		t.Fatalf("RenderPages: %v", err)
	}
	wantNumbers := []int{3, 5, 12}
	if len(got) != len(wantNumbers) {
		t.Fatalf("pages = %#v, want %d pages", got, len(wantNumbers))
	}
	for i, page := range got {
		if page.Number != wantNumbers[i] {
			t.Errorf("page %d number = %d, want %d", i, page.Number, wantNumbers[i])
		}
		if filepath.Base(page.ImagePath) != fmt.Sprintf("page-%03d.png", wantNumbers[i]) {
			t.Errorf("page %d path = %q, want canonical name", i, page.ImagePath)
		}
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != len(wantNumbers) {
		t.Fatalf("output entries = %d, want only canonical images", len(entries))
	}
}

func TestRenderPagesCleansTempFilesOnCommandError(t *testing.T) {
	outputDir := t.TempDir()
	var prefix string
	runner := &fakeCommandRunner{run: func(name string, args []string) error {
		prefix = args[len(args)-1]
		return errors.New("pdftoppm failed")
	}}

	if _, err := RenderPages(context.Background(), runner, "catalog.pdf", outputDir, []int{5}); err == nil {
		t.Fatal("RenderPages succeeded, want command error")
	}
	if prefix == "" {
		t.Fatal("fake runner did not observe render prefix")
	}
	if _, err := os.Stat(filepath.Dir(prefix)); !os.IsNotExist(err) {
		t.Fatalf("temporary render directory stat error = %v, want not exist", err)
	}
}

func containsArgs(args []string, want ...string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		match := true
		for j := range want {
			if args[i+j] != want[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
