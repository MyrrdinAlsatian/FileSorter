package metadata

import "os"

func FileSystemDate(path string) FileData {
	info, err := os.Stat(path)
	if err != nil {
		return FileData{Valid: false}
	}

	return FileData{
		Time:   info.ModTime(),
		Source: "filesystem:modification",
		Valid:  true,
	}
}

func BestDate(path string, isImage bool) FileData {
	if isImage {
		if d := ImageData(path); d.Valid {
			return d
		}
	}

	if d := FileSystemDate(path); d.Valid {
		return d
	}

	return FileData{Valid: false, Source: "unknown"}
}
