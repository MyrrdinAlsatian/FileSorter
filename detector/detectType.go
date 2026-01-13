package detector

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/h2non/filetype"
)

var headerSize = 512 //512 bytes is enough for filetype detection

// bufferPool is a sync.Pool that provides temporary byte slices for file type detection.
// It helps reduce memory allocation overhead by reusing byte slices.
// The New function initializes a new byte slice of headerSize when needed.
// This improves performance, especially when detecting file types for multiple files.
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, headerSize)
	},
}

func detectFileType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "Error opening file"
	}
	defer f.Close()

	buf := bufferPool.Get().([]byte)
	n, err := f.Read(buf)

	defer bufferPool.Put(buf)

	if err != nil && err != io.EOF || n == 0 {
		bufferPool.Put(buf)
		return "Error reading file"
	}

	kind, err := filetype.Match(buf[:n])
	if err != nil {
		return "Error detecting file type"
	}
	if kind != filetype.Unknown {
		return kind.Extension
	}
	// Safe extension extraction
	ext := filepath.Ext(path)
	if len(ext) > 1 {
		return ext[1:] // return extension without dot
	}
	return detectPattern(buf[:n]) // fallback to pattern detection
}
