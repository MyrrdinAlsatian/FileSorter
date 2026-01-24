package scanner

import "sync"

// SafeStats gère les statistiques de façon thread-safe pour les goroutines parallèles.
//
// CONCEPT : Thread-safety avec Mutex
// ============================================
// Quand plusieurs goroutines accèdent aux mêmes données, des race conditions
// peuvent survenir. Un Mutex (mutual exclusion lock) assure qu'une seule
// goroutine modifie les données à la fois.
//
// EXEMPLE de race condition (MAUVAIS) :
//
//	FilesByType["jpg"]++  // 2 goroutines font ça simultanément
//	// Résultat : +1 au lieu de +2 (une écriture perdue)
//
// AVEC Mutex (BON) :
//
//	mu.Lock()
//	FilesByType["jpg"]++  // Une seule goroutine à la fois
//	mu.Unlock()
//
// defer mu.Unlock() : Même en cas de panique, le mutex est libéré
//
// PERFORMANCE :
// - Les mutex ralentissent un peu (contention)
// - Mais c'est nécessaire pour l'exactitude
// - Le buffer du canal reduit les appels AddFile, réduisant la contention
type SafeStats struct {
	mu sync.Mutex // Mutex pour protéger les accès

	// Statistiques par type de fichier
	FilesByType map[string]int64 // Nombre de fichiers par type (ex: map["jpg"] = 1500)
	BytesByType map[string]int64 // Taille en octets par type

	// Statistiques globales
	TotalFiles int64 // Nombre total de fichiers traités
	TotalSize  int64 // Taille totale en octets

	Errors int64 // Nombre de fichiers avec erreur
}

// NewStats crée une nouvelle instance de SafeStats avec les maps initialisées.
//
// POURQUOI une fonction New() ?
// En Go, c'est une convention de nommer les fonctions constructeurs "New" suivi du type.
// Cela permet d'initialiser correctement les maps (qui ne peuvent pas être nil).
//
// IMPORTANTE : Les maps doivent être initialisées avec make(), sinon
// tenter d'y écrire causera une panique "assignment to entry in nil map".
//
// Retour :
//   - *SafeStats : pointeur vers une nouvelle instance initialisée
func NewStats() *SafeStats {
	return &SafeStats{
		FilesByType: make(map[string]int64),
		BytesByType: make(map[string]int64),
	}
}

// AddFile ajoute les statistiques d'un fichier traité de façon thread-safe.
//
// Cette fonction est appelée par chaque goroutine worker quand elle traite un fichier.
// Elle incrémente les compteurs de façon synchronisée.
//
// PROCESSUS :
// 1. Lock() : prendre le verrou (attendre si occupé)
// 2. Mettre à jour les données
// 3. defer Unlock() : libérer le verrou (même en cas d'erreur)
//
// Paramètres :
//   - fileType : type du fichier (ex: "jpg")
//   - size : taille du fichier en octets
//   - hasError : true si une erreur s'est produite lors du traitement
//
// Note : Cette fonction est appelée très fréquemment, donc elle doit être rapide.
// Le buffer du canal (100 éléments) aide à réduire l'appel à cette fonction.
func (s *SafeStats) AddFile(fileType string, size int64, hasError bool) {
	s.mu.Lock()         // Verrouiller avant de modifier les données
	defer s.mu.Unlock() // Déverrouiller après (même si panique)

	// Incrémenter le comptage des fichiers de ce type
	s.FilesByType[fileType]++
	// Ajouter la taille à l'accumulation de ce type
	s.BytesByType[fileType] += size

	// Statistiques globales
	s.TotalFiles++
	s.TotalSize += size

	if hasError {
		s.Errors++
	}
}
