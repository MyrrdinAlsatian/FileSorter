package metadata

import "time"

type FileData struct {
	OriginalName string
	Time         time.Time
	Source       string // Source of the metadata (e.g., "filesystem", "exif", etc.)
	Valid        bool
}
