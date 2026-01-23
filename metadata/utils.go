package metadata

import (
	"os"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/rwcarlsen/goexif/exif"
)

// GetFileMeta extrait les métadonnées d'un fichier selon son type
func GetFileMeta(path string, fileType string) *FileData {
	meta := &FileData{}

	f, err := os.Open(path)
	if err != nil {
		return meta
	}
	defer f.Close()

	lowerType := strings.ToLower(fileType)

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

	// Audio/Video metadata via tags
	metaTags, err := tag.ReadFrom(f)
	if err != nil {
		return meta
	}

	if title := metaTags.Title(); title != "" {
		meta.OriginalName = title
		meta.Source = "tag:Title"
	}

	if year := metaTags.Year(); year != 0 {
		meta.Time = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		meta.Source = "tag:Year"
		meta.Valid = true
	}

	return meta
}
