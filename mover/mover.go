// Package mover gère le déplacement et la copie des fichiers vers leur destination.
//
// Ce package est responsable de :
// - Générer un plan de déplacement à partir du JSONL
// - Exécuter les opérations (copy, move, hardlink, symlink)
// - Vérifier l'intégrité après copie
// - Fournir des statistiques et une progression
//
// CONCEPT : PLAN D'EXÉCUTION
// ==========================
// Plutôt que d'exécuter les opérations directement, on génère d'abord un "plan"
// qui décrit toutes les opérations à effectuer. Cela permet :
// - De prévisualiser les changements (dry-run)
// - De vérifier l'espace disque nécessaire
// - De détecter les conflits avant d'agir
// - D'annuler proprement en cas d'erreur
package mover

import (
	"fmt"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// MODES D'OPÉRATION
// ═══════════════════════════════════════════════════════════════════════════

// Mode définit le type d'opération à effectuer sur les fichiers.
type Mode string

const (
	// ModeCopy copie les fichiers (conserve les originaux)
	ModeCopy Mode = "copy"

	// ModeMove déplace les fichiers (supprime les originaux après copie réussie)
	ModeMove Mode = "move"

	// ModeHardlink crée des hardlinks (même partition uniquement)
	// Un hardlink est un deuxième nom pour le même fichier sur le disque.
	// Avantage : pas d'espace disque supplémentaire
	// Limitation : même partition uniquement
	ModeHardlink Mode = "hardlink"

	// ModeSymlink crée des liens symboliques
	// Un symlink est un "raccourci" qui pointe vers le fichier original.
	// Avantage : fonctionne entre partitions
	// Inconvénient : si l'original est supprimé, le lien est cassé
	ModeSymlink Mode = "symlink"
)

// ═══════════════════════════════════════════════════════════════════════════
// OPTIONS DE DÉPLACEMENT
// ═══════════════════════════════════════════════════════════════════════════

// Options contient les paramètres pour l'exécution du plan.
type Options struct {
	Mode          Mode   // Mode d'opération (copy, move, hardlink, symlink)
	Destination   string // Répertoire de destination
	DryRun        bool   // Mode simulation (ne fait rien)
	Verify        bool   // Vérifier le hash après copie
	Workers       int    // Nombre de workers parallèles
	OverwriteMode string // "skip", "overwrite", "rename"
	Verbose       bool   // Affichage détaillé

	// Filtres
	SkipCorrupted bool     // Ignorer les fichiers corrompus (valid=false)
	SkipSmall     int64    // Ignorer les fichiers < N bytes
	OnlyTypes     []string // Uniquement ces types (vide = tous)
	ExcludeTypes  []string // Exclure ces types
}

// DefaultOptions retourne les options par défaut.
func DefaultOptions() Options {
	return Options{
		Mode:          ModeCopy,
		Destination:   "",
		DryRun:        false,
		Verify:        false,
		Workers:       4,
		OverwriteMode: "skip",
		Verbose:       false,
		SkipCorrupted: true,
		SkipSmall:     0,
		OnlyTypes:     nil,
		ExcludeTypes:  nil,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// OPÉRATION INDIVIDUELLE
// ═══════════════════════════════════════════════════════════════════════════

// Operation représente une opération de fichier à effectuer.
type Operation struct {
	Source      string // Chemin source du fichier
	Destination string // Chemin de destination
	Size        int64  // Taille du fichier
	Hash        string // Hash pour vérification (si disponible)

	// Métadonnées pour le tri
	Type     string // Type de fichier (jpg, mp4, etc.)
	Category string // Catégorie (images/photos, videos/movies, etc.)

	// État de l'opération
	Status    OperationStatus // pending, success, failed, skipped
	Error     string          // Message d'erreur si échoué
	StartTime time.Time       // Heure de début
	EndTime   time.Time       // Heure de fin
}

// OperationStatus représente l'état d'une opération.
type OperationStatus string

const (
	StatusPending  OperationStatus = "pending"
	StatusRunning  OperationStatus = "running"
	StatusSuccess  OperationStatus = "success"
	StatusFailed   OperationStatus = "failed"
	StatusSkipped  OperationStatus = "skipped"
	StatusConflict OperationStatus = "conflict"
)

// ═══════════════════════════════════════════════════════════════════════════
// PLAN D'EXÉCUTION
// ═══════════════════════════════════════════════════════════════════════════

// Plan contient toutes les opérations à effectuer.
type Plan struct {
	Operations []Operation // Liste des opérations
	Options    Options     // Options d'exécution

	// Statistiques pré-calculées
	TotalFiles      int   // Nombre total de fichiers
	TotalSize       int64 // Taille totale à traiter
	SkippedFiles    int   // Fichiers ignorés (filtres, corrupted, etc.)
	ConflictFiles   int   // Fichiers avec conflit de destination
	DirectoriesUsed int   // Nombre de répertoires de destination

	// Mutex pour modification thread-safe
	mu sync.RWMutex
}

// NewPlan crée un nouveau plan vide.
func NewPlan(opts Options) *Plan {
	return &Plan{
		Operations: make([]Operation, 0),
		Options:    opts,
	}
}

// AddOperation ajoute une opération au plan.
func (p *Plan) AddOperation(op Operation) {
	p.mu.Lock()
	defer p.mu.Unlock()

	op.Status = StatusPending
	p.Operations = append(p.Operations, op)
	p.TotalFiles++
	p.TotalSize += op.Size
}

// GetPendingOperations retourne les opérations en attente.
func (p *Plan) GetPendingOperations() []Operation {
	p.mu.RLock()
	defer p.mu.RUnlock()

	pending := make([]Operation, 0)
	for _, op := range p.Operations {
		if op.Status == StatusPending {
			pending = append(pending, op)
		}
	}
	return pending
}

// ═══════════════════════════════════════════════════════════════════════════
// RÉSULTATS D'EXÉCUTION
// ═══════════════════════════════════════════════════════════════════════════

// ExecutionResult contient les résultats de l'exécution d'un plan.
type ExecutionResult struct {
	Plan *Plan // Plan exécuté

	// Compteurs
	Succeeded   int   // Opérations réussies
	Failed      int   // Opérations échouées
	Skipped     int   // Opérations ignorées
	BytesCopied int64 // Octets copiés

	// Timing
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	// Erreurs
	Errors []OperationError
}

// OperationError associe une erreur à son opération.
type OperationError struct {
	Operation Operation
	Error     error
}

// Summary retourne un résumé textuel de l'exécution.
func (r *ExecutionResult) Summary() string {
	return fmt.Sprintf(
		"Terminé en %v: %d réussis, %d échoués, %d ignorés (%s copiés)",
		r.Duration.Round(time.Second),
		r.Succeeded,
		r.Failed,
		r.Skipped,
		formatBytes(r.BytesCopied),
	)
}

// formatBytes formate une taille en bytes de manière lisible.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
