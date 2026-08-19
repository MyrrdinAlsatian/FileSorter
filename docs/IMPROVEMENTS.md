# Pistes d'amélioration pour FileSorter

Ce document propose une feuille de route pour faire évoluer FileSorter sans
vouloir tout réécrire. Les priorités tiennent compte de la nature du projet :
il traite beaucoup de fichiers et certaines actions (`move`, `delete`,
`overwrite`) sont difficiles à annuler.

Une amélioration devrait suivre ce cycle : écrire un test qui décrit le
comportement attendu, modifier une petite partie du code, puis lancer `task`.
Pour les détails des commandes, voir [TASKFILE.md](TASKFILE.md).

## Priorité 1 : rendre les opérations sur fichiers plus sûres

| Amélioration | Pourquoi | Piste concrète | Concept Go |
|---|---|---|---|
| Vérifier les chemins | Une destination placée dans la source peut produire des copies récursives. | Résoudre les chemins avec `filepath.Abs`, refuser une destination identique ou incluse dans la source, et définir une politique claire pour les liens symboliques. | `filepath`, erreurs sentinelles, tests de tables. |
| Confirmer les actions destructrices | `move`, `delete` et `overwrite` modifient des données. | Afficher le résumé du plan puis demander confirmation, avec `--yes` pour les scripts. | I/O, séparation CLI/métier. |
| Vérifier l'espace disque | `CheckDiskSpace` est actuellement un emplacement réservé. Une copie peut échouer tardivement faute d'espace. | Ajouter une implémentation par système avec `diskspace_windows.go` et `diskspace_unix.go`, puis vérifier l'espace avant l'exécution. | Build tags, interfaces, API système. |
| Copier vers un fichier temporaire | Une interruption peut laisser une destination partielle. | Copier vers `destination.part`, synchroniser, vérifier le hash, puis renommer vers la destination finale. | `os.Rename`, atomicité, `defer`. |
| Journaliser les opérations | Après des milliers d'actions, il faut savoir ce qui a été fait. | Écrire un JSONL avec source, destination, action, date et résultat ; prévoir une restauration pour les déplacements. | JSON, log append-only, conception de types. |

Commencer par les contrôles de chemin et la copie temporaire : ils protègent les
données avant même l'arrivée d'une interface graphique.

## Priorité 2 : préparer un cœur métier réutilisable

`main.go` orchestre actuellement les étapes et affiche directement beaucoup
d'informations. C'est naturel pour une première CLI, mais une interface Wails
aura besoin d'appeler la même logique sans dépendre de `fmt.Printf` ni d'une
barre de progression terminal.

| Amélioration | Bénéfice | Direction proposée |
|---|---|---|
| Créer un package d'application | La CLI et Wails appellent les mêmes cas d'usage. | Ajouter par exemple `app/` avec un service `Scan(ctx, options)` retournant un rapport structuré. |
| Retourner des résultats structurés | Une interface présente statistiques et erreurs sans analyser du texte. | Remplacer les affichages métier par des valeurs comme `ScanResult`, `ExecutionResult` et `[]OperationError`. |
| Introduire `context.Context` | L'utilisateur peut annuler un scan, une copie ou un hash. | Ajouter `ctx context.Context` aux opérations longues et vérifier `ctx.Err()` dans les boucles et workers. |
| Injecter les dépendances système | Les tests deviennent plus indépendants du disque. | Définir de petites interfaces pour le système de fichiers ou l'horloge, seulement là où le test en a besoin. |
| Séparer progression et traitement | La progression peut être affichée en CLI, dans Wails ou enregistrée dans un log. | Exposer un callback ou un canal d'événements typés, plutôt qu'une barre de progression dans le cœur métier. |

Le détail propre à l'interface est dans [WAILS_GUIDE.md](WAILS_GUIDE.md). Les
notions Go nécessaires sont expliquées dans [GO_ADVANCED.md](GO_ADVANCED.md).

## Priorité 3 : renforcer la qualité du code Go

### Erreurs et observabilité

Les packages métier devraient retourner des erreurs enrichies et laisser
`main` décider de leur affichage :

```go
return fmt.Errorf("copy %q to %q: %w", source, destination, err)
```

La CLI peut ainsi afficher simplement l'erreur, tandis qu'une future interface
peut la transformer en message utilisateur. Un logger structuré avec `log/slog`
remplacerait progressivement les `fmt.Printf` dispersés et associerait le
chemin, l'opération et l'erreur à une même entrée de journal.

### Types plus stricts

Certaines options sont des chaînes. Les convertir tôt vers des types dédiés
limite les valeurs invalides et centralise leur validation :

```go
type OverwriteMode string

const (
    OverwriteSkip      OverwriteMode = "skip"
    OverwriteRename    OverwriteMode = "rename"
    OverwriteOverwrite OverwriteMode = "overwrite"
)
```

Les flags sont convertis à la frontière de la CLI ; le reste du programme ne
manipule que le type validé.

### Hash et vérification d'intégrité

Le format de données devrait distinguer explicitement un *quick hash* d'un
SHA-256 complet. La vérification après copie devrait utiliser uniquement le
hash complet : comparer un préfixe est moins fort qu'une comparaison totale.
Cette distinction rend une reprise et un audit plus fiables.

### Registre des validateurs

Le registre avec `init()` est pratique pour découvrir le pattern. À mesure que
le projet grandit, une initialisation explicite peut être plus lisible :
`validator.NewRegistry(imageValidator, audioValidator)`. Les dépendances sont
visibles et les tests peuvent construire un registre minimal.

## Priorité 4 : approfondir les tests

Les tests unitaires constituent une bonne base. Les compléments les plus
intéressants sont :

| Type de test | Exemple adapté au projet | Ce qu'il protège |
|---|---|---|
| Intégration avec répertoires temporaires | Créer une arborescence avec `t.TempDir()`, produire un plan puis exécuter une copie. | L'interaction entre scanner, planification et système de fichiers. |
| Cas d'échec d'I/O | Simuler un fichier absent, une destination non inscriptible ou une erreur de fermeture. | Le nettoyage des fichiers partiels et les erreurs. |
| Fuzzing | Fournir des octets aléatoires aux parseurs MKV, MP4, AVI et aux patterns de renommage. | Les paniques et les boucles mal bornées. |
| Test de concurrence | Lancer plusieurs workers et vérifier avec `task test-race`. | Les accès concurrents aux compteurs, résultats et plans. |
| Benchmarks | Mesurer le hash, les métadonnées et un scan sur un jeu fixe. | Les régressions de performance. |
| Tests de CLI | Exécuter l'application sur un petit dossier fixture et vérifier le JSONL produit. | Le contrat réel des flags et des sorties. |

Le fuzzing est particulièrement formateur pour les parseurs binaires : une
fonction de fuzz ne doit jamais paniquer face à des données tronquées ou
incohérentes.

## Priorité 5 : automatiser les contrôles

Le `Taskfile.yml` couvre déjà le formatage, les tests, la détection de courses,
la couverture, `go vet` et le build. Les évolutions suivantes le complètent :

- utiliser ponctuellement `go test -shuffle=on ./...` pour révéler les tests
  dépendants de l'ordre d'exécution ;
- utiliser `go test -count=1 ./...` pendant une investigation pour ignorer le
  cache ;
- ajouter `staticcheck` lorsque les règles du projet sont stabilisées ;
- vérifier `go mod tidy` dans une intégration continue ;
- lancer au minimum `task` et `task build` dans une CI, idéalement sur Windows
  et Linux si la portabilité est un objectif.

Un linter est utile s'il améliore la compréhension et la fiabilité, pas s'il
encourage des modifications mécaniques sans compréhension.

## Priorité 6 : améliorer l'expérience CLI et les données

| Idée | Bénéfice |
|---|---|
| Sous-commandes (`scan`, `duplicates`, `move`, `report`) | Les combinaisons de flags deviennent plus simples à comprendre et valider. |
| Fichier de configuration | Les réglages récurrents peuvent être versionnés et réutilisés. |
| Version du schéma JSONL | Un champ `schema_version` permet de faire évoluer l'export sans casser une reprise ancienne. |
| Filtres d'exclusion | Éviter de scanner la destination, des dossiers système ou des extensions inutiles. |
| Résumé final JSON | Le résultat est utilisable par des scripts, une CI ou Wails. |
| Documentation des liens | Les hardlinks dépendent du volume et les symlinks ont des droits différents selon l'OS. |

## Priorité 7 : performance, après mesure

Le projet emploie déjà workers, channels et pool de buffers. Avant toute
optimisation, ajouter un jeu de données représentatif et mesurer avec des
benchmarks ou `pprof`. Les questions à examiner sont :

- le disque est-il saturé avant le CPU ?
- combien de mémoire consomment les résultats et plans très grands ?
- le nombre de workers optimal change-t-il selon un HDD, un SSD ou un disque
  réseau ?
- faut-il limiter séparément lecture, hash et écriture ?

Un réglage unique de workers est simple ; des limites distinctes peuvent devenir
utiles si les phases du pipeline ont des coûts très différents.

## Fonctionnalités qui pourraient être ajoutées

Les idées ci-dessous sont des évolutions du produit, pas seulement des
améliorations internes. Elles sont regroupées afin de choisir une prochaine
fonctionnalité selon le problème réel à résoudre.

### Récupération et fiabilité des fichiers

| Fonctionnalité | Utilité | Difficulté |
|---|---|---|
| Quarantaine des fichiers suspects | Copier ou déplacer les fichiers corrompus dans un dossier dédié plutôt que de les mélanger aux fichiers valides. | Faible |
| Rapport des fichiers non reconnus | Produire une liste des extensions, signatures inconnues et tailles pour améliorer les détecteurs. | Faible |
| Reprise d'un déplacement | Rejouer uniquement les opérations inachevées après une interruption, grâce au journal d'opérations. | Moyenne |
| Annulation d'un déplacement | Restaurer les fichiers déplacés à partir du journal, quand les chemins n'ont pas été réutilisés. | Moyenne |
| Conservation des attributs | Préserver permissions, dates, attributs cachés et, lorsque possible, ACL. | Moyenne |
| Politique pour les liens symboliques | Choisir explicitement de suivre, ignorer ou traiter comme un lien les symlinks rencontrés pendant le scan. | Moyenne |
| Détection de fichiers incomplets | Identifier des séries de fragments, des tailles anormalement faibles ou des en-têtes présents sans fin de fichier valide. | Élevée |

### Recherche, filtrage et tri

| Fonctionnalité | Utilité | Difficulté |
|---|---|---|
| Filtres avancés | Inclure ou exclure selon le type, la taille, la date, le dossier, la validité ou la présence de métadonnées. | Faible |
| Règles de classement personnalisées | Permettre des règles comme « photos de 2024 vers `Photos/2024` » ou « PDF vers `Documents` ». | Moyenne |
| Prévisualisation du plan | Exporter le plan en JSON, CSV ou tableau avant toute écriture. | Faible |
| Recherche dans le JSONL | Rechercher un fichier par nom, hash, date, appareil photo ou catégorie sans refaire un scan. | Moyenne |
| Index local | Stocker les résultats dans SQLite pour filtrer rapidement plusieurs millions de fichiers. | Élevée |
| Détection de noms similaires | Regrouper `IMG_0001`, `IMG_0001 (1)` et variantes pour aider au nettoyage manuel. | Moyenne |
| Étiquettes manuelles | Ajouter des tags comme `à_conserver`, `à_vérifier` ou `famille`. | Moyenne |

### Doublons et similarité

| Fonctionnalité | Utilité | Difficulté |
|---|---|---|
| Choix interactif du fichier à garder | Proposer les métadonnées et chemins de chaque groupe de doublons avant une action. | Moyenne |
| Regroupement par contenu proche | Identifier des photos redimensionnées ou réencodées qui ne partagent pas le même SHA-256. | Élevée |
| Perceptual hash d'images | Détecter des images visuellement semblables avec des hashes comme dHash ou pHash. | Élevée |
| Détection de vidéos similaires | Comparer quelques images-clés ou métadonnées plutôt que le fichier entier. | Élevée |
| Gestion des doublons par règles | Toujours garder, par exemple, le fichier avec la meilleure résolution, les métadonnées EXIF ou le chemin prioritaire. | Moyenne |
| Rapport d'espace récupérable | Simuler plusieurs stratégies de déduplication et comparer l'espace économisé. | Faible |

### Métadonnées et formats

| Fonctionnalité | Utilité | Difficulté |
|---|---|---|
| Plus de formats RAW | Reconnaître CR2/CR3, NEF, ARW, DNG et leurs métadonnées utiles. | Moyenne |
| Plus de formats de documents | Extraire titre, auteur, date et pages depuis PDF, Office ou EPUB. | Moyenne |
| Métadonnées audio enrichies | Lire artiste, album, piste et jaquette pour une organisation musicale plus pertinente. | Moyenne |
| OCR optionnel | Rechercher du texte dans des scans ou des captures d'écran. | Élevée |
| Géolocalisation | Organiser les photos par pays, ville ou coordonnées GPS lorsqu'elles sont présentes. | Moyenne |
| Fuseaux horaires et dates incertaines | Signaler les dates ambiguës et permettre de définir une règle de correction. | Moyenne |

### Rapports et automatisation

| Fonctionnalité | Utilité | Difficulté |
|---|---|---|
| Rapport JSON de synthèse | Rendre les résultats faciles à exploiter par un script, une CI ou Wails. | Faible |
| Export CSV | Ouvrir les résultats dans un tableur pour un tri manuel. | Faible |
| Comparaison de deux scans | Voir les fichiers ajoutés, disparus ou modifiés entre deux exécutions. | Moyenne |
| Mode surveillance | Scanner automatiquement les nouveaux fichiers d'un dossier à intervalles réguliers. | Moyenne |
| Hooks post-traitement | Exécuter une commande après un scan ou un déplacement réussi. | Moyenne |
| Notifications | Prévenir lorsque le scan est fini ou lorsqu'un grand nombre d'erreurs survient. | Faible |
| Export de métriques | Exposer nombre de fichiers, erreurs, débit et espace économisé pour suivi. | Moyenne |

### Interface Wails

Ces fonctionnalités deviennent particulièrement intéressantes avec une
interface graphique, mais le cœur métier doit rester utilisable sans elle :

- sélection de source et de destination avec validation immédiate ;
- aperçu de photos, métadonnées et groupes de doublons ;
- tableau filtrable des résultats du scan ;
- comparaison visuelle avant de choisir le fichier à conserver ;
- suivi de progression, annulation et journal des erreurs ;
- éditeur de règles de classement et de modèles de renommage ;
- assistant pas à pas : scanner, examiner, simuler, puis appliquer.

Pour l'architecture de cette interface, voir [WAILS_GUIDE.md](WAILS_GUIDE.md).

### Choisir une prochaine fonctionnalité

Pour progresser en Go sans bloquer le projet, choisir une idée qui possède :

1. une entrée simple (un dossier, un JSONL ou des options) ;
2. un résultat observable et testable ;
3. une modification limitée à un ou deux packages ;
4. un bénéfice réel pour ton propre usage.

Un bon premier choix serait le rapport des fichiers non reconnus, les filtres
avancés, l'export CSV ou le rapport d'espace récupérable. Ils réutilisent les
données déjà produites par le scan et permettent de pratiquer les structs, le
parsing de flags, les fichiers et les tests, sans risquer de modifier les
fichiers source.

## Ordre conseillé pour apprendre et avancer

1. Ajouter des tests d'intégration autour de `mover` avec `t.TempDir()`.
2. Sécuriser les chemins source/destination et les copies temporaires.
3. Faire passer `context.Context` du point d'entrée aux workers.
4. Extraire un package `app` qui retourne des résultats au lieu d'imprimer.
5. Remplacer progressivement les chaînes d'options par des types validés.
6. Ajouter fuzzing et benchmarks aux parseurs sensibles.
7. Construire ensuite l'adaptateur Wails sur ce cœur métier.

L'objectif n'est pas de tout appliquer immédiatement. Chaque étape est
indépendante, testable et introduit un concept Go concret.
