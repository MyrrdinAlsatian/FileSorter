package main

import (
	"fmt"
	"os"
	"strings"

	"FileRecoveryOrganizer/detector"
	"FileRecoveryOrganizer/exporter"
	"FileRecoveryOrganizer/scanner"
	"FileRecoveryOrganizer/types"
	"FileRecoveryOrganizer/utils"

	"github.com/h2non/filetype"
)

func main() {

	var sourceDir string

	if len(os.Args) > 1 {
		sourceDir = os.Args[1]
	} else {
		dir, err := os.Getwd()
		if err != nil {
			fmt.Println("Error during getting current directory:", err)
			return
		}
		sourceDir = dir
	}

	fmt.Println("Start directory scan :", sourceDir)
	detector.RegisterCustomMatchers()

	collector := scanner.NewCollector()
	statsSafe := scanner.NewStats()
	// Map pour compter les extensions de fichiers
	stats := &types.Stats{
		DetectedFileType: make(map[string]int),
	}

	err := scanner.CountFile(sourceDir, stats)
	// Parcours du répertoire source
	if err != nil {
		fmt.Println("Error during the file count process")
		return
	}

	fmt.Printf(" ➡ Total files: %d, Total directories: %d\n", stats.TotalFiles, stats.TotalDirs)
	fmt.Printf("Taille totale des fichiers: %s\n", utils.ReadableSize(stats.TotalSize))
	fmt.Printf("Start scanning...")

	bar := scanner.CreateProgessBar(stats.TotalFiles)
	//  Scan the directory and update progress bar
	err = scanner.ScanDirectoryParallel(sourceDir, collector, statsSafe, func() {
		bar.Add(1)
	}, 4)

	if err != nil {
		fmt.Println("Error during scanning:", err)
		return
	}

	results := collector.GetResults()

	exporter, err := exporter.NewJSONExporter("scan_results.jsonl")
	if err != nil {
		log.Fatalf("Failed to create JSON exporter: %v", err)
	}
	defer exporter.Close()
	for _, result := range results {
		if err := exporter.Write(result); err != nil {
			log.Printf("Failed to write result for %s: %v", result.Path, err)
		}
	}
	fmt.Println("📁 Export JSONL terminé :", len(results), "entrées")
	// Affichage des résultats
	fmt.Println("-------------------------")
	fmt.Println("📊 Result summary :")
	fmt.Println("-------------------------")
	fmt.Printf("Total de fichiers: %d\n", statsSafe.TotalFiles)
	fmt.Printf("Total de répertoires: %d\n", stats.TotalDirs)
	fmt.Printf("Taille totale des fichiers: %s\n", utils.ReadableSize(statsSafe.TotalSize))
	fmt.Println("Extensions de fichiers trouvées:")
	// Tri et affichage des extensions
	for fileType, count := range statsSafe.FilesByType {
		fmt.Printf("%-10s %8d files  %10s\n",
			fileType,
			count,
			utils.ReadableSize(statsSafe.BytesByType[fileType]),
		)
	}
	fmt.Println("-------------------------")
	fmt.Printf("Collected results: %d\n", len(results))
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
