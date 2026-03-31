// Package metadata gère l'extraction et le stockage des métadonnées des fichiers.
//
// Ce package fournit :
// - Types pour représenter les données de fichiers (dates, noms, sources)
// - Détection du type de fichier (image, audio, vidéo, etc.)
// - Extraction de métadonnées depuis EXIF (images) et tags ID3 (audio)
// - Détermination de la meilleure date pour un fichier
package metadata

import "time"

// ImageTypes est une map (dictionnaire) contenant tous les types d'images supportées.
//
// En Go, une map[clé]valeur associe des clés à des valeurs.
// Ici, la clé est l'extension (string) et la valeur est toujours true (bool).
// Cela permet une vérification rapide O(1) de si un type est une image.
//
// Exemple :
//
//	ImageTypes["jpg"]  // retourne true
//	ImageTypes["pdf"]  // retourne false
var ImageTypes = map[string]bool{
	"jpg": true, "jpeg": true, "png": true,
	"tiff": true, "heic": true, "heif": true, "webp": true,
	"gif": true, "bmp": true,
}

// VideoTypes est une map contenant tous les types de vidéos supportés.
var VideoTypes = map[string]bool{
	"mp4": true, "m4v": true, "mov": true,
	"mkv": true, "webm": true, "mka": true,
	"avi": true, "wmv": true, "asf": true,
	"flv": true, "3gp": true, "3g2": true,
	"mpeg": true, "mpg": true, "ts": true,
}

// AudioTypes est une map contenant tous les types audio supportés.
var AudioTypes = map[string]bool{
	"mp3": true, "flac": true, "wav": true, "wave": true,
	"ogg": true, "oga": true, "opus": true,
	"aac": true, "m4a": true, "wma": true,
	"aiff": true, "aif": true, "alac": true,
}

// IsImageType vérifie si le type de fichier fourni est une image supportée.
//
// Cette fonction encapsule l'accès à la map ImageTypes.
// Elle existe pour plusieurs raisons :
// 1. Abstraction : le code n'a pas besoin de connaître les détails internes
// 2. Facilité : IsImageType() est plus lisible que ImageTypes[fileType]
// 3. Flexibilité : on peut changer l'implémentation ultérieurement
//
// Paramètres :
//   - fileType : l'extension du fichier (ex: "jpg", "pdf")
//
// Retour :
//   - bool : true si c'est une image supportée, false sinon
func IsImageType(fileType string) bool {
	return ImageTypes[fileType]
}

// IsVideoType vérifie si le type de fichier fourni est une vidéo supportée.
func IsVideoType(fileType string) bool {
	return VideoTypes[fileType]
}

// IsAudioType vérifie si le type de fichier fourni est un fichier audio supporté.
func IsAudioType(fileType string) bool {
	return AudioTypes[fileType]
}

// FileData contient toutes les métadonnées de date et d'identification d'un fichier.
//
// Cette structure regroupe les informations extraites de plusieurs sources
// (système de fichiers, EXIF, tags ID3, etc.).
//
// Les tags JSON (ex: `json:"original_name,omitempty"`) indiquent comment
// sérialiser cette structure en JSON :
// - "original_name" : le nom du champ en JSON
// - "omitempty" : n'inclure le champ que s'il n'est pas vide
type FileData struct {
	// OriginalName est le nom original du fichier (extrait des métadonnées)
	// Exemple : titre du morceau musical, nom du fichier original d'une image
	OriginalName string `json:"original_name,omitempty"`

	// Time est la date/heure du fichier (création ou modification)
	// Type time.Time est le type standard en Go pour les dates
	Time time.Time `json:"time,omitempty"`

	// Source indique d'où vient la métadonnée
	// Exemples : "filesystem:modification", "exif:DateTimeOriginal", "tag:Title"
	Source string `json:"source,omitempty"`

	// Valid indique si les données sont valides et complètes
	// Certaines sources peuvent retourner une structure vide avec Valid=false
	Valid bool `json:"valid,omitempty"`

	// AdditionalInfo peut contenir des informations supplémentaires
	// sous forme de paires clé-valeur (map[string]string)
	AdditionalInfo map[string]string `json:"additional_info,omitempty"`
}
