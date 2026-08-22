package eventfeed

import (
	"strings"
	"unicode/utf8"
)

// Continuation and resume URL validation (SPEC.md §23 "Continuation and
// Resume URL Validation"). The two absolute URLs the poll lane follows — the
// envelope's `next` continuation and a 410 body's `resume` URL — carry the
// caller's Authorization bearer when followed, so each is validated against
// the configured base origin BEFORE any poll call is made, under §8's
// Same-Origin Validation Algorithm plus downgrade rejection. The mint's cable
// url is deliberately exempt: it is server-directed topology, cross-host by
// design, dialed with the short-lived ticket as its own credential and
// governed by the cable-URL policy in transport.go instead.

// checkContinuation validates a continuation or resume URL against baseOrigin,
// which must already be in CanonicalOrigin form. It returns nil when the URL
// may be followed, else a Terminal(invalid_continuation) error — no request is
// issued to the failing URL, there is no retry and no handler, and the
// rejected URL is carried redacted to its origin (a hostile continuation's
// path and query are exactly what must not be echoed).
//
// It takes the origin rather than the run loop because the validation is a
// pure function of the two URLs: nothing here needs a run, and keeping it
// free-standing is what lets this file be reviewed and tested on its own.
func checkContinuation(baseOrigin, pageURL string) *TerminalError {
	// Validated RAW, before canonicalization, mirroring validateConfig's
	// raw-origin check: CanonicalOrigin lowercases through strings.ToLower,
	// which rewrites every invalid byte to U+FFFD, so a continuation host
	// carrying a raw invalid byte would collapse to the same canonical form
	// as a configured origin that legitimately contains the replacement
	// character — and same-origin validation must never equate distinct byte
	// strings. A conformant server's URLs are valid UTF-8, so refusing the
	// bytes outright is fail-closed.
	if !utf8.ValidString(pageURL) {
		return &TerminalError{
			Reason: ReasonInvalidContinuation,
			Msg:    "the continuation URL is not valid UTF-8",
		}
	}
	origin, err := CanonicalOrigin(pageURL)
	if err != nil {
		return &TerminalError{
			Reason: ReasonInvalidContinuation,
			Msg:    "the continuation URL is not an absolute URL carrying a scheme and host",
		}
	}
	if scheme, _, _ := strings.Cut(origin, "://"); scheme != "http" && scheme != "https" {
		return &TerminalError{
			Reason: ReasonInvalidContinuation,
			Msg:    "the continuation URL scheme " + `"` + scheme + `"` + " is not http(s)",
		}
	}
	// Canonical-origin equality IS §8's algorithm — scheme plus normalized
	// host with the default port stripped — and it subsumes the downgrade
	// rejection: an https base never equals an http continuation.
	if origin != baseOrigin {
		return &TerminalError{
			Reason: ReasonInvalidContinuation,
			Msg:    "the continuation URL origin " + origin + " is not the configured base origin",
		}
	}
	return nil
}
