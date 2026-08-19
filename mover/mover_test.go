package mover

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
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
