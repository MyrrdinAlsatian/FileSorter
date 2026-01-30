// Package scanner - Tests unitaires pour le scanner et les statistiques.
//
// Ces tests vérifient notamment la THREAD-SAFETY du code,
// c'est-à-dire que le code fonctionne correctement quand plusieurs
// goroutines accèdent aux mêmes données simultanément.
package scanner

import (
	"sync"
	"testing"
)

// TestSafeStats_AddFile teste que AddFile fonctionne correctement
// avec des accès CONCURRENTS (plusieurs goroutines en même temps).
//
// CONCEPT GO : TESTER LA CONCURRENCE
// ==================================
// Pour tester du code concurrent, on lance plusieurs goroutines
// qui accèdent aux mêmes données, puis on vérifie le résultat.
//
// Si le code n'est PAS thread-safe, le test échouera (parfois).
// C'est ce qu'on appelle une "race condition" : le résultat dépend
// de l'ordre d'exécution des goroutines.
//
// OUTIL DE DÉTECTION : go test -race ./...
// Le flag -race active le "race detector" de Go qui détecte les
// accès concurrents non protégés.
func TestSafeStats_AddFile(t *testing.T) {
	stats := NewStats()

	// Configuration du test
	numGoroutines := 100     // Nombre de goroutines lancées
	filesPerGoroutine := 100 // Fichiers ajoutés par goroutine

	// sync.WaitGroup permet d'attendre que toutes les goroutines terminent
	// C'est comme un compteur :
	// - Add(n) : ajoute n au compteur
	// - Done() : décrémente de 1
	// - Wait() : bloque jusqu'à ce que le compteur soit à 0
	var wg sync.WaitGroup

	// Lancer plusieurs goroutines qui ajoutent des fichiers simultanément
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1) // Incrémenter AVANT de lancer la goroutine

		go func() {
			defer wg.Done() // Décrémenter à la fin de la goroutine

			for j := 0; j < filesPerGoroutine; j++ {
				// Toutes les goroutines appellent AddFile en même temps
				// Sans mutex, cela causerait des "race conditions"
				stats.AddFile("jpg", 1024, false)
			}
		}()
	}

	// Attendre que toutes les goroutines aient terminé
	wg.Wait()

	// Calculer les valeurs attendues
	expectedFiles := int64(numGoroutines * filesPerGoroutine) // 100 * 100 = 10000
	expectedSize := expectedFiles * 1024                      // 10000 * 1024 bytes

	// Vérifier que les compteurs sont corrects
	// Si le code n'était pas thread-safe, ces valeurs seraient incorrectes
	if stats.TotalFiles != expectedFiles {
		t.Errorf("TotalFiles = %d, want %d", stats.TotalFiles, expectedFiles)
	}

	if stats.TotalSize != expectedSize {
		t.Errorf("TotalSize = %d, want %d", stats.TotalSize, expectedSize)
	}

	if stats.FilesByType["jpg"] != expectedFiles {
		t.Errorf("FilesByType[\"jpg\"] = %d, want %d", stats.FilesByType["jpg"], expectedFiles)
	}
}

// TestSafeStats_AddFileWithError vérifie que le compteur d'erreurs fonctionne.
func TestSafeStats_AddFileWithError(t *testing.T) {
	stats := NewStats()

	stats.AddFile("jpg", 1024, false) // Sans erreur
	stats.AddFile("png", 2048, true)  // Avec erreur

	if stats.TotalFiles != 2 {
		t.Errorf("TotalFiles = %d, want 2", stats.TotalFiles)
	}

	if stats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats.Errors)
	}
}

// TestNewStats vérifie que NewStats initialise correctement les maps.
//
// CONCEPT GO : MAPS NIL VS MAPS VIDES
// ===================================
// Une map non initialisée (nil) et une map vide se comportent différemment :
//
//	var m map[string]int  // m est nil
//	m["key"] = 1          // PANIC ! "assignment to entry in nil map"
//
//	m = make(map[string]int)  // m est maintenant une map vide (non nil)
//	m["key"] = 1              // OK !
//
// C'est pourquoi NewStats() doit utiliser make() pour initialiser les maps.
func TestNewStats(t *testing.T) {
	stats := NewStats()

	// Vérifier que les maps sont initialisées (non nil)
	if stats.FilesByType == nil {
		t.Error("FilesByType should be initialized")
	}

	if stats.BytesByType == nil {
		t.Error("BytesByType should be initialized")
	}

	// Vérifier que les compteurs sont à zéro
	if stats.TotalFiles != 0 {
		t.Errorf("TotalFiles should be 0, got %d", stats.TotalFiles)
	}
}

// TestCollector vérifie que NewCollector crée un canal avec la bonne capacité.
//
// CONCEPT GO : CAPACITÉ D'UN CANAL
// ================================
// Un canal peut être :
// - Non bufferisé : make(chan T) - capacité 0
// - Bufferisé : make(chan T, n) - capacité n
//
// La fonction cap() retourne la capacité d'un canal (ou slice).
// La fonction len() retourne le nombre d'éléments actuellement dans le canal.
func TestCollector(t *testing.T) {
	collector := NewCollector(10)

	// Vérifier que le canal est créé
	if collector.Results == nil {
		t.Error("Results channel should be initialized")
	}

	// Vérifier que le canal a la bonne capacité
	// cap() retourne la capacité d'un canal
	if cap(collector.Results) != 10 {
		t.Errorf("Results channel capacity = %d, want 10", cap(collector.Results))
	}
}
