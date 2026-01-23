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
	patternType := detectPattern([]byte(path))

	if patternType != "other extension" {
		return patternType
	}

	return "unknown"
}
