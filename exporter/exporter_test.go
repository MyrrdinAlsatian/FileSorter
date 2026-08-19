package exporter

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"FileRecoveryOrganizer/types"
)

func TestByType(t *testing.T) {
	filter := ByType("jpg", "png")
	if !filter(types.Result{Type: "jpg"}) || filter(types.Result{Type: "mp4"}) {
		t.Fatal("ByType returned an unexpected result")
	}
}

func TestJSONExporterWriteAndAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.jsonl")
	first := types.Result{Path: "a.jpg", Type: "jpg", Size: 10}
	second := types.Result{Path: "b.jpg", Type: "jpg", Size: 20}

	exporter, err := NewJSONExporter(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := exporter.Write(first); err != nil {
		t.Fatal(err)
	}
	if err := exporter.Close(); err != nil {
		t.Fatal(err)
	}

	exporter, err = NewJSONExporterAppend(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := exporter.Write(second); err != nil {
		t.Fatal(err)
	}
	if err := exporter.Close(); err != nil {
		t.Fatal(err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var results []types.Result
	for scanner.Scan() {
		var result types.Result
		if err := json.Unmarshal(scanner.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		results = append(results, result)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[1].Path != second.Path {
		t.Fatalf("exported results = %#v", results)
	}
}

func TestStreamJSONLWithFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filtered.jsonl")
	results := make(chan types.Result, 2)
	results <- types.Result{Path: "photo.jpg", Type: "jpg"}
	results <- types.Result{Path: "video.mp4", Type: "mp4"}
	close(results)

	if err := StreamJSONLWithFilter(path, results, ByType("jpg")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || string(data) == "\n" {
		t.Fatal("filtered export is empty")
	}
	if string(data) != "{\"path\":\"photo.jpg\",\"size\":0,\"type\":\"jpg\"}\n" {
		t.Fatalf("unexpected filtered JSONL: %s", data)
	}
}
