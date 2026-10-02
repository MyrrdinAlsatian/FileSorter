# Comprendre les changements de performance

Ce guide explique les optimisations apportées au projet, le problème auquel chacune répond et les notions de Go qu'elles illustrent. L'objectif est de relier le code aux contraintes concrètes du projet : analyser environ 672 947 fichiers représentant près de 300 Go sur un disque dur.

## Le contexte matériel compte

Avec autant de fichiers, la durée du traitement ne dépend pas seulement du volume total. Le disque dur doit aussi parcourir les répertoires, lire les métadonnées et ouvrir de nombreux fichiers. Pour de petits fichiers, ces opérations peuvent coûter plus cher que la lecture de leur contenu. Le chiffre moyen (environ 446 Ko par fichier) ne décrit toutefois pas la répartition réelle : certains fichiers peuvent être très petits et d'autres très volumineux.

Les améliorations visent donc à réduire les parcours, les ouvertures, les lectures inutiles et les données conservées en mémoire. Elles ne garantissent pas un gain chiffré sans mesure sur la même machine, le même disque et le même jeu de données. Pour un traitement dominé par la vitesse du HDD, changer de langage ne rendrait pas le disque plus rapide.

## Quelques notions utiles

- **Complexité algorithmique** : elle décrit comment le travail augmente avec le nombre d'éléments. Un parcours en `O(n)` examine chaque élément une fois. Une recherche imbriquée peut approcher `O(n²)` dans un groupe défavorable.
- **Traitement en flux** : on consomme un élément, on le traite, puis on le relâche. Cela évite de garder le jeu de données complet en mémoire.
- **Allocation** : créer un tableau ou un objet réserve de la mémoire. Réduire les allocations répétées peut réduire le travail du ramasse-miettes de Go.
- **Goroutine et concurrence** : le scanner peut faire avancer plusieurs tâches en parallèle. Les structures partagées doivent alors être protégées, ici notamment par des mutex.
- **Cache** : une valeur calculée est réutilisée. Il faut aussi vérifier qu'elle décrit toujours le fichier courant avant de s'en servir.

## 1. Le rapport HTML est construit au fil du scan

Dans `main.go`, chaque résultat est transmis directement au constructeur de rapport au lieu d'être ajouté à une grande liste `allResults`. Le constructeur, dans [`report/html.go`](../report/html.go), garde les compteurs nécessaires et au plus 1 000 exemples à afficher.

**Pourquoi :** avec plusieurs centaines de milliers de fichiers, conserver chaque résultat jusqu'à la fin peut occuper beaucoup de mémoire. Le rapport a surtout besoin des totaux par catégorie et d'un échantillon pour son affichage.

**Concept à retenir :** c'est un traitement en flux avec une mémoire bornée. La mémoire du rapport dépend surtout du nombre de catégories et de types, plus la limite d'exemples, et non du nombre total de fichiers.

**Compromis :** le rapport ne peut pas afficher tous les résultats individuels quand il y en a plus de 1 000. L'ancienne fonction `GenerateHTML` reste disponible et s'appuie sur le constructeur, ce qui conserve son interface pour les autres appels.

## 2. Le précomptage est facultatif

Le paramètre `--precount` est ajouté dans [`utils/flags.go`](../utils/flags.go). Par défaut, la barre de progression utilise un indicateur sans pourcentage exact. Avec `--precount`, le programme parcourt d'abord les fichiers pour compter le total, puis affiche une progression précise.

**Pourquoi :** obtenir un dénominateur exact demande de parcourir l'arborescence avant le scan réel. Sur un HDD et plusieurs centaines de milliers de fichiers, cette seconde visite peut coûter du temps.

**Concept à retenir :** une interface plus précise peut avoir un coût de calcul. Ici, on choisit entre démarrer rapidement avec une progression indéterminée et effectuer un passage supplémentaire pour obtenir un pourcentage.

Exemples :

```powershell
# Démarre sans passage préalable de comptage
go run . --source "D:\A_analyser"

# Effectue aussi un comptage préalable pour afficher le pourcentage
go run . --source "D:\A_analyser" --precount
```

## 3. Le scan ne calcule pas les hash sans raison

Dans `main.go`, le calcul demandé au scanner est maintenant déclenché par `--hash` ou par la reprise d'un déplacement (`--move-resume`). La détection des doublons utilise une autre voie : elle commence par réunir les fichiers de même taille, puis calcule les hash nécessaires à ces candidats.

**Pourquoi :** calculer un hash pour chaque fichier peut demander des lectures supplémentaires. Or les fichiers de tailles différentes ne peuvent pas être identiques octet pour octet ; leur taille permet donc d'écarter rapidement beaucoup de candidats.

**Concept à retenir :** un filtre peu coûteux précède une opération plus coûteuse. Le pipeline des doublons utilise la taille, puis un hash rapide, puis le hash complet pour confirmer les groupes candidats.

**Comportement à connaître :** pour obtenir à la fois le rapport de doublons et le champ `quick_hash` dans les résultats exportés, utilisez `--hash --duplicates`. `--duplicates` seul cherche les doublons, mais ne demande plus au scanner d'ajouter ce champ à chaque résultat.

## 4. Les informations déjà lues sont réutilisées

Le scanner transmet à `types.Result` l'heure de modification observée pendant son `os.Stat`. Cette valeur interne, `ScanModTime`, porte le tag `json:"-"` : elle n'est pas ajoutée aux exports JSON. Elle permet notamment au détecteur de doublons de savoir à quel état du fichier correspond le hash rapide fourni par le scan.

Pour les images, [`enricher/image.go`](../enricher/image.go) renvoie maintenant les informations extraites du même décodage EXIF. Le scanner utilise ces données et ne relit plus les métadonnées du système de fichiers quand il les a déjà. [`metadata/date.go`](../metadata/date.go) fournit cette variante à partir d'un `os.FileInfo` existant.

**Pourquoi :** une nouvelle lecture de métadonnées ou un second décodage d'image refait un travail déjà accompli et peut provoquer des accès supplémentaires au disque.

**Concept à retenir :** faire remonter une valeur calculée à l'appelant évite de refaire le calcul. En Go, une fonction peut retourner une structure métier en plus de son résultat principal ; ici l'enrichisseur retourne les dates qu'il a extraites.

## 5. Le regroupement des doublons évite une recherche imbriquée

Dans [`dedup/hash.go`](../dedup/hash.go), `bySize` conserve désormais des `hashJob` contenant le chemin, la taille, le hash rapide et l'heure observée. Les étapes suivantes gardent ces informations ensemble au lieu de rechercher à nouveau la taille en parcourant les fichiers.

Le déroulement est :

1. Regrouper les fichiers par taille.
2. Ignorer les groupes contenant un seul fichier.
3. Réutiliser ou calculer le hash rapide pour les candidats.
4. Calculer le hash complet seulement pour les groupes qui restent ambigus.
5. Ne présenter comme doublons que les fichiers dont le hash complet correspond.

**Pourquoi :** l'ancienne organisation devait retrouver certaines informations en reparcourant les chemins. Garder un petit objet avec les données utiles simplifie le pipeline et évite cette recherche répétée.

**Concept à retenir :** une `map` est adaptée à un regroupement par clé, ici la taille. Le type `hashJob` regroupe les valeurs qui doivent rester cohérentes pendant les différentes étapes.

## 6. Un hash rapide réutilisé reste vérifié

Le hash rapide peut être réutilisé lorsqu'il est fourni par le scan. Avant cela, le code vérifie que la taille et l'heure de modification du fichier correspondent toujours à celles observées au scan. Si elles ont changé, le hash rapide est recalculé. Le hash complet reste l'étape qui confirme les doublons.

**Pourquoi :** un fichier peut être modifié entre son scan et son analyse comme doublon. Réutiliser aveuglément un hash ancien risquerait de comparer une valeur qui ne représente plus le contenu actuel.

**Concept à retenir :** un cache doit avoir une règle de validité. La taille et la date de modification sont un contrôle pratique, mais elles ne constituent pas une preuve cryptographique de stabilité ; c'est pourquoi le hash complet est conservé pour la confirmation.

## 7. Les buffers sont réutilisés avec `sync.Pool`

Le calcul des hash utilise des buffers réutilisables via `sync.Pool` dans [`dedup/hash.go`](../dedup/hash.go). La détection de motifs dans [`detector/detect.go`](../detector/detect.go) utilise elle aussi un buffer partagé de taille maximale bornée.

**Pourquoi :** allouer un grand buffer pour chaque fichier candidat multiplie les allocations. Réutiliser des buffers peut réduire ce coût et la pression sur le ramasse-miettes.

**Concept à retenir :** `sync.Pool` est un réservoir temporaire d'objets réutilisables, pas un cache permanent. Go peut vider ce réservoir lors d'un ramassage mémoire ; le programme doit donc rester correct si `Get` fournit un nouvel objet. Le contenu d'un buffer doit être remis dans un état sûr avant de le réutiliser.

**Compromis :** la taille du buffer influence les appels de lecture, les allocations et l'usage mémoire. Il ne faut pas la modifier au hasard : mesurez d'abord sur des fichiers et un stockage représentatifs.

## 8. La détection de contenu ne charge plus un gros fichier entier

Le fallback de détection dans [`detector/detect.go`](../detector/detect.go) ne fait plus `os.ReadFile` pour charger l'intégralité d'un fichier inconnu. Il lit un préfixe borné à 64 Kio. Pour les fichiers JSON, un `json.Decoder` vérifie la syntaxe en flux, sans conserver tout le document en mémoire.

**Pourquoi :** un seul gros fichier inconnu pouvait provoquer une allocation proportionnelle à sa taille. Une lecture plafonnée maintient un coût mémoire prévisible.

**Concept à retenir :** les décodeurs Go peuvent lire progressivement depuis un `io.Reader`. Un flux n'implique pas nécessairement de stocker le document complet.

**Limite :** les autres motifs du fallback ne voient que le préfixe lu. Un motif situé très loin dans un fichier texte sans extension peut donc ne plus être trouvé. Cette limite est le compromis de la borne mémoire.

## 9. Les métadonnées ne sont demandées que pour les types concernés

Dans [`metadata/utils.go`](../metadata/utils.go), les types de fichiers qui ne sont ni image, ni audio, ni vidéo, ni DivX reçoivent immédiatement des métadonnées vides. Le lecteur de tags n'essaie donc pas d'ouvrir tous les autres fichiers.

**Pourquoi :** interroger un lecteur de métadonnées pour un fichier qui ne peut pas fournir ces informations entraîne des vérifications et parfois des accès disque inutiles.

**Concept à retenir :** filtrer tôt avec une condition simple permet d'éviter le travail en aval. Cela améliore également la lisibilité : les types pris en charge sont explicites.

## 10. Le compteur de dossiers est mis à jour pendant le parcours

`SafeStats` ajoute `TotalDirs`, incrémenté par le producteur du scanner lorsqu'il rencontre un dossier. Le résumé récupère ensuite cette statistique au lieu de devoir refaire un parcours pour recompter les dossiers.

**Pourquoi :** le scanner connaît déjà les dossiers visités. Réutiliser ce comptage évite une autre source de vérité et un travail séparé.

**Concept à retenir :** plusieurs goroutines peuvent mettre à jour des statistiques partagées. Le mutex de `SafeStats` protège ces modifications et évite les courses entre accès simultanés.

## 11. Le mode debug affiche le chemin en cours

Avec l'option `--debug`, chaque worker signale le chemin du fichier qu'il commence à traiter. La barre de progression affiche ce chemin et l'actualise au plus une fois par seconde. Sans cette option, le scanner ne publie pas de chemin et la barre garde son affichage normal. `--verbose` reste indépendant.

Comme plusieurs workers travaillent en parallèle, il s'agit du dernier chemin signalé, qui peut différer du fichier le plus lent.

**Pourquoi :** lorsqu'on observe une longue pause dans le compteur, le chemin affiché donne un indice concret sur le fichier traité récemment.

**Concept à retenir :** `atomic.Value` permet aux workers de publier le dernier chemin sans verrou explicite. Une goroutine avec un ticker lit cette valeur à intervalle régulier pour rafraîchir l'affichage. Cette goroutine n'existe qu'en mode debug, et limiter les rafraîchissements évite d'écrire dans le terminal pour chacun des centaines de milliers de fichiers.

## Valeurs de référence avant les modifications

Le relevé fourni correspond à un scan en **mode simulation** du dossier `E:\Archive`. Il sert de point de comparaison avant les optimisations :

| Mesure observée | Valeur |
|---|---:|
| Fichiers traités | 672 947 |
| Dossiers parcourus | 1 495 |
| Durée affichée | 1 h 3 min 54 s |
| Débit affiché | environ 175 fichiers/s |
| Entrées exportées | 672 947 |
| Taille indiquée pendant le scan | 297,19 Go |
| Taille indiquée dans le résumé final | 297,30 Go |

Le journal affiche deux tailles différentes, avec un écart de 0,11 Go. Il faut garder cette différence à l'esprit lorsqu'on comparera les résultats ; le relevé seul ne permet pas d'en déterminer la cause. Il ne contient pas de mesure de mémoire ou de processeur, et ne précise pas toutes les options passées au programme. La durée et le débit sont donc des références observées, pas un benchmark contrôlé.

## Premier relevé après les modifications

Le nouveau journal concerne lui aussi `E:\Archive`, en mode simulation, avec 4 workers. La validation de tous les types et le mode verbeux étaient activés. Les noms de fichiers dans la progression n'étaient pas encore activés pour cette exécution.

| Mesure | Avant | Après | Écart observé |
|---|---:|---:|---:|
| Fichiers traités | 672 947 | 672 947 | identique |
| Dossiers parcourus | 1 495 | 1 495 | identique |
| Durée affichée par la barre | 1 h 3 min 54 s | 55 min 13 s | 8 min 41 s de moins, environ 13,6 % |
| Débit affiché | environ 175 fichiers/s | environ 203 fichiers/s | environ 16 % de plus |
| Taille du résumé final | 297,30 Go | 297,45 Go | +0,15 Go |
| Entrées exportées | 672 947 | 672 947 | identique |

Ces durées sont celles affichées par la barre de progression. Le relevé initial affichait un total exact, ce qui indique un précomptage ; le nouveau affiche `672947/-`, sans total préalable. Le temps du précomptage initial n'est pas inclus dans la durée visible de sa barre.

Cette comparaison est encourageante, mais ne prouve pas à elle seule que les modifications expliquent le gain. Le nouveau run faisait un travail supplémentaire avec la validation de tous les types. Le mode verbeux peut aussi ajouter des écritures dans le terminal. Le relevé initial ne précise ni ces options ni le nombre de workers. La taille totale diffère aussi de 0,15 Go malgré le même nombre de fichiers, et les types détectés ne sont pas tout à fait les mêmes. Il faut donc traiter le gain comme une observation, pas comme une mesure contrôlée.

Pour confirmer l'effet, relance les deux versions avec le même dossier et les mêmes options, idéalement plusieurs fois. Compare séparément le temps total réel, précomptage compris, et le débit du scan. Note aussi si le cache du système a déjà servi les fichiers.

## Mesurer les effets sur votre machine

Pour comparer deux versions, gardez le même dossier source, les mêmes options, le même disque et des conditions proches. Notez au minimum la durée totale et la mémoire maximale. Un HDD peut donner des temps différents selon que les données sont déjà dans le cache du système ou non. Faites plusieurs passages et comparez les résultats plutôt qu'un seul chronométrage.

Les changements réduisent des opérations ou la mémoire. Les deux journaux disponibles montrent un écart de temps et de débit, mais leurs options ne sont pas assez documentées pour attribuer sûrement cet écart aux seules modifications ; aucun benchmark contrôlé avant/après n'a encore été réalisé.

## Pour approfondir en lisant le code

1. Suivez le résultat produit par le scanner jusqu'à `OnResult` dans `main.go`.
2. Dans `dedup/hash.go`, notez les données transportées dans `hashJob` et les étapes de filtrage.
3. Repérez où un mutex est pris puis relâché dans `scanner/stats.go` et demandez-vous ce qui se passerait sans lui.
4. Comparez la mémoire nécessaire à une liste complète de résultats avec celle du constructeur HTML qui ne garde qu'un échantillon limité.
5. Modifiez mentalement la limite de 64 Kio du détecteur : quels motifs risquent alors d'être manqués, et quel serait le coût d'une limite plus grande ?

Pour apprendre par la mesure, une prochaine étape utile serait de créer des benchmarks Go ciblés pour le hashing, la détection et l'agrégation HTML, puis de comparer temps et allocations avec `go test -bench . -benchmem`. Ces benchmarks ne sont pas inclus dans les changements décrits ici.
