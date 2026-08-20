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
	"fmt"
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
			path:    "steps.13.expectConnect.url",
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

// underShortWatchdog runs a scenario that is EXPECTED to fail under a short
// rendezvous window. A hostile scenario often fails by never satisfying a
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
