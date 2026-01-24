package detector

import (
	"FileRecoveryOrganizer/types"
	"path/filepath"
	"strings"
)

// DetectThumbnail vérifie si une image est une miniature (thumbnail).
//
// CONCEPT : Pattern recognition pour classification
// ==================================================
// Une "miniature" est une petite version compressée d'une image pour
// prévisualization rapide. Celles-ci :
// - Sont nommées avec une convention (souvent commençant par 't')
// - Sont très petites en taille
// - N'ont généralement pas de métadonnées EXIF
//
// STRATÉGIES DE DÉTECTION :
// 1. Convention de nommage : le nom commence par 't' (thumbnail)
// 2. Taille : < 50 KB (thumbnails optimisées)
// 3. Pas d'EXIF : les miniatures générées n'en ont pas
//
// EXEMPLES :
// - "thumb_001.jpg" -> Détecté comme miniature
// - "t_preview.jpg" -> Détecté comme miniature
// - "photo.jpg" -> Non détecté (pas de 't' au début)
//
// UTILISATION :
// Cette fonction est appelée après EnrichImage pour les fichiers images.
// Permet de séparer les miniatures des photos originales.
//
// Paramètres :
//   - path : chemin du fichier (utilisé pour extraire le nom de fichier)
//   - size : taille du fichier en octets
//   - img : pointeur vers ImageMeta contenant les métadonnées
//
// Retour :
//   - bool : true si c'est probablement une miniature, false sinon
//
// Note : Cette fonction MODIFIE img en définissant IsThumb et Reason
func DetectThumbnail(path string, size int64, img *types.ImageMeta) bool {

	if img == nil {
		return false
	}

	// filepath.Base extrait juste le nom de fichier du chemin complet
	// Exemple : "/photos/thumb_001.jpg" -> "thumb_001.jpg"
	name := strings.ToLower(filepath.Base(path))

	// Vérifier les conditions pour une miniature :
	// 1. Pas de métadonnées EXIF (générées, pas en photo)
	// 2. Nom commence par 't' (convention "thumb_" ou "t_")
	// 3. Taille < 50 KB (fichier compressé/optimisé)
	if !img.HasExif && strings.HasPrefix(name, "t") {
		if size < 50_000 {
			img.IsThumb = true
			img.Reason = "Filename starts with 't' and size < 50KB without EXIF"
			return true
		}
	}

	return false
}
