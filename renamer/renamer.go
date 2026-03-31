// renamer/renamer.go - Moteur de renommage
//
// Ce fichier contient la logique principale pour appliquer les patterns
// de renommage aux fichiers.
package renamer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"FileRecoveryOrganizer/types"
)

// ═══════════════════════════════════════════════════════════════════════════
// RENAMER
// ═══════════════════════════════════════════════════════════════════════════

// Renamer applique des patterns de renommage aux fichiers.
type Renamer struct {
	Pattern *Pattern

	// Compteur de séquence global
	seqCounter int
	seqMu      sync.Mutex

	// Compteurs par groupe (pour séquences par dossier/date/etc.)
	groupCounters map[string]int
	groupMu       sync.Mutex
}

// NewRenamer crée un nouveau renamer avec le pattern spécifié.
func NewRenamer(patternStr string) (*Renamer, error) {
	pattern, err := ParsePattern(patternStr)
	if err != nil {
		return nil, err
	}

	if err := pattern.Validate(); err != nil {
		return nil, err
	}

	return &Renamer{
		Pattern:       pattern,
		seqCounter:    0,
		groupCounters: make(map[string]int),
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// CONTEXTE DE RENOMMAGE
// ═══════════════════════════════════════════════════════════════════════════

// Context contient toutes les données disponibles pour le renommage.
type Context struct {
	// Fichier source
	OriginalPath string
	OriginalName string
	Extension    string

	// Type et catégorie
	Type     string
	Category string

	// Métadonnées de date
	Date    time.Time
	HasDate bool

	// Métadonnées image
	Camera string
	Width  int
	Height int

	// Hash
	Hash string

	// Taille
	Size int64

	// Séquence
	Sequence int

	// Groupe pour séquence locale (ex: date, dossier)
	Group string
}

// NewContextFromResult crée un contexte à partir d'un types.Result.
func NewContextFromResult(result *types.Result) *Context {
	ctx := &Context{
		OriginalPath: result.Path,
		OriginalName: strings.TrimSuffix(filepath.Base(result.Path), filepath.Ext(result.Path)),
		Extension:    strings.TrimPrefix(filepath.Ext(result.Path), "."),
		Type:         result.Type,
		Size:         result.Size,
		HasDate:      false,
	}

	// Extraire la catégorie du TargetPath
	if result.TargetPath != "" {
		parts := strings.Split(filepath.ToSlash(result.TargetPath), "/")
		if len(parts) >= 2 {
			ctx.Category = parts[0] + "/" + parts[1]
		} else if len(parts) == 1 {
			ctx.Category = parts[0]
		}
	} else {
		ctx.Category = categorizeByType(result.Type)
	}

	// Hash
	if result.FullHash != "" {
		ctx.Hash = result.FullHash
	} else if result.QuickHash != "" {
		ctx.Hash = result.QuickHash
	}

	// Métadonnées image
	if result.Image != nil {
		ctx.Width = result.Image.Width
		ctx.Height = result.Image.Height

		if result.Image.Exif != nil {
			ctx.Camera = sanitizeCamera(result.Image.Exif.CameraModel)

			// Parser la date EXIF
			if result.Image.Exif.DateTaken != "" {
				if t, err := parseExifDate(result.Image.Exif.DateTaken); err == nil {
					ctx.Date = t
					ctx.HasDate = true
				}
			}
		}
	}

	// Date depuis AdditionalInfo
	if !ctx.HasDate && result.AdditionalInfo != nil && result.AdditionalInfo.Valid {
		ctx.Date = result.AdditionalInfo.Time
		ctx.HasDate = true
	}

	return ctx
}

// ═══════════════════════════════════════════════════════════════════════════
// APPLICATION DU PATTERN
// ═══════════════════════════════════════════════════════════════════════════

// Rename génère le nouveau nom de fichier selon le pattern.
func (r *Renamer) Rename(ctx *Context) string {
	// Incrémenter la séquence globale
	r.seqMu.Lock()
	r.seqCounter++
	ctx.Sequence = r.seqCounter
	r.seqMu.Unlock()

	// Construire le nouveau nom
	var result strings.Builder

	for _, token := range r.Pattern.Tokens {
		switch token.Type {
		case TokenLiteral:
			result.WriteString(token.Value)

		case TokenVariable:
			value := r.resolveVariable(token, ctx)
			result.WriteString(value)
		}
	}

	return sanitizeFilename(result.String())
}

// RenameWithGroup génère le nom avec une séquence locale au groupe.
func (r *Renamer) RenameWithGroup(ctx *Context, group string) string {
	ctx.Group = group

	// Incrémenter le compteur du groupe
	r.groupMu.Lock()
	r.groupCounters[group]++
	ctx.Sequence = r.groupCounters[group]
	r.groupMu.Unlock()

	return r.Rename(ctx)
}

// resolveVariable retourne la valeur d'une variable.
func (r *Renamer) resolveVariable(token Token, ctx *Context) string {
	switch token.Value {
	// Date et heure
	case "date":
		if !ctx.HasDate {
			return "unknown_date"
		}
		format := token.Options["format"]
		if format == "" {
			return ctx.Date.Format("2006-01-02")
		}
		return formatDate(ctx.Date, format)

	case "datetime":
		if !ctx.HasDate {
			return "unknown_datetime"
		}
		format := token.Options["format"]
		if format == "" {
			return ctx.Date.Format("2006-01-02_150405")
		}
		return formatDate(ctx.Date, format)

	case "year":
		if !ctx.HasDate {
			return "unknown"
		}
		return ctx.Date.Format("2006")

	case "month":
		if !ctx.HasDate {
			return "unknown"
		}
		return ctx.Date.Format("01")

	case "day":
		if !ctx.HasDate {
			return "unknown"
		}
		return ctx.Date.Format("02")

	case "hour":
		if !ctx.HasDate {
			return "00"
		}
		return ctx.Date.Format("15")

	case "minute":
		if !ctx.HasDate {
			return "00"
		}
		return ctx.Date.Format("04")

	case "second":
		if !ctx.HasDate {
			return "00"
		}
		return ctx.Date.Format("05")

	// Métadonnées
	case "camera":
		if ctx.Camera == "" {
			return "unknown_camera"
		}
		return ctx.Camera

	case "original":
		return ctx.OriginalName

	case "ext":
		if ctx.Extension == "" {
			return ctx.Type
		}
		return strings.ToLower(ctx.Extension)

	case "type":
		return ctx.Type

	case "category":
		return strings.ReplaceAll(ctx.Category, "/", "_")

	// Hash
	case "hash":
		if ctx.Hash == "" {
			return "nohash"
		}
		length := 8
		if l, ok := token.Options["length"]; ok {
			if n, err := strconv.Atoi(l); err == nil {
				length = n
			}
		}
		if len(ctx.Hash) < length {
			return ctx.Hash
		}
		return ctx.Hash[:length]

	// Séquence
	case "seq":
		width := 4
		if w, ok := token.Options["width"]; ok {
			if n, err := strconv.Atoi(w); err == nil {
				width = n
			}
		}
		return fmt.Sprintf("%0*d", width, ctx.Sequence)

	// Dimensions
	case "size":
		return strconv.FormatInt(ctx.Size, 10)

	case "width":
		if ctx.Width == 0 {
			return "0"
		}
		return strconv.Itoa(ctx.Width)

	case "height":
		if ctx.Height == 0 {
			return "0"
		}
		return strconv.Itoa(ctx.Height)

	default:
		return "{" + token.Value + "}"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS UTILITAIRES
// ═══════════════════════════════════════════════════════════════════════════

// sanitizeFilename nettoie un nom de fichier pour le rendre valide.
func sanitizeFilename(name string) string {
	// Caractères interdits sur Windows et Unix
	invalidChars := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	name = invalidChars.ReplaceAllString(name, "_")

	// Remplacer les espaces multiples
	spaces := regexp.MustCompile(`\s+`)
	name = spaces.ReplaceAllString(name, " ")

	// Supprimer les espaces en début et fin
	name = strings.TrimSpace(name)

	// Limiter la longueur (255 caractères max pour la plupart des systèmes)
	if len(name) > 255 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		maxBase := 255 - len(ext)
		if maxBase > 0 {
			name = base[:maxBase] + ext
		}
	}

	// Éviter les noms réservés Windows
	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true,
		"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
		"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}

	baseName := strings.ToUpper(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)))
	if reserved[baseName] {
		name = "_" + name
	}

	return name
}

// sanitizeCamera nettoie le nom de l'appareil photo.
func sanitizeCamera(camera string) string {
	if camera == "" {
		return ""
	}

	// Supprimer les caractères spéciaux
	var result strings.Builder
	for _, r := range camera {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			result.WriteRune(r)
		} else if r == ' ' {
			result.WriteRune('_')
		}
	}

	// Limiter la longueur
	s := result.String()
	if len(s) > 30 {
		s = s[:30]
	}

	return s
}

// parseExifDate parse une date au format EXIF.
func parseExifDate(dateStr string) (time.Time, error) {
	// Format EXIF : "2024:01:15 14:30:52"
	formats := []string{
		"2006:01:02 15:04:05",
		"2006-01-02 15:04:05",
		"2006:01:02",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse date: %s", dateStr)
}

// formatDate formate une date selon un pattern personnalisé.
//
// Remplacements :
//   - YYYY -> année 4 chiffres
//   - YY   -> année 2 chiffres
//   - MM   -> mois 2 chiffres
//   - DD   -> jour 2 chiffres
//   - HH   -> heure 2 chiffres
//   - mm   -> minute 2 chiffres
//   - ss   -> seconde 2 chiffres
func formatDate(t time.Time, pattern string) string {
	replacements := map[string]string{
		"YYYY": t.Format("2006"),
		"YY":   t.Format("06"),
		"MM":   t.Format("01"),
		"DD":   t.Format("02"),
		"HH":   t.Format("15"),
		"mm":   t.Format("04"),
		"ss":   t.Format("05"),
	}

	result := pattern
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

// categorizeByType retourne une catégorie basique selon le type.
func categorizeByType(fileType string) string {
	switch strings.ToLower(fileType) {
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff", "heic", "heif":
		return "images"
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm":
		return "videos"
	case "mp3", "flac", "wav", "ogg", "m4a", "aac":
		return "audio"
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx":
		return "documents"
	default:
		return "other"
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// PRÉVISUALISATION
// ═══════════════════════════════════════════════════════════════════════════

// Preview génère un aperçu du renommage sans l'appliquer.
type Preview struct {
	OriginalPath string
	OriginalName string
	NewName      string
	Variables    map[string]string // Valeurs des variables utilisées
}

// PreviewRename génère un aperçu du renommage.
func (r *Renamer) PreviewRename(ctx *Context) *Preview {
	// Sauvegarder le compteur
	r.seqMu.Lock()
	savedSeq := r.seqCounter
	r.seqMu.Unlock()

	// Générer le nouveau nom
	newName := r.Rename(ctx)

	// Restaurer le compteur (c'est juste un preview)
	r.seqMu.Lock()
	r.seqCounter = savedSeq
	r.seqMu.Unlock()

	// Collecter les valeurs des variables
	vars := make(map[string]string)
	for _, token := range r.Pattern.Tokens {
		if token.Type == TokenVariable {
			vars[token.Value] = r.resolveVariable(token, ctx)
		}
	}

	return &Preview{
		OriginalPath: ctx.OriginalPath,
		OriginalName: ctx.OriginalName + "." + ctx.Extension,
		NewName:      newName,
		Variables:    vars,
	}
}

// ResetSequence remet le compteur de séquence à zéro.
func (r *Renamer) ResetSequence() {
	r.seqMu.Lock()
	r.seqCounter = 0
	r.seqMu.Unlock()
}

// ResetGroupSequences remet tous les compteurs de groupe à zéro.
func (r *Renamer) ResetGroupSequences() {
	r.groupMu.Lock()
	r.groupCounters = make(map[string]int)
	r.groupMu.Unlock()
}
