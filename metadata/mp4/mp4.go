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
// └─ moov (movie metadata)
//
//	├─ mvhd (movie header - durée, timescale)
//	├─ trak (track)
//	│  ├─ tkhd (track header - dimensions)
//	│  └─ mdia (media)
//	└─ ...
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
// Chaque boîte commence par ces 4 bytes
const (
	boxMvhd = "mvhd" // Movie header (durée, timescale)
	boxTkhd = "tkhd" // Track header (dimensions)
	boxCnam = "@nam" // Titre
)

// Parse extrait les métadonnées d'un fichier MP4.
//
// PROCESSUS :
// 1. Ouvrir le fichier MP4
// 2. Lire les boîtes en séquence
// 3. Parsifier les boîtes importantes (mvhd pour durée, tkhd pour dimensions)
// 4. Retourner la structure remplie
//
// NOTE : Cette implémentation est simplifiée. Un parseur MP4 complet
// devrait gérer la hiérarchie complète des boîtes.
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

	meta := &MP4Metadata{}

	for {
		// Lire la taille de la boîte (4 bytes, big-endian)
		// Big-endian : octet de poids fort en premier
		var size uint32
		var boxType [4]byte

		if err := binary.Read(f, binary.BigEndian, &size); err != nil {
			return meta, nil
		}

		// Lire le type de la boîte (4 bytes)
		if _, err := io.ReadFull(f, boxType[:]); err != nil {
			return meta, nil
		}

		// Dispatcher selon le type de boîte
		switch string(boxType[:]) {
		case boxMvhd:
			// Movie header : contient la durée et le timescale
			ParseMvhd(f, meta, size)
		case boxTkhd:
			// Track header : contient les dimensions (largeur x hauteur)
			ParseTkhd(f, meta, size)
		default:
			// Boîte inconnue : sauter le contenu
			if _, err := f.Seek(int64(size-8), io.SeekCurrent); err != nil {
				return meta, nil
			}
		}
	}
}

// ParseMvhd extrait les informations du Movie Header.
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
func ParseMvhd(r io.Reader, meta *MP4Metadata, size uint32) {
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
		meta.Duration = time.Duration(duration) * time.Second / time.Duration(timescale)
		// Convertir le timestamp Unix (MP4 utilise un epoch décalé)
		meta.Date = time.Unix(int64(ctime-2_082_844_800), 0)
		meta.Valid = true
	} else {
		// Version 1 : 64 bits (non implémenté pour simplifier)
		skip := make([]byte, size-8-4)
		io.ReadFull(r, skip)
	}
}

// ParseTkhd extrait les dimensions du Track Header.
//
// Le Track Header (tkhd) contient une matrice de transformation
// qui inclut la largeur et hauteur en format fixed-point 16.16.
// Cela signifie que les 16 bits hauts = partie entière,
// les 16 bits bas = décimales.
//
// Pour obtenir la largeur/hauteur entière : >> 16 (décalage à droite)
func ParseTkhd(r io.Reader, meta *MP4Metadata, size uint32) {
	// Sauter les 76 premiers bytes
	skip := make([]byte, 76)
	io.ReadFull(r, skip)

	// Lire width et height en format fixed-point 16.16
	var widthFixed, heightFixed uint32
	binary.Read(r, binary.BigEndian, &widthFixed)
	binary.Read(r, binary.BigEndian, &heightFixed)

	// Extraire la partie entière (16 bits hauts)
	meta.Width = int(widthFixed >> 16)
	meta.Height = int(heightFixed >> 16)
}
