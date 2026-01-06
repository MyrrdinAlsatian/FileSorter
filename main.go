package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	noExtCount := 0

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

		// Extraction de l'extension du fichier
		ext := strings.ToLower(filepath.Ext(d.Name()))

		if ext == "" || ext == ".txt" {
			noExtCount++
		} else {
			extCount[ext]++
		}
		return nil
	})

	if err != nil {
		fmt.Println("Erreur lors du parcours du répertoire:", err)
		return
	}

	// Affichage des résultats
	fmt.Println("Scan terminé !")
	fmt.Printf("Total de fichiers: %d\n", totalFiles)
	fmt.Printf("Total de répertoires: %d\n", totalDirs)
	fmt.Printf("Fichiers sans extension ou avec .txt: %d\n", noExtCount)
	fmt.Println("Extensions de fichiers trouvées:")
	// Tri et affichage des extensions
	for ext, count := range extCount {
		fmt.Printf("Extension: %s, Nombre de fichiers: %d\n", ext, count)
	}
}
