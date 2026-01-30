// Package classifier - Tests unitaires pour la classification des fichiers.
//
// CONCEPT GO : LES TESTS UNITAIRES
// =================================
// En Go, les tests sont dans des fichiers suffixés par "_test.go".
// Ils utilisent le package "testing" de la bibliothèque standard.
//
// CONVENTIONS :
// - Fichier : xxx_test.go (dans le même package que le code testé)
// - Fonction : func TestXxx(t *testing.T) où Xxx commence par une majuscule
// - Lancement : go test ./... (tous les tests) ou go test -v (verbose)
//
// STRUCTURE D'UN TEST :
//
//	func TestAddition(t *testing.T) {
//	    result := Add(2, 3)
//	    if result != 5 {
//	        t.Errorf("Add(2, 3) = %d, want 5", result)
//	    }
//	}
//
// MÉTHODES DE *testing.T :
// - t.Error(args...) : signale une erreur mais continue
// - t.Errorf(format, args...) : comme Error avec formatage
// - t.Fatal(args...) : signale une erreur et ARRÊTE le test
// - t.Fatalf(format, args...) : comme Fatal avec formatage
// - t.Skip(args...) : saute le test (pour les tests désactivés)
// - t.Run(name, func) : lance un sous-test (pour les tests tabulaires)
package classifier

import (
	"testing"

	"FileRecoveryOrganizer/types"
)

// TestGetCategory teste la fonction GetCategory avec plusieurs cas.
//
// CONCEPT GO : TESTS TABULAIRES (TABLE-DRIVEN TESTS)
// ==================================================
// C'est un pattern très courant en Go pour tester plusieurs cas.
// On définit une slice de cas de test, puis on itère dessus.
//
// AVANTAGES :
// - Facile d'ajouter de nouveaux cas
// - Code DRY (Don't Repeat Yourself)
// - Lisible et maintenable
// - Chaque cas a un nom clair
//
// STRUCTURE :
//
//	tests := []struct {
//	    name     string   // Nom du cas (pour l'affichage)
//	    input    T        // Entrée(s)
//	    expected U        // Sortie attendue
//	}{
//	    {"cas 1", input1, output1},
//	    {"cas 2", input2, output2},
//	}
//
//	for _, tt := range tests {
//	    t.Run(tt.name, func(t *testing.T) {
//	        // Tester
//	    })
//	}
func TestGetCategory(t *testing.T) {
	// Définir les cas de test
	// struct anonyme : on définit la structure directement sans lui donner de nom
	tests := []struct {
		name     string // Nom du cas de test (affiché si échec)
		fileType string // Entrée : l'extension à classifier
		expected string // Sortie attendue : le chemin de destination
	}{
		// Vidéos
		{"MP4 video", "mp4", "videos/mp4"},
		{"MKV video", "mkv", "videos/mkv"},

		// Audio
		{"MP3 audio", "mp3", "audio/mp3"},
		{"FLAC audio", "flac", "audio/flac"},

		// Images
		{"JPEG image", "jpg", "images/jpg"},
		{"PNG image", "png", "images/png"},

		// Documents
		{"PDF document", "pdf", "documents/pdf"},
		{"Word document", "docx", "documents/text/docx"},

		// Archives
		{"ZIP archive", "zip", "archives/zip"},
		{"RAR archive", "rar", "archives/rar"},

		// Code
		{"Go source", "go", "code/go/go"},
		{"Python source", "py", "code/python/py"},

		// Type inconnu - doit retourner "others/<type>"
		{"Unknown type", "xyz123", "others/xyz123"},
	}

	// Itérer sur tous les cas de test
	// _ ignore l'index, tt est le cas de test courant
	for _, tt := range tests {
		// t.Run() crée un sous-test avec son propre nom
		// Cela permet de voir quel cas a échoué précisément
		t.Run(tt.name, func(t *testing.T) {
			result := GetCategory(tt.fileType)
			if result != tt.expected {
				// %q = chaîne entre guillemets (utile pour voir les espaces)
				t.Errorf("GetCategory(%q) = %q, want %q", tt.fileType, result, tt.expected)
			}
		})
	}
}

// TestClassify teste la fonction Classify qui assigne un TargetPath à un Result.
//
// NOTE : On teste ici les différents chemins de code :
// 1. Fichier simple (pas d'image) → classification par type
// 2. Image thumbnail → images/thumbnails
// 3. Image asset → images/assets
// 4. Image originale → images/originals
// 5. Type inconnu → others/<type>
func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		result   types.Result // La structure Result à classifier
		expected string       // Le TargetPath attendu après classification
	}{
		{
			name:     "Simple MP4 file",
			result:   types.Result{Type: "mp4"},
			expected: "videos/mp4",
		},
		{
			name:     "Simple JPG file",
			result:   types.Result{Type: "jpg"},
			expected: "images/jpg",
		},
		{
			name: "Image thumbnail",
			// On crée un Result avec un pointeur vers ImageMeta
			// Le & crée un pointeur vers la struct littérale
			result: types.Result{
				Type:  "jpg",
				Image: &types.ImageMeta{IsThumb: true},
			},
			expected: "images/thumbnails",
		},
		{
			name: "Image asset",
			result: types.Result{
				Type:  "png",
				Image: &types.ImageMeta{IsAsset: true},
			},
			expected: "images/assets",
		},
		{
			name: "Original image with EXIF",
			result: types.Result{
				Type:  "jpg",
				Image: &types.ImageMeta{HasExif: true},
			},
			expected: "images/originals",
		},
		{
			name:     "Unknown file type",
			result:   types.Result{Type: "unknownext"},
			expected: "others/unknownext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// IMPORTANT : Créer une copie locale pour éviter les effets de bord
			// car Classify() modifie la structure passée en paramètre
			r := tt.result
			Classify(&r) // Passer un pointeur car Classify modifie r
			if r.TargetPath != tt.expected {
				t.Errorf("Classify() TargetPath = %q, want %q", r.TargetPath, tt.expected)
			}
		})
	}
}

// TestRegisterCategory vérifie qu'on peut ajouter une nouvelle catégorie dynamiquement.
func TestRegisterCategory(t *testing.T) {
	// Enregistrer une nouvelle catégorie
	RegisterCategory("custom", "custom/path")

	// Vérifier qu'elle est bien enregistrée
	result := GetCategory("custom")
	if result != "custom/path" {
		t.Errorf("GetCategory(\"custom\") = %q, want \"custom/path\"", result)
	}
}

// TestGetAllCategories vérifie que GetAllCategories retourne une COPIE, pas une référence.
func TestGetAllCategories(t *testing.T) {
	categories := GetAllCategories()

	// Modifier la copie
	categories["test_key"] = "test_value"

	// Vérifier que l'original n'est PAS modifié
	// Si c'était une référence, GetCategory retournerait "test_value"
	if GetCategory("test_key") != "others/test_key" {
		t.Error("GetAllCategories() should return a copy, not a reference")
	}
}
