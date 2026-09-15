package feedtest

import (
	"context"
	"errors"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

func TestStore_DefaultIsMissingWithSavesSucceeding(t *testing.T) {
	s := NewStore()
	pos, ok, err := s.Load(context.Background(), eventfeed.CheckpointKey{AccountID: "1"})
	if pos != "" || ok || err != nil {
		t.Fatalf("Load() = (%q, %v, %v), want Missing", pos, ok, err)
	}
	if err := s.Save(context.Background(), eventfeed.CheckpointKey{AccountID: "1"}, "pos-1"); err != nil {
		t.Fatalf("Save() = %v, want success", err)
	}
}

func TestStore_StoredResolvesLoaded(t *testing.T) {
	s := NewStore()
	s.Stored("pos-9")
	pos, ok, err := s.Load(context.Background(), eventfeed.CheckpointKey{})
	if pos != "pos-9" || !ok || err != nil {
		t.Fatalf("Load() = (%q, %v, %v), want Loaded(pos-9)", pos, ok, err)
	}
}

func TestStore_FailLoadIsFailedNotMissing(t *testing.T) {
	s := NewStore()
	// Failed must win even over a scripted position: the Terminal
	// (checkpoint_load) path is deliberately NOT collapsible to Missing.
	s.Stored("pos-9")
	boom := errors.New("boom")
	s.FailLoad(boom)
	pos, ok, err := s.Load(context.Background(), eventfeed.CheckpointKey{})
	if err != boom {
		t.Fatalf("Load() error = %v, want the scripted failure", err)
	}
	if pos != "" || ok {
		t.Fatalf("Load() = (%q, %v) alongside the failure, want zero values", pos, ok)
	}
}

func TestStore_FailNextSaveIsFIFOThenSucceeds(t *testing.T) {
	s := NewStore()
	first, second := errors.New("first"), errors.New("second")
	s.FailNextSave(first)
	s.FailNextSave(second)
	key := eventfeed.CheckpointKey{AccountID: "1"}
	if err := s.Save(context.Background(), key, "pos-1"); err != first {
		t.Fatalf("first save = %v, want the first scripted failure", err)
	}
	if err := s.Save(context.Background(), key, "pos-2"); err != second {
		t.Fatalf("second save = %v, want the second scripted failure", err)
	}
	if err := s.Save(context.Background(), key, "pos-3"); err != nil {
		t.Fatalf("third save = %v, want success once the script is drained", err)
	}
	// The ledger records CALLS, so the failed saves appear too.
	if saves := s.Saves(); len(saves) != 3 || saves[0] != "pos-1" || saves[1] != "pos-2" || saves[2] != "pos-3" {
		t.Fatalf("Saves() = %v, want all three calls in order", saves)
	}
}

func TestStore_LedgersRecordEveryCallInOrder(t *testing.T) {
	s := NewStore()
	k1 := eventfeed.CheckpointKey{AccountID: "1", FilterKey: "srv1-a"}
	k2 := eventfeed.CheckpointKey{AccountID: "2", FilterKey: "srv1-b"}
	_, _, _ = s.Load(context.Background(), k1)
	_ = s.Save(context.Background(), k2, "pos-1")
	_, _, _ = s.Load(context.Background(), k2)
	_ = s.Save(context.Background(), k1, "pos-2")
	if loads := s.Loads(); len(loads) != 2 || loads[0] != k1 || loads[1] != k2 {
		t.Fatalf("Loads() = %v, want both keys in order", loads)
	}
	if keys := s.SaveKeys(); len(keys) != 2 || keys[0] != k2 || keys[1] != k1 {
		t.Fatalf("SaveKeys() = %v, want the key each save carried, in order", keys)
	}
	if saves := s.Saves(); len(saves) != 2 || saves[0] != "pos-1" || saves[1] != "pos-2" {
		t.Fatalf("Saves() = %v, want both positions in order", saves)
	}
}

func TestStore_OnSaveFiresBeforeReturnAndOutsideTheLock(t *testing.T) {
	s := NewStore()
	var seen []string
	s.OnSave(func(position string) {
		// Calling back into the store must not deadlock: the callback runs
		// outside the lock, after the call is logged, so an ordering
		// assertion can read the ledger from inside it.
		saves := s.Saves()
		seen = append(seen, position, saves[len(saves)-1])
	})
	if err := s.Save(context.Background(), eventfeed.CheckpointKey{}, "pos-1"); err != nil {
		t.Fatalf("Save() = %v, want success", err)
	}
	if len(seen) != 2 || seen[0] != "pos-1" || seen[1] != "pos-1" {
		t.Fatalf("OnSave saw %v, want the position, already in the ledger", seen)
	}
}
