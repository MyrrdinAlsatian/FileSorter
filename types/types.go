package types

type Stats struct {
	TotalFiles int
	TotalDirs  int
	TotalSize  int64

	DetectedFileType map[string]int
}

type ImageExif struct {
	DateTaken    string  `json:"date_taken,omitempty"`
	CameraModel  string  `json:"camera_model,omitempty"`
	GPSLatitude  float64 `json:"lat,omitempty"`
	GPSLongitude float64 `json:"lon,omitempty"`
}

type Result struct {
	Path  string     `json:"path"`
	Size  int64      `json:"size"`
	Type  string     `json:"type"`
	Image *ImageExif `json:"image_exif,omitempty"`
	Error string     `json:"error,omitempty"`
}

type StatsFile struct {
	FilesByType map[string]int64
	BytesByType map[string]int64
	Errors      int64
}
