// Package classifier catégorise les fichiers selon leur type et les organise en dossiers.
//
// Ce package fournit une logique de classification pour organiser les fichiers
// en catégories logiques et les assigner à des chemins de destination.
//
// STRUCTURE DE DESTINATION :
// - images/thumbnails : miniatures d'images
// - images/originals : images complètes
// - videos/[type] : fichiers vidéo classés par type
// - audio/[type] : fichiers audio classés par type
// - documents/* : documents texte, spreadsheets, présentations
// - design/* : fichiers de conception graphique
// - code/* : fichiers source et scripts
// etc.
//
// ORGANISATION PAR DATE (optionnelle) :
// Quand activée, les fichiers sont organisés en sous-dossiers temporels :
// - images/originals/2024/01/photo.jpg (année/mois)
// - images/originals/2024/photo.jpg (année seule)
// - images/originals/2024/01/15/photo.jpg (année/mois/jour)
package classifier

import (
	"FileRecoveryOrganizer/organizer"
	"FileRecoveryOrganizer/types"
)

// ClassifyOptions contient les options de classification.
//
// Cette structure permet de configurer le comportement de la classification,
// notamment l'organisation par date.
type ClassifyOptions struct {
	// DateOrganization définit comment organiser par date
	// Utiliser organizer.DateNone pour désactiver
	DateOrganization organizer.DateOrganization

	// DateSource définit d'où provient la date
	DateSource organizer.DateSource
}

// DefaultClassifyOptions retourne les options par défaut (sans organisation par date).
func DefaultClassifyOptions() ClassifyOptions {
	return ClassifyOptions{
		DateOrganization: organizer.DateNone,
		DateSource:       organizer.DateSourceAuto,
	}
}

// Classify assigne un chemin de destination à un fichier basé sur son type et ses propriétés.
//
// LOGIQUE :
// 1. Si c'est une image avec métadonnées :
//   - Si c'est une miniature -> images/thumbnails
//   - Si c'est un asset -> images/assets
//   - Sinon -> images/originals
//
// 2. Sinon : utiliser la map categoryMap pour classifier par type
//
// La classification est importante car elle crée une structure organisée :
// - Les utilisateurs trouvent facilement les fichiers
// - Les fichiers similaires sont groupés
// - C'est plus facile à exporter ou traiter en batch
//
// Paramètres :
//   - r : pointeur vers le Result contenant les informations du fichier
//     Cette fonction MODIFIE r en définissant TargetPath
func Classify(r *types.Result) {
	ClassifyWithOptions(r, DefaultClassifyOptions())
}

// ClassifyWithOptions assigne un chemin de destination avec des options personnalisées.
//
// Cette fonction permet d'activer l'organisation par date et d'autres options.
// Elle est plus flexible que Classify() mais nécessite de passer des options.
//
// CONCEPT GO : SURCHARGE DE FONCTION
// ==================================
// Go ne supporte pas la surcharge de fonctions (plusieurs fonctions avec le même nom).
// On utilise donc deux fonctions distinctes :
// - Classify() : version simple avec options par défaut
// - ClassifyWithOptions() : version complète avec options personnalisées
//
// Paramètres :
//   - r : pointeur vers le Result
//   - opts : options de classification
func ClassifyWithOptions(r *types.Result, opts ClassifyOptions) {
	// Étape 1 : Déterminer la catégorie de base
	categoryPath := getCategoryPath(r)

	// Étape 2 : Appliquer l'organisation par date si activée
	if opts.DateOrganization != organizer.DateNone {
		dateOpts := organizer.Options{
			Organization:      opts.DateOrganization,
			Source:            opts.DateSource,
			UnknownDateFolder: "unknown_date",
			IncludeCategory:   true,
		}
		r.TargetPath = organizer.OrganizeByDate(r, categoryPath, dateOpts)
	} else {
		r.TargetPath = categoryPath
	}
}

// getCategoryPath détermine le chemin de catégorie de base (sans date).
//
// Cette fonction interne extrait la logique de catégorisation pour la réutiliser.
func getCategoryPath(r *types.Result) string {
	// Traitement spécial pour les images avec métadonnées EXIF
	if r.Image != nil {
		if r.Image.IsThumb {
			return "images/thumbnails"
		}
		if r.Image.IsAsset {
			return "images/assets"
		}
		return "images/originals"
	}

	// Utiliser la map categoryMap pour classifier par type
	return GetCategory(r.Type)
}
