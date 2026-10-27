package main

import (
	"bytes"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The fixtures under testdata are a two-release stand-in for the SDK and a
// downstream module pinned to the first release (through a replace). See the
// comments in each fixture file for what it exercises.

var (
	fixtureOnce sync.Once
	fixtureV1   *SDK
	fixtureV2   *SDK
	fixtureErr  error
)

func fixtureSDKs(t *testing.T) (*SDK, *SDK) {
	t.Helper()
	fixtureOnce.Do(func() {
		if fixtureV1, fixtureErr = loadSDK("testdata/sdk-v1", "v1.0.0"); fixtureErr != nil {
			return
		}
		fixtureV2, fixtureErr = loadSDK("testdata/sdk-v2", "v2.0.0")
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	return fixtureV1, fixtureV2
}

func fixtureRepo(t *testing.T, patterns ...string) *Repo {
	t.Helper()
	if len(patterns) == 0 {
		patterns = []string{"./cmd/app"}
	}
	r, err := loadRepo("testdata/downstream", patterns)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func ids(ops []Operation) []string {
	out := []string{}
	for _, op := range ops {
		out = append(out, op.ID)
	}
	sort.Strings(out)
	return out
}

func coveredIDs(r *Report) []string {
	out := []string{}
	for _, c := range r.Covered {
		out = append(out, c.ID)
	}
	return out
}

func assertIDs(t *testing.T, what string, got, want []string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if got == nil {
		got = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

const things = "(*" + sdkPkgPath + ".ThingsService)."

func TestSDKReach(t *testing.T) {
	v1, _ := fixtureSDKs(t)
	for sym, want := range map[string][]string{
		"Get":    {"GetThing"},
		"Create": {"CreateThing"},
		// Through its own Get and an unexported helper's raw PUT.
		"Update":  {"GetThing", "ReplaceThing"},
		"replace": {"ReplaceThing"},
		// A composite reaching into GadgetsService performs nothing of its own.
		"Summary": nil,
	} {
		assertIDs(t, "reach of "+sym, v1.reach[things+sym], want)
	}
	if !v1.declared[things+"Delete"] {
		t.Error("v1 should declare ThingsService.Delete")
	}
	if _, ok := v1.generatedOp("NewClientWithResponses"); ok {
		t.Error("NewClientWithResponses is not an operation")
	}
	for name, want := range map[string]string{
		"CreateThingWithBodyWithResponse": "CreateThing",
		"GetThingWithResponse":            "GetThing",
		"GetThing":                        "GetThing",
		"NewGetThingRequest":              "GetThing",
		"NewCreateThingRequestWithBody":   "CreateThing",
	} {
		if got, _ := v1.generatedOp(name); got != want {
			t.Errorf("generatedOp(%s) = %q, want %q", name, got, want)
		}
	}
}

func TestCoverageAgainstPinnedRelease(t *testing.T) {
	v1, _ := fixtureSDKs(t)
	repo := fixtureRepo(t)
	if repo.PinnedSDK != "v1.0.0" {
		t.Errorf("pinned SDK = %q, want v1.0.0 (the replaced requirement)", repo.PinnedSDK)
	}
	r := buildReport(v1, nil, v1, repo)

	assertIDs(t, "covered", coveredIDs(r), []string{
		"CreateThing",  // a method value
		"DeleteGadget", // through an interface embedded in the repository's own
		"DeleteThing",  // a direct call
		"GetMyProfile", // a raw GET of a literal path
		"GetPerson",    // a raw GET of a concatenated path
		"GetThing",     // a direct call
		"ListThings",   // through the repository's own interface
	})
	// GetGadget is reached only through a composite and a test file;
	// ReplaceThing only from a package the shipped binary does not import.
	assertIDs(t, "uncovered", ids(r.Uncovered), []string{"GetGadget", "ReplaceThing"})
	if len(r.Stale) != 0 {
		t.Errorf("stale = %+v, want none against the pinned release", r.Stale)
	}
	if r.Delta != nil {
		t.Errorf("delta = %+v, want none without a previous release", r.Delta)
	}

	var raw []string
	for _, c := range r.RawCalls {
		raw = append(raw, c.Method+" "+c.Path)
	}
	sort.Strings(raw)
	assertIDs(t, "unmatched raw calls", raw, []string{"GET ", "GET /nowhere/{…}.json"})
}

func TestCoverageCountsOnlyTheChosenPackages(t *testing.T) {
	v1, _ := fixtureSDKs(t)
	r := buildReport(v1, nil, v1, fixtureRepo(t, "./..."))
	assertIDs(t, "uncovered with ./...", ids(r.Uncovered), []string{"GetGadget"})
}

func TestDeltaAgainstPreviousRelease(t *testing.T) {
	v1, v2 := fixtureSDKs(t)
	r := buildReport(v2, v1, v1, fixtureRepo(t))

	assertIDs(t, "covered", coveredIDs(r), []string{"CreateThing", "DeleteGadget", "FetchThing", "GetMyProfile", "GetPerson", "ListThings"})
	assertIDs(t, "uncovered", ids(r.Uncovered), []string{"ArchiveThing", "GetGadget", "GetThing", "ListGadgets", "ReplaceThing"})

	d := r.Delta
	if d == nil || d.Previous != "v1.0.0" {
		t.Fatalf("delta = %+v, want one against v1.0.0", d)
	}
	assertIDs(t, "added", d.Added, []string{"ArchiveThing", "FetchThing", "ListGadgets"})
	assertIDs(t, "removed", d.Removed, []string{"DeleteThing"})
	// FetchThing is new and reached, so it is not newly uncovered.
	assertIDs(t, "newly uncovered", ids(d.NewlyUncovered), []string{"ArchiveThing", "ListGadgets"})
	// GetThing is in both releases; v1's ThingsService.Get performed it and
	// v2's does not.
	assertIDs(t, "lost coverage", ids(d.LostCoverage), []string{"GetThing"})

	if len(r.Stale) != 1 {
		t.Fatalf("stale = %+v, want ThingsService.Delete alone", r.Stale)
	}
	st := r.Stale[0]
	if st.Symbol != "(*basecamp.ThingsService).Delete" || !strings.Contains(st.Reason, "not in SDK v2.0.0") {
		t.Errorf("stale = %+v", st)
	}
	assertIDs(t, "stale operations", st.Operations, []string{"DeleteThing"})
	if len(st.Sites) != 1 || st.Sites[0].File != "internal/cmds/cmds.go" {
		t.Errorf("stale sites = %+v", st.Sites)
	}
}

func TestMarkdown(t *testing.T) {
	v1, v2 := fixtureSDKs(t)
	r := buildReport(v2, v1, v1, fixtureRepo(t))
	r.Commit = "abc123"
	var buf bytes.Buffer
	renderMarkdown(&buf, r)
	md := buf.String()
	for _, want := range []string{
		"### SDK operation coverage: downstream at `abc123` against SDK v2.0.0",
		"| SDK v2.0.0 operations | 11 |",
		"| Reached by downstream | 6 (55%) |",
		"| Not reached | 5 |",
		"| New since v1.0.0 | 3, 2 not reached |",
		"| Removed since v1.0.0 | 1 |",
		"| Reached through v1.0.0, not through v2.0.0 | 1 |",
		"| Stale references to the SDK | 1 |",
		"- `ArchiveThing` (Things): `PUT /{accountId}/things/{thingId}/status/archived.json`",
		"- `(*basecamp.ThingsService).Delete`: not in SDK v2.0.0 (performed `DeleteThing`); `internal/cmds/cmds.go:",
		"| Gadgets | `GetGadget`, `ListGadgets` |",
		"The code is pinned to SDK v1.0.0.",
		"1 raw HTTP call with a path computed at run time may reach operations",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
}
