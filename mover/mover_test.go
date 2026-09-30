package mover

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"FileRecoveryOrganizer/types"
)

func TestPlanAddAndPendingOperations(t *testing.T) {
	p := NewPlan(DefaultOptions())
	p.AddOperation(Operation{Source: "a", Destination: "b", Size: 12})
	if p.TotalFiles != 1 || p.TotalSize != 12 || len(p.GetPendingOperations()) != 1 {
		t.Fatalf("plan = %#v", p)
	}
}

func TestCopyFileAndVerifyHash(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "nested", "destination.txt")
	data := []byte("copy me")
	if err := os.WriteFile(src, data, 0644); err != nil {
		t.Fatal(err)
	}

	executor := NewExecutor(&Plan{Options: DefaultOptions()})
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	if err := executor.copyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	if !executor.verifyHash(dst, hex.EncodeToString(hash[:])) {
		t.Fatal("verifyHash rejected a copied file")
	}
}

func TestGeneratePlanAndExecuteCopy(t *testing.T) {
	// TempDir crée un dossier isolé pour ce test, que Go supprime automatiquement à la fin.
	dir := t.TempDir()
	sourceRoot := filepath.Join(dir, "input")
	if err := os.Mkdir(sourceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceRoot, "source.txt")
	// []byte représente les octets exacts du fichier, que la copie doit préserver.
	data := []byte("copy me through the mover")
	if err := os.WriteFile(source, data, 0644); err != nil {
		t.Fatal(err)
	}

	// On calcule le hash attendu à partir des octets source; l'exécuteur le comparera après la copie.
	hash := sha256.Sum256(data)
	result := types.Result{
		Path:       source,
		Size:       int64(len(data)),
		Type:       "txt",
		TargetPath: "documents/source.txt",
		FullHash:   hex.EncodeToString(hash[:]),
	}
	// json.Marshal sérialise la struct Go en JSON; le format JSONL place un objet JSON sur chaque ligne.
	jsonl, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(dir, "scan.jsonl")
	// Le saut de ligne termine l'enregistrement pour que le scanner le lise comme une ligne JSONL.
	if err := os.WriteFile(jsonlPath, append(jsonl, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	// On part des valeurs par défaut et on ne modifie que les options utiles à ce test.
	options := DefaultOptions()
	options.Source = sourceRoot
	options.Destination = filepath.Join(dir, "organized")
	options.Verify = true
	// Un seul worker rend ce petit test déterministe tout en passant par le mécanisme de workers.
	options.Workers = 1
	// La génération lit le JSONL et transforme TargetPath, relatif, en opération avec un chemin concret.
	plan, err := GeneratePlanFromJSONL(jsonlPath, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Operations) != 1 {
		t.Fatalf("planned %d operations, want 1", len(plan.Operations))
	}

	// Execute renvoie des compteurs et des erreurs structurés; le test peut vérifier le résultat directement.
	execution := NewExecutor(plan).Execute()
	if execution.Succeeded != 1 || execution.Failed != 0 {
		t.Fatalf("execution result = %d succeeded, %d failed; errors: %v", execution.Succeeded, execution.Failed, execution.Errors)
	}

	destination := filepath.Join(options.Destination, "documents", "source.txt")
	// ReadFile renvoie les octets copiés; on les compare au contenu initial pour vérifier la copie.
	copied, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(copied) != string(data) {
		t.Fatalf("destination contents = %q, want %q", copied, data)
	}
	// Stat vérifie que le mode copie a créé la destination sans supprimer le fichier source.
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source should remain after copy: %v", err)
	}
}

func TestGeneratePlanRejectsDestinationPathEscape(t *testing.T) {
	dir := t.TempDir()
	sourceRoot := filepath.Join(dir, "input")
	if err := os.Mkdir(sourceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceRoot, "source.txt")
	if err := os.WriteFile(source, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		targetPath string
	}{
		{name: "parent traversal", targetPath: filepath.Join("..", "outside.txt")},
		{name: "absolute path", targetPath: filepath.Join(dir, "outside.txt")},
	}

	// Chaque sous-test fournit un chemin malveillant différent au même point d'entrée.
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := types.Result{
				Path:       source,
				Size:       int64(len("source")),
				Type:       "txt",
				TargetPath: test.targetPath,
			}
			jsonl, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}

			jsonlPath := filepath.Join(dir, test.name+".jsonl")
			if err := os.WriteFile(jsonlPath, append(jsonl, '\n'), 0644); err != nil {
				t.Fatal(err)
			}

			options := DefaultOptions()
			options.Source = sourceRoot
			options.Destination = filepath.Join(dir, "organized")
			if _, err := GeneratePlanFromJSONL(jsonlPath, options); err == nil {
				t.Fatalf("TargetPath %q was accepted; want a path validation error", test.targetPath)
			}
		})
	}
}

func TestGeneratePlanRejectsDestinationInsideSource(t *testing.T) {
	dir := t.TempDir()
	sourceRoot := filepath.Join(dir, "input")
	if err := os.Mkdir(sourceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(jsonlPath, nil, 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		destination string
	}{
		{name: "same directory", destination: sourceRoot},
		{name: "nested directory", destination: filepath.Join(sourceRoot, "organized")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := DefaultOptions()
			options.Source = sourceRoot
			options.Destination = test.destination
			if _, err := GeneratePlanFromJSONL(jsonlPath, options); err == nil {
				t.Fatalf("destination %q was accepted; want a path validation error", test.destination)
			}
		})
	}
}

func TestGeneratePlanRejectsSourceOutsideSourceRoot(t *testing.T) {
	dir := t.TempDir()
	sourceRoot := filepath.Join(dir, "input")
	if err := os.Mkdir(sourceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	sourceOutside := filepath.Join(dir, "outside.txt")
	if err := os.WriteFile(sourceOutside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}

	result := types.Result{Path: sourceOutside, Type: "txt", TargetPath: "documents/outside.txt"}
	jsonl, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(dir, "scan.jsonl")
	if err := os.WriteFile(jsonlPath, append(jsonl, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	options := DefaultOptions()
	options.Source = sourceRoot
	options.Destination = filepath.Join(dir, "organized")
	if _, err := GeneratePlanFromJSONL(jsonlPath, options); err == nil {
		t.Fatal("source outside the selected root was accepted")
	}
}

func TestGeneratePlanRejectsDestinationSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	sourceRoot := filepath.Join(dir, "input")
	destinationRoot := filepath.Join(dir, "organized")
	outside := filepath.Join(dir, "outside")
	for _, path := range []string{sourceRoot, destinationRoot, outside} {
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(sourceRoot, "source.txt")
	if err := os.WriteFile(source, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}

	// Sur Windows, créer un symlink peut demander un droit système; dans ce cas ce test est ignoré.
	link := filepath.Join(destinationRoot, "redirect")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("impossible de créer le symlink de test: %v", err)
	}

	result := types.Result{Path: source, Type: "txt", TargetPath: filepath.Join("redirect", "copied.txt")}
	jsonl, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(dir, "scan.jsonl")
	if err := os.WriteFile(jsonlPath, append(jsonl, '\n'), 0644); err != nil {
		t.Fatal(err)
	}

	options := DefaultOptions()
	options.Source = sourceRoot
	options.Destination = destinationRoot
	if _, err := GeneratePlanFromJSONL(jsonlPath, options); err == nil {
		t.Fatal("destination path escaping through a symlink was accepted")
	}
}

func TestMoveFileRemovesSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "destination.txt")
	if err := os.WriteFile(src, []byte("move me"), 0644); err != nil {
		t.Fatal(err)
	}

	executor := NewExecutor(&Plan{Options: DefaultOptions()})
	if err := executor.moveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source still exists or returned another error: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("destination missing: %v", err)
	}
}
