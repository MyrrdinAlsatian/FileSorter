// Package renamer gère le renommage intelligent des fichiers selon des patterns.
//
// Ce package permet de renommer les fichiers en utilisant des variables
// extraites des métadonnées (date, appareil photo, etc.) et des séquences.
//
// CONCEPT : PATTERNS DE RENOMMAGE
// ===============================
// Un pattern est une chaîne contenant des variables entre accolades.
// Exemple : "{date}_{camera}_{seq:4}.{ext}"
// Résultat : "2024-01-15_Canon_0001.jpg"
package renamer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════
// TOKENS DE PATTERN
// ═══════════════════════════════════════════════════════════════════════════

// TokenType représente le type d'un token dans un pattern.
type TokenType int

const (
	TokenLiteral  TokenType = iota // Texte brut
	TokenVariable                  // Variable {xxx}
)

// Token représente un élément d'un pattern parsé.
type Token struct {
	Type    TokenType
	Value   string            // Pour Literal : le texte, pour Variable : le nom
	Options map[string]string // Options de la variable (ex: width pour seq)
}

// ═══════════════════════════════════════════════════════════════════════════
// PATTERN PARSÉ
// ═══════════════════════════════════════════════════════════════════════════

// Pattern représente un pattern de renommage parsé.
type Pattern struct {
	Raw    string  // Pattern original
	Tokens []Token // Tokens parsés
}

// ═══════════════════════════════════════════════════════════════════════════
// PARSING DE PATTERNS
// ═══════════════════════════════════════════════════════════════════════════

// variableRegex capture les variables du type {name} ou {name:option}
var variableRegex = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)(?::([^}]+))?\}`)

// ParsePattern parse un pattern de renommage et retourne sa représentation structurée.
//
// Syntaxe supportée :
//   - Texte brut : copié tel quel
//   - {variable} : remplacée par la valeur
//   - {variable:option} : avec option (ex: {seq:4} pour séquence sur 4 chiffres)
//
// Variables supportées :
//   - {date}      : Date au format YYYY-MM-DD
//   - {datetime}  : Date et heure YYYY-MM-DD_HHMMSS
//   - {year}      : Année (4 chiffres)
//   - {month}     : Mois (2 chiffres)
//   - {day}       : Jour (2 chiffres)
//   - {hour}      : Heure (2 chiffres)
//   - {minute}    : Minute (2 chiffres)
//   - {second}    : Seconde (2 chiffres)
//   - {camera}    : Modèle de l'appareil photo (EXIF)
//   - {original}  : Nom original du fichier (sans extension)
//   - {ext}       : Extension du fichier
//   - {type}      : Type détecté (jpg, mp4, etc.)
//   - {category}  : Catégorie (images, videos, etc.)
//   - {hash:N}    : N premiers caractères du hash
//   - {seq:N}     : Numéro de séquence sur N chiffres
//   - {size}      : Taille en bytes
//   - {width}     : Largeur de l'image
//   - {height}    : Hauteur de l'image
func ParsePattern(pattern string) (*Pattern, error) {
	p := &Pattern{
		Raw:    pattern,
		Tokens: make([]Token, 0),
	}

	// Position courante dans le pattern
	pos := 0

	// Trouver toutes les variables
	matches := variableRegex.FindAllStringSubmatchIndex(pattern, -1)

	for _, match := range matches {
		// match[0]:match[1] = position complète de {variable:option}
		// match[2]:match[3] = position du nom de variable
		// match[4]:match[5] = position de l'option (ou -1 si pas d'option)

		// Ajouter le texte littéral avant cette variable
		if match[0] > pos {
			p.Tokens = append(p.Tokens, Token{
				Type:  TokenLiteral,
				Value: pattern[pos:match[0]],
			})
		}

		// Extraire le nom de la variable
		varName := pattern[match[2]:match[3]]

		// Extraire l'option si présente
		options := make(map[string]string)
		if match[4] != -1 && match[5] != -1 {
			optionStr := pattern[match[4]:match[5]]
			// Parser les options (format: key=value ou juste value)
			options = parseOptions(varName, optionStr)
		}

		// Ajouter le token variable
		p.Tokens = append(p.Tokens, Token{
			Type:    TokenVariable,
			Value:   strings.ToLower(varName),
			Options: options,
		})

		pos = match[1]
	}

	// Ajouter le texte restant après la dernière variable
	if pos < len(pattern) {
		p.Tokens = append(p.Tokens, Token{
			Type:  TokenLiteral,
			Value: pattern[pos:],
		})
	}

	return p, nil
}

// parseOptions parse les options d'une variable.
//
// Pour la plupart des variables, l'option est une valeur simple (ex: {seq:4}).
// Pour certaines, on pourrait avoir key=value, mais pour l'instant on garde simple.
func parseOptions(varName, optionStr string) map[string]string {
	options := make(map[string]string)

	switch strings.ToLower(varName) {
	case "seq":
		// {seq:4} = séquence sur 4 chiffres
		if width, err := strconv.Atoi(optionStr); err == nil {
			options["width"] = strconv.Itoa(width)
		} else {
			options["width"] = "4" // défaut
		}

	case "hash":
		// {hash:8} = 8 premiers caractères du hash
		if length, err := strconv.Atoi(optionStr); err == nil {
			options["length"] = strconv.Itoa(length)
		} else {
			options["length"] = "8" // défaut
		}

	case "date":
		// {date:YYYY/MM/DD} = format personnalisé
		options["format"] = optionStr

	case "datetime":
		// {datetime:YYYY-MM-DD_HH-mm-ss}
		options["format"] = optionStr

	default:
		// Option générique
		options["value"] = optionStr
	}

	return options
}

// ═══════════════════════════════════════════════════════════════════════════
// VALIDATION DE PATTERNS
// ═══════════════════════════════════════════════════════════════════════════

// ValidVariables contient les noms de variables valides.
var ValidVariables = map[string]string{
	"date":     "Date au format YYYY-MM-DD",
	"datetime": "Date et heure YYYY-MM-DD_HHMMSS",
	"year":     "Année (4 chiffres)",
	"month":    "Mois (2 chiffres)",
	"day":      "Jour (2 chiffres)",
	"hour":     "Heure (2 chiffres)",
	"minute":   "Minute (2 chiffres)",
	"second":   "Seconde (2 chiffres)",
	"camera":   "Modèle appareil photo (EXIF)",
	"original": "Nom original sans extension",
	"ext":      "Extension du fichier",
	"type":     "Type détecté (jpg, mp4...)",
	"category": "Catégorie (images, videos...)",
	"hash":     "Hash du fichier (tronqué)",
	"seq":      "Numéro de séquence",
	"size":     "Taille en bytes",
	"width":    "Largeur image",
	"height":   "Hauteur image",
}

// Validate vérifie qu'un pattern est valide.
func (p *Pattern) Validate() error {
	for _, token := range p.Tokens {
		if token.Type == TokenVariable {
			if _, ok := ValidVariables[token.Value]; !ok {
				return fmt.Errorf("variable inconnue: {%s}", token.Value)
			}
		}
	}
	return nil
}

// GetVariables retourne la liste des variables utilisées dans le pattern.
func (p *Pattern) GetVariables() []string {
	vars := make([]string, 0)
	for _, token := range p.Tokens {
		if token.Type == TokenVariable {
			vars = append(vars, token.Value)
		}
	}
	return vars
}

// HasVariable vérifie si le pattern contient une variable spécifique.
func (p *Pattern) HasVariable(name string) bool {
	for _, token := range p.Tokens {
		if token.Type == TokenVariable && token.Value == name {
			return true
		}
	}
	return false
}

// String retourne une représentation lisible du pattern.
func (p *Pattern) String() string {
	return p.Raw
}

// ═══════════════════════════════════════════════════════════════════════════
// PATTERNS PRÉDÉFINIS
// ═══════════════════════════════════════════════════════════════════════════

// PredefinedPatterns contient des patterns prêts à l'emploi.
var PredefinedPatterns = map[string]string{
	"simple":   "{original}.{ext}",
	"dated":    "{date}_{original}.{ext}",
	"photo":    "{date}_{camera}_{seq:4}.{ext}",
	"video":    "{year}/{month}/{original}.{ext}",
	"hash":     "{hash:8}_{original}.{ext}",
	"sequence": "{seq:6}.{ext}",
	"full":     "{year}/{month}/{day}/{datetime}_{camera}_{seq:3}.{ext}",
}

// GetPredefinedPattern retourne un pattern prédéfini par son nom.
func GetPredefinedPattern(name string) (string, bool) {
	pattern, ok := PredefinedPatterns[name]
	return pattern, ok
}

// ListPredefinedPatterns retourne la liste des patterns prédéfinis.
func ListPredefinedPatterns() map[string]string {
	// Retourner une copie pour éviter les modifications
	copy := make(map[string]string)
	for k, v := range PredefinedPatterns {
		copy[k] = v
	}
	return copy
}
