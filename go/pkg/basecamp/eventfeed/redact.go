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
// it, the handler receives it — but the copy handed to an observer renders
// none of the server's bytes at all. The redactor's output is a closed set:
// the empty string, the caller's own configured origin, or one of two fixed
// placeholders. An earlier design rendered the URL's canonical origin, on the
// theory that an origin cannot carry a secret. It can: the ticket is opaque,
// so any server-controlled component can be its value — an all-digit ticket
// in the port ("wss://cable.example:54321/cable?ticket=54321"), a
// ticket-shaped host label — and §23's "never log the ticket" binds on the
// value, which no component position can discharge. What an operator needs
// from an observer trace is whether the URL was the configured origin; the
// server's own spelling of somewhere else is exactly what must not be echoed.

// redactedURL is what redactURL yields for input it cannot reduce to an
// origin. It is deliberately not a URL: a placeholder that still parsed would
// invite a consumer to follow, retry, or compare it, and an unparseable input
// is exactly the case where nothing about it should be trusted.
const redactedURL = "[redacted]"

// crossOriginURL is what redactURL yields for a URL whose origin is not the
// configured base origin. The only fact rendered is the one that matters —
// the URL pointed somewhere else; naming where would echo server-chosen
// text. Like redactedURL, it deliberately does not parse as a URL. The cable
// url always renders this way: it is cross-origin by design, and its query
// is the ticket.
const crossOriginURL = "[cross-origin URL]"

// redactURL renders raw for an observer, against the connector's configured
// baseOrigin (CanonicalOrigin form, like checkContinuation's): the caller's
// own origin when raw is same-origin with it, crossOriginURL when it points
// anywhere else, redactedURL when it cannot be reduced to an origin at all.
//
// An empty string stays empty rather than becoming a placeholder: "" is not
// a URL that failed to parse, it is the documented absence of one — a 410
// that carried no resume URL — and collapsing the two would tell an observer
// a URL was withheld when none existed.
//
// The failure direction is "less diagnostic", never "leaks": no branch
// echoes the input, and the same-origin branch renders the PARAMETER's
// bytes, not the input's — the two compare equal there, but rendering the
// configured value keeps the closed output set closed by construction.
func redactURL(baseOrigin, raw string) string {
	if raw == "" {
		return ""
	}
	origin, err := CanonicalOrigin(raw)
	if err != nil || origin == "" {
		return redactedURL
	}
	if origin != baseOrigin {
		return crossOriginURL
	}
	return baseOrigin
}

// redactCursor returns cursor with its PageURL reduced for an observer,
// against the same configured baseOrigin as redactURL. The other fields are
// connector-generated — a position or a since= value, both server-issued
// opaque ids rather than URLs — so they pass through.
func redactCursor(baseOrigin string, cursor Cursor) Cursor {
	cursor.PageURL = redactURL(baseOrigin, cursor.PageURL)
	return cursor
}
