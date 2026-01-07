package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
		info, err := d.Info()
		if err != nil {
			return nil
		}
		stats.TotalSize += info.Size()
		stats.TotalFiles++

		return nil
	})
}

func ScanDirectoryParallel(sourceDir string, stats *types.Stats, barUpdate func(), worker int) error {

	fileCh := make(chan string, 100) // jusqu'a 100 fichiers en attente

	var wg sync.WaitGroup // pour attendre la fin des goroutines

	var mu sync.Mutex // pour protéger l'accès à stats

	// Lancer les workers

	for i := 0; i < worker; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileCh {
				info, err := os.Stat(path)
				if err != nil || info.IsDir() {
					continue
				}
				fileType := detector.Detect(path)
				mu.Lock()
				stats.DetectedFileType[fileType]++
				mu.Unlock()
				if barUpdate != nil {
					barUpdate()
				}
			}
		}()
	}

	// Parcourir le répertoire source et envoyer les chemins de fichiers au canal
	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		fileCh <- path
		return nil
	})

	close(fileCh) // fermer le canal après avoir envoyé tous les fichiers
	wg.Wait()     // attendre que tous les workers aient terminé

	return err
}
