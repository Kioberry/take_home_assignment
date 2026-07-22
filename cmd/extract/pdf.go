package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr []byte, err error)
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return []byte(stdout.String()), []byte(stderr.String()), err
}

func ParsePageRange(value string, max int) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" || max < 1 {
		return nil, fmt.Errorf("invalid page range %q", value)
	}
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return nil, fmt.Errorf("invalid page range %q", value)
	}
	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid page range %q: %w", value, err)
	}
	end := start
	if len(parts) == 2 {
		end, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid page range %q: %w", value, err)
		}
	}
	if start < 1 || end < start || end > max {
		return nil, fmt.Errorf("page range %q is outside 1-%d", value, max)
	}
	pages := make([]int, 0, end-start+1)
	for page := start; page <= end; page++ {
		pages = append(pages, page)
	}
	return pages, nil
}

func RenderPages(ctx context.Context, runner CommandRunner, pdfPath, outputDir string, selected []int) ([]Page, error) {
	if runner == nil {
		return nil, fmt.Errorf("render pages: nil command runner")
	}
	if pdfPath == "" || outputDir == "" {
		return nil, fmt.Errorf("render pages: pdf path and output directory are required")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create render directory %s: %w", outputDir, err)
	}
	prefix := filepath.Join(outputDir, "page")
	expected := make(map[string]bool, len(selected))
	for _, page := range selected {
		if page < 1 {
			return nil, fmt.Errorf("render page %d: page number must be positive", page)
		}
		args := []string{"-png", "-r", "200", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), pdfPath, prefix}
		_, stderr, err := runner.Run(ctx, "pdftoppm", args...)
		if err != nil {
			return nil, commandError("render page", err, stderr)
		}
		path := filepath.Join(outputDir, fmt.Sprintf("page-%03d.png", page))
		expected[path] = true
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("render page %d: expected output %s: %w", page, path, err)
		}
	}

	paths, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, fmt.Errorf("find rendered pages: %w", err)
	}
	pages := make([]Page, 0, len(paths))
	for _, path := range paths {
		if len(selected) > 0 && !expected[path] {
			continue
		}
		base := strings.TrimSuffix(filepath.Base(path), ".png")
		pageText := strings.TrimPrefix(base, "page-")
		page, parseErr := strconv.Atoi(pageText)
		if parseErr != nil || page < 1 {
			continue
		}
		pages = append(pages, Page{Number: page, ImagePath: path})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Number < pages[j].Number })
	return pages, nil
}

func commandError(operation string, err error, stderr []byte) error {
	if trimmed := strings.TrimSpace(string(stderr)); trimmed != "" {
		return fmt.Errorf("%s: %w (stderr: %s)", operation, err, trimmed)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
