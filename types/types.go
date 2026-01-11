package types

type Stats struct {
	TotalFiles int
	TotalDirs  int
	TotalSize  int64

	DetectedFileType map[string]int
}

type Result struct {
	Path   string
	Size   int64
	Type   string
	Method string
	Error  string
}

type StatsFile struct {
	FilesByType map[string]int64
	BytesByType map[string]int64
	Errors      int64
}
