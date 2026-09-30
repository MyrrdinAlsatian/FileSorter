# ADR 0005 : Journalisation incrémentale des opérations

- **Statut :** Acceptée
- **Date :** 2026-10-01

## Contexte

Le premier journal du mover écrivait un instantané des résultats après le retour de `ExecuteContext`. Si le processus s'arrêtait avant cette écriture finale, les résultats des opérations déjà terminées n'étaient pas conservés.

Le mover exécute plusieurs opérations en parallèle. Le journal doit donc accepter des ajouts concurrents sans entremêler les lignes, tout en conservant la distinction entre le résultat d'une opération fichier et une erreur d'écriture du journal.

## Décision

Lorsque `--move-journal <PATH>` est fourni, le CLI ouvre un journal JSONL en ajout avant de lancer l'exécuteur. Il ne tronque pas un journal existant. L'exécuteur ajoute une entrée lorsqu'une opération atteint un état terminal : succès, échec ou opération ignorée. Chaque entrée contient la version du schéma, l'horodatage, le mode, les chemins source et destination, la taille, le statut et, si présent, le message d'erreur.

`OperationJournal` protège l'écriture avec un mutex et écrit chaque entrée comme une ligne JSON complète. Les entrées apparaissent dans l'ordre où les opérations terminent leur traitement; avec plusieurs workers, cet ordre n'est pas l'ordre du plan.

Si le journal ne peut pas être ouvert, le CLI n'exécute pas le plan. Si une écriture échoue en cours d'exécution, l'erreur est exposée séparément dans `ExecutionResult.JournalError`; elle ne transforme pas une opération fichier réussie en échec. L'exécuteur cesse les tentatives d'écriture ultérieures après la première erreur, mais continue les opérations du plan.

`AppendOperationJournal` reste disponible pour ajouter après coup un instantané d'un `ExecutionResult`, notamment pour les appelants qui ne gèrent pas un exécuteur avec writer partagé.

## Conséquences

### Positives

- Les opérations déjà terminées sont inscrites au fil du traitement, sans attendre la fin de tout le plan.
- Les écritures de plusieurs workers sont sérialisées en lignes JSONL indépendantes.
- Un problème de journalisation est observable sans fausser les compteurs ou le statut réel des opérations de fichiers.
- L'ajout en mode append permet de conserver les résultats de plusieurs exécutions dans le même fichier.

### Coûts et limites

- Le journal n'est pas encore un mécanisme de reprise : aucune lecture ne reconstruit ni ne rejoue un plan.
- Une opération interrompue avant son état terminal n'a pas d'entrée finale dans le journal. La dernière opération en cours peut donc nécessiter une inspection manuelle après un arrêt brutal.
- Chaque entrée est écrite avec `os.File.Write`, sans `Sync` systématique. Cela évite une synchronisation disque coûteuse à chaque fichier, mais ne garantit pas la persistance sur support en cas de panne système ou de perte d'alimentation.
- Si l'écriture échoue, les opérations continuent mais les entrées suivantes ne sont plus tentées; le CLI signale `JournalError` à la fin.
- L'ordre des lignes reflète les terminaisons concurrentes et peut varier d'une exécution à l'autre.

## Alternatives considérées

- **Écrire uniquement après `ExecuteContext` :** rejetée pour le chemin CLI, car un arrêt avant la sérialisation finale perdrait les résultats déjà obtenus.
- **Synchroniser physiquement chaque ligne :** rejetée par défaut, car le coût d'une synchronisation par fichier pénaliserait fortement les gros plans. Une option de durabilité renforcée pourra être évaluée séparément.
- **Marquer l'opération fichier en échec quand le journal échoue :** rejetée, car une erreur d'audit ne change pas le résultat réel d'une copie ou d'un déplacement.
- **Rejouer automatiquement le journal :** reportée; une reprise devra définir les règles de validation des fichiers et des chemins avant de modifier ou supprimer une source.

## Vérification

Les tests vérifient l'ajout de plusieurs entrées, l'écriture concurrente de succès, d'échec et d'opération ignorée, ainsi que le signalement distinct d'une erreur d'écriture sans requalification d'une copie réussie. `go test ./...`, `go vet ./...` et `go build ./...` ont réussi pour cette évolution.

## Références d'implémentation

- `mover/journal.go` : format JSONL et writer append concurrent.
- `mover/executor.go` : ajout des entrées aux transitions terminales et collecte de `JournalError`.
- `mover/mover.go` : résultat structuré d'exécution.
- `main.go` et `utils/flags.go` : option `--move-journal` et cycle de vie du writer.
- `mover/mover_test.go` : tests du journal append et de l'intégration aux workers.
- `docs/IMPROVEMENTS.md` : feuille de route mise à jour.