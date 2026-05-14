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
	"os"
	"path/filepath"
	"testing"
)

func strPtr(s string) *string { return &s }

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
