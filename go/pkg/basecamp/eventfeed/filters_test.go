package eventfeed

import (
	"errors"
	"testing"
)

func TestFiltersValidate_AcceptsValidFilters(t *testing.T) {
	ids := make([]int64, maxFilterIDs)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	valid := []Filters{
		{}, // the empty filter set
		{Types: []string{"message.created", "todo.completed"}},
		{Buckets: ids, Creators: []int64{7}},
		// Catalog membership is server-owned and never client-validated: a
		// syntactically valid but uncataloged type passes.
		{Types: []string{"not.a.cataloged.type"}},
	}
	for _, f := range valid {
		if err := f.Validate(); err != nil {
			t.Errorf("Validate(%+v) = %v, want nil", f, err)
		}
	}
}

func TestFiltersValidate_RejectsInvalidFilters(t *testing.T) {
	tooMany := make([]int64, maxFilterIDs+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	cases := []struct {
		name    string
		filters Filters
	}{
		{"empty type string", Filters{Types: []string{""}}},
		{"type with comma", Filters{Types: []string{"message.created,todo.completed"}}},
		{"type with space", Filters{Types: []string{"message created"}}},
		{"type with tab", Filters{Types: []string{"message\tcreated"}}},
		{"type with newline", Filters{Types: []string{"message\ncreated"}}},
		{"type with double quote", Filters{Types: []string{`message."created"`}}},
		{"type with single quote", Filters{Types: []string{"message.'created'"}}},
		{"zero bucket id", Filters{Buckets: []int64{0}}},
		{"negative bucket id", Filters{Buckets: []int64{-5}}},
		{"zero creator id", Filters{Creators: []int64{0}}},
		{"negative creator id", Filters{Creators: []int64{-1}}},
		{"over 100 buckets", Filters{Buckets: tooMany}},
		{"over 100 creators", Filters{Creators: tooMany}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.filters.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want usage-coded error")
			}
			var term *TerminalError
			if !errors.As(err, &term) {
				t.Fatalf("expected *TerminalError, got %T: %v", err, err)
			}
			if term.Reason != ReasonUsage {
				t.Errorf("Reason = %q, want %q", term.Reason, ReasonUsage)
			}
		})
	}
}

// TestFiltersCloneDoesNotAliasCallerSlices pins clone's whole reason for
// existing. connector_test.go covers the same invariant end-to-end through
// WithFilters, but only for the paths a constructed Connector happens to
// take; this is the method's own contract. Filters is three slices, so a
// clone that copied the struct alone would leave the connector holding the
// caller's backing arrays — and a mutation after construction would then
// change the subscription identifier, the poll parameters and the checkpoint
// lineage AFTER Validate passed, including into a set Validate rejects.
func TestFiltersCloneDoesNotAliasCallerSlices(t *testing.T) {
	caller := Filters{
		Types:    []string{"message.created"},
		Buckets:  []int64{1},
		Creators: []int64{2},
	}
	got := caller.clone()

	// Mutating every one of the caller's slices in place must not be visible
	// through the clone. In-place assignment, not append: append may or may
	// not share the array depending on capacity, so it cannot discriminate.
	caller.Types[0] = "" // a value Validate rejects
	caller.Buckets[0] = -1
	caller.Creators[0] = -1

	if got.Types[0] != "message.created" {
		t.Errorf("clone().Types[0] = %q after caller mutation, want %q", got.Types[0], "message.created")
	}
	if got.Buckets[0] != 1 {
		t.Errorf("clone().Buckets[0] = %d after caller mutation, want 1", got.Buckets[0])
	}
	if got.Creators[0] != 2 {
		t.Errorf("clone().Creators[0] = %d after caller mutation, want 2", got.Creators[0])
	}
	if err := got.Validate(); err != nil {
		t.Errorf("clone().Validate() = %v after the caller mutated itself into an invalid set, want nil", err)
	}
}

// A nil slice must clone to a nil slice, not an empty one: clone preserves
// the value verbatim instead of normalizing it. Nothing downstream tells the
// two apart — canonicalJSON and subscribeIdentifier both branch on len, so
// the filter key is identical either way, as the second assertion shows.
func TestFiltersCloneKeepsNilDistinctFromEmpty(t *testing.T) {
	got := Filters{}.clone()
	if got.Types != nil || got.Buckets != nil || got.Creators != nil {
		t.Errorf("Filters{}.clone() = %#v, want all-nil slices", got)
	}
	if want := (Filters{}).FilterKey(); want != got.FilterKey() {
		t.Errorf("clone changed the filter key: %q != %q", want, got.FilterKey())
	}
}
