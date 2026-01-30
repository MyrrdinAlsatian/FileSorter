// Package utils contient des utilitaires réutilisables dans tout le projet.
//
// Ce package fournit notamment la gestion des options de ligne de commande
// et des fonctions de formatage.
package utils

import (
	"flag"
	"fmt"
	"os"
)

// Options contient toutes les options de configuration de l'application.
//
// CONCEPT GO : STRUCTURES (STRUCTS)
// =================================
// Une struct en Go est similaire à une classe sans méthodes (données seulement).
// On peut y ajouter des méthodes plus tard avec des "receivers".
//
// Les noms de champs commençant par une majuscule sont EXPORTÉS (publics).
// Les noms commençant par une minuscule sont PRIVÉS au package.
//
// Exemple :
//
//	type Person struct {
//	    Name string  // Exporté (accessible depuis d'autres packages)
//	    age  int     // Privé (accessible seulement dans ce package)
//	}
type Options struct {
	ExportPath string // Chemin du fichier d'export JSONL
	SourceDir  string // Répertoire source à scanner
	DryRun     bool   // Mode simulation (ne modifie rien)
	Verbose    bool   // Affichage détaillé
	Workers    int    // Nombre de workers parallèles
	Help       bool   // Afficher l'aide
}

// DefaultOptions retourne les options par défaut.
//
// CONCEPT GO : FONCTIONS FACTORY
// ==============================
// En Go, on utilise souvent des fonctions "factory" (usine) pour créer
// des instances avec des valeurs par défaut. C'est un pattern courant
// car Go n'a pas de constructeurs comme en Java/C++.
//
// Convention de nommage :
// - NewXxx() : crée un pointeur (*Xxx)
// - DefaultXxx() : retourne une valeur (Xxx)
func DefaultOptions() Options {
	return Options{
		ExportPath: "scan_results.jsonl",
		SourceDir:  ".",
		DryRun:     false,
		Verbose:    false,
		Workers:    4,
		Help:       false,
	}
}

// ParseFlags parse les arguments de ligne de commande et retourne les options.
//
// CONCEPT GO : LE PACKAGE FLAG
// ============================
// Le package "flag" de la bibliothèque standard permet de définir et parser
// les arguments de ligne de commande de façon simple.
//
// DEUX FAÇONS DE DÉFINIR UN FLAG :
//
//  1. flag.String("name", "default", "help") → retourne un *string
//     ptr := flag.String("name", "default", "help")
//     // Utiliser avec *ptr
//
//  2. flag.StringVar(&variable, "name", "default", "help") → modifie variable
//     var s string
//     flag.StringVar(&s, "name", "default", "help")
//     // Utiliser directement s
//
// On utilise la méthode 2 ici car on a déjà la struct Options.
//
// LE & (OPÉRATEUR D'ADRESSE)
// ==========================
// &variable retourne un pointeur vers la variable.
// C'est nécessaire car flag.StringVar doit modifier la variable originale.
func ParseFlags() Options {
	// Partir des valeurs par défaut
	opts := DefaultOptions()

	// Définir les flags avec leurs raccourcis
	// Chaque flag a une version longue (--export) et courte (-e)
	flag.StringVar(&opts.ExportPath, "export", opts.ExportPath, "Chemin du fichier d'export JSONL")
	flag.StringVar(&opts.ExportPath, "e", opts.ExportPath, "Chemin du fichier d'export (raccourci)")

	flag.StringVar(&opts.SourceDir, "source", opts.SourceDir, "Répertoire source à scanner")
	flag.StringVar(&opts.SourceDir, "s", opts.SourceDir, "Répertoire source (raccourci)")

	flag.BoolVar(&opts.DryRun, "dry-run", opts.DryRun, "Mode simulation - n'effectue aucune modification")
	flag.BoolVar(&opts.DryRun, "n", opts.DryRun, "Mode dry-run (raccourci)")

	flag.BoolVar(&opts.Verbose, "verbose", opts.Verbose, "Affichage détaillé")
	flag.BoolVar(&opts.Verbose, "v", opts.Verbose, "Mode verbose (raccourci)")

	flag.IntVar(&opts.Workers, "workers", opts.Workers, "Nombre de workers parallèles")
	flag.IntVar(&opts.Workers, "w", opts.Workers, "Nombre de workers (raccourci)")

	flag.BoolVar(&opts.Help, "help", false, "Afficher l'aide")
	flag.BoolVar(&opts.Help, "h", false, "Afficher l'aide (raccourci)")

	// flag.Parse() lit os.Args et remplit les variables liées aux flags
	flag.Parse()

	// Si l'utilisateur demande l'aide, l'afficher et quitter
	if opts.Help {
		PrintUsage()
		os.Exit(0) // Code 0 = succès (pas une erreur)
	}

	// Validation des options
	// On s'assure que le nombre de workers est dans une plage raisonnable
	if opts.Workers < 1 {
		opts.Workers = 1
	}
	if opts.Workers > 32 {
		opts.Workers = 32 // Limite pour éviter de surcharger le système
	}

	return opts
}

// PrintUsage affiche l'aide d'utilisation.
func PrintUsage() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════════╗
║              FILE RECOVERY ORGANIZER - Aide                       ║
╚═══════════════════════════════════════════════════════════════════╝

USAGE:
    filesorter [OPTIONS]

OPTIONS:
    -s, --source <PATH>    Répertoire source à scanner (défaut: .)
    -e, --export <PATH>    Chemin du fichier d'export JSONL (défaut: scan_results.jsonl)
    -w, --workers <N>      Nombre de workers parallèles (défaut: 4, max: 32)
    -n, --dry-run          Mode simulation - n'effectue aucune modification
    -v, --verbose          Affichage détaillé des opérations
    -h, --help             Afficher cette aide

EXEMPLES:
    # Scanner le répertoire courant
    filesorter

    # Scanner un répertoire spécifique
    filesorter -s /chemin/vers/dossier

    # Mode simulation avec logs détaillés
    filesorter -s /data/recovery -n -v

    # Export vers un fichier spécifique avec 8 workers
    filesorter -s /data -e rapport.jsonl -w 8

NOTES:
    - Le mode dry-run est recommandé pour la première utilisation
    - Les fichiers ne sont jamais modifiés pendant le scan
    - L'export JSONL contient toutes les informations pour un tri ultérieur
`)
}
