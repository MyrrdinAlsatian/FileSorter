# ADR 0003 : Types métier pour les modes du mover

- **Statut :** Acceptée
- **Date :** 2026-10-01

## Contexte

Le mover représente déjà son mode d'opération par le type défini `mover.Mode`, mais les choix de comportement restent exprimés par des chaînes aux frontières : `utils.Options.MoveMode` et `MoveOverwrite` reçoivent directement les valeurs des flags CLI. La politique de conflit est aussi stockée comme `string` dans `mover.Options` et comparée à des littéraux dans le plan et l'exécuteur.

Cette représentation laisse passer les valeurs inconnues. En particulier, un mode d'opération mal orthographié pouvait retomber sur le mode copie dans le CLI; une politique de conflit inconnue n'avait pas de comportement métier explicite.

## Décision

Conserver `mover.Mode` comme type métier défini et ajouter `mover.OverwriteMode` pour les stratégies de conflit. Les options du mover utilisent ces types plutôt que des chaînes libres. Les valeurs prises en charge sont représentées par des constantes nommées : `ModeCopy`, `ModeMove`, `ModeHardlink`, `ModeSymlink`, `ConflictSkip`, `ConflictOverwrite` et `ConflictRename`.

Ajouter `ParseMode` et `ParseOverwriteMode` pour convertir les chaînes des flags en valeurs métier validées. Ces parseurs acceptent uniquement les valeurs connues et retournent une erreur explicite pour toute autre valeur. Le CLI effectue cette conversion avant de construire `mover.Options` et avant de planifier ou exécuter les opérations.

La structure `utils.Options` conserve des champs `string` : elle représente les entrées textuelles de la ligne de commande et n'a pas à dépendre des types métier du mover. Les branches du plan, de l'exécuteur et de la confirmation utilisent les constantes typées plutôt que de répéter des littéraux.

## Conséquences

### Positives

- Le contrat des options métier est plus lisible et différencié des chaînes ordinaires.
- Les valeurs CLI inconnues sont signalées au lieu d'être interprétées comme un comportement implicite.
- Les politiques de conflit utilisent les mêmes constantes dans le plan et l'exécution.
- La couche générique des flags reste découplée du package `mover`.

### Coûts et limites

- Les chaînes provenant de la CLI doivent être converties explicitement avant usage.
- En Go, un type défini sur `string` n'est pas un enum fermé : un appelant peut toujours construire une valeur arbitraire par conversion explicite. Les parseurs protègent les entrées textuelles standard, mais les API publiques doivent continuer à traiter leurs options comme des données à valider.
- Les tests et intégrations qui construisent `mover.Options` doivent utiliser les constantes typées.

## Alternatives considérées

- **Garder les chaînes et valider uniquement dans `ParseFlags` :** rejetée, car la validation serait liée au CLI et les consommateurs directs du package mover garderaient des champs de type trop large.
- **Faire dépendre `utils` de `mover` pour typer directement les flags :** rejetée, car les utilitaires de parsing ne doivent pas dépendre d'un cas métier particulier.
- **Remplacer les chaînes par des entiers avec `iota` :** rejetée, car les valeurs textuelles correspondent directement aux options CLI et sont plus faciles à afficher et à diagnostiquer.

## Vérification

Les tests vérifient les valeurs acceptées par les parseurs ainsi que le rejet des valeurs inconnues. Les tests mover et CLI vérifient que les politiques typées préservent la confirmation des actions destructrices et les comportements existants. Les contrôles `go test ./...`, `go vet ./...` et `go build ./...` passent.

## Références d'implémentation

- `mover/mover.go` : types, constantes et parseurs.
- `mover/plan.go` et `mover/executor.go` : décisions selon les constantes typées.
- `main.go` : conversion des chaînes de flags en options métier.
- `mover/mover_test.go` et `main_test.go` : parseurs et décisions de confirmation.