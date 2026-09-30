# ADR 0002 : Extraire l'orchestration scan et export

- **Statut :** Acceptée
- **Date :** 2026-10-01

## Contexte

Le CLI coordonnait directement le scan parallèle, le consommateur du canal de résultats, l'écriture JSONL et la fermeture de l'exporteur. Cette coordination mélangeait le cycle de vie du pipeline avec la configuration des flags, l'affichage terminal et les rapports. Elle était aussi responsable d'attendre le consommateur et de fermer correctement l'exporteur après une annulation.

Les packages `scanner` et `exporter` fournissent déjà les primitives nécessaires, mais aucun cas d'usage ne les orchestre en dehors du point d'entrée CLI. Il faut pouvoir tester cette coordination sans démarrer le CLI ni simuler les interactions terminal.

## Décision

Créer le package `app` et y extraire le cas d'usage `ScanAndExport`, qui possède le cycle de vie du scan et de son export JSONL.

`ScanRequest` regroupe les paramètres du cas d'usage : répertoire source, chemin d'export, mode append, nombre de workers, options de scan et callbacks facultatifs de progression et de résultat. `ScanOutcome` retourne les statistiques, le nombre de résultats consommés et une erreur d'export éventuelle.

`ScanAndExport` ouvre un exporteur neuf ou en append, démarre un consommateur concurrent des résultats, exécute le scan avec le contexte fourni, attend la fermeture du canal de résultats, puis ferme l'exporteur avant de retourner. Une erreur de scan est retournée séparément; une erreur d'écriture est conservée dans `ScanOutcome.ExportError`. L'échec d'une écriture n'empêche pas le consommateur de drainer les résultats restants.

Les callbacks sont des adaptateurs fournis par l'appelant. Ils permettent au CLI de mettre à jour la progression, d'alimenter la détection de doublons et de collecter les résultats du rapport HTML sans faire dépendre `app` de ces fonctions de présentation. Le callback de résultat est exécuté par le consommateur, en série avec l'export des résultats.

Le CLI garde la responsabilité du précomptage, des options de commande, des messages et rapports affichés, de l'installation du contexte Ctrl+C, ainsi que de la confirmation des opérations du mover. Cette première extraction ne déplace pas les rapports ni l'exécution du mover.

## Conséquences

### Positives

- Le cycle scan/export est réutilisable sans dépendre du package `main`.
- Les canaux, le consommateur, l'attente de fin et la fermeture du fichier sont testables ensemble.
- La fermeture de l'exporteur est garantie après un scan réussi ou annulé, une fois le consommateur terminé.
- Les callbacks gardent les intégrations de progression, de doublons et de rapport HTML hors de l'orchestration générique.
- Le package `app` dépend de `scanner`, `exporter` et `types`, sans introduire de dépendance inverse depuis ces packages.

### Coûts et limites

- `ScanOutcome.ExportError` doit être vérifié séparément de l'erreur retournée par `ScanAndExport`.
- Les callbacks sont invoqués depuis le consommateur; un futur callback bloquant ou non thread-safe devra respecter ce contrat d'exécution.
- Les paramètres du cas d'usage sont regroupés dans des structures publiques, ce qui crée un contrat d'API à maintenir.
- Le précomptage et les autres phases du CLI ne font pas encore partie du package `app`.

## Alternatives considérées

- **Laisser toute la coordination dans `main.go` :** rejetée, car le cycle de vie des canaux et de l'export ne pouvait pas être testé comme un cas d'usage autonome.
- **Déplacer toute la fonction `main` dans `app` :** rejetée, car les flags, l'affichage, les rapports et la confirmation relèvent de l'adaptateur CLI et élargiraient inutilement la première extraction.
- **Faire dépendre `scanner` de `exporter` :** rejetée, car l'orchestration doit rester au-dessus des deux packages et éviter une dépendance de bas niveau vers l'export.

## Vérification

Les tests du package `app` vérifient l'export d'un résultat JSONL et les statistiques retournées, puis l'annulation en cours de scan avec validation de toutes les lignes exportées avant le retour. Les contrôles `go test ./...`, `go vet ./...` et `go build ./...` passent.

## Références d'implémentation

- `app/scan.go` : requête, résultat et orchestration scan/export.
- `app/scan_test.go` : export normal et fermeture après annulation.
- `main.go` : assemblage des callbacks et conservation des responsabilités CLI.
- `docs/adr/0001-annulation-cooperative-avec-context.md` : décision antérieure sur la propagation de l'annulation.