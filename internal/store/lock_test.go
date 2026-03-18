package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireAndReleaseLock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}

	lockPath := filepath.Join(dir, lockFileName)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file does not exist: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock file still exists after release")
	}
}

func TestAcquireLockConflict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	lock1, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("first AcquireLock() error = %v", err)
	}
	defer lock1.Release()

	_, err = AcquireLock(dir)
	if err == nil {
		t.Fatalf("second AcquireLock() should fail")
	}
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("error = %v, want ErrLockHeld", err)
	}
}

func TestAcquireLockCleansUpStaleLock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	lockPath := filepath.Join(dir, lockFileName)

	// Write a lock file with a PID that doesn't exist
	// PID 2147483647 is unlikely to be in use
	content := fmt.Sprintf("pid=%d\ntime=2026-01-01T00:00:00Z\n", 2147483647)
	if err := os.WriteFile(lockPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	lock, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock() with stale lock error = %v", err)
	}
	defer lock.Release()
}

func TestReleasNilLock(t *testing.T) {
	t.Parallel()

	var lock *Lock
	if err := lock.Release(); err != nil {
		t.Fatalf("nil Release() error = %v", err)
	}
}
