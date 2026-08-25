// Tier-2 conformance: the driver's own tests.
//
// A driver that cannot fail proves nothing. Every case here derives a HOSTILE
// scenario in memory — never a committed fixture; `conformance/event-feed/` is
// merged contract — and requires the driver to reject it. The mutation cases
// follow the family's own pin-probe discipline: each asserts the unmutated
// control PASSES and the single-delta mutant FAILS, so a case that stops
// discriminating fails loudly instead of going vacuous.
package eventfeed_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// TestScenarioDriverRejectsMutatedFixtures is the driver's kill test: a
// one-field mutation of a real fixture must turn a passing scenario red.
func TestScenarioDriverRejectsMutatedFixtures(t *testing.T) {
	for _, probe := range []struct {
		name    string
		fixture string
		path    string
		value   any
		// wants is a fragment the failure must name, so a mutant rejected for
		// the wrong reason cannot pass as the pin firing.
		wants string
	}{
		{
			name:    "delivered set",
			fixture: "01-happy-path-confirm-catchup-stream.json",
			path:    "finally.delivered.exact",
			value:   []any{101, 102, 103, 104},
			wants:   "finally.delivered",
		},
		{
			name:    "checkpoint ledger",
			fixture: "01-happy-path-confirm-catchup-stream.json",
			path:    "finally.checkpoints.exact",
			value:   []any{"{{POS:1}}"},
			wants:   "finally.checkpoints",
		},
		{
			name:    "mint seam-call count",
			fixture: "05-fresh-ticket-reconnect-after-ttl.json",
			path:    "finally.mintCount",
			value:   1,
			wants:   "finally.mintCount",
		},
		{
			name:    "outstanding timer set",
			fixture: "04-terminal-rejection.json",
			path:    "finally.timers.exact",
			value:   map[string]any{"backoff": 1},
			wants:   "finally.timers",
		},
		{
			name:    "terminal reason",
			fixture: "06-protocol-fatal-disconnect.json",
			path:    "finally.error.reason",
			value:   "subscription_rejected",
			wants:   "finally.error",
		},
		{
			name:    "handler invocation record",
			fixture: "24-overflow-handler-terminate.json",
			path:    "finally.handlerInvocations.exact",
			value:   []any{map[string]any{"kind": "bufferOverflow", "disposition": "accept"}},
			wants:   "finally.handlerInvocations",
		},
		{
			name:    "reconnect dials the previous mint's url",
			fixture: "05-fresh-ticket-reconnect-after-ttl.json",
			path:    "steps.14.expectConnect.url",
			value:   "{{CABLE_URL:1}}",
			wants:   "a cable dial",
		},
		{
			name:    "catch-up poll cursor",
			fixture: "12-checkpoint-after-handoff.json",
			path:    "steps.5.expectPoll.query.position",
			value:   "{{POS:9}}",
			wants:   "poll query",
		},
		{
			name:    "subscribe identifier params",
			fixture: "01-happy-path-confirm-catchup-stream.json",
			path:    "steps.3.expectSubscribe.params.types",
			value:   "todo.created",
			wants:   "subscribe identifier",
		},
		{
			name:    "signal dropped ids",
			fixture: "21-overflow-default-terminal.json",
			path:    "steps.7.expectSignal.droppedIds",
			value:   []any{52},
			wants:   "dropped ids",
		},
		{
			name:    "buffered occupancy rendezvous",
			fixture: "19-present-entry-buffered-lower-id.json",
			path:    "steps.6.expectBuffered.count",
			value:   2,
			wants:   "live buffer occupancy",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			control := readFixture(t, probe.fixture)
			if err := runScenarioBytes(control, probe.fixture); err != nil {
				t.Fatalf("the control must pass, else the mutant proves nothing: %v", err)
			}
			mutant := mutateFixture(t, control, probe.path, probe.value)
			err := underShortWatchdog(func() error { return runScenarioBytes(mutant, probe.fixture) })
			if err == nil {
				t.Fatalf("the driver accepted %s = %v — a mutated expectation must turn the scenario red", probe.path, probe.value)
			}
			if !strings.Contains(err.Error(), probe.wants) {
				t.Fatalf("the mutant failed for the wrong reason:\n  got  %v\n  want a failure naming %q", err, probe.wants)
			}
		})
	}
}

// TestScenarioDriverEnforcesDeliveryBeforeCheckpoint pins the driver's
// arrival-strict save rule — the teeth of checkpoint-after-handoff, which no
// end-state assertion can catch. Fixture 02's entry is position-resume, so its
// empty final page legitimately saves BEFORE the buffered event drains, and
// the fixture scripts exactly that order. Swapping its last two steps demands
// the delivery first; the save still arrives first, and the driver must say so
// rather than matching it against a later step.
//
// It is the ARRIVAL rule that says so. The save's own ordering witness — the
// delivery count `saveRecord` captures — is the second line, and on a scenario
// path it no longer gets to speak: a save that beats the deliveries its script
// demands lands while the pointer is still on the unsatisfied `expectDelivered`
// step, which is an early arrival before the witness is ever compared. The
// witness is retained because the lookahead that computes the current step errs
// deliberately permissive (see stepSatisfiedLocked), and it is the check that
// still fires if a step's arm is ever loosened.
func TestScenarioDriverEnforcesDeliveryBeforeCheckpoint(t *testing.T) {
	const fixture = "02-confirmation-gating.json"
	control := readFixture(t, fixture)
	if err := runScenarioBytes(control, fixture); err != nil {
		t.Fatalf("the control must pass, else the reordering proves nothing: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(control, &doc); err != nil {
		t.Fatalf("decoding the control fixture: %v", err)
	}
	steps, ok := doc["steps"].([]any)
	if !ok || len(steps) < 2 {
		t.Fatalf("the control fixture has no steps to reorder")
	}
	last, penultimate := len(steps)-1, len(steps)-2
	if !stepIs(steps[penultimate], "expectCheckpoint") || !stepIs(steps[last], "expectDelivered") {
		t.Fatalf("the control fixture no longer ends in expectCheckpoint, expectDelivered — re-derive this probe")
	}
	steps[penultimate], steps[last] = steps[last], steps[penultimate]
	mutant, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-encoding the mutant: %v", err)
	}

	err = underShortWatchdog(func() error { return runScenarioBytes(mutant, fixture) })
	if err == nil {
		t.Fatal("a save that arrives before the deliveries the script requires must fail the scenario")
	}
	if !strings.Contains(err.Error(), "arrival-strict") || !strings.Contains(err.Error(), "expectDelivered") {
		t.Fatalf("failed for the wrong reason:\n  got  %v\n  want the save rejected on arrival, under the still-pending expectDelivered step", err)
	}
}

// TestScenarioDriverIsArrivalStrict pins the two arrival-strict classes and,
// more to the point, the handoff that makes them survivable.
//
// A naive "is the pointer sitting on my expect step?" check reports every
// rendezvous-adjacent action as an early arrival, because the connector reacts
// to a rendezvous being satisfied before the driver has stepped over it. Two
// mechanisms answer that, and this test is written to fail if either is
// removed: `await` advances the pointer inside the critical section that
// satisfies a rendezvous, and `currentStepLocked` looks ahead over steps the
// history has already satisfied but the driver has not consumed yet. Each
// legal case below is paired with the illegal one it differs from by a single
// fact, so a mechanism that stops discriminating fails loudly rather than
// making everything legal.
//
// Every case drives the harness's own record paths — `Save`, and the frame
// recorder `readPeer` funnels through — which is where the rule lives.
func TestScenarioDriverIsArrivalStrict(t *testing.T) {
	save := func(h *scenarioHarness) { _ = h.Save(context.Background(), eventfeed.CheckpointKey{}, arrivalPosition) }
	subscribe := func(h *scenarioHarness, peer *cablePeer) {
		h.recordClientFrame(peer, clientFrame{kind: "text", data: []byte(`{"command":"subscribe"}`)})
	}

	t.Run("a save at its expectCheckpoint step is legal", func(t *testing.T) {
		h, _ := arrivalProgram(t, 1, stepServe(), stepExpectCheckpoint())
		save(h)
		requireNoViolation(t, h)
	})

	t.Run("a save under an unexecuted action step is an early arrival", func(t *testing.T) {
		h, _ := arrivalProgram(t, 0, stepServe(), stepExpectCheckpoint())
		save(h)
		requireViolation(t, h, "step 1 (serve)", "expectCheckpoint")
	})

	t.Run("beginStep hands an action step's reaction to the step after it", func(t *testing.T) {
		h, d := arrivalProgram(t, 0, stepServe(), stepExpectCheckpoint())
		d.beginStep(0, d.sc.Steps[0])
		save(h)
		requireNoViolation(t, h)
	})

	// The rendezvous-adjacency pair. Both saves arrive with the pointer parked
	// on an expectDelivered step the driver has not reached; only the delivery
	// separates them. A driver that judged against the parked step alone would
	// reject the first, which is what fixture 01, 12, 22 and 26 do on every run.
	t.Run("a save handed over by a satisfied rendezvous is legal", func(t *testing.T) {
		h, _ := arrivalProgram(t, 0, stepExpectDelivered(1), stepExpectCheckpoint())
		h.recordDelivered(1)
		save(h)
		requireNoViolation(t, h)
	})

	t.Run("a save ahead of that rendezvous is an early arrival", func(t *testing.T) {
		h, _ := arrivalProgram(t, 0, stepExpectDelivered(1), stepExpectCheckpoint())
		save(h)
		requireViolation(t, h, "step 1 (expectDelivered)", "expectCheckpoint")
	})

	t.Run("a client close before its step is an early arrival", func(t *testing.T) {
		h, _ := arrivalProgram(t, 0, stepExpectSubscribe(), stepExpectClientClose())
		h.recordClientFrame(&cablePeer{}, clientFrame{kind: "close", code: 1000})
		requireViolation(t, h, "step 1 (expectSubscribe)", "expectClientClose")
	})

	t.Run("the handoff out of a consumed rendezvous is atomic", func(t *testing.T) {
		// expectCheckpoint's own condition goes FALSE again the moment it
		// consumes the save, so nothing downstream could look ahead over it:
		// only the advance inside `await` puts the pointer on the close step.
		h, d := arrivalProgram(t, 0, stepExpectCheckpoint(), stepExpectClientClose())
		save(h)
		if err := d.expectCheckpoint(arrivalPosition); err != nil {
			t.Fatalf("the checkpoint step must match its save: %v", err)
		}
		h.recordClientFrame(&cablePeer{}, clientFrame{kind: "close", code: 1000})
		requireNoViolation(t, h)
	})

	// The subscribe-at-dial mutant, deterministically. The dial is recorded, so
	// the expectConnect step is satisfied and the lookahead runs past it — and
	// stops at the `serve welcome` the driver has not performed, which is the
	// whole reason an unexecuted action step must not be skippable.
	t.Run("a subscribe written at dial is an early arrival", func(t *testing.T) {
		h, d := arrivalProgram(t, 0, stepExpectConnect(), stepServe(), stepExpectSubscribe())
		peer := recordDial(h, d)
		subscribe(h, peer)
		requireViolation(t, h, "step 2 (serve)", "expectSubscribe")
	})

	t.Run("a subscribe written on welcome is legal", func(t *testing.T) {
		h, d := arrivalProgram(t, 0, stepExpectConnect(), stepServe(), stepExpectSubscribe())
		peer := recordDial(h, d)
		d.beginStep(1, d.sc.Steps[1])
		subscribe(h, peer)
		requireNoViolation(t, h)
	})
}

// arrivalPosition is the checkpoint position every case above saves.
const arrivalPosition = "pos-1"

// arrivalProgram binds a harness to a driver over a synthetic step program,
// with the arrival rule's pointer parked on step `at`.
func arrivalProgram(t *testing.T, at int, steps ...scenarioStep) (*scenarioHarness, *driver) {
	t.Helper()
	h := newScenarioHarness()
	t.Cleanup(h.close)
	d := &driver{h: h, sc: &scenario{Steps: steps}}
	h.attach(d)
	h.enterStep(at)
	return h, d
}

// recordDial records one accepted cable dial and hands the driver its socket,
// exactly as a matched expectConnect step would. The peer carries no live
// socket — these cases exercise the recording paths, not the loopback — so it
// deliberately stays out of h.peers, which teardown closes.
func recordDial(h *scenarioHarness, d *driver) *cablePeer {
	peer := &cablePeer{}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connects = append(h.connects, &connectAttempt{url: h.cableURL(1), kind: "accept", peer: peer})
	d.peer = peer
	return peer
}

func requireNoViolation(t *testing.T, h *scenarioHarness) {
	t.Helper()
	if err := h.violation(); err != nil {
		t.Fatalf("an action legal under the arrival rule was rejected: %v", err)
	}
}

func requireViolation(t *testing.T, h *scenarioHarness, fragments ...string) {
	t.Helper()
	err := h.violation()
	if err == nil {
		t.Fatal("an action arriving outside its matching expect step must be a violation")
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("rejected for the wrong reason:\n  got  %v\n  want a failure naming %q", err, fragment)
		}
	}
}

func stepServe() scenarioStep {
	return scenarioStep{Kind: "serve", Payload: &serveStep{Frame: "welcome"}}
}

func stepExpectConnect() scenarioStep {
	return scenarioStep{Kind: "expectConnect", Payload: &expectConnectStep{}}
}

func stepExpectSubscribe() scenarioStep {
	return scenarioStep{Kind: "expectSubscribe", Payload: &expectSubscribeStep{Channel: "EventsChannel"}}
}

func stepExpectClientClose() scenarioStep {
	return scenarioStep{Kind: "expectClientClose", Payload: &expectClientCloseStep{}}
}

func stepExpectDelivered(ids ...int64) scenarioStep {
	return scenarioStep{Kind: "expectDelivered", Payload: &exactIDs{Exact: ids}}
}

func stepExpectCheckpoint() scenarioStep {
	return scenarioStep{Kind: "expectCheckpoint", Payload: &expectCheckpointStep{Position: arrivalPosition}}
}

func stepIs(step any, directive string) bool {
	object, ok := step.(map[string]any)
	if !ok {
		return false
	}
	_, present := object[directive]
	return present
}

// TestScenarioDriverRejectsUnmodelledScripts pins the load-time half: an
// unknown directive, an unknown key, an unknown placeholder, or a construct
// the driver does not model must abort the fixture with a named error rather
// than being skipped. This is what keeps the driver honest as PR 4 adds
// fixtures and schema constructs.
func TestScenarioDriverRejectsUnmodelledScripts(t *testing.T) {
	base := readFixture(t, "04-terminal-rejection.json")
	for _, probe := range []struct {
		name   string
		script string
		wants  string
	}{
		{
			name:   "unknown directive",
			script: `{"name":"x","description":"d","steps":[{"expectSomethingNew":{}}],"finally":{"state":"closed"}}`,
			wants:  "unknown step directive",
		},
		{
			name:   "two directives in one step",
			script: `{"name":"x","description":"d","steps":[{"sever":true,"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "exactly one directive",
		},
		{
			name:   "unknown key inside a directive",
			script: `{"name":"x","description":"d","steps":[{"advance":{"ms":1,"jitter":2}}],"finally":{"state":"closed"}}`,
			wants:  "jitter",
		},
		{
			name:   "unknown top-level key",
			script: `{"name":"x","description":"d","only":true,"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  `unknown key "only"`,
		},
		{
			name:   "unknown placeholder",
			script: `{"name":"x","description":"d","steps":[{"expectConnect":{"url":"{{CABLE_HOST:1}}"}}],"finally":{"state":"closed"}}`,
			wants:  "unknown placeholder",
		},
		{
			name:   "unknown timer kind",
			script: `{"name":"x","description":"d","steps":[{"fireTimer":{"kind":"reconnect"}}],"finally":{"state":"closed"}}`,
			wants:  "unknown timer kind",
		},
		{
			name:   "unknown terminal reason",
			script: `{"name":"x","description":"d","steps":[{"expectError":{"reason":"exploded"}}],"finally":{"state":"terminal"}}`,
			wants:  "unknown terminal reason",
		},
		{
			name:   "inverted fireTimer envelope",
			script: `{"name":"x","description":"d","steps":[{"fireTimer":{"kind":"backoff","assertDelayMs":{"min":1000,"max":10}}}],"finally":{"state":"closed"}}`,
			wants:  "exceeds max",
		},
		{
			// The schema's shared ms maximum, driver-enforced: one past Go's
			// int64-nanosecond line, time.Duration(ms)*time.Millisecond goes
			// NEGATIVE and an accepted advance would REWIND virtual time. The
			// bound is checked at load so the overflow is a fixture error in
			// every driver, not a representation accident in one.
			name:   "advance ms beyond the 10-virtual-year maximum",
			script: `{"name":"x","description":"d","steps":[{"advance":{"ms":9223372036855}}],"finally":{"state":"closed"}}`,
			wants:  "10 virtual years",
		},
		{
			name:   "fireTimer envelope beyond the 10-virtual-year maximum",
			script: `{"name":"x","description":"d","steps":[{"fireTimer":{"kind":"backoff","assertDelayMs":{"min":0,"max":9223372036855}}}],"finally":{"state":"closed"}}`,
			wants:  "10 virtual years",
		},
		{
			name:   "config stalenessMs beyond the 10-virtual-year maximum",
			script: `{"name":"x","description":"d","config":{"stalenessMs":9223372036855},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "10 virtual years",
		},
		{
			// Explicit zero is not omission: the schema says minimum 1, and a
			// plain int64 decode read {"stalenessMs":0} as "absent, use the
			// default" — accepting a value the schema rejects and silently
			// substituting another. Presence is preserved with pointers.
			name:   "config stalenessMs explicit zero",
			script: `{"name":"x","description":"d","config":{"stalenessMs":0},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "must be in [1,",
		},
		{
			name:   "config confirmationDeadlineMs explicit zero",
			script: `{"name":"x","description":"d","config":{"confirmationDeadlineMs":0},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "must be in [1,",
		},
		{
			name:   "config repairPollBaseMs explicit zero",
			script: `{"name":"x","description":"d","config":{"repairPollBaseMs":0},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "must be in [1,",
		},
		{
			// The unmodeled pair is unmodeled at ANY supplied value: explicit
			// zero used to slip past the != 0 check into silence.
			name:   "config backoffBaseMs explicit zero",
			script: `{"name":"x","description":"d","config":{"backoffBaseMs":0},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "not modeled",
		},
		{
			// Explicit null is the presence saga's second act: *int64 read
			// {"stalenessMs":null} and omission both as nil, silently
			// defaulting a value the schema's "type": "integer" rejects.
			name:   "config stalenessMs explicit null",
			script: `{"name":"x","description":"d","config":{"stalenessMs":null},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "supplied as JSON null",
		},
		{
			name:   "config confirmationDeadlineMs explicit null",
			script: `{"name":"x","description":"d","config":{"confirmationDeadlineMs":null},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "supplied as JSON null",
		},
		{
			name:   "config repairPollBaseMs explicit null",
			script: `{"name":"x","description":"d","config":{"repairPollBaseMs":null},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "supplied as JSON null",
		},
		{
			name:   "config backoffBaseMs explicit null",
			script: `{"name":"x","description":"d","config":{"backoffBaseMs":null},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "not modeled",
		},
		{
			name:   "config backoffCapMs explicit null",
			script: `{"name":"x","description":"d","config":{"backoffCapMs":null},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "not modeled",
		},
		{
			// The envelope has the same three states the config durations do:
			// a pointer read "assertDelayMs": null as omission, skipping the
			// assertion the script wrote.
			name:   "fireTimer assertDelayMs explicit null",
			script: `{"name":"x","description":"d","steps":[{"fireTimer":{"kind":"backoff","assertDelayMs":null}}],"finally":{"state":"closed"}}`,
			wants:  "supplied as JSON null",
		},
		{
			// min and max are schema-required: an absent member decoded to
			// int64(0), inside the allowed range, silently converting the
			// authored envelope into a different one.
			name:   "fireTimer assertDelayMs missing min",
			script: `{"name":"x","description":"d","steps":[{"fireTimer":{"kind":"backoff","assertDelayMs":{"max":10}}}],"finally":{"state":"closed"}}`,
			wants:  "needs both min and max",
		},
		{
			name:   "fireTimer assertDelayMs null member",
			script: `{"name":"x","description":"d","steps":[{"fireTimer":{"kind":"backoff","assertDelayMs":{"min":null,"max":10}}}],"finally":{"state":"closed"}}`,
			wants:  "supplied as JSON null",
		},
		{
			// draft 2020-12 judges the VALUE, not the spelling: 1000.5 is not
			// an integer instance, and the refusal should say so in the
			// schema's terms rather than in encoding/json's.
			name:   "advance ms non-integral number",
			script: `{"name":"x","description":"d","steps":[{"advance":{"ms":1000.5}}],"finally":{"state":"closed"}}`,
			wants:  "is not an integer",
		},
		{
			// An advance is deterministic only from a scripted rendezvous:
			// an action's completion can precede the timer arms its
			// transition causes (expectConnect returns when the dial is
			// recorded; the handshake deadline arms on the connector's
			// goroutine after), so an unrendezvoused advance races the arm —
			// accepted on one schedule, rejected on another. The rendezvous
			// is authored, not guessed: expectTimers' exact-set match is the
			// settle, and the load rule makes its absence unscriptable.
			name:   "an advance not preceded by an expectTimers rendezvous",
			script: `{"name":"x","description":"d","steps":[{"expectMint":{"respond":{"status":200,"body":{"ticket":"{{TICKET:1}}","expires_in":120,"url":"{{CABLE_URL:1}}"}}}},{"expectConnect":{"url":"{{CABLE_URL:1}}"}},{"advance":{"ms":30000}}],"finally":{"state":"closed"}}`,
			wants:  "expectState + expectTimers rendezvous",
		},
		{
			// An EMPTY rendezvous set can never contain an arm of the
			// preceding transition, so its match orders nothing: after a
			// released failed mint, {"exact":{}} matches before backoff is
			// armed, and the advance races the arm exactly as if the
			// rendezvous were absent. A scenario with nothing yet armed
			// advances as its first step instead.
			name:   "an advance behind an empty expectTimers rendezvous",
			script: `{"name":"x","description":"d","steps":[{"expectState":{"is":"backoff"}},{"expectTimers":{"exact":{}}},{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "an empty rendezvous orders nothing",
		},
		{
			// The set match alone can coincide with a TRANSIENT mid-surgery
			// set (the welcome transition stops handshake-deadline and arms
			// confirmation-deadline in separate clock acquisitions, so
			// {staleness:1} exists in between). The state announcement bounds
			// the surgery: expectState blocks until the transition announces,
			// and every timer still unarmed at an announcement is exactly
			// what the set match then waits for.
			name:   "an advance whose rendezvous lacks the state barrier",
			script: `{"name":"x","description":"d","steps":[{"expectMint":{"respond":{"status":200,"body":{"ticket":"{{TICKET:1}}","expires_in":120,"url":"{{CABLE_URL:1}}"}}}},{"expectConnect":{"url":"{{CABLE_URL:1}}"}},{"expectTimers":{"exact":{"handshake-deadline":1}}},{"advance":{"ms":30000}}],"finally":{"state":"closed"}}`,
			wants:  "expectState + expectTimers rendezvous",
		},
		{
			name:   "droppedCount disagreeing with droppedIds",
			script: `{"name":"x","description":"d","steps":[{"expectSignal":{"kind":"bufferOverflow","droppedIds":[1],"droppedCount":2}}],"finally":{"state":"closed"}}`,
			wants:  "disagrees with",
		},
		{
			name:   "a push event missing a payload key",
			script: `{"name":"x","description":"d","steps":[{"serve":{"frame":"message","event":{"id":1,"kind":"m","event_type":"t","action":"a","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"recording_id":4}}}],"finally":{"state":"closed"}}`,
			wants:  "visible_to_clients",
		},
		{
			name:   "a construct the driver does not model",
			script: `{"name":"x","description":"d","config":{"backoffBaseMs":1000},"steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "not modeled",
		},
		{
			name:   "a name disagreeing with the filename",
			script: `{"name":"other-name","description":"d","steps":[{"advance":{"ms":1}}],"finally":{"state":"closed"}}`,
			wants:  "must match its filename",
		},
	} {
		t.Run(probe.name, func(t *testing.T) {
			err := runScenarioBytes([]byte(probe.script), "x.json")
			if err == nil {
				t.Fatalf("the driver accepted a script it cannot model: %s", probe.script)
			}
			if !strings.Contains(err.Error(), probe.wants) {
				t.Fatalf("rejected for the wrong reason:\n  got  %v\n  want a failure naming %q", err, probe.wants)
			}
		})
	}
	// The base fixture still passes: the cases above reject scripts, not
	// everything.
	if err := runScenarioBytes(base, "04-terminal-rejection.json"); err != nil {
		t.Fatalf("the control fixture must still pass: %v", err)
	}
}

// TestScenarioDriverRejectsUnmatchedActions pins the strict interleave's
// residue half: a protocol action no expect step matched, and a timer
// directive with no such timer armed, each fail rather than passing silently.
func TestScenarioDriverRejectsUnmatchedActions(t *testing.T) {
	t.Run("an unmatched mint seam call", func(t *testing.T) {
		// The connector mints immediately; the script expects nothing and
		// claims a terminal state, so the parked call is never matched.
		script := `{"name":"x","description":"d","steps":[{"expectState":{"is":"minting"}}],
			"finally":{"state":"minting","mintCount":0}}`
		err := underShortWatchdog(func() error { return runScenarioBytes([]byte(script), "x.json") })
		if err == nil {
			t.Fatal("a mint seam call no expect step matched must fail the scenario")
		}
		if !strings.Contains(err.Error(), "finally.mintCount") && !strings.Contains(err.Error(), "still parked") {
			t.Fatalf("failed for the wrong reason: %v", err)
		}
	})

	t.Run("a fireTimer with no such timer armed", func(t *testing.T) {
		script := `{"name":"x","description":"d","steps":[{"fireTimer":{"kind":"poll-retry"}}],
			"finally":{"state":"minting"}}`
		err := underShortWatchdog(func() error { return runScenarioBytes([]byte(script), "x.json") })
		if err == nil {
			t.Fatal("firing a timer that is not armed must fail the scenario, never pass vacuously")
		}
		if !strings.Contains(err.Error(), "poll-retry") {
			t.Fatalf("failed for the wrong reason: %v", err)
		}
	})
}

// TestScenarioDriverRejectsSchedulingDependentAdvance pins the advance guard.
// A driver that quietly took the scheduling-dependent path would produce a
// result that differs between languages for the same fixture, which is worse
// than a failure because nothing reports it.
//
// The rule is about what an advance would FIRE, not about what it arms, and
// the third case below is where those differ: a firing that replaces nothing
// is still rejected. That is stricter than the arming rule this replaced, and
// deliberately so — the arming rule could only be enforced by sampling, and a
// sampled MUST is not one.
//
// TestScenarioMsAcceptsIntegralNumberSpellings pins the schema's number
// model onto the loader: draft 2020-12's "integer" is any number whose
// MATHEMATICAL value is integral, so 1000.0 and 1e3 are integer instances a
// schema-valid fixture may carry, and only the Go driver was refusing them —
// the same float-spelled-int class FlexInt absorbs on the rich-text lane.
// Integrality is decidable without precision loss: everything in range sits
// far below float64's 2^53 exact-integer ceiling.
func TestScenarioMsAcceptsIntegralNumberSpellings(t *testing.T) {
	base := `{"name":"x","description":"d","config":{"stalenessMs":%s},"steps":[{"advance":{"ms":%s}}],"finally":{"state":"closed"}}`
	cases := []struct {
		name, staleness, ms, wantErr string
	}{
		{"float spelling", "1000.0", "1000.0", ""},
		{"exponent spelling", "1e3", "1e3", ""},
		{"non-integral", "1000.5", "1000", "is not an integer"},
		{"non-integral ms", "1000", "1000.5", "is not an integer"},
		// Integrality is a fact about the TEXT: float64 rounds these to
		// exactly 1000 and exactly the maximum before any Trunc can look.
		{"rounding-boundary fraction", "1000.00000000000001", "1000", "is not an integer"},
		{"near-maximum fraction", "315575999999.99999", "1000", "is not an integer"},
		{"exact maximum float spelling", "315576000000.0", "1000", ""},
		// A quoted "1000" is a STRING instance — schema type integer refuses
		// it, and json.Number's Unmarshal would happily have taken it.
		{"quoted number", `"1000"`, "1000", "is a string"},
		{"quoted ms", "1000", `"1000"`, "is a string"},
		// An exponent bomb must be refused by reading the exponent, never by
		// materializing the number.
		{"exponent bomb", "1e999999999", "1000", "beyond any modeled ms value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := fmt.Sprintf(base, tc.staleness, tc.ms)
			sc, err := parseScenario([]byte(raw), "x.json")
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("an integral spelling must load: %v", err)
				}
				if got := sc.Config.StalenessMs.v; got != 1000 && got != 315576000000 {
					t.Errorf("stalenessMs decoded to %d, want the literal's integral value", got)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want one naming %q", err, tc.wantErr)
			}
		})
	}
}

// The control matters as much as the mutants: an advance over a window with
// nothing due is ordinary and must still pass, or the guard would be rejecting
// every advance and the suite's one real advance (fixture 05) would be failing
// for the wrong reason.
func TestScenarioDriverRejectsSchedulingDependentAdvance(t *testing.T) {
	t.Run("an advance during which the connector arms a timer", func(t *testing.T) {
		// Advancing past the handshake deadline fires it, and the teardown it
		// causes arms `backoff` inside the same window — the reentrant clause
		// the algorithm cannot resolve identically across languages when the
		// recipient is another goroutine. The guard never has to observe that
		// arming: the firing alone is enough to reject the script.
		script := `{"name":"x","description":"d","steps":[
			{"expectMint":{"respond":{"status":200,"body":{"ticket":"{{TICKET:1}}","expires_in":120,"url":"{{CABLE_URL:1}}"}}}},
			{"expectConnect":{"url":"{{CABLE_URL:1}}"}},
			{"expectState":{"is":"awaiting_welcome"}},
			{"expectTimers":{"exact":{"handshake-deadline":1,"staleness":1}}},
			{"advance":{"ms":30000}}],
			"finally":{"state":"backoff"}}`
		err := underShortWatchdog(func() error { return runScenarioBytes([]byte(script), "x.json") })
		if err == nil {
			t.Fatal("an advance that changes the outstanding timer set must fail the scenario")
		}
		if !strings.Contains(err.Error(), "would fire") {
			t.Fatalf("failed for the wrong reason: %v", err)
		}
		if !strings.Contains(err.Error(), "fireTimer") {
			t.Errorf("the rejection must name the deterministic alternative: %v", err)
		}
	})

	t.Run("an advance over a quiet window is ordinary", func(t *testing.T) {
		// No connection yet, so nothing is armed and nothing is due: the
		// guard must not reject an advance merely for existing.
		script := `{"name":"x","description":"d","steps":[
			{"advance":{"ms":1000}},
			{"expectMint":{"respond":{"status":200,"body":{"ticket":"{{TICKET:1}}","expires_in":120,"url":"{{CABLE_URL:1}}"}}}},
			{"expectConnect":{"url":"{{CABLE_URL:1}}"}}],
			"finally":{"state":"awaiting_welcome"}}`
		if err := underShortWatchdog(func() error { return runScenarioBytes([]byte(script), "x.json") }); err != nil {
			t.Fatalf("an advance over a window that arms nothing must pass: %v", err)
		}
	})

	// A firing that replaces nothing is STILL rejected, and this is the case
	// that shows the rule changed rather than merely being reimplemented.
	// Here the backoff deadline expires and the connector's next act is a mint,
	// which parks inside the seam until the driver releases it, so nothing is
	// armed anywhere in the window. Under the arming rule this was legal. It is
	// not any more, because "did anything get armed?" can only be answered by
	// waiting and hoping, while "is anything due?" is one atomic read — and the
	// script that wanted this has `fireTimer`, which says which timer it means.
	t.Run("an advance in which a due timer fires without replacement is still rejected", func(t *testing.T) {
		script := `{"name":"x","description":"d","steps":[
			{"expectMint":{"respond":{"status":200,"body":{"ticket":"{{TICKET:1}}","expires_in":120,"url":"{{CABLE_URL:1}}"}}}},
			{"expectConnect":{"url":"{{CABLE_URL:1}}"}},
			{"serve":{"frame":"welcome"}},
			{"expectSubscribe":{"channel":"EventsChannel"}},
			{"fireTimer":{"kind":"confirmation-deadline"}},
			{"expectClientClose":{}},
			{"expectState":{"is":"backoff"}},
			{"expectTimers":{"exact":{"backoff":1}}},
			{"advance":{"ms":1000}},
			{"expectMint":{"respond":{"status":200,"body":{"ticket":"{{TICKET:2}}","expires_in":120,"url":"{{CABLE_URL:2}}"}}}},
			{"expectConnect":{"url":"{{CABLE_URL:2}}"}}],
			"finally":{"state":"awaiting_welcome"}}`
		err := underShortWatchdog(func() error { return runScenarioBytes([]byte(script), "x.json") })
		if err == nil {
			t.Fatal("an advance whose window fires a timer must fail, even when it replaces nothing")
		}
		if !strings.Contains(err.Error(), "would fire") {
			t.Fatalf("failed for the wrong reason: %v", err)
		}
		if !strings.Contains(err.Error(), "fireTimer") {
			t.Errorf("the rejection must name the deterministic alternative: %v", err)
		}
	})
}

// underShortWatchdog runs a scenario under a short rendezvous window. Its
// usual use is a scenario EXPECTED to fail, but it serves any case whose waits
// are all short by construction. A hostile scenario often fails by never satisfying a
// rendezvous, and waiting the full window for each would cost more than the
// whole conformance suite; every caller still pins the failure's reason, so a
// mutant rejected for the wrong reason cannot pass as the pin firing.
func underShortWatchdog(run func() error) error {
	previous := scenarioWatchdog
	scenarioWatchdog = 500 * time.Millisecond
	defer func() { scenarioWatchdog = previous }()
	return run()
}

// readFixture loads one committed fixture verbatim.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(scenarioFixtureGlob), name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return raw
}

// mutateFixture derives a single-delta mutant in memory: it walks a
// dot-separated path (numeric segments index `steps`) and replaces the value
// there. A path that does not resolve, or a mutation equal to the control's
// value, fails the probe as vacuous.
func mutateFixture(t *testing.T, raw []byte, path string, value any) []byte {
	t.Helper()
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decoding the control fixture: %v", err)
	}
	segments := strings.Split(path, ".")
	cursor := doc
	for _, segment := range segments[:len(segments)-1] {
		cursor = descend(t, cursor, segment, path)
	}
	last := segments[len(segments)-1]
	container, ok := cursor.(map[string]any)
	if !ok {
		t.Fatalf("path %s does not end in an object", path)
	}
	previous, present := container[last]
	if !present {
		t.Fatalf("path %s is absent from the control — a mutation must be a real delta", path)
	}
	if fmt.Sprint(previous) == fmt.Sprint(value) {
		t.Fatalf("path %s already holds %v — a mutation equal to the control proves nothing", path, value)
	}
	container[last] = value
	mutant, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-encoding the mutant: %v", err)
	}
	return mutant
}

func descend(t *testing.T, cursor any, segment, path string) any {
	t.Helper()
	switch node := cursor.(type) {
	case map[string]any:
		next, ok := node[segment]
		if !ok {
			t.Fatalf("path %s: %q is absent", path, segment)
		}
		return next
	case []any:
		index := 0
		if _, err := fmt.Sscanf(segment, "%d", &index); err != nil {
			t.Fatalf("path %s: %q is not an array index", path, segment)
		}
		if index >= len(node) {
			t.Fatalf("path %s: index %d is out of range (%d entries)", path, index, len(node))
		}
		return node[index]
	default:
		t.Fatalf("path %s: %q has no children", path, segment)
		return nil
	}
}

// TestScenarioDriverEnforcesTheFinalErrorElement pins the three halves of
// §23's "exactly one final error element that ends iteration" that no
// end-state assertion can catch.
//
// The driver used to record a terminal and `continue` ranging, keeping only
// the LATEST element. So a connector that yielded the expected reason twice,
// or delivered an event after it, or never ended iteration at all, passed
// every terminal fixture identically to a compliant one — the defining
// property of a test that cannot fail the contract it enforces.
//
// Each case drives the harness's own record path, which is where the guard
// lives, and asserts through `await` — the same call every expect step makes,
// and the one that consults recorded violations. That is what ties these
// guards to scenario failure rather than to a flag nobody reads.
func TestScenarioDriverEnforcesTheFinalErrorElement(t *testing.T) {
	terminal := func(reason eventfeed.TerminalReason) error {
		return &eventfeed.TerminalError{Reason: reason, Msg: "scenario selftest"}
	}
	// A check that always passes: any failure returned by await is therefore
	// the recorded violation, never the predicate.
	passes := func() (bool, string) { return true, "" }

	t.Run("a second error element is rejected", func(t *testing.T) {
		h := newScenarioHarness()
		defer h.close()
		h.recordTerminal(terminal(eventfeed.ReasonProtocolFatal))
		h.recordTerminal(terminal(eventfeed.ReasonProtocolFatal))

		err := underShortWatchdog(func() error { return h.await("anything", passes) })
		if err == nil {
			t.Fatal("the driver accepted two error elements; §23 gives the feed exactly one")
		}
		if !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("failed for the wrong reason: %v", err)
		}
	})

	t.Run("a delivery after the error element is rejected", func(t *testing.T) {
		h := newScenarioHarness()
		defer h.close()
		h.recordTerminal(terminal(eventfeed.ReasonProtocolFatal))
		h.recordDelivered(42)

		err := underShortWatchdog(func() error { return h.await("anything", passes) })
		if err == nil {
			t.Fatal("the driver accepted an event delivered after the final error element")
		}
		if !strings.Contains(err.Error(), "after its error element") {
			t.Fatalf("failed for the wrong reason: %v", err)
		}
	})

	t.Run("an iteration that never ends is rejected", func(t *testing.T) {
		h := newScenarioHarness()
		defer h.close()
		const state = "terminal"
		h.recordState(state)
		h.recordTerminal(terminal(eventfeed.ReasonProtocolFatal))
		// Deliberately no recordIterDone: the connector yielded the right
		// element and stayed open.

		d := &driver{h: h, sc: &scenario{Finally: scenarioFinally{
			State: state,
			Error: &errorExpect{Reason: string(eventfeed.ReasonProtocolFatal)},
		}}}
		err := underShortWatchdog(d.runFinally)
		if err == nil {
			t.Fatal("the driver accepted a terminal scenario whose iteration never ended")
		}
		if !strings.Contains(err.Error(), "end on its final error element") {
			t.Fatalf("failed for the wrong reason: %v", err)
		}
	})

	t.Run("the compliant shape still passes", func(t *testing.T) {
		h := newScenarioHarness()
		defer h.close()
		const state = "terminal"
		h.recordState(state)
		h.recordDelivered(41)
		h.recordTerminal(terminal(eventfeed.ReasonProtocolFatal))
		h.recordIterDone()

		d := &driver{h: h, sc: &scenario{Finally: scenarioFinally{
			State: state,
			Error: &errorExpect{Reason: string(eventfeed.ReasonProtocolFatal)},
		}}}
		if err := underShortWatchdog(d.runFinally); err != nil {
			t.Fatalf("the compliant shape must pass, else the three probes above prove nothing: %v", err)
		}
	})
}

// TestScenarioDriverScanScopesOccupancyHistoryToTheCurrentEra: the
// arrival-strict lookahead treats a pending expectBuffered(N) as satisfied
// when occupancy EVER reached N — including in an era the pointer has long
// left. The scan then reads through to a following expectCheckpoint and
// judges an arriving save against it, accepting an out-of-order checkpoint
// the fixture ordered after a buffer state that never recurred. History must
// be scoped to a boundary captured when the step pointer moves.
func TestScenarioDriverScanScopesOccupancyHistoryToTheCurrentEra(t *testing.T) {
	h := &scenarioHarness{}
	d := &driver{h: h, sc: &scenario{Steps: []scenarioStep{
		{Kind: "expectState", Payload: &expectStateStep{Is: "streaming"}},
		{Kind: "expectBuffered", Payload: &expectBufferedStep{Count: 1}},
		{Kind: "expectCheckpoint", Payload: &expectCheckpointStep{}},
	}}}
	// Occupancy reached 1 and left it while the pointer was on step 1; the
	// pointer then moves on to step 2 (expectBuffered) with occupancy 0.
	h.occupancy = 0
	h.occupancyHistory = []int{1, 0}
	d.beginStep(1, d.sc.Steps[1])

	h.mu.Lock()
	kind, where := d.currentStepLocked()
	h.mu.Unlock()
	if kind != "expectBuffered" {
		t.Fatalf("the scan read through expectBuffered(1) on occupancy from a previous era: an arriving save would be judged against %s and accepted out of order", where)
	}
}

// intp is a scripted-status literal.
func intp(s int) *int { return &s }

// TestDriverSeamClassificationIsPresenceKeyed: SPEC §6 fixes the adapter
// mapping the driver models — "a retryable outcome exhausted inside the seam
// whose last response carried a parsed Retry-After maps to
// throttled(retry_after) whatever its status, and one without maps to
// transient. A mapping keyed on status instead ... would honour the header at
// one retryable status and drop it at another, which is exactly the gate
// this section removes." And §6's parsing algorithm never yields zero
// ("zero is read as 'no usable value'"), so an unusable header is absence,
// not a throttled zero.
func TestDriverSeamClassificationIsPresenceKeyed(t *testing.T) {
	d := &driver{h: newScenarioHarness()}
	defer d.h.close()

	// A retryable non-429/503 status WITH a parsed Retry-After is throttled,
	// carrying the value.
	out := d.mintOutcomeFrom(mintRespond{Status: intp(502), Headers: map[string]string{"Retry-After": "3"}})
	var me *eventfeed.MintError
	if !errors.As(out.err, &me) || me.Kind != eventfeed.MintThrottled || me.RetryAfter != 3*time.Second {
		t.Fatalf("502 with Retry-After: 3 = %+v, want MintThrottled carrying 3s (presence-keyed, not status-keyed)", out.err)
	}
	// A 429 with NO usable header is transient: undefined falls to backoff.
	out = d.mintOutcomeFrom(mintRespond{Status: intp(429)})
	if !errors.As(out.err, &me) || me.Kind != eventfeed.MintTransient {
		t.Fatalf("bare 429 = %+v, want MintTransient (no parsed Retry-After)", out.err)
	}
	// Retry-After: 0 is undefined per the parsing algorithm (step 1 requires
	// > 0) — transient, never a throttled zero.
	out = d.mintOutcomeFrom(mintRespond{Status: intp(503), Headers: map[string]string{"Retry-After": "0"}})
	if !errors.As(out.err, &me) || me.Kind != eventfeed.MintTransient {
		t.Fatalf("503 with Retry-After: 0 = %+v, want MintTransient (zero is 'no usable value')", out.err)
	}

	// The poll lane, same mapping.
	po, err := d.pollOutcomeFrom(pollRespond{Status: intp(502), Headers: map[string]string{"Retry-After": "2"}})
	if err != nil {
		t.Fatalf("poll 502 with Retry-After: 2: %v", err)
	}
	var pe *eventfeed.PollError
	if !errors.As(po.err, &pe) || pe.Kind != eventfeed.PollThrottled || pe.RetryAfter != 2*time.Second {
		t.Fatalf("poll 502 with Retry-After: 2 = %+v, want PollThrottled carrying 2s", po.err)
	}
	po, err = d.pollOutcomeFrom(pollRespond{Status: intp(429)})
	if err != nil {
		t.Fatalf("bare poll 429: %v", err)
	}
	if !errors.As(po.err, &pe) || pe.Kind != eventfeed.PollTransient {
		t.Fatalf("bare poll 429 = %+v, want PollTransient", po.err)
	}
}

// TestDriverRetryAfterParseIsTheSpecAlgorithm: SPEC §6's parsing algorithm,
// which the driver claims to emulate — integer valid and > 0; else HTTP-date
// with max(0, date − now) rounded UP on a sub-second remainder, > 0; else
// undefined. Dates are judged against the harness's virtual clock.
func TestDriverRetryAfterParseIsTheSpecAlgorithm(t *testing.T) {
	d := &driver{h: newScenarioHarness()}
	defer d.h.close()

	// An HTTP-date carries whole seconds only, so the sub-second remainder
	// must live in NOW: park the virtual clock exactly 90.3s before a
	// whole-second instant, deterministically whatever the epoch's own
	// fraction.
	now0 := d.h.clock.Now()
	dateInstant := now0.Truncate(time.Second).Add(93 * time.Second)
	d.h.clock.Advance(dateInstant.Add(-90*time.Second - 300*time.Millisecond).Sub(now0))
	now := d.h.clock.Now()

	// A valid RFC 7231 HTTP-date 90.3s out parses, rounded UP to 91s.
	date := dateInstant.UTC().Format(http.TimeFormat)
	out := d.mintOutcomeFrom(mintRespond{Status: intp(429), Headers: map[string]string{"Retry-After": date}})
	var me *eventfeed.MintError
	if !errors.As(out.err, &me) || me.Kind != eventfeed.MintThrottled || me.RetryAfter != 91*time.Second {
		t.Fatalf("HTTP-date 90.3s out = %+v, want MintThrottled carrying 91s (sub-second remainder rounds UP)", out.err)
	}
	// A date already passed is undefined — max(0, ·) is not > 0.
	past := now.Add(-time.Minute).UTC().Format(http.TimeFormat)
	for name, header := range map[string]string{
		"past date": past,
		"negative":  "-5",
		"malformed": "soon",
	} {
		out = d.mintOutcomeFrom(mintRespond{Status: intp(429), Headers: map[string]string{"Retry-After": header}})
		if !errors.As(out.err, &me) || me.Kind != eventfeed.MintTransient || me.RetryAfter != 0 {
			t.Fatalf("%s Retry-After = %+v, want MintTransient with no value (undefined)", name, out.err)
		}
	}
}

// TestDriverRedirectRefusalUsesTheRealPredicate: the driver models the
// Layer-1 poll adapter, whose per-hop redirect rule IS checkContinuation —
// "a redirect Location failing per-hop validation inside a poll seam call
// (Continuation and Resume URL Validation)". Classifying by an ad-hoc origin
// comparison instead re-implements the predicate, and diverges at its edges:
// a Location that cannot be reduced to an origin at all ("https://", a
// relative path) is a REFUSAL under the real rule — the seam refuses the
// hop, zero requests to the failing URL — where the ad-hoc parse turned it
// into a scenario error, as if the fixture were malformed rather than the
// peer hostile.
func TestDriverRedirectRefusalUsesTheRealPredicate(t *testing.T) {
	d := &driver{h: newScenarioHarness()}
	defer d.h.close()

	for name, location := range map[string]string{
		"unreducible location": "https://",
		"relative location":    "/events.json",
	} {
		po, err := d.redirectRefusalFrom(map[string]string{"Location": location})
		if err != nil {
			t.Fatalf("%s: a hostile Location must classify as a refusal, not fail the scenario: %v", name, err)
		}
		var pe *eventfeed.PollError
		if !errors.As(po.err, &pe) || pe.Kind != eventfeed.PollRedirectRefused {
			t.Fatalf("%s = %+v, want PollRedirectRefused", name, po.err)
		}
	}
	// And the refusal decision agrees with the predicate on the ordinary
	// cross-origin case.
	po, err := d.redirectRefusalFrom(map[string]string{"Location": "https://attacker.example.com/events.json"})
	if err != nil {
		t.Fatalf("cross-origin Location: %v", err)
	}
	var pe *eventfeed.PollError
	if !errors.As(po.err, &pe) || pe.Kind != eventfeed.PollRedirectRefused {
		t.Fatalf("cross-origin Location = %+v, want PollRedirectRefused", po.err)
	}
}

// TestDriverRetryAfterParseMatchesTheSDKParser: the SDK's parseRetryAfter
// (go/pkg/basecamp/client.go) pins the delta-seconds form to RFC 9110's
// 1*DIGIT — no sign — parses through int64 so the verdict cannot vary with
// the platform's int width, and clamps a representable-but-unschedulable
// value to the portable MaxInt32-seconds ceiling. The driver models the same
// §6 algorithm and must match: a signed "+5" is not a delay at all, digits
// too large for int64 are malformed, and a huge representable value
// saturates instead of overflowing the duration multiply into garbage.
func TestDriverRetryAfterParseMatchesTheSDKParser(t *testing.T) {
	d := &driver{h: newScenarioHarness()}
	defer d.h.close()
	var me *eventfeed.MintError

	// Signed values are not 1*DIGIT: undefined, never throttled.
	for _, header := range []string{"+5", "-5", " 5", "5 "} {
		out := d.mintOutcomeFrom(mintRespond{Status: intp(429), Headers: map[string]string{"Retry-After": header}})
		if !errors.As(out.err, &me) || me.Kind != eventfeed.MintTransient || me.RetryAfter != 0 {
			t.Fatalf("Retry-After %q = %+v, want MintTransient with no value (not 1*DIGIT)", header, out.err)
		}
	}
	// Digits too large for int64 are malformed — undefined on every platform.
	out := d.mintOutcomeFrom(mintRespond{Status: intp(429), Headers: map[string]string{"Retry-After": "18446744073709551616"}})
	if !errors.As(out.err, &me) || me.Kind != eventfeed.MintTransient || me.RetryAfter != 0 {
		t.Fatalf("over-int64 digits = %+v, want MintTransient (malformed)", out.err)
	}
	// Representable but unschedulable saturates at the SDK's portable
	// ceiling; the naive duration multiply would overflow into garbage.
	out = d.mintOutcomeFrom(mintRespond{Status: intp(429), Headers: map[string]string{"Retry-After": "99999999999"}})
	if !errors.As(out.err, &me) || me.Kind != eventfeed.MintThrottled || me.RetryAfter != time.Duration(math.MaxInt32)*time.Second {
		t.Fatalf("huge representable value = %+v (retryAfter %v), want MintThrottled clamped to MaxInt32 seconds", out.err, me.RetryAfter)
	}
}
