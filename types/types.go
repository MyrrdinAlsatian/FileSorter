package types

type Stats struct {
	TotalFiles int
	totalDirs  int

	extensionCount map[string]int
	detectFileType map[string]int
}
