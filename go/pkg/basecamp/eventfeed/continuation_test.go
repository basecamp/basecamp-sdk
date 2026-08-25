package eventfeed

import (
	"strings"
	"testing"
)

// The loop-level tests reach this validator only through a full catch-up walk
// (catchup_test.go, recovery_test.go), which exercises the accept/reject
// decision but not the boundaries between its three rejection reasons. These
// call it directly, which is the whole point of it being free-standing: the
// SPEC.md §8 Same-Origin Validation Algorithm is a pure function of two URLs
// and is worth pinning as one.

const testBase = "https://3.basecampapi.com"

func TestCheckContinuation_AcceptsSameOrigin(t *testing.T) {
	for _, pageURL := range []string{
		"https://3.basecampapi.com/5951425/events.json?page=2",
		// The default port is stripped by CanonicalOrigin on both sides, so
		// an explicitly-ported URL is the same origin.
		"https://3.basecampapi.com:443/5951425/events.json",
		// Host case is normalized.
		"https://3.BasecampAPI.com/5951425/events.json",
		// A zero-padded default port is still the default port: Port()
		// returns the raw spelling, so the comparison must be numeric or a
		// normal continuation gets rejected over its spelling.
		"https://3.basecampapi.com:000443/5951425/events.json",
	} {
		if terr := checkContinuation(testBase, pageURL); terr != nil {
			t.Errorf("checkContinuation(%q) = %v, want nil", pageURL, terr)
		}
	}
}

func TestCheckContinuation_RejectsNonAbsoluteURL(t *testing.T) {
	for _, pageURL := range []string{
		"",
		"/5951425/events.json?page=2",
		"3.basecampapi.com/events.json",
		"https://",
		"://missing-scheme",
	} {
		terr := checkContinuation(testBase, pageURL)
		if terr == nil {
			t.Fatalf("checkContinuation(%q) = nil, want Terminal(invalid_continuation)", pageURL)
		}
		if terr.Reason != ReasonInvalidContinuation {
			t.Errorf("checkContinuation(%q).Reason = %q, want %q", pageURL, terr.Reason, ReasonInvalidContinuation)
		}
	}
}

func TestCheckContinuation_RejectsNonHTTPScheme(t *testing.T) {
	for _, pageURL := range []string{
		"ftp://3.basecampapi.com/events.json",
		"file://3.basecampapi.com/etc/passwd",
		"ws://3.basecampapi.com/events.json",
	} {
		terr := checkContinuation(testBase, pageURL)
		if terr == nil {
			t.Fatalf("checkContinuation(%q) = nil, want Terminal(invalid_continuation)", pageURL)
		}
		if terr.Reason != ReasonInvalidContinuation {
			t.Errorf("checkContinuation(%q).Reason = %q, want %q", pageURL, terr.Reason, ReasonInvalidContinuation)
		}
	}
}

// A downgrade is an origin mismatch, not a separate check: an https base can
// never equal an http continuation, which is what makes canonical-origin
// equality subsume §8's downgrade rejection rather than merely accompany it.
func TestCheckContinuation_RejectsDowngradeAndCrossOrigin(t *testing.T) {
	for _, pageURL := range []string{
		"http://3.basecampapi.com/5951425/events.json",
		"https://evil.example/5951425/events.json",
		"https://3.basecampapi.com.evil.example/events.json",
		"https://3.basecampapi.com:8443/events.json",
	} {
		terr := checkContinuation(testBase, pageURL)
		if terr == nil {
			t.Fatalf("checkContinuation(%q) = nil, want Terminal(invalid_continuation)", pageURL)
		}
		if terr.Reason != ReasonInvalidContinuation {
			t.Errorf("checkContinuation(%q).Reason = %q, want %q", pageURL, terr.Reason, ReasonInvalidContinuation)
		}
	}
}

// The rejected URL is carried redacted to its origin. A hostile continuation's
// path and query are exactly what must not reach an observer's log, so the
// message must quote neither.
func TestCheckContinuation_RejectionMessageOmitsPathAndQuery(t *testing.T) {
	const hostile = "https://evil.example/steal?ticket=s3cr3t-ticket-value#frag"
	terr := checkContinuation(testBase, hostile)
	if terr == nil {
		t.Fatal("checkContinuation(hostile) = nil, want Terminal(invalid_continuation)")
	}
	for _, leaked := range []string{"s3cr3t-ticket-value", "ticket=", "/steal", "frag"} {
		if strings.Contains(terr.Msg, leaked) {
			t.Errorf("rejection message %q leaks %q", terr.Msg, leaked)
		}
	}
	if !strings.Contains(terr.Msg, "https://evil.example") {
		t.Errorf("rejection message %q does not name the offending origin", terr.Msg)
	}
}

// The verdict is a function of BOTH arguments, not a hard-coded allowlist.
// Every case above holds the base fixed, which cannot tell "validates against
// the configured origin" apart from "validates against 3.basecampapi.com" —
// the SDK supports configurable base URLs, so that distinction is the whole
// contract.
func TestCheckContinuation_VerdictFollowsTheConfiguredBase(t *testing.T) {
	const selfHosted = "https://bc.internal.example:8443"
	const pageURL = "https://bc.internal.example:8443/5951425/events.json?page=2"

	if terr := checkContinuation(selfHosted, pageURL); terr != nil {
		t.Errorf("checkContinuation(%q, %q) = %v, want nil", selfHosted, pageURL, terr)
	}
	// The identical URL is cross-origin under the default base.
	if terr := checkContinuation(testBase, pageURL); terr == nil {
		t.Errorf("checkContinuation(%q, %q) = nil, want Terminal(invalid_continuation)", testBase, pageURL)
	}
	// And the default base's own continuation is cross-origin under this one.
	const defaultPage = "https://3.basecampapi.com/5951425/events.json"
	if terr := checkContinuation(selfHosted, defaultPage); terr == nil {
		t.Errorf("checkContinuation(%q, %q) = nil, want Terminal(invalid_continuation)", selfHosted, defaultPage)
	}
}

// TestCheckContinuation_RejectsInvalidUTF8BeforeCanonicalizing: same-origin
// validation must never equate distinct byte strings. CanonicalOrigin
// lowercases through strings.ToLower, which rewrites every invalid byte to
// U+FFFD — so a continuation host carrying a raw invalid byte collapses to
// the same canonical form as a configured origin that legitimately contains
// the replacement character (valid UTF-8, so construction accepts it), and
// the authenticated poll would follow a URL the server never named. The raw
// bytes are refused before the lossy step, mirroring validateConfig's
// raw-origin check; a conformant server's URLs are valid UTF-8, so refusal
// is fail-closed.
func TestCheckContinuation_RejectsInvalidUTF8BeforeCanonicalizing(t *testing.T) {
	base, err := CanonicalOrigin("https://�.example.com")
	if err != nil {
		t.Fatalf("CanonicalOrigin: %v", err)
	}
	terr := checkContinuation(base, "https://\xff.example.com/5951425/events.json?page=2")
	if terr == nil {
		t.Fatal("an invalid-UTF-8 continuation host collapsed to the configured origin's U+FFFD and was accepted")
	}
	if terr.Reason != ReasonInvalidContinuation {
		t.Fatalf("reason = %q, want %q", terr.Reason, ReasonInvalidContinuation)
	}
	if !strings.Contains(terr.Msg, "UTF-8") {
		t.Fatalf("message %q does not name the UTF-8 refusal", terr.Msg)
	}
}

// TestCanonicalOrigin_PortSpellingsCollapse pins that the canonical form is
// a function of the ORIGIN, not the port's spelling — the identity-split
// class checkIdentityText exists to refuse: two spellings of one origin
// yielding two checkpoint keys silently forks the lineage. Out-of-range
// ports are refused outright, matching the cable-URL policy's stance on a
// port no client can connect to.
func TestCanonicalOrigin_PortSpellingsCollapse(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"https://3.basecampapi.com:000443/events.json", "https://3.basecampapi.com"},
		{"https://3.basecampapi.com:443/events.json", "https://3.basecampapi.com"},
		{"http://3.basecampapi.com:080/events.json", "http://3.basecampapi.com"},
		{"https://bc.internal.example:08443/events.json", "https://bc.internal.example:8443"},
		{"https://bc.internal.example:8443/events.json", "https://bc.internal.example:8443"},
	} {
		got, err := CanonicalOrigin(tc.raw)
		if err != nil || got != tc.want {
			t.Errorf("CanonicalOrigin(%q) = (%q, %v), want (%q, nil)", tc.raw, got, err, tc.want)
		}
	}
	for _, raw := range []string{
		"https://3.basecampapi.com:0/events.json",
		"https://3.basecampapi.com:000/events.json",
		"https://3.basecampapi.com:99999/events.json",
		"https://3.basecampapi.com:99999999999999999999/events.json",
	} {
		if got, err := CanonicalOrigin(raw); err == nil {
			t.Errorf("CanonicalOrigin(%q) = %q, want a refusal — an unusable port is not an identity", raw, got)
		}
	}
}
