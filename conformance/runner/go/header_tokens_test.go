package main

import (
	"strings"
	"testing"
	"time"
)

// A quarter-second into 10:18:14, so the floor and the round-up land on
// different seconds and a resolver that rounded would show it.
var tokenNow = time.Date(2021, time.June, 9, 10, 18, 14, 250_000_000, time.UTC)

func TestResolveHeaderValue_PassesPlainValuesThrough(t *testing.T) {
	for _, v := range []string{"", "2", "Wed, 09 Jun 2021 10:18:14 GMT", "application/json", "{not a token}"} {
		got, err := resolveHeaderValue(v, tokenNow)
		if err != nil || got != v {
			t.Errorf("resolveHeaderValue(%q) = %q, %v; want the value unchanged", v, got, err)
		}
	}
}

func TestResolveHeaderValue_ResolvesHttpdateToTheWholeSecondPastN(t *testing.T) {
	cases := map[string]string{
		"{{httpdate+2s}}":  "Wed, 09 Jun 2021 10:18:17 GMT",
		"{{httpdate+0s}}":  "Wed, 09 Jun 2021 10:18:15 GMT",
		"{{httpdate+10s}}": "Wed, 09 Jun 2021 10:18:25 GMT",
	}
	for token, want := range cases {
		got, err := resolveHeaderValue(token, tokenNow)
		if err != nil {
			t.Fatalf("resolveHeaderValue(%q): %v", token, err)
		}
		if got != want {
			t.Errorf("resolveHeaderValue(%q) = %q, want %q (floor(now) + N + 1, IMF-fixdate)", token, got, want)
		}
	}
}

func TestResolveHeaderValue_RejectsAnUnknownToken(t *testing.T) {
	for _, v := range []string{"{{httpdate}}", "{{httpdate+2}}", "{{httpdate-2s}}", "{{now}}", "{{}}", "{{httpdate+1000000000s}}"} {
		got, err := resolveHeaderValue(v, tokenNow)
		if err == nil {
			t.Errorf("resolveHeaderValue(%q) = %q, want an error — an unknown token must never be served literally", v, got)
			continue
		}
		if !strings.Contains(err.Error(), v) {
			t.Errorf("error for %q does not name the token: %v", v, err)
		}
	}
}
