# ADR 0004 : Précontrôle de l'espace disque du mover

- **Statut :** Acceptée
- **Date :** 2026-10-01

## Contexte

Le mover copie le contenu de chaque fichier vers sa destination, y compris en mode `move` avant de supprimer la source. Quand l'espace libre est insuffisant, une copie peut donc échouer après le démarrage des workers et après que d'autres opérations du plan ont déjà modifié le disque.

`CheckDiskSpace` existait comme emplacement réservé. Les plans peuvent cibler une destination qui n'a pas encore été créée, et le contrôle doit rester testable sans dépendre de la capacité réelle du poste.

## Décision

Implémenter `CheckDiskSpace` avec une mesure propre à chaque famille de systèmes :

- Windows interroge l'espace disponible pour l'appelant avec `GetDiskFreeSpaceEx`.
- Linux, macOS et les BSD pris en charge utilisent `statfs` et calculent l'espace disponible à partir des blocs disponibles et de leur taille.
- Les autres systèmes retournent une erreur explicite plutôt que de prétendre connaître la capacité.

La vérification additionne les tailles des opérations encore en attente. Les tailles négatives et les dépassements d'addition sont des erreurs. Pour trouver le volume cible lorsque la destination n'existe pas encore, elle remonte jusqu'au premier répertoire existant. Si la racine du plan n'est pas fournie, elle utilise le répertoire de la première destination en attente.

Les modes copie et déplacement sont précontrôlés. Les modes hardlink et symlink ne recopient pas le contenu complet et ne requièrent donc pas cette capacité; le contrôle retourne alors « suffisant » sans interroger le système.

`Executor.ExecuteContext` exécute le précontrôle avant de lancer les workers. Si le contrôle échoue ou si l'espace mesuré ne suffit pas, les opérations en attente sont marquées en échec et l'exécuteur retourne sans écrire de destination. Une annulation déjà présente est traitée avant l'appel système. Une fonction de mesure injectable permet aux tests de simuler capacité et erreurs de façon déterministe.

## Conséquences

### Positives

- Un plan manifestement trop volumineux est refusé avant le début des copies.
- Une erreur de mesure est visible dans les résultats du mover et ne déclenche pas les opérations.
- Les tests couvrent insuffisance, erreur système et modes de lien sans dépendre du disque réel.
- Les fichiers temporaires et la logique transactionnelle existants restent la protection contre les erreurs survenant pendant la copie.

### Coûts et limites

- Le précontrôle est une estimation : un autre processus peut consommer l'espace après la mesure, et d'autres erreurs d'I/O restent possibles.
- L'estimation repose sur les tailles enregistrées dans le plan; elle ne réserve pas physiquement l'espace.
- Le contrôle interroge un volume de destination pour un plan. Il suppose que les destinations du plan partagent ce volume; une destination traversant plusieurs points de montage nécessiterait un regroupement par volume.
- Sur un système non pris en charge, les copies et déplacements sont refusés si le contrôle ne peut pas être effectué.

## Alternatives considérées

- **Ne vérifier qu'après chaque copie :** rejetée, car le manque d'espace serait détecté après le lancement des opérations et pourrait affecter une partie du plan.
- **Traiter l'erreur de mesure comme un avertissement et continuer :** rejetée pour l'instant; le mover choisit un comportement prudent et n'effectue pas une opération destructive sans le précontrôle disponible.
- **Tester uniquement avec l'espace libre du poste :** rejetée, car cela rendrait les scénarios d'insuffisance et d'erreur non déterministes.

## Vérification

Les tests vérifient le retour d'espace disponible, l'insuffisance, la propagation d'erreurs, l'absence de mesure pour hardlink/symlink, et le refus d'exécuter une copie si le précontrôle échoue. `go test ./...`, `go vet ./...` et les builds Windows, Linux et macOS ont été exécutés avec succès.

## Références d'implémentation

- `mover/diskspace.go` : calcul du besoin, choix du chemin de volume et validation.
- `mover/diskspace_windows.go`, `mover/diskspace_unix.go` et `mover/diskspace_unsupported.go` : adaptateurs par système.
- `mover/executor.go` : précontrôle avant le démarrage des workers.
- `mover/mover_test.go` : tests déterministes de capacité et d'échec.
- `docs/IMPROVEMENTS.md` : feuille de route mise à jour.