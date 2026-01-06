package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"
	"github.com/schollz/progressbar/v3"
)

func main() {

	var sourceDir string

	if len(os.Args) > 1 {
		sourceDir = os.Args[1]
	} else {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Println("Erreur lors de la récupération du répertoire courant:", err)
			return
		}
		sourceDir = dir
	}

	fmt.Println("Analyse du répertoire :", sourceDir)
	fmt.Println("Veuillez patienter...")

	// Map pour compter les extensions de fichiers
	extCount := make(map[string]int)

	// Compteur pour les fichiers sans extension
	noExtCount := make(map[string]int)

	// Compteurs pour les fichiers et les répertoires
	totalFiles := 0
	totalDirs := 0

	// Parcours du répertoire source
	err := filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {

		if err != nil {
			fmt.Printf("Erreur d'accès à %q: %v\n", path, err)
			return nil
		}

		if d.IsDir() {
			totalDirs++
			return nil
		}
		totalFiles++
		return nil
	})

	bar := progressbar.Default(int64(totalFiles))

	filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, err error) error {

		if err != nil {
			fmt.Printf("Erreur d'accès à %q: %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		// Extraction de l'extension du fichier
		ext := strings.ToLower(filepath.Ext(d.Name()))

		if ext == "" || ext == ".txt" {
			realType := detectFileType(path)
			noExtCount[realType]++
		} else {
			extCount[ext]++
		}
		bar.Add(1)
		return nil
	})

	if err != nil {
		fmt.Println("Erreur lors du parcours du répertoire:", err)
		return
	}

	// Affichage des résultats
	fmt.Println("Scan terminé !")
	fmt.Println("-------------------------")
	fmt.Printf("Total de fichiers: %d\n", totalFiles)
	fmt.Printf("Total de répertoires: %d\n", totalDirs)
	fmt.Println("Extensions de fichiers trouvées:")
	// Tri et affichage des extensions
	for ext, count := range extCount {
		fmt.Printf("Extension: %s, Nombre de fichiers: %d\n", ext, count)
	}

	fmt.Println("Fichiers sans extension ou de type texte détectés par type réel:")
	for fileType, count := range noExtCount {
		fmt.Printf("Type: %s, Nombre de fichiers: %d\n", fileType, count)
	}
	fmt.Println("-------------------------")
}

func detectFileType(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return "Erreur d'ouverture du fichier"
	}
	// Permet de fermer le fichier après la détection mais à la fin de la fonction detectFileType
	// Bonner pratique pour éviter les fuites de ressources, à mettre juste après l'ouverture du fichier
	// tout ce qui est marqué defer est exécuté à la fin, même si plusieurs defer sont empilés,
	// ils s’exécutent dans l’ordre inverse (pile LIFO)
	defer f.Close()

	// Lire les premiers 512 octets du fichier pour la détection du type MIME
	buf := make([]byte, 512)
	n, err := f.Read(buf)

	if err != nil {
		return "Erreur de lecture du fichier"
	}

	kind, err := filetype.Match(buf[:n])
	if err != nil {
		return "Erreur de détection du type de fichier"
	}
	if kind != filetype.Unknown {
		return kind.MIME.Value
	}
	// Par défaut, retourner le type MIME détecté
	return "inconnu"
}
