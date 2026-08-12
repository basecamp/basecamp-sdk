package eventfeed

import (
	"net/url"
	"strings"
)

// Cable-URL policy (SPEC.md §23 "Security Invariants"): the connector
// pre-checks what it can before handing the mint's URL to
// CableTransport.Dial — parse and scheme; only the transport can see a
// redirect. The CableTransport, CableConn, and DialError seam types live in
// seams.go; the default WebSocket transport implementation lands separately
// so the dependency decision stays one revertible diff.

// checkCableURL applies the cable-URL policy to a mint's url before any
// dial: it must parse, carry a host, and use wss:// — ws:// is allowed only
// for localhost/loopback (the §9 carve-out). It returns nil when the URL may
// be dialed, else a policy-kind *DialError → Terminal(invalid_cable_url),
// never Backoff: the violation recurs by construction, because a fresh mint
// returns the same unusable URL. The returned error never carries the URL or
// its query string — the ticket rides in it.
func checkCableURL(wsURL string) *DialError {
	u, err := url.Parse(wsURL)
	if err != nil {
		// Deliberately NOT attached as Err: url.Parse errors embed the raw
		// URL, query string included.
		return &DialError{Kind: DialPolicy, Reason: "unparseable cable URL"}
	}
	switch scheme := strings.ToLower(u.Scheme); scheme {
	case "wss":
	case "ws":
		if !isLoopbackHost(u.Hostname()) {
			return &DialError{Kind: DialPolicy, Reason: "cable URL must use wss:// outside localhost"}
		}
	default:
		// The scheme is structural, never secret — safe to name in the
		// reason.
		return &DialError{Kind: DialPolicy, Reason: "cable URL scheme " + `"` + scheme + `"` + " is not ws(s)"}
	}
	// Hostname(), not Host: the authority can be nonempty while naming no host
	// at all — "wss://:443/feed" parses with Host ":443" and "wss://user@/feed"
	// with Host "user@". Neither is dialable, and neither failure is
	// transient, so accepting them here would send the connector round the
	// reconnect cycle re-minting and re-dialing a permanently unusable URL
	// instead of surfacing Terminal(invalid_cable_url). The ws:// branch above
	// already reads Hostname(), as does CanonicalOrigin behind the
	// continuation check.
	if u.Hostname() == "" {
		return &DialError{Kind: DialPolicy, Reason: "cable URL has no host"}
	}
	return nil
}

// isLoopbackHost reports whether host (a url.Hostname() value: lowercased
// comparison, port and brackets already stripped) is localhost-class per the
// SPEC.md §9 carve-out: localhost, 127.0.0.1, ::1, and *.localhost
// subdomains (RFC 6761).
func isLoopbackHost(host string) bool {
	host = strings.ToLower(host)
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	return strings.HasSuffix(host, ".localhost")
}
