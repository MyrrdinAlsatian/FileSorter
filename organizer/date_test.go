// Package organizer - Tests unitaires pour l'organisation par date.
package organizer

import (
	"testing"
	"time"

	"FileRecoveryOrganizer/metadata"
	"FileRecoveryOrganizer/types"
)

// ═══════════════════════════════════════════════════════════════════════════
// TESTS DE buildDatePath
// ═══════════════════════════════════════════════════════════════════════════

func TestBuildDatePath(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		org      DateOrganization
		expected string
	}{
		{
			name:     "YearMonth - date valide",
			time:     time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			org:      DateYearMonth,
			expected: "2024/01",
		},
		{
			name:     "YearMonth - décembre",
			time:     time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC),
			org:      DateYearMonth,
			expected: "2023/12",
		},
		{
			name:     "Year only",
			time:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			org:      DateYear,
			expected: "2024",
		},
		{
			name:     "YearMonthDay",
			time:     time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			org:      DateYearMonthDay,
			expected: "2024/01/05",
		},
		{
			name:     "Date zéro",
			time:     time.Time{},
			org:      DateYearMonth,
			expected: "unknown_date",
		},
		{
			name:     "Année trop ancienne",
			time:     time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC),
			org:      DateYearMonth,
			expected: "unknown_date",
		},
		{
			name:     "Année future aberrante",
			time:     time.Date(2150, 1, 1, 0, 0, 0, 0, time.UTC),
			org:      DateYearMonth,
			expected: "unknown_date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := Options{
				Organization:      tt.org,
				UnknownDateFolder: "unknown_date",
			}
			result := buildDatePath(tt.time, opts)
			if result != tt.expected {
				t.Errorf("buildDatePath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TESTS DE OrganizeByDate
// ═══════════════════════════════════════════════════════════════════════════

func TestOrganizeByDate(t *testing.T) {
	// Créer un résultat avec métadonnées de date
	result := &types.Result{
		Path: "/test/photo.jpg",
		Type: "jpg",
		AdditionalInfo: &metadata.FileData{
			Time:   time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			Source: "exif:DateTimeOriginal",
			Valid:  true,
		},
	}

	tests := []struct {
		name         string
		categoryPath string
		opts         Options
		expected     string
	}{
		{
			name:         "DateNone - pas de changement",
			categoryPath: "images/originals",
			opts:         Options{Organization: DateNone},
			expected:     "images/originals",
		},
		{
			name:         "YearMonth avec catégorie",
			categoryPath: "images/originals",
			opts: Options{
				Organization:      DateYearMonth,
				Source:            DateSourceAuto,
				UnknownDateFolder: "unknown_date",
				IncludeCategory:   true,
			},
			expected: "images/originals/2024/03",
		},
		{
			name:         "Year sans catégorie",
			categoryPath: "images/originals",
			opts: Options{
				Organization:      DateYear,
				Source:            DateSourceAuto,
				UnknownDateFolder: "unknown_date",
				IncludeCategory:   false,
			},
			expected: "2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OrganizeByDate(result, tt.categoryPath, tt.opts)
			if got != tt.expected {
				t.Errorf("OrganizeByDate() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TESTS DE ParseDateOrganization
// ═══════════════════════════════════════════════════════════════════════════

func TestParseDateOrganization(t *testing.T) {
	tests := []struct {
		input    string
		expected DateOrganization
	}{
		{"none", DateNone},
		{"off", DateNone},
		{"", DateNone},
		{"year", DateYear},
		{"y", DateYear},
		{"YYYY", DateYear},
		{"year-month", DateYearMonth},
		{"ym", DateYearMonth},
		{"yearmonth", DateYearMonth},
		{"YYYY/MM", DateYearMonth},
		{"year-month-day", DateYearMonthDay},
		{"ymd", DateYearMonthDay},
		{"YYYY/MM/DD", DateYearMonthDay},
		{"unknown", DateYearMonth}, // Défaut
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseDateOrganization(tt.input)
			if got != tt.expected {
				t.Errorf("ParseDateOrganization(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TESTS DE String()
// ═══════════════════════════════════════════════════════════════════════════

func TestDateOrganization_String(t *testing.T) {
	tests := []struct {
		org      DateOrganization
		contains string
	}{
		{DateNone, "none"},
		{DateYear, "year"},
		{DateYearMonth, "year-month"},
		{DateYearMonthDay, "year-month-day"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			got := tt.org.String()
			if got == "" {
				t.Errorf("String() returned empty for %v", tt.org)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TESTS AVEC RÉSULTAT SANS MÉTADONNÉES
// ═══════════════════════════════════════════════════════════════════════════

func TestOrganizeByDate_NoMetadata(t *testing.T) {
	// Résultat sans métadonnées de date
	result := &types.Result{
		Path:           "/test/file.txt",
		Type:           "txt",
		AdditionalInfo: nil,
	}

	opts := Options{
		Organization:      DateYearMonth,
		Source:            DateSourceMetadata, // Force métadonnées uniquement
		UnknownDateFolder: "sans_date",
		IncludeCategory:   true,
	}

	got := OrganizeByDate(result, "documents", opts)
	expected := "documents/sans_date"

	if got != expected {
		t.Errorf("OrganizeByDate() sans métadonnées = %q, want %q", got, expected)
	}
}
