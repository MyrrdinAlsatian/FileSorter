// Package checkpoint gère la sauvegarde et la reprise des scans interrompus.
//
// CONCEPT : CHECKPOINT / RESUME
// =============================
// Lors d'un scan de millions de fichiers, il est utile de pouvoir :
// 1. Interrompre le scan (Ctrl+C ou fermeture)
// 2. Reprendre là où on s'était arrêté
//
// STRATÉGIE :
// - On utilise le fichier JSONL existant comme source de vérité
// - Au démarrage avec --resume, on charge tous les chemins déjà traités
// - Pendant le scan, on ignore ces fichiers
//
// Avantage : pas de fichier supplémentaire à gérer !
package checkpoint

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// ProcessedFiles garde la trace des fichiers déjà traités.
// Thread-safe pour utilisation avec goroutines.
type ProcessedFiles struct {
	mu    sync.RWMutex
	files map[string]bool
	count int
}

// NewProcessedFiles crée un nouveau tracker de fichiers traités.
func NewProcessedFiles() *ProcessedFiles {
	return &ProcessedFiles{
		files: make(map[string]bool),
	}
}

// resultEntry représente une entrée minimale du JSONL (juste le path).
type resultEntry struct {
	Path string `json:"path"`
}

// LoadFromJSONL charge les chemins de fichiers déjà traités depuis un fichier JSONL.
//
// Cette fonction lit le fichier JSONL ligne par ligne et extrait les chemins.
// Elle est optimisée pour gérer de gros fichiers (lecture streaming).
//
// Paramètres :
//   - path : chemin du fichier JSONL existant
//
// Retour :
//   - error : erreur si le fichier n'existe pas ou n'est pas lisible
func (pf *ProcessedFiles) LoadFromJSONL(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fichier n'existe pas = rien à charger, pas d'erreur
			return nil
		}
		return fmt.Errorf("impossible d'ouvrir %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Augmenter le buffer pour les longues lignes
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // Max 1MB par ligne

	pf.mu.Lock()
	defer pf.mu.Unlock()

	lineCount := 0
	for scanner.Scan() {
		lineCount++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry resultEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			// Ignorer les lignes mal formées
			continue
		}

		if entry.Path != "" {
			pf.files[entry.Path] = true
			pf.count++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("erreur de lecture: %w", err)
	}

	return nil
}

// IsProcessed vérifie si un fichier a déjà été traité.
// Thread-safe.
func (pf *ProcessedFiles) IsProcessed(path string) bool {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.files[path]
}

// Count retourne le nombre de fichiers déjà traités.
func (pf *ProcessedFiles) Count() int {
	pf.mu.RLock()
	defer pf.mu.RUnlock()
	return pf.count
}

// Add marque un fichier comme traité.
// Thread-safe.
func (pf *ProcessedFiles) Add(path string) {
	pf.mu.Lock()
	defer pf.mu.Unlock()
	if !pf.files[path] {
		pf.files[path] = true
		pf.count++
	}
}
