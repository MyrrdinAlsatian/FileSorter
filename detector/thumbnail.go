package detector

import (
	"FileRecoveryOrganizer/types"
	"path/filepath"
	"strings"
)

func DetectThumbnail(path string, size int64, img *types.ImageMeta) bool {

	if img == nil {
		return false
	}
	name := strings.ToLower(filepath.Base(path))

	if !img.HasExif && strings.HasPrefix(name, "t") {
		if size < 50_000 {
			img.IsThumb = true
			img.Reason = "Filename starts with 't' and size < 50KB without EXIF"
			return true
		}
	}

	return false
}
