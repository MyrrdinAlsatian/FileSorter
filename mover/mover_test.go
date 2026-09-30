package mover

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"FileRecoveryOrganizer/types"
)

func TestPlanAddAndPendingOperations(t *testing.T) {
	p := NewPlan(DefaultOptions())
	p.AddOperation(Operation{Source: "a", Destination: "b", Size: 12})
	if p.TotalFiles != 1 || p.TotalSize != 12 || len(p.GetPendingOperations()) != 1 {
		t.Fatalf("plan = %#v", p)
	}
}

func TestCheckDiskSpaceReturnsAvailableCapacity(t *testing.T) {
	plan := NewPlan(Options{
		Mode:        ModeCopy,
		Destination: t.TempDir(),
	})
	plan.AddOperation(Operation{Source: "source", Destination: "destination", Size: 1})

	_, available, err := CheckDiskSpace(plan)
	if err != nil {
		t.Fatalf("CheckDiskSpace error = %v", err)
	}
	if available < 0 {
		t.Fatalf("available bytes = %d, want a nonnegative value", available)
	}
}

func TestCheckDiskSpaceReportsInsufficientCapacity(t *testing.T) {
	root := t.TempDir()
	plan := NewPlan(Options{
		Mode:        ModeCopy,
		Destination: filepath.Join(root, "not-created-yet"),
	})
	plan.AddOperation(Operation{Source: "source", Destination: "destination", Size: 101})

	enough, available, err := checkDiskSpace(plan, func(path string) (uint64, error) {
		if path != root {
			t.Fatalf("volume path = %q, want nearest existing directory %q", path, root)
		}
		return 100, nil
	})
	if err != nil {
		t.Fatalf("checkDiskSpace error = %v", err)
	}
	if enough || available != 100 {
		t.Fatalf("checkDiskSpace = (%t, %d), want (false, 100)", enough, available)
	}
}

func TestCheckDiskSpacePropagatesLookupErrors(t *testing.T) {
	plan := NewPlan(Options{Mode: ModeCopy, Destination: t.TempDir()})
	plan.AddOperation(Operation{Source: "source", Size: 1})
	wantErr := errors.New("volume unavailable")

	_, _, err := checkDiskSpace(plan, func(string) (uint64, error) {
		return 0, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("checkDiskSpace error = %v, want wrapped %v", err, wantErr)
	}
}

func TestCheckDiskSpaceSkipsLinkModes(t *testing.T) {
	for _, mode := range []Mode{ModeHardlink, ModeSymlink} {
		t.Run(string(mode), func(t *testing.T) {
			plan := NewPlan(Options{Mode: mode})
			plan.AddOperation(Operation{Source: "source", Size: 4096})
			enough, available, err := checkDiskSpace(plan, func(string) (uint64, error) {
				t.Fatal("link operation should not query free space")
				return 0, nil
			})
			if err != nil || !enough || available != 0 {
				t.Fatalf("checkDiskSpace = (%t, %d, %v), want (true, 0, nil)", enough, available, err)
			}
		})
	}
}

func TestExecuteRefusesCopyWhenDiskSpaceIsInsufficient(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "destination", "source.txt")
	if err := os.WriteFile(source, []byte("keep the source"), 0644); err != nil {
		t.Fatal(err)
	}
	plan := NewPlan(Options{Mode: ModeCopy, Destination: filepath.Dir(destination), Workers: 1})
	plan.AddOperation(Operation{Source: source, Destination: destination, Size: int64(len("keep the source"))})
	executor := NewExecutor(plan)
	executor.diskSpaceCheck = func(*Plan) (bool, int64, error) {
		return false, 0, nil
	}

	result := executor.Execute()
	if result.Failed != 1 {
		t.Fatalf("failed operations = %d, want 1", result.Failed)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination should not be created when space is insufficient: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source should remain after preflight refusal: %v", err)
	}
}

func TestExecuteRefusesCopyWhenDiskCheckFails(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "destination", "source.txt")
	if err := os.WriteFile(source, []byte("keep the source"), 0644); err != nil {
		t.Fatal(err)
	}
	plan := NewPlan(Options{Mode: ModeCopy, Destination: filepath.Dir(destination), Workers: 1})
	plan.AddOperation(Operation{Source: source, Destination: destination, Size: int64(len("keep the source"))})
	executor := NewExecutor(plan)
	executor.diskSpaceCheck = func(*Plan) (bool, int64, error) {
		return false, 0, errors.New("volume unavailable")
	}

	result := executor.Execute()
	if result.Failed != 1 {
		t.Fatalf("failed operations = %d, want 1", result.Failed)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination should not be created when the disk check fails: %v", err)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source should remain after preflight failure: %v", err)
	}
}

func TestAppendOperationJournalWritesJSONLAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.jsonl")
	timestamp := time.Date(2026, time.October, 1, 12, 30, 0, 0, time.UTC)
	results := []*ExecutionResult{
		{
			Plan: &Plan{
				Options: Options{Mode: ModeMove},
				Operations: []Operation{{
					Source:      "source-a.jpg",
					Destination: "destination-a.jpg",
					Size:        12,
					Status:      StatusSuccess,
					EndTime:     timestamp,
				}},
			},
		},
		{
			Plan: &Plan{
				Options: Options{Mode: ModeMove},
				Operations: []Operation{{
					Source:      "source-b.jpg",
					Destination: "destination-b.jpg",
					Size:        20,
					Status:      StatusFailed,
					Error:       "copy failed",
					EndTime:     timestamp.Add(time.Minute),
				}},
			},
		},
	}

	for _, result := range results {
		if err := AppendOperationJournal(path, result); err != nil {
			t.Fatalf("AppendOperationJournal error = %v", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read journal: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	if len(lines) != 2 {
		t.Fatalf("journal lines = %d, want 2: %s", len(lines), data)
	}

	var first, second struct {
		SchemaVersion int             `json:"schema_version"`
		Timestamp     time.Time       `json:"timestamp"`
		Action        Mode            `json:"action"`
		Source        string          `json:"source"`
		Destination   string          `json:"destination"`
		Size          int64           `json:"size"`
		Status        OperationStatus `json:"status"`
		Error         string          `json:"error"`
	}
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatalf("decode first journal line: %v", err)
	}
	if err := json.Unmarshal(lines[1], &second); err != nil {
		t.Fatalf("decode second journal line: %v", err)
	}
	if first.SchemaVersion != 1 || first.Timestamp != timestamp || first.Action != ModeMove || first.Source != "source-a.jpg" || first.Destination != "destination-a.jpg" || first.Size != 12 || first.Status != StatusSuccess {
		t.Fatalf("first journal entry = %#v", first)
	}
	if second.SchemaVersion != 1 || second.Source != "source-b.jpg" || second.Status != StatusFailed || second.Error != "copy failed" {
		t.Fatalf("second journal entry = %#v", second)
	}

	entries, err := ReadOperationJournal(path)
	if err != nil {
		t.Fatalf("ReadOperationJournal error = %v", err)
	}
	if len(entries) != 2 || entries[0].Source != "source-a.jpg" || entries[1].Status != StatusFailed {
		t.Fatalf("read journal entries = %#v", entries)
	}
}

func TestReadOperationJournalRejectsMalformedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.jsonl")
	valid, err := json.Marshal(OperationJournalEntry{
		SchemaVersion: 1,
		Timestamp:     time.Date(2026, time.October, 1, 12, 30, 0, 0, time.UTC),
		Action:        ModeCopy,
		Source:        "source.txt",
		Destination:   "destination.txt",
		Size:          1,
		Status:        StatusSuccess,
	})
	if err != nil {
		t.Fatal(err)
	}
	content := append(append(valid, '\n'), []byte("not-json\n")...)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadOperationJournal(path); err == nil {
		t.Fatal("ReadOperationJournal error = nil, want malformed-line error")
	} else if !bytes.Contains([]byte(err.Error()), []byte("line 2")) {
		t.Fatalf("ReadOperationJournal error = %v, want line number 2", err)
	}
}

func TestReadOperationJournalRejectsInvalidEntries(t *testing.T) {
	validTimestamp := time.Date(2026, time.October, 1, 12, 30, 0, 0, time.UTC)
	tests := []struct {
		name  string
		entry OperationJournalEntry
	}{
		{
			name: "unsupported schema",
			entry: OperationJournalEntry{
				SchemaVersion: 2,
				Timestamp:     validTimestamp,
				Action:        ModeCopy,
				Status:        StatusSuccess,
			},
		},
		{
			name: "missing timestamp",
			entry: OperationJournalEntry{
				SchemaVersion: 1,
				Action:        ModeCopy,
				Status:        StatusSuccess,
			},
		},
		{
			name: "unknown action",
			entry: OperationJournalEntry{
				SchemaVersion: 1,
				Timestamp:     validTimestamp,
				Action:        Mode("unknown"),
				Status:        StatusSuccess,
			},
		},
		{
			name: "negative size",
			entry: OperationJournalEntry{
				SchemaVersion: 1,
				Timestamp:     validTimestamp,
				Action:        ModeCopy,
				Size:          -1,
				Status:        StatusSuccess,
			},
		},
		{
			name: "non-terminal status",
			entry: OperationJournalEntry{
				SchemaVersion: 1,
				Timestamp:     validTimestamp,
				Action:        ModeCopy,
				Status:        StatusPending,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			line, err := json.Marshal(test.entry)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "operations.jsonl")
			if err := os.WriteFile(path, append(line, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadOperationJournal(path); err == nil {
				t.Fatal("ReadOperationJournal error = nil, want validation error")
			}
		})
	}
}

func TestExecutorWritesOperationJournal(t *testing.T) {
	root := t.TempDir()
	destinationRoot := filepath.Join(root, "destination")
	successSource := filepath.Join(root, "success.txt")
	skippedSource := filepath.Join(root, "skipped.txt")
	skippedDestination := filepath.Join(destinationRoot, "skipped.txt")
	if err := os.WriteFile(successSource, []byte("copy"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skippedSource, []byte("skip"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destinationRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skippedDestination, []byte("keep existing"), 0644); err != nil {
		t.Fatal(err)
	}

	plan := NewPlan(Options{
		Mode:          ModeCopy,
		Destination:   destinationRoot,
		Workers:       3,
		OverwriteMode: ConflictSkip,
	})
	plan.AddOperation(Operation{Source: successSource, Destination: filepath.Join(destinationRoot, "success.txt"), Size: 4})
	plan.AddOperation(Operation{Source: filepath.Join(root, "missing.txt"), Destination: filepath.Join(destinationRoot, "missing.txt"), Size: 1})
	plan.AddOperation(Operation{Source: skippedSource, Destination: skippedDestination, Size: 4})

	journal, err := NewOperationJournal(filepath.Join(root, "operations.jsonl"))
	if err != nil {
		t.Fatalf("NewOperationJournal error = %v", err)
	}
	executor := NewExecutor(plan)
	executor.SetOperationJournal(journal)
	executor.diskSpaceCheck = func(*Plan) (bool, int64, error) {
		return true, 1 << 30, nil
	}
	result := executor.Execute()
	if err := journal.Close(); err != nil {
		t.Fatalf("close operation journal: %v", err)
	}

	if result.Succeeded != 1 || result.Failed != 1 || result.Skipped != 1 {
		t.Fatalf("execution counts = (%d succeeded, %d failed, %d skipped)", result.Succeeded, result.Failed, result.Skipped)
	}
	if result.JournalError != nil {
		t.Fatalf("journal error = %v", result.JournalError)
	}

	data, err := os.ReadFile(filepath.Join(root, "operations.jsonl"))
	if err != nil {
		t.Fatalf("read operation journal: %v", err)
	}
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	if len(lines) != 3 {
		t.Fatalf("journal lines = %d, want 3: %s", len(lines), data)
	}
	statuses := make(map[string]OperationStatus, len(lines))
	for _, line := range lines {
		var entry OperationJournalEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("decode operation journal entry: %v", err)
		}
		statuses[entry.Source] = entry.Status
	}
	if statuses[successSource] != StatusSuccess {
		t.Errorf("success source status = %q, want %q", statuses[successSource], StatusSuccess)
	}
	if statuses[filepath.Join(root, "missing.txt")] != StatusFailed {
		t.Errorf("missing source status = %q, want %q", statuses[filepath.Join(root, "missing.txt")], StatusFailed)
	}
	if statuses[skippedSource] != StatusSkipped {
		t.Errorf("skipped source status = %q, want %q", statuses[skippedSource], StatusSkipped)
	}
}

func TestExecutorReportsJournalWriteErrorSeparately(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "destination", "source.txt")
	if err := os.WriteFile(source, []byte("preserve operation status"), 0644); err != nil {
		t.Fatal(err)
	}
	plan := NewPlan(Options{
		Mode:          ModeCopy,
		Destination:   filepath.Dir(destination),
		Workers:       1,
		OverwriteMode: ConflictSkip,
	})
	plan.AddOperation(Operation{Source: source, Destination: destination, Size: int64(len("preserve operation status"))})

	journal, err := NewOperationJournal(filepath.Join(root, "operations.jsonl"))
	if err != nil {
		t.Fatalf("NewOperationJournal error = %v", err)
	}
	if err := journal.Close(); err != nil {
		t.Fatalf("close operation journal: %v", err)
	}
	executor := NewExecutor(plan)
	executor.SetOperationJournal(journal)
	executor.diskSpaceCheck = func(*Plan) (bool, int64, error) {
		return true, 1 << 30, nil
	}
	result := executor.Execute()

	if result.Succeeded != 1 || result.Failed != 0 {
		t.Fatalf("file operation counts = (%d succeeded, %d failed), want (1, 0)", result.Succeeded, result.Failed)
	}
	if result.JournalError == nil {
		t.Fatal("JournalError = nil, want write failure")
	}
	if _, err := os.Stat(destination); err != nil {
		t.Fatalf("successful destination missing: %v", err)
	}
}

func TestParseMoverModes(t *testing.T) {
	// Les parseurs retournent des types distincts; ces adaptateurs permettent de partager une table de tests.
	tests := []struct {
		name    string
		value   string
		parse   func(string) (string, error)
		want    string
		wantErr bool
	}{
		{name: "copy mode", value: "copy", parse: func(value string) (string, error) { parsed, err := ParseMode(value); return string(parsed), err }, want: string(ModeCopy)},
		{name: "move mode", value: "move", parse: func(value string) (string, error) { parsed, err := ParseMode(value); return string(parsed), err }, want: string(ModeMove)},
		{name: "invalid operation mode", value: "unknown", parse: func(value string) (string, error) { parsed, err := ParseMode(value); return string(parsed), err }, wantErr: true},
		{name: "skip conflicts", value: "skip", parse: func(value string) (string, error) {
			parsed, err := ParseOverwriteMode(value)
			return string(parsed), err
		}, want: string(ConflictSkip)},
		{name: "overwrite conflicts", value: "overwrite", parse: func(value string) (string, error) {
			parsed, err := ParseOverwriteMode(value)
			return string(parsed), err
		}, want: string(ConflictOverwrite)},
		{name: "rename conflicts", value: "rename", parse: func(value string) (string, error) {
			parsed, err := ParseOverwriteMode(value)
			return string(parsed), err
		}, want: string(ConflictRename)},
		{name: "invalid conflict mode", value: "unknown", parse: func(value string) (string, error) {
			parsed, err := ParseOverwriteMode(value)
			return string(parsed), err
		}, wantErr: true},
	}

	for _, test := range tests {
		// t.Run nomme chaque cas et rend son échec identifiable séparément.
		t.Run(test.name, func(t *testing.T) {
			got, err := test.parse(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("parse error = %v, wantErr %t", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("parsed value = %q, want %q", got, test.want)
			}
		})
	}
}

func TestExecuteContextCancellationPreservesSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	destination := filepath.Join(root, "destination", "source.txt")
	if err := os.WriteFile(source, []byte("keep the original"), 0644); err != nil {
		t.Fatal(err)
	}
	options := DefaultOptions()
	options.Mode = ModeMove
	options.Workers = 1
	plan := NewPlan(options)
	plan.AddOperation(Operation{Source: source, Destination: destination, Size: int64(len("keep the original"))})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := NewExecutor(plan).ExecuteContext(ctx)
	if result.Failed != 1 {
		t.Fatalf("failed operations = %d, want 1", result.Failed)
	}
	if _, err := os.Stat(source); err != nil {
		t.Fatalf("source should remain after cancellation: %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination should not exist after cancellation, stat error = %v", err)
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
	fullHash := hex.EncodeToString(hash[:])
	if !executor.verifyHash(dst, fullHash) {
		t.Fatal("verifyHash rejected a copied file")
	}
	// Un quick hash n'est qu'un préfixe et ne suffit pas à prouver l'intégrité complète.
	if executor.verifyHash(dst, fullHash[:8]) {
		t.Fatal("verifyHash accepted a hash prefix")
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

func TestMoveVerificationFailurePreservesSource(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destination := filepath.Join(dir, "organized", "source.txt")
	data := []byte("keep the only verified original")
	if err := os.WriteFile(source, data, 0644); err != nil {
		t.Fatal(err)
	}

	options := DefaultOptions()
	options.Mode = ModeMove
	options.Verify = true
	options.Workers = 1
	plan := NewPlan(options)
	plan.AddOperation(Operation{
		Source:      source,
		Destination: destination,
		Size:        int64(len(data)),
		Hash:        "0000000000000000000000000000000000000000000000000000000000000000",
	})

	// Un hash volontairement faux simule une copie altérée ou un résultat JSONL incohérent.
	execution := NewExecutor(plan).Execute()
	if execution.Failed != 1 {
		t.Fatalf("failed operations = %d, want 1", execution.Failed)
	}

	remaining, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("la source doit rester disponible après l'échec: %v", err)
	}
	if string(remaining) != string(data) {
		t.Fatalf("contenu source = %q, want %q", remaining, data)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("la destination ne doit pas être publiée après l'échec, erreur Stat = %v", err)
	}
}

func TestCopyVerificationFailurePreservesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destinationDir := filepath.Join(dir, "organized")
	destination := filepath.Join(destinationDir, "source.txt")
	if err := os.Mkdir(destinationDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("new contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old contents"), 0644); err != nil {
		t.Fatal(err)
	}

	options := DefaultOptions()
	options.Verify = true
	options.Workers = 1
	options.OverwriteMode = ConflictOverwrite
	plan := NewPlan(options)
	plan.AddOperation(Operation{
		Source:      source,
		Destination: destination,
		Hash:        "0000000000000000000000000000000000000000000000000000000000000000",
	})

	// L'échec de validation ne doit remplacer ni l'ancien fichier ni laisser le temporaire.
	execution := NewExecutor(plan).Execute()
	if execution.Failed != 1 {
		t.Fatalf("failed operations = %d, want 1", execution.Failed)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("l'ancienne destination doit rester présente: %v", err)
	}
	if string(contents) != "old contents" {
		t.Fatalf("destination = %q, want %q", contents, "old contents")
	}
	entries, err := os.ReadDir(destinationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("nombre de fichiers de destination = %d, want 1 (temporaire nettoyé)", len(entries))
	}
}

func TestCopyOverwriteReplacesDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	destinationDir := filepath.Join(dir, "organized")
	destination := filepath.Join(destinationDir, "source.txt")
	data := []byte("verified new contents")
	if err := os.Mkdir(destinationDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old contents"), 0644); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)

	options := DefaultOptions()
	options.Verify = true
	options.Workers = 1
	options.OverwriteMode = ConflictOverwrite
	plan := NewPlan(options)
	plan.AddOperation(Operation{
		Source:      source,
		Destination: destination,
		Hash:        hex.EncodeToString(hash[:]),
	})

	execution := NewExecutor(plan).Execute()
	if execution.Succeeded != 1 {
		t.Fatalf("succeeded operations = %d, want 1; errors: %v", execution.Succeeded, execution.Errors)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(data) {
		t.Fatalf("destination = %q, want %q", contents, data)
	}
}

func TestGeneratePlanRejectsDestinationOverSourceFile(t *testing.T) {
	dir := t.TempDir()
	sourceRoot := filepath.Join(dir, "input")
	if err := os.Mkdir(sourceRoot, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(sourceRoot, "source.txt")
	if err := os.WriteFile(source, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}

	result := types.Result{Path: source, Type: "txt", TargetPath: filepath.Join("input", "source.txt")}
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
	options.Destination = dir
	if _, err := GeneratePlanFromJSONL(jsonlPath, options); err == nil {
		t.Fatal("destination resolving to the source file was accepted")
	}
}
