package eventfeed

// Observer-safe URL rendering (SPEC.md §23 "Security Invariants").
//
// Three of the connector's absolute URLs are attacker-influenced: the
// envelope's `next` continuation, a 410 body's `resume` URL, and the mint's
// cable url. Every one arrives in a server response, and every one carries a
// credential — the first two are followed with the caller's Authorization
// bearer, the third with the short-lived ticket in its own query string.
//
// Observer callbacks are the one place those URLs leave the connector for a
// destination it knows nothing about: a host's log, its error tracker, its
// metrics labels. The URL that DRIVES behavior stays whole — the walk follows
// it, the handler receives it — but the copy handed to an observer is reduced
// to its origin, which is the part an operator actually needs (which host is
// misbehaving) and the part that cannot carry a secret.

// redactedURL is what redactURL yields for input it cannot reduce to an
// origin. It is deliberately not a URL: a placeholder that still parsed would
// invite a consumer to follow, retry, or compare it, and an unparseable input
// is exactly the case where nothing about it should be trusted.
const redactedURL = "[redacted]"

// redactURL renders raw for an observer: its canonical origin — lowercased
// scheme and host, the default port dropped — and nothing else. Path, query,
// fragment and USERINFO are all discarded (CanonicalOrigin reads Hostname(),
// so a URL like "https://user:pw@evil.example/x?t=s" renders
// "https://evil.example").
//
// An empty string stays empty rather than becoming the placeholder: "" is not
// a URL that failed to parse, it is the documented absence of one — a 410 that
// carried no resume URL — and collapsing the two would tell an observer a URL
// was withheld when none existed.
//
// Anything else that will not reduce to an origin yields redactedURL. The
// failure direction is "less diagnostic", never "leaks": there is no branch
// that falls back to echoing the input.
func redactURL(raw string) string {
	if raw == "" {
		return ""
	}
	origin, err := CanonicalOrigin(raw)
	if err != nil || origin == "" {
		return redactedURL
	}
	return origin
}

// redactCursor returns cursor with its PageURL reduced for an observer. The
// other fields are connector-generated — a position or a since= value, both
// server-issued opaque ids rather than URLs — so they pass through.
func redactCursor(cursor Cursor) Cursor {
	cursor.PageURL = redactURL(cursor.PageURL)
	return cursor
}
