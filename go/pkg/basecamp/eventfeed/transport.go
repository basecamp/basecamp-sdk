package eventfeed

import (
	"net/url"
	"strconv"
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
// returns the same unusable URL. The returned error never carries the URL,
// its query string, or any raw component of it: reasons are a closed
// vocabulary, and the one diagnostic kept — the http(s)-class mis-mint — is
// named from a fixed allowlist, never echoed.
func checkCableURL(wsURL string) *DialError {
	u, err := url.Parse(wsURL)
	if err != nil {
		// Deliberately NOT attached as Err: url.Parse errors embed the raw
		// URL, query string included.
		return &DialError{Kind: DialPolicy, Reason: "unparseable cable URL"}
	}
	// Userinfo is refused outright, before anything else looks at the URL.
	// net/http's send() turns URL userinfo into a Basic Authorization header
	// (`if u := req.URL.User; u != nil && req.Header.Get("Authorization") ==
	// ""`), so a mint whose url carried userinfo would make the connector
	// AUTHENTICATE to the cable origin with a credential the server chose —
	// and that origin is cross-host by design. There is no legitimate reading
	// of userinfo on a cable URL: the ticket is the credential, and it rides
	// in the query.
	//
	// Neither the username nor the password is named in the reason: the
	// password is obviously secret, and the username is attacker-controlled
	// text that would otherwise be echoed into an observer's log.
	if u.User != nil {
		return &DialError{Kind: DialPolicy, Reason: "cable URL carries userinfo"}
	}
	// A fragment is never part of a request target — the WebSocket handshake
	// strips it — so a mint that put ticket or routing data there composed a
	// URL the dial cannot honor, and dialing the stripped remainder would
	// silently connect to something other than what the mint named.
	// Permanent, like every policy case: a fresh mint returns the same URL.
	// The reason is value-free; a fragment is server text and can carry the
	// ticket.
	if u.Fragment != "" {
		return &DialError{Kind: DialPolicy, Reason: "cable URL carries a fragment"}
	}
	switch scheme := strings.ToLower(u.Scheme); scheme {
	case "wss":
	case "ws":
		if !isLoopbackHost(u.Hostname()) {
			return &DialError{Kind: DialPolicy, Reason: "cable URL must use wss:// outside localhost"}
		}
	default:
		// The scheme is never echoed. It is server-controlled text, and the
		// ticket is opaque, so no position in the URL is guaranteed not to
		// BE the ticket: a mint returning "t-abc://host/cable?ticket=t-abc"
		// makes the scheme literally the credential §23's Security
		// Invariants say is never logged — an invariant on the value, which
		// "the scheme is structural" cannot discharge. The one realistic
		// mis-mint — an http(s) URL where ws(s) belonged — keeps its
		// diagnostic, named from this fixed allowlist; everything else is
		// named by class alone.
		if scheme == "http" || scheme == "https" {
			return &DialError{Kind: DialPolicy, Reason: "mint returned an http(s) URL where ws(s) was required"}
		}
		return &DialError{Kind: DialPolicy, Reason: "cable URL scheme is not ws(s)"}
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
	// An EXPLICIT port must be a usable TCP port. url.Parse checks only that
	// the port is digits, so "wss://example.com:99999/feed" parses, carries a
	// hostname, and passes everything above — then fails in the network
	// stack, which WebSocketTransport.Dial classifies DialTransient. That
	// sends the connector round the reconnect cycle re-minting and re-dialing
	// forever, which is the same permanently-unusable-URL failure the
	// hostname check just above exists to convert into
	// Terminal(invalid_cable_url). Port 0 is included: it names no port a
	// client can connect to.
	if port := u.Port(); port != "" {
		n, perr := strconv.Atoi(port)
		if perr != nil || n < 1 || n > 65535 {
			// Value-free, like the scheme above: every port reaching this
			// branch is out of range or overlong, so the class is the whole
			// diagnostic — and an opaque all-digit ticket could otherwise be
			// echoed ("wss://host:99999/cable?ticket=99999").
			return &DialError{Kind: DialPolicy, Reason: "cable URL explicit port is not a usable TCP port"}
		}
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
