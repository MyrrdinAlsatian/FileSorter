package detector

import (
	"os"

	"github.com/h2non/filetype"
)

func detectFileType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "Error opening file"
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)

	if err != nil {
		return "Error reading file"
	}

	kind, err := filetype.Match(buf[:n])
	if err != nil {
		return "Error detecting file type"
	}
	if kind != filetype.Unknown {
		return kind.Extension
	}
	return "other extension"
}
