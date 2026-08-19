package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var ErrLocked = errors.New("run is locked")

type LockOwner struct {
	PID        int       `json:"pid"`
	Command    string    `json:"command"`
	Generation uint64    `json:"generation"`
	AcquiredAt time.Time `json:"acquiredAt"`
}

type RunLock struct {
	path string
}

func AcquireLock(runDir string, owner LockOwner) (*RunLock, error) {
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return nil, fmt.Errorf("create run directory: %w", err)
	}
	lockPath := filepath.Join(runDir, "lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("create run lock: %w", err)
	}

	data, err := json.Marshal(owner)
	if err != nil {
		os.Remove(lockPath)
		return nil, fmt.Errorf("marshal lock owner: %w", err)
	}
	if err := os.WriteFile(
		filepath.Join(lockPath, "owner.json"),
		append(data, '\n'),
		0o600,
	); err != nil {
		os.RemoveAll(lockPath)
		return nil, fmt.Errorf("write lock owner: %w", err)
	}
	return &RunLock{path: lockPath}, nil
}

func (l *RunLock) Release() error {
	if l == nil || l.path == "" {
		return nil
	}
	if err := os.RemoveAll(l.path); err != nil {
		return fmt.Errorf("release run lock: %w", err)
	}
	l.path = ""
	return nil
}
