package main

import (
	"context"
	"fmt"
	"strings"
)

func OCRPages(ctx context.Context, runner CommandRunner, pages []Page) ([]OCRPage, error) {
	if runner == nil {
		return nil, fmt.Errorf("OCR pages: nil command runner")
	}
	ocr := make([]OCRPage, 0, len(pages))
	for _, page := range pages {
		stdout, stderr, err := runner.Run(ctx, "tesseract", page.ImagePath, "stdout", "-l", "eng", "--psm", "6")
		if err != nil {
			return nil, commandError(fmt.Sprintf("OCR page %d", page.Number), err, stderr)
		}
		text := strings.TrimRight(string(stdout), " \t\r\n")
		if strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("OCR page %d: blank OCR output", page.Number)
		}
		ocr = append(ocr, OCRPage{Number: page.Number, Text: text})
	}
	return ocr, nil
}
