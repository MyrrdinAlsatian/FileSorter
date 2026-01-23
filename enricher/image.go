// Package enricher enrichit les résultats avec des données supplémentaires.
//
// Ce package extrait des informations détaillées des fichiers, notamment
// les métadonnées EXIF complètes des images.
package enricher

import (
	"image"
	"os"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"FileRecoveryOrganizer/types"

	"github.com/rwcarlsen/goexif/exif"
)

// EnrichImage enrichit un Result avec les métadonnées EXIF complètes d'une image.
//
// CONCEPT : Ce processus "enrichit" les données en ajoutant des informations supplémentaires.
// Au départ, on connaît juste le chemin et le type. Après EnrichImage, on sait :
// - Les dimensions de l'image
// - L'appareil photo utilisé
// - La date de prise de vue
// - La localisation GPS
// etc.
//
// PROCESSUS :
// 1. Ouvrir le fichier image
// 2. Décoder les métadonnées EXIF
// 3. Extraire les dimensions de l'image
// 4. Extraire les informations EXIF pertinentes
// 5. Créer une structure ImageMeta et l'assigner au Result
//
// Paramètres :
//   - res : pointeur vers le Result à enrichir
//     Cette fonction MODIFIE res en définissant res.Image
//
// Note : Si une erreur survient (fichier illisible, pas d'EXIF, etc.),
// la fonction retourne silencieusement. res.Image restera nil.
func EnrichImage(res *types.Result) {
	f, err := os.Open(res.Path)
	if err != nil {
		return // Impossible d'ouvrir le fichier
	}
	defer f.Close()

	// Décoder les données EXIF
	// exif.Decode lit le fichier image et extrait les métadonnées EXIF
	x, err := exif.Decode(f)
	if err != nil {
		return // Le fichier n'a pas de données EXIF valides
	}

	// Créer la structure ImageMeta pour stocker les résultats
	meta := &types.ImageMeta{
		HasExif: true,
	}

	// Essayer de décoder les dimensions de l'image
	if cfg, _, err := image.DecodeConfig(f); err == nil {
		meta.Width = cfg.Width
		meta.Height = cfg.Height
	}

	// Remettre le curseur de fichier au début
	// (le décalage des dimensions a bougé le pointeur)
	_, _ = f.Seek(0, 0)

	// Créer la structure pour les données EXIF détaillées
	iexif := &types.ImageExif{}

	// Extraire le modèle de l'appareil photo
	if camModel, err := x.Get(exif.Model); err == nil {
		iexif.CameraModel, _ = camModel.StringVal()
	}

	// Extraire la date de prise de vue
	if dt, err := x.Get(exif.DateTimeOriginal); err == nil {
		iexif.DateTaken, _ = dt.StringVal()
	}

	// Extraire la localisation GPS si disponible
	if lat, lon, err := x.LatLong(); err == nil {
		iexif.GPSLatitude = lat
		iexif.GPSLongitude = lon
	}

	// Assigner les données EXIF au Result
	meta.Exif = iexif
	res.Image = meta
}
