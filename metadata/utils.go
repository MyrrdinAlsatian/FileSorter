package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"FileRecoveryOrganizer/metadata/mkv"
	"FileRecoveryOrganizer/metadata/mp4"

	"github.com/dhowden/tag"
	"github.com/rwcarlsen/goexif/exif"
)

// GetFileMeta extrait les métadonnées appropriées selon le type de fichier.
//
// Cette fonction centralise la détection et l'extraction de métadonnées :
// - Images (jpg, png, etc.) : extrait les données EXIF
// - Audio/Vidéo (mp3, mp4, mkv, etc.) : extrait les tags ID3 et métadonnées vidéo
//
// EXTENSION : Par rapport à la version précédente, cette fonction enrichit maintenant
// les données avec un map AdditionalInfo contenant :
// - Artist, Album, Genre, FileType pour l'audio
// - Données vidéo MP4 (dimensions, durée)
// - Titre MKV pour les vidéos Matroska
//
// CONCEPT d'abstraction : Cette fonction cache les détails d'implémentation.
// L'appelant n'a pas besoin de savoir comment fonctionnent EXIF, ID3 ou les parsers vidéo,
// il reçoit simplement une structure FileData enrichie.
//
// Paramètres :
//   - path : chemin complet vers le fichier
//   - fileType : type/extension du fichier (ex: "jpg", "mp3", "mp4", "mkv")
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
	// Les tags ID3 sont des métadonnées standardisées dans les fichiers audio
	metaTags, err := tag.ReadFrom(f)
	meta.AdditionalInfo = make(map[string]string) // Initialiser la map pour les données supplémentaires

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

	// Ajouter des informations supplémentaires en tant que key-value
	// Cette map permet de stocker des données optionnelles sans modifier la structure FileData
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

	// Traitement spécial pour MP4 : extraire les métadonnées vidéo
	if fileType == "mp4" {
		mp4Meta, err := mp4.Parse(path)
		if err == nil && mp4Meta != nil {
			// Convertir les métadonnées MP4 en string pour le stockage dans la map
			// %+v affiche la structure avec les noms des champs
			meta.AdditionalInfo["Video"] = fmt.Sprintf("%+v", mp4Meta)
		}
	}

	// Traitement spécial pour MKV : extraire les métadonnées Matroska
	// Le parser MKV cherche le titre dans :
	// 1. Info → Title (méthode principale)
	// 2. Tags → SimpleTag TITLE (méthode alternative)
	// 3. Nom de fichier nettoyé (fallback)
	if fileType == "mkv" {
		mkvFile, err := os.Open(path)
		if err == nil {
			defer mkvFile.Close()

			// Lire les premiers 512KB (suffisant pour les métadonnées)
			mkvBuf := make([]byte, mkv.ScanSize)
			n, _ := mkvFile.Read(mkvBuf)

			if n > 0 {
				// Extraire le nom de fichier pour le fallback
				filename := filepath.Base(path)

				// Parser avec la nouvelle fonction qui retourne toutes les métadonnées
				mkvMeta := mkv.ParseFile(mkvBuf[:n], filename)

				if mkvMeta.Valid {
					// Stocker le titre
					if mkvMeta.Title != "" {
						meta.OriginalName = mkvMeta.Title
						meta.Source = "mkv:" + mkvMeta.Source
					}

					// Stocker la date si disponible
					if !mkvMeta.Date.IsZero() {
						meta.Time = mkvMeta.Date
						meta.Valid = true
					}

					// Ajouter les infos dans AdditionalInfo pour le JSON
					meta.AdditionalInfo["VideoTitle"] = mkvMeta.Title
					meta.AdditionalInfo["VideoSource"] = mkvMeta.Source
					if !mkvMeta.Date.IsZero() {
						meta.AdditionalInfo["VideoDate"] = mkvMeta.Date.Format(time.RFC3339)
					}
				}
			}
		}
	}

	return meta
}
