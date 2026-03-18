package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const lockFileName = ".letterhead.lock"

var (
	ErrLockHeld = errors.New("sync already running")
)

// Lock represents a held file lock.
type Lock struct {
	path string
}

// AcquireLock attempts to acquire an exclusive lock for the given archive root.
// If a stale lock exists (process dead), it is cleaned up and re-acquired.
func AcquireLock(archiveRoot string) (*Lock, error) {
	lockPath := filepath.Join(archiveRoot, lockFileName)

	// Check for existing lock
	data, err := os.ReadFile(lockPath)
	if err == nil {
		// Lock file exists — check if the holder is still alive
		pid, parseErr := parseLockPID(string(data))
		if parseErr == nil && processAlive(pid) {
			return nil, fmt.Errorf("%w (PID %d)", ErrLockHeld, pid)
		}
		// Stale lock — remove it
		_ = os.Remove(lockPath)
	}

	// Create the lock file
	content := fmt.Sprintf("pid=%d\ntime=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if err := os.WriteFile(lockPath, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	return &Lock{path: lockPath}, nil
}

// Release removes the lock file.
func (l *Lock) Release() error {
	if l == nil {
		return nil
	}
	return os.Remove(l.path)
}

func parseLockPID(content string) (int, error) {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "pid=") {
			return strconv.Atoi(strings.TrimPrefix(line, "pid="))
		}
	}
	return 0, errors.New("no pid in lock file")
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Use kill(0) to check.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
