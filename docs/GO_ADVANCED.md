# Concepts Go intéressants à ajouter

FileSorter utilise déjà les packages, structs, interfaces, erreurs, channels, goroutines, mutex, `sync.Pool`, JSON, tests et plusieurs patterns d'architecture. Les sujets suivants ne sont pas encore réellement utilisés ou pourraient être approfondis.

## 1. `context.Context`

À utiliser pour annuler les opérations longues : scan, hash, validation, déplacement.

```go
func Scan(ctx context.Context, source string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}
```

Dans Wails, cela permettra d'ajouter un bouton « Annuler ».

Bonnes pratiques :

- le contexte est généralement le premier argument ;
- ne jamais stocker un contexte dans une struct ;
- propager `ctx` aux fonctions appelées ;
- vérifier régulièrement `ctx.Done()` dans les boucles longues.

## 2. `select`

`select` attend plusieurs événements de channels :

```go
select {
case job := <-jobs:
	process(job)
case <-ctx.Done():
	return ctx.Err()
}
```

Il est utile pour combiner travail, annulation et timeout.

## 3. Timeouts

Une opération de fichier ou de réseau ne doit pas toujours attendre indéfiniment. Un timeout peut être créé avec `context.WithTimeout` ou `time.After`.

Pour un projet de récupération de fichiers, il faut cependant distinguer un timeout d'une vraie erreur de lecture.

## 4. Generics

Les generics permettent d'écrire des fonctions réutilisables avec plusieurs types :

```go
func Contains[T comparable](values []T, wanted T) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
```

Ils sont utiles pour des helpers de collections. Il ne faut pas les utiliser uniquement pour éviter une petite duplication : une fonction spécialisée est parfois plus lisible.

## 5. Embedding

Go permet d'intégrer une struct ou une interface dans une autre :

```go
type LoggedExecutor struct {
	*Executor
	Logger *slog.Logger
}
```

L'embedding favorise la composition. Il ne doit pas servir à recréer une hiérarchie de classes.

## 6. Functional options

Pour les composants qui possèdent beaucoup d'options, on peut remplacer une grosse struct publique par des fonctions de configuration :

```go
type Option func(*Scanner)

func WithWorkers(n int) Option {
	return func(s *Scanner) { s.workers = n }
}

func NewScanner(options ...Option) *Scanner {
	s := &Scanner{workers: 4}
	for _, option := range options {
		option(s)
	}
	return s
}
```

Cette technique est intéressante si l'API Wails doit rester stable malgré l'ajout d'options.

## 7. Injection de dépendances

Le code appelle actuellement directement `os.Open`, `os.Remove` et `os.Rename`. Pour tester des erreurs difficiles à provoquer, on peut injecter une abstraction :

```go
type FileSystem interface {
	Open(string) (*os.File, error)
	Remove(string) error
	Rename(string, string) error
}
```

En pratique, il faut éviter de créer une interface gigantesque. Une petite interface appartenant au package consommateur est préférable.

## 8. `io.Reader`, `io.Writer` et `io/fs`

Les parsers utilisent déjà `io.Reader`. Cette abstraction permet de tester avec `bytes.NewReader` sans créer de fichier réel.

Le package `io/fs` permet de parcourir un système de fichiers abstrait. Il peut servir à tester le scanner avec `fstest.MapFS`.

## 9. `embed`

Le package `embed` permet d'intégrer des templates ou des ressources dans le binaire :

```go
//go:embed templates/report.html
var reportTemplate string
```

Cela pourrait rendre le rapport HTML autonome et simplifier la distribution.

## 10. Fuzzing

Les parsers de fichiers sont de bons candidats au fuzzing :

```go
func FuzzDetectPattern(f *testing.F) {
	f.Add([]byte(`{"key":"value"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = detectPattern(data)
	})
}
```

Le test doit surtout vérifier l'absence de panic et de dépassement de limites. C'est très utile pour AVI, MKV, MP4 et les détections binaires.

## 11. Benchmarks

Les benchmarks permettraient de comparer le scan séquentiel et parallèle, ou le hash rapide et complet :

```go
func BenchmarkComputeFullHash(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = ComputeFullHash(testFile)
	}
}
```

Un benchmark doit utiliser des données représentatives et éviter de mesurer uniquement la création des fixtures.

## 12. `log/slog`

Le projet utilise encore beaucoup de `fmt.Printf`. `log/slog` permettrait de produire des logs structurés :

```go
logger.Info("file processed", "path", path, "type", fileType)
```

La CLI pourrait afficher du texte tandis que Wails recevrait des événements structurés.

## 13. `errors.Join`

Quand plusieurs opérations échouent, `errors.Join` permet de conserver plusieurs erreurs au lieu de n'en retourner qu'une seule :

```go
return errors.Join(errorsList...)
```

Cela pourrait améliorer le retour d'erreurs de batch dans `mover` et `validator`.

## 14. Build tags et fichiers par OS

La vérification d'espace disque dépend du système. Les build tags permettent d'avoir une implémentation Windows et une implémentation Unix :

```go
//go:build windows
```

Les fichiers pourraient être nommés `diskspace_windows.go` et `diskspace_unix.go`.

## 15. `internal` et API publique

Un dossier `internal/` interdit l'import depuis un autre module. Les helpers qui ne font pas partie de l'API publique pourraient y être déplacés.

Une API publique doit rester petite : exposer uniquement les types et fonctions nécessaires à la CLI et à Wails.

# Bonnes pratiques à retenir

## Nommage

- préférer des noms courts mais explicites ;
- éviter `data`, `tmp` et `manager` lorsqu'un nom métier est possible ;
- nommer les erreurs avec le contexte utile ;
- commenter les symboles exportés avec leur nom.

## Gestion des erreurs

- vérifier toutes les erreurs importantes, y compris `Close`, `Flush`, `Sync`, `Chmod` et `Chtimes` ;
- ajouter le chemin du fichier dans le contexte ;
- ne pas utiliser `panic` pour une erreur utilisateur ou disque ;
- ne pas ignorer une erreur sans commentaire expliquant pourquoi.

## Concurrence

- définir clairement le propriétaire d'une map ou d'un channel ;
- fermer les channels depuis le producteur ;
- utiliser un `WaitGroup` pour toutes les goroutines lancées ;
- limiter les workers pour ne pas saturer le disque ;
- lancer `go test -race` après chaque changement concurrent.

## Fichiers et sécurité

- préférer des opérations temporaires et récupérables ;
- valider les chemins de destination ;
- éviter les suppressions irréversibles sans confirmation ;
- vérifier que la destination ne sort pas du dossier choisi ;
- ne jamais faire confiance à un chemin provenant d'un JSONL sans validation.

## Tests

- tester le comportement public avant les détails internes ;
- ajouter au moins un cas nominal, un cas limite et un cas d'erreur ;
- utiliser `t.TempDir` ;
- éviter les tests dépendants de l'heure, du disque ou de l'ordre global ;
- utiliser des tests fuzz pour les données binaires ;
- mesurer les performances avec des benchmarks plutôt qu'avec une impression subjective.

## Architecture

- séparer le cœur métier de l'affichage ;
- retourner des résultats plutôt que d'imprimer dans les packages ;
- utiliser des interfaces petites aux frontières du système ;
- conserver la CLI comme une interface parmi d'autres ;
- préparer le cœur métier à recevoir un contexte pour Wails.
