// Package types définit les structures de données principales du projet.
//
// Ce package centralise les types utilisés par tous les autres packages
// pour assurer une cohérence dans la structure des données.
package types

import "FileRecoveryOrganizer/metadata"

// Stats contient les statistiques globales d'un répertoire.
//
// Cette structure accumule les informations basiques sur les fichiers
// trouvés : nombre, taille totale, types identifiés.
type Stats struct {
	TotalFiles       int            // Nombre total de fichiers trouvés
	TotalDirs        int            // Nombre total de répertoires
	TotalSize        int64          // Taille totale en octets
	DetectedFileType map[string]int // Nombre de fichiers par type (ex: map["jpg"] = 150)
}

// ImageExif contient les métadonnées EXIF spécifiques aux images.
//
// EXIF (Exchangeable Image File Format) stocke les métadonnées des photos :
// - Appareil photo qui a pris la photo
// - Date de prise de vue
// - Localisation GPS
// etc.
type ImageExif struct {
	DateTaken    string  `json:"date_taken,omitempty"`   // Date au format EXIF
	CameraModel  string  `json:"camera_model,omitempty"` // Modèle de l'appareil
	GPSLatitude  float64 `json:"lat,omitempty"`          // Latitude GPS
	GPSLongitude float64 `json:"lon,omitempty"`          // Longitude GPS
}

// ImageMeta contient les métadonnées complètes d'une image.
//
// Cette structure regroupe :
// - Propriétés basiques (dimensions)
// - Données EXIF extraites
// - Classification (miniature, asset, etc.)
type ImageMeta struct {
	Width   int        // Largeur en pixels
	Height  int        // Hauteur en pixels
	HasExif bool       // Indique si l'image contient des données EXIF
	Exif    *ImageExif // Pointeur vers les métadonnées EXIF (peut être nil)

	IsThumb bool   // True si c'est une miniature
	IsAsset bool   // True si c'est une image asset (petite, icône, etc.)
	Reason  string // Explication pour la classification
}

// Result représente le résultat du traitement complet d'un fichier.
//
// Cette structure contient TOUTES les informations extraites d'un fichier
// après son détection, enrichissement et classification.
// Elle est le point final du pipeline de traitement.
//
// Chaque résultat peut être sérialisé en JSON pour export.
type Result struct {
	Path       string             `json:"path"`                  // Chemin complet du fichier
	Size       int64              `json:"size"`                  // Taille en octets
	Type       string             `json:"type"`                  // Type détecté (jpg, pdf, mp4, etc.)
	Image      *ImageMeta         `json:"image_exif,omitempty"`  // Métadonnées image (nil si pas une image)
	Error      string             `json:"error,omitempty"`       // Message d'erreur si problème
	TargetPath string             `json:"target_path,omitempty"` // Chemin de destination classifié
	Date       *metadata.FileData `json:"date,omitempty"`        // Métadonnées de date extraites
}
