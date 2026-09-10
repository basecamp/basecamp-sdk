package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

var (
	headerToken   = regexp.MustCompile(`^\{\{(.*)\}\}$`)
	httpdateToken = regexp.MustCompile(`^httpdate\+([0-9]+)s$`)
)

// resolveHeaderValue substitutes the one token a fixture header value may
// carry, `{{httpdate+Ns}}`, at the moment the response is served (SPEC §19,
// conformance/schema.json). Every other value passes through untouched.
//
// The token resolves to the IMF-fixdate of floor(now) + N + 1 seconds: the
// first whole second strictly more than N seconds after the second the
// response is served in. A compliant SPEC §6 parser sees a remainder in
// (N - latency, N + 1] and, rounding up, computes at least N whole seconds, so
// the fixture pairs it with a `delayBetweenRequests` floor of N × 1000 ms. It
// exists because a static fixture has no clock: a literal past date pins only
// the fall-through, and a far-future one is differently behaved per host.
//
// An unrecognised `{{…}}` is an error rather than a literal: a typo'd token
// served verbatim would be an unparseable header, which the SDK answers with
// its ordinary backoff — the exact outcome the case exists to distinguish from.
func resolveHeaderValue(value string, now time.Time) (string, error) {
	token := headerToken.FindStringSubmatch(value)
	if token == nil {
		return value, nil
	}
	inner := httpdateToken.FindStringSubmatch(token[1])
	if inner == nil {
		return "", fmt.Errorf("unrecognised header token %q: only {{httpdate+Ns}} is defined (conformance/schema.json)", value)
	}
	n, err := strconv.ParseInt(inner[1], 10, 64)
	if err != nil {
		return "", fmt.Errorf("header token %q: %w", value, err)
	}
	at := time.Unix(now.Unix()+n+1, 0).UTC()
	return at.Format(http.TimeFormat), nil
}
