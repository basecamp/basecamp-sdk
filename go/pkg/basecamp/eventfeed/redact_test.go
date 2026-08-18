package eventfeed

import (
	"strings"
	"testing"
)

// The credential this protects is the reason for every case below: a `next`
// continuation and a 410 `resume` URL are followed with the caller's bearer,
// and the mint's cable url carries the ticket in its query string. Each is
// attacker-influenced — all three arrive in a server response.
func TestRedactURL_KeepsOnlyTheOrigin(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{
			"query dropped",
			"https://3.basecampapi.com/5951425/events.json?since=now&ticket=sekrit",
			"https://3.basecampapi.com",
		},
		{
			"path dropped",
			"https://3.basecampapi.com/5951425/buckets/2/events.json",
			"https://3.basecampapi.com",
		},
		{
			"fragment dropped",
			"https://3.basecampapi.com/events.json#sekrit",
			"https://3.basecampapi.com",
		},
		{
			// The one that matters most for a hostile URL: userinfo is a
			// credential in the authority, not the path, so a redactor that
			// only stripped path and query would forward it.
			"userinfo dropped",
			"https://attacker:hunter2@evil.example/steal?ticket=sekrit",
			"https://evil.example",
		},
		{
			"default port dropped",
			"https://3.basecampapi.com:443/events.json?ticket=sekrit",
			"https://3.basecampapi.com",
		},
		{
			"non-default port kept: it names the host an operator must look at",
			"https://bc.internal.example:8443/events.json?ticket=sekrit",
			"https://bc.internal.example:8443",
		},
		{
			"scheme and host lowercased",
			"HTTPS://3.BasecampAPI.com/events.json",
			"https://3.basecampapi.com",
		},
		{
			// A cable URL, which is the third attacker-influenced URL and the
			// one whose whole query IS the credential.
			"cable url",
			"wss://28.cable.basecamp.com/cable?ticket=t-abc123",
			"wss://28.cable.basecamp.com",
		},
		{
			// Absence, not failure. A 410 may legitimately carry no resume URL.
			"empty stays empty",
			"",
			"",
		},
		{
			"relative URL cannot be reduced",
			"/5951425/events.json?ticket=sekrit",
			redactedURL,
		},
		{
			"scheme with no host cannot be reduced",
			"https://?ticket=sekrit",
			redactedURL,
		},
		{
			"unparseable cannot be reduced",
			"ht!tp://\x7f/x?ticket=sekrit",
			redactedURL,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactURL(tc.raw); got != tc.want {
				t.Errorf("redactURL(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// The property that matters more than any single case: whatever the input, the
// output never contains the secret-bearing parts. Stated separately because the
// table above pins exact strings and would be satisfied by a redactor that
// happened to produce them while still leaking on an input nobody listed.
func TestRedactURL_NeverEchoesQueryOrCredentials(t *testing.T) {
	const secret = "sekrit-ticket-value"
	for _, raw := range []string{
		"https://3.basecampapi.com/events.json?ticket=" + secret,
		"https://3.basecampapi.com/events.json?a=1&b=" + secret + "&c=3",
		"https://3.basecampapi.com/" + secret + "/events.json",
		"https://3.basecampapi.com/events.json#" + secret,
		"https://user:" + secret + "@3.basecampapi.com/events.json",
		"https://" + secret + ":pw@3.basecampapi.com/events.json",
		"wss://28.cable.basecamp.com/cable?" + secret,
		"//3.basecampapi.com/events.json?ticket=" + secret,
		"not a url at all ?ticket=" + secret,
		"https://3.basecampapi.com/events.json?ticket=" + secret + "&next=https%3A%2F%2Fevil.example%2F" + secret,
	} {
		got := redactURL(raw)
		if strings.Contains(got, secret) {
			t.Errorf("redactURL(%q) = %q, which echoes the credential", raw, got)
		}
		// Neither a query nor a fragment can survive at all: their presence is
		// the leak regardless of what they hold.
		if strings.ContainsAny(got, "?#") {
			t.Errorf("redactURL(%q) = %q, which retains a query or fragment", raw, got)
		}
	}
}

// redactedURL must not be mistakable for a URL a consumer could act on.
func TestRedactedURL_IsNotAURL(t *testing.T) {
	if _, err := CanonicalOrigin(redactedURL); err == nil {
		t.Errorf("redactedURL %q reduces to an origin; the placeholder must not parse as a URL", redactedURL)
	}
}

func TestRedactCursor_ReducesOnlyThePageURL(t *testing.T) {
	got := redactCursor(Cursor{
		Position: "pos-99",
		Since:    "now",
		PageURL:  "https://3.basecampapi.com/5951425/events.json?page=2&ticket=sekrit",
	})
	if got.PageURL != "https://3.basecampapi.com" {
		t.Errorf("PageURL = %q, want the origin only", got.PageURL)
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
