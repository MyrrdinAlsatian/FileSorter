// Package validator fournit des outils pour valider l'intégrité des fichiers.
//
// CONCEPT : VALIDATION DE FICHIERS
// =================================
// Lors de la récupération de données, beaucoup de fichiers sont corrompus :
// - Fichiers tronqués (récupération partielle)
// - Données corrompues (secteurs défectueux)
// - En-têtes invalides
//
// Ce package détecte ces problèmes AVANT de trier les fichiers,
// évitant de garder des fichiers inutilisables.
package validator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ═══════════════════════════════════════════════════════════════════════════
// TYPES D'ERREURS DE VALIDATION
// ═══════════════════════════════════════════════════════════════════════════

// ValidationError représente une erreur de validation avec des détails.
//
// CONCEPT GO : ERREURS PERSONNALISÉES
// ===================================
// En Go, on peut créer ses propres types d'erreurs en implémentant
// l'interface error (qui a juste une méthode Error() string).
// Cela permet d'ajouter des informations contextuelles.
type ValidationError struct {
	Type    string // Type d'erreur : "truncated", "corrupted", "invalid_header", etc.
	Message string // Message descriptif
	Offset  int64  // Position dans le fichier où l'erreur a été détectée (-1 si N/A)
}

// Error implémente l'interface error.
func (e *ValidationError) Error() string {
	if e.Offset >= 0 {
		return fmt.Sprintf("%s at byte %d: %s", e.Type, e.Offset, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Erreurs courantes pré-définies
var (
	ErrTruncated     = &ValidationError{Type: "truncated", Message: "file is truncated", Offset: -1}
	ErrInvalidHeader = &ValidationError{Type: "invalid_header", Message: "invalid file header", Offset: 0}
	ErrCorrupted     = &ValidationError{Type: "corrupted", Message: "file data is corrupted", Offset: -1}
	ErrEmpty         = &ValidationError{Type: "empty", Message: "file is empty", Offset: -1}
	ErrUnsupported   = &ValidationError{Type: "unsupported", Message: "validation not supported for this type", Offset: -1}
)

// ═══════════════════════════════════════════════════════════════════════════
// RÉSULTAT DE VALIDATION
// ═══════════════════════════════════════════════════════════════════════════

// ValidationResult contient le résultat de la validation d'un fichier.
type ValidationResult struct {
	Path     string            // Chemin du fichier
	Valid    bool              // true si le fichier est valide
	Error    *ValidationError  // Erreur si invalide (nil si valide)
	FileType string            // Type de fichier validé (jpeg, png, mp4, etc.)
	Details  map[string]string // Détails supplémentaires (dimensions, durée, etc.)
}

// ═══════════════════════════════════════════════════════════════════════════
// INTERFACE VALIDATOR
// ═══════════════════════════════════════════════════════════════════════════

// Validator est l'interface que chaque validateur de type doit implémenter.
//
// CONCEPT GO : INTERFACES
// =======================
// Une interface définit un COMPORTEMENT (ensemble de méthodes).
// Tout type qui implémente ces méthodes satisfait automatiquement l'interface.
// C'est du "duck typing" : si ça marche comme un canard, c'est un canard.
//
// Avantage : on peut ajouter de nouveaux validateurs sans modifier le code existant.
type Validator interface {
	// Validate vérifie l'intégrité d'un fichier.
	// Retourne un ValidationResult avec les détails.
	Validate(path string) ValidationResult

	// SupportedExtensions retourne la liste des extensions supportées.
	// Exemple : []string{".jpg", ".jpeg", ".png"}
	SupportedExtensions() []string

	// SupportedMIMETypes retourne la liste des types MIME supportés.
	// Exemple : []string{"image/jpeg", "image/png"}
	SupportedMIMETypes() []string
}

// ═══════════════════════════════════════════════════════════════════════════
// REGISTRE DE VALIDATEURS
// ═══════════════════════════════════════════════════════════════════════════

// registry stocke tous les validateurs enregistrés.
// On utilise une map pour un accès O(1) par extension.
//
// CONCEPT GO : VARIABLES PACKAGE-LEVEL
// ====================================
// Les variables déclarées au niveau du package (hors fonction) sont
// initialisées au démarrage du programme. On les utilise pour les
// singletons et les registres globaux.
var (
	registry     = make(map[string]Validator) // extension -> validator
	registryMIME = make(map[string]Validator) // mime -> validator
	registryMu   sync.RWMutex                 // Mutex pour accès concurrent
)

// Register enregistre un validateur dans le registre global.
//
// CONCEPT GO : INIT ET REGISTRATION PATTERN
// =========================================
// Chaque validateur s'enregistre lui-même via une fonction init().
// C'est le pattern "self-registration" commun en Go.
//
// Exemple dans image.go:
//
//	func init() {
//	    Register(&JPEGValidator{})
//	}
func Register(v Validator) {
	registryMu.Lock()
	defer registryMu.Unlock()

	// Enregistrer par extension
	for _, ext := range v.SupportedExtensions() {
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		registry[ext] = v
	}

	// Enregistrer par type MIME
	for _, mime := range v.SupportedMIMETypes() {
		registryMIME[strings.ToLower(mime)] = v
	}
}

// GetValidatorForExtension retourne le validateur approprié pour une extension.
func GetValidatorForExtension(ext string) (Validator, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	v, ok := registry[ext]
	return v, ok
}

// GetValidatorForMIME retourne le validateur approprié pour un type MIME.
func GetValidatorForMIME(mimeType string) (Validator, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	v, ok := registryMIME[strings.ToLower(mimeType)]
	return v, ok
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTION DE VALIDATION PRINCIPALE
// ═══════════════════════════════════════════════════════════════════════════

// ValidateFile valide un fichier en utilisant le validateur approprié.
//
// La fonction :
// 1. Détermine l'extension du fichier
// 2. Trouve le validateur correspondant
// 3. Exécute la validation
// 4. Retourne le résultat
func ValidateFile(path string) ValidationResult {
	// Vérifier que le fichier existe
	info, err := os.Stat(path)
	if err != nil {
		return ValidationResult{
			Path:  path,
			Valid: false,
			Error: &ValidationError{
				Type:    "access_error",
				Message: err.Error(),
				Offset:  -1,
			},
		}
	}

	// Vérifier que ce n'est pas un dossier
	if info.IsDir() {
		return ValidationResult{
			Path:  path,
			Valid: false,
			Error: &ValidationError{
				Type:    "not_a_file",
				Message: "path is a directory",
				Offset:  -1,
			},
		}
	}

	// Vérifier que le fichier n'est pas vide
	if info.Size() == 0 {
		return ValidationResult{
			Path:  path,
			Valid: false,
			Error: ErrEmpty,
		}
	}

	// Trouver le validateur approprié
	ext := filepath.Ext(path)
	v, ok := GetValidatorForExtension(ext)
	if !ok {
		return ValidationResult{
			Path:     path,
			Valid:    true, // On considère valide si pas de validateur
			FileType: "unknown",
			Details:  map[string]string{"note": "no validator for extension " + ext},
		}
	}

	// Exécuter la validation
	return v.Validate(path)
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION BATCH (PARALLÈLE)
// ═══════════════════════════════════════════════════════════════════════════

// ValidateFiles valide plusieurs fichiers en parallèle.
//
// CONCEPT GO : FAN-OUT/FAN-IN
// ===========================
// Pattern classique de concurrence :
// - Fan-out : distribuer les tâches à plusieurs workers
// - Fan-in : collecter les résultats dans un seul canal
//
// Paramètres :
// - paths : liste des fichiers à valider
// - workers : nombre de goroutines parallèles
// - progress : callback appelé après chaque fichier (peut être nil)
func ValidateFiles(paths []string, workers int, progress func(done, total int)) []ValidationResult {
	if workers < 1 {
		workers = 1
	}

	total := len(paths)
	results := make([]ValidationResult, total)

	// Canal pour distribuer les tâches (index dans paths)
	jobs := make(chan int, workers)

	// WaitGroup pour attendre tous les workers
	var wg sync.WaitGroup

	// Compteur de progression (thread-safe)
	var done int
	var doneMu sync.Mutex

	// Lancer les workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				// Valider le fichier
				results[idx] = ValidateFile(paths[idx])

				// Mettre à jour la progression
				if progress != nil {
					doneMu.Lock()
					done++
					current := done
					doneMu.Unlock()
					progress(current, total)
				}
			}
		}()
	}

	// Envoyer les tâches
	for i := range paths {
		jobs <- i
	}
	close(jobs)

	// Attendre la fin
	wg.Wait()

	return results
}

// ═══════════════════════════════════════════════════════════════════════════
// FILTRES UTILITAIRES
// ═══════════════════════════════════════════════════════════════════════════

// FilterCorrupted retourne uniquement les fichiers invalides.
func FilterCorrupted(results []ValidationResult) []ValidationResult {
	var corrupted []ValidationResult
	for _, r := range results {
		if !r.Valid {
			corrupted = append(corrupted, r)
		}
	}
	return corrupted
}

// FilterValid retourne uniquement les fichiers valides.
func FilterValid(results []ValidationResult) []ValidationResult {
	var valid []ValidationResult
	for _, r := range results {
		if r.Valid {
			valid = append(valid, r)
		}
	}
	return valid
}

// GroupByErrorType regroupe les fichiers invalides par type d'erreur.
func GroupByErrorType(results []ValidationResult) map[string][]ValidationResult {
	groups := make(map[string][]ValidationResult)
	for _, r := range results {
		if !r.Valid && r.Error != nil {
			groups[r.Error.Type] = append(groups[r.Error.Type], r)
		}
	}
	return groups
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════════════════

// NewValidationError crée une nouvelle erreur de validation.
func NewValidationError(errType, message string, offset int64) *ValidationError {
	return &ValidationError{
		Type:    errType,
		Message: message,
		Offset:  offset,
	}
}

// WrapError convertit une erreur Go standard en ValidationError.
func WrapError(err error, errType string) *ValidationError {
	if err == nil {
		return nil
	}

	// Si c'est déjà une ValidationError, la retourner
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve
	}

	return &ValidationError{
		Type:    errType,
		Message: err.Error(),
		Offset:  -1,
	}
}
