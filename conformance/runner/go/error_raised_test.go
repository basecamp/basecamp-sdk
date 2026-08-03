package main

import (
	"errors"
	"testing"
)

// The wording is asserted verbatim, here and in the five sibling runners. A
// fixture debugged in one language should not read differently in another.
const errorRaisedMessage = "Expected the call to fail, but it succeeded"

// Both directions of the errorRaised contract.
//
// Only the passing direction ever runs against a committed fixture: every case
// declaring errorRaised is one the SDK does refuse. A handler that accepted
// everything would therefore look green in all six runners, which is exactly
// how #563 shipped a vacuous delayBetweenRequests check.
func TestErrorRaisedFailure(t *testing.T) {
	if msg := errorRaisedFailure(true); msg != "" {
		t.Fatalf("a failed dispatch satisfies errorRaised, got %q", msg)
	}
	if msg := errorRaisedFailure(false); msg != errorRaisedMessage {
		t.Fatalf("expected %q for a successful dispatch, got %q", errorRaisedMessage, msg)
	}
}

// The wiring, not just the predicate: checkAssertion must route the assertion
// type to that branch and surface its message. A typo'd case label would leave
// errorRaised falling through to the default and asserting nothing.
func TestCheckAssertionRoutesErrorRaised(t *testing.T) {
	tc := TestCase{Name: "update-kill"}
	assertion := Assertion{Type: "errorRaised"}

	if res := checkAssertion(tc, assertion, operationResult{err: errors.New("malformed response")}, 1, nil, nil, nil, nil, nil); res != nil {
		t.Fatalf("expected a refused dispatch to pass, got %q", res.Message)
	}

	res := checkAssertion(tc, assertion, operationResult{}, 2, nil, nil, nil, nil, nil)
	if res == nil {
		t.Fatal("expected checkAssertion to fail when the dispatch succeeded, got a pass")
	}
	if res.Message != errorRaisedMessage {
		t.Fatalf("expected %q, got %q", errorRaisedMessage, res.Message)
	}
}
