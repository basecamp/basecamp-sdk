package eventfeed

import (
	"fmt"
	"unicode"
)

// maxFilterIDs caps each id list (SPEC.md §23: at most 100 ids per list).
const maxFilterIDs = 100

// Filters narrows the feed to the given event types, buckets, and creators.
// Positions are filter-bound: changing filters starts a new checkpoint
// lineage (the server enforces this with 409).
type Filters struct {
	// Types filters by event type. The catalog is server-owned and never
	// client-validated: a syntactically valid but uncataloged type forms a
	// well-defined filter key, and the first poll draws the server's filter
	// 400 (Terminal(filter_invalid)).
	Types []string
	// Buckets filters by bucket (project) id: positive ids, at most 100.
	Buckets []int64
	// Creators filters by creator id: positive ids, at most 100.
	Creators []int64
}

// Validate applies SPEC.md §23's client-side, fail-closed filter validation:
// type strings must be non-empty with no commas, whitespace, or quotes; ids
// must be positive; each id list is capped at 100. A violation is a
// usage-coded *TerminalError surfaced at construction, with zero wire
// attempts.
func (f Filters) Validate() error {
	for _, typ := range f.Types {
		if typ == "" {
			return usageError("filter types must be non-empty strings")
		}
		for _, r := range typ {
			if r == ',' || r == '"' || r == '\'' || unicode.IsSpace(r) {
				return usageError(fmt.Sprintf("filter type %q must not contain commas, whitespace, or quotes", typ))
			}
		}
	}
	if err := validateFilterIDs("buckets", f.Buckets); err != nil {
		return err
	}
	return validateFilterIDs("creators", f.Creators)
}

// validateFilterIDs enforces the per-list id constraints: at most
// maxFilterIDs entries, every id positive.
func validateFilterIDs(list string, ids []int64) error {
	if len(ids) > maxFilterIDs {
		return usageError(fmt.Sprintf("%s filter lists at most %d ids, got %d", list, maxFilterIDs, len(ids)))
	}
	for _, id := range ids {
		if id <= 0 {
			return usageError(fmt.Sprintf("%s filter ids must be positive, got %d", list, id))
		}
	}
	return nil
}
