# Guide Go appliqué à FileSorter

## 1. Modules et packages

`go.mod` définit le module et ses dépendances. Chaque dossier Go est généralement un package importable.

Une déclaration qui commence par une majuscule est exportée :

```go
hash, err := dedup.ComputeFullHash(path)
```

Une déclaration minuscule reste privée au package, comme `processFile`.

## 2. Variables, constantes et enums

```go
const DefaultMinSize = 1 * 1024 * 1024
size := int64(1024)
```

`:=` déclare une variable dans une fonction. `iota` permet de créer des valeurs séquentielles :

```go
type Action int

const (
    Delete Action = iota
    Hardlink
    Symlink
)
```

## 3. Structs et tags JSON

Les données du pipeline sont des structs :

```go
type Result struct {
    Path string `json:"path"`
    Size int64  `json:"size"`
}
```

Les tags contrôlent la sérialisation JSON. Les initialisations nommées sont préférables aux initialisations positionnelles :

```go
result := types.Result{Path: path, Size: info.Size()}
```

## 4. Pointeurs

Un pointeur permet à une fonction de modifier la valeur reçue :

```go
func EnrichImage(result *types.Result) {
    result.Type = "jpg"
}
```

Un pointeur peut être `nil`. Il faut le vérifier avant de le déréférencer.

Utiliser un pointeur pour modifier une struct ou représenter l'absence d'une valeur. Utiliser une valeur pour de petites données copiables.

## 5. Méthodes

Une méthode possède un receiver :

```go
func (p *Plan) AddOperation(op Operation) {
    p.Operations = append(p.Operations, op)
}
```

Le receiver pointeur permet de modifier le plan. Le receiver valeur est adapté aux types petits et immuables.

## 6. Interfaces

Une interface décrit un comportement :

```go
type Validator interface {
    Validate(path string) ValidationResult
    SupportedExtensions() []string
}
```

Un type satisfait automatiquement une interface s'il possède ses méthodes. Il n'existe pas de mot-clé `implements`.

Dans `validator`, cela permet d'ajouter un nouveau format sans modifier le code appelant.

## 7. Erreurs

Les erreurs sont des valeurs retournées explicitement :

```go
data, err := os.ReadFile(path)
if err != nil {
    return nil, fmt.Errorf("lecture de %s: %w", path, err)
}
```

`%w` conserve la cause. Elle peut être inspectée avec `errors.Is` ou `errors.As`.

Bonnes pratiques :

- vérifier l'erreur immédiatement ;
- ajouter un contexte utile ;
- ne jamais remplacer silencieusement une erreur par `nil` ;
- utiliser une erreur typée lorsque l'appelant doit distinguer plusieurs cas.

`validator.ValidationError` transporte un type, un message et un offset dans le fichier.

## 8. defer et ressources

```go
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()
```

`defer` garantit la fermeture à la sortie de la fonction, y compris après un `return`. Il est aussi utilisé pour libérer un mutex ou rendre un buffer à un pool.

## 9. Fichiers et chemins

Utiliser `os` pour les fichiers et `filepath` pour les chemins système :

```go
destination := filepath.Join(root, "images", "2024", "photo.jpg")
```

Ne pas concaténer les chemins avec `/` ou `\\` lorsqu'il s'agit d'un chemin système. Les chemins logiques d'un format interne peuvent avoir une convention distincte, mais cela doit être documenté.

## 10. JSONL et streaming

JSONL signifie JSON Lines : un objet JSON par ligne. `exporter` écrit chaque résultat immédiatement, ce qui évite de charger des millions de résultats en mémoire.

```go
for result := range results {
    data, err := json.Marshal(result)
    if err != nil {
        return err
    }
    writer.Write(data)
    writer.WriteByte('\n')
}
```

## 11. Goroutines et channels

Une goroutine exécute une fonction concurremment :

```go
go process(path)
```

Un channel transporte des valeurs :

```go
jobs := make(chan string)
jobs <- path
path := <-jobs
close(jobs)
```

Une boucle `for value := range channel` se termine lorsque le channel est fermé. Le producteur doit généralement fermer le channel.

## 12. Synchronisation

### WaitGroup

Attend plusieurs goroutines :

```go
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    work()
}()
wg.Wait()
```

### Mutex et RWMutex

Protègent une map ou une struct partagée :

```go
mu.Lock()
defer mu.Unlock()
sharedMap[key] = value
```

`RWMutex` permet plusieurs lecteurs simultanés avec `RLock`.

### Atomic

Convient aux compteurs simples :

```go
atomic.AddInt64(&processed, 1)
```

Utiliser un mutex lorsqu'une opération doit modifier plusieurs valeurs de façon cohérente.

### sync.Pool

Réutilise des buffers temporaires dans les copies et les hash. Toujours rendre l'objet avec `Put`.

## 13. Tests

Les tests se trouvent dans des fichiers `_test.go` :

```go
func TestReadableSize(t *testing.T) {
    if got := ReadableSize(1024); got != "1.00 KB" {
        t.Fatalf("got %q", got)
    }
}
```

Le projet utilise :

- `t.TempDir()` pour isoler les fichiers ;
- des tests table-driven ;
- des tests d'erreurs ;
- `errors.Is` pour les erreurs enveloppées ;
- `os.SameFile` pour vérifier un hardlink ;
- des tests de dry-run.

## 14. Outils de qualité

- `gofmt` : formatage standard ;
- `go vet` : erreurs probables ;
- `go test -race` : accès concurrents dangereux ;
- `go test -cover` : couverture des instructions.

## 15. Dépendances et bibliothèques

Le projet utilise notamment :

- `filetype` pour les magic bytes ;
- `goexif` pour les métadonnées EXIF ;
- `dhowden/tag` pour les tags audio ;
- `progressbar` pour l'affichage de progression.

Une dépendance doit résoudre un besoin clair. Il faut vérifier ses erreurs, limiter son périmètre et éviter de coupler toute l'application à une bibliothèque.

