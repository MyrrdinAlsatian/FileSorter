// mover/plan.go - Génération du plan de déplacement à partir du JSONL
//
// Ce fichier contient les fonctions pour :
// - Charger les fichiers depuis un JSONL
// - Appliquer les filtres (type, taille, corrupted)
// - Générer les chemins de destination
// - Détecter les conflits
package mover

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"FileRecoveryOrganizer/types"
)

// ═══════════════════════════════════════════════════════════════════════════
// GÉNÉRATION DU PLAN
// ═══════════════════════════════════════════════════════════════════════════

// GeneratePlanFromJSONL génère un plan de déplacement à partir d'un fichier JSONL.
//
// CONCEPT : PIPELINE DE TRAITEMENT
// ================================
// Le traitement se fait en plusieurs étapes :
// 1. Lecture du JSONL
// 2. Filtrage (type, taille, corrupted)
// 3. Génération du chemin de destination
// 4. Détection des conflits
// 5. Ajout au plan
func GeneratePlanFromJSONL(jsonlPath string, opts Options) (*Plan, error) {
	plan := NewPlan(opts)

	// Ouvrir le fichier JSONL
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open JSONL file: %w", err)
	}
	defer file.Close()

	// Map pour détecter les conflits de destination
	// clé = chemin destination, valeur = chemin source
	destMap := make(map[string]string)

	// Lire ligne par ligne
	scanner := bufio.NewScanner(file)

	// Augmenter la taille du buffer pour les longues lignes
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Ignorer les lignes vides
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parser le JSON
		var result types.Result
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			// Log l'erreur mais continue
			if opts.Verbose {
				fmt.Printf("⚠️  Ligne %d: JSON invalide: %v\n", lineNum, err)
			}
			continue
		}

		// Appliquer les filtres
		if !shouldInclude(result, opts) {
			plan.SkippedFiles++
			continue
		}

		// Générer le chemin de destination
		destPath := generateDestinationPath(result, opts.Destination)

		// Vérifier les conflits
		if existingSource, exists := destMap[destPath]; exists {
			// Conflit détecté
			plan.ConflictFiles++

			if opts.OverwriteMode == "rename" {
				// Renommer pour éviter le conflit
				destPath = resolveConflict(destPath, destMap)
			} else if opts.OverwriteMode == "skip" {
				// Ignorer ce fichier
				if opts.Verbose {
					fmt.Printf("⚠️  Conflit ignoré: %s -> %s (déjà utilisé par %s)\n",
						result.Path, destPath, existingSource)
				}
				plan.SkippedFiles++
				continue
			}
			// "overwrite" : on garde le chemin (écrasera l'existant)
		}

		// Enregistrer le mapping
		destMap[destPath] = result.Path

		// Créer l'opération
		op := Operation{
			Source:      result.Path,
			Destination: destPath,
			Size:        result.Size,
			Type:        result.Type,
			Category:    extractCategory(result.TargetPath),
		}

		// Ajouter le hash si disponible (pour vérification)
		if result.FullHash != "" {
			op.Hash = result.FullHash
		} else if result.QuickHash != "" {
			op.Hash = result.QuickHash
		}

		plan.AddOperation(op)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading JSONL: %w", err)
	}

	// Calculer le nombre de répertoires utilisés
	dirs := make(map[string]bool)
	for _, op := range plan.Operations {
		dirs[filepath.Dir(op.Destination)] = true
	}
	plan.DirectoriesUsed = len(dirs)

	return plan, nil
}

// ═══════════════════════════════════════════════════════════════════════════
// FILTRAGE
// ═══════════════════════════════════════════════════════════════════════════

// shouldInclude détermine si un fichier doit être inclus dans le plan.
func shouldInclude(result types.Result, opts Options) bool {
	// Filtre : fichiers corrompus
	if opts.SkipCorrupted && result.Valid != nil && !*result.Valid {
		return false
	}

	// Filtre : taille minimale
	if opts.SkipSmall > 0 && result.Size < opts.SkipSmall {
		return false
	}

	// Filtre : types exclus
	if len(opts.ExcludeTypes) > 0 {
		for _, t := range opts.ExcludeTypes {
			if strings.EqualFold(result.Type, t) {
				return false
			}
		}
	}

	// Filtre : types inclus uniquement
	if len(opts.OnlyTypes) > 0 {
		found := false
		for _, t := range opts.OnlyTypes {
			if strings.EqualFold(result.Type, t) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// ═══════════════════════════════════════════════════════════════════════════
// GÉNÉRATION DES CHEMINS
// ═══════════════════════════════════════════════════════════════════════════

// generateDestinationPath génère le chemin de destination pour un fichier.
//
// La structure de destination est basée sur le TargetPath calculé lors du scan.
// Exemple : images/photos/2024/01/IMG_1234.jpg
func generateDestinationPath(result types.Result, destRoot string) string {
	// Si TargetPath existe, l'utiliser
	if result.TargetPath != "" {
		return filepath.Join(destRoot, result.TargetPath)
	}

	// Sinon, construire un chemin basique basé sur le type
	category := categorizeByType(result.Type)
	filename := filepath.Base(result.Path)

	return filepath.Join(destRoot, category, filename)
}

// categorizeByType retourne une catégorie basique selon le type de fichier.
func categorizeByType(fileType string) string {
	switch strings.ToLower(fileType) {
	// Images
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff", "heic", "heif":
		return "images"

	// Vidéos
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpeg", "mpg":
		return "videos"

	// Audio
	case "mp3", "flac", "wav", "ogg", "m4a", "aac", "wma", "opus":
		return "audio"

	// Documents
	case "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "rtf", "odt":
		return "documents"

	// Archives
	case "zip", "rar", "7z", "tar", "gz", "bz2":
		return "archives"

	default:
		return "other"
	}
}

// extractCategory extrait la catégorie principale du TargetPath.
// Ex: "images/photos/2024/01/file.jpg" -> "images/photos"
func extractCategory(targetPath string) string {
	if targetPath == "" {
		return "unknown"
	}

	// Normaliser les séparateurs
	targetPath = filepath.ToSlash(targetPath)

	// Prendre les deux premiers segments
	parts := strings.Split(targetPath, "/")
	if len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "unknown"
}

// ═══════════════════════════════════════════════════════════════════════════
// RÉSOLUTION DE CONFLITS
// ═══════════════════════════════════════════════════════════════════════════

// resolveConflict génère un nouveau nom pour éviter un conflit.
//
// Stratégie : ajouter un suffixe numérique avant l'extension.
// Ex: photo.jpg -> photo_001.jpg -> photo_002.jpg
func resolveConflict(destPath string, existingDests map[string]string) string {
	dir := filepath.Dir(destPath)
	ext := filepath.Ext(destPath)
	base := strings.TrimSuffix(filepath.Base(destPath), ext)

	// Essayer des suffixes numériques
	for i := 1; i < 10000; i++ {
		newName := fmt.Sprintf("%s_%03d%s", base, i, ext)
		newPath := filepath.Join(dir, newName)

		if _, exists := existingDests[newPath]; !exists {
			return newPath
		}
	}

	// Fallback : utiliser un UUID-like
	return filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, os.Getpid(), ext))
}

// ═══════════════════════════════════════════════════════════════════════════
// PRÉVISUALISATION DU PLAN
// ═══════════════════════════════════════════════════════════════════════════

// PrintSummary affiche un résumé du plan.
func (p *Plan) PrintSummary() {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    PLAN DE DÉPLACEMENT                            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  📁 Fichiers à traiter : %d\n", p.TotalFiles)
	fmt.Printf("  💾 Taille totale      : %s\n", formatBytes(p.TotalSize))
	fmt.Printf("  📂 Répertoires        : %d\n", p.DirectoriesUsed)
	fmt.Printf("  ⏭️  Fichiers ignorés   : %d\n", p.SkippedFiles)
	fmt.Printf("  ⚠️  Conflits           : %d\n", p.ConflictFiles)
	fmt.Println()
	fmt.Printf("  Mode       : %s\n", p.Options.Mode)
	fmt.Printf("  Destination: %s\n", p.Options.Destination)
	fmt.Printf("  Vérifier   : %v\n", p.Options.Verify)
	fmt.Println()
}

// PrintPreview affiche un aperçu des premières opérations.
func (p *Plan) PrintPreview(maxItems int) {
	if maxItems <= 0 {
		maxItems = 10
	}

	fmt.Println("📋 Aperçu des opérations:")
	fmt.Println()

	count := 0
	for _, op := range p.Operations {
		if count >= maxItems {
			remaining := len(p.Operations) - maxItems
			fmt.Printf("   ... et %d autres opérations\n", remaining)
			break
		}

		// Tronquer les chemins longs
		src := truncatePath(op.Source, 40)
		dst := truncatePath(op.Destination, 40)

		fmt.Printf("   %s\n", src)
		fmt.Printf("   └─▶ %s\n", dst)
		fmt.Println()

		count++
	}
}

// truncatePath tronque un chemin s'il est trop long.
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Garder le début et la fin
	start := maxLen/2 - 2
	end := maxLen/2 - 1
	return path[:start] + "..." + path[len(path)-end:]
}

// ═══════════════════════════════════════════════════════════════════════════
// STATISTIQUES PAR CATÉGORIE
// ═══════════════════════════════════════════════════════════════════════════

// GetStatsByCategory retourne les statistiques groupées par catégorie.
func (p *Plan) GetStatsByCategory() map[string]CategoryStats {
	stats := make(map[string]CategoryStats)

	for _, op := range p.Operations {
		cat := op.Category
		if cat == "" {
			cat = "unknown"
		}

		s := stats[cat]
		s.Count++
		s.TotalSize += op.Size
		stats[cat] = s
	}

	return stats
}

// CategoryStats contient les statistiques pour une catégorie.
type CategoryStats struct {
	Count     int
	TotalSize int64
}

// PrintStatsByCategory affiche les statistiques par catégorie.
func (p *Plan) PrintStatsByCategory() {
	stats := p.GetStatsByCategory()

	fmt.Println("📊 Répartition par catégorie:")
	fmt.Println()

	for cat, s := range stats {
		fmt.Printf("   %-20s %6d fichiers  %10s\n", cat, s.Count, formatBytes(s.TotalSize))
	}
	fmt.Println()
}
