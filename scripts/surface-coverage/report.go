package main

import (
	"sort"
	"strings"
)

// Report is the coverage of one SDK version's operations by one downstream
// repository. It is also the JSON output's schema.
type Report struct {
	SDK        string   `json:"sdk"`
	Repo       string   `json:"repo"`
	Ref        string   `json:"ref,omitempty"`
	Commit     string   `json:"commit,omitempty"`
	PinnedSDK  string   `json:"pinned_sdk"`
	Patterns   []string `json:"patterns"`
	Packages   int      `json:"packages"`
	TypeErrors int      `json:"type_errors"`

	Operations int         `json:"operations"`
	Covered    []CoveredOp `json:"covered"`
	Uncovered  []Operation `json:"uncovered"`
	Stale      []StaleRef  `json:"stale"`
	Missing    []site      `json:"missing_symbols,omitempty"`
	Ambiguous  []site      `json:"ambiguous_interface_calls,omitempty"`
	RawCalls   []RawCall   `json:"unmatched_raw_calls,omitempty"`
	Delta      *Delta      `json:"delta,omitempty"`
}

// CoveredOp is an operation the repository reaches, and from where.
type CoveredOp struct {
	Operation
	Via []Via `json:"via"`
}

// Via is one place the repository reaches an operation: the SDK symbol it
// references there.
type Via struct {
	Symbol string `json:"symbol"`
	site
}

// StaleRef is a reference to SDK surface this SDK version no longer has.
type StaleRef struct {
	Symbol string `json:"symbol"`
	Reason string `json:"reason"`
	// Operations are what the symbol performed in the SDK the repository is
	// pinned to, when that differs from the version reported on.
	Operations []string `json:"operations,omitempty"`
	Sites      []site   `json:"sites"`
}

// RawCall is an untyped HTTP call whose path matches no operation, or whose
// path is computed beyond what the tool can read.
type RawCall struct {
	Method string `json:"method"`
	Path   string `json:"path,omitempty"` // with {…} where computed; empty when unreadable
	Site   site   `json:"site"`
}

// Delta is what changed against the previous SDK release.
type Delta struct {
	Previous string   `json:"previous"`
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	// NewlyUncovered are operations added in this release that the
	// repository does not reach.
	NewlyUncovered []Operation `json:"newly_uncovered"`
	// LostCoverage are operations both releases have that the repository
	// reached through the previous release's SDK and does not reach through
	// this one — a wrapper that stopped calling an operation, say.
	LostCoverage []Operation `json:"lost_coverage"`
}

// coverage resolves the repository's references through one SDK version.
func coverage(s *SDK, refs []siteRef) (covered map[string][]Via, undeclared map[string][]siteRef) {
	covered = map[string][]Via{}
	undeclared = map[string][]siteRef{}
	for _, r := range refs {
		ops, ok := s.opsFor(r.ref)
		if !ok {
			undeclared[r.Key] = append(undeclared[r.Key], r)
			continue
		}
		for _, op := range ops {
			covered[op] = append(covered[op], Via{Symbol: shortSymbol(r.Key), site: r.Site})
		}
	}
	return covered, undeclared
}

// buildReport computes the report for repo against sdk. prev (the previous
// release) and pinned (the version the repository's go.mod resolves) are
// optional; either may be the same *SDK as sdk.
func buildReport(sdk, prev, pinned *SDK, repo *Repo) *Report {
	rep := &Report{
		SDK:        sdk.Version,
		Repo:       repo.Module,
		PinnedSDK:  repo.PinnedSDK,
		Patterns:   repo.Patterns,
		Packages:   len(repo.Packages),
		TypeErrors: repo.TypeErrors,
		Operations: len(sdk.Operations),
		Missing:    repo.Missing,
		Ambiguous:  repo.Ambiguous,
		// Empty lists, not null, so a JSON reader can take len() of each.
		Covered:   []CoveredOp{},
		Uncovered: []Operation{},
		Stale:     []StaleRef{},
	}
	covered, undeclared := coverage(sdk, repo.Refs)
	for _, op := range sdk.Operations {
		if via, ok := covered[op.ID]; ok {
			rep.Covered = append(rep.Covered, CoveredOp{Operation: op, Via: dedupeVia(via)})
		} else {
			rep.Uncovered = append(rep.Uncovered, op)
		}
	}

	// Stale: symbols this SDK does not declare, and symbols that reached
	// operations (in the pinned SDK) this SDK no longer has.
	for _, key := range sortedKeys(keysOf(undeclared)) {
		st := StaleRef{Symbol: shortSymbol(key), Reason: "not in SDK " + sdk.Version}
		ops := map[string]bool{}
		for _, r := range undeclared[key] {
			st.Sites = append(st.Sites, r.Site)
			if pinned != nil && pinned != sdk {
				performed, _ := pinned.opsFor(r.ref)
				for _, op := range performed {
					ops[op] = true
				}
			}
		}
		if len(ops) > 0 {
			st.Operations = sortedKeys(ops)
		}
		rep.Stale = append(rep.Stale, st)
	}
	if pinned != nil && pinned != sdk {
		gone := map[string][]siteRef{}
		var goneOps = map[string]map[string]bool{}
		for _, r := range repo.Refs {
			if _, isUndeclared := undeclared[r.Key]; isUndeclared {
				continue
			}
			ops, _ := pinned.opsFor(r.ref)
			for _, op := range ops {
				if _, still := sdk.ops[op]; still {
					continue
				}
				id := r.Key
				if r.Raw != "" {
					id = r.Raw + " " + display(r.Pattern)
				}
				gone[id] = append(gone[id], r)
				if goneOps[id] == nil {
					goneOps[id] = map[string]bool{}
				}
				goneOps[id][op] = true
			}
		}
		for _, id := range sortedKeys(keysOf(gone)) {
			st := StaleRef{Symbol: shortSymbol(id), Reason: "operation removed in SDK " + sdk.Version, Operations: sortedKeys(goneOps[id])}
			for _, r := range gone[id] {
				st.Sites = append(st.Sites, r.Site)
			}
			rep.Stale = append(rep.Stale, st)
		}
	}

	seenRaw := map[RawCall]bool{}
	for _, r := range repo.Refs {
		if r.Raw == "" {
			continue
		}
		c := RawCall{Method: r.Raw, Site: r.Site}
		if r.Resolved {
			if _, ok := sdk.routes.match(r.Raw, r.Pattern); ok {
				continue
			}
			c.Path = display(r.Pattern)
		}
		if !seenRaw[c] {
			seenRaw[c] = true
			rep.RawCalls = append(rep.RawCalls, c)
		}
	}

	if prev != nil && prev != sdk {
		rep.Delta = delta(sdk, prev, repo, rep)
	}
	return rep
}

func delta(sdk, prev *SDK, repo *Repo, rep *Report) *Delta {
	d := &Delta{Previous: prev.Version, Added: []string{}, Removed: []string{}, NewlyUncovered: []Operation{}, LostCoverage: []Operation{}}
	for _, op := range sdk.Operations {
		if _, ok := prev.ops[op.ID]; !ok {
			d.Added = append(d.Added, op.ID)
		}
	}
	for _, op := range prev.Operations {
		if _, ok := sdk.ops[op.ID]; !ok {
			d.Removed = append(d.Removed, op.ID)
		}
	}
	prevCovered, _ := coverage(prev, repo.Refs)
	for _, op := range rep.Uncovered {
		if _, existed := prev.ops[op.ID]; !existed {
			d.NewlyUncovered = append(d.NewlyUncovered, op)
		} else if _, was := prevCovered[op.ID]; was {
			d.LostCoverage = append(d.LostCoverage, op)
		}
	}
	return d
}

func dedupeVia(via []Via) []Via {
	seen := map[Via]bool{}
	var out []Via
	for _, v := range via {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func keysOf[V any](m map[string]V) map[string]bool {
	out := map[string]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

// shortSymbol drops the SDK's module path from a symbol for display:
// "(*github.com/basecamp/basecamp-sdk/go/pkg/basecamp.TodosService).List"
// reads "(*basecamp.TodosService).List".
func shortSymbol(key string) string {
	return strings.ReplaceAll(key, sdkModulePath+"/pkg/", "")
}

// display renders a raw path pattern with {…} where it is computed.
func display(pattern string) string {
	return strings.ReplaceAll(pattern, wild, "{…}")
}
