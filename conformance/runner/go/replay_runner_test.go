// Regression tests for the wire-replay runner. Covers:
//
//  * The empty-bodyText decode-masking bug. Pre-fix, BodyText was a
//    `string` so encoding/json zero-fills a missing key with "". The
//    decode path then conflated "" (missing) with "" (empty HTTP body)
//    and re-serialized `body` instead, silently green-passing an
//    actually-empty wire payload. Post-fix, BodyText is `*string` and
//    resolveBodyText distinguishes nil (missing) from &"" (empty),
//    letting the decoder fail honestly on an empty body.
//
//  * The empty-pages snapshot green-pass bug. Pre-fix, a snapshot like
//    `{"operation":"GetProject"}` unmarshaled with Pages == nil; the
//    per-page loop ran zero times and Run() recorded zero failures —
//    a silent success without any decode attempted. Post-fix,
//    readSnapshot rejects empty pages and pages_count mismatches.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func strPtr(s string) *string { return &s }

// liveFixtureOperations returns the distinct operations conformance's live
// fixture declares — the same set the replay runner's coverage gate compares
// against, read from the same file.
func liveFixtureOperations(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "tests", "live-my-surface.json"))
	if err != nil {
		t.Fatalf("reading live fixture: %v", err)
	}
	var all []fixtureTest
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatalf("parsing live fixture: %v", err)
	}
	seen := map[string]bool{}
	var ops []string
	for _, tc := range all {
		if tc.Mode == "live" && !seen[tc.Operation] {
			seen[tc.Operation] = true
			ops = append(ops, tc.Operation)
		}
	}
	if len(ops) == 0 {
		t.Fatal("live fixture declared no live operations — the reader is broken")
	}
	sort.Strings(ops)
	return ops
}

// The runtime coverage gate makes the same assertion, but it only runs during
// a live canary — which skips whenever the canary secrets are unset. That is
// how this map sat 20 operations behind the fixture (#553). Asserting it as a
// unit test means the drift fails a normal CI run.
func TestDecodersCoverEveryLiveFixtureOperation(t *testing.T) {
	for _, op := range liveFixtureOperations(t) {
		if _, ok := decoders[op]; !ok {
			t.Errorf("no decoder registered for live operation %s", op)
		}
	}
}

func TestNoDecoderRegisteredForAnUnknownOperation(t *testing.T) {
	live := map[string]bool{}
	for _, op := range liveFixtureOperations(t) {
		live[op] = true
	}
	for op := range decoders {
		if !live[op] {
			t.Errorf("decoder registered for %s, which is not a live fixture operation", op)
		}
	}
}

// Every decoder must be bound to a CONCRETE response shape: exactly one of
// `[]` and `{}` decodes. Registering the wrong generated type (a struct where
// the wire sends an array, say GetEverythingMessagesResponseContent vs
// Recording) still compiles and still populates the map, so the parity check
// alone would call it covered. A decoder that accepted both would be typed
// `any` or `json.RawMessage` and assert nothing at all.
func TestEveryDecoderIsBoundToOneWireShape(t *testing.T) {
	for op, dec := range decoders {
		arrayOK := dec(`[]`) == nil
		objectOK := dec(`{}`) == nil
		if arrayOK == objectOK {
			t.Errorf("decoder %s accepts array=%v object=%v; expected exactly one — it is not bound to a concrete response type",
				op, arrayOK, objectOK)
		}
	}
}

func TestResolveBodyText_EmptyPassesThrough(t *testing.T) {
	got := resolveBodyText(wirePage{BodyText: strPtr("")})
	if got != "" {
		t.Fatalf("empty bodyText should pass through as empty string; got %q", got)
	}
}

func TestResolveBodyText_MissingFallsBackToBody(t *testing.T) {
	got := resolveBodyText(wirePage{Body: map[string]any{"a": 1}})
	want := `{"a":1}`
	if got != want {
		t.Fatalf("missing bodyText should serialize body; got %q want %q", got, want)
	}
}

func TestResolveBodyText_NonEmptyWinsOverBody(t *testing.T) {
	got := resolveBodyText(wirePage{
		BodyText: strPtr(`{"b":2}`),
		Body:     map[string]any{"a": 1},
	})
	if got != `{"b":2}` {
		t.Fatalf("bodyText should win over body; got %q", got)
	}
}

func TestDecoder_ErrorsOnEmptyBodyText(t *testing.T) {
	// Composes the regression: empty bodyText → "" → decoder errors.
	// Pre-fix this path would have green-passed because "" got replaced
	// by `{}` before reaching the decoder.
	text := resolveBodyText(wirePage{BodyText: strPtr("")})
	dec, ok := decoders["GetProject"]
	if !ok {
		t.Fatal("GetProject decoder missing from decoders map")
	}
	if err := dec(text); err == nil {
		t.Fatal("decoder should error on empty bodyText; got nil")
	}
	// Sanity: a syntactically valid empty object should still decode cleanly.
	if err := dec(`{}`); err != nil {
		t.Fatalf("decoder should accept {}; got %v", err)
	}
}

// readSnapshotFixture writes a wire snapshot and a minimal openapi/fixture
// so that readSnapshot has a runner to call against.
func readSnapshotFixture(t *testing.T, testName, snapshotBody string) *ReplayRunner {
	t.Helper()
	dir := t.TempDir()
	openapi := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(openapi, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(dir, "fixture.json")
	if err := os.WriteFile(fixture, []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}
	wireDir := filepath.Join(dir, "replay", "bc4", "wire")
	if err := os.MkdirAll(wireDir, 0o755); err != nil {
		t.Fatal(err)
	}
	snapPath := filepath.Join(wireDir, safeName(testName)+".json")
	if err := os.WriteFile(snapPath, []byte(snapshotBody), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := NewReplayRunner(filepath.Join(dir, "replay"), "bc4", fixture, openapi)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestReadSnapshot_RejectsMissingPages(t *testing.T) {
	// Pre-fix: `{"operation":"GetProject"}` unmarshaled with Pages == nil
	// and the per-page loop ran zero times — silent green-pass with no
	// decode attempted. Post-fix: readSnapshot returns an error.
	r := readSnapshotFixture(t, "Test", `{"operation":"GetProject"}`)
	if _, err := r.readSnapshot("Test"); err == nil {
		t.Fatal("readSnapshot should error on missing pages; got nil")
	}
}

func TestReadSnapshot_RejectsEmptyPages(t *testing.T) {
	r := readSnapshotFixture(t, "Test", `{"operation":"GetProject","pages":[],"pages_count":0}`)
	if _, err := r.readSnapshot("Test"); err == nil {
		t.Fatal("readSnapshot should error on empty pages; got nil")
	}
}

func TestReadSnapshot_RejectsMismatchedPagesCount(t *testing.T) {
	r := readSnapshotFixture(t, "Test",
		`{"operation":"GetProject","pages":[{"status":200,"bodyText":"{}"}],"pages_count":2}`)
	if _, err := r.readSnapshot("Test"); err == nil {
		t.Fatal("readSnapshot should error on mismatched pages_count; got nil")
	}
}

func TestReadSnapshot_AcceptsMatchingPagesCount(t *testing.T) {
	r := readSnapshotFixture(t, "Test",
		`{"operation":"GetProject","pages":[{"status":200,"bodyText":"{}"}],"pages_count":1}`)
	if _, err := r.readSnapshot("Test"); err != nil {
		t.Fatalf("readSnapshot should accept matching pages_count; got %v", err)
	}
}

func TestReadSnapshot_AcceptsEmptySkipMarker(t *testing.T) {
	// A skip marker (live test skipped before wire capture — e.g. an unset
	// env-var-only fixture ID) legitimately has zero pages. Without the
	// skipped branch this would be rejected by the empty-pages guard.
	r := readSnapshotFixture(t, "Test",
		`{"operation":"GetCalendar","skipped":true,"skip_reason":"Fixture ID for ${CALENDAR_ID} not available","pages":[],"pages_count":0}`)
	snap, err := r.readSnapshot("Test")
	if err != nil {
		t.Fatalf("readSnapshot should accept an empty skip marker; got %v", err)
	}
	if !snap.Skipped || snap.SkipReason == "" {
		t.Fatalf("skip marker fields should round-trip; got skipped=%v reason=%q", snap.Skipped, snap.SkipReason)
	}
}

func TestReadSnapshot_RejectsSkipMarkerWithPages(t *testing.T) {
	// A skipped marker carrying pages means the TS runner's contract
	// drifted — refuse it rather than silently ignoring captured data.
	r := readSnapshotFixture(t, "Test",
		`{"operation":"GetCalendar","skipped":true,"pages":[{"status":200,"bodyText":"{}"}],"pages_count":1}`)
	if _, err := r.readSnapshot("Test"); err == nil {
		t.Fatal("readSnapshot should reject a skip marker that carries pages; got nil")
	}
}
