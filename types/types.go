package types

type Stats struct {
	TotalFiles int
	TotalDirs  int

	ExtensionCount   map[string]int
	DetectedFileType map[string]int
}
