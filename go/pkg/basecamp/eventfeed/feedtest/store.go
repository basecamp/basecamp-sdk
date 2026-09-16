package feedtest

import (
	"context"
	"slices"
	"sync"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// Store is a scripted eventfeed.CheckpointStore exercising the seam's
// tri-state contract (SPEC.md §23 "Seam Contracts"): load resolves to
// Loaded / Missing / Failed, and each save to Saved or Failed. It records the
// full ordered ledger — every key loaded, every position saved, and the key
// each save carried — so checkpoint identity and save ordering are assertable
// rather than inferred. The default store is Missing on load with every save
// succeeding.
type Store struct {
	mu sync.Mutex

	loadPosition string
	loadOK       bool
	loadErr      error

	saveErrs []error
	onSave   func(position string)

	loads    []eventfeed.CheckpointKey
	saves    []string
	saveKeys []eventfeed.CheckpointKey
}

var _ eventfeed.CheckpointStore = (*Store)(nil)

// NewStore returns a store whose load is Missing and whose saves all succeed.
func NewStore() *Store {
	return &Store{}
}

// Stored scripts load to resolve Loaded(position).
func (s *Store) Stored(position string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadPosition = position
	s.loadOK = true
	s.loadErr = nil
}

// FailLoad scripts load to resolve Failed(err) — the Terminal(checkpoint_load)
// path, which is deliberately NOT collapsible to Missing.
func (s *Store) FailLoad(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadErr = err
}

// FailNextSave scripts the next save to resolve Failed(err) (FIFO when queued
// repeatedly); unscripted saves succeed. Save failures never end the feed, so
// a later save is still expected.
func (s *Store) FailNextSave(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saveErrs = append(s.saveErrs, err)
}

// OnSave registers a callback invoked with the position inside every save
// call, before it returns — the hook an ordering assertion (delivery strictly
// precedes that page's save) records against.
func (s *Store) OnSave(fn func(position string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onSave = fn
}

// Loads returns every key load was called with, in order.
func (s *Store) Loads() []eventfeed.CheckpointKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.loads)
}

// Saves returns every position save was called with, in order — the
// checkpoint ledger records CALLS, so a failed save appears too.
func (s *Store) Saves() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.saves)
}

// SaveKeys returns the key each save call carried, in order.
func (s *Store) SaveKeys() []eventfeed.CheckpointKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.saveKeys)
}

// Load implements eventfeed.CheckpointStore.
func (s *Store) Load(_ context.Context, key eventfeed.CheckpointKey) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loads = append(s.loads, key)
	if s.loadErr != nil {
		return "", false, s.loadErr
	}
	return s.loadPosition, s.loadOK, nil
}

// Save implements eventfeed.CheckpointStore.
func (s *Store) Save(_ context.Context, key eventfeed.CheckpointKey, position string) error {
	s.mu.Lock()
	s.saves = append(s.saves, position)
	s.saveKeys = append(s.saveKeys, key)
	var err error
	if len(s.saveErrs) > 0 {
		err = s.saveErrs[0]
		s.saveErrs = s.saveErrs[1:]
	}
	onSave := s.onSave
	s.mu.Unlock()
	if onSave != nil {
		onSave(position)
	}
	return err
}
