package main

import (
	"fmt"
	"log"
	"os"

	"FileRecoveryOrganizer/detector"
	"FileRecoveryOrganizer/exporter"
	"FileRecoveryOrganizer/scanner"
	"FileRecoveryOrganizer/types"
	"FileRecoveryOrganizer/utils"
)

func main() {

	var sourceDir string

	opts := utils.ParseFlags()
	if opts.SourceDir != "" {
		sourceDir = opts.SourceDir
	}

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

	collector := scanner.NewCollector(1024)
	statsSafe := scanner.NewStats()

	if opts.ExportPath != "" {
		fmt.Println("Export path set to:", opts.ExportPath)
		go exporter.StreamJSONLWithFilter(opts.ExportPath, collector.Results)
	}
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

	results := collector.Results

	exporter, err := exporter.NewJSONExporter("scan_results.jsonl")
	if err != nil {
		log.Fatalf("Failed to create JSON exporter: %v", err)
	}
	defer exporter.Close()
	for result := range results {
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
