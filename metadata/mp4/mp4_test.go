package mp4

import (
	"path/filepath"
	"testing"
)

func TestParseMissingMP4ReturnsError(t *testing.T) {
	if _, err := Parse(filepath.Join(t.TempDir(), "missing.mp4")); err == nil {
		t.Fatal("Parse() accepted a missing MP4 file")
	}
}
