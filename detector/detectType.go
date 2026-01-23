package detector

import (
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/h2non/filetype"
)

// headerSize définit le nombre de bytes à lire pour la détection
// 512 bytes est généralement suffisant pour identifier les magic numbers
const headerSize = 512

// bufferPool est un sync.Pool qui fournit des slices de bytes réutilisables.
//
// CONCEPT : Un sync.Pool est une collection thread-safe d'objets réutilisables.
// Au lieu d'allouer un nouveau []byte à chaque fois, on en réutilise un existant.
//
// AVANTAGES :
// - Réduit les allocations mémoire
// - Réduit la pression sur le garbage collector
// - Améliore les performances, surtout avec beaucoup de fichiers
//
// FONCTIONNEMENT :
// - Get() : prend un buffer du pool ou en crée un nouveau
// - Put() : remet le buffer dans le pool pour réutilisation
var bufferPool = sync.Pool{
	New: func() any {
		return make([]byte, headerSize)
	},
}

// detectFileType détecte le type de fichier en analysant ses magic numbers.
//
// Les magic numbers sont des bytes caractéristiques au début d'un fichier
// qui l'identifient de manière unique :
// - PDF : %PDF
// - JPEG : FF D8 FF
// - PNG : 89 50 4E 47
// - ZIP : 50 4B 03 04
// etc.
//
// STRATEGIE :
// 1. Ouvrir le fichier
// 2. Lire les premiers 512 bytes (headerSize)
// 3. Utiliser la librairie filetype pour identifier les magic numbers
// 4. Si non trouvé, fallback sur l'extension du fichier
//
// Paramètres :
//   - path : chemin vers le fichier
//
// Retour :
//   - string : extension détectée ou "Error..." en cas de problème
func detectFileType(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "Error opening file"
	}
	defer f.Close()

	// Obtenir un buffer du pool (réutilisable)
	buf := bufferPool.Get().([]byte)
	n, err := f.Read(buf)

	// Remettre le buffer dans le pool pour réutilisation
	defer bufferPool.Put(buf)

	if err != nil && err != io.EOF || n == 0 {
		bufferPool.Put(buf)
		return "Error reading file"
	}

	// filetype.Match identifie le type basé sur les magic numbers
	kind, err := filetype.Match(buf[:n])
	if err != nil {
		return "Error detecting file type"
	}

	// Si un type a été trouvé, retourner son extension
	if kind != filetype.Unknown {
		return kind.Extension
	}

	// Fallback : extraire l'extension du nom de fichier
	ext := filepath.Ext(path)
	if len(ext) > 1 {
		return ext[1:] // retourner extension sans le point
	}

	// Dernier fallback : essayer la détection par pattern sur le contenu
	return detectPattern(buf[:n])
}
