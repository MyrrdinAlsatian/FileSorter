package mover

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type diskSpaceLookup func(path string) (uint64, error)

// CheckDiskSpace compare la taille des opérations en attente à l'espace disponible.
// Les hardlinks et symlinks ne recopient pas le contenu des fichiers.
func CheckDiskSpace(plan *Plan) (bool, int64, error) {
	return checkDiskSpace(plan, availableDiskSpace)
}

// checkDiskSpace reçoit la fonction système en paramètre pour tester le calcul sans dépendre du disque réel.
func checkDiskSpace(plan *Plan, lookup diskSpaceLookup) (bool, int64, error) {
	if plan == nil {
		return false, 0, fmt.Errorf("cannot check disk space for a nil plan")
	}

	switch plan.Options.Mode {
	case ModeHardlink, ModeSymlink:
		return true, 0, nil
	case ModeCopy, ModeMove:
	default:
		return false, 0, fmt.Errorf("unsupported mover mode %q", plan.Options.Mode)
	}

	required, err := pendingBytes(plan)
	if err != nil {
		return false, 0, err
	}
	if required == 0 {
		return true, 0, nil
	}

	destination, err := planDestination(plan)
	if err != nil {
		return false, 0, err
	}
	volumePath, err := nearestExistingDirectory(destination)
	if err != nil {
		return false, 0, fmt.Errorf("resolve destination volume: %w", err)
	}

	free, err := lookup(volumePath)
	if err != nil {
		return false, 0, fmt.Errorf("read free space for %q: %w", volumePath, err)
	}
	// The public result uses int64; saturation preserves the enough-space answer for very large volumes.
	maxInt64 := uint64(1<<63 - 1)
	if free > maxInt64 {
		return true, int64(maxInt64), nil
	}
	available := int64(free)
	return available >= required, available, nil
}

func pendingBytes(plan *Plan) (int64, error) {
	const maxInt64 = int64(1<<63 - 1)
	var required int64

	plan.mu.RLock()
	defer plan.mu.RUnlock()
	for _, operation := range plan.Operations {
		if operation.Status != StatusPending {
			continue
		}
		if operation.Size < 0 {
			return 0, fmt.Errorf("operation %q has a negative size", operation.Source)
		}
		if operation.Size > maxInt64-required {
			return 0, fmt.Errorf("total size of pending operations overflows int64")
		}
		required += operation.Size
	}
	return required, nil
}

func planDestination(plan *Plan) (string, error) {
	if destination := strings.TrimSpace(plan.Options.Destination); destination != "" {
		return destination, nil
	}

	plan.mu.RLock()
	defer plan.mu.RUnlock()
	for _, operation := range plan.Operations {
		if operation.Status == StatusPending && operation.Destination != "" {
			return filepath.Dir(operation.Destination), nil
		}
	}
	return "", fmt.Errorf("destination directory is required to check disk space")
}

// nearestExistingDirectory monte vers les parents, car la racine de destination peut ne pas encore exister.
func nearestExistingDirectory(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("destination path is empty")
	}
	current, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	for {
		info, statErr := os.Stat(current)
		if statErr == nil {
			if info.IsDir() {
				return current, nil
			}
			current = filepath.Dir(current)
			continue
		}
		if !os.IsNotExist(statErr) {
			return "", statErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", fmt.Errorf("no existing parent directory for %q", path)
		}
		current = parent
	}
}
