package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestComputeFullHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.bin")
	content := []byte("hash me")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ComputeFullHash(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(content)
	if got != hex.EncodeToString(hash[:]) {
		t.Fatalf("ComputeFullHash() = %q", got)
	}
}

func TestComputeQuickHashSmallFileMatchesFullHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.bin")
	content := []byte("small file")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	quick, err := ComputeQuickHash(path, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	full, err := ComputeFullHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if quick != full {
		t.Fatalf("quick hash = %q, full hash = %q", quick, full)
	}
}

func TestDuplicateFinderGroupsIdenticalFiles(t *testing.T) {
	dir := t.TempDir()
	content := make([]byte, SmallFileThreshold+1024)
	for i := range content {
		content[i] = byte(i % 251)
	}
	first := filepath.Join(dir, "first.bin")
	second := filepath.Join(dir, "second.bin")
	if err := os.WriteFile(first, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, content, 0644); err != nil {
		t.Fatal(err)
	}

	finder := NewDuplicateFinderWithOptions(FinderOptions{MinSize: 0, Workers: 2})
	finder.AddFile(first, int64(len(content)))
	finder.AddFile(second, int64(len(content)))
	report := finder.FindDuplicates()
	if report.DuplicateGroups != 1 || report.DuplicateFiles != 2 {
		t.Fatalf("duplicate report = %#v", report)
	}
	if report.WastedSpace != int64(len(content)) {
		t.Fatalf("wasted space = %d, want %d", report.WastedSpace, len(content))
	}
}
