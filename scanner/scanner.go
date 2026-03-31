// Package scanner gère le parcours des répertoires et le traitement des fichiers.
//
// Ce package est responsable de :
// - Parcourir les répertoires source
// - Détecter les types de fichiers
// - Enrichir les données des fichiers avec les métadonnées
// - Classer les fichiers selon leur type
// - Calculer les hash pour la détection de doublons (optionnel)
//
// Les fichiers sont traités de manière parallèle en utilisant des goroutines
// pour améliorer les performances lors du traitement d'un grand nombre de fichiers.
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"FileRecoveryOrganizer/classifier"
	"FileRecoveryOrganizer/dedup"
	"FileRecoveryOrganizer/detector"
	"FileRecoveryOrganizer/enricher"
	"FileRecoveryOrganizer/metadata"
	"FileRecoveryOrganizer/types"
	"FileRecoveryOrganizer/validator"
)

// Result est un alias vers types.Result pour simplifier les imports
type Result = types.Result

// SkipChecker est une interface pour vérifier si un fichier doit être ignoré.
// Utilisé pour le mode resume.
type SkipChecker interface {
	IsProcessed(path string) bool
}

// ScanOptions contient toutes les options pour le scan parallèle.
type ScanOptions struct {
	ClassifyOpts  classifier.ClassifyOptions // Options de classification
	ComputeHash   bool                       // Calculer les hash pour détecter les doublons
	SkipChecker   SkipChecker                // Vérificateur de fichiers à ignorer (nil = aucun)
	Validate      bool                       // Valider l'intégrité des fichiers
	ValidateTypes string                     // Types à valider (image, video, audio, all)
}

// DefaultScanOptions retourne les options de scan par défaut.
func DefaultScanOptions() ScanOptions {
	return ScanOptions{
		ClassifyOpts:  classifier.DefaultClassifyOptions(),
		ComputeHash:   false,
		SkipChecker:   nil,
		Validate:      false,
		ValidateTypes: "all",
	}
}

// ScanDirectory parcourt un répertoire de manière synchrone (bloquante).
//
// Cette fonction utilise filepath.WalkDir pour parcourir tous les fichiers
// du répertoire source. Elle est simple mais plus lente que ScanDirectoryParallel
// car elle traite les fichiers un par un.
//
// Paramètres :
//   - sourceDir : le chemin du répertoire à parcourir
//   - stats : pointeur vers la structure Statistics pour accumuler les statistiques
//   - barUpdate : fonction de rappel appelée après chaque fichier traité
//
// Note : Cette fonction n'est pas utilisée dans la version parallèle.
// Voir ScanDirectoryParallel pour une version plus performante.
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

// CountFile compte tous les fichiers et calcule la taille totale d'un répertoire.
//
// Cette fonction parcourt le répertoire source et accumule :
// - Le nombre total de fichiers
// - Le nombre total de répertoires
// - La taille totale en octets
//
// Elle est généralement appelée avant ScanDirectoryParallel pour obtenir
// une indication du nombre de fichiers à traiter.
//
// Paramètres :
//   - sourceDir : le chemin du répertoire à analyser
//   - stats : pointeur vers la structure Statistics à mettre à jour
//
// Retour :
//   - error : une erreur en cas de problème lors du parcours
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

// ScanDirectoryParallel parcourt un répertoire en utilisant des goroutines pour le traitement parallèle.
//
// CONCEPTS CLE :
// - Goroutines : fonction légère exécutée en parallèle (équivalent Go des threads)
// - Canaux (channels) : permettent la communication sécurisée entre goroutines
// - WaitGroup : synchronise l'attente de plusieurs goroutines
// - Mutex : assure que seule une goroutine accède aux données à la fois
//
// FONCTIONNEMENT :
// 1. Lance plusieurs workers (goroutines) qui attendent des chemins de fichiers
// 2. Utilise un canal pour envoyer les chemins aux workers
// 3. Chaque worker traite son fichier indépendamment
// 4. Les résultats sont envoyés dans le canal Results
//
// AVANTAGES :
// - Traitement simultané de plusieurs fichiers
// - Meilleur usage des CPU multi-cœurs
// - Performances améliorées sur de gros volumes
//
// Paramètres :
//   - sourceDir : chemin du répertoire à parcourir
//   - collector : collecteur de résultats avec un canal Results
//   - stats : statistiques thread-safe
//   - barUpdate : fonction de callback pour mettre à jour la progression
//   - worker : nombre de goroutines workers à lancer
//
// Retour :
//   - error : erreur lors du parcours du répertoire
func ScanDirectoryParallel(sourceDir string, collector *Collector, stats *SafeStats, barUpdate func(), worker int) error {
	// Utilise les options par défaut (sans organisation par date ni hash)
	return ScanDirectoryParallelWithScanOptions(sourceDir, collector, stats, barUpdate, worker, DefaultScanOptions())
}

// ScanDirectoryParallelWithOptions est comme ScanDirectoryParallel mais avec des options de classification.
//
// Cette variante permet de configurer l'organisation par date lors de la classification.
// Utilisez classifier.ClassifyOptions pour définir le format de date souhaité.
//
// Paramètres supplémentaires :
//   - classifyOpts : options de classification (organisation par date, etc.)
func ScanDirectoryParallelWithOptions(sourceDir string, collector *Collector, stats *SafeStats, barUpdate func(), worker int, classifyOpts classifier.ClassifyOptions) error {
	opts := ScanOptions{
		ClassifyOpts: classifyOpts,
		ComputeHash:  false,
	}
	return ScanDirectoryParallelWithScanOptions(sourceDir, collector, stats, barUpdate, worker, opts)
}

// ScanDirectoryParallelWithScanOptions est la version la plus complète du scanner parallèle.
//
// Cette variante accepte toutes les options possibles incluant :
// - Classification avec organisation par date
// - Calcul de hash pour détection de doublons
//
// Paramètres supplémentaires :
//   - opts : structure ScanOptions contenant toutes les options
func ScanDirectoryParallelWithScanOptions(sourceDir string, collector *Collector, stats *SafeStats, barUpdate func(), worker int, opts ScanOptions) error {

	// Canal pour envoyer les chemins de fichiers aux workers
	// Buffer de 100 permet à plusieurs fichiers d'être en attente
	fileCh := make(chan string, 100)

	// WaitGroup pour synchroniser l'attente des workers
	// sync.WaitGroup compte les goroutines actives et attend leur fin
	var wg sync.WaitGroup

	// Lance le nombre de workers demandés
	for i := 0; i < worker; i++ {
		wg.Add(1) // Ajoute 1 à la liste d'attente

		// go : mot-clé pour créer une goroutine
		// La goroutine exécutera la fonction de manière asynchrone
		go func() {
			defer wg.Done() // Marque cette goroutine comme terminée

			// Boucle qui reçoit les chemins du canal fileCh
			// range sur un canal recevra les valeurs jusqu'à sa fermeture
			for path := range fileCh {

				// Récupère les informations du fichier
				info, err := os.Stat(path)
				if err != nil || info.IsDir() {
					continue // Saute ce fichier s'il y a erreur ou si c'est un répertoire
				}

				// Détecte le type de fichier (jpg, pdf, mp4, etc.)
				fileType := detector.Detect(path)

				// Ajoute une statistique de manière thread-safe
				stats.AddFile(fileType, info.Size(), err != nil)

				// Crée un nouveau résultat pour ce fichier
				result := types.Result{
					Path:           path,
					Size:           info.Size(),
					Type:           fileType,
					AdditionalInfo: &metadata.FileData{}, // Initialise un pointeur vers une structure FileData vide
				}

				// Enrichir les images avec les métadonnées EXIF
				if metadata.IsImageType(fileType) {
					enricher.EnrichImage(&result)
					detector.DetectAssetImg(path, info.Size(), result.Image)
					detector.DetectThumbnail(path, info.Size(), result.Image)
					fileDate := metadata.BestDate(path, true)
					result.AdditionalInfo = &fileDate
				} else {
					// Pour les autres types (audio, vidéo, etc.)
					meta := metadata.GetFileMeta(path, fileType)
					if meta.OriginalName != "" {
						result.AdditionalInfo.OriginalName = meta.OriginalName
						result.AdditionalInfo.Source = meta.Source
					}
					if meta.Valid {
						result.AdditionalInfo.Time = meta.Time
						result.AdditionalInfo.Source = meta.Source
						result.AdditionalInfo.Valid = true
					}
				}

				// Calcul du hash pour la détection de doublons (si activé)
				// Le quick hash est rapide et suffisant pour la plupart des cas
				if opts.ComputeHash {
					if hash, err := dedup.ComputeQuickHash(path, info.Size()); err == nil {
						result.QuickHash = hash
					}
				}

				// Validation d'intégrité des fichiers (si activée)
				if opts.Validate && shouldValidate(fileType, opts.ValidateTypes) {
					validResult := validator.ValidateFile(path)
					result.Valid = &validResult.Valid
					if !validResult.Valid && validResult.Error != nil {
						result.ValidationError = validResult.Error.Error()
					}
					if validResult.Details != nil && len(validResult.Details) > 0 {
						result.ValidDetails = validResult.Details
					}
				}

				// Classe le fichier dans une catégorie (avec options de date)
				classifier.ClassifyWithOptions(&result, opts.ClassifyOpts)

				// Envoie le résultat dans le canal Results
				collector.Results <- result

				// Appelle le callback pour mettre à jour la barre de progression
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

		// Mode resume : vérifier si le fichier a déjà été traité
		if opts.SkipChecker != nil && opts.SkipChecker.IsProcessed(path) {
			// Fichier déjà traité, mettre à jour la barre mais ne pas retraiter
			if barUpdate != nil {
				barUpdate()
			}
			return nil
		}

		// Envoie le chemin au canal (sera reçu par un worker)
		fileCh <- path
		return nil
	})

	close(fileCh)            // Ferme le canal - cela signale aux workers qu'il n'y a plus de fichiers
	wg.Wait()                // Attend que tous les workers aient terminé
	close(collector.Results) // Ferme le canal des résultats
	return err
}

// shouldValidate détermine si un fichier doit être validé selon son type.
//
// validateTypes peut être :
// - "all" : valider tous les types supportés
// - "image" : valider uniquement les images
// - "video" : valider uniquement les vidéos
// - "audio" : valider uniquement les fichiers audio
// - combinaisons séparées par des virgules : "image,video"
func shouldValidate(fileType string, validateTypes string) bool {
	if validateTypes == "all" {
		return true
	}

	// Déterminer la catégorie du fichier
	var category string
	switch {
	case metadata.IsImageType(fileType):
		category = "image"
	case metadata.IsVideoType(fileType):
		category = "video"
	case metadata.IsAudioType(fileType):
		category = "audio"
	default:
		return false // Type non supporté pour la validation
	}

	// Vérifier si la catégorie est dans la liste
	types := strings.Split(strings.ToLower(validateTypes), ",")
	for _, t := range types {
		if strings.TrimSpace(t) == category {
			return true
		}
	}
	return false
}
