# File Recovery Organizer - Projet Go

## 🎯 Objectif du projet

Ce projet a pour but de **scanner et organiser des fichiers récupérés d’un disque dur**, y compris ceux qui :

- n’ont plus leur extension d’origine  
- ont été renommés en `.txt`  
- sont dans de nombreux sous-dossiers (ex : `recup_dir.*`)  

L’objectif final est de **retrouver la vraie extension des fichiers** avant de les trier dans des dossiers appropriés, tout en **garantissant la sécurité maximale** (aucune modification des fichiers avant validation).

---

## 📂 Architecture du projet

```sql
project/
│
├─ main.go # Point d’entrée du programme
├─ scan/ # Fonctions de scan et inventaire
│ └─ scan.go
├─ classify/ # Fonctions de détection des types et signatures
│ └─ magic.go
├─ report/ # Génération de rapports JSON ou texte
│ └─ summary.go
└─ README.md
```

---

## 🛠 Fonctionnalités

### Phase 1 — Scan et inventaire

- Parcours récursif des dossiers (`recup_dir.*`)
- Comptage des fichiers et dossiers
- Extraction des extensions existantes
- Détection des fichiers sans extension ou renommés `.txt`
- Génération d’un rapport initial (JSON ou TXT)
- **Aucune modification** des fichiers pendant cette phase

### Phase 2 — Détection et correction d’extensions

- Lecture des premiers octets pour détecter le type réel (signature magique / magic numbers)
- Détection automatique des types courants : JPEG, PNG, PDF, ZIP, MP3, etc.
- Ajout d’une future extension pour chaque fichier détecté
- Fichiers inconnus restant classés dans une catégorie `Inconnus`

### Phase 3 — Création automatique des dossiers de réception

- Arborescence dynamique basée sur les catégories détectées
- Exemple :
- TRI/
    Images/
    jpg/
    png/
    Vidéos/
    mp4/
    Documents/
    pdf/
    Audio/
    mp3/
    Archives/
    zip/
    Inconnus/
- Aucun déplacement effectué à ce stade — uniquement création de dossiers

### Phase 4 — Tri réel (à faire plus tard)

- Déplacement des fichiers vers leurs dossiers correspondants
- Gestion des doublons
- Mise à jour d’un log complet

---

## ⚙️ Installation

1. Installer Go : [https://go.dev/dl/](https://go.dev/dl/)  
2. Cloner ce projet ou télécharger le ZIP  
3. Dans le terminal, compiler le projet :
 ```bash
 go build main.go
 ```
 Exécuter le programme :

./main.exe   # Windows
./main       # Linux

## 🧪 Mode Simulation (recommandé)

Avant de déplacer quoi que ce soit, le programme peut être lancé en mode simulation :

`var dryRun = true`


- Affiche ce qu’il ferait
- Ne modifie aucun fichier
- Idéal pour vérifier l’arborescence et les fichiers à traiter

Pour le tri réel, passer à :

`var dryRun = false`

## 💡 Bonnes pratiques

- Toujours travailler sur une copie des fichiers récupérés
- Éviter de trier sur un disque différent pour les fichiers volumineux
- Ne pas interrompre le programme pendant un déplacement
- Vérifier les logs en cas d’erreur

## 📝 Notes pédagogiques

Ce projet te permet d’apprendre :
- Les bases du langage Go : `map`, `slice`, `struct`, fonctions
- La lecture de fichiers et gestion des dossiers (`os`, `filepath`)
- La détection de types via signatures binaires (`magic numbers`)
- La génération de rapports JSON et TXT
- La création d’un workflow sûr pour manipuler de très gros volumes de fichiers

## 🔜 Étapes futures

- Ajout d’un tri réel automatisé
- Détection avancée pour formats rares ou corrompus
- Interface CLI avec options :
    - `--dry-run`
    - `--category images`
    - `--output json`

- Gestion des doublons par hash