package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"FileRecoveryOrganizer/types"
)

func TestGenerateHTMLCreatesEscapedReport(t *testing.T) {
	output := filepath.Join(t.TempDir(), "report.html")
	results := []types.Result{{
		Path:       filepath.Join(t.TempDir(), "photo.jpg"),
		Size:       1024,
		Type:       "jpg",
		TargetPath: "images/originals",
	}}
	if err := GenerateHTML(results, "source", output, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "photo.jpg") || !strings.Contains(string(data), "1.00 KB") {
		t.Fatal("generated report does not contain expected data")
	}
}
