# Patterns utilisés dans FileSorter

## Pipeline

Le scan transforme progressivement un chemin :

```
chemin -> type -> métadonnées -> validation -> catégorie -> résultat -> JSONL
```

Chaque étape enrichit `types.Result`. Cette séparation rend les fonctions testables.

## Worker pool

Utilisé dans `scanner`, `dedup`, `validator` et `mover`.

Un nombre fixe de workers consomme des jobs depuis un channel. Cela limite la pression sur le CPU et le disque.

```go
jobs := make(chan Job)
for i := 0; i < workers; i++ {
    go worker(jobs)
}
```

Risques : oublier de fermer le channel, envoyer après fermeture, bloquer sur un channel de résultats ou lancer trop de workers.

## Fan-out / fan-in

Le travail est distribué vers plusieurs workers, puis leurs résultats sont regroupés. `dedup.parallelHash` illustre ce modèle.

La map de résultats est protégée par un mutex car plusieurs workers l'écrivent.

## Producer / consumer

Le scanner produit des chemins et les workers les consomment. L'exporteur consomme les résultats. Les buffers de channels absorbent les différences de vitesse.

## Registry et auto-enregistrement

`validator` possède un registre de validateurs :

```go
func init() {
    Register(&ImageValidator{})
}
```

Avantage : un nouveau format s'ajoute sans modifier un grand `switch`. Limite : l'état global et `init` rendent les dépendances moins explicites.

## Strategy pattern

Les stratégies de déduplication et de conflits sont choisies à l'exécution :

- premier, plus court, plus ancien ou plus récent ;
- incrément, skip, overwrite, hash ou timestamp.

Le pipeline commun reste le même, seul le choix change.

## Plan puis exécution

`mover` sépare la génération d'un `Plan`, la prévisualisation et l'exécution. C'est adapté aux opérations dangereuses et à une future interface Wails.

## Dry-run

Le dry-run calcule et affiche les opérations sans écrire, supprimer, déplacer ou créer de lien. C'est un pattern de sécurité essentiel pour une CLI.

## Rollback

La déduplication crée un lien temporaire, sauvegarde l'ancien fichier, installe le remplacement puis supprime la sauvegarde.

```
créer le lien temporaire
        |
sauvegarder l'ancien fichier
        |
installer le lien final
        |
supprimer la sauvegarde
```

Si l'installation échoue, l'ancien fichier peut être restauré.

## Idempotence

`checkpoint.ProcessedFiles.Add` n'incrémente le compteur qu'une fois pour un même chemin. Une opération idempotente peut être répétée sans créer un état incohérent.

C'est important pour le mode resume.

## Adapter par interface

```go
type SkipChecker interface {
    IsProcessed(path string) bool
}
```

Le scanner dépend du comportement, pas de l'implémentation JSONL du checkpoint.

## Composition

Go privilégie la composition à l'héritage :

- un `Result` contient des métadonnées ;
- un `Plan` contient des opérations ;
- un validateur satisfait une interface ;
- un executor utilise un plan.

## Patterns à améliorer

Pour la future interface Wails :

- remplacer les `fmt.Printf` métier par des résultats et événements ;
- ajouter `context.Context` aux opérations longues ;
- injecter les dépendances système pour simuler les erreurs ;
- remplacer les chaînes d'action par des types plus stricts ;
- rendre les opérations annulables.

