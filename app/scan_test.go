package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"FileRecoveryOrganizer/scanner"
	"FileRecoveryOrganizer/types"
)

func TestScanAndExportWritesResults(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	if err := os.Mkdir(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(sourceDir, "note.txt")
	if err := os.WriteFile(sourcePath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(root, "scan.jsonl")
	var callbackResults []types.Result

	outcome, err := ScanAndExport(context.Background(), ScanRequest{
		SourceDir:   sourceDir,
		ExportPath:  exportPath,
		Workers:     1,
		ScanOptions: scanner.DefaultScanOptions(),
		OnResult: func(result types.Result) {
			callbackResults = append(callbackResults, result)
		},
	})
	if err != nil {
		t.Fatalf("ScanAndExport error = %v", err)
	}
	if outcome.ExportError != nil {
		t.Fatalf("export error = %v", outcome.ExportError)
	}
	if outcome.ResultCount != 1 || outcome.Stats.TotalFiles != 1 || len(callbackResults) != 1 {
		t.Fatalf("outcome = (%d results, %d files), callback received %d; want 1 each", outcome.ResultCount, outcome.Stats.TotalFiles, len(callbackResults))
	}

	file, err := os.Open(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var exported types.Result
	if err := json.NewDecoder(bufio.NewReader(file)).Decode(&exported); err != nil {
		t.Fatalf("decode exported JSONL result: %v", err)
	}
	if exported.Path != sourcePath || exported.Size != 5 {
		t.Fatalf("exported result = (%q, %d bytes), want (%q, 5 bytes)", exported.Path, exported.Size, sourcePath)
	}
}

func TestScanAndExportCancellationFlushesResults(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "source")
	if err := os.Mkdir(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 256; i++ {
		path := filepath.Join(sourceDir, fmt.Sprintf("file-%03d.txt", i))
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	exportPath := filepath.Join(root, "scan.jsonl")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	outcome, err := ScanAndExport(ctx, ScanRequest{
		SourceDir:   sourceDir,
		ExportPath:  exportPath,
		Workers:     1,
		ScanOptions: scanner.DefaultScanOptions(),
		OnProgress:  cancel,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ScanAndExport error = %v, want context.Canceled", err)
	}
	if outcome.ExportError != nil {
		t.Fatalf("export error = %v", outcome.ExportError)
	}
	if outcome.ResultCount == 0 {
		t.Fatal("expected at least one result to be flushed before cancellation")
	}

	file, err := os.Open(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	lineCount := 0
	for scanner := bufio.NewScanner(file); scanner.Scan(); {
		var result types.Result
		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			t.Fatalf("invalid JSONL line after cancellation: %v", err)
		}
		lineCount++
	}
	if lineCount != outcome.ResultCount {
		t.Fatalf("exported %d lines, outcome reports %d", lineCount, outcome.ResultCount)
	}
}
