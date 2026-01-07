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
		ext := kind.Extension
		mime := kind.MIME.Value

		switch {
		case strings.HasPrefix(mime, "image/"):
			return "image"
		case strings.HasPrefix(mime, "video/"):
			return "video"
		case strings.HasPrefix(mime, "audio/"):
			return "audio"
		case strings.HasPrefix(mime, "application/pdf"):
			return "pdf"
		case strings.HasPrefix(mime, "application/zip") ||
			strings.HasPrefix(mime, "application/x-rar") ||
			strings.HasPrefix(mime, "application/x-7z-compressed"):
			return "archive"
		case strings.HasPrefix(mime, "text/"):
			return "texte"
		default:
			// Tout le reste → on utilise l'extension détectée
			return ext
		}
	} else {
		return detectPattern(filePath)
	}
}

func detectPattern(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return "Erreur d'ouverture du fichier"
	}
	defer f.Close()

	buf := make([]byte, 1024)
	n, err := f.Read(buf)

	if err != nil {
		return "Erreur de lecture du fichier"
	}

	content := string(buf[:n])
	content = strings.TrimSpace(content)
	// ---------- Langages et scripts ----------
	switch {
	case strings.HasPrefix(content, "package ") || strings.Contains(content, "func ") || strings.Contains(content, "import "):
		return "go"
	case strings.HasPrefix(content, "<?php"):
		return "php"
	case strings.HasPrefix(content, "#!/usr/bin/python") || strings.Contains(content, "def "):
		return "python"
	case strings.HasPrefix(content, "#!/bin/bash") || strings.Contains(content, "echo ") || strings.Contains(content, "function "):
		return "bash"
	case strings.HasPrefix(content, "#!/usr/bin/env ruby") || strings.Contains(content, "def ") || strings.Contains(content, "class "):
		return "ruby"
	case strings.HasPrefix(content, "#!/usr/bin/perl"):
		return "perl"
	case strings.Contains(content, "function") || strings.Contains(content, "var ") || strings.Contains(content, "let ") ||
		strings.Contains(content, "const ") || strings.Contains(content, "=>"):
		return "js"
	case strings.Contains(content, "interface") || strings.Contains(content, "type ") || strings.Contains(content, "export "):
		return "ts"
	}
	// ---------- Documents / Web ----------
	switch {
	case strings.HasPrefix(content, "{") || strings.HasPrefix(content, "["):
		return "json"
	case strings.HasPrefix(content, "<?xml"):
		return "xml"
	case strings.HasPrefix(content, "#") || strings.Contains(content, "\n#"):
		return "md"
	case strings.HasPrefix(content, "<!DOCTYPE html>") || strings.HasPrefix(content, "<html"):
		return "html"
	case strings.HasPrefix(content, "\\documentclass"):
		return "latex"
	case strings.Contains(content, "{") && strings.Contains(content, "}") &&
		(strings.Contains(content, "color") || strings.Contains(content, "background") || strings.Contains(content, "font")):
		return "css"
	}

	// ---------- Fichiers de config ----------
	switch {
	case strings.Contains(content, "---") && strings.Contains(content, ":"):
		return "yaml"
	case strings.Contains(content, "[") && strings.Contains(content, "]") && strings.Contains(content, "="):
		return "ini"
	case strings.Contains(content, "[") && strings.Contains(content, "]") && strings.Contains(content, "="):
		return "toml"
	}

	return "inconnu"
}
