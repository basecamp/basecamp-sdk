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
