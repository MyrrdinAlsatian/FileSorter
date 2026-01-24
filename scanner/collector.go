// Package scanner gère le parcours et la collecte des résultats de traitement des fichiers.
package scanner

import (
	"FileRecoveryOrganizer/types"
)

// Collector accumule les résultats de traitement des fichiers.
//
// CONCEPT : Communication entre goroutines
// =========================================
// Un canal (channel) en Go est un "tuyau" qui permet la communication sécurisée
// entre goroutines. On peut envoyer (->)  ou recevoir (<-) des valeurs.
//
// CARACTÉRISTIQUES DES CANAUX :
// - Thread-safe : pas besoin de mutex
// - Bloquant : l'envoi attend un lecteur, la réception attend un émetteur
// - Typé : on spécifie le type des données transmises
//
// EXEMPLE :
//
//	results := make(chan Result, 100)
//	go func() { results <- myResult }()  // Envoyer un résultat
//	result := <- results                 // Recevoir un résultat
//
// LE BUFFER :
// - make(chan T) : canal non bufferisé (0 capacity)
// - make(chan T, 100) : canal bufferisé (100 éléments peuvent attendre)
// - Plus le buffer, moins les goroutines bloquent
//
// FERMETURE :
// - close(ch) : ferme le canal
// - La lecture sur un canal fermé retourne la valeur par défaut
// - Utile pour signaler "je n'enverrai plus de données"
type Collector struct {
	// Results est un canal qui reçoit les résultats de traitement
	// C'est comment les workers envoient leurs résultats au programme principal
	Results chan types.Result
}

// NewCollector crée un nouveau collecteur avec un canal bufferisé.
//
// PARAMÈTRES :
// Le buffer (100 dans ScanDirectoryParallel) est un équilibre :
// - Petit buffer : moins de mémoire, mais les workers bloquent plus souvent
// - Grand buffer : plus de mémoire, mais les workers restent libres
//
// EXEMPLE d'utilisation :
//
//	collector := NewCollector(100)
//	// Envoyer un résultat (depuis un worker)
//	collector.Results <- myResult
//	// Recevoir les résultats (depuis le main)
//	for result := range collector.Results {
//	    fmt.Println(result)
//	}
//
// Paramètres :
//   - buffer : taille du buffer du canal
//
// Retour :
//   - *Collector : pointeur vers un nouveau collecteur
func NewCollector(buffer int) *Collector {
	return &Collector{
		Results: make(chan types.Result, buffer),
	}
}
