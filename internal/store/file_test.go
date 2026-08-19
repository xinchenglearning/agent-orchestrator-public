package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/xinchenglearning/agent-orchestrator/internal/store"
)

func TestFileStoreWritesStateAtomically(t *testing.T) {
	root := t.TempDir()
	s := store.New(root)

	if err := s.WriteState("run-1", map[string]string{"state": "CREATED"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "runs", "run-1", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got["state"] != "CREATED" {
		t.Fatalf("got state %q", got["state"])
	}
	var reread map[string]string
	if err := s.ReadState("run-1", &reread); err != nil {
		t.Fatal(err)
	}
	if reread["state"] != "CREATED" {
		t.Fatalf("reread state %q", reread["state"])
	}
	matches, err := filepath.Glob(filepath.Join(root, "runs", "run-1", ".state-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary state files remain: %v", matches)
	}
}

func TestFileStorePreservesEventsAndRejectsOldGeneration(t *testing.T) {
	s := store.New(t.TempDir())
	events := []store.Event{
		{Type: "created", Generation: 2},
		{Type: "prepared", Generation: 2},
	}
	for _, event := range events {
		if err := s.AppendEvent("run-1", event); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.AppendEvent("run-1", store.Event{
		Type:       "late",
		Generation: 1,
	}); err == nil {
		t.Fatal("expected old generation rejection")
	}

	got, err := s.ReadEvents("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, events) {
		t.Fatalf("got %#v, want %#v", got, events)
	}
}

func TestFileStoreReportsUnsettledEffects(t *testing.T) {
	s := store.New(t.TempDir())
	for _, event := range []store.Event{
		{Type: store.EventEffectIntent, EffectID: "write-1", Generation: 1},
		{Type: store.EventEffectIntent, EffectID: "write-2", Generation: 1},
		{Type: store.EventEffectSettlement, EffectID: "write-2", Generation: 1},
	} {
		if err := s.AppendEvent("run-1", event); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.AmbiguousEffects("run-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"write-1"}) {
		t.Fatalf("got %v", got)
	}
}
