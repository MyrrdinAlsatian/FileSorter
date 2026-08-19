# Utiliser le Taskfile

Le fichier [`Taskfile.yml`](../Taskfile.yml) fournit des raccourcis pour les
commandes courantes du projet. Il évite de mémoriser les options de chaque
outil Go et rend les vérifications plus faciles à reproduire.

## Installation

Le projet utilise [Task](https://taskfile.dev/). Vérifier son installation
avec :

```bash
task --version
```

Depuis la racine du projet, une tâche se lance avec :

```bash
task <nom-de-la-tache>
```

## Tâches disponibles

| Tâche | Utilité | Quand l'utiliser |
|---|---|---|
| `task` | Lance `fmt`, `test` et `vet`. | Vérification rapide avant de considérer un changement terminé. |
| `task fmt` | Exécute `go fmt ./...` sur tous les packages. | Après avoir modifié du code Go ou avant un commit. |
| `task test` | Exécute tous les tests avec `go test ./...`. | Après chaque modification fonctionnelle. |
| `task test-verbose` | Exécute les tests avec `go test -v ./...` et affiche chaque test. | Pour comprendre quel test échoue ou suivre un scénario. |
| `task test-race` | Recherche les accès concurrents dangereux avec `go test -race ./...`. | Après une modification du scanner, des channels ou des goroutines. |
| `task coverage` | Génère `coverage.out` et affiche la couverture par fonction. | Pour repérer les parties du code peu testées. |
| `task coverage-html` | Génère `coverage.out` et `coverage.html`. | Pour explorer visuellement les lignes couvertes dans un navigateur. |
| `task vet` | Exécute `go vet ./...`. | Pour détecter des constructions Go probablement incorrectes. |
| `task build` | Compile l'application dans `filesorter.exe`. | Pour vérifier que le projet produit bien un exécutable. |
| `task check` | Lance `fmt`, `test` et `vet`. | Pour effectuer les contrôles principaux sans compiler l'exécutable. |
| `task clean` | Supprime les rapports de couverture et l'exécutable générés. | Pour nettoyer les artefacts locaux après une vérification ou un build. |

## `go fmt` : formater le code

`go fmt ./...` applique le formateur officiel de Go à tous les packages du
projet.

Le formatage concerne notamment :

- l'indentation avec des tabulations selon les conventions Go ;
- les espaces et les retours à la ligne ;
- la présentation des imports ;
- l'alignement de certaines déclarations et structures ;
- la mise en forme des littéraux et des blocs de code.

`go fmt` modifie directement les fichiers source si nécessaire. Il ne vérifie
pas que le programme est correct et ne remplace pas les tests. Son objectif
est que le code ait une présentation uniforme, quel que soit l'éditeur utilisé
par chaque contributeur.

Exemple :

```bash
go fmt ./...
```

La commande renvoie généralement les fichiers qu'elle a reformattés. Si elle
ne produit aucune sortie, les fichiers étaient déjà correctement formatés.
Dans ce projet, `task fmt` est le raccourci équivalent.

## `go vet` : détecter des erreurs probables

`go vet ./...` analyse le code compilable et signale des constructions qui
sont souvent des bugs, même si le compilateur les accepte.

Il peut notamment repérer :

- des arguments incorrects dans certains appels de formatage ;
- des `Printf` dont le format ne correspond pas aux arguments ;
- des méthodes de copie problématique de types contenant un mutex ;
- des directives ou tags de struct mal formés dans certains cas ;
- des chemins de build ou directives mal formés ;
- des erreurs liées à des usages connus de la bibliothèque standard, comme
  l'oubli d'annuler un contexte ou une réponse HTTP non fermée.

`go vet` ne prouve pas que le programme est exempt de bugs. Il ne connaît pas
toutes les règles métier et ne remplace ni les tests, ni la revue de code, ni
le compilateur. C'est un analyseur statique : il examine le code sans lancer
le programme.

Exemple :

```bash
go vet ./...
```

Si une erreur est signalée, il faut lire le fichier et la ligne indiqués,
comprendre l'avertissement, puis corriger le code ou vérifier que l'usage est
réellement intentionnel. Dans ce projet, `task vet` est le raccourci
équivalent.

## Pourquoi lancer les deux ?

Ces outils ont des rôles différents :

| Outil | Action principale | Modifie les fichiers ? | Trouve principalement |
|---|---|---:|---|
| `go fmt` | Normalise la présentation | Oui | Les différences de formatage |
| `go vet` | Analyse les constructions suspectes | Non | Les erreurs probables |

Le workflow habituel est donc de formater avant d'analyser :

```bash
task fmt
task vet
task test
```

La tâche `task` exécute déjà ces opérations avec les tests : elle lance
`fmt`, puis `test`, puis `vet`.

## Différence entre `task` et `task check`

Dans la configuration actuelle, ces deux tâches effectuent les mêmes contrôles :

```text
go fmt ./...
go test ./...
go vet ./...
```

Cette duplication est volontairement simple pour l'apprentissage. La tâche
`default` représente la commande lancée par `task` sans argument, tandis que
`check` exprime plus clairement l'intention dans un script ou une intégration
continue.

## Rapports générés

Les tâches de couverture créent deux fichiers à la racine du projet :

- `coverage.out` : données brutes utilisées par l'outil de couverture Go ;
- `coverage.html` : rapport lisible dans un navigateur, créé par
  `coverage-html`.

Ces fichiers sont des artefacts temporaires. `task clean` les supprime avec
`filesorter.exe` sous Windows.

## Parcours conseillé

Pour une modification courante :

```bash
task fmt
task test
task vet
```

Pour une modification touchant la concurrence :

```bash
task test
task test-race
```

Avant un commit important :

```bash
task
task test-race
task build
```

Si les tests échouent, `task test-verbose` aide à identifier le scénario
précis. Si la couverture semble insuffisante, `task coverage-html` permet de
voir les lignes qui ne sont pas encore exercées par les tests.

## Remarques Windows

La tâche `clean` utilise PowerShell et est donc déclarée pour Windows dans le
Taskfile. La commande `test-race` peut nécessiter un compilateur Cgo installé.
Si elle ne fonctionne pas sur la machine, les tests classiques avec `task test`
restent utilisables.
