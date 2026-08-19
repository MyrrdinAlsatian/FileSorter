package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileTypeHelpers(t *testing.T) {
	if !IsImageType("jpg") || IsImageType("txt") || !IsVideoType("mp4") || !IsAudioType("mp3") {
		t.Fatal("file type helpers returned unexpected values")
	}
}

func TestFileSystemDateAndBestDate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	date := FileSystemDate(path)
	if !date.Valid || date.Source != "filesystem:modification" {
		t.Fatalf("FileSystemDate() = %#v", date)
	}
	if got := BestDate(path, false); !got.Valid {
		t.Fatalf("BestDate() = %#v", got)
	}
}

func TestFileSystemDateMissingFile(t *testing.T) {
	if got := FileSystemDate(filepath.Join(t.TempDir(), "missing")); got.Valid {
		t.Fatal("missing file returned a valid date")
	}
}
