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
	"bytes"
	"context"
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
	journal *OperationJournal

	journalErr   error
	journalErrMu sync.Mutex

	// diskSpaceCheck permet de simuler le contrôle système dans les tests.
	diskSpaceCheck func(*Plan) (bool, int64, error)

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

// SetOperationJournal active l'écriture des opérations terminées dans un journal partagé.
func (e *Executor) SetOperationJournal(journal *OperationJournal) {
	e.journal = journal
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
	return e.ExecuteContext(context.Background())
}

// ExecuteContext exécute le plan jusqu'à sa fin ou jusqu'à l'annulation du contexte.
func (e *Executor) ExecuteContext(ctx context.Context) *ExecutionResult {
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
	if err := ctx.Err(); err != nil {
		e.failPendingOperations(err)
		return e.finalizeExecutionResult(result)
	}

	enoughSpace, available, err := e.checkPlanDiskSpace()
	if err != nil {
		e.failPendingOperations(fmt.Errorf("disk space preflight failed: %w", err))
		return e.finalizeExecutionResult(result)
	}
	if !enoughSpace {
		required, sizeErr := pendingBytes(e.plan)
		if sizeErr != nil {
			e.failPendingOperations(sizeErr)
		} else {
			e.failPendingOperations(fmt.Errorf("insufficient disk space: need %d bytes, have %d bytes available", required, available))
		}
		return e.finalizeExecutionResult(result)
	}
	if err := ctx.Err(); err != nil {
		e.failPendingOperations(err)
		return e.finalizeExecutionResult(result)
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
			for {
				select {
				case <-ctx.Done():
					return
				case idx, ok := <-jobs:
					if !ok {
						return
					}
					e.executeOperation(ctx, idx)
					e.bar.Add(1)
				}
			}
		}()
	}

	// Envoyer les opérations aux workers
	for i := range e.plan.Operations {
		if e.plan.Operations[i].Status == StatusPending {
			select {
			case <-ctx.Done():
				break
			case jobs <- i:
			}
			if ctx.Err() != nil {
				break
			}
		}
	}
	close(jobs)

	// Attendre la fin
	wg.Wait()
	if ctx.Err() != nil {
		// Les jobs non distribués sont comptés comme échoués pour rendre l'interruption visible.
		for i := range e.plan.Operations {
			if e.plan.Operations[i].Status == StatusPending {
				e.recordError(i, ctx.Err())
			}
		}
	}
	e.bar.Finish()

	return e.finalizeExecutionResult(result)
}

func (e *Executor) checkPlanDiskSpace() (bool, int64, error) {
	if e.diskSpaceCheck != nil {
		return e.diskSpaceCheck(e.plan)
	}
	return CheckDiskSpace(e.plan)
}

func (e *Executor) failPendingOperations(err error) {
	for i := range e.plan.Operations {
		if e.plan.Operations[i].Status == StatusPending {
			e.recordError(i, err)
		}
	}
}

func (e *Executor) finalizeExecutionResult(result *ExecutionResult) *ExecutionResult {
	result.Succeeded = int(atomic.LoadInt64(&e.succeeded))
	result.Failed = int(atomic.LoadInt64(&e.failed))
	result.Skipped = int(atomic.LoadInt64(&e.skipped))
	result.BytesCopied = atomic.LoadInt64(&e.bytesCopied)
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	e.errMu.Lock()
	result.Errors = append([]OperationError(nil), e.errors...)
	e.errMu.Unlock()
	e.journalErrMu.Lock()
	result.JournalError = e.journalErr
	e.journalErrMu.Unlock()

	return result
}

// executeOperation exécute une opération individuelle.
func (e *Executor) executeOperation(ctx context.Context, idx int) {
	op := &e.plan.Operations[idx]
	op.StartTime = time.Now()
	op.Status = StatusRunning
	if err := ctx.Err(); err != nil {
		e.recordError(idx, err)
		return
	}

	// Une vérification d'intégrité fiable exige toujours les 32 octets du SHA-256.
	expectedHash := ""
	if e.options.Verify && (e.options.Mode == ModeCopy || e.options.Mode == ModeMove) {
		if !isFullSHA256(op.Hash) {
			e.recordError(idx, fmt.Errorf("full SHA-256 hash required for verification"))
			return
		}
		expectedHash = op.Hash
	}

	var err error

	// Créer le répertoire de destination si nécessaire
	if err = ctx.Err(); err != nil {
		e.recordError(idx, err)
		return
	}
	destDir := filepath.Dir(op.Destination)
	if err = os.MkdirAll(destDir, 0755); err != nil {
		e.recordError(idx, fmt.Errorf("cannot create directory: %w", err))
		return
	}

	// Vérifier si la destination existe déjà
	if _, statErr := os.Stat(op.Destination); statErr == nil {
		switch e.options.OverwriteMode {
		case ConflictSkip:
			op.Status = StatusSkipped
			op.Error = "destination exists"
			op.EndTime = time.Now()
			atomic.AddInt64(&e.skipped, 1)
			e.appendOperationJournal(op)
			return
		case ConflictOverwrite:
			// Pour copy/move, garder l'ancien fichier jusqu'à la publication atomique du nouveau.
			if e.options.Mode != ModeCopy && e.options.Mode != ModeMove {
				if err = os.Remove(op.Destination); err != nil {
					e.recordError(idx, fmt.Errorf("cannot remove existing file: %w", err))
					return
				}
			}
		}
		// "rename" est déjà géré lors de la génération du plan
	}

	// Exécuter selon le mode
	switch e.options.Mode {
	case ModeCopy:
		err = e.copyFileAtomicContext(ctx, op.Source, op.Destination, expectedHash)
	case ModeMove:
		err = e.moveFileWithHashContext(ctx, op.Source, op.Destination, expectedHash)
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

	// Succès !
	op.Status = StatusSuccess
	op.EndTime = time.Now()
	atomic.AddInt64(&e.succeeded, 1)
	atomic.AddInt64(&e.bytesCopied, op.Size)
	e.appendOperationJournal(op)
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
	e.appendOperationJournal(op)
}

func (e *Executor) appendOperationJournal(operation *Operation) {
	if e.journal == nil {
		return
	}
	e.journalErrMu.Lock()
	defer e.journalErrMu.Unlock()
	if e.journalErr != nil {
		return
	}
	if err := e.journal.Append(newOperationJournalEntry(e.options.Mode, *operation, time.Time{})); err != nil {
		e.journalErr = err
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// OPÉRATIONS DE FICHIERS
// ═══════════════════════════════════════════════════════════════════════════

// copyFile copie vers un temporaire, puis publie le fichier complet par renommage.
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
	return e.copyFileAtomicContext(context.Background(), src, dst, "")
}

// copyFileAtomic publie une copie seulement après copie complète et vérification éventuelle.
func (e *Executor) copyFileAtomic(src, dst, expectedHash string) error {
	return e.copyFileAtomicContext(context.Background(), src, dst, expectedHash)
}

func (e *Executor) copyFileAtomicContext(ctx context.Context, src, dst, expectedHash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Ouvrir le fichier source
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("cannot open source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("cannot inspect source: %w", err)
	}

	// Le temporaire est créé dans le même dossier pour que Rename ne change pas de volume.
	tempFile, err := os.CreateTemp(filepath.Dir(dst), ".filesorter-*.part")
	if err != nil {
		return fmt.Errorf("cannot create temporary destination: %w", err)
	}
	tempPath := tempFile.Name()
	tempClosed := false
	published := false
	defer func() {
		if !tempClosed {
			_ = tempFile.Close()
		}
		if !published {
			// Un temporaire peut avoir hérité de permissions en lecture seule; les retirer avant nettoyage.
			_ = os.Chmod(tempPath, 0600)
			_ = os.Remove(tempPath)
		}
	}()

	// Obtenir un buffer du pool
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)
	buf := *bufPtr

	// Copier avec le buffer
	if _, err = io.CopyBuffer(tempFile, contextReader{ctx: ctx, reader: srcFile}, buf); err != nil {
		return fmt.Errorf("copy failed: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}

	// Copier les métadonnées avant Sync pour inclure aussi ces changements sur le disque.
	if err = tempFile.Chmod(srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("cannot preserve file permissions: %w", err)
	}
	if err = os.Chtimes(tempPath, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		return fmt.Errorf("cannot preserve file timestamps: %w", err)
	}
	if err = tempFile.Sync(); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("cannot close temporary destination: %w", err)
	}
	tempClosed = true

	if err = srcFile.Close(); err != nil {
		return fmt.Errorf("cannot close source: %w", err)
	}

	// Le hash est vérifié sur le temporaire : une copie invalide n'atteint jamais le chemin final.
	if expectedHash != "" {
		valid, verifyErr := e.verifyHashContext(ctx, tempPath, expectedHash)
		if verifyErr != nil {
			return verifyErr
		}
		if !valid {
			return fmt.Errorf("hash verification failed")
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}

	if err = os.Rename(tempPath, dst); err != nil {
		return fmt.Errorf("cannot publish destination: %w", err)
	}
	published = true
	return nil
}

// moveFile déplace un fichier en publiant d'abord une copie, puis en supprimant la source.
// Cette séquence conserve l'original si la copie échoue, même si source et destination partagent un volume.
func (e *Executor) moveFile(src, dst string) error {
	return e.moveFileWithHash(src, dst, "")
}

// moveFileWithHash conserve la source jusqu'à la publication d'une copie vérifiée.
func (e *Executor) moveFileWithHash(src, dst, expectedHash string) error {
	return e.moveFileWithHashContext(context.Background(), src, dst, expectedHash)
}

func (e *Executor) moveFileWithHashContext(ctx context.Context, src, dst, expectedHash string) error {
	if err := e.copyFileAtomicContext(ctx, src, dst, expectedHash); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		// La destination publiée est complète; garder les deux fichiers vaut mieux que perdre la source.
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
	valid, err := e.verifyHashContext(context.Background(), path, expectedHash)
	return err == nil && valid
}

func (e *Executor) verifyHashContext(ctx context.Context, path string, expectedHash string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	expected, err := hex.DecodeString(expectedHash)
	if err != nil || len(expected) != sha256.Size {
		return false, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	h := sha256.New()

	// Utiliser un buffer du pool
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)

	if _, err := io.CopyBuffer(h, contextReader{ctx: ctx, reader: f}, *bufPtr); err != nil {
		return false, err
	}

	return bytes.Equal(h.Sum(nil), expected), nil
}

// contextReader propage l'annulation entre les lectures d'une copie ou d'une vérification.
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

// isFullSHA256 vérifie le format hexadécimal et la longueur exacte d'un SHA-256.
func isFullSHA256(hash string) bool {
	decoded, err := hex.DecodeString(hash)
	return err == nil && len(decoded) == sha256.Size
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
