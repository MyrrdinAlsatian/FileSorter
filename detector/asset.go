package detector

import "FileRecoveryOrganizer/types"

// DetectAssetImg vérifie si une image est un "asset" (petit fichier graphique).
//
// CONCEPT : Classification heuristique
// =====================================
// Un "asset" est un petit fichier graphique utilisé comme composant graphique :
// - Icônes (16x16, 32x32, 64x64)
// - Sprites (petites images)
// - Éléments graphiques
//
// Ces fichiers sont différents des photos/images complètes et méritent
// une classification séparée pour l'organisation.
//
// HEURISTIQUES UTILISÉES :
// 1. Les assets ont rarement de métadonnées EXIF (pas de camera)
// 2. Les assets sont très petits en dimensions (< 128x128)
// 3. Les assets ont une petite taille en octets (< 10 KB)
//
// LIMITATION :
// - Ce sont des heuristiques, pas des certitudes
// - Un photo compressée peut être petit
// - Mais combinées, elles sont assez fiables
//
// Paramètres :
//   - path : chemin du fichier (non utilisé actuellement, mais pour future extensibilité)
//   - size : taille du fichier en octets
//   - img : pointeur vers ImageMeta contenant les dimensions et données EXIF
//
// Retour :
//   - bool : true si c'est probablement un asset, false sinon
//
// Note : Cette fonction MODIFIE img en définissant IsAsset et Reason
func DetectAssetImg(path string, size int64, img *types.ImageMeta) bool {

	if img == nil {
		return false
	}

	// Si l'image a des métadonnées EXIF, ce n'est probablement pas un asset
	// (les assets sont générés, pas pris en photo)
	if img.HasExif {
		return false
	}

	// Heuristique 1 : dimensions très petites (< 128x128)
	// C'est typique des icônes et sprites
	if img.Width <= 128 && img.Height <= 128 {
		img.IsAsset = true
		img.Reason = "Image dimensions are typical for asset images"
		return true
	}

	// Heuristique 2 : fichier très petit (< 10 KB)
	// Les assets sont généralement optimisés et compressés
	if size < 10_000 {
		img.IsAsset = true
		img.Reason = "Image file size is typical for asset images"
		return true
	}

	return false
}
