package validator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidationErrorAndWrapError(t *testing.T) {
	err := NewValidationError("corrupted", "bad data", 12)
	if err.Error() != "corrupted at byte 12: bad data" {
		t.Fatalf("error string = %q", err.Error())
	}
	if WrapError(nil, "ignored") != nil {
		t.Fatal("WrapError(nil) should return nil")
	}
	if got := WrapError(errors.New("boom"), "custom"); got.Type != "custom" || got.Message != "boom" {
		t.Fatalf("wrapped error = %#v", got)
	}
}

func TestValidateFileBasicCases(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.jpg")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatal(err)
	}
	result := ValidateFile(empty)
	if result.Valid || result.Error != ErrEmpty {
		t.Fatalf("empty validation = %#v", result)
	}

	unknown := filepath.Join(dir, "file.custom")
	if err := os.WriteFile(unknown, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	result = ValidateFile(unknown)
	if !result.Valid || result.FileType != "unknown" {
		t.Fatalf("unknown validation = %#v", result)
	}
}

func TestValidationFilters(t *testing.T) {
	results := []ValidationResult{
		{Valid: true},
		{Valid: false, Error: ErrCorrupted},
		{Valid: false, Error: ErrTruncated},
	}
	if len(FilterValid(results)) != 1 || len(FilterCorrupted(results)) != 2 {
		t.Fatal("validation filters returned incorrect counts")
	}
	groups := GroupByErrorType(results)
	if len(groups["corrupted"]) != 1 || len(groups["truncated"]) != 1 {
		t.Fatalf("error groups = %#v", groups)
	}
}
