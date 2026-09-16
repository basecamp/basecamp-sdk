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
		{Performers: ids, ExcludePerformers: []int64{9}, ActorTypes: []string{ActorTypeAgent, ActorTypePerson}},
		// The actor-type vocabulary is server-owned like the type catalog:
		// syntactically valid strings pass, and the server's filter 400
		// decides membership.
		{ActorTypes: []string{"integration"}},
		{Reasons: []string{"mentioned", "assigned"}},
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
		{"zero performer id", Filters{Performers: []int64{0}}},
		{"negative excluded performer id", Filters{ExcludePerformers: []int64{-1}}},
		{"over 100 performers", Filters{Performers: tooMany}},
		{"over 100 excluded performers", Filters{ExcludePerformers: tooMany}},
		{"empty actor type", Filters{ActorTypes: []string{""}}},
		{"actor type with comma", Filters{ActorTypes: []string{"agent,person"}}},
		{"actor type with space", Filters{ActorTypes: []string{"agent person"}}},
		{"actor type with quote", Filters{ActorTypes: []string{`"agent"`}}},
		{"empty reason", Filters{Reasons: []string{""}}},
		{"reason with comma", Filters{Reasons: []string{"mentioned,assigned"}}},
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
		Types:             []string{"message.created"},
		Buckets:           []int64{1},
		Creators:          []int64{2},
		Performers:        []int64{3},
		ExcludePerformers: []int64{4},
		ActorTypes:        []string{ActorTypeAgent},
		Reasons:           []string{"mentioned"},
	}
	got := caller.clone()

	// Mutating every one of the caller's slices in place must not be visible
	// through the clone. In-place assignment, not append: append may or may
	// not share the array depending on capacity, so it cannot discriminate.
	caller.Types[0] = "" // a value Validate rejects
	caller.Buckets[0] = -1
	caller.Creators[0] = -1
	caller.Performers[0] = -1
	caller.ExcludePerformers[0] = -1
	caller.ActorTypes[0] = ""
	caller.Reasons[0] = ""

	if got.Types[0] != "message.created" {
		t.Errorf("clone().Types[0] = %q after caller mutation, want %q", got.Types[0], "message.created")
	}
	if got.Buckets[0] != 1 {
		t.Errorf("clone().Buckets[0] = %d after caller mutation, want 1", got.Buckets[0])
	}
	if got.Creators[0] != 2 {
		t.Errorf("clone().Creators[0] = %d after caller mutation, want 2", got.Creators[0])
	}
	if got.Performers[0] != 3 || got.ExcludePerformers[0] != 4 || got.ActorTypes[0] != ActorTypeAgent || got.Reasons[0] != "mentioned" {
		t.Errorf("clone() = %+v after caller mutation, want performers 3 / excluded 4 / actor type agent / reason mentioned", got)
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
	if got.Types != nil || got.Buckets != nil || got.Creators != nil ||
		got.Performers != nil || got.ExcludePerformers != nil || got.ActorTypes != nil || got.Reasons != nil {
		t.Errorf("Filters{}.clone() = %#v, want all-nil slices", got)
	}
	if want := (Filters{}).FilterKey(); want != got.FilterKey() {
		t.Errorf("clone changed the filter key: %q != %q", want, got.FilterKey())
	}
}
