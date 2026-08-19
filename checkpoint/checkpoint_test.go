package checkpoint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessedFilesAddIsIdempotent(t *testing.T) {
	pf := NewProcessedFiles()
	pf.Add("a.jpg")
	pf.Add("a.jpg")
	if pf.Count() != 1 || !pf.IsProcessed("a.jpg") {
		t.Fatalf("tracker = count %d, processed %v", pf.Count(), pf.IsProcessed("a.jpg"))
	}
	if pf.IsProcessed("b.jpg") {
		t.Fatal("an unknown path is marked as processed")
	}
}

func TestLoadFromJSONLIgnoresInvalidLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "scan.jsonl")
	content := "{\"path\":\"a.jpg\"}\nnot-json\n{\"other\":true}\n{\"path\":\"b.jpg\"}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	pf := NewProcessedFiles()
	if err := pf.LoadFromJSONL(path); err != nil {
		t.Fatal(err)
	}
	if pf.Count() != 2 || !pf.IsProcessed("b.jpg") {
		t.Fatalf("loaded tracker = count %d", pf.Count())
	}
}

func TestLoadFromJSONLMissingFileIsAllowed(t *testing.T) {
	if err := NewProcessedFiles().LoadFromJSONL(filepath.Join(t.TempDir(), "missing.jsonl")); err != nil {
		t.Fatalf("missing checkpoint error = %v", err)
	}
}
