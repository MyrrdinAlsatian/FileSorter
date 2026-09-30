# ADR 0001 : Annulation coopérative avec `context.Context`

- **Statut :** Acceptée
- **Date :** 2026-10-01

## Contexte

Le scan, le calcul des hash, l'analyse des doublons et les copies peuvent durer longtemps. Avant cette décision, ces opérations ne disposaient pas d'un mécanisme commun pour recevoir une demande d'arrêt. Un Ctrl+C pouvait interrompre le processus sans laisser aux workers et à l'exporteur le temps de terminer proprement.

Le projet expose déjà des fonctions utilisées par plusieurs packages et tests. La mise en place de l'annulation ne doit donc pas imposer une migration immédiate à tous les appelants.

## Décision

Utiliser `context.Context` pour propager une demande d'annulation de manière coopérative à travers les opérations longues.

Le CLI crée un contexte avec `signal.NotifyContext` pour traiter Ctrl+C, puis le transmet au précomptage, au scan, à l'analyse des doublons, à la génération du plan et à l'exécution du mover.

Les fonctions qui existaient avant cette évolution restent disponibles. Elles délèguent aux variantes contextuelles avec `context.Background()`, ce qui préserve leur contrat et la compatibilité des appelants existants. Les nouvelles variantes suivent la convention Go : le contexte est leur premier paramètre.

Les boucles et workers consultent le contexte aux points où ils peuvent s'arrêter proprement : parcours et distribution des fichiers, traitement des jobs, envoi des résultats, lectures de hash et copie par blocs. Les producteurs ferment leurs canaux et attendent les workers avant de retourner. Le CLI attend aussi la fin de l'exporteur et ferme l'export JSONL, y compris lorsque le scan est annulé.

Pour le mover, l'annulation pendant une copie retire le fichier temporaire et ne publie pas de destination partielle. Un move ne supprime la source qu'après la publication réussie de la copie et une dernière vérification du contexte. Si l'annulation arrive juste après cette publication, la destination complète et la source peuvent toutes deux subsister; la source n'est pas supprimée par précaution.

## Conséquences

### Positives

- Ctrl+C arrête progressivement les opérations longues sans interrompre brutalement la coordination interne.
- Les workers bloqués sur la distribution ou l'envoi des résultats peuvent réagir à l'annulation.
- Les copies interrompues avant publication ne laissent pas de destination partielle, et la source est préservée.
- Les API existantes restent utilisables sans changement.
- Les tests peuvent déclencher l'annulation de façon déterministe sans dépendre d'un signal système réel.

### Coûts et limites

- L'annulation est coopérative : chaque opération doit consulter le contexte.
- Une lecture ou un appel système déjà en cours n'est pas nécessairement interrompu immédiatement; l'annulation est observée au prochain point de contrôle.
- Le mover représente les opérations interrompues comme des erreurs dans `ExecutionResult`; son API ne retourne pas directement une erreur globale d'annulation.
- Après publication d'une copie complète, une annulation peut laisser les deux fichiers présents. C'est intentionnel afin de ne pas risquer la perte de la source.
- Le détecteur de courses `go test -race` n'a pas pu être exécuté dans l'environnement de validation, faute de cgo et de compilateur C.

## Alternatives considérées

- **Interrompre immédiatement le processus sur Ctrl+C :** rejetée, car les workers et l'exporteur n'auraient pas l'occasion de terminer leur nettoyage.
- **Ajouter un mécanisme d'arrêt propre à chaque package :** rejetée, car cela dupliquerait la signalisation et compliquerait la coordination entre producteurs, workers et consommateurs.
- **Remplacer directement les signatures existantes :** rejetée pour cette étape, car cela imposerait de modifier tous les appelants. Les wrappers permettent une adoption progressive.

## Vérification

Les tests couvrent l'annulation du hash et de la génération de doublons, le précomptage, l'annulation en plein scan avec fermeture du canal de résultats, ainsi que la préservation de la source pour un mover annulé. La suite `go test ./...`, `go vet ./...` et `go build ./...` passe.

## Références d'implémentation

- `dedup/hash.go` : variantes contextuelles des fonctions de hash et du détecteur de doublons.
- `scanner/scanner.go` : précomptage et scan parallèles annulables.
- `mover/plan.go` et `mover/executor.go` : génération de plan, copie et exécution annulables.
- `main.go` : contexte associé à Ctrl+C et orchestration du pipeline.