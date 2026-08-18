package eventfeed

import (
	"strings"
	"testing"
)

func TestCheckCableURL_Accepts(t *testing.T) {
	for _, u := range []string{
		"wss://28.cable.basecamp.com/cable?ticket=t-1",
		"WSS://28.CABLE.BASECAMP.COM/cable", // schemes and hosts are case-insensitive
		"ws://localhost:28080/cable",
		"ws://127.0.0.1:3000/cable?ticket=t-1",
		"ws://[::1]:28080/cable",
		"ws://app.localhost/cable",
		"ws://LOCALHOST/cable",
	} {
		if err := checkCableURL(u); err != nil {
			t.Errorf("checkCableURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestCheckCableURL_RefusesAsPolicy(t *testing.T) {
	// Every refusal is DialPolicy → Terminal(invalid_cable_url): a fresh
	// mint returns the same unusable URL, so the Backoff path never applies.
	cases := []struct {
		name string
		url  string
	}{
		{"ws outside localhost", "ws://28.cable.basecamp.com/cable"},
		{"ws on a localhost lookalike", "ws://localhost.evil.example/cable"},
		{"https scheme", "https://3.basecampapi.com/cable"},
		{"http scheme even on localhost", "http://localhost/cable"},
		{"relative URL (no scheme)", "3.basecampapi.com/cable"},
		{"unparseable", "wss://bad host/cable"},
		{"control character", "wss://h\x00st/cable"},
		{"empty host", "wss:///cable"},
		{"empty", ""},
		// A port-only or userinfo-only authority parses with a NONEMPTY
		// url.Host (":443", "user@") and an EMPTY hostname. The dial can only
		// fail, and it fails as an ordinary transient — so the connector would
		// re-mint and retry a permanently unusable URL forever instead of
		// surfacing invalid_cable_url. Authority is not hostname.
		{"port-only authority", "wss://:443/feed?ticket=t-1"},
		{"port-only authority on ws", "ws://:28080/cable"},
		{"userinfo-only authority", "wss://user:pass@/cable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkCableURL(tc.url)
			if err == nil {
				t.Fatalf("checkCableURL(%q) = nil, want policy refusal", tc.url)
			}
			if err.Kind != DialPolicy {
				t.Errorf("Kind = %v, want %v", err.Kind, DialPolicy)
			}
			if err.Reason == "" {
				t.Error("Reason is empty, want a named violation")
			}
		})
	}
}

func TestCheckCableURL_RefusalNeverCarriesQueryString(t *testing.T) {
	// §23 Security Invariants: the ticket rides in the mint URL's query
	// string; no rendering of a refusal may carry it.
	for _, u := range []string{
		"ws://3.basecampapi.com/cable?ticket=SECRET-TICKET",
		"wss://bad host/cable?ticket=SECRET-TICKET", // url.Parse error paths embed the raw URL
		"https://example.com/cable?ticket=SECRET-TICKET",
		"wss://:443/cable?ticket=SECRET-TICKET",
	} {
		err := checkCableURL(u)
		if err == nil {
			t.Fatalf("checkCableURL(%q) = nil, want refusal", u)
		}
		if rendered := err.Error(); strings.Contains(rendered, "SECRET-TICKET") {
			t.Errorf("refusal rendering leaks the query string: %q", rendered)
		}
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost":         true,
		"LocalHost":         true,
		"127.0.0.1":         true,
		"::1":               true,
		"app.localhost":     true,
		"sub.app.localhost": true,
		"localhost.evil":    false,
		"127.0.0.2":         false, // §9 lists 127.0.0.1 exactly
		"3.basecampapi.com": false,
		"":                  false,
	} {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

// TestCheckCableURL_RefusesOutOfRangePorts is the port-range half of the
// permanently-unusable-URL policy. url.Parse checks only that an explicit
// port is digits, so "wss://h:99999/" parses, carries a hostname, and clears
// every other check — then fails in the network stack, which
// WebSocketTransport.Dial classifies DialTransient. That sends the connector
// round the reconnect cycle re-minting and re-dialing forever, which is the
// exact failure the port-only-authority cases above were added to prevent.
func TestCheckCableURL_RefusesOutOfRangePorts(t *testing.T) {
	for _, u := range []string{
		"wss://cable.example.com:99999/feed?ticket=t-1",
		"wss://cable.example.com:65536/feed",
		"wss://cable.example.com:0/feed",
		"ws://localhost:70000/cable",
	} {
		err := checkCableURL(u)
		if err == nil {
			t.Fatalf("checkCableURL(%q) = nil, want policy refusal — an unusable port is not transient", u)
		}
		if err.Kind != DialPolicy {
			t.Errorf("checkCableURL(%q) Kind = %v, want %v", u, err.Kind, DialPolicy)
		}
		if strings.Contains(err.Reason, "ticket") {
			t.Errorf("checkCableURL(%q) Reason %q leaks the query string", u, err.Reason)
		}
	}
	// The boundary values stay dialable.
	for _, u := range []string{"wss://cable.example.com:1/feed", "wss://cable.example.com:65535/feed"} {
		if err := checkCableURL(u); err != nil {
			t.Errorf("checkCableURL(%q) = %v, want nil", u, err)
		}
	}
}
