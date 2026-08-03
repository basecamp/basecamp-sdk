// Bounds contract for the requestCount assertion (#573).
//
// Until this commit the five non-Swift runners evaluated requestCount as a
// LOWER bound whenever any mock response carried `Link: rel="next"`. Every
// committed fixture passes under both rules, so nothing in the suite could tell
// them apart — the same shape as the #563 delayBetweenRequests regression these
// support modules exist to pin. The over-fetch case below is the one that
// distinguishes them, and it is the case that matters: pagination.json's
// maxPages and maxItems fixtures each queue three pages and assert two
// requests, so a lower bound green-passes an SDK that ignored the cap.

package main

import "testing"

func TestRequestCountAcceptsTheExactCount(t *testing.T) {
	if msg := checkRequestCount(2, 2); msg != "" {
		t.Fatalf("exact match should pass; got %q", msg)
	}
}

func TestRequestCountRejectsAnUnderFetch(t *testing.T) {
	if msg := checkRequestCount(1, 2); msg == "" {
		t.Fatal("1 request where 2 were expected should fail")
	}
}

func TestRequestCountRejectsAnOverFetch(t *testing.T) {
	// The regression. Under the old lower bound this returned "" — an SDK that
	// walked all three queued pages instead of stopping at the maxPages cap
	// reported a clean pass.
	if msg := checkRequestCount(3, 2); msg == "" {
		t.Fatal("3 requests where 2 were expected should fail; a lower bound would accept it")
	}
}

func TestRequestCountMessageNamesBothCounts(t *testing.T) {
	msg := checkRequestCount(3, 2)
	if msg != "Expected 2 requests, got 3" {
		t.Fatalf("failure message should name expected and actual; got %q", msg)
	}
}

func TestRequestCountZeroRequestsIsNotAFreePass(t *testing.T) {
	// A test whose operation never reached the wire records zero requests.
	// That must fail an assertion expecting one, not read as "no data, no
	// opinion".
	if msg := checkRequestCount(0, 1); msg == "" {
		t.Fatal("0 requests where 1 was expected should fail")
	}
}

func TestRequestCountZeroExpectedRequiresZeroActual(t *testing.T) {
	if msg := checkRequestCount(0, 0); msg != "" {
		t.Fatalf("0 expected and 0 actual should pass; got %q", msg)
	}
	if msg := checkRequestCount(1, 0); msg == "" {
		t.Fatal("1 request where 0 were expected should fail")
	}
}

// Applicability contract (#573). The `link-header` fixture's requestCount is
// inapplicable to an auto-paginating SDK; its statusCode and noError
// assertions are not. Suppressing the CASE instead of the ASSERTION left the
// fixture executed by nothing at all — it stays in pagination.json and passes
// conformance-fixtures-check and check-fixture-coverage either way, so nothing
// else would have reported it.

func TestRequestCountDoesNotApplyToLinkHeaderFixtures(t *testing.T) {
	if requestCountApplies([]string{"pagination", "link-header"}) {
		t.Fatal("link-header fixtures must not have their requestCount asserted")
	}
}

func TestRequestCountAppliesToEveryOtherFixture(t *testing.T) {
	for _, tags := range [][]string{nil, {}, {"pagination"}, {"retry", "idempotent"}} {
		if !requestCountApplies(tags) {
			t.Fatalf("requestCount must be asserted for tags %v", tags)
		}
	}
}

// The suppression is one assertion wide. If it ever grows to the whole case
// again, the fixture's other two assertions stop running everywhere they still
// run, and nothing downstream notices.
func TestLinkHeaderSuppressionIsScopedToTheCountAssertion(t *testing.T) {
	tc := TestCase{
		Name: "List operation returns first page with Link header",
		Tags: []string{"pagination", "link-header"},
		Assertions: []Assertion{
			{Type: "requestCount", Expected: float64(1)},
			{Type: "statusCode", Expected: float64(200)},
			{Type: "noError"},
		},
	}
	live := 0
	for _, a := range tc.Assertions {
		if a.Type == "requestCount" && !requestCountApplies(tc.Tags) {
			continue
		}
		live++
	}
	if live != 2 {
		t.Fatalf("statusCode and noError must still be evaluated; %d assertion(s) live", live)
	}
}
