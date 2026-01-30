// Package organizer gère l'organisation des fichiers selon différents critères.
//
// Ce package fournit des stratégies pour organiser les fichiers :
// - Par date (année/mois)
// - Par type
// - Par catégorie
//
// CONCEPT GO : SÉPARATION DES PRÉOCCUPATIONS
// ==========================================
// Ce package est distinct de "classifier" car :
// - classifier : détermine la CATÉGORIE (images, vidéos, documents)
// - organizer : détermine l'ARBORESCENCE finale (année/mois, etc.)
//
// Cette séparation permet de combiner les deux logiques :
// Par exemple : "images/originals/2024/01" combine catégorie + date
package organizer

import (
	"fmt"
	"path/filepath"
	"time"

	"FileRecoveryOrganizer/metadata"
	"FileRecoveryOrganizer/types"
)

// ═══════════════════════════════════════════════════════════════════════════
// CONSTANTES ET TYPES
// ═══════════════════════════════════════════════════════════════════════════

// DateOrganization définit comment organiser les fichiers par date.
//
// CONCEPT GO : TYPES PERSONNALISÉS
// ================================
// On peut créer des types à partir de types existants.
// `type DateOrganization int` crée un nouveau type basé sur int.
// Cela permet d'avoir des constantes typées et auto-documentées.
type DateOrganization int

const (
	// DateNone : pas d'organisation par date (comportement par défaut)
	DateNone DateOrganization = iota

	// DateYearMonth : organisation en année/mois (2024/01)
	// Format de dossier : {catégorie}/2024/01/fichier.jpg
	DateYearMonth

	// DateYear : organisation par année seulement (2024)
	// Format de dossier : {catégorie}/2024/fichier.jpg
	DateYear

	// DateYearMonthDay : organisation année/mois/jour (2024/01/15)
	// Format de dossier : {catégorie}/2024/01/15/fichier.jpg
	DateYearMonthDay
)

// DateSource définit la source de date à utiliser en priorité.
type DateSource int

const (
	// DateSourceAuto : sélection automatique de la meilleure source
	// Priorité : métadonnées (EXIF/MKV) > date système
	DateSourceAuto DateSource = iota

	// DateSourceMetadata : utiliser uniquement les métadonnées
	// Si pas de métadonnées, le fichier va dans "unknown_date"
	DateSourceMetadata

	// DateSourceFilesystem : utiliser la date du système de fichiers
	// Toujours disponible mais moins fiable
	DateSourceFilesystem
)

// Options contient les options d'organisation par date.
type Options struct {
	// Organization définit le format de l'arborescence temporelle
	Organization DateOrganization

	// Source définit d'où provient la date
	Source DateSource

	// UnknownDateFolder est le nom du dossier pour les fichiers sans date
	// Par défaut : "unknown_date"
	UnknownDateFolder string

	// IncludeCategory inclut la catégorie dans le chemin
	// true : images/originals/2024/01/photo.jpg
	// false : 2024/01/photo.jpg
	IncludeCategory bool
}

// DefaultOptions retourne les options par défaut pour l'organisation par date.
func DefaultOptions() Options {
	return Options{
		Organization:      DateYearMonth,
		Source:            DateSourceAuto,
		UnknownDateFolder: "unknown_date",
		IncludeCategory:   true,
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS PRINCIPALES
// ═══════════════════════════════════════════════════════════════════════════

// OrganizeByDate génère le chemin de destination en incluant l'organisation par date.
//
// CONCEPT GO : FONCTION AVEC OPTIONS
// ==================================
// Plutôt que d'avoir beaucoup de paramètres, on utilise une structure Options.
// Cela rend la fonction plus flexible et évolutive.
//
// Paramètres :
//   - result : le résultat du traitement du fichier (contient les métadonnées)
//   - categoryPath : le chemin de catégorie déterminé par classifier (ex: "images/originals")
//   - opts : options d'organisation
//
// Retour :
//   - string : le chemin final (ex: "images/originals/2024/01")
func OrganizeByDate(result *types.Result, categoryPath string, opts Options) string {
	// Si pas d'organisation par date, retourner le chemin de catégorie tel quel
	if opts.Organization == DateNone {
		return categoryPath
	}

	// Obtenir la date du fichier selon la source configurée
	fileDate := getFileDate(result, opts.Source)

	// Construire le composant de date
	datePath := buildDatePath(fileDate, opts)

	// Combiner catégorie et date selon les options
	if opts.IncludeCategory && categoryPath != "" {
		return filepath.Join(categoryPath, datePath)
	}
	return datePath
}

// getFileDate extrait la date du fichier selon la source configurée.
//
// CONCEPT GO : SWITCH STATEMENT
// =============================
// Le switch en Go est plus puissant qu'en C/Java :
// - Pas besoin de "break" (implicite)
// - Peut switcher sur n'importe quel type comparable
// - Peut avoir des expressions dans les cases
func getFileDate(result *types.Result, source DateSource) time.Time {
	switch source {
	case DateSourceMetadata:
		// Utiliser uniquement les métadonnées
		if result.AdditionalInfo != nil && result.AdditionalInfo.Valid {
			return result.AdditionalInfo.Time
		}
		// Retourner time.Time zéro si pas de métadonnées
		return time.Time{}

	case DateSourceFilesystem:
		// Utiliser la date du système de fichiers
		fsDate := metadata.FileSystemDate(result.Path)
		if fsDate.Valid {
			return fsDate.Time
		}
		return time.Time{}

	case DateSourceAuto:
		fallthrough // Continue vers le default
	default:
		// Mode automatique : priorité aux métadonnées, fallback filesystem
		if result.AdditionalInfo != nil && result.AdditionalInfo.Valid {
			return result.AdditionalInfo.Time
		}
		// Fallback vers la date filesystem
		fsDate := metadata.FileSystemDate(result.Path)
		if fsDate.Valid {
			return fsDate.Time
		}
		return time.Time{}
	}
}

// buildDatePath construit le chemin de date selon le format d'organisation.
//
// Exemples de résultats :
//   - DateYearMonth : "2024/01"
//   - DateYear : "2024"
//   - DateYearMonthDay : "2024/01/15"
//   - Date invalide : "unknown_date"
func buildDatePath(t time.Time, opts Options) string {
	// Vérifier si la date est valide (non-zéro)
	// time.Time{} est la valeur zéro, on la détecte avec IsZero()
	if t.IsZero() {
		return opts.UnknownDateFolder
	}

	// Extraire les composantes de date
	year := t.Year()
	month := int(t.Month())
	day := t.Day()

	// Valider l'année (éviter les dates aberrantes)
	// Les photos numériques datent généralement d'après 1990
	if year < 1990 || year > 2100 {
		return opts.UnknownDateFolder
	}

	// Construire le chemin selon le format demandé
	switch opts.Organization {
	case DateYear:
		return fmt.Sprintf("%d", year)

	case DateYearMonth:
		// %02d formate le mois sur 2 chiffres avec zéro initial (01, 02, ... 12)
		return fmt.Sprintf("%d/%02d", year, month)

	case DateYearMonthDay:
		return fmt.Sprintf("%d/%02d/%02d", year, month, day)

	default:
		return opts.UnknownDateFolder
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS UTILITAIRES
// ═══════════════════════════════════════════════════════════════════════════

// GetDateInfo retourne une description lisible de la date et sa source.
//
// Cette fonction est utile pour le débogage et l'affichage à l'utilisateur.
// Elle indique d'où vient la date utilisée pour organiser le fichier.
//
// Retour :
//   - dateStr : la date formatée (ex: "2024-01-15")
//   - source : la source de la date (ex: "exif:DateTimeOriginal")
//   - valid : true si une date valide a été trouvée
func GetDateInfo(result *types.Result) (dateStr string, source string, valid bool) {
	if result.AdditionalInfo != nil && result.AdditionalInfo.Valid {
		t := result.AdditionalInfo.Time
		if !t.IsZero() {
			return t.Format("2006-01-02"), result.AdditionalInfo.Source, true
		}
	}

	// Fallback filesystem
	fsDate := metadata.FileSystemDate(result.Path)
	if fsDate.Valid && !fsDate.Time.IsZero() {
		return fsDate.Time.Format("2006-01-02"), fsDate.Source, true
	}

	return "", "", false
}

// ParseDateOrganization convertit une chaîne en DateOrganization.
//
// Cette fonction est utile pour parser les arguments de ligne de commande.
// Elle accepte plusieurs formats pour plus de flexibilité.
//
// Exemples d'entrées acceptées :
//   - "year-month", "ym", "yearmonth" → DateYearMonth
//   - "year", "y" → DateYear
//   - "year-month-day", "ymd" → DateYearMonthDay
//   - "none", "off", "" → DateNone
func ParseDateOrganization(s string) DateOrganization {
	switch s {
	case "year-month", "ym", "yearmonth", "YYYY/MM":
		return DateYearMonth
	case "year", "y", "YYYY":
		return DateYear
	case "year-month-day", "ymd", "YYYY/MM/DD":
		return DateYearMonthDay
	case "none", "off", "":
		return DateNone
	default:
		// Par défaut : année/mois
		return DateYearMonth
	}
}

// String retourne une représentation textuelle de DateOrganization.
//
// CONCEPT GO : MÉTHODE STRINGER
// =============================
// En implémentant String() string, notre type satisfait l'interface fmt.Stringer.
// Cela permet à fmt.Println() et %v d'afficher une représentation lisible.
func (d DateOrganization) String() string {
	switch d {
	case DateNone:
		return "none"
	case DateYearMonth:
		return "year-month (YYYY/MM)"
	case DateYear:
		return "year (YYYY)"
	case DateYearMonthDay:
		return "year-month-day (YYYY/MM/DD)"
	default:
		return "unknown"
	}
}
