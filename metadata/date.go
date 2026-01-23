package metadata

import "os"

// FileSystemDate extrait la date de modification du système de fichiers.
//
// CONCEPT : Cette fonction montre comment accéder aux métadonnées du système.
// En Go, os.Stat() retourne un os.FileInfo qui contient l'information sur le fichier.
// ModTime() retourne la date de dernière modification.
//
// Paramètres :
//   - path : chemin complet vers le fichier
//
// Retour :
//   - FileData : structure contenant la date, la source et un flag de validité
//     Si une erreur survient, Valid sera false
func FileSystemDate(path string) FileData {
	info, err := os.Stat(path)
	if err != nil {
		// En cas d'erreur, retourner une FileData vide et invalide
		return FileData{Valid: false}
	}

	return FileData{
		Time:   info.ModTime(),
		Source: "filesystem:modification",
		Valid:  true,
	}
}

// BestDate détermine la meilleure date disponible pour un fichier.
//
// PRIORITE :
// 1. Pour les images : essayer d'abord les métadonnées EXIF
// 2. Sinon : utiliser la date du système de fichiers
// 3. Si tout échoue : retourner une structure invalide
//
// Cette fonction encapsule la logique de décision pour choisir la meilleure source
// de date, ce qui est utile car certains fichiers ont plusieurs dates possibles.
//
// Paramètres :
//   - path : chemin vers le fichier
//   - isImage : true si le fichier est une image (cela active la vérification EXIF)
//
// Retour :
//   - FileData : la meilleure date trouvée, ou une structure invalide si aucune n'existe
func BestDate(path string, isImage bool) FileData {
	if isImage {
		// Pour les images, essayer les métadonnées EXIF en priorité
		if d := ImageData(path); d.Valid {
			return d
		}
	}

	// Fallback : utiliser la date du système de fichiers
	if d := FileSystemDate(path); d.Valid {
		return d
	}

	// Aucune date trouvée
	return FileData{Valid: false, Source: "unknown"}
}
