# ADR 0005 : Journalisation incrémentale des opérations

- **Statut :** Acceptée
- **Date :** 2026-10-01

## Contexte

Le premier journal du mover écrivait un instantané des résultats après le retour de `ExecuteContext`. Si le processus s'arrêtait avant cette écriture finale, les résultats des opérations déjà terminées n'étaient pas conservés.

Le mover exécute plusieurs opérations en parallèle. Le journal doit donc accepter des ajouts concurrents sans entremêler les lignes, tout en conservant la distinction entre le résultat d'une opération fichier et une erreur d'écriture du journal.

## Décision

Lorsque `--move-journal <PATH>` est fourni, le CLI ouvre un journal JSONL en ajout avant de lancer l'exécuteur. Il ne tronque pas un journal existant. L'exécuteur ajoute une entrée lorsqu'une opération atteint un état terminal : succès, échec ou opération ignorée. Chaque entrée contient la version du schéma, l'horodatage, le mode, les chemins source et destination, la taille, le SHA-256 complet s'il est connu, le statut et, si présent, le message d'erreur.

La reprise est activée explicitement avec `--move-resume` et exige `--move-journal`; elle n'est disponible qu'en modes `copy` et `move`. Cette option force le calcul du hash pendant le scan courant. Pour que les succès de l'exécution précédente soient reprenables, celle-ci doit également avoir calculé les hash (`--hash`) afin que le journal contienne leur SHA-256. Après génération du plan, une opération n'est marquée comme reprise que si une entrée de succès correspond exactement au mode, à la source, à la destination et à la taille, si le plan et l'entrée portent le même SHA-256 complet, et si la destination existe encore comme fichier régulier dont le contenu a ce hash. Les entrées anciennes sans hash, les destinations absentes ou modifiées et les métadonnées qui ne correspondent pas restent en attente. Un journal invalide arrête la préparation au lieu d'être ignoré. Les opérations reprises comptent comme réussies, mais leurs octets ne sont pas comptés une seconde fois.

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

- `--move-resume` ne rejoue pas les opérations sans preuve SHA-256 complète; un ancien journal valide au format v1 reste lisible, mais ses succès sans hash ne sont pas repris.
- La vérification relit intégralement chaque destination candidate, ce qui ajoute des E/S proportionnelles aux fichiers repris.
- Une opération interrompue avant son état terminal n'a pas d'entrée finale dans le journal. La dernière opération en cours peut donc nécessiter une inspection manuelle après un arrêt brutal.
- Chaque entrée est écrite avec `os.File.Write`, sans `Sync` systématique. Cela évite une synchronisation disque coûteuse à chaque fichier, mais ne garantit pas la persistance sur support en cas de panne système ou de perte d'alimentation.
- Si l'écriture échoue, les opérations continuent mais les entrées suivantes ne sont plus tentées; le CLI signale `JournalError` à la fin.
- L'ordre des lignes reflète les terminaisons concurrentes et peut varier d'une exécution à l'autre.

## Alternatives considérées

- **Écrire uniquement après `ExecuteContext` :** rejetée pour le chemin CLI, car un arrêt avant la sérialisation finale perdrait les résultats déjà obtenus.
- **Synchroniser physiquement chaque ligne :** rejetée par défaut, car le coût d'une synchronisation par fichier pénaliserait fortement les gros plans. Une option de durabilité renforcée pourra être évaluée séparément.
- **Marquer l'opération fichier en échec quand le journal échoue :** rejetée, car une erreur d'audit ne change pas le résultat réel d'une copie ou d'un déplacement.
- **Rejouer automatiquement le journal sans vérification :** rejetée, car une destination peut avoir disparu ou changé depuis l'exécution précédente.

## Vérification

Les tests vérifient l'ajout de plusieurs entrées, la lecture d'un journal valide, le rejet d'une ligne mal formée ou d'un enregistrement invalide, l'écriture concurrente de succès, d'échec et d'opération ignorée, le signalement distinct d'une erreur d'écriture, et la reprise uniquement après correspondance exacte et vérification du contenu SHA-256 de destination. Les succès legacy sans hash et les destinations absentes, non régulières ou modifiées ne sont pas repris.

## Références d'implémentation

- `mover/journal.go` : format JSONL, writer append concurrent et lecteur strict.
- `mover/executor.go` : ajout des entrées aux transitions terminales et collecte de `JournalError`.
- `mover/mover.go` : résultat structuré d'exécution.
- `main.go` et `utils/flags.go` : options `--move-journal` et `--move-resume`, validation et cycle de vie du writer.
- `mover/resume.go` : rapprochement exact des opérations et vérification SHA-256 des destinations.
- `mover/mover_test.go` : tests du journal append et de l'intégration aux workers.
- `docs/IMPROVEMENTS.md` : feuille de route mise à jour.