package eventfeed

import (
	"fmt"
	"slices"
	"unicode"
)

// maxFilterIDs caps each id list (SPEC.md §23: at most 100 ids per list).
const maxFilterIDs = 100

// Actor types for Filters.ActorTypes (SPEC.md §23 "Consumer Surface"): a
// delegated event's actor is the agent that performed it; a direct event's
// actor is its creator, typed by what it currently is — humans, integrations,
// and tombstones are all `person`. The vocabulary is server-owned, like the
// type catalog: these constants name the two published values, and the
// connector never validates membership.
const (
	ActorTypeAgent  = "agent"
	ActorTypePerson = "person"
)

// Filters narrows the feed. Positions are filter-bound: changing filters
// starts a new checkpoint lineage (the server enforces this with 409).
//
// Every list is a filter dimension of the server's srv2 digest scheme
// (Filters.Digest), and every id is an id: the server's `self` literal —
// "the request's effective actor", which it resolves to that principal's id
// before filtering and before digesting — is NOT accepted here. The
// connector performs no wire I/O of its own, so it cannot resolve a
// principal, and the checkpoint key IS the server digest, which is defined
// over the resolved id. A caller that wants the loop guard
// (`exclude_performers=self`) resolves its own id once — the identity
// endpoint, or the agent connection's account record — and passes it in
// ExcludePerformers; the server treats the id and the literal identically.
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
	// Performers filters by effective performer id — the agent that carried
	// out a delegated action (the event's performed_by_id), else the
	// creator: positive ids, at most 100.
	Performers []int64
	// ExcludePerformers excludes events by effective performer id: positive
	// ids, at most 100. An agent that acts on what it hears excludes its own
	// id here rather than suppressing all agent activity with ActorTypes —
	// agent activity is real account activity, and suppressing it wholesale
	// breaks agent-to-agent workflows.
	ExcludePerformers []int64
	// ActorTypes filters by actor kind: ActorTypeAgent, ActorTypePerson, or
	// both. An opt-in filter, never a default. The vocabulary is
	// server-owned and validated only syntactically, like Types.
	ActorTypes []string
	// Reasons is the inbox lane's own dimension — why an item addressed the
	// principal (mentioned, assigned, subscribed, watched, pinged, boosted).
	// It is an srv2 digest dimension like the others, so a checkpoint
	// lineage keyed by it is well-defined here; the account feed has no use
	// for it. The vocabulary is server-owned and validated only
	// syntactically, like Types.
	Reasons []string
}

// clone returns a connector-owned copy of the filter set. Filters is seven
// slices, so the connector must not retain the caller's backing arrays: a
// mutation after construction would otherwise change the subscription, the
// poll parameters and the checkpoint lineage AFTER validation passed —
// including into a filter set Validate rejects — and could split them from
// one another, since the subscription identifier is frozen once per
// connection while polls and checkpoint keys are derived per use.
func (f Filters) clone() Filters {
	return Filters{
		Types:             slices.Clone(f.Types),
		Buckets:           slices.Clone(f.Buckets),
		Creators:          slices.Clone(f.Creators),
		Performers:        slices.Clone(f.Performers),
		ExcludePerformers: slices.Clone(f.ExcludePerformers),
		ActorTypes:        slices.Clone(f.ActorTypes),
		Reasons:           slices.Clone(f.Reasons),
	}
}

// Validate applies SPEC.md §23's client-side, fail-closed filter validation:
// type, actor-type and reason strings must be non-empty, valid UTF-8, with no
// commas, whitespace, or quotes; ids must be positive; each id list is
// capped at 100. A violation is a usage-coded *TerminalError surfaced at
// construction, with zero wire attempts.
func (f Filters) Validate() error {
	if err := validateFilterStrings("type", f.Types); err != nil {
		return err
	}
	if err := validateFilterStrings("actor type", f.ActorTypes); err != nil {
		return err
	}
	if err := validateFilterStrings("reason", f.Reasons); err != nil {
		return err
	}
	for _, list := range []struct {
		name string
		ids  []int64
	}{
		{"buckets", f.Buckets},
		{"creators", f.Creators},
		{"performers", f.Performers},
		{"exclude_performers", f.ExcludePerformers},
	} {
		if err := validateFilterIDs(list.name, list.ids); err != nil {
			return err
		}
	}
	return nil
}

// validateFilterStrings enforces the string-dimension constraints: every
// entry non-empty, valid UTF-8, free of commas, whitespace, and quotes.
// Entries feed the srv2 digest — a checkpoint-identity component — and the
// comma-joined subscription identifier, both of which encode rune-wise (see
// checkIdentityText).
func validateFilterStrings(what string, values []string) error {
	for _, v := range values {
		if v == "" {
			return usageError(fmt.Sprintf("filter %ss must be non-empty strings", what))
		}
		if err := checkIdentityText(fmt.Sprintf("filter %s %q", what, v), v); err != nil {
			return err
		}
		for _, r := range v {
			if r == ',' || r == '"' || r == '\'' || unicode.IsSpace(r) {
				return usageError(fmt.Sprintf("filter %s %q must not contain commas, whitespace, or quotes", what, v))
			}
		}
	}
	return nil
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
