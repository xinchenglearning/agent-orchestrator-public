package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xinchenglearning/agent-orchestrator/internal/store"
)

func TestRunLockRejectsConcurrentMutation(t *testing.T) {
	runDir := t.TempDir()
	first, err := store.AcquireLock(runDir, store.LockOwner{
		PID:        os.Getpid(),
		Command:    "run",
		Generation: 1,
		AcquiredAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()

	if _, err := store.AcquireLock(runDir, store.LockOwner{
		PID:        os.Getpid(),
		Command:    "cancel",
		Generation: 1,
		AcquiredAt: time.Now(),
	}); !errors.Is(err, store.ErrLocked) {
		t.Fatalf("expected lock contention, got %v", err)
	}
}

func TestRunLockIsNeverReclaimedAutomatically(t *testing.T) {
	runDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(runDir, "lock"), 0o700); err != nil {
		t.Fatal(err)
	}

	if _, err := store.AcquireLock(runDir, store.LockOwner{
		PID:        999999,
		Command:    "run",
		Generation: 1,
		AcquiredAt: time.Now().Add(-time.Hour),
	}); !errors.Is(err, store.ErrLocked) {
		t.Fatalf("expected stale lock to remain, got %v", err)
	}
}

func TestRunLockCanBeReleasedByItsHolder(t *testing.T) {
	runDir := t.TempDir()
	lock, err := store.AcquireLock(runDir, store.LockOwner{
		PID:        os.Getpid(),
		Command:    "run",
		Generation: 1,
		AcquiredAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "lock")); !os.IsNotExist(err) {
		t.Fatalf("lock directory still exists: %v", err)
	}
}
