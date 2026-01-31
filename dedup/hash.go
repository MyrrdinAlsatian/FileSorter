// Package dedup gère la détection de doublons via calcul de hash.
//
// CONCEPT : HASH CRYPTOGRAPHIQUE POUR IDENTIFIER LES FICHIERS
// ===========================================================
// Un hash (empreinte) est une valeur calculée à partir du contenu d'un fichier.
// Deux fichiers avec le même contenu auront TOUJOURS le même hash.
// Deux fichiers différents auront (presque) TOUJOURS des hash différents.
//
// ALGORITHMES :
// - MD5 : rapide, 128 bits, mais vulnérable aux collisions (suffisant pour dédup)
// - SHA256 : plus lent, 256 bits, très sûr (recommandé)
//
// STRATÉGIE D'OPTIMISATION :
// Pour les gros fichiers, calculer le hash complet est lent.
// On utilise une approche en 2 passes :
// 1. "Quick hash" : hash des premiers et derniers 64KB
// 2. "Full hash" : hash complet seulement si quick hash identique
//
// Cela permet d'éliminer rapidement les fichiers différents.
package dedup

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/schollz/progressbar/v3"
)

// ═══════════════════════════════════════════════════════════════════════════
// CONSTANTES
// ═══════════════════════════════════════════════════════════════════════════

const (
	// QuickHashSize est la taille lue au début et à la fin pour le hash rapide
	QuickHashSize = 64 * 1024 // 64 KB

	// SmallFileThreshold : en dessous de cette taille, on fait le hash complet directement
	SmallFileThreshold = 256 * 1024 // 256 KB
)

// ═══════════════════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════════════════

// FileHash contient les informations de hash d'un fichier.
type FileHash struct {
	QuickHash string `json:"quick_hash,omitempty"` // Hash rapide (début + fin)
	FullHash  string `json:"full_hash,omitempty"`  // Hash complet
	Size      int64  `json:"size"`                 // Taille du fichier
}

// DuplicateGroup représente un groupe de fichiers identiques.
type DuplicateGroup struct {
	Hash  string   `json:"hash"`         // Hash commun
	Size  int64    `json:"size"`         // Taille de chaque fichier
	Count int      `json:"count"`        // Nombre de fichiers
	Paths []string `json:"paths"`        // Chemins des fichiers
	Waste int64    `json:"wasted_space"` // Espace gaspillé (Size * (Count-1))
}

// DuplicateReport est le rapport complet de détection de doublons.
type DuplicateReport struct {
	TotalFiles      int              `json:"total_files"`
	UniqueFiles     int              `json:"unique_files"`
	DuplicateFiles  int              `json:"duplicate_files"`
	DuplicateGroups int              `json:"duplicate_groups"`
	WastedSpace     int64            `json:"wasted_space_bytes"`
	Groups          []DuplicateGroup `json:"groups,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS DE HASH
// ═══════════════════════════════════════════════════════════════════════════

// ComputeQuickHash calcule un hash rapide basé sur le début et la fin du fichier.
//
// ALGORITHME :
// 1. Lire les premiers 64KB
// 2. Lire les derniers 64KB
// 3. Hasher les deux blocs ensemble
//
// Pour les petits fichiers (< 256KB), on hash le contenu complet.
//
// Paramètres :
//   - path : chemin du fichier
//   - size : taille du fichier (pour optimisation)
//
// Retour :
//   - string : hash en hexadécimal
//   - error : erreur si le fichier n'est pas lisible
func ComputeQuickHash(path string, size int64) (string, error) {
	// Petits fichiers : hash complet directement
	if size <= SmallFileThreshold {
		return ComputeFullHash(path)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()

	// Lire le début
	buf := make([]byte, QuickHashSize)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	hasher.Write(buf[:n])

	// Lire la fin
	if size > QuickHashSize*2 {
		_, err = f.Seek(-QuickHashSize, io.SeekEnd)
		if err != nil {
			return "", err
		}
		n, err = f.Read(buf)
		if err != nil && err != io.EOF {
			return "", err
		}
		hasher.Write(buf[:n])
	}

	// Inclure la taille dans le hash pour différencier les fichiers
	// qui auraient le même début/fin mais des tailles différentes
	sizeBytes := []byte{
		byte(size >> 56), byte(size >> 48), byte(size >> 40), byte(size >> 32),
		byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size),
	}
	hasher.Write(sizeBytes)

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeFullHash calcule le hash SHA256 complet d'un fichier.
//
// Cette fonction lit le fichier entier par blocs de 32KB.
// Elle est plus lente que QuickHash mais garantit l'unicité.
//
// Paramètres :
//   - path : chemin du fichier
//
// Retour :
//   - string : hash SHA256 en hexadécimal (64 caractères)
//   - error : erreur si le fichier n'est pas lisible
func ComputeFullHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()

	// Lire par blocs de 32KB pour économiser la mémoire
	buf := make([]byte, 32*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ═══════════════════════════════════════════════════════════════════════════
// DÉTECTION DE DOUBLONS
// ═══════════════════════════════════════════════════════════════════════════

// DuplicateFinder détecte les doublons dans une liste de fichiers.
//
// CONCEPT GO : STRUCTURE AVEC MUTEX
// =================================
// Cette structure utilise un sync.Mutex pour protéger l'accès concurrent
// aux maps internes. Cela permet d'appeler AddFile depuis plusieurs goroutines.
type DuplicateFinder struct {
	mu sync.Mutex

	// Première passe : grouper par taille (les doublons ont forcément la même taille)
	bySize map[int64][]string

	// Deuxième passe : grouper par quick hash
	byQuickHash map[string][]string

	// Troisième passe : grouper par full hash (confirmation finale)
	byFullHash map[string][]string
}

// NewDuplicateFinder crée un nouveau détecteur de doublons.
func NewDuplicateFinder() *DuplicateFinder {
	return &DuplicateFinder{
		bySize:      make(map[int64][]string),
		byQuickHash: make(map[string][]string),
		byFullHash:  make(map[string][]string),
	}
}

// AddFile ajoute un fichier au détecteur.
//
// Cette fonction est thread-safe et peut être appelée depuis plusieurs goroutines.
func (df *DuplicateFinder) AddFile(path string, size int64) {
	df.mu.Lock()
	defer df.mu.Unlock()

	df.bySize[size] = append(df.bySize[size], path)
}

// FindDuplicates analyse les fichiers et retourne le rapport de doublons.
//
// ALGORITHME EN 3 PASSES :
// 1. Filtrer par taille : garder seulement les tailles avec 2+ fichiers
// 2. Pour ces fichiers, calculer le quick hash et grouper
// 3. Pour les quick hash avec 2+ fichiers, calculer le full hash
//
// Cette approche évite de hasher les fichiers uniques (gain de temps énorme).
func (df *DuplicateFinder) FindDuplicates() *DuplicateReport {
	report := &DuplicateReport{}

	// Compter le total de fichiers et ceux avec taille unique
	var uniqueSizeCount int
	for _, paths := range df.bySize {
		report.TotalFiles += len(paths)
		if len(paths) == 1 {
			uniqueSizeCount++
		}
	}

	// Passe 1 : Filtrer les tailles avec potentiels doublons
	fmt.Println("   📋 Passe 1/3: Filtrage par taille...")
	var candidates []struct {
		size  int64
		paths []string
	}

	var candidateCount int
	for size, paths := range df.bySize {
		if len(paths) > 1 {
			candidates = append(candidates, struct {
				size  int64
				paths []string
			}{size, paths})
			candidateCount += len(paths)
		}
	}

	// Afficher les statistiques de filtrage
	fmt.Printf("   ✓ %d fichiers avec taille unique → ignorés (pas de doublon possible)\n", uniqueSizeCount)
	fmt.Printf("   ✓ %d fichiers candidats (%d groupes de même taille)\n", candidateCount, len(candidates))
	skippedPercent := float64(uniqueSizeCount) * 100 / float64(report.TotalFiles)
	fmt.Printf("   💡 %.1f%% des fichiers ignorés grâce au filtrage par taille\n", skippedPercent)

	if candidateCount == 0 {
		fmt.Println("   ✨ Aucun doublon potentiel trouvé!")
		report.UniqueFiles = report.TotalFiles
		return report
	}

	// Passe 2 : Quick hash des candidats
	fmt.Println("   🔍 Passe 2/3: Calcul des hash rapides...")
	bar2 := progressbar.NewOptions(candidateCount,
		progressbar.OptionSetDescription("   Quick hash"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
	)

	for _, candidate := range candidates {
		for _, path := range candidate.paths {
			hash, err := ComputeQuickHash(path, candidate.size)
			if err == nil {
				df.byQuickHash[hash] = append(df.byQuickHash[hash], path)
			}
			bar2.Add(1)
		}
	}
	fmt.Println() // Nouvelle ligne après la barre

	// Compter les fichiers pour la passe 3
	var fullHashCount int
	for _, paths := range df.byQuickHash {
		if len(paths) > 1 {
			fullHashCount += len(paths)
		}
	}
	fmt.Printf("   ✓ %d fichiers avec hash rapide identique\n", fullHashCount)

	if fullHashCount == 0 {
		fmt.Println("   ✨ Aucun doublon confirmé!")
		report.UniqueFiles = report.TotalFiles
		return report
	}

	// Passe 3 : Full hash pour confirmation
	fmt.Println("   🔐 Passe 3/3: Vérification complète...")
	bar3 := progressbar.NewOptions(fullHashCount,
		progressbar.OptionSetDescription("   Full hash "),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
	)

	for _, paths := range df.byQuickHash {
		if len(paths) > 1 {
			for _, path := range paths {
				hash, err := ComputeFullHash(path)
				if err == nil {
					df.byFullHash[hash] = append(df.byFullHash[hash], path)
				}
				bar3.Add(1)
			}
		}
	}
	fmt.Println() // Nouvelle ligne après la barre

	// Construire le rapport final
	seen := make(map[string]bool)
	for hash, paths := range df.byFullHash {
		if len(paths) > 1 && !seen[hash] {
			seen[hash] = true

			// Obtenir la taille (tous les fichiers ont la même)
			var size int64
			if info, err := os.Stat(paths[0]); err == nil {
				size = info.Size()
			}

			group := DuplicateGroup{
				Hash:  hash[:16] + "...", // Tronquer pour lisibilité
				Size:  size,
				Count: len(paths),
				Paths: paths,
				Waste: size * int64(len(paths)-1),
			}

			report.Groups = append(report.Groups, group)
			report.DuplicateGroups++
			report.DuplicateFiles += len(paths)
			report.WastedSpace += group.Waste
		}
	}

	report.UniqueFiles = report.TotalFiles - report.DuplicateFiles + report.DuplicateGroups

	return report
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS UTILITAIRES
// ═══════════════════════════════════════════════════════════════════════════

// FormatSize formate une taille en bytes de façon lisible.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return string(rune(bytes)) + " B"
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return string(rune(bytes/div)) + " " + units[exp]
}
