package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	EventEffectIntent     = "effect_intent"
	EventEffectSettlement = "effect_settlement"
)

type Event struct {
	Type       string          `json:"type"`
	EffectID   string          `json:"effectId,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
	Generation uint64          `json:"generation"`
}

type FileStore struct {
	root string
}

func New(root string) *FileStore {
	return &FileStore{root: root}
}

func (s *FileStore) RunDir(runID string) string {
	return filepath.Join(s.root, "runs", runID)
}

func (s *FileStore) WriteState(runID string, state any) error {
	runDir := s.RunDir(runID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return fmt.Errorf("create run directory: %w", err)
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	data = append(data, '\n')

	file, err := os.CreateTemp(runDir, ".state-*")
	if err != nil {
		return fmt.Errorf("create temporary state: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("protect temporary state: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write temporary state: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync temporary state: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary state: %w", err)
	}
	if err := os.Rename(tempPath, filepath.Join(runDir, "state.json")); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}
	return syncDirectory(runDir)
}

func (s *FileStore) ReadState(runID string, state any) error {
	data, err := os.ReadFile(filepath.Join(s.RunDir(runID), "state.json"))
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(data, state); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}
	return nil
}

func (s *FileStore) AppendEvent(runID string, event Event) error {
	events, err := s.ReadEvents(runID)
	if err != nil {
		return err
	}
	for _, existing := range events {
		if existing.Generation > event.Generation {
			return fmt.Errorf(
				"event generation %d is older than %d",
				event.Generation,
				existing.Generation,
			)
		}
	}

	runDir := s.RunDir(runID)
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return fmt.Errorf("create run directory: %w", err)
	}
	file, err := os.OpenFile(
		filepath.Join(runDir, "events.jsonl"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0o600,
	)
	if err != nil {
		return fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync event log: %w", err)
	}
	return nil
}

func (s *FileStore) ReadEvents(runID string) ([]Event, error) {
	file, err := os.Open(filepath.Join(s.RunDir(runID), "events.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open event log: %w", err)
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode event: %w", err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan event log: %w", err)
	}
	return events, nil
}

func (s *FileStore) AmbiguousEffects(runID string) ([]string, error) {
	events, err := s.ReadEvents(runID)
	if err != nil {
		return nil, err
	}
	pending := make(map[string]struct{})
	for _, event := range events {
		switch event.Type {
		case EventEffectIntent:
			pending[event.EffectID] = struct{}{}
		case EventEffectSettlement:
			delete(pending, event.EffectID)
		}
	}
	result := make([]string, 0, len(pending))
	for effectID := range pending {
		result = append(result, effectID)
	}
	sort.Strings(result)
	return result, nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
