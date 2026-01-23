package metadata

import (
	"os"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/rwcarlsen/goexif/exif"
)

type FileMeta struct {
	OriginalName string
	Time         *time.Time
	Source       string // Source of the metadata (e.g., "filesystem", "exif", etc.)
}

func GetFileMeta(path string, fileType string) *FileMeta {

	meta := &FileMeta{}

	f, err := os.Open(path)
	if err != nil {
		return meta
	}
	defer f.Close()

	switch strings.ToLower(fileType) {
	case "jpeg", "jpg", "png", "tiff", "heic", "heif", "webp":

		exifData, err := exif.Decode(f)
		if err == nil {
			if dt, err := exifData.DateTime(); err == nil {
				meta.Time = &dt
				meta.Source = "exif:DateTimeOriginal"
			}
		}

	default:
		// Implement audio metadata extraction if needed
		metaTags, err := tag.ReadFrom(f)
		if err != nil {
			return meta
		}

		if title := metaTags.Title(); title != "" {
			meta.OriginalName = title
			meta.Source = "tag:Title"
		}

		if year := metaTags.Year(); year != 0 {
			tm := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
			meta.Time = &tm
			meta.Source = "tag:Year"
		}
	}

	return meta
}
