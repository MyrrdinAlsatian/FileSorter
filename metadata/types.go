package metadata

import "time"

// ImageTypes contient les extensions d'images supportées pour l'extraction EXIF
var ImageTypes = map[string]bool{
	"jpg": true, "jpeg": true, "png": true,
	"tiff": true, "heic": true, "heif": true, "webp": true,
}

// IsImageType vérifie si le type de fichier est une image supportée
func IsImageType(fileType string) bool {
	return ImageTypes[fileType]
}

// FileData contient les métadonnées de date d'un fichier
type FileData struct {
	OriginalName string    `json:"original_name,omitempty"`
	Time         time.Time `json:"time,omitempty"`
	Source       string    `json:"source,omitempty"` // Source of the metadata (e.g., "filesystem", "exif", etc.)
	Valid        bool      `json:"valid,omitempty"`
}
