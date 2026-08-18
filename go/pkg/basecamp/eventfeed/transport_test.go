package eventfeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

// TestCableProxy_NeverProxiesCleartext is the rule behind
// TestWebSocketTransport_CleartextDialNeverReachesAProxy, asserted directly so
// it holds independently of whether any particular dial happens to be
// attempted. Both arms matter: refusing to proxy cleartext is the fix, and
// continuing to proxy TLS is what keeps it from being "disable proxies
// wholesale", which would break every deployment behind one.
func TestCableProxy_NeverProxiesCleartext(t *testing.T) {
	sentinel := &url.URL{Scheme: "http", Host: "proxy.internal:3128"}
	prev := proxyFromEnvironment
	proxyFromEnvironment = func(*http.Request) (*url.URL, error) { return sentinel, nil }
	t.Cleanup(func() { proxyFromEnvironment = prev })

	// A ws:// handshake is an http:// request by the time it reaches the
	// RoundTripper, and it is forwarded in absolute form — request line,
	// ticket and all — so it must never be proxied.
	for _, raw := range []string{
		"http://app.localhost:3000/cable?ticket=sekrit",
		"http://28.cable.basecamp.com/cable?ticket=sekrit",
	} {
		req := &http.Request{URL: mustParseURL(t, raw)}
		got, err := cableProxy(req)
		if err != nil {
			t.Errorf("cableProxy(%q) error = %v", raw, err)
		}
		if got != nil {
			t.Errorf("cableProxy(%q) = %v, want nil — cleartext must never be proxied", raw, got)
		}
	}

	// A wss:// handshake reaches the proxy as CONNECT host:port, so the
	// ticket stays inside the tunnel and the proxy is honoured.
	req := &http.Request{URL: mustParseURL(t, "https://28.cable.basecamp.com/cable?ticket=sekrit")}
	got, err := cableProxy(req)
	if err != nil {
		t.Fatalf("cableProxy(https) error = %v", err)
	}
	if got != sentinel {
		t.Errorf("cableProxy(https) = %v, want the environment's proxy %v", got, sentinel)
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) = %v", raw, err)
	}
	return u
}

// TestCableDial_CleartextNeverReachesAProxy is the egress proof behind the
// rule above: not "cableProxy returns nil" but "nothing arrived at the
// proxy". The sentinel stands in for a deployment's HTTP_PROXY. It must record
// nothing, and the assertion holds whether or not the dial itself succeeds —
// it does not, since nothing is listening on the target.
//
// The resolver is swapped rather than the environment because
// http.ProxyFromEnvironment caches HTTP_PROXY behind a package-wide sync.Once:
// an env-based version of this test passes without exercising anything the
// moment some earlier test in the binary has resolved a proxy, which is the
// definition of a test that goes green for the wrong reason.
func TestCableDial_CleartextNeverReachesAProxy(t *testing.T) {
	var hits atomic.Int64
	var mu sync.Mutex
	var lines []string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		mu.Lock()
		lines = append(lines, r.Method+" "+r.RequestURI)
		mu.Unlock()
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer proxy.Close()

	proxyURL := mustParseURL(t, proxy.URL)
	prev := proxyFromEnvironment
	proxyFromEnvironment = func(*http.Request) (*url.URL, error) { return proxyURL, nil }
	t.Cleanup(func() { proxyFromEnvironment = prev })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A *.localhost host, which is exactly the gap: net/http's proxy rules
	// exempt the literal "localhost" and loopback IPs, but not subdomains of
	// .localhost, and the SPEC §9 carve-out admits ws:// for all of them.
	conn, err := (&WebSocketTransport{}).Dial(ctx, "ws://app.localhost:9/cable?ticket=sekrit-ticket-value", 1<<20)
	if err == nil {
		_ = conn.Close(1000, "")
		t.Fatal("dial to a discard port succeeded; the test proves nothing")
	}

	if n := hits.Load(); n != 0 {
		mu.Lock()
		got := slices.Clone(lines)
		mu.Unlock()
		t.Errorf("a cleartext cable dial reached the proxy %d time(s): %q", n, got)
	}
}

// TestCableHTTPClient_IsWiredShut pins what the behavior tests structurally
// cannot see.
//
// The two tests above route their sentinel through proxyFromEnvironment, so
// they detect a regression only while cableProxy is still the installed Proxy
// function. A change to `Proxy: http.ProxyFromEnvironment` would restore the
// exact hazard and pass both of them — that gap is not hypothetical, it is how
// the first mutant written against this fix survived. Likewise a Jar or a
// TLSClientConfig added later is a credential the origin would receive; the
// DefaultClient-pollution test proves only that none is INHERITED, not that
// none is set here.
//
// So this holds the shape of the wiring, which is what a static assertion is
// good for, alongside the behavior tests that hold the rule.
func TestCableHTTPClient_IsWiredShut(t *testing.T) {
	if cableHTTPClient.Jar != nil {
		t.Error("cableHTTPClient has a cookie jar; the cable origin must receive no cookie")
	}
	if cableHTTPClient.CheckRedirect == nil {
		t.Error("cableHTTPClient does not refuse redirects; a redirect can carry the ticket to an unvetted origin")
	}
	tr, ok := cableHTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("cableHTTPClient.Transport = %T, want *http.Transport", cableHTTPClient.Transport)
	}
	if tr.TLSClientConfig != nil {
		t.Error("cableHTTPClient carries a TLSClientConfig; it must present no client certificate and use the system roots")
	}
	if tr == http.DefaultTransport {
		t.Fatal("cableHTTPClient uses http.DefaultTransport; any library in the process can have replaced it")
	}
	if got, want := reflect.ValueOf(tr.Proxy).Pointer(), reflect.ValueOf(cableProxy).Pointer(); got != want {
		t.Error("cableHTTPClient.Transport.Proxy is not cableProxy; cleartext dials would proxy the ticket")
	}
}
