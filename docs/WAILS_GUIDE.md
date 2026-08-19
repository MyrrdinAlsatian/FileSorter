# Guide Wails pour FileSorter

Ce document concerne uniquement l'intégration future de Wails. Les concepts généraux du langage Go sont documentés dans [GO_GUIDE.md](GO_GUIDE.md) et [GO_ADVANCED.md](GO_ADVANCED.md).

## 1. Rôle de Wails

Wails permet d'emballer une application Go avec une interface frontend. Le backend reste du Go et le frontend utilise des technologies web.

Pour FileSorter, Wails pourrait fournir :

- sélection de dossiers ;
- aperçu du plan de déplacement ;
- progression du scan ;
- annulation d'une opération ;
- confirmation avant suppression ou déduplication ;
- affichage des erreurs et des doublons.

## 2. Architecture recommandée

```text
core/
  scanner/
  detector/
  dedup/
  mover/

app/
  app.go          API exposée à Wails
  events.go       progression et logs

cmd/
  filesorter/     interface CLI

frontend/         interface utilisateur
```

Le cœur métier ne doit pas importer Wails. Il doit retourner des résultats Go, des erreurs et des événements abstraits.

La CLI et Wails deviennent deux interfaces différentes au-dessus du même cœur.

## 3. Struct App

Wails expose généralement les méthodes publiques d'une struct applicative :

```go
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) ScanDirectory(path string) (*ScanReport, error) {
	return nil, nil
}
```

Les méthodes appelées depuis le frontend doivent avoir des arguments et des retours facilement sérialisables : strings, nombres, booléens, slices et structs simples.

Éviter d'exposer directement des channels, mutex, fichiers ouverts ou pointeurs internes au frontend.

## 4. Context et annulation

Une opération longue doit accepter un contexte :

```go
func (a *App) ScanDirectory(path string) error {
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	return scan(ctx, path)
}
```

Pour un vrai bouton d'annulation, l'application doit conserver l'annulation de l'opération en cours de façon protégée par un mutex :

```go
type App struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}
```

Il faut refuser ou gérer explicitement le lancement de deux scans simultanés.

## 5. Événements de progression

Une interface graphique ne doit pas attendre la fin d'un scan pour connaître son avancement. Définir un événement indépendant de Wails :

```go
type ProgressEvent struct {
	Current int
	Total   int
	Message string
	Path    string
}
```

Le cœur peut recevoir un callback ou un petit port d'événements :

```go
type ProgressFunc func(ProgressEvent)
```

La couche Wails traduit ensuite cet événement en événement frontend. Le scanner ne doit pas appeler directement une fonction Wails.

## 6. Événements et thread principal

Les workers Go peuvent produire des événements depuis plusieurs goroutines. Il faut :

- protéger l'état partagé ;
- éviter de modifier directement un état frontend depuis plusieurs goroutines ;
- centraliser la publication des événements ;
- limiter la fréquence des événements pour ne pas saturer l'interface.

Un événement par fichier peut être trop coûteux pour des millions de fichiers. Une publication toutes les 100 ou 250 ms est souvent plus adaptée.

## 7. États de l'interface

L'interface devrait représenter explicitement les états :

```text
idle -> scanning -> scan-complete
                 \-> cancelled
                 \-> failed

scan-complete -> previewing -> executing -> complete
```

Le backend doit être la source de vérité pour les compteurs et les erreurs. Le frontend ne doit pas déduire qu'une opération est terminée uniquement parce qu'une barre atteint 100 %.

## 8. Sécurité des opérations destructives

Les actions `delete`, `move`, `hardlink` et `symlink` doivent suivre ce flux :

1. analyser ;
2. générer un plan ;
3. afficher un aperçu ;
4. demander une confirmation ;
5. exécuter ;
6. afficher les erreurs individuelles ;
7. proposer une récupération lorsque c'est possible.

Le mode dry-run doit être utilisé pour alimenter l'aperçu sans modifier les fichiers.

## 9. DTO et séparation des types

Les structs internes peuvent contenir des mutex, channels ou détails d'implémentation. Il est préférable de créer des DTO dédiés à l'interface :

```go
type ScanStatus struct {
	State     string `json:"state"`
	Current   int    `json:"current"`
	Total     int    `json:"total"`
	Errors    int    `json:"errors"`
	CanCancel bool   `json:"can_cancel"`
}
```

Cela évite de coupler le frontend à `scanner.SafeStats` ou à `mover.Executor`.

## 10. Erreurs pour le frontend

Les erreurs destinées au frontend doivent être compréhensibles et, si nécessaire, structurées :

```go
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}
```

Conserver l'erreur Go originale dans les logs, mais éviter d'afficher une stack technique brute à l'utilisateur.

## 11. Dialogues et sélection de fichiers

La sélection d'un dossier, la confirmation de suppression et l'ouverture d'un rapport sont des responsabilités de l'interface. Le cœur doit recevoir un chemin déjà choisi et le valider avant utilisation.

Ne jamais considérer un chemin fourni par l'interface comme sûr sans vérifier :

- qu'il existe lorsque c'est nécessaire ;
- qu'il s'agit d'un fichier ou dossier attendu ;
- que la destination reste dans la zone autorisée ;
- que la source et la destination ne sont pas identiques.

## 12. Préparation actuelle du projet

Avant d'ajouter Wails, les améliorations utiles sont :

- supprimer progressivement les `fmt.Printf` des packages métier ;
- retourner des rapports et erreurs plutôt que d'imprimer ;
- ajouter `context.Context` aux opérations longues ;
- isoler la progression dans un callback ;
- écrire des tests sans dépendance à l'interface ;
- conserver la CLI comme test manuel du cœur métier.

## 13. Cycle de développement

1. modifier le cœur métier ;
2. ajouter un test Go ;
3. lancer `go test ./...` et `go vet ./...` ;
4. tester le mode dry-run ;
5. exposer une petite méthode dans `App` ;
6. connecter l'action frontend ;
7. tester les erreurs, l'annulation et les opérations répétées.

La règle importante est de ne pas déplacer la logique métier dans les handlers frontend.
