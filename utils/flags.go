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
	Resume     bool   // Reprendre un scan interrompu

	// Options d'organisation par date
	// Valeurs possibles : "none", "year", "year-month", "year-month-day"
	DateOrg string // Format d'organisation par date

	// Options de détection de doublons
	ComputeHash bool  // Calculer les hash pour détecter les doublons
	HashReport  bool  // Générer un rapport de doublons
	MinDupSize  int64 // Taille minimale pour chercher les doublons (en bytes)

	// Options de déduplication active
	DeduplicateAction   string // Action : delete, hardlink, symlink, dry-run
	DeduplicateStrategy string // Stratégie de sélection : shortest, oldest, newest, first
	DeduplicatePriority string // Chemin prioritaire (pour stratégie path)

	// Options de rapport
	HTMLReport string // Chemin du rapport HTML (vide = pas de rapport)

	// Options de validation d'intégrité
	Validate      bool   // Valider l'intégrité des fichiers
	ValidateTypes string // Types à valider (image,video,audio,all)

	// Options de déplacement (mover)
	MoveTo        string // Répertoire de destination pour déplacer/copier
	MoveMode      string // Mode : copy, move, hardlink, symlink
	MoveVerify    bool   // Vérifier le hash après copie
	MoveOverwrite string // Gestion des conflits : skip, overwrite, rename
	SkipCorrupted bool   // Ignorer les fichiers corrompus lors du déplacement

	// Options de renommage (renamer)
	RenamePattern  string // Pattern de renommage ({date}_{seq:4}.{ext})
	RenameConflict string // Stratégie de conflit : increment, skip, hash, timestamp
	RenamePreview  bool   // Aperçu du renommage sans l'appliquer
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
		ExportPath:  "scan_results.jsonl",
		SourceDir:   ".",
		DryRun:      false,
		Verbose:     false,
		Workers:     4,
		Help:        false,
		Resume:      false,           // Par défaut : nouveau scan
		DateOrg:     "none",          // Par défaut : pas d'organisation par date
		ComputeHash: false,           // Par défaut : pas de calcul de hash
		HashReport:  false,           // Par défaut : pas de rapport de doublons
		MinDupSize:  1 * 1024 * 1024, // Par défaut : 1 MB minimum pour les doublons

		// Déduplication active
		DeduplicateAction:   "",         // Par défaut : pas de déduplication
		DeduplicateStrategy: "shortest", // Par défaut : garder le chemin le plus court
		DeduplicatePriority: "",         // Par défaut : pas de chemin prioritaire

		HTMLReport:    "",     // Par défaut : pas de rapport HTML
		Validate:      false,  // Par défaut : pas de validation
		ValidateTypes: "all",  // Par défaut : valider tous les types supportés
		MoveTo:        "",     // Par défaut : pas de déplacement
		MoveMode:      "copy", // Par défaut : copier (ne pas supprimer les originaux)
		MoveVerify:    false,  // Par défaut : pas de vérification hash
		MoveOverwrite: "skip", // Par défaut : ignorer les conflits
		SkipCorrupted: true,   // Par défaut : ignorer les fichiers corrompus

		// Renommage
		RenamePattern:  "",          // Par défaut : pas de renommage
		RenameConflict: "increment", // Par défaut : ajouter un suffixe numérique
		RenamePreview:  false,       // Par défaut : appliquer le renommage
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

	flag.BoolVar(&opts.Resume, "resume", opts.Resume,
		"Reprendre un scan interrompu (ignore les fichiers déjà dans le JSONL)")
	flag.BoolVar(&opts.Resume, "R", opts.Resume,
		"Reprendre le scan (raccourci)")

	// Options d'organisation par date
	// Valeurs : none, year, year-month, year-month-day (ou ym, ymd)
	flag.StringVar(&opts.DateOrg, "date-org", opts.DateOrg,
		"Organisation par date: none, year, year-month, year-month-day")
	flag.StringVar(&opts.DateOrg, "d", opts.DateOrg,
		"Organisation par date (raccourci)")

	// Options de détection de doublons
	flag.BoolVar(&opts.ComputeHash, "hash", opts.ComputeHash,
		"Calculer les hash SHA256 pour détecter les doublons")
	flag.BoolVar(&opts.ComputeHash, "H", opts.ComputeHash,
		"Activer le calcul de hash (raccourci)")
	flag.BoolVar(&opts.HashReport, "duplicates", opts.HashReport,
		"Générer un rapport de fichiers doublons")
	flag.BoolVar(&opts.HashReport, "D", opts.HashReport,
		"Rapport de doublons (raccourci)")
	flag.Int64Var(&opts.MinDupSize, "min-size", opts.MinDupSize,
		"Taille minimale en bytes pour chercher les doublons (défaut: 1MB)")
	flag.Int64Var(&opts.MinDupSize, "m", opts.MinDupSize,
		"Taille minimale pour doublons (raccourci)")

	// Options de déduplication active
	flag.StringVar(&opts.DeduplicateAction, "dedup", opts.DeduplicateAction,
		"Action de déduplication: delete, hardlink, symlink, dry-run")
	flag.StringVar(&opts.DeduplicateAction, "X", opts.DeduplicateAction,
		"Déduplication (raccourci)")
	flag.StringVar(&opts.DeduplicateStrategy, "dedup-keep", opts.DeduplicateStrategy,
		"Stratégie: shortest, oldest, newest, first, path (défaut: shortest)")
	flag.StringVar(&opts.DeduplicatePriority, "dedup-priority", opts.DeduplicatePriority,
		"Chemin prioritaire pour la stratégie 'path'")

	// Options de rapport HTML
	flag.StringVar(&opts.HTMLReport, "report", opts.HTMLReport,
		"Générer un rapport HTML au chemin spécifié")
	flag.StringVar(&opts.HTMLReport, "r", opts.HTMLReport,
		"Rapport HTML (raccourci)")

	// Options de validation d'intégrité
	flag.BoolVar(&opts.Validate, "validate", opts.Validate,
		"Valider l'intégrité des fichiers (détecter les fichiers corrompus)")
	flag.BoolVar(&opts.Validate, "V", opts.Validate,
		"Valider l'intégrité (raccourci)")
	flag.StringVar(&opts.ValidateTypes, "validate-types", opts.ValidateTypes,
		"Types à valider: image, video, audio, all (défaut: all)")

	// Options de déplacement (mover)
	flag.StringVar(&opts.MoveTo, "move-to", opts.MoveTo,
		"Destination pour déplacer/copier les fichiers triés")
	flag.StringVar(&opts.MoveTo, "M", opts.MoveTo,
		"Destination (raccourci)")
	flag.StringVar(&opts.MoveMode, "move-mode", opts.MoveMode,
		"Mode: copy, move, hardlink, symlink (défaut: copy)")
	flag.BoolVar(&opts.MoveVerify, "verify", opts.MoveVerify,
		"Vérifier l'intégrité après copie (compare les hash)")
	flag.StringVar(&opts.MoveOverwrite, "overwrite", opts.MoveOverwrite,
		"Gestion des conflits: skip, overwrite, rename (défaut: skip)")
	flag.BoolVar(&opts.SkipCorrupted, "skip-corrupted", opts.SkipCorrupted,
		"Ignorer les fichiers corrompus lors du déplacement (défaut: true)")

	// Options de renommage (renamer)
	flag.StringVar(&opts.RenamePattern, "rename", opts.RenamePattern,
		"Pattern de renommage ({date}_{seq:4}.{ext}, ou preset: simple, dated, photo, video)")
	flag.StringVar(&opts.RenamePattern, "p", opts.RenamePattern,
		"Pattern de renommage (raccourci)")
	flag.StringVar(&opts.RenameConflict, "conflict", opts.RenameConflict,
		"Stratégie de conflit: increment, skip, hash, timestamp (défaut: increment)")
	flag.BoolVar(&opts.RenamePreview, "preview", opts.RenamePreview,
		"Aperçu du renommage sans l'appliquer")
	flag.BoolVar(&opts.RenamePreview, "P", opts.RenamePreview,
		"Aperçu du renommage (raccourci)")

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
	fmt.Print(`
╔═══════════════════════════════════════════════════════════════════╗
║              FILE RECOVERY ORGANIZER - Aide                       ║
╚═══════════════════════════════════════════════════════════════════╝

USAGE:
    filesorter [OPTIONS]

OPTIONS GÉNÉRALES:
    -s, --source <PATH>    Répertoire source à scanner (défaut: .)
    -e, --export <PATH>    Chemin du fichier d'export JSONL (défaut: scan_results.jsonl)
    -w, --workers <N>      Nombre de workers parallèles (défaut: 4, max: 32)
    -d, --date-org <MODE>  Organisation par date (voir ci-dessous)
    -n, --dry-run          Mode simulation - n'effectue aucune modification
    -v, --verbose          Affichage détaillé des opérations
    -h, --help             Afficher cette aide

REPRISE DE SCAN:
    -R, --resume           Reprendre un scan interrompu (ignore les fichiers
                           déjà présents dans le fichier JSONL)

DÉTECTION DE DOUBLONS:
    -H, --hash             Calculer les hash SHA256 pour tous les fichiers
    -D, --duplicates       Générer un rapport de fichiers doublons
    -m, --min-size <BYTES> Taille minimale pour chercher les doublons (défaut: 1MB)
                           Accepte : 1024, 1KB, 1MB, 1GB

RAPPORTS:
    -r, --report <PATH>    Générer un rapport HTML interactif au chemin spécifié

VALIDATION D'INTÉGRITÉ:
    -V, --validate         Valider l'intégrité des fichiers (détecter les corrompus)
    --validate-types       Types à valider: image, video, audio, all (défaut: all)
                           Les fichiers corrompus sont marqués dans le JSONL

DÉDUPLICATION ACTIVE:
    -X, --dedup <ACTION>   Action sur les doublons: delete, hardlink, symlink, dry-run
    --dedup-keep <MODE>    Stratégie de sélection de l'original:
                             shortest  Garder le chemin le plus court (défaut)
                             oldest    Garder le fichier le plus ancien
                             newest    Garder le fichier le plus récent
                             first     Garder le premier trouvé
                             path      Préférer un chemin spécifique
    --dedup-priority PATH  Chemin prioritaire (pour --dedup-keep path)

DÉPLACEMENT DE FICHIERS:
    -M, --move-to <PATH>   Destination pour copier/déplacer les fichiers triés
    --move-mode <MODE>     Mode: copy, move, hardlink, symlink (défaut: copy)
    --verify               Vérifier l'intégrité après copie
    --overwrite <MODE>     Conflits: skip, overwrite, rename (défaut: skip)
    --skip-corrupted       Ignorer les fichiers corrompus (défaut: true)

RENOMMAGE:
    -p, --rename <PATTERN> Pattern de renommage ou preset
    --conflict <MODE>      Conflits: increment, skip, hash, timestamp (défaut: increment)
    -P, --preview          Aperçu du renommage sans l'appliquer

PATTERNS DE RENOMMAGE:
    Variables disponibles:
      {date}          Date (YYYY-MM-DD)
      {datetime}      Date et heure (YYYY-MM-DD_HHMMSS)
      {year}/{month}/{day}  Composants de date
      {hour}/{minute}/{second}  Composants d'heure
      {camera}        Modèle de l'appareil photo (EXIF)
      {original}      Nom de fichier original (sans extension)
      {ext}           Extension du fichier
      {type}          Type de fichier (jpg, mp4, etc.)
      {category}      Catégorie (images, videos, etc.)
      {hash:8}        Hash du fichier (8 premiers caractères)
      {seq:4}         Numéro séquentiel (4 chiffres avec zéros)
      {width}/{height}  Dimensions de l'image

    Presets prédéfinis:
      simple     {original}.{ext}
      dated      {date}_{original}.{ext}
      photo      {year}/{month}/{camera}_{seq:4}.{ext}
      video      {year}/{month}/{date}_{seq:4}.{ext}
      hash       {hash:8}.{ext}
      full       {year}/{month}/{day}/{camera}_{datetime}_{seq:4}.{ext}

MODES D'ORGANISATION PAR DATE (-d, --date-org):
    none            Pas d'organisation par date (défaut)
    year            Par année : images/originals/2024/
    year-month      Par année/mois : images/originals/2024/01/
    year-month-day  Par année/mois/jour : images/originals/2024/01/15/

    Alias : y (year), ym (year-month), ymd (year-month-day)

    La date est extraite en priorité des métadonnées (EXIF, MKV, etc.)
    puis du système de fichiers si non disponible.
    Les fichiers sans date vont dans le dossier "unknown_date".

EXEMPLES:
    # Scanner le répertoire courant
    filesorter

    # Scanner un répertoire spécifique
    filesorter -s /chemin/vers/dossier

    # Organisation par année/mois avec logs détaillés
    filesorter -s /data/photos -d year-month -v

    # Mode simulation avec organisation par année
    filesorter -s /data/recovery -d y -n -v

    # Export vers un fichier spécifique avec 8 workers
    filesorter -s /data -e rapport.jsonl -w 8

    # Reprendre un scan interrompu
    filesorter -s /data -R

    # Détection de doublons avec rapport, fichiers > 5MB seulement
    filesorter -s /data -D -m 5MB

    # Générer un rapport HTML complet
    filesorter -s /data -r rapport.html

    # Valider l'intégrité des fichiers (détecter les corrompus)
    filesorter -s /data -V

    # Valider uniquement les images
    filesorter -s /data -V --validate-types image

    # Déplacer les fichiers vers une destination triée (mode copie)
    filesorter -e scan.jsonl -M /destination

    # Déplacer avec vérification d'intégrité
    filesorter -e scan.jsonl -M /destination --verify

    # Mode déplacement (supprime les originaux après copie)
    filesorter -e scan.jsonl -M /destination --move-mode move

    # Créer des hardlinks (même partition, pas d'espace supplémentaire)
    filesorter -e scan.jsonl -M /destination --move-mode hardlink

    # Renommer les fichiers avec un pattern simple
    filesorter -e scan.jsonl -M /sorted -p dated

    # Renommer les photos avec appareil et séquence
    filesorter -e scan.jsonl -M /sorted -p "{camera}_{date}_{seq:4}.{ext}"

    # Aperçu du renommage sans copier
    filesorter -e scan.jsonl -M /sorted -p photo -P

    # Renommer avec hash pour les conflits
    filesorter -e scan.jsonl -M /sorted -p dated --conflict hash

    # Simuler la déduplication (dry-run)
    filesorter -e scan.jsonl -X dry-run

    # Supprimer les doublons (garde le chemin le plus court)
    filesorter -e scan.jsonl -X delete

    # Remplacer les doublons par des hardlinks
    filesorter -e scan.jsonl -X hardlink --dedup-keep oldest

    # Préférer les fichiers dans /sorted/ lors de la déduplication
    filesorter -e scan.jsonl -X hardlink --dedup-keep path --dedup-priority /sorted

WORKFLOW RECOMMANDÉ:
    1. Premier scan avec validation : filesorter -s /data -e scan.jsonl -V
    2. Si interrompu : filesorter -s /data -e scan.jsonl -R
    3. Analyse doublons : filesorter -s /data -D -r rapport.html
    4. Dédupliquer (simulation) : filesorter -e scan.jsonl -X dry-run
    5. Dédupliquer (hardlink) : filesorter -e scan.jsonl -X hardlink
    6. Déplacer vers destination : filesorter -e scan.jsonl -M /sorted --verify

NOTES:
    - Le mode dry-run est recommandé pour la première utilisation
    - Les fichiers ne sont jamais modifiés pendant le scan
    - L'export JSONL contient toutes les informations pour un tri ultérieur
    - La validation (-V) détecte les fichiers tronqués ou corrompus
    - L'organisation par date utilise les métadonnées EXIF/MKV quand disponibles
    - Le mode resume (-R) lit le JSONL existant et saute les fichiers déjà traités
`)
}
