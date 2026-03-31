// renamer/conflict.go - Gestion des conflits de noms
//
// Ce fichier gère les cas où plusieurs fichiers auraient le même nom
// après renommage.
package renamer

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// STRATÉGIES DE RÉSOLUTION
// ═══════════════════════════════════════════════════════════════════════════

// ConflictStrategy définit comment résoudre les conflits de noms.
type ConflictStrategy int

const (
	// StrategyIncrement ajoute un suffixe numérique : photo_001, photo_002
	StrategyIncrement ConflictStrategy = iota

	// StrategySkip ignore le fichier en cas de conflit
	StrategySkip

	// StrategyOverwrite écrase le fichier existant
	StrategyOverwrite

	// StrategyHash ajoute le hash au nom : photo_a1b2c3d4
	StrategyHash

	// StrategyTimestamp ajoute un timestamp : photo_20240115143052
	StrategyTimestamp
)

// String retourne le nom de la stratégie.
func (s ConflictStrategy) String() string {
	switch s {
	case StrategyIncrement:
		return "increment"
	case StrategySkip:
		return "skip"
	case StrategyOverwrite:
		return "overwrite"
	case StrategyHash:
		return "hash"
	case StrategyTimestamp:
		return "timestamp"
	default:
		return "unknown"
	}
}

// ParseStrategy convertit une chaîne en stratégie.
func ParseStrategy(s string) ConflictStrategy {
	switch strings.ToLower(s) {
	case "increment", "inc", "number":
		return StrategyIncrement
	case "skip", "ignore":
		return StrategySkip
	case "overwrite", "replace":
		return StrategyOverwrite
	case "hash":
		return StrategyHash
	case "timestamp", "time":
		return StrategyTimestamp
	default:
		return StrategyIncrement
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// RÉSOLVEUR DE CONFLITS
// ═══════════════════════════════════════════════════════════════════════════

// ConflictResolver gère les conflits de noms de fichiers.
type ConflictResolver struct {
	Strategy ConflictStrategy

	// usedNames trace les noms déjà utilisés par dossier
	usedNames map[string]map[string]int // dossier -> nom -> count
	mu        sync.Mutex
}

// NewConflictResolver crée un nouveau résolveur.
func NewConflictResolver(strategy ConflictStrategy) *ConflictResolver {
	return &ConflictResolver{
		Strategy:  strategy,
		usedNames: make(map[string]map[string]int),
	}
}

// ResolveResult représente le résultat de la résolution.
type ResolveResult struct {
	Name     string // Nom final du fichier
	Action   string // "rename", "skip", "overwrite"
	Conflict bool   // Y avait-il un conflit ?
}

// Resolve résout un conflit de nom potentiel.
//
// Paramètres :
//   - dir      : dossier de destination
//   - name     : nom souhaité (sans extension)
//   - ext      : extension (avec le point)
//   - hash     : hash du fichier (pour StrategyHash)
//
// Le résolveur maintient un état interne des noms déjà utilisés.
func (cr *ConflictResolver) Resolve(dir, name, ext, hash string) *ResolveResult {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// Initialiser le map pour ce dossier
	if cr.usedNames[dir] == nil {
		cr.usedNames[dir] = make(map[string]int)
	}

	fullName := name + ext
	usedInDir := cr.usedNames[dir]

	// Vérifier si le nom est déjà utilisé
	count, exists := usedInDir[fullName]
	if !exists {
		// Pas de conflit, marquer comme utilisé
		usedInDir[fullName] = 1
		return &ResolveResult{
			Name:     fullName,
			Action:   "rename",
			Conflict: false,
		}
	}

	// Conflit détecté ! Appliquer la stratégie
	switch cr.Strategy {
	case StrategyIncrement:
		return cr.resolveIncrement(dir, name, ext, count)

	case StrategySkip:
		return &ResolveResult{
			Name:     fullName,
			Action:   "skip",
			Conflict: true,
		}

	case StrategyOverwrite:
		usedInDir[fullName] = count + 1
		return &ResolveResult{
			Name:     fullName,
			Action:   "overwrite",
			Conflict: true,
		}

	case StrategyHash:
		return cr.resolveHash(dir, name, ext, hash)

	case StrategyTimestamp:
		return cr.resolveTimestamp(dir, name, ext)

	default:
		return cr.resolveIncrement(dir, name, ext, count)
	}
}

// resolveIncrement ajoute un suffixe numérique.
func (cr *ConflictResolver) resolveIncrement(dir, name, ext string, startCount int) *ResolveResult {
	usedInDir := cr.usedNames[dir]

	// Trouver le prochain numéro disponible
	for i := startCount + 1; i < 1000000; i++ {
		newName := fmt.Sprintf("%s_%04d%s", name, i, ext)
		if _, exists := usedInDir[newName]; !exists {
			usedInDir[newName] = 1
			// Mettre à jour le compteur du nom original
			usedInDir[name+ext] = i
			return &ResolveResult{
				Name:     newName,
				Action:   "rename",
				Conflict: true,
			}
		}
	}

	// Fallback improbable
	return &ResolveResult{
		Name:     name + "_overflow" + ext,
		Action:   "rename",
		Conflict: true,
	}
}

// resolveHash ajoute le hash au nom.
func (cr *ConflictResolver) resolveHash(dir, name, ext, hash string) *ResolveResult {
	usedInDir := cr.usedNames[dir]

	// Utiliser les 8 premiers caractères du hash
	shortHash := hash
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}
	if shortHash == "" {
		shortHash = "nohash"
	}

	newName := fmt.Sprintf("%s_%s%s", name, shortHash, ext)

	// Vérifier si ce nom unique est déjà pris (très improbable)
	if _, exists := usedInDir[newName]; exists {
		// Fallback vers increment
		return cr.resolveIncrement(dir, name+"_"+shortHash, ext, 0)
	}

	usedInDir[newName] = 1
	return &ResolveResult{
		Name:     newName,
		Action:   "rename",
		Conflict: true,
	}
}

// resolveTimestamp ajoute l'heure courante au nom.
func (cr *ConflictResolver) resolveTimestamp(dir, name, ext string) *ResolveResult {
	usedInDir := cr.usedNames[dir]

	// Format : _HHMMSS_micro
	now := time.Now()
	timestamp := now.Format("150405")
	micro := fmt.Sprintf("%06d", now.Nanosecond()/1000)

	newName := fmt.Sprintf("%s_%s_%s%s", name, timestamp, micro[:3], ext)

	// En cas de collision (très improbable)
	if _, exists := usedInDir[newName]; exists {
		return cr.resolveIncrement(dir, name+"_"+timestamp, ext, 0)
	}

	usedInDir[newName] = 1
	return &ResolveResult{
		Name:     newName,
		Action:   "rename",
		Conflict: true,
	}
}

// Reset efface l'historique des noms utilisés.
func (cr *ConflictResolver) Reset() {
	cr.mu.Lock()
	cr.usedNames = make(map[string]map[string]int)
	cr.mu.Unlock()
}

// ResetDir efface l'historique pour un dossier spécifique.
func (cr *ConflictResolver) ResetDir(dir string) {
	cr.mu.Lock()
	delete(cr.usedNames, dir)
	cr.mu.Unlock()
}

// ═══════════════════════════════════════════════════════════════════════════
// BATCH RESOLVER
// ═══════════════════════════════════════════════════════════════════════════

// BatchRename représente une opération de renommage planifiée.
type BatchRename struct {
	OriginalPath string
	TargetDir    string
	NewName      string
	Action       string // "rename", "skip", "overwrite"
	Conflict     bool
	Hash         string
}

// BatchResolver résout les conflits pour un lot de fichiers.
//
// Cette approche permet de :
// 1. Calculer tous les noms à l'avance
// 2. Détecter les conflits entre fichiers du même batch
// 3. Résoudre de manière cohérente
//
// Paramètres :
//   - renames : liste des renommages souhaités (chemin -> nouveau nom)
//   - strategy : stratégie de résolution
//
// Retourne la liste des renommages avec les conflits résolus.
func BatchResolver(renames map[string]BatchRename, strategy ConflictStrategy) []BatchRename {
	resolver := NewConflictResolver(strategy)
	result := make([]BatchRename, 0, len(renames))

	for originalPath, rename := range renames {
		dir := rename.TargetDir
		fullName := rename.NewName
		ext := filepath.Ext(fullName)
		name := strings.TrimSuffix(fullName, ext)

		resolved := resolver.Resolve(dir, name, ext, rename.Hash)

		result = append(result, BatchRename{
			OriginalPath: originalPath,
			TargetDir:    dir,
			NewName:      resolved.Name,
			Action:       resolved.Action,
			Conflict:     resolved.Conflict,
			Hash:         rename.Hash,
		})
	}

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// STATISTIQUES
// ═══════════════════════════════════════════════════════════════════════════

// ConflictStats contient les statistiques de résolution.
type ConflictStats struct {
	TotalFiles     int
	Conflicts      int
	Renamed        int
	Skipped        int
	Overwritten    int
	ConflictsByDir map[string]int
}

// AnalyzeConflicts analyse les conflits dans un batch.
func AnalyzeConflicts(renames []BatchRename) *ConflictStats {
	stats := &ConflictStats{
		ConflictsByDir: make(map[string]int),
	}

	for _, r := range renames {
		stats.TotalFiles++

		if r.Conflict {
			stats.Conflicts++
			stats.ConflictsByDir[r.TargetDir]++
		}

		switch r.Action {
		case "rename":
			stats.Renamed++
		case "skip":
			stats.Skipped++
		case "overwrite":
			stats.Overwritten++
		}
	}

	return stats
}
