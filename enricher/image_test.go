package enricher

import (
	"path/filepath"
	"testing"

	"FileRecoveryOrganizer/types"
)

func TestEnrichImageMissingFileDoesNotPanic(t *testing.T) {
	result := &types.Result{Path: filepath.Join(t.TempDir(), "missing.jpg")}
	EnrichImage(result)
	if result.Image != nil {
		t.Fatal("missing image should not be enriched")
	}
}
