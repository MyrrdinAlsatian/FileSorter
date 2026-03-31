// dedup/deduplicate.go - Déduplication active
//
// Ce fichier gère la suppression ou le remplacement des fichiers doublons.
// Contrairement à hash.go qui détecte seulement les doublons,
// ce module permet de les traiter activement.
package dedup

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/schollz/progressbar/v3"
)

// ═══════════════════════════════════════════════════════════════════════════
// ACTIONS DE DÉDUPLICATION
// ═══════════════════════════════════════════════════════════════════════════

// DeduplicateAction définit l'action à effectuer sur les doublons.
type DeduplicateAction int

const (
	// ActionDelete supprime les doublons (garde l'original)
	ActionDelete DeduplicateAction = iota

	// ActionHardlink remplace les doublons par des hardlinks vers l'original
	// Avantage : un seul fichier physique, espace récupéré
	// Limitation : même partition uniquement
	ActionHardlink

	// ActionSymlink remplace les doublons par des liens symboliques
	// Avantage : fonctionne entre partitions
	// Inconvénient : si l'original est supprimé, les liens sont cassés
	ActionSymlink

	// ActionDryRun liste les actions sans les exécuter
	ActionDryRun
)

// String retourne le nom de l'action.
func (a DeduplicateAction) String() string {
	switch a {
	case ActionDelete:
		return "delete"
	case ActionHardlink:
		return "hardlink"
	case ActionSymlink:
		return "symlink"
	case ActionDryRun:
		return "dry-run"
	default:
		return "unknown"
	}
}

// ParseAction convertit une chaîne en action.
func ParseAction(s string) DeduplicateAction {
	switch s {
	case "delete", "del", "rm":
		return ActionDelete
	case "hardlink", "hard", "link":
		return ActionHardlink
	case "symlink", "sym", "soft":
		return ActionSymlink
	case "dry-run", "dryrun", "dry", "n":
		return ActionDryRun
	default:
		return ActionDryRun // Par défaut, ne rien faire
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SÉLECTION DE L'ORIGINAL
// ═══════════════════════════════════════════════════════════════════════════

// SelectionStrategy définit comment choisir le fichier à garder.
type SelectionStrategy int

const (
	// KeepFirst garde le premier fichier trouvé
	KeepFirst SelectionStrategy = iota

	// KeepShortest garde le fichier avec le chemin le plus court
	// Logique : plus le chemin est court, plus le fichier est "bien rangé"
	KeepShortest

	// KeepOldest garde le fichier avec la date de modification la plus ancienne
	KeepOldest

	// KeepNewest garde le fichier avec la date de modification la plus récente
	KeepNewest

	// KeepByPath garde le fichier dans un chemin spécifique (priorité)
	// Utile pour préférer les fichiers dans /sorted/ vs /recovery/
	KeepByPath
)

// String retourne le nom de la stratégie.
func (s SelectionStrategy) String() string {
	switch s {
	case KeepFirst:
		return "first"
	case KeepShortest:
		return "shortest"
	case KeepOldest:
		return "oldest"
	case KeepNewest:
		return "newest"
	case KeepByPath:
		return "path"
	default:
		return "unknown"
	}
}

// ParseStrategy convertit une chaîne en stratégie.
func ParseStrategy(s string) SelectionStrategy {
	switch s {
	case "first":
		return KeepFirst
	case "shortest", "short":
		return KeepShortest
	case "oldest", "old":
		return KeepOldest
	case "newest", "new":
		return KeepNewest
	case "path":
		return KeepByPath
	default:
		return KeepShortest
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// DÉDUPLICATEUR
// ═══════════════════════════════════════════════════════════════════════════

// DeduplicateOptions contient les options de déduplication.
type DeduplicateOptions struct {
	Action       DeduplicateAction // Action à effectuer
	Strategy     SelectionStrategy // Stratégie de sélection
	PriorityPath string            // Chemin prioritaire (pour KeepByPath)
	Workers      int               // Nombre de workers parallèles
	Verbose      bool              // Afficher les détails
}

// DefaultDeduplicateOptions retourne les options par défaut.
func DefaultDeduplicateOptions() DeduplicateOptions {
	return DeduplicateOptions{
		Action:   ActionDryRun,
		Strategy: KeepShortest,
		Workers:  4,
		Verbose:  false,
	}
}

// DeduplicateResult contient le résultat d'une déduplication.
type DeduplicateResult struct {
	Group       *DuplicateGroup // Groupe traité
	KeptPath    string          // Fichier conservé
	Processed   []string        // Fichiers traités (supprimés/liés)
	Errors      []string        // Erreurs rencontrées
	SpaceSaved  int64           // Espace récupéré
	ActionTaken DeduplicateAction
}

// DeduplicateReport contient le rapport complet.
type DeduplicateReport struct {
	StartTime       time.Time
	EndTime         time.Time
	Duration        time.Duration
	Action          DeduplicateAction
	Strategy        SelectionStrategy
	GroupsTotal     int
	GroupsProcessed int
	FilesRemoved    int
	FilesLinked     int
	Errors          int
	SpaceSaved      int64
	Results         []DeduplicateResult
}

// Deduplicator gère la déduplication active.
type Deduplicator struct {
	Options DeduplicateOptions
}

// NewDeduplicator crée un nouveau déduplicateur.
func NewDeduplicator(opts DeduplicateOptions) *Deduplicator {
	return &Deduplicator{Options: opts}
}

// ═══════════════════════════════════════════════════════════════════════════
// TRAITEMENT
// ═══════════════════════════════════════════════════════════════════════════

// ProcessGroups traite une liste de groupes de doublons.
func (d *Deduplicator) ProcessGroups(groups []DuplicateGroup) *DeduplicateReport {
	report := &DeduplicateReport{
		StartTime:   time.Now(),
		Action:      d.Options.Action,
		Strategy:    d.Options.Strategy,
		GroupsTotal: len(groups),
		Results:     make([]DeduplicateResult, 0, len(groups)),
	}

	if len(groups) == 0 {
		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		return report
	}

	// Progress bar
	bar := progressbar.NewOptions(len(groups),
		progressbar.OptionSetDescription(fmt.Sprintf("[%s] Déduplication", d.Options.Action)),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "│",
			BarEnd:        "│",
		}),
	)

	// Canal pour les résultats
	resultsChan := make(chan DeduplicateResult, len(groups))
	var wg sync.WaitGroup

	// Compteurs atomiques pour le parallélisme
	var groupsProcessed int64
	var filesRemoved int64
	var filesLinked int64
	var errCount int64
	var spaceSaved int64

	// Workers
	workers := d.Options.Workers
	if workers < 1 {
		workers = 1
	}
	semaphore := make(chan struct{}, workers)

	for i := range groups {
		wg.Add(1)
		semaphore <- struct{}{} // Acquérir un slot

		go func(group DuplicateGroup) {
			defer wg.Done()
			defer func() { <-semaphore }() // Libérer le slot

			result := d.processGroup(&group)
			resultsChan <- result

			// Compteurs atomiques
			atomic.AddInt64(&groupsProcessed, 1)
			atomic.AddInt64(&spaceSaved, result.SpaceSaved)

			if len(result.Errors) > 0 {
				atomic.AddInt64(&errCount, int64(len(result.Errors)))
			}

			switch result.ActionTaken {
			case ActionDelete:
				atomic.AddInt64(&filesRemoved, int64(len(result.Processed)))
			case ActionHardlink, ActionSymlink:
				atomic.AddInt64(&filesLinked, int64(len(result.Processed)))
			}

			bar.Add(1)
		}(groups[i])
	}

	// Attendre la fin et fermer le canal
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collecter les résultats
	for result := range resultsChan {
		report.Results = append(report.Results, result)
	}

	bar.Finish()

	// Remplir le rapport
	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime)
	report.GroupsProcessed = int(atomic.LoadInt64(&groupsProcessed))
	report.FilesRemoved = int(atomic.LoadInt64(&filesRemoved))
	report.FilesLinked = int(atomic.LoadInt64(&filesLinked))
	report.Errors = int(atomic.LoadInt64(&errCount))
	report.SpaceSaved = atomic.LoadInt64(&spaceSaved)

	return report
}

// processGroup traite un groupe de doublons.
func (d *Deduplicator) processGroup(group *DuplicateGroup) DeduplicateResult {
	result := DeduplicateResult{
		Group:       group,
		Processed:   make([]string, 0),
		Errors:      make([]string, 0),
		ActionTaken: d.Options.Action,
	}

	if len(group.Paths) < 2 {
		return result // Pas de doublon à traiter
	}

	// Sélectionner le fichier à garder
	keepPath := d.selectOriginal(group.Paths)
	result.KeptPath = keepPath

	// Traiter les autres fichiers
	for _, path := range group.Paths {
		if path == keepPath {
			continue
		}

		err := d.processFile(path, keepPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", path, err))
		} else {
			result.Processed = append(result.Processed, path)
			result.SpaceSaved += group.Size
		}
	}

	return result
}

// selectOriginal sélectionne le fichier à garder selon la stratégie.
func (d *Deduplicator) selectOriginal(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	if len(paths) == 1 {
		return paths[0]
	}

	switch d.Options.Strategy {
	case KeepFirst:
		return paths[0]

	case KeepShortest:
		shortest := paths[0]
		for _, p := range paths[1:] {
			if len(p) < len(shortest) {
				shortest = p
			}
		}
		return shortest

	case KeepOldest:
		return d.findByModTime(paths, true)

	case KeepNewest:
		return d.findByModTime(paths, false)

	case KeepByPath:
		// Préférer les fichiers dans le chemin prioritaire
		if d.Options.PriorityPath != "" {
			for _, p := range paths {
				if containsPath(p, d.Options.PriorityPath) {
					return p
				}
			}
		}
		// Fallback : chemin le plus court
		return d.selectOriginal(paths[:1])

	default:
		return paths[0]
	}
}

// findByModTime trouve le fichier le plus ancien ou récent.
func (d *Deduplicator) findByModTime(paths []string, oldest bool) string {
	var selected string
	var selectedTime time.Time
	first := true

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}

		modTime := info.ModTime()
		if first {
			selected = p
			selectedTime = modTime
			first = false
			continue
		}

		if oldest {
			if modTime.Before(selectedTime) {
				selected = p
				selectedTime = modTime
			}
		} else {
			if modTime.After(selectedTime) {
				selected = p
				selectedTime = modTime
			}
		}
	}

	if selected == "" && len(paths) > 0 {
		return paths[0]
	}
	return selected
}

// processFile exécute l'action sur un fichier doublon.
func (d *Deduplicator) processFile(duplicatePath, originalPath string) error {
	switch d.Options.Action {
	case ActionDryRun:
		// Ne rien faire, juste simuler
		if d.Options.Verbose {
			fmt.Printf("  [DRY-RUN] %s -> garder %s\n", duplicatePath, originalPath)
		}
		return nil

	case ActionDelete:
		return d.deleteFile(duplicatePath)

	case ActionHardlink:
		return d.replaceWithHardlink(duplicatePath, originalPath)

	case ActionSymlink:
		return d.replaceWithSymlink(duplicatePath, originalPath)

	default:
		return nil
	}
}

// deleteFile supprime un fichier.
func (d *Deduplicator) deleteFile(path string) error {
	if d.Options.Verbose {
		fmt.Printf("  [DELETE] %s\n", path)
	}
	return os.Remove(path)
}

// replaceWithHardlink remplace un fichier par un hardlink.
func (d *Deduplicator) replaceWithHardlink(duplicatePath, originalPath string) error {
	if d.Options.Verbose {
		fmt.Printf("  [HARDLINK] %s -> %s\n", duplicatePath, originalPath)
	}

	// Supprimer le doublon
	if err := os.Remove(duplicatePath); err != nil {
		return fmt.Errorf("cannot remove %s: %w", duplicatePath, err)
	}

	// Créer le hardlink
	if err := os.Link(originalPath, duplicatePath); err != nil {
		return fmt.Errorf("cannot create hardlink: %w", err)
	}

	return nil
}

// replaceWithSymlink remplace un fichier par un lien symbolique.
func (d *Deduplicator) replaceWithSymlink(duplicatePath, originalPath string) error {
	if d.Options.Verbose {
		fmt.Printf("  [SYMLINK] %s -> %s\n", duplicatePath, originalPath)
	}

	// Calculer le chemin relatif pour le symlink
	relPath, err := filepath.Rel(filepath.Dir(duplicatePath), originalPath)
	if err != nil {
		relPath = originalPath // Utiliser le chemin absolu si relatif échoue
	}

	// Supprimer le doublon
	if err := os.Remove(duplicatePath); err != nil {
		return fmt.Errorf("cannot remove %s: %w", duplicatePath, err)
	}

	// Créer le symlink
	if err := os.Symlink(relPath, duplicatePath); err != nil {
		return fmt.Errorf("cannot create symlink: %w", err)
	}

	return nil
}

// containsPath vérifie si un chemin contient un sous-chemin.
func containsPath(fullPath, subPath string) bool {
	// Normaliser les chemins
	fullPath = filepath.Clean(fullPath)
	subPath = filepath.Clean(subPath)

	// Vérifier si subPath est un préfixe de fullPath
	return len(fullPath) >= len(subPath) &&
		fullPath[:len(subPath)] == subPath
}

// ═══════════════════════════════════════════════════════════════════════════
// RAPPORT
// ═══════════════════════════════════════════════════════════════════════════

// PrintReport affiche le rapport de déduplication.
func (r *DeduplicateReport) PrintReport() {
	fmt.Println()
	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                  RAPPORT DE DÉDUPLICATION                         ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	fmt.Printf("  Action        : %s\n", r.Action)
	fmt.Printf("  Stratégie     : %s (sélection de l'original)\n", r.Strategy)
	fmt.Printf("  Durée         : %v\n", r.Duration.Round(time.Millisecond))
	fmt.Println()

	fmt.Printf("  Groupes traités  : %d / %d\n", r.GroupsProcessed, r.GroupsTotal)

	switch r.Action {
	case ActionDelete:
		fmt.Printf("  Fichiers supprimés : %d\n", r.FilesRemoved)
	case ActionHardlink, ActionSymlink:
		fmt.Printf("  Fichiers liés      : %d\n", r.FilesLinked)
	case ActionDryRun:
		fmt.Printf("  Fichiers (simulation) : %d\n", r.FilesRemoved+r.FilesLinked+len(r.Results))
	}

	if r.Errors > 0 {
		fmt.Printf("  Erreurs          : %d\n", r.Errors)
	}

	fmt.Println()
	fmt.Printf("  Espace récupéré : %s\n", formatSize(r.SpaceSaved))
	fmt.Println()

	if r.Action == ActionDryRun {
		fmt.Println("  ⚠️  Mode simulation - aucune modification effectuée")
		fmt.Println("     Utilisez --dedup delete|hardlink|symlink pour exécuter")
		fmt.Println()
	}
}
