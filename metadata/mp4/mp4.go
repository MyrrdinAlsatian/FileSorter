package mp4

// Package mp4 extrait les métadonnées des fichiers vidéo MP4.
//
// CONCEPT : Parsing binaire avec structure d'atomes
// ================================================
// MP4 est un format conteneur hiérarchique où les données sont organisées
// en "boîtes" (boxes/atoms). Chaque boîte a :
// - Taille (4 bytes, big-endian)
// - Type (4 bytes, identifiant ASCII)
// - Contenu (données brutes ou sous-boîtes)
//
// STRUCTURE SIMPLIFIÉE D'UN MP4 :
// ┌─ ftyp (file type)
// ├─ mdat (media data - les vidéos/audios)
// └─ moov (movie metadata)       ◄── Conteneur principal
//
//	├─ mvhd (movie header - durée, timescale)
//	├─ trak (track)
//	│  ├─ tkhd (track header - dimensions)
//	│  └─ mdia (media)
//	└─ udta (user data)
//	   └─ meta
//	      └─ ilst
//	         └─ ©nam (title)       ◄── Le titre est ici !
//
// Cette structure permet au lecteur de sauter les données sans les traiter.

import (
	"encoding/binary"
	"io"
	"os"
	"time"
)

// MP4Metadata contient les métadonnées extraites d'un fichier MP4.
//
// Ces données permettent de connaître les caractéristiques vidéo sans décoder
// la vidéo complète (qui prendrait beaucoup de temps).
type MP4Metadata struct {
	Title    string        `json:"title,omitempty"`    // Titre de la vidéo
	Width    int           `json:"width,omitempty"`    // Largeur en pixels
	Height   int           `json:"height,omitempty"`   // Hauteur en pixels
	Duration time.Duration `json:"duration,omitempty"` // Durée totale
	Date     time.Time     `json:"date,omitempty"`     // Date de création
	Valid    bool          `json:"valid"`              // Indique si les données sont valides
}

// Constantes des identifiants de boîtes MP4
// Ces identifiants sont des chaînes ASCII de 4 caractères
const (
	boxMoov = "moov" // Movie container - CONTIENT mvhd, trak, udta
	boxMvhd = "mvhd" // Movie header (durée, timescale)
	boxTrak = "trak" // Track container
	boxTkhd = "tkhd" // Track header (dimensions)
	boxUdta = "udta" // User data container
	boxMeta = "meta" // Metadata container
	boxIlst = "ilst" // iTunes-style metadata list
	boxNam  = "©nam" // Title (iTunes format)
	boxNam2 = "@nam" // Title (autre format)
)

// containerBoxes liste les boîtes qui contiennent d'autres boîtes
// On doit descendre dans celles-ci pour trouver les métadonnées
var containerBoxes = map[string]bool{
	"moov": true,
	"trak": true,
	"mdia": true,
	"minf": true,
	"stbl": true,
	"udta": true,
	"meta": true,
	"ilst": true,
}

// Parse extrait les métadonnées d'un fichier MP4.
//
// PROCESSUS AMÉLIORÉ :
// 1. Ouvrir le fichier MP4
// 2. Parcourir récursivement les boîtes
// 3. Descendre dans les boîtes conteneurs (moov, trak, udta, meta, ilst)
// 4. Extraire mvhd (durée), tkhd (dimensions), ©nam (titre)
// 5. Retourner la structure remplie
//
// Paramètres :
//   - path : chemin vers le fichier MP4
//
// Retour :
//   - *MP4Metadata : métadonnées extraites
//   - error : erreur si fichier illisible
func Parse(path string) (*MP4Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return &MP4Metadata{Valid: false}, err
	}
	defer f.Close()

	// Obtenir la taille du fichier
	stat, err := f.Stat()
	if err != nil {
		return &MP4Metadata{Valid: false}, err
	}

	meta := &MP4Metadata{}
	parseBoxes(f, meta, stat.Size())

	return meta, nil
}

// parseBoxes parcourt les boîtes MP4 de manière récursive.
//
// Cette fonction lit les boîtes séquentiellement et descend dans
// les conteneurs pour trouver les métadonnées imbriquées.
func parseBoxes(r io.ReadSeeker, meta *MP4Metadata, endPos int64) {
	for {
		currentPos, _ := r.Seek(0, io.SeekCurrent)
		if currentPos >= endPos {
			return
		}

		// Lire l'en-tête de la boîte (8 bytes minimum)
		var size uint32
		var boxType [4]byte

		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return
		}
		if _, err := io.ReadFull(r, boxType[:]); err != nil {
			return
		}

		boxName := string(boxType[:])
		boxDataSize := int64(size) - 8 // Taille des données (sans l'en-tête)

		// Gérer les tailles spéciales
		if size == 0 {
			// size=0 signifie "jusqu'à la fin du fichier"
			boxDataSize = endPos - currentPos - 8
		} else if size == 1 {
			// size=1 signifie "taille étendue sur 64 bits"
			var extendedSize uint64
			if err := binary.Read(r, binary.BigEndian, &extendedSize); err != nil {
				return
			}
			boxDataSize = int64(extendedSize) - 16
		}

		if boxDataSize < 0 {
			return
		}

		boxEndPos := currentPos + 8 + boxDataSize

		// Traiter selon le type de boîte
		switch boxName {
		case boxMvhd:
			parseMvhd(r, meta)
		case boxTkhd:
			parseTkhd(r, meta)
		case "©nam", "@nam", "\xa9nam":
			// Boîte titre - lire le contenu
			parseNameBox(r, meta, boxDataSize)
		case "meta":
			// meta a parfois 4 bytes de flags supplémentaires
			var flags [4]byte
			r.Read(flags[:])
			// Puis descendre dans le contenu
			parseBoxes(r, meta, boxEndPos)
		default:
			// Si c'est un conteneur, descendre dedans
			if containerBoxes[boxName] {
				parseBoxes(r, meta, boxEndPos)
			}
		}

		// Aller à la fin de cette boîte pour lire la suivante
		r.Seek(boxEndPos, io.SeekStart)
	}
}

// parseNameBox extrait le titre d'une boîte ©nam ou @nam
//
// Structure iTunes :
// ©nam
//
//	└── data (sous-boîte)
//	    ├── type (4 bytes) - souvent 0x00000001 pour UTF-8
//	    ├── locale (4 bytes) - souvent 0x00000000
//	    └── texte UTF-8
func parseNameBox(r io.ReadSeeker, meta *MP4Metadata, size int64) {
	if size > 1024 || size < 8 {
		return
	}

	startPos, _ := r.Seek(0, io.SeekCurrent)
	data := make([]byte, size)
	n, _ := r.Read(data)
	if n < 8 {
		return
	}

	// Chercher la sous-boîte "data" dans le contenu
	// Format : [4 bytes taille][4 bytes "data"][8 bytes header][texte]
	for i := 0; i < n-16; i++ {
		// Chercher "data" comme identifiant de boîte
		if data[i+4] == 'd' && data[i+5] == 'a' && data[i+6] == 't' && data[i+7] == 'a' {
			// Lire la taille de la boîte data
			dataBoxSize := int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
			if dataBoxSize < 16 || dataBoxSize > n-i {
				continue
			}

			// Le texte commence après : 4 (size) + 4 (type "data") + 4 (data type) + 4 (locale)
			textStart := i + 16
			textEnd := i + dataBoxSize

			if textStart < textEnd && textEnd <= n {
				text := string(data[textStart:textEnd])
				text = cleanString(text)
				if text != "" && text != "data" && len(text) > 1 {
					meta.Title = text
					meta.Valid = true
					return
				}
			}
		}
	}

	// Fallback : chercher du texte directement si pas de boîte "data"
	// (certains MP4 n'utilisent pas le format iTunes)
	r.Seek(startPos, io.SeekStart)
	text := extractText(data[:n])
	if text != "" && text != "data" && len(text) > 1 {
		meta.Title = text
		meta.Valid = true
	}
}

// cleanString nettoie une chaîne en supprimant les caractères nuls et les espaces
func cleanString(s string) string {
	// Supprimer les caractères nuls
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != 0 && s[i] >= 32 {
			result = append(result, s[i])
		}
	}
	// Trim les espaces
	start := 0
	end := len(result)
	for start < end && result[start] == ' ' {
		start++
	}
	for end > start && result[end-1] == ' ' {
		end--
	}
	return string(result[start:end])
}

// extractText extrait le texte UTF-8 d'un buffer
// en sautant les en-têtes binaires éventuels
func extractText(data []byte) string {
	// Chercher le premier caractère imprimable (après position 8 pour éviter les headers)
	startSearch := 0
	if len(data) > 16 {
		startSearch = 8 // Sauter les headers potentiels
	}
	for i := startSearch; i < len(data); i++ {
		if data[i] >= 32 && data[i] < 127 {
			// Trouver la fin de la chaîne
			end := i
			for end < len(data) && data[end] != 0 && data[end] >= 32 {
				end++
			}
			if end > i+1 { // Au moins 2 caractères
				text := string(data[i:end])
				// Ignorer si c'est juste "data"
				if text != "data" {
					return text
				}
			}
		}
	}
	return ""
}

// parseMvhd extrait les informations du Movie Header.
//
// Le Movie Header (mvhd) contient :
// - Creation time (32 ou 64 bits selon version)
// - Modification time
// - Timescale : fréquence d'horloge (ex: 1000 = 1 tick = 1ms)
// - Duration : nombre de ticks
//
// La durée réelle = duration / timescale (en secondes)
//
// Exemple :
// - Duration = 30000, Timescale = 1000 -> 30 secondes
// - Duration = 120000, Timescale = 1000 -> 120 secondes (2 minutes)
func parseMvhd(r io.Reader, meta *MP4Metadata) {
	var versionAndFlags [1]byte
	r.Read(versionAndFlags[:])
	// Sauter les 3 bytes de flags
	skip := make([]byte, 3)
	io.ReadFull(r, skip)

	if versionAndFlags[0] == 0 {
		// Version 0 : 32 bits pour les timestamps
		var ctime, mtime, timescale, duration uint32

		binary.Read(r, binary.BigEndian, &ctime)
		binary.Read(r, binary.BigEndian, &mtime)
		binary.Read(r, binary.BigEndian, &timescale)
		binary.Read(r, binary.BigEndian, &duration)

		// Convertir en time.Duration
		if timescale > 0 {
			meta.Duration = time.Duration(duration) * time.Second / time.Duration(timescale)
		}
		// Convertir le timestamp (MP4 utilise un epoch du 1er janvier 1904)
		if ctime > 2082844800 { // Éviter les dates invalides
			meta.Date = time.Unix(int64(ctime-2_082_844_800), 0)
		}
		meta.Valid = true
	} else {
		// Version 1 : 64 bits pour les timestamps
		var ctime, mtime uint64
		var timescale uint32
		var duration uint64

		binary.Read(r, binary.BigEndian, &ctime)
		binary.Read(r, binary.BigEndian, &mtime)
		binary.Read(r, binary.BigEndian, &timescale)
		binary.Read(r, binary.BigEndian, &duration)

		if timescale > 0 {
			meta.Duration = time.Duration(duration) * time.Second / time.Duration(timescale)
		}
		if ctime > 2082844800 {
			meta.Date = time.Unix(int64(ctime-2_082_844_800), 0)
		}
		meta.Valid = true
	}
}

// parseTkhd extrait les dimensions du Track Header.
//
// Le Track Header (tkhd) contient une matrice de transformation
// qui inclut la largeur et hauteur en format fixed-point 16.16.
// Cela signifie que les 16 bits hauts = partie entière,
// les 16 bits bas = décimales.
//
// Pour obtenir la largeur/hauteur entière : >> 16 (décalage à droite)
func parseTkhd(r io.Reader, meta *MP4Metadata) {
	// Sauter les 76 premiers bytes (version, flags, timestamps, etc.)
	skip := make([]byte, 76)
	io.ReadFull(r, skip)

	// Lire width et height en format fixed-point 16.16
	var widthFixed, heightFixed uint32
	binary.Read(r, binary.BigEndian, &widthFixed)
	binary.Read(r, binary.BigEndian, &heightFixed)

	// Extraire la partie entière (16 bits hauts)
	w := int(widthFixed >> 16)
	h := int(heightFixed >> 16)

	// Garder seulement si ce sont des valeurs raisonnables
	if w > 0 && h > 0 && w < 10000 && h < 10000 {
		meta.Width = w
		meta.Height = h
	}
}
