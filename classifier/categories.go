// Package classifier définit les catégories et mappings de types de fichiers.
//
// Ce fichier contient la MAP CENTRALE de classification des fichiers.
// Chaque extension de fichier est associée à un chemin de destination.
package classifier

// categoryMap associe chaque extension à son chemin de destination.
//
// CONCEPT GO : LES MAPS (DICTIONNAIRES)
// =====================================
// Une map en Go est une structure clé-valeur, similaire aux :
// - dictionnaires en Python
// - HashMap en Java
// - objets en JavaScript
//
// SYNTAXE DE DÉCLARATION :
//
//	var m map[string]int                    // Déclaration (nil, inutilisable)
//	m := make(map[string]int)               // Initialisation vide
//	m := map[string]int{"a": 1, "b": 2}     // Initialisation avec valeurs
//
// OPÉRATIONS COURANTES :
//
//	m["key"] = value                        // Ajouter/modifier
//	value := m["key"]                       // Lire (retourne valeur zéro si absent)
//	value, exists := m["key"]               // Lire avec vérification d'existence
//	delete(m, "key")                        // Supprimer
//	len(m)                                  // Nombre d'éléments
//
// POURQUOI UNE MAP PLUTÔT QU'UN SWITCH ?
// ======================================
// 1. Plus MAINTENABLE : ajouter un type = ajouter une ligne
// 2. Plus EXTENSIBLE : on peut modifier la map à runtime
// 3. Plus PERFORMANT : accès O(1) vs O(n) pour un switch
// 4. Plus TESTABLE : on peut facilement vérifier le contenu
//
// VARIABLE GLOBALE AU PACKAGE :
// Cette variable est accessible depuis tous les fichiers du package classifier
// mais pas depuis les autres packages (minuscule = privé).
var categoryMap = map[string]string{
	// ═══════════════════════════════════════════════════════════════════════
	// VIDÉOS - Conteneurs vidéo courants
	// ═══════════════════════════════════════════════════════════════════════
	"mp4": "videos/mp4", "mkv": "videos/mkv", "avi": "videos/avi",
	"mov": "videos/mov", "wmv": "videos/wmv", "flv": "videos/flv",
	"webm": "videos/webm", "m4v": "videos/m4v", "mpg": "videos/mpg",
	"mpeg": "videos/mpeg", "3gp": "videos/3gp",

	// ═══════════════════════════════════════════════════════════════════════
	// AUDIO - Formats audio courants
	// ═══════════════════════════════════════════════════════════════════════
	"mp3": "audio/mp3", "flac": "audio/flac", "wav": "audio/wav",
	"wma": "audio/wma", "au": "audio/au", "wv": "audio/wv",
	"aac": "audio/aac", "ogg": "audio/ogg", "m4a": "audio/m4a",
	"opus": "audio/opus", "aiff": "audio/aiff", "mid": "audio/mid",
	"midi": "audio/midi",

	// Playlists audio
	"m3u": "audio/playlists/m3u", "m3u8": "audio/playlists/m3u8",
	"pls": "audio/playlists/pls",

	// Images standard
	"jpg": "images/jpg", "jpeg": "images/jpeg", "png": "images/png",
	"gif": "images/gif", "bmp": "images/bmp", "tiff": "images/tiff",
	"tif": "images/tif", "webp": "images/webp", "svg": "images/svg",
	"ico": "images/ico", "jp2": "images/jp2", "pcx": "images/pcx",
	"nef": "images/raw/nef", "cr2": "images/raw/cr2", "arw": "images/raw/arw",
	"dng": "images/raw/dng", "orf": "images/raw/orf", "rw2": "images/raw/rw2",

	// Design & Graphics
	"xcf": "design/xcf", "ai": "design/ai", "eps": "design/eps",
	"indd": "design/indd", "cdr": "design/cdr", "psd": "design/psd",
	"sketch": "design/sketch", "fig": "design/fig", "xd": "design/xd",

	// Images 3D haute dynamique
	"hdr": "images/3d/hdr", "exr": "images/3d/exr",

	// Modèles 3D
	"fbx": "3d/models/fbx", "blend": "3d/models/blend", "obj": "3d/models/obj",
	"stl": "3d/models/stl", "dae": "3d/models/dae", "3ds": "3d/models/3ds",
	"max": "3d/models/max", "c4d": "3d/models/c4d", "ma": "3d/models/ma",
	"mb": "3d/models/mb",

	// Ebooks
	"epub": "ebooks/epub", "mobi": "ebooks/mobi", "azw": "ebooks/azw",
	"azw3": "ebooks/azw3", "djvu": "ebooks/djvu", "fb2": "ebooks/fb2",

	// PDF (catégorie séparée car peut être doc ou ebook)
	"pdf": "documents/pdf",

	// Documents texte
	"doc": "documents/text/doc", "docx": "documents/text/docx",
	"txt": "documents/text/txt", "rtf": "documents/text/rtf",
	"odt": "documents/text/odt", "one": "documents/text/one",
	"md": "documents/text/md", "pages": "documents/text/pages",

	// Feuilles de calcul
	"xls": "documents/spreadsheets/xls", "xlsx": "documents/spreadsheets/xlsx",
	"csv": "documents/spreadsheets/csv", "ods": "documents/spreadsheets/ods",
	"numbers": "documents/spreadsheets/numbers",

	// Présentations
	"ppt": "documents/presentations/ppt", "pptx": "documents/presentations/pptx",
	"odp": "documents/presentations/odp", //"key": "documents/presentations/key",

	// Archives compressées
	"zip": "archives/zip", "rar": "archives/rar", "7z": "archives/7z",
	"tar": "archives/tar", "gz": "archives/gz", "bz2": "archives/bz2",
	"xz": "archives/xz", "jar": "archives/jar", "swc": "archives/swc",
	"cab": "archives/cab", "iso": "archives/iso", "dmg": "archives/dmg",
	"lz": "archives/lz", "zst": "archives/zst",

	// Bases de données
	"db": "databases/db", "sqlite": "databases/sqlite", "sqlite3": "databases/sqlite3",
	"mdb": "databases/mdb", "accdb": "databases/accdb", "sql": "databases/sql",

	// Code source et scripts
	"html": "code/web/html", "htm": "code/web/htm", "css": "code/web/css",
	"js": "code/web/js", "jsx": "code/web/jsx", "ts": "code/web/ts",
	"tsx": "code/web/tsx", "vue": "code/web/vue", "svelte": "code/web/svelte",
	"json": "code/data/json", "xml": "code/data/xml", "yaml": "code/data/yaml",
	"yml": "code/data/yml", "toml": "code/data/toml",
	"py": "code/python/py", "pyc": "code/python/pyc", "pyw": "code/python/pyw",
	"java": "code/java/java", "class": "code/java/class", "kt": "code/kotlin/kt",
	"c": "code/c/c", "cpp": "code/cpp/cpp", "h": "code/c/h", "hpp": "code/cpp/hpp",
	"go": "code/go/go", "rs": "code/rust/rs", "rb": "code/ruby/rb",
	"php": "code/php/php", "pl": "code/perl/pl", "pm": "code/perl/pm",
	"sh": "code/shell/sh", "bash": "code/shell/bash", "zsh": "code/shell/zsh",
	"ps1": "code/powershell/ps1", "bat": "code/batch/bat", "cmd": "code/batch/cmd",
	"swift": "code/swift/swift", "scala": "code/scala/scala",
	"r": "code/r/r", "lua": "code/lua/lua", "dart": "code/dart/dart",
	"ex": "code/elixir/ex", "exs": "code/elixir/exs",
	"clj": "code/clojure/clj", "hs": "code/haskell/hs",

	// Exécutables et binaires
	"exe": "executables/exe", "dll": "executables/dll", "so": "executables/so",
	"dylib": "executables/dylib", "app": "executables/app", "msi": "executables/msi",
	"deb": "executables/deb", "rpm": "executables/rpm", "apk": "executables/apk",
	"ipa": "executables/ipa",

	// Certificats et clés de sécurité
	"pfx": "certificates/pfx", "p12": "certificates/p12", "cer": "certificates/cer",
	"crt": "certificates/crt", "pem": "certificates/pem", "key": "certificates/key",
	"dsa": "certificates/dsa", "pub": "certificates/pub", "ppk": "certificates/ppk",

	// Configuration système
	"lnk": "system/shortcuts/lnk", "ini": "system/config/ini",
	"cfg": "system/config/cfg", "conf": "system/config/conf",
	"plist": "system/config/plist", "reg": "system/registry/reg",
	"ds_store": "system/macos/ds_store",

	// Polices d'écriture
	"ttf": "fonts/ttf", "otf": "fonts/otf", "woff": "fonts/woff",
	"woff2": "fonts/woff2", "eot": "fonts/eot", "fon": "fonts/fon",

	// Email et contacts
	"eml": "contacts/email/eml", "msg": "contacts/email/msg",
	"mbox": "contacts/email/mbox", "vcf": "contacts/vcf", "ics": "contacts/ics",
	"pst": "contacts/email/pst", "ost": "contacts/email/ost",

	// Bandes dessinées numériques
	"cbz": "ebooks/comics/cbz", "cbr": "ebooks/comics/cbr",
	"cbt": "ebooks/comics/cbt", "cba": "ebooks/comics/cba",
	"cb7": "ebooks/comics/cb7",

	// Torrents
	"torrent": "torrents/torrent",

	// Profils de couleur
	"icm": "color-profiles/icm", "icc": "color-profiles/icc",

	// Flash et animation
	"swf": "animation/swf", "fla": "animation/fla", "ani": "animation/ani",
	// "gif": "images/gif", // GIF animés aussi dans images

	// Projets vidéo/montage
	"prproj": "video-projects/premiere", "aep": "video-projects/aftereffects",
	"veg": "video-projects/vegas", "drp": "video-projects/resolve",
	"fcpx": "video-projects/finalcut",

	// Projets audio/DAW
	"als": "audio-projects/ableton", "flp": "audio-projects/flstudio",
	"logic": "audio-projects/logic", "ptx": "audio-projects/protools",
	"cpr": "audio-projects/cubase",

	// Jeux et émulation
	"sav": "games/saves/sav", "rom": "games/roms/rom",
	"nes": "games/roms/nes", "snes": "games/roms/snes",
	"gba": "games/roms/gba", "nds": "games/roms/nds",
	"n64": "games/roms/n64", "gb": "games/roms/gb",
	"gbc": "games/roms/gbc",

	// Machines virtuelles
	"vmdk": "virtualization/vmdk", "vdi": "virtualization/vdi",
	"vhd": "virtualization/vhd", "vhdx": "virtualization/vhdx",
	"ova": "virtualization/ova", "ovf": "virtualization/ovf",
	"qcow2": "virtualization/qcow2",

	// Fichiers système Windows
	"sys": "system/windows/sys", "drv": "system/windows/drv",
	"ocx": "system/windows/ocx", "cpl": "system/windows/cpl",

	// Logs
	"log": "logs/log",

	// Sauvegardes
	"bak": "backups/bak", "backup": "backups/backup",
	"old": "backups/old", "orig": "backups/orig",
}

// GetCategory retourne la catégorie pour un type de fichier donné.
//
// CONCEPT GO : VÉRIFICATION D'EXISTENCE DANS UNE MAP
// ===================================================
// En Go, lire une map peut retourner 1 ou 2 valeurs :
//
//	value := m["key"]              // Retourne la valeur (ou valeur zéro si absent)
//	value, exists := m["key"]      // Retourne la valeur ET un bool d'existence
//
// C'est utile pour distinguer "la clé existe avec valeur zéro" de "la clé n'existe pas".
//
// Exemple :
//
//	m := map[string]int{"a": 0, "b": 1}
//	v := m["a"]           // v = 0 (existe)
//	v = m["c"]            // v = 0 (n'existe pas, mais même résultat !)
//	v, ok := m["a"]       // v = 0, ok = true
//	v, ok = m["c"]        // v = 0, ok = false (on peut distinguer !)
//
// Paramètres :
//   - fileType : l'extension du fichier (ex: "mp4", "jpg", "pdf")
//
// Retour :
//   - string : le chemin de destination, ou "others/<type>" si inconnu
func GetCategory(fileType string) string {
	if path, exists := categoryMap[fileType]; exists {
		return path
	}
	return "others/" + fileType
}

// RegisterCategory permet d'ajouter dynamiquement une nouvelle catégorie.
//
// CONCEPT GO : MODIFIER UNE MAP À RUNTIME
// =======================================
// Les maps Go sont mutables : on peut ajouter/modifier/supprimer des entrées
// à tout moment. Cela permet d'étendre les catégories sans recompiler.
//
// ATTENTION : Cette fonction n'est PAS thread-safe !
// Si plusieurs goroutines appellent RegisterCategory en même temps,
// il peut y avoir des "race conditions". En production, il faudrait
// ajouter un mutex (sync.Mutex) pour protéger la map.
//
// Exemple d'utilisation :
//
//	classifier.RegisterCategory("custom", "mes-fichiers/custom")
//
// Paramètres :
//   - fileType : l'extension à enregistrer
//   - targetPath : le chemin de destination
func RegisterCategory(fileType, targetPath string) {
	categoryMap[fileType] = targetPath
}

// GetAllCategories retourne une COPIE de la map des catégories.
//
// CONCEPT GO : COPIER UNE MAP
// ===========================
// En Go, assigner une map ne crée PAS une copie, mais une référence :
//
//	m1 := map[string]int{"a": 1}
//	m2 := m1        // m2 pointe vers la MÊME map que m1
//	m2["a"] = 2     // Modifie aussi m1 !
//
// Pour créer une vraie copie, il faut itérer et copier chaque élément.
// C'est ce que fait cette fonction.
//
// POURQUOI RETOURNER UNE COPIE ?
// On ne veut pas que l'appelant puisse modifier notre map interne.
// C'est un principe d'encapsulation important.
//
// Retour :
//   - map[string]string : une copie indépendante de categoryMap
func GetAllCategories() map[string]string {
	// make() avec une capacité initiale évite les réallocations
	// len(categoryMap) = nombre d'éléments actuels
	copy := make(map[string]string, len(categoryMap))
	for k, v := range categoryMap {
		copy[k] = v
	}
	return copy
}
