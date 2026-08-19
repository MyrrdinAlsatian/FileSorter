# Parcours d'apprentissage Go avec FileSorter

## Étape 1 : fonctions simples

Packages : `utils`, `metadata`.

Exercices : ajouter une unité à `ReadableSize`, créer `IsDocumentType`, tester zéro et les seuils exacts.

Notions : types, `switch`, fonctions, tests table-driven.

## Étape 2 : structs et JSON

Packages : `types`, `exporter`, `checkpoint`.

Exercices : ajouter un champ optionnel à `types.Result`, vérifier son export JSON et préserver la compatibilité JSONL.

Notions : structs, tags JSON, streaming, compatibilité.

## Étape 3 : erreurs

Packages : `detector`, `validator`, `metadata`.

Exercices : envelopper une erreur avec `%w`, tester `errors.Is` ou `errors.As`, distinguer fichier absent, dossier et fichier vide.

## Étape 4 : interfaces

Package : `validator`.

Exercices : créer un validateur pour un format fictif, l'enregistrer et tester la recherche par extension et MIME.

Notions : interfaces implicites, composition, registre.

## Étape 5 : concurrence

Packages : `scanner`, `validator`, `dedup`.

Exercices : ajouter un compteur de progression, vérifier le callback, exécuter `go test -race`, comparer traitement séquentiel et worker pool.

Notions : goroutines, channels, WaitGroup, mutex, atomic, data race.

## Étape 6 : fichiers et sécurité

Packages : `mover`, `dedup`.

Exercices : tester les conflits, les copies incomplètes, le dry-run et la restauration après échec.

Notions : `os`, chemins, opérations atomiques, rollback, invariants.

## Étape 7 : parsing binaire

Packages : `metadata/avi`, `metadata/mkv`, `metadata/mp4`.

Exercices : construire un petit buffer binaire, tester une taille invalide et un fallback sur le nom du fichier.

Notions : bytes, endianess, `io.Reader`, formats conteneurs.

## Étape 8 : préparer Wails

Les opérations longues devraient accepter un `context.Context` :

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

Il faudra séparer le cœur métier, la CLI qui affiche, et l'API Wails qui publie des événements UI.

## Routine recommandée

1. écrire ou mettre à jour un test ;
2. implémenter le comportement minimal ;
3. lancer `gofmt` ;
4. lancer `go test ./...` ;
5. lancer `go vet ./...` ;
6. lancer `go test -race ./...` pour la concurrence ;
7. relire les erreurs et les chemins.

## Questions à se poser

- Qui possède cette donnée ?
- Que se passe-t-il si l'appel système échoue ?
- Qui ferme ce channel ?
- Quelle goroutine possède cette map ?
- L'opération peut-elle être répétée ?
- Le test dépend-il de l'OS ou de l'ordre d'exécution ?
- Cette fonction fait-elle trop de choses ?

