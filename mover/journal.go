package mover

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
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

		entries = append(entries, OperationJournalEntry{
			SchemaVersion: 1,
			Timestamp:     timestamp,
			Action:        result.Plan.Options.Mode,
			Source:        operation.Source,
			Destination:   operation.Destination,
			Size:          operation.Size,
			Status:        operation.Status,
			Error:         operation.Error,
		})
	}
	if len(entries) == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open mover journal %q: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close mover journal %q: %w", path, err))
		}
	}()

	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return fmt.Errorf("append mover journal %q: %w", path, err)
		}
	}
	return nil
}
