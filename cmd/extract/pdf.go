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
	tempDir, err := os.MkdirTemp(outputDir, ".render-")
	if err != nil {
		return nil, fmt.Errorf("create temporary render directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	prefix := filepath.Join(tempDir, "page")
	for _, page := range selected {
		if page < 1 {
			return nil, fmt.Errorf("render page %d: page number must be positive", page)
		}
		args := []string{"-png", "-r", "200", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), pdfPath, prefix}
		_, stderr, err := runner.Run(ctx, "pdftoppm", args...)
		if err != nil {
			return nil, commandError("render page", err, stderr)
		}
	}
	if len(selected) == 0 {
		args := []string{"-png", "-r", "200", pdfPath, prefix}
		_, stderr, err := runner.Run(ctx, "pdftoppm", args...)
		if err != nil {
			return nil, commandError("render pages", err, stderr)
		}
	}

	paths, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, fmt.Errorf("find rendered pages: %w", err)
	}
	counts := make(map[int]int, len(paths))
	pagePaths := make(map[int]string, len(paths))
	for _, path := range paths {
		base := strings.TrimSuffix(filepath.Base(path), ".png")
		pageText := strings.TrimPrefix(base, "page-")
		page, parseErr := strconv.Atoi(pageText)
		if parseErr != nil || page < 1 {
			return nil, fmt.Errorf("invalid rendered page filename %s", path)
		}
		counts[page]++
		pagePaths[page] = path
	}
	if len(selected) > 0 {
		expected := make(map[int]bool, len(selected))
		for _, page := range selected {
			expected[page] = true
		}
		for page, count := range counts {
			if !expected[page] {
				return nil, fmt.Errorf("unexpected rendered page %d", page)
			}
			if count != 1 {
				return nil, fmt.Errorf("rendered page %d appears %d times, want exactly once", page, count)
			}
		}
		for page := range expected {
			if counts[page] != 1 {
				return nil, fmt.Errorf("rendered page %d appears %d times, want exactly once", page, counts[page])
			}
		}
	}

	pages := make([]Page, 0, len(pagePaths))
	for page, path := range pagePaths {
		canonical := filepath.Join(outputDir, fmt.Sprintf("page-%03d.png", page))
		if err := os.Rename(path, canonical); err != nil {
			return nil, fmt.Errorf("rename rendered page %d to %s: %w", page, canonical, err)
		}
		pages = append(pages, Page{Number: page, ImagePath: canonical})
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
