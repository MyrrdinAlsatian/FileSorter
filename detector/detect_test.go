package detector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	// Créer un répertoire temporaire pour les tests
	tmpDir, err := os.MkdirTemp("", "detector_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name     string
		content  []byte
		filename string
		expected string
	}{
		{
			name:     "JPEG file",
			content:  []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'},
			filename: "test.jpg",
			expected: "jpg",
		},
		{
			name:     "PNG file",
			content:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			filename: "test.png",
			expected: "png",
		},
		{
			name:     "PDF file",
			content:  []byte{'%', 'P', 'D', 'F', '-', '1', '.', '4'},
			filename: "test.pdf",
			expected: "pdf",
		},
		{
			name:     "ZIP file",
			content:  []byte{0x50, 0x4B, 0x03, 0x04},
			filename: "test.zip",
			expected: "zip",
		},
		{
			name:     "Empty file",
			content:  []byte{},
			filename: "empty.txt",
			expected: "txt", // Fallback to extension
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Créer le fichier de test
			path := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(path, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			result := Detect(path)

			// Pour les fichiers vides, on accepte plusieurs résultats valides
			if tt.name == "Empty file" {
				if result != "txt" && result != "empty" && result != "unknown" {
					t.Errorf("Detect(%q) = %q, want txt, empty, or unknown", tt.filename, result)
				}
				return
			}

			if result != tt.expected {
				t.Errorf("Detect(%q) = %q, want %q", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestDetectPattern(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected string
	}{
		{
			name:     "JSON object",
			content:  []byte(`{"key": "value"}`),
			expected: "json",
		},
		{
			name:     "JSON array",
			content:  []byte(`[1, 2, 3]`),
			expected: "json",
		},
		{
			name:     "XML document",
			content:  []byte(`<?xml version="1.0"?><root></root>`),
			expected: "xml",
		},
		{
			name:     "HTML document",
			content:  []byte(`<!DOCTYPE html><html><head></head></html>`),
			expected: "html",
		},
		{
			name:     "Binary file",
			content:  []byte{0x00, 0x01, 0x02, 0x03},
			expected: "binary",
		},
		{
			name:     "Empty content",
			content:  []byte{},
			expected: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectPattern(tt.content)
			if result != tt.expected {
				t.Errorf("detectPattern() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDetectNonExistentFile(t *testing.T) {
	result := Detect("/non/existent/file.txt")
	if result != "Error opening file" {
		t.Errorf("Detect() for non-existent file = %q, want \"Error opening file\"", result)
	}
}
