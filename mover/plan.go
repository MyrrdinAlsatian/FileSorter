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
	"context"
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
	return GeneratePlanFromJSONLContext(context.Background(), jsonlPath, opts)
}

// GeneratePlanFromJSONLContext génère un plan et vérifie l'annulation entre les lignes JSONL.
func GeneratePlanFromJSONLContext(ctx context.Context, jsonlPath string, opts Options) (*Plan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sourceRoot, destinationRoot, err := validatePlanRoots(opts)
	if err != nil {
		return nil, err
	}
	opts.Source = sourceRoot
	opts.Destination = destinationRoot
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
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

		// Vérifier les chemins issus du JSONL avant de les ajouter au plan.
		destPath, err := validateResultPaths(result, sourceRoot, destinationRoot)
		if err != nil {
			return nil, fmt.Errorf("invalid paths on JSONL line %d: %w", lineNum, err)
		}

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

		// Seul le SHA-256 complet permet de vérifier l'intégrité d'une copie.
		op.Hash = result.FullHash

		plan.AddOperation(op)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading JSONL: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Calculer le nombre de répertoires utilisés
	dirs := make(map[string]bool)
	for _, op := range plan.Operations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dirs[filepath.Dir(op.Destination)] = true
	}
	plan.DirectoriesUsed = len(dirs)

	return plan, nil
}

// validatePlanRoots normalise les racines et interdit une destination située dans la source.
func validatePlanRoots(opts Options) (string, string, error) {
	if strings.TrimSpace(opts.Source) == "" {
		return "", "", fmt.Errorf("source directory is required")
	}
	if strings.TrimSpace(opts.Destination) == "" {
		return "", "", fmt.Errorf("destination directory is required")
	}

	sourceRoot, err := filepath.Abs(opts.Source)
	if err != nil {
		return "", "", fmt.Errorf("resolve source directory: %w", err)
	}
	destinationRoot, err := filepath.Abs(opts.Destination)
	if err != nil {
		return "", "", fmt.Errorf("resolve destination directory: %w", err)
	}

	// Refuser une racine source symbolique rend explicite que le plan ne suit pas les symlinks.
	sourceInfo, err := os.Lstat(sourceRoot)
	if err != nil {
		return "", "", fmt.Errorf("inspect source directory: %w", err)
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", fmt.Errorf("source directory must not be a symbolic link")
	}
	if !sourceInfo.IsDir() {
		return "", "", fmt.Errorf("source path is not a directory")
	}

	if info, err := os.Stat(destinationRoot); err == nil && !info.IsDir() {
		return "", "", fmt.Errorf("destination path is not a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("inspect destination directory: %w", err)
	}

	// Abs donne une base commune aux comparaisons; EvalSymlinks compare ensuite l'emplacement réel.
	resolvedSource, err := resolveExistingPath(sourceRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve source directory links: %w", err)
	}
	resolvedDestination, err := resolveExistingPath(destinationRoot)
	if err != nil {
		return "", "", fmt.Errorf("resolve destination directory links: %w", err)
	}
	insideSource, err := pathWithinRoot(resolvedSource, resolvedDestination)
	if err != nil {
		return "", "", fmt.Errorf("compare source and destination: %w", err)
	}
	if insideSource {
		return "", "", fmt.Errorf("destination directory must not be the source directory or a directory inside it")
	}

	return filepath.Clean(sourceRoot), filepath.Clean(destinationRoot), nil
}

// validateResultPaths vérifie que la source et la destination de l'enregistrement restent dans leurs racines.
func validateResultPaths(result types.Result, sourceRoot, destinationRoot string) (string, error) {
	if strings.TrimSpace(result.Path) == "" {
		return "", fmt.Errorf("source file path is empty")
	}
	if filepath.IsAbs(result.TargetPath) || filepath.VolumeName(result.TargetPath) != "" {
		return "", fmt.Errorf("target path must be relative")
	}

	sourcePath, err := filepath.Abs(result.Path)
	if err != nil {
		return "", fmt.Errorf("resolve source file path: %w", err)
	}
	insideSource, err := pathWithinRoot(sourceRoot, sourcePath)
	if err != nil {
		return "", fmt.Errorf("compare source file with source directory: %w", err)
	}
	if !insideSource {
		return "", fmt.Errorf("source file is outside the source directory")
	}
	if hasSymlink, err := hasSymlinkComponent(sourceRoot, sourcePath); err != nil {
		return "", fmt.Errorf("inspect source file path: %w", err)
	} else if hasSymlink {
		return "", fmt.Errorf("source file path must not pass through a symbolic link")
	}

	destinationPath, err := filepath.Abs(generateDestinationPath(result, destinationRoot))
	if err != nil {
		return "", fmt.Errorf("resolve destination file path: %w", err)
	}
	insideDestination, err := pathWithinRoot(destinationRoot, destinationPath)
	if err != nil {
		return "", fmt.Errorf("compare destination file with destination directory: %w", err)
	}
	if !insideDestination {
		return "", fmt.Errorf("target path escapes the destination directory")
	}

	// Un répertoire symlinké dans la destination ne doit pas rediriger l'écriture hors de sa racine.
	resolvedRoot, err := resolveExistingPath(destinationRoot)
	if err != nil {
		return "", fmt.Errorf("resolve destination directory links: %w", err)
	}
	resolvedPath, err := resolveExistingPath(destinationPath)
	if err != nil {
		return "", fmt.Errorf("resolve destination file links: %w", err)
	}
	insideResolvedDestination, err := pathWithinRoot(resolvedRoot, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("compare resolved destination paths: %w", err)
	}
	if !insideResolvedDestination {
		return "", fmt.Errorf("target path escapes the destination directory through a symbolic link")
	}
	resolvedSourceRoot, err := resolveExistingPath(sourceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve source directory links: %w", err)
	}
	insideResolvedSource, err := pathWithinRoot(resolvedSourceRoot, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("compare destination with source directory: %w", err)
	}
	if insideResolvedSource {
		return "", fmt.Errorf("target path resolves inside the source directory")
	}

	return destinationPath, nil
}

// pathWithinRoot compare des chemins absolus avec filepath.Rel, sans confondre ".." et "..notes".
func pathWithinRoot(root, candidate string) (bool, error) {
	if !strings.EqualFold(filepath.VolumeName(root), filepath.VolumeName(candidate)) {
		return false, nil
	}
	// Rel exprime candidate par rapport à root : un résultat ".." indique une sortie de la racine.
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)), nil
}

// resolveExistingPath résout les liens existants même si les derniers éléments n'existent pas encore.
func resolveExistingPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	current := absolute
	var missingParts []string
	for {
		// EvalSymlinks exige un chemin existant; on remonte donc jusqu'au premier parent présent.
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			// Les éléments ont été collectés en remontant; on les réassemble du parent vers l'enfant.
			for i := len(missingParts) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, missingParts[i])
			}
			return filepath.Clean(resolved), nil
		} else if !os.IsNotExist(err) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing parent directory for %q", path)
		}
		missingParts = append(missingParts, filepath.Base(current))
		current = parent
	}
}

// hasSymlinkComponent recherche un lien symbolique entre la racine et le chemin inclus.
func hasSymlinkComponent(root, candidate string) (bool, error) {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	if relative == "." {
		return false, nil
	}
	current := root
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		// Lstat inspecte le lien lui-même; Stat suivrait le lien et cacherait sa présence.
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
	}
	return false, nil
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
