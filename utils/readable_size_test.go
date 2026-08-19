package utils

import "testing"

func TestReadableSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
	}
	for _, tt := range tests {
		if got := ReadableSize(tt.bytes); got != tt.want {
			t.Errorf("ReadableSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
