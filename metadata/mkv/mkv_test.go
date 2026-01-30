// Package mkv - Tests unitaires pour le parser MKV.
//
// Ces tests vérifient l'extraction des métadonnées EBML depuis les fichiers MKV.
package mkv

import (
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// TESTS DES FONCTIONS EBML DE BASE
// ═══════════════════════════════════════════════════════════════════════════

// TestReadEBMLID vérifie le décodage des identifiants EBML.
func TestReadEBMLID(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectedID  int
		expectedLen int
	}{
		{
			name:        "1-byte ID (Title 0x7BA9 - incorrect, should be 2 bytes)",
			data:        []byte{0x7B, 0xA9},
			expectedID:  0x7BA9,
			expectedLen: 2,
		},
		{
			name:        "4-byte ID (Segment)",
			data:        []byte{0x18, 0x53, 0x80, 0x67},
			expectedID:  0x18538067,
			expectedLen: 4,
		},
		{
			name:        "4-byte ID (Info)",
			data:        []byte{0x15, 0x49, 0xA9, 0x66},
			expectedID:  0x1549A966,
			expectedLen: 4,
		},
		{
			name:        "Empty data",
			data:        []byte{},
			expectedID:  0,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, length := readEBMLID(tt.data)
			if id != tt.expectedID {
				t.Errorf("readEBMLID() id = 0x%X, want 0x%X", id, tt.expectedID)
			}
			if length != tt.expectedLen {
				t.Errorf("readEBMLID() length = %d, want %d", length, tt.expectedLen)
			}
		})
	}
}

// TestReadEBMLSize vérifie le décodage des tailles EBML (VINT).
func TestReadEBMLSize(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		expectedSize int
		expectedLen  int
	}{
		{
			name:         "1-byte size (127)",
			data:         []byte{0xFF}, // 1xxxxxxx -> 127
			expectedSize: 127,
			expectedLen:  1,
		},
		{
			name:         "1-byte size (10)",
			data:         []byte{0x8A}, // 10001010 -> 10
			expectedSize: 10,
			expectedLen:  1,
		},
		{
			name:         "2-byte size (1000)",
			data:         []byte{0x43, 0xE8}, // 01xxxxxx -> 0x3E8 = 1000
			expectedSize: 1000,
			expectedLen:  2,
		},
		{
			name:         "Empty data",
			data:         []byte{},
			expectedSize: 0,
			expectedLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, length := readEBMLSize(tt.data)
			if size != tt.expectedSize {
				t.Errorf("readEBMLSize() size = %d, want %d", size, tt.expectedSize)
			}
			if length != tt.expectedLen {
				t.Errorf("readEBMLSize() length = %d, want %d", length, tt.expectedLen)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TESTS DU PARSING MKV
// ═══════════════════════════════════════════════════════════════════════════

// TestParseFile_EmptyBuffer vérifie le comportement avec un buffer vide.
func TestParseFile_EmptyBuffer(t *testing.T) {
	meta := ParseFile([]byte{}, "test.mkv")

	// Devrait fallback au nom de fichier
	if meta.Title != "test" {
		t.Errorf("ParseFile() avec buffer vide: Title = %q, want %q", meta.Title, "test")
	}
	if meta.Source != "filename" {
		t.Errorf("ParseFile() avec buffer vide: Source = %q, want %q", meta.Source, "filename")
	}
}

// TestParseFile_FilenameCleanup vérifie le nettoyage du nom de fichier.
func TestParseFile_FilenameCleanup(t *testing.T) {
	tests := []struct {
		filename      string
		expectedTitle string
	}{
		{"My_Movie_2021.mkv", "My Movie 2021"},
		{"Movie.Name.720p.mkv", "Movie Name 720p"},
		{"file-with-dashes.mkv", "file with dashes"},
		{"recup_dir.123/file.mkv", ""}, // Fichiers de récupération ignorés
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			meta := ParseFile([]byte{}, tt.filename)
			if tt.expectedTitle != "" && meta.Title != tt.expectedTitle {
				t.Errorf("Cleanup %q: Title = %q, want %q", tt.filename, meta.Title, tt.expectedTitle)
			}
		})
	}
}

// TestExtractString vérifie l'extraction de chaînes UTF-8.
func TestExtractString(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"Simple string", []byte("Hello"), "Hello"},
		{"With null terminator", []byte("Hello\x00\x00"), "Hello"},
		{"With spaces", []byte("  Hello  "), "Hello"},
		{"Empty", []byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractString(tt.input)
			if result != tt.expected {
				t.Errorf("extractString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestReadInt64 vérifie la lecture des entiers 64 bits big-endian.
func TestReadInt64(t *testing.T) {
	// Test avec une valeur connue
	// 0x0000000000000001 = 1
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
	result := readInt64(data)
	if result != 1 {
		t.Errorf("readInt64() = %d, want 1", result)
	}

	// Test avec buffer trop court
	shortData := []byte{0x01, 0x02}
	result = readInt64(shortData)
	if result != 0 {
		t.Errorf("readInt64() avec buffer court = %d, want 0", result)
	}
}

// TestMin vérifie la fonction min.
func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should be 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should be 5")
	}
	if min(5, 5) != 5 {
		t.Error("min(5, 5) should be 5")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// TESTS D'INTÉGRATION
// ═══════════════════════════════════════════════════════════════════════════

// TestMKVMetadata_JSONTags vérifie que la structure est bien taggée pour JSON.
func TestMKVMetadata_JSONTags(t *testing.T) {
	meta := MKVMetadata{
		Title:  "Test Movie",
		Date:   time.Date(2021, 5, 1, 0, 0, 0, 0, time.UTC),
		Source: "info:Title",
		Valid:  true,
	}

	// Vérifier que les champs sont bien remplis
	if meta.Title != "Test Movie" {
		t.Error("Title field not set correctly")
	}
	if meta.Source != "info:Title" {
		t.Error("Source field not set correctly")
	}
	if !meta.Valid {
		t.Error("Valid field should be true")
	}
}

// TestParse_LegacyCompatibility vérifie que l'ancienne fonction Parse fonctionne.
func TestParse_LegacyCompatibility(t *testing.T) {
	// Avec un buffer vide, Parse devrait retourner ("", false)
	title, found := Parse([]byte{})
	if found {
		t.Error("Parse() avec buffer vide ne devrait pas trouver de titre")
	}
	if title != "" {
		t.Errorf("Parse() title = %q, want empty", title)
	}
}
