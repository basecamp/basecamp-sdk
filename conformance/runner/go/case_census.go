package main

// Case census (#602): every non-live fixture case must be accounted for by the
// run.
//
//	passed + failed + skipped  ==  cases in conformance/tests/**/*.json
//	                               whose mode != "live"
//
// The left side is what the runner actually did. The right side is counted by
// countNonLiveCases below — a SEPARATE walk and parse, deliberately not the
// runner's own load path. That independence is the entire point: a check fed by
// the load path can only ever confirm the load path agrees with itself.
//
// Why `mode != "live"` rather than `mode == "mock"`. All six runners select
// cases with "mock unless told otherwise" (`isMockMode` here, and its five
// equivalents), so a typo'd `mode: "moc"` is dropped by every runner at once
// with nothing printed anywhere. Counting the expected side as "not explicitly
// live" is what turns that silent divergence into arithmetic.
//
// What it catches: a typo'd or otherwise unrecognized `mode`; a fixture file
// that failed to parse or was never globbed (including one nested below
// conformance/tests/, which no runner discovers — hence the recursive walk); a
// case dropped between load and dispatch; a whole fixture emptied to `[]` (which
// the census REFUSES rather than counts — see countNonLiveCases, and note that
// counting it would make this bullet a lie); and any future skip channel that
// bypasses the counters, because the counters are what it reads rather than any
// particular skip mechanism.
//
// The typo is not this check's alone to catch, and saying so is what keeps the
// rest of the list honest: `make conformance-fixtures-check` validates
// conformance/tests/*.json against conformance/schema.json, whose `mode` is
// `enum: ["mock", "live"]`, so a typo in a TOP-LEVEL fixture fails there first
// and this census is defense in depth for that one case. What that gate
// structurally cannot see is everything else above — its glob is not recursive,
// so a fixture nested below conformance/tests/ is validated by nothing AND run
// by nothing (verified: such a file passes the schema gate and fails this
// census); a fixture truncated to `[]` is a valid array of zero cases; and a
// case dropped between load and dispatch is not a fixture-format question at
// all. Nor does that gate run when `make conformance-<lang>` is invoked alone.
//
// What it does NOT catch, stated rather than implied: the literal all-six case
// #602 names — one fixture case that every runner excludes for its own separate
// reason. Each runner's census is green in that situation, because each runner
// counted its own skip. Detecting it needs the six exclusion sets in one place,
// which needs artifact plumbing across six CI jobs; #602 stays open for that.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// isMockMode reports whether a fixture case's `mode` selects this runner.
//
// Absent means mock: live cases are TS-only (the canonical wire-capturer), and
// every other mode value is nobody's. Shared with the census self-tests so the
// rule the run loop applies is the rule under test, not a copy of it.
//
// The parameter is a pointer because a plain string cannot tell an ABSENT key
// from `"mode": ""`, and the two must not be the same answer. The other five
// runners null-coalesce, so they run the first and refuse the second; a Go
// `string` collapses both to "" and would run an unrecognized mode the other
// five reject — a hole in the very invariant this census exists to hold, in the
// reference runner.
func isMockMode(mode *string) bool {
	return mode == nil || *mode == "mock"
}

// countNonLiveCases counts the fixture cases whose mode is not "live",
// recursively, under dir.
//
// Fail-closed in three places, each of which is a way the count could certify
// nothing while looking green: an unreadable tree, a fixture that does not
// parse, and a walk that found no fixture files at all.
func countNonLiveCases(dir string) (int, error) {
	cases, files := 0, 0

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		files++

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Only `mode` is read: the census must survive a fixture whose other
		// fields this runner cannot model, or it would report a parse failure
		// for a case the run itself handled fine.
		var parsed []struct {
			Mode *string `json:"mode"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		// A top-level `null` unmarshals into a nil slice without error, so it
		// would otherwise pass as a fixture of zero cases — the one non-array
		// root the other five runners reject and this one would not.
		if parsed == nil {
			return fmt.Errorf("%s: fixture is not a JSON array of cases", path)
		}
		// An emptied fixture is REFUSED, not counted as zero, and this is the
		// one rejection that carries the whole-file guarantee. It is the single
		// truncation both sides of the census read identically: the runner
		// registers nothing from the file and the census expects nothing, so
		// the two totals fall together and no mismatch ever appears. Counting
		// it would make "a fixture truncated to []" a claim this check cannot
		// keep. A file declaring no cases tests nothing, so refusing it costs
		// nothing — and it closes the same hole in conformance-fixtures-check,
		// where an empty array is a schema-valid list of zero items.
		if len(parsed) == 0 {
			return fmt.Errorf("%s: fixture declares no cases; delete the file or restore its cases", path)
		}
		for _, c := range parsed {
			if c.Mode == nil || *c.Mode != "live" {
				cases++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if files == 0 {
		return 0, fmt.Errorf("no *.json fixture files found under %s", dir)
	}
	return cases, nil
}

// caseCountFailure compares what the run accounted for against the census,
// returning "" when they agree and a message naming the short side otherwise.
func caseCountFailure(ran, expected int) string {
	switch {
	case ran == expected:
		return ""
	case ran < expected:
		return fmt.Sprintf(
			"case census: the run accounted for %d case(s) (passed+failed+skipped) "+
				"but conformance/tests holds %d non-live case(s) — %d executed by nothing. "+
				"An unrecognized `mode`, a fixture that failed to parse or was never globbed, "+
				"or a case dropped between load and dispatch will do this.",
			ran, expected, expected-ran)
	default:
		return fmt.Sprintf(
			"case census: the run accounted for %d case(s) (passed+failed+skipped) "+
				"but conformance/tests holds only %d non-live case(s) — %d more than the fixtures declare.",
			ran, expected, ran-expected)
	}
}
