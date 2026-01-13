package classifier

import "FileRecoveryOrganizer/types"

func Classify(r *types.Result) {
	if r.Image != nil {
		if r.Image.IsThumb {
			r.TargetPath = "images/thumbnails"
			return
		}
		r.TargetPath = "images/originals"
		return
	}

	switch r.Type {
	// Videos
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg":
		r.TargetPath = "videos/" + r.Type

	// Audio
	case "mp3", "flac", "wav", "wma", "au", "wv", "aac", "ogg", "m4a", "opus":
		r.TargetPath = "audio/" + r.Type

	// Playlists
	case "m3u", "m3u8", "pls":
		r.TargetPath = "audio/playlists/" + r.Type

	// Images
	case "jpg", "jpeg", "png", "gif", "bmp", "tiff", "tif", "webp", "svg", "ico", "jp2", "pcx", "nef":
		r.TargetPath = "images/" + r.Type

	// Design & Graphics
	case "xcf", "ai", "eps", "indd", "cdr", "psd", "sketch", "fig":
		r.TargetPath = "design/" + r.Type

	// 3D Images
	case "hdr", "exr":
		r.TargetPath = "images/3d/" + r.Type

	// 3D Models
	case "fbx", "blend", "obj", "stl", "dae", "3ds", "max":
		r.TargetPath = "3d/models/" + r.Type

	// Ebooks
	case "epub", "mobi", "azw", "azw3", "pdf", "djvu", "fb2":
		r.TargetPath = "ebooks"

	// Documents
	case "doc", "docx", "txt", "rtf", "odt", "one", "md":
		r.TargetPath = "documents/text/" + r.Type

	// Spreadsheets
	case "xls", "xlsx", "csv", "ods":
		r.TargetPath = "documents/spreadsheets/" + r.Type

	// Presentations
	case "ppt", "pptx", "odp":
		r.TargetPath = "documents/presentations/" + r.Type
	// Archives
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "jar", "swc", "cab", "iso":
		r.TargetPath = "archives/" + r.Type

	// Databases
	case "db", "sqlite", "mdb", "accdb", "sql":
		r.TargetPath = "databases/" + r.Type

	// Code & Scripts
	case "html", "htm", "css", "json", "xml", "yaml", "yml", "js", "ts", "jsx", "tsx", "py", "pyc", "java", "class", "c", "cpp", "h", "hpp", "go", "rs", "rb", "php", "pl", "sh", "bash", "pm", "jsp":
		r.TargetPath = "code/" + r.Type

	// Executables
	case "exe", "dll", "so", "dylib", "app", "msi", "bat", "cmd", "com":
		r.TargetPath = "executables/" + r.Type

	// Certificates & Security
	case "pfx", "p12", "cer", "crt", "pem", "key", "dsa":
		r.TargetPath = "certificates/" + r.Type

	// System & Config
	case "lnk", "ini", "cfg", "conf", "plist", "reg", "ds_store":
		r.TargetPath = "system/config/" + r.Type

	// Fonts
	case "ttf", "otf", "woff", "woff2", "eot":
		r.TargetPath = "fonts/" + r.Type

	// Email & Contacts
	case "eml", "msg", "mbox", "vcf", "ics":
		r.TargetPath = "contacts/email/" + r.Type

	// Books & Publications
	case "cbz", "cbr", "cbt", "cba":
		r.TargetPath = "ebooks/comics/" + r.Type

	// Torrents
	case "torrent":
		r.TargetPath = "torrents/"

	// Color Profiles
	case "icm", "icc":
		r.TargetPath = "color-profiles"

	// Flash & Animation
	case "swf", "fla", "ani":
		r.TargetPath = "animation"

	// Video Projects
	case "prproj", "aep", "veg":
		r.TargetPath = "video-projects"

	default:
		r.TargetPath = "others/" + r.Type
	}
}
