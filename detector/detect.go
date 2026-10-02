// Package detector identifie le type de fichier en utilisant plusieurs stratégies.
//
// CONCEPTS CLE :
// - Magic numbers : bytes spécifiques au début du fichier identifiant le type
// - Extensions : suffixe du nom de fichier
// - Patterns : recherche de chaînes caractéristiques dans le contenu
//
// STRATEGIES (dans l'ordre) :
// 1. Magic numbers (binaire) : la plus fiable car basée sur le contenu réel
// 2. Extension (fallback) : plus rapide mais peut être trompeuse
// 3. Pattern matching (fallback) : analyse le contenu texte pour les fichiers texte
//
// Exemple : un fichier .jpg
//   - Magic number : FF D8 FF (bytes spécifiques aux JPEGs)
//   - Extension : jpg
//   - Pattern : non utilisé pour les binaires
package detector

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
)

const maxPatternBytes = 64 * 1024

var patternBufferPool = sync.Pool{
	New: func() any { return new([maxPatternBytes]byte) },
}

// Detect détermine le type de fichier en utilisant plusieurs stratégies.
//
// La fonction essaie les méthodes dans cet ordre :
// 1. Détection par magic numbers (fiabilité maximale)
// 2. Détection par extension (fallback rapide)
// 3. Détection par pattern dans le contenu (fallback pour fichiers texte)
//
// Paramètres :
//   - path : chemin complet vers le fichier à analyser
//
// Retour :
//   - string : l'extension du type de fichier détecté (ex: "jpg", "pdf")
//     ou "unknown" si le type ne peut pas être déterminé
func Detect(path string) string {
	// Essayer d'abord la détection par magic numbers/binaire
	typeOrExt := detectFileType(path)
	if typeOrExt == "" {
		return "unknown"
	}
	// Si trouvé et ce n'est pas un fallback, retourner
	if typeOrExt != "other extension" {
		return typeOrExt
	}

	// Deuxième tentative : analyse du contenu (patterns)
	f, err := os.Open(path)
	if err != nil {
		return "unknown"
	}
	defer f.Close()

	buffer := patternBufferPool.Get().(*[maxPatternBytes]byte)
	defer patternBufferPool.Put(buffer)
	n, err := io.ReadFull(f, buffer[:])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "unknown"
	}
	content := buffer[:n]
	patternType := detectPattern(content)

	if patternType != "other extension" {
		return patternType
	}

	// Valider les JSON volumineux en flux : on lit tout le contenu sans le charger
	// intégralement en mémoire. Les autres patterns restent limités à l'en-tête.
	if looksLikeJSON(content) {
		if _, err := f.Seek(0, io.SeekStart); err == nil && isValidJSONStream(f) {
			return "json"
		}
	}

	return "unknown"
}

func looksLikeJSON(content []byte) bool {
	content = bytes.TrimSpace(content)
	if len(content) == 0 {
		return false
	}
	switch content[0] {
	case '{', '[', '"', 't', 'f', 'n', '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return true
	default:
		return false
	}
}

func isValidJSONStream(r io.Reader) bool {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	token, err := decoder.Token()
	if err != nil {
		return false
	}
	depth := 0
	if delim, ok := token.(json.Delim); ok {
		if delim != '{' && delim != '[' {
			return false
		}
		depth = 1
	}

	for depth > 0 {
		token, err = decoder.Token()
		if err != nil {
			return false
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}

	_, err = decoder.Token()
	return errors.Is(err, io.EOF)
}
