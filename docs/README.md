# Documentation pédagogique

Cette documentation explique FileSorter et les concepts de Go utilisés dans le projet.

## Parcours conseillé

1. [Architecture du projet](ARCHITECTURE.md)
2. [Guide Go appliqué au projet](GO_GUIDE.md)
3. [Patterns utilisés](PATTERNS.md)
4. [Parcours d'apprentissage](LEARNING_PATH.md)
5. [Guide Wails séparé](WAILS_GUIDE.md)
6. [Utiliser le Taskfile](TASKFILE.md)

## Packages importants

| Package | Responsabilité | Concepts |
|---|---|---|
| `types` | Structures partagées | structs, pointeurs, tags JSON |
| `scanner` | Parcours parallèle | goroutines, channels, WaitGroup |
| `detector` | Détection de formats | fichiers, buffers, fallbacks |
| `metadata` | Métadonnées | parsing binaire, dates |
| `dedup` | Doublons | SHA-256, mutex, stratégies |
| `mover` | Copie et déplacement | plans, rollback, concurrence |
| `validator` | Intégrité | interfaces, registre |
| `exporter` | JSONL | streaming, channels |
| `renamer` | Noms de fichiers | parsing, stratégies |

## Commandes

```bash
go test ./...
go test -v ./dedup
go test -race ./...
go test -cover ./...
go vet ./...
gofmt -w .
```

Sous Windows, `go test -race` nécessite un compilateur C. Les tests classiques restent utiles si cet outil manque.

## Taskfile

Le fichier `Taskfile.yml` regroupe les commandes de développement. Avec [Task](https://taskfile.dev/) installé :

```bash
task
task test
task test-race
task coverage
task coverage-html
task build
task clean
```

`task` lance le formatage, les tests et `go vet`. Le task `test-race` peut nécessiter un compilateur C sous Windows.
