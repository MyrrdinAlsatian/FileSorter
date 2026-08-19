package dedup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectOriginalKeepByPathFallsBackToShortest(t *testing.T) {
	d := NewDeduplicator(DeduplicateOptions{
		Strategy: KeepByPath,
	})

	paths := []string{
		filepath.Join("root", "a", "photo.jpg"),
		filepath.Join("root", "photo.jpg"),
	}

	if got, want := d.selectOriginal(paths), paths[1]; got != want {
		t.Fatalf("selectOriginal() = %q, want %q", got, want)
	}
}

func TestContainsPathRespectsDirectoryBoundaries(t *testing.T) {
	if !containsPath(filepath.Join("root", "sorted", "photo.jpg"), filepath.Join("root", "sorted")) {
		t.Fatal("containsPath() should match a child path")
	}
	if containsPath(filepath.Join("root", "sorted2", "photo.jpg"), filepath.Join("root", "sorted")) {
		t.Fatal("containsPath() must not match a directory prefix")
	}
}

func TestReplaceWithHardlinkPreservesContent(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "original.txt")
	duplicate := filepath.Join(dir, "duplicate.txt")
	content := []byte("same content")

	if err := os.WriteFile(original, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(duplicate, content, 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDeduplicator(DeduplicateOptions{})
	if err := d.replaceWithHardlink(duplicate, original); err != nil {
		t.Fatalf("replaceWithHardlink() error = %v", err)
	}

	got, err := os.ReadFile(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("duplicate content = %q, want %q", got, content)
	}

	originalInfo, err := os.Stat(original)
	if err != nil {
		t.Fatal(err)
	}
	duplicateInfo, err := os.Stat(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(originalInfo, duplicateInfo) {
		t.Fatal("duplicate is not a hardlink to the original")
	}
}

func TestReplaceWithLinkFailureLeavesOriginalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := []byte("must survive")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDeduplicator(DeduplicateOptions{})
	wantErr := errors.New("simulated link creation failure")
	err := d.replaceWithLink(path, func(string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("replaceWithLink() error = %v, want wrapped %v", err, wantErr)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("original content = %q, want %q", got, content)
	}
}

func TestProcessFileDryRunDoesNotModifyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	content := []byte("dry run")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	d := NewDeduplicator(DeduplicateOptions{
		Action: ActionDryRun,
	})
	if err := d.processFile(path, "original.txt"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("file content = %q, want %q", got, content)
	}
}
