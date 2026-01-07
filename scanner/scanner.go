package scanner

import (
	"fmt"
	"os"
	"path/filepath"

	"FileRecoveryOrganizer/detector"
	"FileRecoveryOrganizer/types"
)

func ScanDirectory(sourceDir string, stats *types.Stats, barUpdate func()) error {

	filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {

		if err != nil {
			fmt.Printf("Erreur d'accès à %q: %v\n", path, err)
			return nil
		}

		if d.IsDir() {
			return nil
		}

		fileType := detector.Detect(path)

		stats.DetectedFileType[fileType]++

		if barUpdate != nil {
			barUpdate()
		}
		return nil

	})

	return nil
}

func CountFile(sourceDir string, stats *types.Stats) error {
	return filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			stats.TotalDirs++
			return nil
		}
		stats.TotalFiles++
		return nil
	})
}
