package types

type Stats struct {
	TotalFiles int
	TotalDirs  int
	TotalSize  int64

	DetectedFileType map[string]int
}
