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
package classifier

import "FileRecoveryOrganizer/types"

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
	// Traitement spécial pour les images avec métadonnées EXIF
	if r.Image != nil {
		if r.Image.IsThumb {
			r.TargetPath = "images/thumbnails"
			return
		}
		if r.Image.IsAsset {
			r.TargetPath = "images/assets"
			return
		}
		r.TargetPath = "images/originals"
		return
	}

	// Utiliser la map categoryMap pour classifier par type
	r.TargetPath = GetCategory(r.Type)
}
