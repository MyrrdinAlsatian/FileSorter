package mover

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// ResumeReport résume le traitement des opérations déjà présentes dans le journal.
type ResumeReport struct {
	Resumed    int
	Unverified int
}

type journalOperationKey struct {
	action      Mode
	source      string
	destination string
	size        int64
}

// ResumeOperationsFromJournal marque comme réussies les opérations dont la destination est vérifiée.
func ResumeOperationsFromJournal(ctx context.Context, plan *Plan, entries []OperationJournalEntry) (ResumeReport, error) {
	var report ResumeReport
	if plan == nil {
		return report, fmt.Errorf("mover plan is nil")
	}
	if plan.Options.Mode != ModeCopy && plan.Options.Mode != ModeMove {
		return report, fmt.Errorf("journal resume supports only copy and move modes")
	}

	completed := make(map[journalOperationKey][]OperationJournalEntry)
	for index, entry := range entries {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if err := validateOperationJournalEntry(entry); err != nil {
			return report, fmt.Errorf("invalid mover journal entry %d: %w", index+1, err)
		}
		if entry.Action != plan.Options.Mode || entry.Status != StatusSuccess {
			continue
		}
		key := journalOperationKey{
			action:      entry.Action,
			source:      entry.Source,
			destination: entry.Destination,
			size:        entry.Size,
		}
		completed[key] = append(completed[key], entry)
	}

	verified := make(map[int]OperationJournalEntry)
	for index := range plan.Operations {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		operation := &plan.Operations[index]
		if operation.Status != StatusPending {
			continue
		}
		key := journalOperationKey{
			action:      plan.Options.Mode,
			source:      operation.Source,
			destination: operation.Destination,
			size:        operation.Size,
		}
		candidates := completed[key]
		if len(candidates) == 0 {
			continue
		}

		matched := false
		for _, entry := range candidates {
			if !isFullSHA256(operation.Hash) || !isFullSHA256(entry.SHA256) || !strings.EqualFold(operation.Hash, entry.SHA256) {
				continue
			}
			destinationMatches, err := journalDestinationMatches(ctx, operation.Destination, entry.SHA256, operation.Size)
			if err != nil {
				return report, fmt.Errorf("verify resumed destination %q: %w", operation.Destination, err)
			}
			if destinationMatches {
				verified[index] = entry
				matched = true
				break
			}
		}
		if !matched {
			report.Unverified++
		}
	}

	for index, entry := range verified {
		operation := &plan.Operations[index]
		operation.Status = StatusSuccess
		operation.Resumed = true
		operation.EndTime = entry.Timestamp
	}
	report.Resumed = len(verified)
	return report, nil
}

func journalDestinationMatches(ctx context.Context, path, expectedHash string, expectedSize int64) (bool, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !pathInfo.Mode().IsRegular() || pathInfo.Size() != expectedSize {
		return false, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) || openedInfo.Size() != expectedSize {
		return false, nil
	}

	hash := sha256.New()
	buffer := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(buffer)
	if _, err := io.CopyBuffer(hash, contextReader{ctx: ctx, reader: file}, *buffer); err != nil {
		return false, err
	}
	finalInfo, err := file.Stat()
	if err != nil {
		return false, err
	}
	if !os.SameFile(openedInfo, finalInfo) || finalInfo.Size() != expectedSize || !finalInfo.ModTime().Equal(openedInfo.ModTime()) {
		return false, nil
	}
	finalPathInfo, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if !finalPathInfo.Mode().IsRegular() || !os.SameFile(finalInfo, finalPathInfo) {
		return false, nil
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedHash), nil
}
