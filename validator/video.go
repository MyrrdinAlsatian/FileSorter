// validator/video.go - Validation d'intégrité des fichiers vidéo
//
// Ce fichier implémente la validation pour les formats vidéo courants :
// - MP4/M4V : Vérifie les atoms (ftyp, moov, mdat)
// - MKV/WebM : Vérifie la structure EBML
// - AVI : Vérifie les chunks RIFF
// - MOV : Même structure que MP4
package validator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATEUR VIDÉO
// ═══════════════════════════════════════════════════════════════════════════

// VideoValidator valide les fichiers vidéo.
type VideoValidator struct{}

func init() {
	Register(&VideoValidator{})
}

// SupportedExtensions retourne les extensions vidéo supportées.
func (v *VideoValidator) SupportedExtensions() []string {
	return []string{
		".mp4", ".m4v", ".m4a", // MP4 container
		".mov",                  // QuickTime
		".mkv", ".webm", ".mka", // Matroska
		".avi",         // AVI
		".wmv", ".asf", // Windows Media
		".flv",         // Flash Video
		".3gp", ".3g2", // 3GPP
	}
}

// SupportedMIMETypes retourne les types MIME supportés.
func (v *VideoValidator) SupportedMIMETypes() []string {
	return []string{
		"video/mp4",
		"video/quicktime",
		"video/x-matroska",
		"video/webm",
		"video/x-msvideo",
		"video/x-ms-wmv",
		"video/x-flv",
		"video/3gpp",
	}
}

// Validate vérifie l'intégrité d'un fichier vidéo.
func (v *VideoValidator) Validate(path string) ValidationResult {
	result := ValidationResult{
		Path:    path,
		Valid:   false,
		Details: make(map[string]string),
	}

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

	if n < 8 {
		result.Error = NewValidationError("truncated", "file too small to identify", 0)
		return result
	}

	// Identifier le format et valider
	switch {
	case isMP4(header):
		result.FileType = "mp4"
		return v.validateMP4(f, result)

	case isMKV(header):
		result.FileType = "mkv"
		return v.validateMKV(f, result)

	case isAVI(header):
		result.FileType = "avi"
		return v.validateAVI(f, result)

	case isWMV(header):
		result.FileType = "wmv"
		return v.validateWMV(f, result)

	case isFLV(header):
		result.FileType = "flv"
		return v.validateFLV(f, result)

	default:
		result.Error = NewValidationError("invalid_header", "unrecognized video format", 0)
		return result
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DÉTECTION DE FORMAT
// ═══════════════════════════════════════════════════════════════════════════

// Magic bytes pour MP4/MOV (ftyp atom)
var ftypMagic = []byte("ftyp")

// Brands MP4 courants
var mp4Brands = [][]byte{
	[]byte("isom"), []byte("iso2"), []byte("iso3"), []byte("iso4"), []byte("iso5"), []byte("iso6"),
	[]byte("mp41"), []byte("mp42"),
	[]byte("avc1"), []byte("hvc1"), []byte("hev1"),
	[]byte("M4V "), []byte("M4A "), []byte("M4P "),
	[]byte("qt  "), // QuickTime
	[]byte("3gp4"), []byte("3gp5"), []byte("3g2a"),
	[]byte("mmp4"),
}

func isMP4(header []byte) bool {
	if len(header) < 12 {
		return false
	}
	// L'atom ftyp est généralement au début
	// Format: size (4 bytes) + "ftyp" (4 bytes) + brand (4 bytes)
	return bytes.Equal(header[4:8], ftypMagic)
}

// EBML header pour MKV/WebM
var ebmlMagic = []byte{0x1A, 0x45, 0xDF, 0xA3}

func isMKV(header []byte) bool {
	return len(header) >= 4 && bytes.HasPrefix(header, ebmlMagic)
}

// AVI (RIFF container)
var aviMagic = []byte("AVI ")

func isAVI(header []byte) bool {
	return len(header) >= 12 &&
		bytes.HasPrefix(header, []byte("RIFF")) &&
		bytes.Equal(header[8:12], aviMagic)
}

// WMV/ASF
var asfMagic = []byte{0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11}

func isWMV(header []byte) bool {
	return len(header) >= 16 && bytes.HasPrefix(header, asfMagic)
}

// FLV
var flvMagic = []byte("FLV")

func isFLV(header []byte) bool {
	return len(header) >= 9 && bytes.HasPrefix(header, flvMagic)
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION MP4
// ═══════════════════════════════════════════════════════════════════════════

// validateMP4 vérifie l'intégrité d'un fichier MP4.
//
// STRUCTURE MP4 (ISO Base Media File Format) :
// - Atoms (boxes) : size (4 bytes) + type (4 bytes) + data
// - Atoms obligatoires : ftyp, moov (métadonnées), mdat (données média)
//
// Un fichier MP4 valide DOIT avoir ftyp et moov.
// mdat peut être absent pour les fichiers de métadonnées uniquement.
func (v *VideoValidator) validateMP4(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}
	fileSize := info.Size()

	// Parser les atoms de niveau supérieur
	foundFtyp := false
	foundMoov := false
	foundMdat := false

	var offset int64 = 0
	atomBuf := make([]byte, 8)

	for offset < fileSize {
		if _, err := f.Seek(offset, 0); err != nil {
			result.Error = WrapError(err, "seek_error")
			return result
		}

		// Lire l'en-tête de l'atom
		n, err := f.Read(atomBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Error = WrapError(err, "read_error")
			return result
		}
		if n < 8 {
			result.Error = NewValidationError("truncated", "incomplete atom header", offset)
			return result
		}

		// Taille de l'atom (big-endian)
		atomSize := int64(binary.BigEndian.Uint32(atomBuf[0:4]))
		atomType := string(atomBuf[4:8])

		// Gestion des tailles spéciales
		if atomSize == 0 {
			// Atom s'étend jusqu'à la fin du fichier
			atomSize = fileSize - offset
		} else if atomSize == 1 {
			// Extended size (64-bit)
			extBuf := make([]byte, 8)
			if _, err := f.Read(extBuf); err != nil {
				result.Error = NewValidationError("truncated", "cannot read extended atom size", offset+8)
				return result
			}
			atomSize = int64(binary.BigEndian.Uint64(extBuf))
		}

		// Vérifier que l'atom ne dépasse pas le fichier
		if offset+atomSize > fileSize {
			result.Error = NewValidationError("truncated",
				fmt.Sprintf("atom %s at offset %d claims size %d but file ends at %d",
					atomType, offset, atomSize, fileSize),
				offset)
			return result
		}

		// Noter les atoms trouvés
		switch atomType {
		case "ftyp":
			foundFtyp = true
			// Lire le brand
			brand := make([]byte, 4)
			if _, err := f.Read(brand); err == nil {
				result.Details["brand"] = string(brand)
			}
		case "moov":
			foundMoov = true
		case "mdat":
			foundMdat = true
			// La présence de mdat indique des données média
			result.Details["has_media_data"] = "true"
		}

		offset += atomSize
	}

	// Vérifier les atoms obligatoires
	if !foundFtyp {
		result.Error = NewValidationError("corrupted", "missing ftyp atom", 0)
		return result
	}

	if !foundMoov {
		result.Error = NewValidationError("corrupted", "missing moov atom (metadata)", -1)
		return result
	}

	// MP4 valide !
	result.Valid = true
	result.Details["has_ftyp"] = "true"
	result.Details["has_moov"] = "true"
	if foundMdat {
		result.Details["has_mdat"] = "true"
	}

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION MKV (EBML)
// ═══════════════════════════════════════════════════════════════════════════

// validateMKV vérifie l'intégrité d'un fichier MKV/WebM.
//
// STRUCTURE EBML (Extensible Binary Meta Language) :
// - EBML Header (ID 0x1A45DFA3)
// - Segment (ID 0x18538067) contenant :
//   - SeekHead, Info, Tracks, Clusters, etc.
//
// Un fichier MKV valide DOIT avoir EBML Header et Segment.
func (v *VideoValidator) validateMKV(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}
	fileSize := info.Size()

	// Vérifier le header EBML
	header := make([]byte, 4)
	if _, err := io.ReadFull(f, header); err != nil {
		result.Error = NewValidationError("truncated", "cannot read EBML header", 0)
		return result
	}

	if !bytes.Equal(header, ebmlMagic) {
		result.Error = NewValidationError("invalid_header", "invalid EBML signature", 0)
		return result
	}

	// Lire la taille de l'élément EBML Header
	ebmlSize, bytesRead, err := readEBMLVInt(f)
	if err != nil {
		result.Error = NewValidationError("truncated", "cannot read EBML header size", 4)
		return result
	}

	// Passer le contenu de l'EBML Header
	ebmlHeaderEnd := int64(4+bytesRead) + int64(ebmlSize)
	if ebmlHeaderEnd > fileSize {
		result.Error = NewValidationError("truncated", "EBML header extends beyond file", 4)
		return result
	}

	if _, err := f.Seek(ebmlHeaderEnd, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Chercher l'élément Segment (0x18538067)
	segmentID := make([]byte, 4)
	if _, err := io.ReadFull(f, segmentID); err != nil {
		result.Error = NewValidationError("truncated", "cannot read Segment ID", ebmlHeaderEnd)
		return result
	}

	expectedSegment := []byte{0x18, 0x53, 0x80, 0x67}
	if !bytes.Equal(segmentID, expectedSegment) {
		result.Error = NewValidationError("corrupted",
			fmt.Sprintf("expected Segment element (18538067), got %X", segmentID),
			ebmlHeaderEnd)
		return result
	}

	// Lire la taille du Segment
	segmentSize, _, err := readEBMLVInt(f)
	if err != nil {
		result.Error = NewValidationError("truncated", "cannot read Segment size", ebmlHeaderEnd+4)
		return result
	}

	// Vérifier que le segment ne dépasse pas le fichier
	// Note: segmentSize peut être "unknown" (-1 ou 0x1FFFFFFFFFFFFFF)
	if segmentSize > 0 && segmentSize < 0x1FFFFFFFFFFFFFF {
		currentPos, _ := f.Seek(0, 1)
		expectedEnd := currentPos + int64(segmentSize)
		if expectedEnd > fileSize {
			result.Error = NewValidationError("truncated",
				fmt.Sprintf("Segment claims size %d but file ends at %d", segmentSize, fileSize),
				currentPos)
			return result
		}
	}

	// MKV valide !
	result.Valid = true
	result.Details["has_ebml_header"] = "true"
	result.Details["has_segment"] = "true"

	return result
}

// readEBMLVInt lit un entier de longueur variable EBML (VINT).
// Retourne la valeur, le nombre d'octets lus, et une éventuelle erreur.
func readEBMLVInt(r io.Reader) (uint64, int, error) {
	firstByte := make([]byte, 1)
	if _, err := io.ReadFull(r, firstByte); err != nil {
		return 0, 0, err
	}

	// Déterminer le nombre d'octets en comptant les zéros de tête
	b := firstByte[0]
	var numBytes int
	var mask byte

	switch {
	case b&0x80 != 0:
		numBytes = 1
		mask = 0x7F
	case b&0x40 != 0:
		numBytes = 2
		mask = 0x3F
	case b&0x20 != 0:
		numBytes = 3
		mask = 0x1F
	case b&0x10 != 0:
		numBytes = 4
		mask = 0x0F
	case b&0x08 != 0:
		numBytes = 5
		mask = 0x07
	case b&0x04 != 0:
		numBytes = 6
		mask = 0x03
	case b&0x02 != 0:
		numBytes = 7
		mask = 0x01
	case b&0x01 != 0:
		numBytes = 8
		mask = 0x00
	default:
		return 0, 0, fmt.Errorf("invalid VINT")
	}

	// Construire la valeur
	value := uint64(b & mask)

	// Lire les octets restants
	if numBytes > 1 {
		remaining := make([]byte, numBytes-1)
		if _, err := io.ReadFull(r, remaining); err != nil {
			return 0, 0, err
		}
		for _, rb := range remaining {
			value = (value << 8) | uint64(rb)
		}
	}

	return value, numBytes, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION AVI
// ═══════════════════════════════════════════════════════════════════════════

// validateAVI vérifie l'intégrité d'un fichier AVI.
//
// STRUCTURE AVI (RIFF container) :
// - "RIFF" (4) + size (4) + "AVI " (4)
// - LIST "hdrl" : header list (avih, strl)
// - LIST "movi" : movie data
// - "idx1" : index (optionnel)
func (v *VideoValidator) validateAVI(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}
	fileSize := info.Size()

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

	// Taille déclarée
	declaredSize := binary.LittleEndian.Uint32(header[4:8])
	expectedSize := int64(declaredSize) + 8

	// Vérifier "AVI "
	if string(header[8:12]) != "AVI " {
		result.Error = NewValidationError("invalid_header", "missing AVI signature", 8)
		return result
	}

	// Vérifier la taille (avec tolérance pour padding)
	if fileSize < expectedSize-2 {
		result.Error = NewValidationError("truncated",
			fmt.Sprintf("file size mismatch: header says %d, actual %d", expectedSize, fileSize),
			fileSize)
		return result
	}

	// Chercher les chunks obligatoires
	foundHdrl := false
	foundMovi := false

	var offset int64 = 12
	chunkBuf := make([]byte, 12)

	for offset < fileSize-8 {
		if _, err := f.Seek(offset, 0); err != nil {
			break
		}

		n, err := f.Read(chunkBuf)
		if err != nil || n < 8 {
			break
		}

		chunkID := string(chunkBuf[0:4])
		chunkSize := int64(binary.LittleEndian.Uint32(chunkBuf[4:8]))

		// Pour les LIST, le type est dans les 4 bytes suivants
		if chunkID == "LIST" && n >= 12 {
			listType := string(chunkBuf[8:12])
			if listType == "hdrl" {
				foundHdrl = true
			} else if listType == "movi" {
				foundMovi = true
			}
		}

		// Passer au chunk suivant (aligné sur 2 bytes)
		if chunkSize%2 != 0 {
			chunkSize++
		}
		offset += 8 + chunkSize
	}

	// Vérifier les chunks obligatoires
	if !foundHdrl {
		result.Error = NewValidationError("corrupted", "missing hdrl (header list)", -1)
		return result
	}

	if !foundMovi {
		result.Error = NewValidationError("corrupted", "missing movi (movie data)", -1)
		return result
	}

	// AVI valide !
	result.Valid = true
	result.Details["has_hdrl"] = "true"
	result.Details["has_movi"] = "true"

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION WMV/ASF
// ═══════════════════════════════════════════════════════════════════════════

// validateWMV vérifie l'intégrité d'un fichier WMV/ASF.
//
// STRUCTURE ASF (Advanced Systems Format) :
// - Header Object (GUID + size)
// - Data Object
// - Simple Index Object (optionnel)
func (v *VideoValidator) validateWMV(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	info, err := f.Stat()
	if err != nil {
		result.Error = WrapError(err, "stat_error")
		return result
	}
	fileSize := info.Size()

	// Lire le header ASF (30 bytes minimum: GUID 16 + size 8 + objects 4 + reserved 2)
	header := make([]byte, 30)
	if _, err := io.ReadFull(f, header); err != nil {
		result.Error = NewValidationError("truncated", "cannot read ASF header", 0)
		return result
	}

	// Vérifier le GUID de l'ASF Header Object
	// 30 26 B2 75 8E 66 CF 11 A6 D9 00 AA 00 62 CE 6C
	expectedGUID := []byte{
		0x30, 0x26, 0xB2, 0x75, 0x8E, 0x66, 0xCF, 0x11,
		0xA6, 0xD9, 0x00, 0xAA, 0x00, 0x62, 0xCE, 0x6C,
	}

	if !bytes.Equal(header[0:16], expectedGUID) {
		result.Error = NewValidationError("invalid_header", "invalid ASF header GUID", 0)
		return result
	}

	// Taille de l'objet header (little-endian, 64-bit)
	headerSize := binary.LittleEndian.Uint64(header[16:24])

	if int64(headerSize) > fileSize {
		result.Error = NewValidationError("truncated",
			fmt.Sprintf("header size %d exceeds file size %d", headerSize, fileSize),
			16)
		return result
	}

	// WMV valide (vérification basique)
	result.Valid = true
	result.Details["header_size"] = fmt.Sprintf("%d", headerSize)

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION FLV
// ═══════════════════════════════════════════════════════════════════════════

// validateFLV vérifie l'intégrité d'un fichier FLV.
//
// STRUCTURE FLV :
// - Header (9 bytes): "FLV" + version + flags + header size
// - PreviousTagSize0 (4 bytes, always 0)
// - Tags (11 bytes header + data + PreviousTagSize)
func (v *VideoValidator) validateFLV(f *os.File, result ValidationResult) ValidationResult {
	if _, err := f.Seek(0, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Lire le header FLV (9 bytes)
	header := make([]byte, 9)
	if _, err := io.ReadFull(f, header); err != nil {
		result.Error = NewValidationError("truncated", "cannot read FLV header", 0)
		return result
	}

	// Vérifier "FLV"
	if string(header[0:3]) != "FLV" {
		result.Error = NewValidationError("invalid_header", "missing FLV signature", 0)
		return result
	}

	// Version
	version := header[3]

	// Flags (bit 0 = video, bit 2 = audio)
	flags := header[4]
	hasVideo := flags&0x01 != 0
	hasAudio := flags&0x04 != 0

	// Header size (big-endian, should be 9)
	headerSize := binary.BigEndian.Uint32(header[5:9])
	if headerSize < 9 {
		result.Error = NewValidationError("corrupted", "invalid FLV header size", 5)
		return result
	}

	// FLV valide !
	result.Valid = true
	result.Details["version"] = fmt.Sprintf("%d", version)
	result.Details["has_video"] = fmt.Sprintf("%v", hasVideo)
	result.Details["has_audio"] = fmt.Sprintf("%v", hasAudio)

	return result
}
