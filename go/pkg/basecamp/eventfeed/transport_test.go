package eventfeed

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
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
		// A fragment is never part of a request target, so a mint that put
		// routing or ticket data there composed a URL the dial cannot honor
		// — permanently, like every policy case: a fresh mint returns the
		// same URL.
		{"fragment", "wss://28.cable.basecamp.com/cable#ticket=t-1"},
		{"bare fragment on ws localhost", "ws://localhost:28080/cable#x"},
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

// TestCheckCableURL_RejectsUserinfo pins the policy half of the credential
// boundary. net/http's send() turns URL userinfo into a Basic Authorization
// header, so a mint url carrying userinfo would make the connector
// authenticate to a server-nominated origin with a credential the server
// chose. The transport-level proof that this precedes all network I/O is
// TestWebSocketTransport_RejectsUserinfoBeforeAnyNetworkIO.
func TestCheckCableURL_RejectsUserinfo(t *testing.T) {
	for _, u := range []string{
		"wss://attacker:hunter2@28.cable.basecamp.com/cable?ticket=t-1",
		"wss://attacker@28.cable.basecamp.com/cable",
		"wss://attacker:@28.cable.basecamp.com/cable",
		"wss://:hunter2@28.cable.basecamp.com/cable",
		// The loopback carve-out does not exempt it.
		"ws://attacker:hunter2@localhost:28080/cable",
	} {
		derr := checkCableURL(u)
		if derr == nil {
			t.Errorf("checkCableURL(%q) = nil, want a policy refusal", u)
			continue
		}
		if derr.Kind != DialPolicy {
			t.Errorf("checkCableURL(%q).Kind = %v, want DialPolicy", u, derr.Kind)
		}
		// Neither half of the userinfo is echoed: the password is obviously
		// secret and the username is attacker-controlled text.
		for _, secret := range []string{"attacker", "hunter2", "t-1"} {
			if strings.Contains(derr.Error(), secret) {
				t.Errorf("checkCableURL(%q) leaked %q: %s", u, secret, derr)
			}
		}
	}
}

// TestCheckCableURL_NeverEchoesServerText pins the closed reason vocabulary.
// §23's "never log the ticket" binds on the VALUE, and the ticket is opaque,
// so any server-controlled URL component can be the ticket itself — a scheme
// spelled as the ticket, or an all-digit ticket in the port position. Neither
// may reach the rendering.
func TestCheckCableURL_NeverEchoesServerText(t *testing.T) {
	for _, tc := range []struct{ name, url, secret string }{
		{"ticket as scheme", "t-sekrit-99://28.cable.basecamp.com/cable?ticket=t-sekrit-99", "sekrit"},
		{"numeric ticket as port", "wss://28.cable.basecamp.com:987654321/cable?ticket=987654321", "987654321"},
		{"ticket in the fragment", "wss://28.cable.basecamp.com/cable#ticket=sekrit-fragment", "sekrit-fragment"},
	} {
		derr := checkCableURL(tc.url)
		if derr == nil {
			t.Fatalf("%s: checkCableURL(%q) = nil, want policy refusal", tc.name, tc.url)
		}
		if derr.Kind != DialPolicy {
			t.Errorf("%s: Kind = %v, want DialPolicy", tc.name, derr.Kind)
		}
		if strings.Contains(derr.Error(), tc.secret) {
			t.Errorf("%s: rendering %q echoes %q", tc.name, derr.Error(), tc.secret)
		}
	}
}

// TestDialFailure_RendersOnlyStandardStatuses pins the closed set behind the
// dial rendering's one interpolation. net/http parses any three-character
// Atoi-parseable status line (response.go: len==3, >=0), so a hostile server
// can answer 999 or a zero-padded 007 — integers with no HTTP meaning. Only
// the semantically defined 100-599 renders; everything else collapses to the
// fixed digit-free marker, so the rendering cannot carry a number the server
// chose freely.
func TestDialFailure_RendersOnlyStandardStatuses(t *testing.T) {
	base := errors.New("boom")
	for _, code := range []int{100, 403, 599} {
		got := dialFailure(base, code).Error()
		if !strings.Contains(got, fmt.Sprintf("HTTP %d", code)) {
			t.Errorf("in-range status %d not rendered: %q", code, got)
		}
	}
	for _, code := range []int{7, 42, 600, 999} {
		got := dialFailure(base, code).Error()
		if strings.Contains(got, strconv.Itoa(code)) {
			t.Errorf("out-of-range status %d rendered: %q", code, got)
		}
		if !strings.Contains(got, "outside the standard range") {
			t.Errorf("out-of-range status %d: %q, want the fixed marker", code, got)
		}
	}
	if got := dialFailure(base, 0).Error(); strings.Contains(got, "answered") {
		t.Errorf("no response, but the rendering claims one: %q", got)
	}
}

// TestWebSocketTransport_RedirectWithMalformedLocationIsPolicy pins that a
// redirect is refused on its status alone: the server-controlled Location is
// never parsed, so a malformed one — which net/http's own redirect machinery
// would have choked on before any classification could see the response — is
// the same policy refusal as a well-formed one.
func TestWebSocketTransport_RedirectWithMalformedLocationIsPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "%zz") // an invalid URL escape: url.Parse refuses it
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	_, err := (&WebSocketTransport{}).Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/cable?ticket=sekrit-ticket-value", 1<<20)
	if err == nil {
		t.Fatal("dial succeeded against a redirect, want a policy refusal")
	}
	var derr *DialError
	if !errors.As(err, &derr) || derr.Kind != DialPolicy {
		t.Fatalf("dial error = %v, want DialPolicy — a redirect is permanent whatever its Location", err)
	}
	for _, leaked := range []string{"sekrit", "%zz"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("dial error %q carries %q", err, leaked)
		}
	}
}

// TestWebSocketTransport_RedirectWithoutLocationIsPolicy: a 3xx with no
// Location never invokes CheckRedirect — net/http returns the response as a
// normal answer — so the sentinel path cannot see it, and the fallback
// classified it DialTransient: an endless re-mint cycle against an endpoint
// that will redirect forever. The status itself names the class.
func TestWebSocketTransport_RedirectWithoutLocationIsPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusFound) // 302, deliberately no Location
	}))
	defer srv.Close()
	_, err := (&WebSocketTransport{}).Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/cable?ticket=sekrit-ticket-value", 1<<20)
	if err == nil {
		t.Fatal("dial succeeded against a bare 302, want a policy refusal")
	}
	var derr *DialError
	if !errors.As(err, &derr) || derr.Kind != DialPolicy {
		t.Fatalf("dial error = %v (kind %v), want DialPolicy — a redirect is permanent, with or without a Location", err, derr.Kind)
	}
	if strings.Contains(err.Error(), "sekrit") {
		t.Errorf("dial error %q carries the ticket", err)
	}
}

// TestWebSocketTransport_UnofferedSubprotocolIsPolicy: a 101 whose
// Sec-WebSocket-Protocol names something the dial never offered is a server
// that deterministically selects a bogus protocol, so re-minting against it
// cannot help. The classification is structural: a selected protocol that is
// not the offer. The peer-controlled header value itself is never rendered.
func TestWebSocketTransport_UnofferedSubprotocolIsPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A hand-rolled upgrade: a correct Sec-WebSocket-Accept (so the
		// verification reaches the subprotocol check) selecting a protocol
		// the client never offered.
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("test server cannot hijack")
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(buf, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\nSec-WebSocket-Protocol: bogus-protocol-sekrit\r\n\r\n",
			base64.StdEncoding.EncodeToString(sum[:]))
		_ = buf.Flush()
	}))
	defer srv.Close()
	_, err := (&WebSocketTransport{}).Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/cable?ticket=sekrit-ticket-value", 1<<20)
	if err == nil {
		t.Fatal("dial succeeded against an unoffered subprotocol, want a policy refusal")
	}
	var derr *DialError
	if !errors.As(err, &derr) || derr.Kind != DialPolicy {
		t.Fatalf("dial error = %v, want DialPolicy — a server selecting a bogus protocol does so deterministically", err)
	}
	for _, leaked := range []string{"sekrit", "bogus-protocol"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("dial error %q carries %q", err, leaked)
		}
	}
}
