# Architecture technique - FileSorter

Ce document décrit l'architecture interne de l'application FileSorter.

---

## Vue d'ensemble

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              main.go                                     │
│                         (Point d'entrée, CLI)                            │
└────────────────────────────────┬────────────────────────────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│    scanner/     │    │    validator/   │    │     mover/      │
│  Scan parallèle │    │   Intégrité     │    │  Copie/Move     │
└────────┬────────┘    └────────┬────────┘    └────────┬────────┘
         │                      │                      │
         ▼                      │                      │
┌─────────────────┐             │                      │
│   detector/     │◄────────────┘                      │
│  Détection type │                                    │
└────────┬────────┘                                    │
         │                                             │
         ▼                                             │
┌─────────────────┐    ┌─────────────────┐             │
│   metadata/     │    │   classifier/   │             │
│  EXIF/MKV/MP4   │───►│  Catégorisation │             │
└─────────────────┘    └────────┬────────┘             │
                                │                      │
                                ▼                      │
                       ┌─────────────────┐             │
                       │   organizer/    │             │
                       │  Org. par date  │             │
                       └────────┬────────┘             │
                                │                      │
         ┌──────────────────────┼──────────────────────┘
         │                      │
         ▼                      ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   exporter/     │    │     dedup/      │    │    report/      │
│   JSONL export  │    │    Doublons     │    │   HTML report   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

---

## Packages

### `scanner/` - Scan parallèle

**Fichiers:**
- `scanner.go` - Logique principale de scan avec workers
- `collector.go` - Collecteur de résultats thread-safe
- `progress.go` - Barre de progression
- `stats.go` - Statistiques thread-safe

**Concepts clés:**
```go
// Worker pool pattern
for i := 0; i < workers; i++ {
    go func() {
        for path := range fileCh {
            result := processFile(path)
            collector.Results <- result
        }
    }()
}
```

**Interface SkipChecker pour le mode resume:**
```go
type SkipChecker interface {
    IsProcessed(path string) bool
}
```

---

### `detector/` - Détection de type

**Fichiers:**
- `detect.go` - Logique principale de détection
- `detectType.go` - Détection par magic bytes
- `customMatcher.go` - Matchers personnalisés (SQLite, etc.)
- `asset.go` - Détection d'images asset
- `thumbnail.go` - Détection de miniatures
- `pattern.go` - Patterns de fichiers

**Flow de détection:**
1. Lecture des premiers 262 octets (magic bytes)
2. Utilisation de `filetype.Match()` 
3. Fallback sur extension si non détecté
4. Matchers personnalisés pour formats spéciaux

---

### `validator/` - Validation d'intégrité

**Fichiers:**
- `validator.go` - Interface et registre
- `image.go` - JPEG, PNG, GIF, WebP, BMP
- `video.go` - MP4, MKV, AVI, WMV, FLV
- `audio.go` - MP3, FLAC, WAV, OGG, AIFF

**Pattern registre auto-enregistrement:**
```go
// Chaque validateur s'enregistre via init()
func init() {
    Register(&ImageValidator{})
}

// Interface commun
type Validator interface {
    Validate(path string) ValidationResult
    SupportedExtensions() []string
    SupportedMIMETypes() []string
}
```

**Types d'erreurs:**
- `truncated` - Fichier tronqué (récupération partielle)
- `corrupted` - Données corrompues
- `invalid_header` - En-tête invalide
- `empty` - Fichier vide

---

### `metadata/` - Extraction de métadonnées

**Fichiers:**
- `date.go` - Logique de détermination de la meilleure date
- `image.go` - Extraction EXIF
- `utils.go` - Helpers pour types de fichiers
- `types.go` - Types de données
- `mkv/mvk.go` - Parser EBML pour MKV
- `mp4/mp4.go` - Parser atoms pour MP4

**Priorité des dates:**
1. EXIF DateTimeOriginal (photos)
2. Métadonnées conteneur (MKV/MP4)
3. Date de modification filesystem
4. Date de création filesystem

**Parser EBML (MKV):**
```go
// Lecture d'un VINT (Variable Integer)
func readEBMLVInt(r io.Reader) (uint64, int, error)

// IDs EBML importants:
// 0x1A45DFA3 - EBML Header
// 0x18538067 - Segment
// 0x1549A966 - Info
// 0x4461     - DateUTC
```

---

### `classifier/` - Catégorisation

**Fichiers:**
- `category.go` - Logique de classification

**Catégories:**
- `images/photos` - Photos avec EXIF
- `images/screenshots` - Captures d'écran
- `images/assets` - Petites images, icônes
- `images/originals` - Autres images
- `videos/movies` - Vidéos longues
- `videos/clips` - Vidéos courtes
- `audio/music` - Musique avec tags
- `documents/` - Documents par type

---

### `organizer/` - Organisation par date

**Fichiers:**
- `date.go` - Génération de chemins par date

**Modes:**
```go
type DateOrgMode string
const (
    DateOrgNone      DateOrgMode = "none"
    DateOrgYear      DateOrgMode = "year"       // 2024/
    DateOrgYearMonth DateOrgMode = "year-month" // 2024/01/
    DateOrgYearMonthDay DateOrgMode = "year-month-day" // 2024/01/15/
)
```

---

### `dedup/` - Détection de doublons

**Fichiers:**
- `hash.go` - Calcul de hash et détection

**Algorithme 3 passes:**
1. **Grouper par taille** - Fichiers de même taille = potentiels doublons
2. **Quick hash** - Hash des 64KB début + fin
3. **Full hash** - SHA256 complet pour confirmation

**Optimisations:**
- Filtrage par taille minimale (`--min-size`)
- Workers parallèles pour le calcul
- Progress bars pour chaque phase

---

### `mover/` - Déplacement de fichiers

**Fichiers:**
- `mover.go` - Types et options
- `plan.go` - Génération du plan depuis JSONL
- `executor.go` - Exécution parallèle

**Modes:**
```go
type Mode string
const (
    ModeCopy     Mode = "copy"     // Copie (conserve originaux)
    ModeMove     Mode = "move"     // Déplace (supprime après copie)
    ModeHardlink Mode = "hardlink" // Liens durs (même partition)
    ModeSymlink  Mode = "symlink"  // Liens symboliques
)
```

**Gestion des conflits:**
- `skip` - Ignorer si existe
- `overwrite` - Écraser
- `rename` - Renommer avec suffixe (`_001`, `_002`)

---

### `checkpoint/` - Reprise de scan

**Fichiers:**
- `checkpoint.go` - Suivi des fichiers traités

**Fonctionnement:**
1. Lecture du JSONL existant au démarrage
2. Extraction des chemins déjà traités
3. Vérification avant traitement de chaque fichier
4. Export en mode append pour les nouveaux fichiers

---

### `exporter/` - Export JSONL

**Fichiers:**
- `jsonl.go` - Export JSON Lines
- `filter.go` - Filtres d'export

**Format JSONL:**
- Un objet JSON par ligne
- Streamable (pas besoin de charger tout en mémoire)
- Compatible avec `jq`, Python, etc.

---

### `report/` - Rapport HTML

**Fichiers:**
- `html.go` - Génération du rapport

**Contenu:**
- Statistiques globales
- Graphiques (Chart.js)
- Distribution par type/catégorie
- Liste des doublons
- Fichiers corrompus

---

## Flux de données

### Scan standard

```
1. main.go
   └── ParseFlags() → Options
   
2. scanner.ScanDirectoryParallelWithScanOptions()
   ├── filepath.WalkDir() → chemins
   ├── Workers (goroutines)
   │   ├── detector.Detect() → type
   │   ├── enricher.EnrichImage() → EXIF
   │   ├── metadata.GetFileMeta() → dates
   │   ├── validator.ValidateFile() → intégrité
   │   └── classifier.ClassifyWithOptions() → catégorie
   └── collector.Results ← résultats

3. Goroutine export
   └── exporter.WriteResult() → JSONL

4. Post-traitement
   ├── dedup.FindDuplicates() → doublons
   └── report.GenerateHTML() → rapport
```

### Déplacement

```
1. mover.GeneratePlanFromJSONL()
   ├── Lecture JSONL
   ├── Filtrage (corrupted, types, taille)
   ├── Génération chemins destination
   └── Détection conflits

2. mover.Execute()
   ├── Création répertoires
   ├── Workers parallèles
   │   ├── copyFile() / moveFile() / Link()
   │   └── verifyHash() si --verify
   └── Statistiques
```

---

## Concurrence

### Patterns utilisés

**Worker Pool:**
```go
jobs := make(chan string, workers*2)
var wg sync.WaitGroup

for i := 0; i < workers; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for job := range jobs {
            process(job)
        }
    }()
}
```

**Fan-out / Fan-in:**
```go
// Fan-out: distribuer aux workers
for _, item := range items {
    jobs <- item
}

// Fan-in: collecter les résultats
for result := range results {
    allResults = append(allResults, result)
}
```

**Sync.Pool pour buffers:**
```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        buf := make([]byte, 1024*1024)
        return &buf
    },
}

// Utilisation
buf := bufferPool.Get().(*[]byte)
defer bufferPool.Put(buf)
```

---

## Extension

### Ajouter un nouveau validateur

1. Créer `validator/newtype.go`
2. Implémenter l'interface `Validator`
3. S'enregistrer dans `init()`

```go
type NewTypeValidator struct{}

func init() {
    Register(&NewTypeValidator{})
}

func (v *NewTypeValidator) SupportedExtensions() []string {
    return []string{".xyz"}
}

func (v *NewTypeValidator) Validate(path string) ValidationResult {
    // Logique de validation
}
```

### Ajouter un parser de métadonnées

1. Créer `metadata/newformat/parser.go`
2. Implémenter la lecture des structures binaires
3. Intégrer dans `metadata.GetFileMeta()`

---

## Tests

```bash
# Tests unitaires
go test ./...

# Tests avec verbose
go test -v ./validator/...

# Coverage
go test -cover ./...
```

---

## Performance

### Optimisations implémentées

1. **Buffers réutilisables** (sync.Pool)
2. **Lecture partielle** pour détection (262 bytes)
3. **Quick hash** avant full hash
4. **Parallélisation** adaptative
5. **Filtrage précoce** (taille, type)

### Limites

- Workers limités à 32 (éviter surcharge I/O)
- Buffer de 1MB pour copies
- Progress bar throttlée (100ms)
