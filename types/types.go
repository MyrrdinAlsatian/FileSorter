package types

type Stats struct {
	TotalFiles int
	TotalDirs  int
	TotalSize  int64

	DetectedFileType map[string]int
}

type Result struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Type   string `json:"type"`
	Method string `json:"method"`
	Error  string `json:"error,omitempty"`
}

type StatsFile struct {
	FilesByType map[string]int64
	BytesByType map[string]int64
	Errors      int64
}
