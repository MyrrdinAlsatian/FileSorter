package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"FileRecoveryOrganizer/classifier"
	"FileRecoveryOrganizer/detector"
	"FileRecoveryOrganizer/enricher"
	"FileRecoveryOrganizer/types"
)

type Result = types.Result

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

func ScanDirectoryParallel(sourceDir string, collector *Collector, stats *SafeStats, barUpdate func(), worker int) error {

	fileCh := make(chan string, 100) // jusqu'a 100 fichiers en attente

	var wg sync.WaitGroup // pour attendre la fin des goroutines

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

				stats.AddFile(fileType, info.Size(), err != nil)

				result := types.Result{
					Path: path,
					Size: info.Size(),
					Type: fileType,
				}
				// Enrichir les images avec les métadonnées EXIF
				if fileType == "jpg" || fileType == "jpeg" || fileType == "png" || fileType == "tiff" || fileType == "heic" {
					enricher.EnrichImage(&result)
					detector.DetectAssetImg(path, info.Size(), result.Image)
					detector.DetectThumbnail(path, info.Size(), result.Image)
				}
				classifier.Classify(&result)
				collector.Results <- result

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

	close(fileCh)            // fermer le canal après avoir envoyé tous les fichiers
	wg.Wait()                // attendre que tous les workers aient terminé
	close(collector.Results) // fermer le canal des résultats
	return err
}
