// Package main est le point d'entrée de l'application File Recovery Organizer.
//
// CE FICHIER MONTRE PLUSIEURS CONCEPTS IMPORTANTS EN GO :
//
// 1. GOROUTINES ET CANAUX (lignes 80-95)
//   - Les goroutines sont des "fils d'exécution légers" lancés avec le mot-clé `go`
//   - Les canaux (channels) permettent de communiquer entre goroutines
//   - Pattern producteur/consommateur : les workers produisent, l'exporter consomme
//
// 2. GESTION DES ERREURS (omniprésent)
//   - Go n'a pas d'exceptions, les erreurs sont des valeurs retournées
//   - Pattern: `if err != nil { /* gérer l'erreur */ }`
//
// 3. DEFER (ligne 107)
//   - `defer` exécute une fonction à la FIN de la fonction courante
//   - Utile pour fermer des ressources (fichiers, connexions)
//
// 4. FONCTIONS DE CALLBACK (ligne 96)
//   - On peut passer des fonctions comme arguments
//   - `func() { bar.Add(1) }` est une fonction anonyme (closure)
//
// 5. POINTEURS VS VALEURS (lignes 48-49)
//   - `*scanner.SafeStats` est un pointeur vers SafeStats
//   - Les pointeurs permettent de modifier la valeur originale
package main

import (
	"fmt"
	"log"
	"os"

	"FileRecoveryOrganizer/classifier"
	"FileRecoveryOrganizer/detector"
	"FileRecoveryOrganizer/exporter"
	"FileRecoveryOrganizer/organizer"
	"FileRecoveryOrganizer/scanner"
	"FileRecoveryOrganizer/types"
	"FileRecoveryOrganizer/utils"
)

// main est la fonction principale, le point d'entrée du programme.
// En Go, la fonction main() du package main est automatiquement appelée au démarrage.
func main() {
	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 1 : CONFIGURATION
	// ═══════════════════════════════════════════════════════════════════════

	// ParseFlags() utilise le package "flag" de la bibliothèque standard
	// pour lire les arguments de ligne de commande (ex: -s /chemin -v)
	opts := utils.ParseFlags()

	// Déterminer le répertoire source
	// Le "." représente le répertoire courant en informatique
	sourceDir := opts.SourceDir
	if sourceDir == "." {
		// os.Getwd() = Get Working Directory (répertoire de travail actuel)
		dir, err := os.Getwd()
		if err != nil {
			fmt.Println("❌ Error during getting current directory:", err)
			return // Quitte la fonction main (et donc le programme)
		}
		sourceDir = dir
	}

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 2 : AFFICHAGE INITIAL
	// ═══════════════════════════════════════════════════════════════════════

	printBanner()

	// Configurer les options de classification (organisation par date)
	// organizer.ParseDateOrganization convertit la chaîne en enum
	dateOrg := organizer.ParseDateOrganization(opts.DateOrg)
	classifyOpts := classifier.ClassifyOptions{
		DateOrganization: dateOrg,
		DateSource:       organizer.DateSourceAuto,
	}

	// Mode verbose : afficher plus de détails (utile pour le débogage)
	if opts.Verbose {
		fmt.Printf("📋 Options:\n")
		fmt.Printf("   Source:   %s\n", sourceDir)
		fmt.Printf("   Export:   %s\n", opts.ExportPath)
		fmt.Printf("   Workers:  %d\n", opts.Workers)
		fmt.Printf("   Dry-run:  %v\n", opts.DryRun) // %v = format par défaut de la valeur
		fmt.Printf("   Date-org: %s\n", dateOrg)     // Affiche le format de date
		fmt.Println()
	}

	if opts.DryRun {
		fmt.Println("🔍 MODE SIMULATION - Aucun fichier ne sera modifié")
		fmt.Println()
	}

	fmt.Println("📂 Scanning directory:", sourceDir)

	// Enregistrer les matchers personnalisés pour la détection de types
	detector.RegisterCustomMatchers()

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 3 : INITIALISATION DES STRUCTURES DE DONNÉES
	// ═══════════════════════════════════════════════════════════════════════

	// Collector : contient un canal (channel) pour recevoir les résultats
	// Le buffer de 2048 permet de stocker 2048 résultats en attente
	// CONCEPT : Un canal bufferisé évite que l'émetteur bloque si le récepteur est lent
	collector := scanner.NewCollector(2048)

	// SafeStats : statistiques thread-safe (protégées par un mutex)
	// CONCEPT : Quand plusieurs goroutines modifient les mêmes données,
	// il faut les protéger avec un mutex pour éviter les "race conditions"
	statsSafe := scanner.NewStats()

	exportPath := opts.ExportPath
	if opts.Verbose {
		fmt.Println("📤 Export path:", exportPath)
	}

	// Statistiques de pré-comptage (avant le scan détaillé)
	// CONCEPT : Le & crée un pointeur vers la structure
	// CONCEPT : make() initialise les maps (les maps nil causent une panique)
	stats := &types.Stats{
		DetectedFileType: make(map[string]int),
	}

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 4 : PRÉ-COMPTAGE DES FICHIERS
	// ═══════════════════════════════════════════════════════════════════════

	// Compter les fichiers AVANT le scan pour afficher une barre de progression
	err := scanner.CountFile(sourceDir, stats)
	if err != nil {
		fmt.Println("Error during the file count process")
		return
	}

	fmt.Printf(" ➡ Total files: %d, Total directories: %d\n", stats.TotalFiles, stats.TotalDirs)
	fmt.Printf("Taille totale des fichiers: %s\n", utils.ReadableSize(stats.TotalSize))
	fmt.Printf("Start scanning...")

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 5 : CONFIGURATION DE L'EXPORT JSONL
	// ═══════════════════════════════════════════════════════════════════════

	// JSONL = JSON Lines : un objet JSON par ligne, idéal pour le streaming
	jsonExporter, err := exporter.NewJSONExporter(exportPath)
	if err != nil {
		// log.Fatalf affiche le message ET termine le programme avec code d'erreur
		log.Fatalf("Failed to create JSON exporter: %v", err)
	}

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 6 : LANCEMENT DE LA GOROUTINE CONSOMMATRICE
	// ═══════════════════════════════════════════════════════════════════════
	//
	// CONCEPT CRUCIAL : PATTERN PRODUCTEUR-CONSOMMATEUR
	// ──────────────────────────────────────────────────
	// On lance AVANT le scan une goroutine qui LIT le canal collector.Results.
	// Pourquoi ? Parce que pendant le scan, les workers ÉCRIVENT dans ce canal.
	//
	// Si personne ne lit le canal et que le buffer est plein → DEADLOCK !
	// (Tout le monde attend, personne n'avance)
	//
	// Visualisation :
	//
	//   [Worker 1] ──┐
	//   [Worker 2] ──┼──▶ [Canal Results] ──▶ [Goroutine Export] ──▶ [Fichier JSONL]
	//   [Worker 3] ──┤         (buffer)
	//   [Worker 4] ──┘

	var resultCount int               // Compteur de résultats exportés
	var exportErr error               // Stocke la dernière erreur d'export
	exportDone := make(chan struct{}) // Canal de signalisation (sans données)

	// `go func() { ... }()` lance une fonction anonyme dans une nouvelle goroutine
	go func() {
		// defer close(exportDone) : à la fin de cette goroutine, fermer le canal
		// Cela signale au programme principal que l'export est terminé
		defer close(exportDone)

		// range sur un canal : itère jusqu'à ce que le canal soit fermé
		for result := range collector.Results {
			if err := jsonExporter.Write(result); err != nil {
				log.Printf("Failed to write result for %s: %v", result.Path, err)
				exportErr = err
			}
			resultCount++
		}
	}()

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 7 : SCAN PARALLÈLE DES FICHIERS
	// ═══════════════════════════════════════════════════════════════════════

	// Créer une barre de progression
	bar := scanner.CreateProgessBar(stats.TotalFiles)

	// ScanDirectoryParallelWithOptions lance plusieurs workers (goroutines) pour traiter
	// les fichiers en parallèle. Le callback `func() { bar.Add(1) }` est appelé
	// après chaque fichier traité pour mettre à jour la barre de progression.
	//
	// On utilise la version WithOptions pour passer les options de classification
	// (notamment l'organisation par date configurée via -d / --date-org)
	//
	// CONCEPT : Les closures "capturent" les variables de leur environnement
	// Ici, `bar` est capturé par la closure
	err = scanner.ScanDirectoryParallelWithOptions(sourceDir, collector, statsSafe, func() {
		bar.Add(1)
	}, opts.Workers, classifyOpts)

	if err != nil {
		fmt.Println("\n❌ Error during scanning:", err)
		return
	}

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 8 : ATTENTE ET FINALISATION
	// ═══════════════════════════════════════════════════════════════════════

	// `<-exportDone` bloque jusqu'à ce que le canal exportDone soit fermé
	// C'est notre façon d'attendre que la goroutine d'export ait terminé
	// CONCEPT : Recevoir d'un canal fermé retourne immédiatement la valeur zéro
	<-exportDone

	// Fermer proprement l'exporter (flush le buffer, ferme le fichier)
	jsonExporter.Close()

	if exportErr != nil {
		log.Printf("⚠️  Some errors occurred during export")
	}

	// Afficher le résumé final
	printSummary(statsSafe, stats, resultCount, opts.Verbose)
}

// printBanner affiche la bannière de l'application.
//
// NOTE : En Go, les chaînes entre backticks (`) sont des "raw strings" :
// - Elles peuvent contenir des sauts de ligne
// - Les caractères spéciaux ne sont pas interprétés
// - Idéal pour l'ASCII art ou le texte multiligne
func printBanner() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════════╗
║              FILE RECOVERY ORGANIZER v1.0                         ║
╚═══════════════════════════════════════════════════════════════════╝`)
	fmt.Println()
}

// printSummary affiche le résumé des résultats du scan.
//
// CONCEPTS GO ILLUSTRÉS :
//
// 1. PASSAGE DE POINTEURS (*scanner.SafeStats, *types.Stats)
//   - On passe des pointeurs pour éviter de copier les grandes structures
//   - Le * devant le type indique "pointeur vers"
//   - Ici on ne modifie pas les données, mais on évite une copie coûteuse
//
// 2. ITÉRATION SUR UNE MAP (for fileType, count := range map)
//   - range sur une map retourne la clé et la valeur
//   - L'ordre d'itération est ALÉATOIRE en Go (par design)
//   - Si on veut un ordre précis, il faut d'abord trier les clés
//
// 3. FORMAT PRINTF
//   - %-12s : chaîne alignée à gauche sur 12 caractères
//   - %8d    : entier aligné à droite sur 8 caractères
//   - %12s   : chaîne alignée à droite sur 12 caractères
//
// Paramètres :
//   - statsSafe : statistiques thread-safe collectées pendant le scan
//   - stats : statistiques initiales (comptage des répertoires)
//   - resultCount : nombre de résultats exportés
//   - verbose : si true, afficher tous les types de fichiers
func printSummary(statsSafe *scanner.SafeStats, stats *types.Stats, resultCount int, verbose bool) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("                        📊 RÉSUMÉ DU SCAN")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("  📁 Total fichiers traités:  %d\n", statsSafe.TotalFiles)
	fmt.Printf("  📂 Total répertoires:       %d\n", stats.TotalDirs)
	fmt.Printf("  💾 Taille totale:           %s\n", utils.ReadableSize(statsSafe.TotalSize))
	fmt.Printf("  📤 Entrées exportées:       %d\n", resultCount)

	if statsSafe.Errors > 0 {
		fmt.Printf("  ⚠️  Erreurs:                 %d\n", statsSafe.Errors)
	}

	fmt.Println()
	fmt.Println("  📈 Types de fichiers détectés:")
	fmt.Println("  ─────────────────────────────────────────────────────────────────")

	// Afficher les types de fichiers
	// NOTE : L'ordre d'affichage est aléatoire car on itère sur une map
	// Pour trier, il faudrait extraire les clés dans un slice et les trier
	for fileType, count := range statsSafe.FilesByType {
		// En mode non-verbose, n'afficher que les types avec > 10 fichiers
		// pour éviter de polluer l'affichage avec des types rares
		if verbose || count > 10 {
			fmt.Printf("     %-12s %8d fichiers  %12s\n",
				fileType,
				count,
				utils.ReadableSize(statsSafe.BytesByType[fileType]),
			)
		}
	}
	fmt.Println("═══════════════════════════════════════════════════════════════════")
}
