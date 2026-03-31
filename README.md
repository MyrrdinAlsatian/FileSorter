# File Recovery Organizer v2.0

<p align="center">
  <strong>🔧 Outil complet de scan, validation, et organisation de fichiers récupérés</strong>
</p>

---

## 🎯 Objectif

Outil en ligne de commande pour **scanner, analyser et organiser des fichiers récupérés** d'un disque dur endommagé. Conçu pour traiter des millions de fichiers provenant de logiciels comme PhotoRec.

### Problèmes résolus

- ✅ Fichiers sans extension ou avec extension `.txt`
- ✅ Fichiers dispersés dans des milliers de sous-dossiers (`recup_dir.*`)
- ✅ Fichiers corrompus ou tronqués
- ✅ Doublons occupant de l'espace inutilement
- ✅ Métadonnées perdues (dates, noms originaux)

---

## 📦 Installation

```bash
# Prérequis : Go 1.21+
git clone https://github.com/votre-repo/FileSorter.git
cd FileSorter
go build -o filesorter .
```

---

## 🚀 Utilisation rapide

```bash
# 1. Scanner un répertoire
./filesorter -s /chemin/vers/recovery -e scan.jsonl

# 2. Reprendre un scan interrompu
./filesorter -s /chemin/vers/recovery -e scan.jsonl -R

# 3. Scanner avec validation d'intégrité
./filesorter -s /chemin/vers/recovery -e scan.jsonl -V

# 4. Détecter les doublons
./filesorter -e scan.jsonl -D

# 5. Générer un rapport HTML
./filesorter -e scan.jsonl -r rapport.html

# 6. Déplacer les fichiers vers leur destination
./filesorter -e scan.jsonl -M /destination/triée
```

---

## 📋 Fonctionnalités

### 🔍 Scanner parallèle
- Parcours récursif ultra-rapide avec workers parallèles
- Détection de type par magic bytes (pas seulement l'extension)
- Extraction de métadonnées (EXIF, ID3, MKV, MP4, AVI)
- Mode resume pour reprendre après interruption (Ctrl+C)

### 🔬 Validation d'intégrité
- Détection des fichiers **corrompus ou tronqués**
- Formats supportés : JPEG, PNG, GIF, WebP, BMP, MP4, MKV, AVI, WMV, FLV, MP3, FLAC, WAV, OGG
- Types d'erreurs : `truncated`, `corrupted`, `invalid_header`, `empty`

### 🔄 Détection de doublons
- Algorithme en 3 passes : taille → quick hash → full SHA256
- Parallélisation optimisée
- Filtre par taille minimale
- Rapport détaillé de l'espace gaspillé

### �️ Déduplication active
- Actions : `delete`, `hardlink`, `symlink`
- Stratégies de sélection : `shortest`, `oldest`, `newest`, `path`
- Mode dry-run pour simulation
- Récupération automatique de l'espace disque

### �📦 Déplacement intelligent
- Modes : `copy`, `move`, `hardlink`, `symlink`
- Vérification d'intégrité post-copie
- Gestion des conflits : `skip`, `overwrite`, `rename`
- Progress bar en temps réel

### ✏️ Renommage intelligent
- Patterns personnalisables : `{date}_{camera}_{seq:4}.{ext}`
- Variables : date, datetime, year, month, camera, original, hash, seq, width, height
- Presets : `simple`, `dated`, `photo`, `video`, `hash`, `full`
- Gestion des conflits : `increment`, `skip`, `hash`, `timestamp`
- Mode preview pour prévisualiser sans appliquer

### 📊 Rapports
- Export JSONL (un objet par ligne, streamable)
- Rapport HTML interactif avec graphiques
- Rapport de doublons en JSON

### 📅 Organisation par date
- Extraction des dates depuis EXIF, MKV, MP4, système de fichiers
- Modes : `year`, `year-month`, `year-month-day`
- Fichiers sans date dans `unknown_date/`

---

## 🛠 Options de ligne de commande

### Options générales
| Option | Description |
|--------|-------------|
| `-s, --source <PATH>` | Répertoire source à scanner (défaut: `.`) |
| `-e, --export <PATH>` | Fichier d'export JSONL (défaut: `scan_results.jsonl`) |
| `-w, --workers <N>` | Nombre de workers parallèles (défaut: 4, max: 32) |
| `-n, --dry-run` | Mode simulation |
| `-v, --verbose` | Affichage détaillé |
| `-h, --help` | Afficher l'aide |

### Reprise de scan
| Option | Description |
|--------|-------------|
| `-R, --resume` | Reprendre un scan interrompu |

### Validation d'intégrité
| Option | Description |
|--------|-------------|
| `-V, --validate` | Valider l'intégrité des fichiers |
| `--validate-types` | Types à valider: `image`, `video`, `audio`, `all` |

### Détection de doublons
| Option | Description |
|--------|-------------|
| `-H, --hash` | Calculer les hash SHA256 |
| `-D, --duplicates` | Générer un rapport de doublons |
| `-m, --min-size <BYTES>` | Taille minimale (défaut: 1MB) |

### Déduplication active
| Option | Description |
|--------|-------------|
| `-X, --dedup <ACTION>` | `delete`, `hardlink`, `symlink`, `dry-run` |
| `--dedup-keep <MODE>` | `shortest`, `oldest`, `newest`, `first`, `path` |
| `--dedup-priority <PATH>` | Chemin prioritaire (pour mode `path`) |

### Organisation par date
| Option | Description |
|--------|-------------|
| `-d, --date-org <MODE>` | `none`, `year`, `year-month`, `year-month-day` |

### Déplacement de fichiers
| Option | Description |
|--------|-------------|
| `-M, --move-to <PATH>` | Destination pour le tri |
| `--move-mode <MODE>` | `copy`, `move`, `hardlink`, `symlink` |
| `--verify` | Vérifier le hash après copie |
| `--overwrite <MODE>` | `skip`, `overwrite`, `rename` |
| `--skip-corrupted` | Ignorer les fichiers corrompus |

### Renommage
| Option | Description |
|--------|-------------|
| `-p, --rename <PATTERN>` | Pattern de renommage ou preset |
| `--conflict <MODE>` | `increment`, `skip`, `hash`, `timestamp` |
| `-P, --preview` | Aperçu du renommage sans l'appliquer |

### Patterns de renommage
| Variable | Description |
|----------|-------------|
| `{date}` | Date YYYY-MM-DD |
| `{datetime}` | Date et heure YYYY-MM-DD_HHMMSS |
| `{year}`, `{month}`, `{day}` | Composants de date |
| `{camera}` | Modèle d'appareil (EXIF) |
| `{original}` | Nom de fichier original |
| `{ext}` | Extension du fichier |
| `{hash:N}` | N premiers caractères du hash |
| `{seq:N}` | Numéro séquentiel (N chiffres) |
| `{width}`, `{height}` | Dimensions de l'image |

**Presets disponibles :** `simple`, `dated`, `photo`, `video`, `hash`, `full`

### Rapports
| Option | Description |
|--------|-------------|
| `-r, --report <PATH>` | Générer un rapport HTML |

---

## 📂 Architecture du projet

```
FileSorter/
├── main.go              # Point d'entrée
├── scanner/             # Scan parallèle + progress bar
├── detector/            # Détection de type (magic bytes, patterns)
├── classifier/          # Catégorisation des fichiers
├── metadata/            # Extraction EXIF, MKV, MP4, AVI
├── organizer/           # Organisation par date
├── validator/           # Validation d'intégrité
│   ├── image.go         # JPEG, PNG, GIF, WebP, BMP
│   ├── video.go         # MP4, MKV, AVI, WMV, FLV
│   └── audio.go         # MP3, FLAC, WAV, OGG
├── dedup/               # Détection et suppression de doublons
│   ├── hash.go          # Calcul de hash (quick + full)
│   └── deduplicate.go   # Déduplication active
├── mover/               # Copie/déplacement des fichiers
├── renamer/             # Renommage intelligent
│   ├── pattern.go       # Parsing des patterns
│   ├── renamer.go       # Moteur de renommage
│   └── conflict.go      # Gestion des conflits
├── checkpoint/          # Reprise de scan
├── exporter/            # Export JSONL
├── report/              # Rapport HTML
├── types/               # Types communs
└── utils/               # Flags, helpers
```

---

## 🔄 Workflow recommandé

```bash
# Étape 1 : Scan initial avec validation
./filesorter -s /data/recovery -e scan.jsonl -V -w 8

# Étape 2 : Si interrompu, reprendre
./filesorter -s /data/recovery -e scan.jsonl -R

# Étape 3 : Analyser les doublons
./filesorterDédupliquer (simulation)
./filesorter -e scan.jsonl -X dry-run

# Étape 5 : Dédupliquer (hardlinks)
./filesorter -e scan.jsonl -X hardlink --dedup-keep shortest

# Étape 6 : Prévisualiser le renommage
./filesorter -e scan.jsonl -M /sorted -p photo -P

# Étape 7ter -e scan.jsonl -M /sorted -p photo -P

# Étape 6 : Déplacer avec renommage et vérification
./filesorter -e scan.jsonl -M /sorted -p "{date}_{camera}_{seq:4}.{ext}" --verify
```

---

## 📊 Format JSONL

Chaque ligne du fichier JSONL contient un objet JSON :

```json
{
  "path": "/data/recovery/recup_dir.1/f0001234.jpg",
  "size": 2456789,
  "type": "jpg",
  "target_path": "images/photos/2024/01/f0001234.jpg",
  "valid": true,
  "valid_details": {"width": "1920", "height": "1080"},
  "quick_hash": "a1b2c3d4e5f6...",
  "image_exif": {
    "date_taken": "2024:01:15 14:30:52",
    "camera_model": "Canon EOS R5"
  }
}
```

---

## 🔧 Types de fichiers supportés

### Images
JPEG, PNG, GIF, WebP, BMP, TIFF, HEIC/HEIF

### Vidéos
MP4, MKV, WebM, AVI, MOV, WMV, FLV, 3GP, MPEG

### Audio
MP3, FLAC, WAV, OGG, Opus, AAC, M4A, WMA, AIFF

### Documents
PDF, DOC/DOCX, XLS/XLSX, PPT/PPTX, ODT

### Archives
ZIP, RAR, 7z, TAR, GZ

---

## 💡 Conseils

1. **Toujours travailler sur une copie** des fichiers récupérés
2. **Utiliser le dry-run** avant tout déplacement
3. **Activer la validation** pour détecter les fichiers corrompus
4. **Créer des hardlinks** si vous triez sur la même partition (économie d'espace)
5. **Reprendre avec -R** si le scan est interrompu

---

## 📝 Notes pédagogiques

Ce projet permet d'apprendre :
- **Concurrence Go** : goroutines, channels, sync.Pool, WaitGroup
- **Patterns** : worker pool, fan-out/fan-in, pipeline
- **Parsing binaire** : magic bytes, EBML (MKV), atoms (MP4), RIFF (AVI)
- **Design patterns** : registre, factory, interface
- **CLI** : package flag, gestion des options

---

## 📜 Licence

MIT License

---

## 🔜 Roadmap

- [x] ~~Renommage intelligent (patterns `{date}_{camera}_{seq}.{ext}`)~~
- [x] ~~Déduplication active (suppression/liens des doublons)~~
- [ ] Interface web pour tri manuel
- [ ] Détection de similarité d'images (perceptual hash)
