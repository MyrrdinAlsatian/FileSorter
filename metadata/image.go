package metadata

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// ImageData extrait la date des métadonnées EXIF d'une image.
//
// EXIF (Exchangeable Image File Format) contient les métadonnées des images
// numériques, incluant :
// - DateTimeOriginal : quand la photo a été prise
// - CreateDate : quand le fichier a été créé
// - ModifyDate : quand le fichier a été modifié
//
// Cette fonction essaie ces dates dans l'ordre de priorité.
//
// Paramètres :
//   - path : chemin vers le fichier image
//
// Retour :
//   - FileData : contient la date EXIF trouvée, ou une structure invalide
//
// Note : Cette fonction nécessite une image valide avec des données EXIF.
// Si le fichier n'est pas une image ou n'a pas de données EXIF, Valid sera false.
func ImageData(path string) FileData {

	f, err := os.Open(path)
	if err != nil {
		return FileData{Valid: false}
	}
	defer f.Close() // defer assure que le fichier est fermé même en cas d'erreur

	// Decode extrait les données EXIF du fichier image
	x, err := exif.Decode(f)
	if err != nil {
		return FileData{Valid: false}
	}

	// Date Priority: DateTimeOriginal > CreateDate > ModifyDate
	// On essaie chaque champ dans l'ordre jusqu'à en trouver un valide
	tags := []string{
		"DateTimeOriginal",
		"CreateDate",
		"ModifyDate",
	}

	for _, tag := range tags {
		// x.Get(exif.FieldName(tag)) retourne le champ EXIF demandé
		if t, err := x.Get(exif.FieldName(tag)); err == nil {
			dateStr, err := t.StringVal()
			if err == nil {
				// EXIF date format: "2006:01:02 15:04:05"
				// Cette chaîne spéciale (appelée "layout" en Go) indique le format des dates
				// 2006 = année, 01 = mois, 02 = jour, 15 = heure, 04 = minute, 05 = seconde
				const exifDateFormat = "2006:01:02 15:04:05"

				// time.Parse convertit une chaîne en objet time.Time selon un layout
				if tm, err := time.Parse(exifDateFormat, dateStr); err == nil {
					return FileData{
						Time:   tm,
						Source: "exif:" + tag,
						Valid:  true,
					}
				}
			}
		}
	}

	return FileData{Valid: false}
}
