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
// 1. Si c'est une image :
//   - Si c'est une miniature -> images/thumbnails
//   - Sinon -> images/originals
//
// 2. Sinon : classer basé sur le type (mp4, pdf, jpg, etc.)
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
		r.TargetPath = "images/originals"
		return
	}

	// Classification par type de fichier
	switch r.Type {
	// Vidéos : classer par codec/conteneur
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg":
		r.TargetPath = "videos/" + r.Type

	// Audio : classer par codec
	case "mp3", "flac", "wav", "wma", "au", "wv", "aac", "ogg", "m4a", "opus":
		r.TargetPath = "audio/" + r.Type

	// Playlists audio
	case "m3u", "m3u8", "pls":
		r.TargetPath = "audio/playlists/" + r.Type

	// Images standard
	case "jpg", "jpeg", "png", "gif", "bmp", "tiff", "tif", "webp", "svg", "ico", "jp2", "pcx", "nef":
		r.TargetPath = "images/" + r.Type

	// Design & Graphics (outils et formats propriétaires)
	case "xcf", "ai", "eps", "indd", "cdr", "psd", "sketch", "fig":
		r.TargetPath = "design/" + r.Type

	// Images 3D haute dynamique
	case "hdr", "exr":
		r.TargetPath = "images/3d/" + r.Type

	// Modèles 3D
	case "fbx", "blend", "obj", "stl", "dae", "3ds", "max":
		r.TargetPath = "3d/models/" + r.Type

	// Ebooks
	case "epub", "mobi", "azw", "azw3", "pdf", "djvu", "fb2":
		r.TargetPath = "ebooks"

	// Documents texte
	case "doc", "docx", "txt", "rtf", "odt", "one", "md":
		r.TargetPath = "documents/text/" + r.Type

	// Feuilles de calcul
	case "xls", "xlsx", "csv", "ods":
		r.TargetPath = "documents/spreadsheets/" + r.Type

	// Présentations
	case "ppt", "pptx", "odp":
		r.TargetPath = "documents/presentations/" + r.Type

	// Archives compressées
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "jar", "swc", "cab", "iso":
		r.TargetPath = "archives/" + r.Type

	// Bases de données
	case "db", "sqlite", "mdb", "accdb", "sql":
		r.TargetPath = "databases/" + r.Type

	// Code source et scripts
	case "html", "htm", "css", "json", "xml", "yaml", "yml", "js", "ts", "jsx", "tsx", "py", "pyc", "java", "class", "c", "cpp", "h", "hpp", "go", "rs", "rb", "php", "pl", "sh", "bash", "pm", "jsp":
		r.TargetPath = "code/" + r.Type

	// Exécutables et binaires
	case "exe", "dll", "so", "dylib", "app", "msi", "bat", "cmd", "com":
		r.TargetPath = "executables/" + r.Type

	// Certificats et clés de sécurité
	case "pfx", "p12", "cer", "crt", "pem", "key", "dsa":
		r.TargetPath = "certificates/" + r.Type

	// Configuration système
	case "lnk", "ini", "cfg", "conf", "plist", "reg", "ds_store":
		r.TargetPath = "system/config/" + r.Type

	// Polices d'écriture
	case "ttf", "otf", "woff", "woff2", "eot":
		r.TargetPath = "fonts/" + r.Type

	// Email et contacts
	case "eml", "msg", "mbox", "vcf", "ics":
		r.TargetPath = "contacts/email/" + r.Type

	// Bandes dessinées numériques
	case "cbz", "cbr", "cbt", "cba":
		r.TargetPath = "ebooks/comics/" + r.Type

	// Torrents
	case "torrent":
		r.TargetPath = "torrents/"

	// Profils de couleur
	case "icm", "icc":
		r.TargetPath = "color-profiles"

	// Flash et animation
	case "swf", "fla", "ani":
		r.TargetPath = "animation"

	// Projets vidéo/montage
	case "prproj", "aep", "veg":
		r.TargetPath = "video-projects"

	// Défaut : classer par type
	default:
		r.TargetPath = "others/" + r.Type
	}
}
