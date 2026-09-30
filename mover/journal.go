package mover

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// OperationJournalEntry décrit une opération terminée dans le journal JSONL.
type OperationJournalEntry struct {
	SchemaVersion int             `json:"schema_version"`
	Timestamp     time.Time       `json:"timestamp"`
	Action        Mode            `json:"action"`
	Source        string          `json:"source"`
	Destination   string          `json:"destination"`
	Size          int64           `json:"size"`
	Status        OperationStatus `json:"status"`
	Error         string          `json:"error,omitempty"`
}

// OperationJournal écrit les résultats au fil de l'exécution.
type OperationJournal struct {
	path   string
	file   *os.File
	mu     sync.Mutex
	closed bool
}

// NewOperationJournal ouvre un journal en ajout sans effacer son contenu existant.
func NewOperationJournal(path string) (*OperationJournal, error) {
	if path == "" {
		return nil, fmt.Errorf("mover journal path is empty")
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open mover journal %q: %w", path, err)
	}
	return &OperationJournal{path: path, file: file}, nil
}

// Append ajoute une ligne JSON complète au journal.
func (j *OperationJournal) Append(entry OperationJournalEntry) error {
	if j == nil {
		return fmt.Errorf("mover journal is nil")
	}
	if entry.SchemaVersion == 0 {
		entry.SchemaVersion = 1
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode mover journal entry: %w", err)
	}
	line = append(line, '\n')

	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return fmt.Errorf("mover journal %q is closed", j.path)
	}
	written, err := j.file.Write(line)
	if err != nil {
		return fmt.Errorf("append mover journal %q: %w", j.path, err)
	}
	if written != len(line) {
		return fmt.Errorf("append mover journal %q: %w", j.path, io.ErrShortWrite)
	}
	return nil
}

// Close ferme le journal après que les workers ont terminé.
func (j *OperationJournal) Close() error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed = true
	if err := j.file.Close(); err != nil {
		return fmt.Errorf("close mover journal %q: %w", j.path, err)
	}
	return nil
}

func newOperationJournalEntry(action Mode, operation Operation, fallbackTime time.Time) OperationJournalEntry {
	timestamp := operation.EndTime
	if timestamp.IsZero() {
		timestamp = fallbackTime
	}
	return OperationJournalEntry{
		SchemaVersion: 1,
		Timestamp:     timestamp,
		Action:        action,
		Source:        operation.Source,
		Destination:   operation.Destination,
		Size:          operation.Size,
		Status:        operation.Status,
		Error:         operation.Error,
	}
}

// AppendOperationJournal ajoute les opérations terminées d'un résultat au journal JSONL.
func AppendOperationJournal(path string, result *ExecutionResult) (returnErr error) {
	if path == "" {
		return fmt.Errorf("mover journal path is empty")
	}
	if result == nil || result.Plan == nil {
		return fmt.Errorf("mover execution result has no plan")
	}

	entries := make([]OperationJournalEntry, 0, len(result.Plan.Operations))
	for _, operation := range result.Plan.Operations {
		switch operation.Status {
		case StatusSuccess, StatusFailed, StatusSkipped, StatusConflict:
		default:
			continue
		}

		timestamp := operation.EndTime
		if timestamp.IsZero() {
			timestamp = result.EndTime
		}
		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		entries = append(entries, newOperationJournalEntry(result.Plan.Options.Mode, operation, timestamp))
	}
	if len(entries) == 0 {
		return nil
	}

	journal, err := NewOperationJournal(path)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, journal.Close())
	}()

	for _, entry := range entries {
		if err := journal.Append(entry); err != nil {
			return err
		}
	}
	return nil
}
