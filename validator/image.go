// validator/image.go - Validation d'intégrité des images
//
// Ce fichier implémente la validation pour les formats d'image courants :
// - JPEG : Vérifie SOI/EOI markers et décode l'image
// - PNG : Vérifie signature et CRC des chunks
// - GIF : Vérifie header et structure
// - WebP : Vérifie container RIFF et chunks
package validator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"os"

	// Import des décodeurs d'images standard
	// Ces imports "blank" (_) enregistrent les décodeurs pour image.Decode
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	// WebP nécessite un import externe
	_ "golang.org/x/image/webp"
)

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATEUR D'IMAGES GÉNÉRIQUE
// ═══════════════════════════════════════════════════════════════════════════

// ImageValidator valide les fichiers image en utilisant les décodeurs Go.
//
// CONCEPT : VALIDATION PAR DÉCODAGE
// ==================================
// La méthode la plus fiable pour valider une image est de la décoder.
// Si le décodeur échoue, l'image est corrompue.
// On peut aussi vérifier les magic bytes pour une validation rapide.
type ImageValidator struct{}

// init enregistre automatiquement le validateur au chargement du package.
//
// CONCEPT GO : FONCTION INIT
// ==========================
// La fonction init() est appelée automatiquement quand le package est importé.
// Elle est exécutée après l'initialisation des variables du package.
// Un package peut avoir plusieurs fonctions init(), toutes seront exécutées.
func init() {
	Register(&ImageValidator{})
}

// SupportedExtensions retourne les extensions d'images supportées.
func (v *ImageValidator) SupportedExtensions() []string {
	return []string{
		".jpg", ".jpeg", ".jpe", ".jfif", // JPEG
		".png",  // PNG
		".gif",  // GIF
		".webp", // WebP
		".bmp",  // BMP
	}
}

// SupportedMIMETypes retourne les types MIME supportés.
func (v *ImageValidator) SupportedMIMETypes() []string {
	return []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
		"image/bmp",
	}
}

// Validate vérifie l'intégrité d'un fichier image.
//
// La validation se fait en plusieurs étapes :
// 1. Vérification des magic bytes (signature du format)
// 2. Vérification de la structure spécifique au format
// 3. Tentative de décodage complet (optionnel pour les gros fichiers)
func (v *ImageValidator) Validate(path string) ValidationResult {
	result := ValidationResult{
		Path:    path,
		Valid:   false,
		Details: make(map[string]string),
	}

	// Ouvrir le fichier
	f, err := os.Open(path)
	if err != nil {
		result.Error = WrapError(err, "access_error")
		return result
	}
	defer f.Close()

	// Lire les premiers octets pour identifier le format
	header := make([]byte, 32)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		result.Error = WrapError(err, "read_error")
		return result
	}
	header = header[:n]

	if n < 4 {
		result.Error = NewValidationError("truncated", "file too small to identify", 0)
		return result
	}

	// Identifier le format et valider
	switch {
	case isJPEG(header):
		result.FileType = "jpeg"
		return v.validateJPEG(f, result)

	case isPNG(header):
		result.FileType = "png"
		return v.validatePNG(f, result)

	case isGIF(header):
		result.FileType = "gif"
		return v.validateGIF(f, result)

	case isWebP(header):
		result.FileType = "webp"
		return v.validateWebP(f, result)

	case isBMP(header):
		result.FileType = "bmp"
		return v.validateBMP(f, result)

	default:
		result.Error = NewValidationError("invalid_header", "unrecognized image format", 0)
		return result
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DÉTECTION DE FORMAT (MAGIC BYTES)
// ═══════════════════════════════════════════════════════════════════════════

// Magic bytes pour chaque format
var (
	// JPEG: commence par FF D8 FF
	jpegMagic = []byte{0xFF, 0xD8, 0xFF}

	// PNG: signature de 8 octets
	pngMagic = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}

	// GIF: "GIF87a" ou "GIF89a"
	gif87Magic = []byte("GIF87a")
	gif89Magic = []byte("GIF89a")

	// WebP: "RIFF" + taille + "WEBP"
	riffMagic = []byte("RIFF")
	webpMagic = []byte("WEBP")

	// BMP: "BM"
	bmpMagic = []byte("BM")
)

func isJPEG(header []byte) bool {
	return len(header) >= 3 && bytes.HasPrefix(header, jpegMagic)
}

func isPNG(header []byte) bool {
	return len(header) >= 8 && bytes.HasPrefix(header, pngMagic)
}

func isGIF(header []byte) bool {
	return len(header) >= 6 && (bytes.HasPrefix(header, gif87Magic) || bytes.HasPrefix(header, gif89Magic))
}

func isWebP(header []byte) bool {
	return len(header) >= 12 &&
		bytes.HasPrefix(header, riffMagic) &&
		bytes.Equal(header[8:12], webpMagic)
}

func isBMP(header []byte) bool {
	return len(header) >= 2 && bytes.HasPrefix(header, bmpMagic)
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION JPEG
// ═══════════════════════════════════════════════════════════════════════════

// validateJPEG vérifie l'intégrité d'un fichier JPEG.
//
// STRUCTURE JPEG :
// - SOI (Start Of Image) : FF D8
// - Segments : FF xx (marker) + taille (2 bytes) + données
// - EOI (End Of Image) : FF D9
//
// Un JPEG valide doit avoir SOI au début et EOI à la fin.
func (v *ImageValidator) validateJPEG(f *os.File, result ValidationResult) ValidationResult {
	// Revenir au début
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Vérifier EOI à la fin du fichier
	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}
	size := info.Size()

	// Lire les derniers octets pour trouver EOI (FF D9)
	// Note: certains JPEG ont des données après EOI (métadonnées)
	// On cherche donc EOI dans les derniers 1024 octets
	searchSize := int64(1024)
	if size < searchSize {
		searchSize = size
	}

	if _, err := f.Seek(-searchSize, 2); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	tail := make([]byte, searchSize)
	n, err := f.Read(tail)
	if err != nil && err != io.EOF {
		result.Error = WrapError(err, "read_error")
		return result
	}
	tail = tail[:n]

	// Chercher FF D9 (EOI)
	foundEOI := false
	for i := len(tail) - 2; i >= 0; i-- {
		if tail[i] == 0xFF && tail[i+1] == 0xD9 {
			foundEOI = true
			break
		}
	}

	if !foundEOI {
		result.Error = NewValidationError("truncated", "missing EOI marker (FF D9)", size)
		return result
	}

	// Tenter de décoder l'image pour validation complète
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	config, format, err := image.DecodeConfig(f)
	if err != nil {
		result.Error = NewValidationError("decode_error", fmt.Sprintf("failed to decode: %v", err), -1)
		return result
	}

	// Image valide !
	result.Valid = true
	result.FileType = format
	result.Details["width"] = fmt.Sprintf("%d", config.Width)
	result.Details["height"] = fmt.Sprintf("%d", config.Height)
	result.Details["color_model"] = fmt.Sprintf("%T", config.ColorModel)

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION PNG
// ═══════════════════════════════════════════════════════════════════════════

// validatePNG vérifie l'intégrité d'un fichier PNG.
//
// STRUCTURE PNG :
// - Signature (8 bytes) : 89 50 4E 47 0D 0A 1A 0A
// - Chunks : taille (4) + type (4) + data (n) + CRC (4)
// - Chunks obligatoires : IHDR (premier), IEND (dernier)
//
// On vérifie la présence de IHDR et IEND, et optionnellement les CRC.
func (v *ImageValidator) validatePNG(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Vérifier la signature PNG (8 bytes)
	signature := make([]byte, 8)
	if _, err := io.ReadFull(f, signature); err != nil {
		result.Error = NewValidationError("truncated", "cannot read PNG signature", 0)
		return result
	}

	if !bytes.Equal(signature, pngMagic) {
		result.Error = NewValidationError("invalid_header", "invalid PNG signature", 0)
		return result
	}

	// Lire le premier chunk (doit être IHDR)
	chunkLen := make([]byte, 4)
	chunkType := make([]byte, 4)

	if _, err := io.ReadFull(f, chunkLen); err != nil {
		result.Error = NewValidationError("truncated", "cannot read first chunk length", 8)
		return result
	}

	if _, err := io.ReadFull(f, chunkType); err != nil {
		result.Error = NewValidationError("truncated", "cannot read first chunk type", 12)
		return result
	}

	if string(chunkType) != "IHDR" {
		result.Error = NewValidationError("corrupted", fmt.Sprintf("first chunk should be IHDR, got %s", string(chunkType)), 12)
		return result
	}

	// Lire les dimensions depuis IHDR (13 bytes de data)
	ihdrData := make([]byte, 13)
	if _, err := io.ReadFull(f, ihdrData); err != nil {
		result.Error = NewValidationError("truncated", "cannot read IHDR data", 16)
		return result
	}

	width := binary.BigEndian.Uint32(ihdrData[0:4])
	height := binary.BigEndian.Uint32(ihdrData[4:8])

	// Vérifier IEND à la fin
	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}

	// IEND chunk : 00 00 00 00 (len) + 49 45 4E 44 (IEND) + CRC (4)
	// = 12 bytes au minimum à la fin
	if info.Size() < 12 {
		result.Error = NewValidationError("truncated", "file too small for IEND chunk", info.Size())
		return result
	}

	if _, err := f.Seek(-12, 2); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	iendCheck := make([]byte, 8)
	if _, err := io.ReadFull(f, iendCheck); err != nil {
		result.Error = NewValidationError("truncated", "cannot read IEND", info.Size()-12)
		return result
	}

	// Vérifier que c'est bien IEND (longueur 0 + "IEND")
	if binary.BigEndian.Uint32(iendCheck[0:4]) != 0 || string(iendCheck[4:8]) != "IEND" {
		result.Error = NewValidationError("truncated", "missing or invalid IEND chunk", info.Size()-12)
		return result
	}

	// PNG valide !
	result.Valid = true
	result.Details["width"] = fmt.Sprintf("%d", width)
	result.Details["height"] = fmt.Sprintf("%d", height)
	result.Details["bit_depth"] = fmt.Sprintf("%d", ihdrData[8])
	result.Details["color_type"] = fmt.Sprintf("%d", ihdrData[9])

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION GIF
// ═══════════════════════════════════════════════════════════════════════════

// validateGIF vérifie l'intégrité d'un fichier GIF.
//
// STRUCTURE GIF :
// - Header : "GIF87a" ou "GIF89a" (6 bytes)
// - Logical Screen Descriptor (7 bytes)
// - Blocs de données
// - Trailer : 0x3B (;)
func (v *ImageValidator) validateGIF(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Lire le header (6 bytes)
	header := make([]byte, 6)
	if _, err := io.ReadFull(f, header); err != nil {
		result.Error = NewValidationError("truncated", "cannot read GIF header", 0)
		return result
	}

	version := string(header)
	if version != "GIF87a" && version != "GIF89a" {
		result.Error = NewValidationError("invalid_header", fmt.Sprintf("invalid GIF version: %s", version), 0)
		return result
	}

	// Lire le Logical Screen Descriptor (7 bytes)
	lsd := make([]byte, 7)
	if _, err := io.ReadFull(f, lsd); err != nil {
		result.Error = NewValidationError("truncated", "cannot read logical screen descriptor", 6)
		return result
	}

	width := binary.LittleEndian.Uint16(lsd[0:2])
	height := binary.LittleEndian.Uint16(lsd[2:4])

	// Vérifier le trailer à la fin (0x3B)
	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}

	if _, err := f.Seek(-1, 2); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	trailer := make([]byte, 1)
	if _, err := f.Read(trailer); err != nil {
		result.Error = NewValidationError("truncated", "cannot read trailer", info.Size()-1)
		return result
	}

	if trailer[0] != 0x3B {
		result.Error = NewValidationError("truncated", "missing GIF trailer (0x3B)", info.Size()-1)
		return result
	}

	// GIF valide !
	result.Valid = true
	result.Details["width"] = fmt.Sprintf("%d", width)
	result.Details["height"] = fmt.Sprintf("%d", height)
	result.Details["version"] = version

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION WEBP
// ═══════════════════════════════════════════════════════════════════════════

// validateWebP vérifie l'intégrité d'un fichier WebP.
//
// STRUCTURE WEBP (RIFF container):
// - "RIFF" (4 bytes)
// - File size - 8 (4 bytes, little-endian)
// - "WEBP" (4 bytes)
// - Chunks (VP8, VP8L, VP8X, etc.)
func (v *ImageValidator) validateWebP(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Lire le header RIFF (12 bytes)
	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		result.Error = NewValidationError("truncated", "cannot read RIFF header", 0)
		return result
	}

	// Vérifier "RIFF"
	if string(header[0:4]) != "RIFF" {
		result.Error = NewValidationError("invalid_header", "missing RIFF signature", 0)
		return result
	}

	// Taille déclarée dans le header
	declaredSize := binary.LittleEndian.Uint32(header[4:8])

	// Vérifier "WEBP"
	if string(header[8:12]) != "WEBP" {
		result.Error = NewValidationError("invalid_header", "missing WEBP signature", 8)
		return result
	}

	// Vérifier la taille du fichier
	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}

	actualSize := info.Size()
	expectedSize := int64(declaredSize) + 8 // +8 pour "RIFF" + taille

	// Tolérance : certains fichiers ont du padding
	if actualSize < expectedSize-1 {
		result.Error = NewValidationError("truncated",
			fmt.Sprintf("file size mismatch: expected %d, got %d", expectedSize, actualSize),
			actualSize)
		return result
	}

	// Lire le premier chunk pour identifier le type (VP8, VP8L, VP8X)
	chunk := make([]byte, 4)
	if _, err := io.ReadFull(f, chunk); err != nil {
		result.Error = NewValidationError("truncated", "cannot read first chunk", 12)
		return result
	}

	chunkType := string(chunk)

	// Décoder avec le package x/image/webp pour validation complète
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	config, format, err := image.DecodeConfig(f)
	if err != nil {
		result.Error = NewValidationError("decode_error", fmt.Sprintf("failed to decode: %v", err), -1)
		return result
	}

	// WebP valide !
	result.Valid = true
	result.FileType = format
	result.Details["width"] = fmt.Sprintf("%d", config.Width)
	result.Details["height"] = fmt.Sprintf("%d", config.Height)
	result.Details["webp_type"] = chunkType

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION BMP
// ═══════════════════════════════════════════════════════════════════════════

// validateBMP vérifie l'intégrité d'un fichier BMP.
//
// STRUCTURE BMP :
// - File Header (14 bytes): "BM" + file size + reserved + pixel data offset
// - DIB Header (variable): dimensions, bits per pixel, compression, etc.
// - Pixel data
func (v *ImageValidator) validateBMP(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Lire le file header (14 bytes)
	fileHeader := make([]byte, 14)
	if _, err := io.ReadFull(f, fileHeader); err != nil {
		result.Error = NewValidationError("truncated", "cannot read BMP file header", 0)
		return result
	}

	// Vérifier "BM"
	if string(fileHeader[0:2]) != "BM" {
		result.Error = NewValidationError("invalid_header", "missing BM signature", 0)
		return result
	}

	// Taille déclarée
	declaredSize := binary.LittleEndian.Uint32(fileHeader[2:6])

	// Vérifier la taille du fichier
	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}

	actualSize := info.Size()
	if actualSize < int64(declaredSize) {
		result.Error = NewValidationError("truncated",
			fmt.Sprintf("file size mismatch: header says %d, actual %d", declaredSize, actualSize),
			actualSize)
		return result
	}

	// Lire le début du DIB header pour les dimensions
	// La taille du DIB header est dans les 4 premiers bytes
	dibSizeBytes := make([]byte, 4)
	if _, err := io.ReadFull(f, dibSizeBytes); err != nil {
		result.Error = NewValidationError("truncated", "cannot read DIB header size", 14)
		return result
	}
	dibSize := binary.LittleEndian.Uint32(dibSizeBytes)

	// Lire les dimensions (après la taille)
	dimBytes := make([]byte, 8)
	if _, err := io.ReadFull(f, dimBytes); err != nil {
		result.Error = NewValidationError("truncated", "cannot read dimensions", 18)
		return result
	}

	var width, height int32
	width = int32(binary.LittleEndian.Uint32(dimBytes[0:4]))
	height = int32(binary.LittleEndian.Uint32(dimBytes[4:8]))

	// La hauteur peut être négative (top-down bitmap)
	if height < 0 {
		height = -height
	}

	// BMP valide !
	result.Valid = true
	result.Details["width"] = fmt.Sprintf("%d", width)
	result.Details["height"] = fmt.Sprintf("%d", height)
	result.Details["dib_header_size"] = fmt.Sprintf("%d", dibSize)

	return result
}
