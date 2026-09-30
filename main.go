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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"

	"FileRecoveryOrganizer/app"
	"FileRecoveryOrganizer/checkpoint"
	"FileRecoveryOrganizer/classifier"
	"FileRecoveryOrganizer/dedup"
	"FileRecoveryOrganizer/detector"
	"FileRecoveryOrganizer/mover"
	"FileRecoveryOrganizer/organizer"
	"FileRecoveryOrganizer/report"
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
	// NotifyContext transforme Ctrl+C en annulation propagée aux opérations longues.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

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
		fmt.Printf("   Source:     %s\n", sourceDir)
		fmt.Printf("   Export:     %s\n", opts.ExportPath)
		fmt.Printf("   Workers:    %d\n", opts.Workers)
		fmt.Printf("   Dry-run:    %v\n", opts.DryRun) // %v = format par défaut de la valeur
		fmt.Printf("   Date-org:   %s\n", dateOrg)     // Affiche le format de date
		fmt.Printf("   Hash:       %v\n", opts.ComputeHash)
		fmt.Printf("   Duplicates: %v\n", opts.HashReport)
		fmt.Printf("   Resume:     %v\n", opts.Resume)
		fmt.Printf("   HTML:       %s\n", opts.HTMLReport)
		fmt.Println()
	}

	if opts.DryRun {
		fmt.Println("🔍 MODE SIMULATION - Aucun fichier ne sera modifié")
		fmt.Println()
	}

	if opts.Validate {
		fmt.Printf("🔬 Validation d'intégrité activée (types: %s)\n", opts.ValidateTypes)
		fmt.Println()
	}

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 2.5 : CHARGEMENT DU CHECKPOINT (si --resume)
	// ═══════════════════════════════════════════════════════════════════════

	processedFiles := checkpoint.NewProcessedFiles()
	if opts.Resume {
		fmt.Printf("🔄 Mode reprise activé, chargement de %s...\n", opts.ExportPath)
		if err := processedFiles.LoadFromJSONL(opts.ExportPath); err != nil {
			fmt.Printf("⚠️  Erreur lors du chargement: %v\n", err)
		} else if processedFiles.Count() > 0 {
			fmt.Printf("   ✓ %d fichiers déjà traités seront ignorés\n", processedFiles.Count())
		} else {
			fmt.Println("   ℹ️  Aucun fichier précédemment traité trouvé")
		}
		fmt.Println()
	}

	fmt.Println("📂 Scanning directory:", sourceDir)

	// Enregistrer les matchers personnalisés pour la détection de types
	detector.RegisterCustomMatchers()

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 3 : INITIALISATION DES STRUCTURES DE DONNÉES
	// ═══════════════════════════════════════════════════════════════════════

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
	err := scanner.CountFileContext(ctx, sourceDir, stats)
	if err != nil {
		fmt.Println("Error during the file count process:", err)
		return
	}

	fmt.Printf(" ➡ Total files: %d, Total directories: %d\n", stats.TotalFiles, stats.TotalDirs)
	fmt.Printf("Taille totale des fichiers: %s\n", utils.ReadableSize(stats.TotalSize))
	fmt.Printf("Start scanning...")

	// Détecteur de doublons (initialisé seulement si demandé)
	var duplicateFinder *dedup.DuplicateFinder
	if opts.ComputeHash || opts.HashReport {
		// Configurer le détecteur avec les options
		dedupOpts := dedup.FinderOptions{
			MinSize: opts.MinDupSize,
			Workers: opts.Workers,
		}
		duplicateFinder = dedup.NewDuplicateFinderWithOptions(dedupOpts)
	}

	// Slice pour collecter les résultats si on génère un rapport HTML
	var allResults []types.Result
	collectResults := opts.HTMLReport != ""

	// Créer une barre de progression
	bar := scanner.CreateProgessBar(stats.TotalFiles)

	// Construire les options de scan complètes
	scanOpts := scanner.ScanOptions{
		ClassifyOpts:  classifyOpts,
		ComputeHash:   opts.ComputeHash || opts.HashReport, // Activer le hash si demandé
		SkipChecker:   processedFiles,                      // Pour le mode resume (sera nil si pas de resume)
		Validate:      opts.Validate,                       // Valider l'intégrité des fichiers
		ValidateTypes: opts.ValidateTypes,                  // Types à valider
	}

	outcome, scanErr := app.ScanAndExport(ctx, app.ScanRequest{
		SourceDir:   sourceDir,
		ExportPath:  exportPath,
		Append:      opts.Resume,
		Workers:     opts.Workers,
		ScanOptions: scanOpts,
		OnProgress:  func() { bar.Add(1) },
		OnResult: func(result types.Result) {
			if duplicateFinder != nil {
				duplicateFinder.AddFile(result.Path, result.Size)
			}
			if collectResults {
				allResults = append(allResults, result)
			}
		},
	})
	if scanErr != nil {
		fmt.Println("\n❌ Error during scanning:", scanErr)
		return
	}

	if outcome.ExportError != nil {
		log.Printf("⚠️  Some errors occurred during export")
	}

	// Afficher le résumé final
	printSummary(outcome.Stats, stats, outcome.ResultCount, opts.Verbose)

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 9 : RAPPORT DE DOUBLONS (si demandé)
	// ═══════════════════════════════════════════════════════════════════════

	// Variable pour stocker le rapport de doublons (utilisé aussi pour le HTML)
	var duplicateReport *dedup.DuplicateReport
	if opts.HashReport && duplicateFinder != nil {
		fmt.Println("\n🔍 Analyse des doublons en cours...")
		var err error
		duplicateReport, err = duplicateFinder.FindDuplicatesContext(ctx)
		if err != nil {
			fmt.Println("\n❌ Error during duplicate analysis:", err)
			return
		}
		printDuplicateReport(duplicateReport, opts.Verbose)

		// Exporter le rapport en JSON
		reportPath := "duplicates_report.json"
		if err := exportDuplicateReport(duplicateReport, reportPath); err != nil {
			log.Printf("⚠️  Erreur lors de l'export du rapport: %v", err)
		} else {
			fmt.Printf("📄 Rapport des doublons exporté: %s\n", reportPath)
		}
	}

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 10 : GÉNÉRATION DU RAPPORT HTML (si demandé)
	// ═══════════════════════════════════════════════════════════════════════

	if opts.HTMLReport != "" {
		fmt.Println("\n📊 Génération du rapport HTML...")
		if err := report.GenerateHTML(allResults, sourceDir, opts.HTMLReport, duplicateReport); err != nil {
			log.Printf("⚠️  Erreur lors de la génération du rapport HTML: %v", err)
		} else {
			fmt.Printf("🌐 Rapport HTML généré: %s\n", opts.HTMLReport)
			fmt.Println("   Ouvrez ce fichier dans un navigateur pour visualiser les résultats.")
		}
	}

	// ═══════════════════════════════════════════════════════════════════════
	// ÉTAPE 11 : DÉPLACEMENT DES FICHIERS (si demandé)
	// ═══════════════════════════════════════════════════════════════════════

	if opts.MoveTo != "" {
		executeMover(ctx, opts, exportPath, sourceDir)
	}
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

// printDuplicateReport affiche le rapport de doublons de manière formatée.
//
// Cette fonction affiche un résumé des fichiers dupliqués trouvés,
// incluant le nombre de groupes, l'espace gaspillé, et les détails
// de chaque groupe (en mode verbose).
//
// Paramètres :
//   - report : le rapport de détection de doublons
//   - verbose : si true, afficher les chemins de tous les fichiers doublons
func printDuplicateReport(report *dedup.DuplicateReport, verbose bool) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("                    🔍 RAPPORT DE DOUBLONS")
	fmt.Println("═══════════════════════════════════════════════════════════════════")

	fmt.Printf("  📁 Fichiers analysés:     %d\n", report.TotalFiles)
	fmt.Printf("  ✅ Fichiers uniques:      %d\n", report.UniqueFiles)
	fmt.Printf("  🔄 Fichiers en double:    %d\n", report.DuplicateFiles)
	fmt.Printf("  📦 Groupes de doublons:   %d\n", report.DuplicateGroups)
	fmt.Printf("  💾 Espace gaspillé:       %s\n", utils.ReadableSize(report.WastedSpace))

	if report.DuplicateGroups == 0 {
		fmt.Println("\n  ✨ Aucun doublon détecté !")
	} else if verbose && len(report.Groups) > 0 {
		fmt.Println()
		fmt.Println("  📋 Détail des groupes de doublons:")
		fmt.Println("  ─────────────────────────────────────────────────────────────────")

		// Afficher les 10 premiers groupes (ou tous en mode très verbose)
		maxGroups := 10
		if len(report.Groups) < maxGroups {
			maxGroups = len(report.Groups)
		}

		for i := 0; i < maxGroups; i++ {
			group := report.Groups[i]
			fmt.Printf("\n  Groupe %d: %d fichiers (%s chacun)\n", i+1, group.Count, utils.ReadableSize(group.Size))
			fmt.Printf("    Hash: %s\n", group.Hash)

			// Afficher les 5 premiers chemins
			maxPaths := 5
			if len(group.Paths) < maxPaths {
				maxPaths = len(group.Paths)
			}
			for j := 0; j < maxPaths; j++ {
				fmt.Printf("      • %s\n", group.Paths[j])
			}
			if len(group.Paths) > 5 {
				fmt.Printf("      ... et %d autres fichiers\n", len(group.Paths)-5)
			}
		}

		if len(report.Groups) > 10 {
			fmt.Printf("\n  ... et %d autres groupes de doublons\n", len(report.Groups)-10)
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════")
}

// exportDuplicateReport exporte le rapport de doublons en JSON.
//
// Le rapport est exporté dans un fichier JSON formaté avec indentation
// pour être facilement lisible et exploitable par d'autres outils.
//
// Paramètres :
//   - report : le rapport de détection de doublons
//   - path : chemin du fichier JSON de sortie
//
// Retour :
//   - error : erreur si l'écriture échoue
func exportDuplicateReport(report *dedup.DuplicateReport, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Utiliser json.MarshalIndent pour un JSON lisible
	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// ═══════════════════════════════════════════════════════════════════════════
// FONCTIONS DE DÉPLACEMENT
// ═══════════════════════════════════════════════════════════════════════════

// executeMover exécute le déplacement des fichiers vers la destination.
//
// Cette fonction :
// 1. Génère un plan à partir du fichier JSONL
// 2. Affiche un aperçu du plan
// 3. Exécute les opérations de copie/déplacement
// 4. Affiche les résultats
func executeMover(ctx context.Context, opts utils.Options, jsonlPath string, sourceDir string) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("                    📦 DÉPLACEMENT DES FICHIERS")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Les flags sont des chaînes; les valider ici avant de construire les options métier typées.
	mode, err := mover.ParseMode(opts.MoveMode)
	if err != nil {
		log.Printf("❌ Erreur de configuration du mover: %v", err)
		return
	}
	overwriteMode, err := mover.ParseOverwriteMode(opts.MoveOverwrite)
	if err != nil {
		log.Printf("❌ Erreur de configuration du mover: %v", err)
		return
	}

	// Configurer les options du mover
	moverOpts := mover.Options{
		Mode:          mode,
		Source:        sourceDir,
		Destination:   opts.MoveTo,
		DryRun:        opts.DryRun,
		Verify:        opts.MoveVerify,
		Workers:       opts.Workers,
		OverwriteMode: overwriteMode,
		Verbose:       opts.Verbose,
		SkipCorrupted: opts.SkipCorrupted,
	}

	// Générer le plan à partir du JSONL
	fmt.Printf("📋 Génération du plan depuis %s...\n", jsonlPath)
	plan, err := mover.GeneratePlanFromJSONLContext(ctx, jsonlPath, moverOpts)
	if err != nil {
		log.Printf("❌ Erreur lors de la génération du plan: %v", err)
		return
	}

	// Afficher le résumé du plan
	plan.PrintSummary()
	plan.PrintStatsByCategory()

	// En mode dry-run, afficher l'aperçu et s'arrêter
	if opts.DryRun {
		plan.PrintPreview(20)
		fmt.Println("🔍 Mode simulation - aucune opération effectuée")
		return
	}

	// Vérifier qu'il y a des opérations à effectuer
	if plan.TotalFiles == 0 {
		fmt.Println("ℹ️  Aucun fichier à déplacer")
		return
	}

	confirmed, err := confirmMoverExecution(
		os.Stdin,
		os.Stderr,
		mode,
		overwriteMode,
		opts.Yes,
		plan.TotalFiles,
		opts.MoveTo,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Erreur pendant la confirmation: %v\n", err)
		return
	}
	if !confirmed {
		fmt.Println("Opération annulée; aucun fichier n'a été modifié.")
		return
	}

	// Exécuter le plan
	fmt.Printf("\n🚀 Démarrage du %s vers %s...\n\n", mode, opts.MoveTo)
	executor := mover.NewExecutor(plan)
	var journal *mover.OperationJournal
	if opts.MoveJournal != "" {
		journal, err = mover.NewOperationJournal(opts.MoveJournal)
		if err != nil {
			log.Printf("❌ Erreur lors de l'ouverture du journal mover: %v", err)
			return
		}
		executor.SetOperationJournal(journal)
	}
	result := executor.ExecuteContext(ctx)
	if journal != nil {
		result.JournalError = errors.Join(result.JournalError, journal.Close())
	}

	// Afficher les résultats
	result.PrintResults()
	if result.JournalError != nil {
		log.Printf("⚠️  Erreur lors de l'écriture du journal mover: %v", result.JournalError)
	} else if journal != nil {
		fmt.Printf("Journal des opérations écrit au fil de l'exécution dans %s\n", opts.MoveJournal)
	}
}

// confirmMoverExecution demande une validation uniquement pour les actions destructrices.
// Le lecteur et l'écrivain sont des paramètres pour tester l'interaction sans terminal réel.
func confirmMoverExecution(reader io.Reader, writer io.Writer, mode mover.Mode, overwriteMode mover.OverwriteMode, yes bool, operationCount int, destination string) (bool, error) {
	destructive := mode == mover.ModeMove || overwriteMode == mover.ConflictOverwrite
	if yes || !destructive {
		return true, nil
	}

	_, err := fmt.Fprintf(writer, "Confirmer %d opération(s) vers %q", operationCount, destination)
	if err != nil {
		return false, err
	}
	if mode == mover.ModeMove {
		if _, err := fmt.Fprint(writer, ", avec suppression des sources après copie validée"); err != nil {
			return false, err
		}
	}
	if overwriteMode == mover.ConflictOverwrite {
		if _, err := fmt.Fprint(writer, ", avec remplacement des fichiers existants"); err != nil {
			return false, err
		}
	}
	if _, err := fmt.Fprint(writer, " ? [o/N] "); err != nil {
		return false, err
	}

	// Scanner lit une réponse par ligne; EOF sans réponse vaut un refus, choix sûr pour un terminal fermé.
	answer := bufio.NewScanner(reader)
	if !answer.Scan() {
		return false, answer.Err()
	}
	response := strings.ToLower(strings.TrimSpace(answer.Text()))
	return response == "o" || response == "oui" || response == "y" || response == "yes", nil
}
