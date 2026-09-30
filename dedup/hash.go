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
// Cela permet d'éliminer rapidement les fichiers différents
// sans avoir à lire tout le contenu de chaque fichier.
// OPTIMISATIONS IMPLÉMENTÉES :
// 1. Filtrage par taille - ignorer les fichiers uniques (pas de doublon possible)
// 2. Filtrage par taille minimale - ignorer les petits fichiers (peu d'espace récupérable)
// 3. Quick hash - hash rapide (début+fin) avant le hash complet
// 4. Parallélisation - plusieurs workers pour calculer les hash simultanément
package dedup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"

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

	// DefaultMinSize : taille minimale par défaut pour chercher les doublons (1 MB)
	DefaultMinSize = 1 * 1024 * 1024
)

// ═══════════════════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════════════════

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
	SkippedSmall    int              `json:"skipped_small_files"`
	Groups          []DuplicateGroup `json:"groups,omitempty"`
}

// FinderOptions contient les options de configuration du détecteur.
type FinderOptions struct {
	MinSize int64 // Taille minimale des fichiers à analyser (0 = tous)
	Workers int   // Nombre de workers parallèles (0 = auto)
}

// DefaultFinderOptions retourne les options par défaut.
func DefaultFinderOptions() FinderOptions {
	return FinderOptions{
		MinSize: DefaultMinSize,
		Workers: runtime.NumCPU(),
	}
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
	return ComputeQuickHashContext(context.Background(), path, size)
}

// ComputeQuickHashContext calcule le hash rapide et vérifie le contexte entre les lectures.
func ComputeQuickHashContext(ctx context.Context, path string, size int64) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Petits fichiers : hash complet directement
	if size <= SmallFileThreshold {
		return ComputeFullHashContext(ctx, path)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()

	// Lire le début
	buf := make([]byte, QuickHashSize)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	hasher.Write(buf[:n])

	// Lire la fin
	if size > QuickHashSize*2 {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		_, err = f.Seek(-QuickHashSize, io.SeekEnd)
		if err != nil {
			return "", err
		}
		if err := ctx.Err(); err != nil {
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
	return ComputeFullHashContext(context.Background(), path)
}

// ComputeFullHashContext calcule le SHA-256 complet en s'interrompant entre deux lectures.
func ComputeFullHashContext(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()

	// Lire par blocs de 32KB pour économiser la mémoire
	buf := make([]byte, 32*1024)
	if _, err := io.CopyBuffer(hasher, contextReader{ctx: ctx, reader: f}, buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// contextReader vérifie l'annulation avant chaque lecture sans changer le lecteur sous-jacent.
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
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
	mu      sync.Mutex
	bySize  map[int64][]string
	options FinderOptions
}

// NewDuplicateFinder crée un nouveau détecteur avec options par défaut.
func NewDuplicateFinder() *DuplicateFinder {
	return NewDuplicateFinderWithOptions(DefaultFinderOptions())
}

// NewDuplicateFinderWithOptions crée un détecteur avec options personnalisées.
func NewDuplicateFinderWithOptions(opts FinderOptions) *DuplicateFinder {
	if opts.Workers <= 0 {
		opts.Workers = runtime.NumCPU()
	}
	return &DuplicateFinder{
		bySize:  make(map[int64][]string),
		options: opts,
	}
}

// AddFile ajoute un fichier au détecteur (thread-safe).
func (df *DuplicateFinder) AddFile(path string, size int64) {
	df.mu.Lock()
	defer df.mu.Unlock()
	df.bySize[size] = append(df.bySize[size], path)
}

// hashJob représente un travail de hash à effectuer.
type hashJob struct {
	path string
	size int64
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
	report, _ := df.FindDuplicatesContext(context.Background())
	return report
}

// FindDuplicatesContext annule les passes de hash si le contexte est interrompu.
func (df *DuplicateFinder) FindDuplicatesContext(ctx context.Context) (*DuplicateReport, error) {
	report := &DuplicateReport{}
	workers := df.options.Workers
	minSize := df.options.MinSize

	fmt.Printf("   ⚙️  Configuration: %d workers, taille min: %s\n", workers, formatSize(minSize))

	// ═══════════════════════════════════════════════════════════════════════
	// PASSE 1 : Filtrage par taille
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("   📋 Passe 1/3: Filtrage par taille...")

	var candidates []hashJob
	var uniqueSizeCount, skippedSmall int

	for size, paths := range df.bySize {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		report.TotalFiles += len(paths)

		// Ignorer les fichiers trop petits
		if size < minSize {
			skippedSmall += len(paths)
			continue
		}

		// Garder seulement les tailles avec 2+ fichiers
		if len(paths) == 1 {
			uniqueSizeCount++
			continue
		}

		// Ajouter comme candidats
		for _, p := range paths {
			candidates = append(candidates, hashJob{path: p, size: size})
		}
	}

	report.SkippedSmall = skippedSmall
	fmt.Printf("   ✓ %d fichiers < %s ignorés (trop petits)\n", skippedSmall, formatSize(minSize))
	fmt.Printf("   ✓ %d fichiers avec taille unique ignorés\n", uniqueSizeCount)
	fmt.Printf("   ✓ %d fichiers candidats à analyser\n", len(candidates))

	if len(candidates) == 0 {
		fmt.Println("   ✨ Aucun doublon potentiel trouvé!")
		report.UniqueFiles = report.TotalFiles - skippedSmall
		return report, nil
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PASSE 2 : Quick hash (parallélisé)
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("   🔍 Passe 2/3: Calcul des hash rapides...")

	byQuickHash, err := df.parallelHashContext(ctx, candidates, workers, "Quick hash", func(ctx context.Context, job hashJob) (string, error) {
		return ComputeQuickHashContext(ctx, job.path, job.size)
	})
	if err != nil {
		return report, err
	}

	// Filtrer pour ne garder que les groupes avec 2+ fichiers
	var fullHashCandidates []hashJob
	for _, paths := range byQuickHash {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if len(paths) > 1 {
			for _, p := range paths {
				// Récupérer la taille depuis bySize
				var size int64
				for s, ps := range df.bySize {
					for _, pp := range ps {
						if pp == p {
							size = s
							break
						}
					}
					if size > 0 {
						break
					}
				}
				fullHashCandidates = append(fullHashCandidates, hashJob{path: p, size: size})
			}
		}
	}

	fmt.Printf("   ✓ %d fichiers avec quick hash identique\n", len(fullHashCandidates))

	if len(fullHashCandidates) == 0 {
		fmt.Println("   ✨ Aucun doublon confirmé!")
		report.UniqueFiles = report.TotalFiles - skippedSmall
		return report, nil
	}

	// ═══════════════════════════════════════════════════════════════════════
	// PASSE 3 : Full hash (parallélisé)
	// ═══════════════════════════════════════════════════════════════════════
	fmt.Println("   🔐 Passe 3/3: Vérification complète...")

	byFullHash, err := df.parallelHashContext(ctx, fullHashCandidates, workers, "Full hash ", func(ctx context.Context, job hashJob) (string, error) {
		return ComputeFullHashContext(ctx, job.path)
	})
	if err != nil {
		return report, err
	}

	// ═══════════════════════════════════════════════════════════════════════
	// Construction du rapport
	// ═══════════════════════════════════════════════════════════════════════
	for hash, paths := range byFullHash {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if len(paths) > 1 {
			var size int64
			if info, err := os.Stat(paths[0]); err == nil {
				size = info.Size()
			}

			group := DuplicateGroup{
				Hash:  hash[:16] + "...",
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

	report.UniqueFiles = report.TotalFiles - report.DuplicateFiles + report.DuplicateGroups - skippedSmall

	return report, nil
}

// parallelHash calcule les hash en parallèle avec une barre de progression.
func (df *DuplicateFinder) parallelHashContext(
	ctx context.Context,
	jobs []hashJob,
	workers int,
	description string,
	hashFunc func(context.Context, hashJob) (string, error),
) (map[string][]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results := make(map[string][]string)
	var mu sync.Mutex
	var processed int64

	// Créer la barre de progression
	bar := progressbar.NewOptions(len(jobs),
		progressbar.OptionSetDescription("   "+description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowIts(),
	)

	// Canal pour distribuer les jobs
	jobChan := make(chan hashJob, workers*2)

	// WaitGroup pour attendre tous les workers
	var wg sync.WaitGroup

	// Lancer les workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-jobChan:
					if !ok {
						return
					}
					hash, err := hashFunc(ctx, job)
					if err == nil {
						mu.Lock()
						results[hash] = append(results[hash], job.path)
						mu.Unlock()
					}
					atomic.AddInt64(&processed, 1)
					bar.Add(1)
				}
			}
		}()
	}

	// Envoyer les jobs
dispatch:
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			break dispatch
		case jobChan <- job:
		}
	}
	close(jobChan)

	// Attendre la fin
	wg.Wait()
	fmt.Println() // Nouvelle ligne après la barre
	if err := ctx.Err(); err != nil {
		return results, err
	}
	return results, nil
}

// formatSize formate une taille en bytes de façon lisible.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}
