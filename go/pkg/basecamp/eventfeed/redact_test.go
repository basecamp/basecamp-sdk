package eventfeed

import (
	"testing"
)

// The credential this protects is the reason for every case below: a `next`
// continuation and a 410 `resume` URL are followed with the caller's bearer,
// and the mint's cable url carries the ticket in its query string. Each is
// attacker-influenced — all three arrive in a server response — and the
// ticket is opaque, so any server-controlled component of a URL can BE the
// ticket's value. The redactor therefore renders no byte of its input:
// same-origin input renders the caller's configured origin, everything else
// a fixed placeholder.
func TestRedactURL_RendersOnlyConfiguredTextOrPlaceholders(t *testing.T) {
	const base = "https://3.basecampapi.com"
	for _, tc := range []struct {
		name string
		base string
		raw  string
		want string
	}{
		{
			"same-origin: query dropped",
			base,
			"https://3.basecampapi.com/5951425/events.json?since=now&ticket=sekrit",
			base,
		},
		{
			"same-origin: path dropped",
			base,
			"https://3.basecampapi.com/5951425/buckets/2/events.json",
			base,
		},
		{
			"same-origin: fragment dropped",
			base,
			"https://3.basecampapi.com/events.json#sekrit",
			base,
		},
		{
			// Userinfo is a credential in the authority, not the path; it
			// neither renders nor breaks the origin comparison
			// (CanonicalOrigin reads Hostname()).
			"same-origin: userinfo dropped",
			base,
			"https://user:hunter2@3.basecampapi.com/events.json",
			base,
		},
		{
			"same-origin: default port dropped",
			base,
			"https://3.basecampapi.com:443/events.json?ticket=sekrit",
			base,
		},
		{
			// The rendered port is the CONFIGURED one: it appears because
			// the operator wrote it, not because the server sent it.
			"same-origin: configured non-default port",
			"https://bc.internal.example:8443",
			"https://bc.internal.example:8443/events.json?ticket=sekrit",
			"https://bc.internal.example:8443",
		},
		{
			"same-origin: scheme and host case-insensitive",
			base,
			"HTTPS://3.BasecampAPI.com/events.json",
			base,
		},
		{
			"cross-origin host is not named",
			base,
			"https://attacker:hunter2@evil.example/steal?ticket=sekrit",
			crossOriginURL,
		},
		{
			// The cable URL is cross-origin by design; its host, port, and
			// scheme are all server text, so it always renders the
			// placeholder.
			"cable url",
			base,
			"wss://28.cable.basecamp.com/cable?ticket=t-abc123",
			crossOriginURL,
		},
		{
			// A same-host cable URL still differs in scheme: cross-origin.
			"same host, ws scheme",
			base,
			"wss://3.basecampapi.com/cable?ticket=t-abc123",
			crossOriginURL,
		},
		{
			// Absence, not failure. A 410 may legitimately carry no resume URL.
			"empty stays empty",
			base,
			"",
			"",
		},
		{
			"relative URL cannot be reduced",
			base,
			"/5951425/events.json?ticket=sekrit",
			redactedURL,
		},
		{
			"scheme with no host cannot be reduced",
			base,
			"https://?ticket=sekrit",
			redactedURL,
		},
		{
			"unparseable cannot be reduced",
			base,
			"ht!tp://\x7f/x?ticket=sekrit",
			redactedURL,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactURL(tc.base, tc.raw); got != tc.want {
				t.Errorf("redactURL(%q, %q) = %q, want %q", tc.base, tc.raw, got, tc.want)
			}
		})
	}
}

// The property that matters more than any single case: whatever the input,
// the output is drawn from the closed set {"", baseOrigin, crossOriginURL,
// redactedURL} — no server byte can flow through, so no component that
// happens to BE the ticket can either. The port and host-label cases are the
// counterexamples that retired the origin-rendering design:
// "wss://cable.example:54321/cable?ticket=54321" put the ticket in the
// rendered origin's port.
func TestRedactURL_OutputIsAClosedSet(t *testing.T) {
	const base = "https://3.basecampapi.com"
	const secret = "sekrit-ticket-value"
	for _, raw := range []string{
		"https://3.basecampapi.com/events.json?ticket=" + secret,
		"https://3.basecampapi.com/events.json?a=1&b=" + secret + "&c=3",
		"https://3.basecampapi.com/" + secret + "/events.json",
		"https://3.basecampapi.com/events.json#" + secret,
		"https://user:" + secret + "@3.basecampapi.com/events.json",
		"https://" + secret + ":pw@3.basecampapi.com/events.json",
		"wss://28.cable.basecamp.com/cable?" + secret,
		"wss://cable.example:54321/cable?ticket=54321",
		"wss://" + secret + ".example/cable?ticket=" + secret,
		"https://3.basecampapi.com:54321/events.json?ticket=54321",
		"//3.basecampapi.com/events.json?ticket=" + secret,
		"not a url at all ?ticket=" + secret,
		"https://3.basecampapi.com/events.json?ticket=" + secret + "&next=https%3A%2F%2Fevil.example%2F" + secret,
	} {
		got := redactURL(base, raw)
		if got != "" && got != base && got != crossOriginURL && got != redactedURL {
			t.Errorf("redactURL(%q, %q) = %q, outside the closed output set", base, raw, got)
		}
	}
}

// Neither placeholder may be mistakable for a URL a consumer could act on.
func TestRedactPlaceholders_AreNotURLs(t *testing.T) {
	for _, placeholder := range []string{redactedURL, crossOriginURL} {
		if _, err := CanonicalOrigin(placeholder); err == nil {
			t.Errorf("placeholder %q reduces to an origin; it must not parse as a URL", placeholder)
		}
	}
}

func TestRedactCursor_ReducesOnlyThePageURL(t *testing.T) {
	const base = "https://3.basecampapi.com"
	got := redactCursor(base, Cursor{
		Position: "pos-99",
		Since:    "now",
		PageURL:  "https://3.basecampapi.com/5951425/events.json?page=2&ticket=sekrit",
	})
	if got.PageURL != base {
		t.Errorf("PageURL = %q, want the configured origin", got.PageURL)
	}
	// Position and Since are connector-generated server-issued ids, not URLs,
	// and an operator correlating an observer trace needs them intact.
	if got.Position != "pos-99" {
		t.Errorf("Position = %q, want it untouched", got.Position)
	}
	if got.Since != "now" {
		t.Errorf("Since = %q, want it untouched", got.Since)
	}
}
