package metadata

import (
	"os"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/rwcarlsen/goexif/exif"
)

// GetFileMeta extrait les métadonnées appropriées selon le type de fichier.
//
// Cette fonction centralise la détection et l'extraction de métadonnées :
// - Images (jpg, png, etc.) : extrait les données EXIF
// - Audio/Vidéo (mp3, mp4, etc.) : extrait les tags ID3 (titre, année, etc.)
//
// CONCEPT d'abstraction : Cette fonction cache les détails d'implémentation.
// L'appelant n'a pas besoin de savoir comment fonctionnent EXIF ou ID3,
// il reçoit simplement une structure FileData.
//
// Paramètres :
//   - path : chemin complet vers le fichier
//   - fileType : type/extension du fichier (ex: "jpg", "mp3")
//
// Retour :
//   - *FileData : pointeur vers la structure contenant les métadonnées
//
// Note : Retourne toujours un pointeur (même si vide) pour cohérence.
// Vérifier FileData.Valid pour savoir si les données ont été trouvées.
func GetFileMeta(path string, fileType string) *FileData {
	meta := &FileData{} // Crée une structure vide

	f, err := os.Open(path)
	if err != nil {
		return meta
	}
	defer f.Close()

	lowerType := strings.ToLower(fileType) // Convertit en minuscules pour comparaison

	// Pour les images : extraire EXIF
	if IsImageType(lowerType) {
		exifData, err := exif.Decode(f)
		if err == nil {
			if dt, err := exifData.DateTime(); err == nil {
				meta.Time = dt
				meta.Source = "exif:DateTimeOriginal"
				meta.Valid = true
			}
		}
		return meta
	}

	// Pour les autres types (audio, vidéo, etc.) : extraire les tags
	// Les tags ID3 sont des métadonnées dans les fichiers audio
	metaTags, err := tag.ReadFrom(f)
	meta.AdditionalInfo = make(map[string]string)
	if err != nil {
		return meta
	}

	// Essayer d'extraire le titre
	if title := metaTags.Title(); title != "" {
		meta.OriginalName = title
		meta.Source = "tag:Title"
	}

	// Essayer d'extraire l'année
	if year := metaTags.Year(); year != 0 {
		meta.Time = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		meta.Source = "tag:Year"
		meta.Valid = true
	}
	// Ajouter des informations supplémentaires
	if artist := metaTags.Artist(); artist != "" {
		meta.AdditionalInfo["Artist"] = artist
	}
	if album := metaTags.Album(); album != "" {
		meta.AdditionalInfo["Album"] = album
	}
	if genre := metaTags.Genre(); genre != "" {
		meta.AdditionalInfo["Genre"] = genre
	}
	if typeFile := metaTags.FileType(); typeFile != "" {
		meta.AdditionalInfo["FileType"] = string(typeFile)
	}
	return meta
}
