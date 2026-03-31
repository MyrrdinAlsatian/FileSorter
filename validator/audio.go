// validator/audio.go - Validation d'intégrité des fichiers audio
//
// Ce fichier implémente la validation pour les formats audio courants :
// - MP3 : Vérifie les frame sync et la structure
// - FLAC : Vérifie le header et les blocs metadata
// - WAV : Vérifie la structure RIFF
// - OGG : Vérifie les pages Ogg
// - M4A/AAC : Utilise le validateur MP4
package validator

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATEUR AUDIO
// ═══════════════════════════════════════════════════════════════════════════

// AudioValidator valide les fichiers audio.
type AudioValidator struct{}

func init() {
	Register(&AudioValidator{})
}

// SupportedExtensions retourne les extensions audio supportées.
func (v *AudioValidator) SupportedExtensions() []string {
	return []string{
		".mp3",          // MPEG Audio Layer 3
		".flac",         // Free Lossless Audio Codec
		".wav", ".wave", // Waveform Audio
		".ogg", ".oga", // Ogg Vorbis
		".opus",         // Opus
		".aac",          // Advanced Audio Coding (raw)
		".wma",          // Windows Media Audio
		".aiff", ".aif", // Audio Interchange File Format
	}
}

// SupportedMIMETypes retourne les types MIME supportés.
func (v *AudioValidator) SupportedMIMETypes() []string {
	return []string{
		"audio/mpeg",
		"audio/mp3",
		"audio/flac",
		"audio/wav",
		"audio/x-wav",
		"audio/ogg",
		"audio/opus",
		"audio/aac",
		"audio/x-ms-wma",
		"audio/aiff",
	}
}

// Validate vérifie l'intégrité d'un fichier audio.
func (v *AudioValidator) Validate(path string) ValidationResult {
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

	if n < 4 {
		result.Error = NewValidationError("truncated", "file too small to identify", 0)
		return result
	}

	// Identifier le format et valider
	switch {
	case isMP3(header):
		result.FileType = "mp3"
		return v.validateMP3(f, result)

	case isFLAC(header):
		result.FileType = "flac"
		return v.validateFLAC(f, result)

	case isWAV(header):
		result.FileType = "wav"
		return v.validateWAV(f, result)

	case isOGG(header):
		result.FileType = "ogg"
		return v.validateOGG(f, result)

	case isAIFF(header):
		result.FileType = "aiff"
		return v.validateAIFF(f, result)

	default:
		result.Error = NewValidationError("invalid_header", "unrecognized audio format", 0)
		return result
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DÉTECTION DE FORMAT
// ═══════════════════════════════════════════════════════════════════════════

// ID3v2 tag (souvent au début des MP3)
var id3v2Magic = []byte("ID3")

// MP3 frame sync (11 bits à 1)
func isMP3FrameSync(b1, b2 byte) bool {
	return b1 == 0xFF && (b2&0xE0) == 0xE0
}

func isMP3(header []byte) bool {
	if len(header) < 3 {
		return false
	}
	// Soit ID3v2 tag au début
	if bytes.HasPrefix(header, id3v2Magic) {
		return true
	}
	// Soit frame sync directement
	return isMP3FrameSync(header[0], header[1])
}

// FLAC magic
var flacMagic = []byte("fLaC")

func isFLAC(header []byte) bool {
	return len(header) >= 4 && bytes.HasPrefix(header, flacMagic)
}

// WAV (RIFF WAVE)
var waveMagic = []byte("WAVE")

func isWAV(header []byte) bool {
	return len(header) >= 12 &&
		bytes.HasPrefix(header, []byte("RIFF")) &&
		bytes.Equal(header[8:12], waveMagic)
}

// OGG
var oggMagic = []byte("OggS")

func isOGG(header []byte) bool {
	return len(header) >= 4 && bytes.HasPrefix(header, oggMagic)
}

// AIFF
var aiffMagic = []byte("AIFF")
var aifcMagic = []byte("AIFC")

func isAIFF(header []byte) bool {
	return len(header) >= 12 &&
		bytes.HasPrefix(header, []byte("FORM")) &&
		(bytes.Equal(header[8:12], aiffMagic) || bytes.Equal(header[8:12], aifcMagic))
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION MP3
// ═══════════════════════════════════════════════════════════════════════════

// validateMP3 vérifie l'intégrité d'un fichier MP3.
//
// STRUCTURE MP3 :
// - [ID3v2 tag] (optionnel, au début)
// - Frames audio : sync (11 bits) + header + data
// - [ID3v1 tag] (optionnel, 128 bytes à la fin)
//
// On vérifie qu'on trouve au moins quelques frames valides.
func (v *AudioValidator) validateMP3(f *os.File, result ValidationResult) ValidationResult {
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

	// Lire le début pour détecter ID3v2
	header := make([]byte, 10)
	if _, err := io.ReadFull(f, header); err != nil {
		result.Error = NewValidationError("truncated", "cannot read header", 0)
		return result
	}

	var audioStart int64 = 0

	// Si ID3v2 présent, calculer sa taille
	if bytes.HasPrefix(header, id3v2Magic) {
		// ID3v2 header : "ID3" (3) + version (2) + flags (1) + size (4)
		// La taille est encodée en "synchsafe" (7 bits par octet)
		size := (int64(header[6]&0x7F) << 21) |
			(int64(header[7]&0x7F) << 14) |
			(int64(header[8]&0x7F) << 7) |
			int64(header[9]&0x7F)

		audioStart = 10 + size
		result.Details["id3v2_size"] = fmt.Sprintf("%d", size)

		// Vérifier que le tag ne dépasse pas le fichier
		if audioStart > fileSize {
			result.Error = NewValidationError("truncated",
				fmt.Sprintf("ID3v2 tag claims size %d but file is only %d bytes", audioStart, fileSize),
				10)
			return result
		}
	}

	// Chercher le premier frame sync valide
	if _, err := f.Seek(audioStart, 0); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	// Lire un bloc pour chercher le sync
	searchBuf := make([]byte, 4096)
	n, err := f.Read(searchBuf)
	if err != nil && err != io.EOF {
		result.Error = WrapError(err, "read_error")
		return result
	}

	// Chercher un frame sync valide
	foundSync := false
	var syncOffset int

	for i := 0; i < n-1; i++ {
		if isMP3FrameSync(searchBuf[i], searchBuf[i+1]) {
			// Vérifier que c'est un vrai frame (layer, bitrate valides)
			if i+3 < n {
				version := (searchBuf[i+1] >> 3) & 0x03 // MPEG version
				layer := (searchBuf[i+1] >> 1) & 0x03   // Layer
				bitrate := (searchBuf[i+2] >> 4) & 0x0F // Bitrate index

				// Version 01 est réservée, layer 00 est réservé
				// Bitrate 0000 = free, 1111 = bad
				if version != 1 && layer != 0 && bitrate != 0 && bitrate != 15 {
					foundSync = true
					syncOffset = i
					break
				}
			}
		}
	}

	if !foundSync {
		result.Error = NewValidationError("corrupted", "no valid MP3 frame found", audioStart)
		return result
	}

	result.Details["first_frame_offset"] = fmt.Sprintf("%d", audioStart+int64(syncOffset))

	// Vérifier ID3v1 à la fin (128 bytes)
	if fileSize >= 128 {
		if _, err := f.Seek(-128, 2); err == nil {
			id3v1 := make([]byte, 3)
			if _, err := f.Read(id3v1); err == nil && string(id3v1) == "TAG" {
				result.Details["has_id3v1"] = "true"
			}
		}
	}

	// MP3 valide !
	result.Valid = true
	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION FLAC
// ═══════════════════════════════════════════════════════════════════════════

// validateFLAC vérifie l'intégrité d'un fichier FLAC.
//
// STRUCTURE FLAC :
// - Magic "fLaC" (4 bytes)
// - Metadata blocks : type (1) + size (3) + data
// - Audio frames
//
// Le premier bloc metadata DOIT être STREAMINFO (type 0).
func (v *AudioValidator) validateFLAC(f *os.File, result ValidationResult) ValidationResult {
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

	// Vérifier le magic
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil {
		result.Error = NewValidationError("truncated", "cannot read FLAC magic", 0)
		return result
	}

	if !bytes.Equal(magic, flacMagic) {
		result.Error = NewValidationError("invalid_header", "invalid FLAC magic", 0)
		return result
	}

	// Lire les blocs metadata
	var offset int64 = 4
	isLast := false
	blockCount := 0

	for !isLast && offset < fileSize {
		if _, err := f.Seek(offset, 0); err != nil {
			result.Error = WrapError(err, "seek_error")
			return result
		}

		// Header du bloc (4 bytes)
		blockHeader := make([]byte, 4)
		if _, err := io.ReadFull(f, blockHeader); err != nil {
			result.Error = NewValidationError("truncated", "cannot read metadata block header", offset)
			return result
		}

		// Premier byte : bit 7 = last, bits 0-6 = type
		isLast = (blockHeader[0] & 0x80) != 0
		blockType := blockHeader[0] & 0x7F

		// Bytes 1-3 : taille (big-endian, 24-bit)
		blockSize := int64(blockHeader[1])<<16 | int64(blockHeader[2])<<8 | int64(blockHeader[3])

		// Le premier bloc DOIT être STREAMINFO (type 0)
		if blockCount == 0 && blockType != 0 {
			result.Error = NewValidationError("corrupted",
				fmt.Sprintf("first metadata block should be STREAMINFO (0), got %d", blockType),
				offset)
			return result
		}

		// STREAMINFO : 34 bytes
		if blockType == 0 {
			if blockSize != 34 {
				result.Error = NewValidationError("corrupted",
					fmt.Sprintf("STREAMINFO should be 34 bytes, got %d", blockSize),
					offset)
				return result
			}

			// Lire les infos du stream
			streamInfo := make([]byte, 34)
			if _, err := io.ReadFull(f, streamInfo); err != nil {
				result.Error = NewValidationError("truncated", "cannot read STREAMINFO", offset+4)
				return result
			}

			// Bytes 10-17 : total samples (36 bits, big-endian)
			// Bytes 18-33 : MD5 signature

			// Sample rate (bits 0-19 of bytes 10-12)
			sampleRate := (uint32(streamInfo[10]) << 12) |
				(uint32(streamInfo[11]) << 4) |
				(uint32(streamInfo[12]) >> 4)

			// Channels (bits 1-3 of byte 12) + 1
			channels := ((streamInfo[12] >> 1) & 0x07) + 1

			// Bits per sample (bits 0 of byte 12 + bits 4-7 of byte 13) + 1
			bitsPerSample := ((uint32(streamInfo[12])&0x01)<<4 | uint32(streamInfo[13]>>4)) + 1

			result.Details["sample_rate"] = fmt.Sprintf("%d", sampleRate)
			result.Details["channels"] = fmt.Sprintf("%d", channels)
			result.Details["bits_per_sample"] = fmt.Sprintf("%d", bitsPerSample)
		}

		offset += 4 + blockSize
		blockCount++
	}

	if blockCount == 0 {
		result.Error = NewValidationError("corrupted", "no metadata blocks found", 4)
		return result
	}

	// FLAC valide !
	result.Valid = true
	result.Details["metadata_blocks"] = fmt.Sprintf("%d", blockCount)

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION WAV
// ═══════════════════════════════════════════════════════════════════════════

// validateWAV vérifie l'intégrité d'un fichier WAV.
//
// STRUCTURE WAV (RIFF) :
// - "RIFF" (4) + size (4) + "WAVE" (4)
// - Chunks : "fmt " (format), "data" (audio data), etc.
func (v *AudioValidator) validateWAV(f *os.File, result ValidationResult) ValidationResult {
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

	// Vérifier "WAVE"
	if string(header[8:12]) != "WAVE" {
		result.Error = NewValidationError("invalid_header", "missing WAVE signature", 8)
		return result
	}

	// Vérifier la taille (avec tolérance)
	if fileSize < expectedSize-2 {
		result.Error = NewValidationError("truncated",
			fmt.Sprintf("file size mismatch: header says %d, actual %d", expectedSize, fileSize),
			fileSize)
		return result
	}

	// Chercher les chunks obligatoires
	foundFmt := false
	foundData := false

	var offset int64 = 12
	chunkBuf := make([]byte, 8)

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

		switch chunkID {
		case "fmt ":
			foundFmt = true
			// Lire les infos du format
			if chunkSize >= 16 {
				fmtData := make([]byte, 16)
				if _, err := f.Read(fmtData); err == nil {
					audioFormat := binary.LittleEndian.Uint16(fmtData[0:2])
					channels := binary.LittleEndian.Uint16(fmtData[2:4])
					sampleRate := binary.LittleEndian.Uint32(fmtData[4:8])
					bitsPerSample := binary.LittleEndian.Uint16(fmtData[14:16])

					result.Details["audio_format"] = fmt.Sprintf("%d", audioFormat)
					result.Details["channels"] = fmt.Sprintf("%d", channels)
					result.Details["sample_rate"] = fmt.Sprintf("%d", sampleRate)
					result.Details["bits_per_sample"] = fmt.Sprintf("%d", bitsPerSample)
				}
			}
		case "data":
			foundData = true
			result.Details["data_size"] = fmt.Sprintf("%d", chunkSize)
		}

		// Passer au chunk suivant (aligné sur 2 bytes)
		if chunkSize%2 != 0 {
			chunkSize++
		}
		offset += 8 + chunkSize
	}

	// Vérifier les chunks obligatoires
	if !foundFmt {
		result.Error = NewValidationError("corrupted", "missing fmt chunk", -1)
		return result
	}

	if !foundData {
		result.Error = NewValidationError("corrupted", "missing data chunk", -1)
		return result
	}

	// WAV valide !
	result.Valid = true
	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION OGG
// ═══════════════════════════════════════════════════════════════════════════

// validateOGG vérifie l'intégrité d'un fichier OGG.
//
// STRUCTURE OGG :
// - Pages : "OggS" (4) + version (1) + flags (1) + granule (8) + ...
// - Chaque page a un CRC32
func (v *AudioValidator) validateOGG(f *os.File, result ValidationResult) ValidationResult {
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

	// Lire le header de la première page (27 bytes minimum)
	pageHeader := make([]byte, 27)
	if _, err := io.ReadFull(f, pageHeader); err != nil {
		result.Error = NewValidationError("truncated", "cannot read OGG page header", 0)
		return result
	}

	// Vérifier "OggS"
	if string(pageHeader[0:4]) != "OggS" {
		result.Error = NewValidationError("invalid_header", "missing OggS signature", 0)
		return result
	}

	// Version (doit être 0)
	if pageHeader[4] != 0 {
		result.Error = NewValidationError("corrupted",
			fmt.Sprintf("unsupported OGG version %d", pageHeader[4]), 4)
		return result
	}

	// Flags : bit 1 = BOS (beginning of stream)
	flags := pageHeader[5]
	isBOS := (flags & 0x02) != 0

	if !isBOS {
		result.Error = NewValidationError("corrupted", "first page missing BOS flag", 5)
		return result
	}

	// Serial number (pour identifier le stream)
	serialNumber := binary.LittleEndian.Uint32(pageHeader[14:18])
	result.Details["serial_number"] = fmt.Sprintf("%d", serialNumber)

	// Nombre de segments dans cette page
	numSegments := int(pageHeader[26])

	// Lire la table des segments
	segments := make([]byte, numSegments)
	if _, err := io.ReadFull(f, segments); err != nil {
		result.Error = NewValidationError("truncated", "cannot read segment table", 27)
		return result
	}

	// Calculer la taille totale des données de la page
	var pageDataSize int
	for _, s := range segments {
		pageDataSize += int(s)
	}

	// Vérifier qu'on peut lire les données
	firstPageEnd := int64(27 + numSegments + pageDataSize)
	if firstPageEnd > fileSize {
		result.Error = NewValidationError("truncated",
			fmt.Sprintf("first page extends beyond file: %d > %d", firstPageEnd, fileSize),
			27)
		return result
	}

	// Vérifier qu'on trouve EOS (end of stream) à la fin
	// On cherche "OggS" avec le flag EOS dans les derniers KB
	searchSize := int64(8192)
	if searchSize > fileSize {
		searchSize = fileSize
	}

	if _, err := f.Seek(-searchSize, 2); err != nil {
		result.Error = WrapError(err, "seek_error")
		return result
	}

	tailBuf := make([]byte, searchSize)
	n, err := f.Read(tailBuf)
	if err != nil && err != io.EOF {
		result.Error = WrapError(err, "read_error")
		return result
	}

	// Chercher la dernière page avec EOS
	foundEOS := false
	for i := n - 27; i >= 0; i-- {
		if string(tailBuf[i:i+4]) == "OggS" {
			// Vérifier le flag EOS (bit 2)
			if (tailBuf[i+5] & 0x04) != 0 {
				foundEOS = true
				break
			}
		}
	}

	if !foundEOS {
		result.Error = NewValidationError("truncated", "missing EOS page at end of file", fileSize-searchSize)
		return result
	}

	// OGG valide !
	result.Valid = true
	result.Details["has_bos"] = "true"
	result.Details["has_eos"] = "true"

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION AIFF
// ═══════════════════════════════════════════════════════════════════════════

// validateAIFF vérifie l'intégrité d'un fichier AIFF.
//
// STRUCTURE AIFF (IFF container) :
// - "FORM" (4) + size (4) + "AIFF" ou "AIFC" (4)
// - Chunks : "COMM" (common), "SSND" (sound data), etc.
func (v *AudioValidator) validateAIFF(f *os.File, result ValidationResult) ValidationResult {
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

	// Lire le header FORM (12 bytes)
	header := make([]byte, 12)
	if _, err := io.ReadFull(f, header); err != nil {
		result.Error = NewValidationError("truncated", "cannot read FORM header", 0)
		return result
	}

	// Vérifier "FORM"
	if string(header[0:4]) != "FORM" {
		result.Error = NewValidationError("invalid_header", "missing FORM signature", 0)
		return result
	}

	// Taille déclarée (big-endian pour AIFF!)
	declaredSize := binary.BigEndian.Uint32(header[4:8])
	expectedSize := int64(declaredSize) + 8

	// Vérifier "AIFF" ou "AIFC"
	formType := string(header[8:12])
	if formType != "AIFF" && formType != "AIFC" {
		result.Error = NewValidationError("invalid_header",
			fmt.Sprintf("expected AIFF or AIFC, got %s", formType), 8)
		return result
	}

	result.Details["form_type"] = formType

	// Vérifier la taille
	if fileSize < expectedSize-2 {
		result.Error = NewValidationError("truncated",
			fmt.Sprintf("file size mismatch: header says %d, actual %d", expectedSize, fileSize),
			fileSize)
		return result
	}

	// Chercher les chunks obligatoires
	foundComm := false
	foundSsnd := false

	var offset int64 = 12
	chunkBuf := make([]byte, 8)

	for offset < fileSize-8 {
		if _, err := f.Seek(offset, 0); err != nil {
			break
		}

		n, err := f.Read(chunkBuf)
		if err != nil || n < 8 {
			break
		}

		chunkID := string(chunkBuf[0:4])
		chunkSize := int64(binary.BigEndian.Uint32(chunkBuf[4:8])) // Big-endian!

		switch chunkID {
		case "COMM":
			foundComm = true
			// Lire les infos communes
			if chunkSize >= 18 {
				commData := make([]byte, 18)
				if _, err := f.Read(commData); err == nil {
					channels := binary.BigEndian.Uint16(commData[0:2])
					sampleRate80 := commData[8:18] // 80-bit extended float

					result.Details["channels"] = fmt.Sprintf("%d", channels)
					// Note: convertir le float 80-bit est complexe, on le saute
					_ = sampleRate80
				}
			}
		case "SSND":
			foundSsnd = true
			result.Details["sound_data_size"] = fmt.Sprintf("%d", chunkSize)
		}

		// Passer au chunk suivant (aligné sur 2 bytes)
		if chunkSize%2 != 0 {
			chunkSize++
		}
		offset += 8 + chunkSize
	}

	// Vérifier les chunks obligatoires
	if !foundComm {
		result.Error = NewValidationError("corrupted", "missing COMM chunk", -1)
		return result
	}

	if !foundSsnd {
		result.Error = NewValidationError("corrupted", "missing SSND chunk", -1)
		return result
	}

	// AIFF valide !
	result.Valid = true
	return result
}
