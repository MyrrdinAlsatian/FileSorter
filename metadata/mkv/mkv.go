// Package mkv extrait les métadonnées des fichiers vidéo Matroska (.mkv).
//
// CONCEPT : Parsing binaire EBML (Extensible Binary Meta Language)
// ================================================================
// Matroska utilise EBML, un format binaire similaire à XML mais en binaire.
// Chaque élément a :
// - Un identifiant (EBML ID) de 1 à 4 bytes
// - Une taille codée en VINT (Variable Integer)
// - Le contenu (données brutes ou sous-éléments)
//
// STRUCTURE D'UN FICHIER MKV :
// ┌─ EBML Header (signature du format)
// └─ Segment (conteneur principal)
//
//	├─ SeekHead (index des sections)
//	├─ Info (métadonnées générales)
//	│  ├─ TimecodeScale
//	│  ├─ Duration
//	│  ├─ DateUTC ◄── Date de création
//	│  └─ Title ◄── Titre principal
//	├─ Tracks (pistes audio/vidéo)
//	├─ Tags (métadonnées étendues)
//	│  └─ Tag
//	│     └─ SimpleTag
//	│        ├─ TagName = "TITLE" ◄── Titre alternatif
//	│        └─ TagString = "..."
//	└─ Clusters (données vidéo)
//
// STRATÉGIE D'EXTRACTION :
// 1. Chercher le titre dans Info → Title (le plus fiable)
// 2. Si absent, chercher dans Tags → SimpleTag où TagName = "TITLE"
// 3. Extraire DateUTC si disponible
// 4. En dernier recours, nettoyer le nom de fichier
package mkv

import (
	"bytes"
	"path/filepath"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// CONSTANTES EBML - Identifiants des éléments Matroska
// ═══════════════════════════════════════════════════════════════════════════

// EBML IDs (en hexadécimal, tels qu'ils apparaissent dans le fichier)
const (
	// Niveau supérieur
	idEBMLHeader = 0x1A45DFA3 // En-tête EBML
	idSegment    = 0x18538067 // Conteneur principal

	// Section Info (métadonnées de base)
	idInfo    = 0x1549A966 // Section Info
	idTitle   = 0x7BA9     // Titre du fichier (UTF-8)
	idDateUTC = 0x4461     // Date de création (nanosecondes depuis 2001-01-01)

	// Section Tags (métadonnées étendues)
	idTags      = 0x1254C367 // Conteneur des tags
	idTag       = 0x7373     // Un tag
	idSimpleTag = 0x67C8     // Tag simple (nom + valeur)
	idTagName   = 0x45A3     // Nom du tag
	idTagString = 0x4487     // Valeur du tag (chaîne)
)

// ScanSize définit la taille du buffer de lecture.
// 512KB est suffisant pour trouver les métadonnées dans la plupart des fichiers.
const ScanSize = 512 * 1024

// ═══════════════════════════════════════════════════════════════════════════
// STRUCTURES DE DONNÉES
// ═══════════════════════════════════════════════════════════════════════════

// MKVMetadata contient les métadonnées extraites d'un fichier MKV.
//
// Cette structure est conçue pour être facilement sérialisable en JSON.
// Le champ Source indique d'où provient le titre (utile pour le debug).
type MKVMetadata struct {
	Title  string    `json:"title,omitempty"`  // Titre extrait ou déduit
	Date   time.Time `json:"date,omitempty"`   // Date de création
	Source string    `json:"source,omitempty"` // Source du titre: "info", "tags", "filename"
	Valid  bool      `json:"valid"`            // True si au moins une donnée a été extraite
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTION PRINCIPALE
// ═══════════════════════════════════════════════════════════════════════════

// ParseFile extrait les métadonnées d'un fichier MKV à partir de son contenu.
//
// ALGORITHME :
// 1. Trouver l'élément Segment
// 2. Scanner les sous-éléments pour trouver Info et Tags
// 3. Extraire Title et DateUTC de Info
// 4. Si pas de titre dans Info, chercher dans Tags
// 5. Utiliser le nom de fichier comme fallback
//
// CONCEPT : Parser EBML sans bibliothèque externe
// Cette implémentation lit le format EBML "à la main" pour éviter
// les dépendances externes et garder le contrôle sur la performance.
//
// Paramètres :
//   - buf : contenu du fichier (premiers 512KB minimum)
//   - filename : nom du fichier (pour fallback)
//
// Retour :
//   - *MKVMetadata : métadonnées extraites
func ParseFile(buf []byte, filename string) *MKVMetadata {
	meta := &MKVMetadata{}

	// Étape 1 : Trouver le Segment
	segmentPos := findElement(buf, idSegment)
	if segmentPos < 0 {
		// Pas de segment trouvé → fallback au nom de fichier
		return fallbackToFilename(meta, filename)
	}

	// Sauter l'ID et la taille du Segment pour accéder au contenu
	_, idLen := readEBMLID(buf[segmentPos:])
	_, sizeLen := readEBMLSize(buf[segmentPos+idLen:])
	segmentContent := buf[segmentPos+idLen+sizeLen:]

	// Étape 2 : Scanner les éléments du Segment
	pos := 0
	maxScan := min(len(segmentContent), ScanSize)

	for pos < maxScan-8 {
		id, idLen := readEBMLID(segmentContent[pos:])
		if idLen == 0 {
			pos++
			continue
		}

		size, sizeLen := readEBMLSize(segmentContent[pos+idLen:])
		if sizeLen == 0 {
			pos++
			continue
		}

		dataStart := pos + idLen + sizeLen
		dataEnd := dataStart + size

		// Vérifier qu'on ne dépasse pas le buffer
		if dataEnd > len(segmentContent) {
			break
		}

		switch id {
		case idInfo:
			// Parser la section Info
			parseInfo(segmentContent[dataStart:dataEnd], meta)

		case idTags:
			// Parser la section Tags (seulement si pas de titre trouvé)
			if meta.Title == "" {
				parseTags(segmentContent[dataStart:dataEnd], meta)
			}
		}

		// Passer à l'élément suivant
		pos = dataEnd
	}

	// Étape 3 : Fallback au nom de fichier si nécessaire
	if meta.Title == "" {
		return fallbackToFilename(meta, filename)
	}

	meta.Valid = true
	return meta
}

// Parse est la fonction legacy pour compatibilité.
// Elle retourne juste le titre et un booléen.
//
// DEPRECATED : Utiliser ParseFile pour avoir toutes les métadonnées.
func Parse(buf []byte) (string, bool) {
	meta := ParseFile(buf, "")
	return meta.Title, meta.Title != ""
}

// ═══════════════════════════════════════════════════════════════════════════
// PARSEURS DE SECTIONS
// ═══════════════════════════════════════════════════════════════════════════

// parseInfo extrait les métadonnées de la section Info.
//
// Éléments recherchés :
// - Title (0x7BA9) : titre du fichier
// - DateUTC (0x4461) : date de création en nanosecondes depuis 2001-01-01
func parseInfo(buf []byte, meta *MKVMetadata) {
	pos := 0

	for pos < len(buf)-4 {
		id, idLen := readEBMLID(buf[pos:])
		if idLen == 0 {
			pos++
			continue
		}

		size, sizeLen := readEBMLSize(buf[pos+idLen:])
		if sizeLen == 0 {
			pos++
			continue
		}

		dataStart := pos + idLen + sizeLen
		dataEnd := dataStart + size

		if dataEnd > len(buf) {
			break
		}

		switch id {
		case idTitle:
			// Extraire le titre (chaîne UTF-8)
			title := extractString(buf[dataStart:dataEnd])
			if title != "" {
				meta.Title = title
				meta.Source = "info:Title"
				meta.Valid = true
			}

		case idDateUTC:
			// Extraire la date (int64 nanosecondes depuis 2001-01-01)
			if size == 8 {
				nanos := readInt64(buf[dataStart:dataEnd])
				// Époque Matroska : 2001-01-01 00:00:00 UTC
				epoch := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
				meta.Date = epoch.Add(time.Duration(nanos))
				meta.Valid = true
			}
		}

		pos = dataEnd
	}
}

// parseTags extrait le titre de la section Tags.
//
// Structure des Tags :
// Tags → Tag → SimpleTag → (TagName + TagString)
//
// On cherche un SimpleTag où TagName = "TITLE" (case-insensitive).
func parseTags(buf []byte, meta *MKVMetadata) {
	pos := 0

	for pos < len(buf)-4 {
		id, idLen := readEBMLID(buf[pos:])
		if idLen == 0 {
			pos++
			continue
		}

		size, sizeLen := readEBMLSize(buf[pos+idLen:])
		if sizeLen == 0 {
			pos++
			continue
		}

		dataStart := pos + idLen + sizeLen
		dataEnd := dataStart + size

		if dataEnd > len(buf) {
			break
		}

		if id == idTag {
			// Parser le contenu du Tag pour trouver les SimpleTags
			parseTag(buf[dataStart:dataEnd], meta)
			if meta.Title != "" {
				return // Titre trouvé, on arrête
			}
		}

		pos = dataEnd
	}
}

// parseTag parse un élément Tag pour trouver les SimpleTags.
func parseTag(buf []byte, meta *MKVMetadata) {
	pos := 0

	for pos < len(buf)-4 {
		id, idLen := readEBMLID(buf[pos:])
		if idLen == 0 {
			pos++
			continue
		}

		size, sizeLen := readEBMLSize(buf[pos+idLen:])
		if sizeLen == 0 {
			pos++
			continue
		}

		dataStart := pos + idLen + sizeLen
		dataEnd := dataStart + size

		if dataEnd > len(buf) {
			break
		}

		if id == idSimpleTag {
			// Parser le SimpleTag
			name, value := parseSimpleTag(buf[dataStart:dataEnd])
			if strings.EqualFold(name, "TITLE") && value != "" {
				meta.Title = value
				meta.Source = "tags:TITLE"
				meta.Valid = true
				return
			}
		}

		pos = dataEnd
	}
}

// parseSimpleTag extrait le nom et la valeur d'un SimpleTag.
//
// Retour :
//   - name : le nom du tag (ex: "TITLE", "ARTIST")
//   - value : la valeur du tag
func parseSimpleTag(buf []byte) (name, value string) {
	pos := 0

	for pos < len(buf)-4 {
		id, idLen := readEBMLID(buf[pos:])
		if idLen == 0 {
			pos++
			continue
		}

		size, sizeLen := readEBMLSize(buf[pos+idLen:])
		if sizeLen == 0 {
			pos++
			continue
		}

		dataStart := pos + idLen + sizeLen
		dataEnd := dataStart + size

		if dataEnd > len(buf) {
			break
		}

		switch id {
		case idTagName:
			name = extractString(buf[dataStart:dataEnd])
		case idTagString:
			value = extractString(buf[dataStart:dataEnd])
		}

		pos = dataEnd
	}

	return name, value
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS UTILITAIRES EBML
// ═══════════════════════════════════════════════════════════════════════════

// readEBMLID décode un identifiant EBML de 1 à 4 bytes.
//
// CONCEPT : Variable-Length ID
// Le premier octet indique la longueur :
// - 1xxxxxxx : 1 byte
// - 01xxxxxx : 2 bytes
// - 001xxxxx : 3 bytes
// - 0001xxxx : 4 bytes
//
// Retour :
//   - int : la valeur de l'ID
//   - int : le nombre de bytes lus (0 si erreur)
func readEBMLID(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}

	b := data[0]
	var length int

	// Déterminer la longueur en comptant les bits de poids fort à 0
	switch {
	case b&0x80 != 0:
		length = 1
	case b&0x40 != 0:
		length = 2
	case b&0x20 != 0:
		length = 3
	case b&0x10 != 0:
		length = 4
	default:
		return 0, 0 // ID invalide
	}

	if length > len(data) {
		return 0, 0
	}

	// Reconstruire la valeur (tous les bytes font partie de l'ID)
	val := int(b)
	for i := 1; i < length; i++ {
		val = (val << 8) | int(data[i])
	}

	return val, length
}

// readEBMLSize décode une taille EBML (VINT).
//
// CONCEPT : Variable Integer (VINT)
// Le premier octet indique la longueur ET contient une partie de la valeur :
// - 1xxxxxxx : 1 byte, valeur = xxxxxxx (7 bits)
// - 01xxxxxx : 2 bytes, valeur = xxxxxx + byte2 (14 bits)
// - 001xxxxx : 3 bytes, valeur = xxxxx + byte2 + byte3 (21 bits)
// etc.
//
// La différence avec l'ID est qu'on masque le bit de longueur pour la valeur.
//
// Retour :
//   - int : la valeur de la taille
//   - int : le nombre de bytes lus (0 si erreur)
func readEBMLSize(data []byte) (int, int) {
	if len(data) == 0 {
		return 0, 0
	}

	b := data[0]
	var length int
	var mask byte

	// Déterminer la longueur et le masque
	switch {
	case b&0x80 != 0:
		length = 1
		mask = 0x7F
	case b&0x40 != 0:
		length = 2
		mask = 0x3F
	case b&0x20 != 0:
		length = 3
		mask = 0x1F
	case b&0x10 != 0:
		length = 4
		mask = 0x0F
	case b&0x08 != 0:
		length = 5
		mask = 0x07
	case b&0x04 != 0:
		length = 6
		mask = 0x03
	case b&0x02 != 0:
		length = 7
		mask = 0x01
	case b&0x01 != 0:
		length = 8
		mask = 0x00
	default:
		return 0, 0
	}

	if length > len(data) {
		return 0, 0
	}

	// Reconstruire la valeur en masquant le(s) bit(s) de longueur
	val := int(b & mask)
	for i := 1; i < length; i++ {
		val = (val << 8) | int(data[i])
	}

	return val, length
}

// findElement cherche la position d'un élément EBML par son ID.
//
// Cette fonction effectue une recherche linéaire dans le buffer.
// Pour les grands fichiers, c'est moins efficace qu'utiliser SeekHead,
// mais c'est plus simple et fonctionne même si SeekHead est absent.
func findElement(buf []byte, targetID int) int {
	pos := 0

	for pos < len(buf)-8 {
		id, idLen := readEBMLID(buf[pos:])
		if idLen == 0 {
			pos++
			continue
		}

		if id == targetID {
			return pos
		}

		size, sizeLen := readEBMLSize(buf[pos+idLen:])
		if sizeLen == 0 {
			pos++
			continue
		}

		// Sauter cet élément
		// Pour les conteneurs (Segment), on ne saute pas le contenu
		if id == idSegment || id == idInfo || id == idTags {
			pos += idLen + sizeLen
		} else {
			pos += idLen + sizeLen + size
		}
	}

	return -1 // Non trouvé
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS UTILITAIRES
// ═══════════════════════════════════════════════════════════════════════════

// extractString extrait une chaîne UTF-8 nettoyée d'un buffer.
func extractString(buf []byte) string {
	// Supprimer les caractères null de fin
	clean := bytes.TrimRight(buf, "\x00")
	s := string(clean)
	// Nettoyer les espaces
	return strings.TrimSpace(s)
}

// readInt64 lit un entier 64 bits big-endian.
func readInt64(buf []byte) int64 {
	if len(buf) < 8 {
		return 0
	}
	var val int64
	for i := 0; i < 8; i++ {
		val = (val << 8) | int64(buf[i])
	}
	return val
}

// fallbackToFilename crée un titre à partir du nom de fichier.
//
// NETTOYAGE :
// - Supprime l'extension
// - Remplace les underscores et points par des espaces
// - Supprime les préfixes courants (recup_dir.*, [brackets], etc.)
// - Nettoie les années entre parenthèses mal formatées
func fallbackToFilename(meta *MKVMetadata, filename string) *MKVMetadata {
	if filename == "" {
		return meta
	}

	// Supprimer l'extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Remplacer les séparateurs courants par des espaces
	replacer := strings.NewReplacer(
		"_", " ",
		".", " ",
		"-", " ",
	)
	name = replacer.Replace(name)

	// Supprimer les patterns de récupération (recup_dir.123, f123456789)
	// Pattern: tout ce qui ressemble à un ID de récupération
	if strings.HasPrefix(strings.ToLower(name), "recup") {
		// Fichier de récupération sans nom utile
		return meta
	}

	// Nettoyer les espaces multiples
	name = strings.Join(strings.Fields(name), " ")

	if name != "" && len(name) > 2 {
		meta.Title = name
		meta.Source = "filename"
		meta.Valid = true
	}

	return meta
}

// min retourne le minimum de deux entiers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
