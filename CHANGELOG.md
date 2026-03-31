# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format est basé sur [Keep a Changelog](https://keepachangelog.com/fr/1.0.0/).

---

## [2.0.0] - 2026-04-01

### Ajouté

#### Validation d'intégrité (`-V, --validate`)
- Détection des fichiers corrompus ou tronqués
- Support images : JPEG, PNG, GIF, WebP, BMP
- Support vidéos : MP4, MKV, AVI, WMV, FLV
- Support audio : MP3, FLAC, WAV, OGG, AIFF
- Types d'erreurs : `truncated`, `corrupted`, `invalid_header`, `empty`
- Option `--validate-types` pour filtrer par catégorie

#### Déplacement intelligent (`-M, --move-to`)
- Mode `copy` : copie en conservant les originaux
- Mode `move` : déplace (supprime après copie réussie)
- Mode `hardlink` : liens durs (même partition)
- Mode `symlink` : liens symboliques
- Vérification d'intégrité post-copie (`--verify`)
- Gestion des conflits : `skip`, `overwrite`, `rename`
- Progress bar temps réel
- Statistiques par catégorie

#### Reprise de scan (`-R, --resume`)
- Sauvegarde automatique dans JSONL
- Détection des fichiers déjà traités
- Mode append pour l'export
- Reprise transparente après Ctrl+C

#### Parsers de métadonnées
- Parser EBML complet pour MKV/WebM
- Parser atoms pour MP4/MOV
- Parser RIFF pour AVI
- Extraction des dates de création

### Modifié

- Restructuration complète du code en packages
- Documentation pédagogique en français
- Help message amélioré avec exemples
- Architecture modulaire avec interfaces

### Corrigé

- Deadlock dans le scanner parallèle
- Double `bufferPool.Put` dans le hash
- Parsing MKV (lecture VINT correcte)
- Parsing MP4 récursif pour atoms imbriqués

---

## [1.5.0] - 2026-03-28

### Ajouté

#### Rapport HTML (`-r, --report`)
- Interface interactive avec Chart.js
- Distribution par type et catégorie
- Liste des doublons avec taille
- Fichiers corrompus listés

#### Détection de doublons optimisée
- Algorithme 3 passes (taille → quick hash → full hash)
- Parallélisation avec workers
- Filtre par taille minimale (`-m, --min-size`)
- Progress bars pour chaque phase

---

## [1.4.0] - 2026-03-25

### Ajouté

#### Organisation par date (`-d, --date-org`)
- Mode `year` : `/2024/`
- Mode `year-month` : `/2024/01/`
- Mode `year-month-day` : `/2024/01/15/`
- Extraction depuis EXIF, MKV, MP4
- Fallback sur dates filesystem

---

## [1.3.0] - 2026-03-20

### Ajouté

#### Détection de doublons (`-H, -D`)
- Calcul SHA256 des fichiers
- Quick hash (64KB début+fin)
- Rapport de doublons en JSON
- Espace gaspillé calculé

---

## [1.2.0] - 2026-03-15

### Ajouté

#### Extraction de métadonnées
- EXIF pour images (date, appareil, GPS)
- ID3 pour audio (titre, artiste)
- Métadonnées MKV basiques

#### Classification avancée
- `images/photos` vs `images/screenshots`
- `videos/movies` vs `videos/clips`
- Détection d'assets et thumbnails

---

## [1.1.0] - 2026-03-10

### Ajouté

#### Scanner parallèle
- Workers configurables (`-w`)
- Progress bar (schollz/progressbar)
- Statistiques thread-safe

#### Export JSONL
- Format JSON Lines
- Streaming (mémoire constante)
- Un objet par ligne

---

## [1.0.0] - 2026-03-05

### Ajouté

- Détection de type par magic bytes
- Parcours récursif des dossiers
- Comptage des fichiers par type
- Mode dry-run
- CLI basique avec flags

---

## Types de changements

- **Ajouté** pour les nouvelles fonctionnalités
- **Modifié** pour les changements de fonctionnalités existantes
- **Déprécié** pour les fonctionnalités qui seront supprimées
- **Supprimé** pour les fonctionnalités supprimées
- **Corrigé** pour les corrections de bugs
- **Sécurité** pour les vulnérabilités
