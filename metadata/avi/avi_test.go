package avi

import "testing"

func TestParseFileFallsBackForInvalidAVI(t *testing.T) {
	meta := ParseFile([]byte("not an avi"), "holiday-video.avi")
	if meta == nil || meta.Title != "holiday video" || !meta.Valid {
		t.Fatalf("invalid AVI metadata = %#v", meta)
	}
	if meta.Source != "filename" {
		t.Fatal("fallback metadata should identify its source")
	}
}
