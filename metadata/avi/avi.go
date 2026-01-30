// Package avi extrait les métadonnées des fichiers vidéo AVI.
//
// CONCEPT : Format RIFF (Resource Interchange File Format)
// ========================================================
// AVI est basé sur le format RIFF de Microsoft, aussi utilisé par WAV.
// Structure en "chunks" (blocs) :
//
//	┌─ RIFF header
//	│  ├─ "RIFF" (4 bytes)
//	│  ├─ Taille totale (4 bytes, little-endian)
//	│  └─ "AVI " (4 bytes) - type de fichier
//	│
//	├─ LIST "hdrl" (header list)
//	│  └─ avih (AVI header - durée, dimensions)
//	│
//	├─ LIST "movi" (movie data)
//	│
//	└─ LIST "INFO" (métadonnées textuelles) ◄── C'est ici !
//	   ├─ INAM : Title (nom)
//	   ├─ IART : Artist
//	   ├─ ICRD : Date de création
//	   ├─ ICMT : Commentaire
//	   └─ ...
//
// Le titre se trouve dans le chunk INFO → INAM
package avi

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"strings"
	"time"
)

// ScanSize définit la taille du buffer de lecture (512KB)
const ScanSize = 512 * 1024

// AVIMetadata contient les métadonnées extraites d'un fichier AVI.
type AVIMetadata struct {
	Title   string    `json:"title,omitempty"`
	Artist  string    `json:"artist,omitempty"`
	Date    time.Time `json:"date,omitempty"`
	Comment string    `json:"comment,omitempty"`
	Source  string    `json:"source,omitempty"`
	Valid   bool      `json:"valid"`
}

// Identifiants des chunks RIFF
var (
	riffMagic = []byte("RIFF")
	aviType   = []byte("AVI ")
	listChunk = []byte("LIST")
	infoType  = []byte("INFO")

	// Tags INFO courants
	tagINAM = []byte("INAM") // Title
	tagIART = []byte("IART") // Artist
	tagICRD = []byte("ICRD") // Creation date
	tagICMT = []byte("ICMT") // Comment
	tagIPRD = []byte("IPRD") // Product (parfois utilisé comme titre)
)

// ParseFile extrait les métadonnées d'un fichier AVI.
//
// Paramètres :
//   - buf : contenu du fichier (premiers 512KB)
//   - filename : nom du fichier pour fallback
//
// Retour :
//   - *AVIMetadata : métadonnées extraites
func ParseFile(buf []byte, filename string) *AVIMetadata {
	meta := &AVIMetadata{}

	// Vérifier la signature RIFF
	if len(buf) < 12 {
		return fallbackToFilename(meta, filename)
	}

	if !bytes.Equal(buf[0:4], riffMagic) {
		return fallbackToFilename(meta, filename)
	}

	// Vérifier le type AVI
	if !bytes.Equal(buf[8:12], aviType) {
		return fallbackToFilename(meta, filename)
	}

	// Chercher le chunk LIST "INFO"
	pos := 12 // Après le header RIFF

	for pos < len(buf)-8 {
		// Lire l'identifiant du chunk (4 bytes)
		chunkID := buf[pos : pos+4]

		// Lire la taille du chunk (4 bytes, little-endian)
		chunkSize := int(binary.LittleEndian.Uint32(buf[pos+4 : pos+8]))

		if chunkSize <= 0 || chunkSize > len(buf)-pos-8 {
			break
		}

		// Si c'est un LIST, vérifier son type
		if bytes.Equal(chunkID, listChunk) && pos+12 <= len(buf) {
			listType := buf[pos+8 : pos+12]

			if bytes.Equal(listType, infoType) {
				// Parser le contenu INFO
				parseINFO(buf[pos+12:pos+8+chunkSize], meta)
				if meta.Title != "" {
					meta.Valid = true
					return meta
				}
			}
		}

		// Passer au chunk suivant (aligné sur 2 bytes)
		pos += 8 + chunkSize
		if chunkSize%2 != 0 {
			pos++ // Padding byte
		}
	}

	// Fallback au nom de fichier
	if meta.Title == "" {
		return fallbackToFilename(meta, filename)
	}

	meta.Valid = true
	return meta
}

// parseINFO extrait les métadonnées du chunk INFO.
func parseINFO(data []byte, meta *AVIMetadata) {
	pos := 0

	for pos < len(data)-8 {
		// Lire l'identifiant du sous-chunk
		tagID := data[pos : pos+4]

		// Lire la taille
		tagSize := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))

		if tagSize <= 0 || tagSize > len(data)-pos-8 {
			break
		}

		// Extraire le contenu texte
		content := extractString(data[pos+8 : pos+8+tagSize])

		// Assigner selon le type de tag
		switch {
		case bytes.Equal(tagID, tagINAM):
			meta.Title = content
			meta.Source = "avi:INFO:INAM"
		case bytes.Equal(tagID, tagIART):
			meta.Artist = content
		case bytes.Equal(tagID, tagICRD):
			// Essayer de parser la date (format variable)
			if t, err := parseDate(content); err == nil {
				meta.Date = t
			}
		case bytes.Equal(tagID, tagICMT):
			meta.Comment = content
		case bytes.Equal(tagID, tagIPRD) && meta.Title == "":
			// Utiliser IPRD comme titre si INAM absent
			meta.Title = content
			meta.Source = "avi:INFO:IPRD"
		}

		// Passer au tag suivant (aligné sur 2 bytes)
		pos += 8 + tagSize
		if tagSize%2 != 0 {
			pos++
		}
	}
}

// extractString nettoie une chaîne en supprimant les caractères nuls
func extractString(data []byte) string {
	// Trouver la fin de la chaîne (null terminator)
	end := bytes.IndexByte(data, 0)
	if end == -1 {
		end = len(data)
	}

	s := string(data[:end])
	return strings.TrimSpace(s)
}

// parseDate essaie de parser une date dans différents formats courants
func parseDate(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"01/02/2006",
		"02-01-2006",
		"2006",
		"January 2, 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, nil
}

// fallbackToFilename utilise le nom de fichier comme titre
func fallbackToFilename(meta *AVIMetadata, filename string) *AVIMetadata {
	if filename == "" {
		return meta
	}

	// Ignorer les fichiers de récupération avec noms cryptiques
	if strings.Contains(strings.ToLower(filename), "recup_dir") {
		return meta
	}

	// Extraire le nom sans extension
	name := filepath.Base(filename)
	ext := filepath.Ext(name)
	if ext != "" {
		name = name[:len(name)-len(ext)]
	}

	// Nettoyer le nom (remplacer _ et . par des espaces)
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// Si le nom est juste des chiffres (fichier de récupération), ignorer
	isNumeric := true
	for _, c := range name {
		if c != ' ' && (c < '0' || c > '9') {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		return meta
	}

	meta.Title = strings.TrimSpace(name)
	meta.Source = "filename"
	meta.Valid = meta.Title != ""

	return meta
}

// Parse est la fonction legacy pour compatibilité
func Parse(buf []byte) (string, bool) {
	meta := ParseFile(buf, "")
	return meta.Title, meta.Title != ""
}
