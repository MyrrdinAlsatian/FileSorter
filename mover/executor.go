// mover/executor.go - Exécution des opérations de fichiers
//
// Ce fichier gère :
// - Copie de fichiers avec buffer optimisé
// - Déplacement de fichiers
// - Création de hardlinks et symlinks
// - Vérification d'intégrité post-copie
// - Exécution parallèle avec progress bar
package mover

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ═══════════════════════════════════════════════════════════════════════════
// EXÉCUTEUR DE PLAN
// ═══════════════════════════════════════════════════════════════════════════

// Executor exécute un plan de déplacement.
type Executor struct {
	plan    *Plan
	options Options

	// Compteurs (thread-safe avec atomic)
	succeeded   int64
	failed      int64
	skipped     int64
	bytesCopied int64

	// Erreurs collectées
	errors []OperationError
	errMu  sync.Mutex

	// Progress bar
	bar *progressbar.ProgressBar
}
// NewExecutor crée un nouvel exécuteur pour un plan.
func NewExecutor(plan *Plan) *Executor {
	return &Executor{
		plan:    plan,
		options: plan.Options,
		errors:  make([]OperationError, 0),
	}
}

// Execute exécute le plan et retourne les résultats.
//
// CONCEPT GO : EXÉCUTION PARALLÈLE CONTRÔLÉE
// ==========================================
// On utilise un pattern "worker pool" :
// 1. Un canal `jobs` distribue les opérations aux workers
// 2. N workers (goroutines) traitent les opérations en parallèle
// 3. Un WaitGroup attend que tous les workers aient fini
func (e *Executor) Execute() *ExecutionResult {
	result := &ExecutionResult{
		Plan:      e.plan,
		StartTime: time.Now(),
	}

	// Mode dry-run : simuler seulement
	if e.options.DryRun {
		fmt.Println("🔍 MODE SIMULATION - Aucun fichier ne sera modifié")
		e.plan.PrintSummary()
		e.plan.PrintPreview(20)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result
	}

	// Créer la progress bar
	e.bar = progressbar.NewOptions64(
		int64(e.plan.TotalFiles),
		progressbar.OptionSetDescription("📦 Déplacement"),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)

	// Déterminer le nombre de workers
	workers := e.options.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > 16 {
		workers = 16 // Limiter pour éviter trop d'I/O simultanées
	}

	// Canal pour distribuer les opérations
	jobs := make(chan int, workers*2)

	// WaitGroup pour attendre les workers
	var wg sync.WaitGroup

	// Lancer les workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				e.executeOperation(idx)
				e.bar.Add(1)
			}
		}()
	}

	// Envoyer les opérations aux workers
	for i := range e.plan.Operations {
		if e.plan.Operations[i].Status == StatusPending {
			jobs <- i
		}
	}
	close(jobs)

	// Attendre la fin
	wg.Wait()
	e.bar.Finish()

	// Compiler les résultats
	result.Succeeded = int(atomic.LoadInt64(&e.succeeded))
	result.Failed = int(atomic.LoadInt64(&e.failed))
	result.Skipped = int(atomic.LoadInt64(&e.skipped))
	result.BytesCopied = atomic.LoadInt64(&e.bytesCopied)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Errors = e.errors

	return result
}

// executeOperation exécute une opération individuelle.
func (e *Executor) executeOperation(idx int) {
	op := &e.plan.Operations[idx]
	op.StartTime = time.Now()
	op.Status = StatusRunning

	var err error

	// Créer le répertoire de destination si nécessaire
	destDir := filepath.Dir(op.Destination)
	if err = os.MkdirAll(destDir, 0755); err != nil {
		e.recordError(idx, fmt.Errorf("cannot create directory: %w", err))
		return
	}

	// Vérifier si la destination existe déjà
	if _, statErr := os.Stat(op.Destination); statErr == nil {
		switch e.options.OverwriteMode {
		case "skip":
			op.Status = StatusSkipped
			op.Error = "destination exists"
			atomic.AddInt64(&e.skipped, 1)
			return
		case "overwrite":
			// Supprimer l'existant
			if err = os.Remove(op.Destination); err != nil {
				e.recordError(idx, fmt.Errorf("cannot remove existing file: %w", err))
				return
			}
		}
		// "rename" est déjà géré lors de la génération du plan
	}

	// Exécuter selon le mode
	switch e.options.Mode {
	case ModeCopy:
		err = e.copyFile(op.Source, op.Destination)
	case ModeMove:
		err = e.moveFile(op.Source, op.Destination)
	case ModeHardlink:
		err = os.Link(op.Source, op.Destination)
	case ModeSymlink:
		err = os.Symlink(op.Source, op.Destination)
	default:
		err = fmt.Errorf("unknown mode: %s", e.options.Mode)
	}

	if err != nil {
		e.recordError(idx, err)
		return
	}

	// Vérification d'intégrité (si demandée et hash disponible)
	if e.options.Verify && op.Hash != "" && (e.options.Mode == ModeCopy || e.options.Mode == ModeMove) {
		if !e.verifyHash(op.Destination, op.Hash) {
			e.recordError(idx, fmt.Errorf("hash verification failed"))
			// Supprimer le fichier corrompu
			os.Remove(op.Destination)
			return
		}
	}

	// Succès !
	op.Status = StatusSuccess
	op.EndTime = time.Now()
	atomic.AddInt64(&e.succeeded, 1)
	atomic.AddInt64(&e.bytesCopied, op.Size)
}

// recordError enregistre une erreur pour une opération.
func (e *Executor) recordError(idx int, err error) {
	op := &e.plan.Operations[idx]
	op.Status = StatusFailed
	op.Error = err.Error()
	op.EndTime = time.Now()

	atomic.AddInt64(&e.failed, 1)

	e.errMu.Lock()
	e.errors = append(e.errors, OperationError{
		Operation: *op,
		Error:     err,
	})
	e.errMu.Unlock()
}

// ═══════════════════════════════════════════════════════════════════════════
// OPÉRATIONS DE FICHIERS
// ═══════════════════════════════════════════════════════════════════════════

// copyFile copie un fichier avec un buffer optimisé.
//
// CONCEPT : BUFFER POOLING
// ========================
// On utilise un pool de buffers pour éviter les allocations répétées.
// Chaque worker réutilise le même buffer pour toutes ses copies.
var bufferPool = sync.Pool{
	New: func() interface{} {
		// Buffer de 1 MB pour des copies efficaces
		buf := make([]byte, 1024*1024)
		return &buf
	},
}

func (e *Executor) copyFile(src, dst string) error {
	// Ouvrir le fichier source
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open source: %w", err)
	}
	defer srcFile.Close()

	// Créer le fichier destination
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("cannot create destination: %w", err)
	}
	defer dstFile.Close()

	// Obtenir un buffer du pool
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)
	buf := *bufPtr

	// Copier avec le buffer
	_, err = io.CopyBuffer(dstFile, srcFile, buf)
	if err != nil {
		// Supprimer la destination partielle en cas d'erreur
		dstFile.Close()
		os.Remove(dst)
		return fmt.Errorf("copy failed: %w", err)
	}

	// Synchroniser sur le disque
	if err = dstFile.Sync(); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	// Copier les permissions
	srcInfo, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, srcInfo.Mode())
	}

	// Copier les timestamps
	if srcInfo != nil {
		os.Chtimes(dst, srcInfo.ModTime(), srcInfo.ModTime())
	}

	return nil
}

// moveFile déplace un fichier (rename ou copy+delete).
//
// On essaie d'abord os.Rename qui est atomique et instantané.
// Si ça échoue (différentes partitions), on fait copy+delete.
func (e *Executor) moveFile(src, dst string) error {
	// Essayer le rename direct (même partition)
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// Rename a échoué, faire copy + delete
	if err := e.copyFile(src, dst); err != nil {
		return err
	}

	// Supprimer la source
	if err := os.Remove(src); err != nil {
		// La copie a réussi mais on n'a pas pu supprimer la source
		// Ce n'est pas critique, on retourne juste un warning
		return fmt.Errorf("copy succeeded but cannot remove source: %w", err)
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// VÉRIFICATION D'INTÉGRITÉ
// ═══════════════════════════════════════════════════════════════════════════

// verifyHash vérifie que le hash du fichier correspond à celui attendu.
func (e *Executor) verifyHash(path string, expectedHash string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	h := sha256.New()

	// Utiliser un buffer du pool
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)

	if _, err := io.CopyBuffer(h, f, *bufPtr); err != nil {
		return false
	}

	actualHash := hex.EncodeToString(h.Sum(nil))

	// Comparer (le hash attendu peut être un quick hash, donc préfixe uniquement)
	if len(expectedHash) < len(actualHash) {
		return actualHash[:len(expectedHash)] == expectedHash
	}
	return actualHash == expectedHash
}

// ═══════════════════════════════════════════════════════════════════════════
// VÉRIFICATION D'ESPACE DISQUE
// ═══════════════════════════════════════════════════════════════════════════

// CheckDiskSpace vérifie qu'il y a assez d'espace disque pour le plan.
func CheckDiskSpace(plan *Plan) (bool, int64, error) {
	// Note: Cette fonction est un placeholder.
	// L'implémentation réelle dépend du système d'exploitation.
	// Sur Windows, on utiliserait syscall.GetDiskFreeSpaceEx
	// Sur Unix, on utiliserait syscall.Statfs

	// Pour l'instant, on retourne toujours OK
	return false, 0, fmt.Errorf("disk space check is not implemented")
}

// ═══════════════════════════════════════════════════════════════════════════
// AFFICHAGE DES RÉSULTATS
// ═══════════════════════════════════════════════════════════════════════════

// PrintResults affiche les résultats de l'exécution.
func (r *ExecutionResult) PrintResults() {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    RÉSULTATS D'EXÉCUTION                          ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Icône selon le succès
	if r.Failed == 0 {
		fmt.Println("  ✅ Terminé avec succès!")
	} else {
		fmt.Println("  ⚠️  Terminé avec des erreurs")
	}
	fmt.Println()

	fmt.Printf("  ⏱️  Durée         : %v\n", r.Duration.Round(time.Second))
	fmt.Printf("  ✓  Réussis       : %d\n", r.Succeeded)
	fmt.Printf("  ✗  Échoués       : %d\n", r.Failed)
	fmt.Printf("  ⏭️  Ignorés       : %d\n", r.Skipped)
	fmt.Printf("  💾 Données copiées: %s\n", formatBytes(r.BytesCopied))

	// Vitesse
	if r.Duration.Seconds() > 0 {
		speed := float64(r.BytesCopied) / r.Duration.Seconds()
		fmt.Printf("  🚀 Vitesse        : %s/s\n", formatBytes(int64(speed)))
	}

	fmt.Println()

	// Afficher les erreurs
	if len(r.Errors) > 0 {
		fmt.Println("  ❌ Erreurs:")
		maxErrors := 10
		for i, e := range r.Errors {
			if i >= maxErrors {
				fmt.Printf("     ... et %d autres erreurs\n", len(r.Errors)-maxErrors)
				break
			}
			fmt.Printf("     • %s: %v\n", filepath.Base(e.Operation.Source), e.Error)
		}
		fmt.Println()
	}
}
