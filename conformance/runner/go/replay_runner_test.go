// Regression test for the empty-bodyText decode-masking bug.
//
// Pre-fix, BodyText was a `string` so encoding/json zero-fills a missing
// key with "". The decode path then conflated "" (missing) with "" (empty
// HTTP body) and re-serialized `body` instead, silently green-passing an
// actually-empty wire payload. Post-fix, BodyText is `*string` and
// resolveBodyText distinguishes nil (missing) from &"" (empty), letting
// the decoder fail honestly on an empty body.

package main

import (
	"encoding/json"
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
		var se *json.SyntaxError
		_ = se
		t.Fatalf("decoder should accept {}; got %v", err)
	}
}
