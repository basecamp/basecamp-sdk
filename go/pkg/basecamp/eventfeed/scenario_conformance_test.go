// Tier-2 conformance driver: every fixture in `conformance/event-feed/`
// executed against the real connector through its seams.
//
// Strictness follows the family README's per-action-class rules exactly.
// Checkpoint saves and outbound frames are ARRIVAL-STRICT: each is judged, in
// the critical section that records it, against the step the script is on, and
// one arriving anywhere but its matching expect step fails the scenario there
// and then. `beginStep`, `currentStepLocked` and `await`'s advance are what
// make that survivable across a step boundary; a save additionally carries an
// ordering witness (how many events had been delivered when it was made), and
// an outbound frame no expect step matched still fails at `finally`. Mint and
// poll seam calls are PARKED: the call blocks inside the seam and its scripted
// response is released only when the driver reaches the matching expect step,
// so an early call cannot fail on arrival and one still parked at the end
// fails. Observation directives are rendezvous assertions under a wall-clock
// watchdog while virtual time stays frozen.
package eventfeed_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// scenarioFixtureGlob discovers the family's fixtures by directory glob, so a
// fixture added by a later PR is executed without touching this driver. The
// sibling `pin-probes/` directory is deliberately NOT globbed: probes are
// inputs to the schema gate, not scenarios.
const scenarioFixtureGlob = "../../../../conformance/event-feed/fixtures/*.json"

// TestEventFeedScenarioConformance runs every tier-2 scenario fixture, one
// subtest per fixture file.
func TestEventFeedScenarioConformance(t *testing.T) {
	paths, err := filepath.Glob(scenarioFixtureGlob)
	if err != nil {
		t.Fatalf("globbing %s: %v", scenarioFixtureGlob, err)
	}
	if len(paths) == 0 {
		t.Fatalf("no scenario fixtures matched %s — a glob that matches nothing proves nothing", scenarioFixtureGlob)
	}
	slices.Sort(paths)
	t.Logf("tier-2 event feed scenarios: %d fixtures discovered under %s", len(paths), filepath.Dir(scenarioFixtureGlob))
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			if err := runScenarioBytes(raw, filepath.Base(path)); err != nil {
				t.Fatalf("%v", err)
			}
		})
	}
}

// runScenarioBytes loads one fixture against a fresh harness and drives it.
func runScenarioBytes(raw []byte, file string) error {
	h := newScenarioHarness()
	defer h.close()
	substituted, err := substitutePlaceholders(raw, h)
	if err != nil {
		return err
	}
	sc, err := parseScenario(substituted, file)
	if err != nil {
		return err
	}
	return runScenario(h, sc)
}

// runScenario drives one loaded scenario to its `finally` block.
func runScenario(h *scenarioHarness, sc *scenario) (err error) {
	h.connectPlan = planConnects(sc)
	conn, cerr := h.newConnector(sc.Config)
	if cerr != nil {
		return fmt.Errorf("connector construction: %w", cerr)
	}
	// The driver is bound to the harness BEFORE the iteration starts: the
	// arrival rule judges against its step script, and a connector action
	// observed before there is a script to judge it against would escape.
	d := &driver{h: h, sc: sc}
	h.attach(d)
	ctx, cancel := context.WithCancel(context.Background())
	iteration := make(chan struct{})
	go func() {
		defer close(iteration)
		for event, iterErr := range conn.Events(ctx) {
			if iterErr != nil {
				h.recordTerminal(iterErr)
				continue
			}
			h.recordDelivered(event.ID)
		}
		h.recordIterDone()
	}()
	defer func() {
		cancel()
		select {
		case <-iteration:
		case <-time.After(scenarioWatchdog):
			if err == nil {
				err = errors.New("the feed iteration did not end after cancellation")
			}
		}
	}()

	for i, step := range sc.Steps {
		d.beginStep(i, step)
		if stepErr := d.runStep(step); stepErr != nil {
			return fmt.Errorf("step %d (%s): %w", i+1, step.Kind, stepErr)
		}
	}
	d.h.enterStep(len(sc.Steps))
	return d.runFinally()
}

// planConnects pre-scans the scenario's connect outcomes so the Nth handshake
// is answered as the Nth `expectConnect` scripts it. Dials happen strictly in
// order (each is gated behind its own parked mint), so the indices line up.
func planConnects(sc *scenario) []connectPlanEntry {
	var plan []connectPlanEntry
	for _, step := range sc.Steps {
		connect, ok := step.Payload.(*expectConnectStep)
		if !ok {
			continue
		}
		entry := connectPlanEntry{kind: "accept"}
		switch {
		case connect.Outcome == nil:
		case connect.Outcome.Refuse != "":
			entry.kind = "refuse"
		case connect.Outcome.RedirectTo != "":
			entry.kind, entry.target = "redirect", connect.Outcome.RedirectTo
		}
		plan = append(plan, entry)
	}
	return plan
}

// driver executes one scenario's steps against the harness.
type driver struct {
	h  *scenarioHarness
	sc *scenario

	peer          *cablePeer
	connectsTaken int
	identifier    string
	// requiredDeliveries is the cumulative delivery count the script has
	// demanded so far — the ordering witness a checkpoint step is matched
	// against.
	requiredDeliveries int
}

// --- the arrival rule's step pointer -------------------------------------

// beginStep moves the arrival rule's pointer onto step i before the step runs.
//
// A DRIVER ACTION — serving a frame, closing or severing the socket, advancing
// or firing the clock — hands the pointer straight on to the next step instead.
// The connector cannot react before the driver acts, so everything an action
// provokes belongs to what FOLLOWS it; judging that reaction against the action
// step itself would fail every fixture whose `serve` is answered with a
// subscribe. It is also the only way an unexecuted action step keeps its teeth:
// the lookahead below stops dead at one, which is exactly what makes a
// subscribe written at dial — before the `serve welcome` that legalizes it —
// an early arrival rather than a match on the step after.
func (d *driver) beginStep(i int, step scenarioStep) {
	d.h.enterStep(i)
	if stepIsDriverAction(step) {
		d.h.enterStep(i + 1)
	}
}

// currentStepLocked names the step an arriving action is judged against: the
// pointer, advanced over every step the recorded history has ALREADY satisfied
// but the driver has not consumed yet.
//
// That lookahead is the other half of the atomic handoff. `await` advances the
// pointer in the critical section that satisfies a rendezvous, which covers the
// driver blocked ON that rendezvous; the lookahead covers the driver that has
// not yet reached it — a page released at `expectPoll` is delivered and then
// checkpointed by one causal chain in the connector, and the driver need not
// have run its `expectDelivered` step by the time the save lands. Scanning is
// safe because a step's condition is checked BEFORE the arriving action is
// recorded: the matching step is still unsatisfied, so the scan stops there
// rather than running past it.
//
// Callers hold h.mu.
func (d *driver) currentStepLocked() (kind, where string) {
	for i := d.h.cursor; i < len(d.sc.Steps); i++ {
		step := d.sc.Steps[i]
		if !d.stepSatisfiedLocked(step) {
			return step.Kind, fmt.Sprintf("step %d (%s)", i+1, step.Kind)
		}
	}
	return "", "the finally block"
}

// stepIsDriverAction reports whether a directive is something the DRIVER does
// rather than something it waits for.
func stepIsDriverAction(step scenarioStep) bool {
	switch step.Payload.(type) {
	case *serveStep, *serverCloseStep, *severStep, *advanceStep, *fireTimerStep:
		return true
	default:
		return false
	}
}

// stepSatisfiedLocked reports whether the recorded history already satisfies a
// step, without consuming anything. It is deliberately a touch more permissive
// than the executor's own rendezvous — it asks "has enough happened" rather
// than "does it match" — because its job is only to say where the script has
// got to. Erring permissive lets the pointer run on; erring strict would stall
// it and fail a compliant connector, so the arms that gate the two
// arrival-strict classes (expectCheckpoint, expectSubscribe/expectClientClose)
// are the ones written tight.
//
// A directive with no arm is a driver action, and the scan stops at one.
//
//nolint:gocyclo,cyclop // one arm per directive, mirroring runStep's dispatch
func (d *driver) stepSatisfiedLocked(step scenarioStep) bool {
	h := d.h
	switch payload := step.Payload.(type) {
	case *expectMintStep:
		return len(h.pendingMints) > 0
	case *expectConnectStep:
		return len(h.connects) > d.connectsTaken
	case *expectSubscribeStep, *expectClientCloseStep:
		return d.peer != nil && d.peer.taken < len(d.peer.frames)
	case *expectPollStep:
		return len(h.pendingPolls) > 0
	case *exactIDs:
		return len(h.delivered) >= len(payload.Exact)
	case *expectCheckpointStep:
		return h.savesTaken < len(h.saves)
	case *timerSet:
		return maps.Equal(timerCounts(h.clock), payload.Exact)
	case *expectStateStep:
		return h.state == payload.Is
	case *errorExpect:
		return h.terminal != nil
	case *expectGapStep:
		return h.gapsTaken < len(h.gaps)
	case *expectBufferedStep:
		// History is scoped to the current era — the boundary captured when
		// the step pointer last moved — so occupancy reached and left before
		// this era cannot satisfy a pending expectBuffered and let the scan
		// read through to an arrival-strict step behind it.
		return h.occupancy == payload.Count || slices.Contains(h.occupancyHistory[h.occupancyEra:], payload.Count)
	case *expectSignalStep:
		return h.signalsTaken < len(h.signals)
	case *exactInvocations:
		return len(h.invocations) >= len(payload.Exact)
	case *expectSaveFailedStep:
		return h.saveFailuresTaken < h.saveFailures
	case *expectPositionRejectedStep:
		return h.posRejectedTaken < len(h.positionRejected)
	case *expectDisconnectedInvalidFrameStep:
		return h.invalidFramesTaken < h.invalidFrames
	default:
		return false
	}
}

// runStep dispatches one directive.
//
// table: flat dispatch is what keeps the mapping auditable.
//
//nolint:gocyclo,cyclop // one arm per directive, mirroring the schema's step
func (d *driver) runStep(step scenarioStep) error {
	switch payload := step.Payload.(type) {
	case *expectMintStep:
		return d.expectMint(payload)
	case *expectConnectStep:
		return d.expectConnect(payload)
	case *serveStep:
		return d.serve(payload)
	case *serverCloseStep:
		return d.serverClose(payload)
	case *severStep:
		return d.sever()
	case *expectSubscribeStep:
		return d.expectSubscribe(payload)
	case *expectPollStep:
		return d.expectPoll(payload)
	case *expectClientCloseStep:
		return d.expectClientClose()
	case *advanceStep:
		// Plain Advance: no fixture scripts a firing that arms a follow-on
		// timer due inside the same window — 05, the suite's only advance,
		// deliberately configures staleness and repair-poll out of it, so the
		// window fires nothing. A script that did want a chained firing would
		// pass feedtest.Clock.AdvanceSettling the rendezvous for the arming,
		// since the connector arms on its own goroutine.
		d.h.clock.Advance(millis(payload.Ms))
		return nil
	case *fireTimerStep:
		return d.fireTimer(payload)
	case *exactIDs:
		return d.expectDelivered(payload.Exact)
	case *expectCheckpointStep:
		return d.expectCheckpoint(payload.Position)
	case *timerSet:
		return d.awaitTimers(payload.Exact)
	case *expectStateStep:
		return d.awaitState(payload.Is)
	case *errorExpect:
		return d.awaitTerminal(payload.Reason)
	case *expectGapStep:
		return d.expectGap(payload.EpochAfterID)
	case *expectBufferedStep:
		return d.expectBuffered(payload.Count)
	case *expectSignalStep:
		return d.expectSignal(payload)
	case *exactInvocations:
		return d.expectInvocations(payload.Exact)
	case *expectSaveFailedStep:
		return d.expectSaveFailed()
	case *expectPositionRejectedStep:
		return d.expectPositionRejected(payload.Kind)
	case *expectDisconnectedInvalidFrameStep:
		return d.expectInvalidFrameDisconnect()
	default:
		return fmt.Errorf("the driver has no executor for directive %q", step.Kind)
	}
}

// --- protocol actions ----------------------------------------------------

func (d *driver) expectMint(step *expectMintStep) error {
	var call *mintCall
	err := d.h.await("a mint seam call", func() (bool, string) {
		if len(d.h.pendingMints) == 0 {
			return false, ""
		}
		call = d.h.pendingMints[0]
		d.h.pendingMints = d.h.pendingMints[1:]
		return true, ""
	})
	if err != nil {
		return err
	}
	call.release <- d.mintOutcomeFrom(step.Respond)
	return nil
}

// expectConnect matches the next cable dial. It takes the dial and installs
// the accepted socket UNDER h.mu, on the satisfying call: `d.peer` and
// `d.connectsTaken` are read by the arrival rule's lookahead from the
// connector's goroutines, so they cannot be settled after the rendezvous
// returns.
func (d *driver) expectConnect(step *expectConnectStep) error {
	index := d.connectsTaken
	return d.h.await(fmt.Sprintf("a cable dial to %s", step.URL), func() (bool, string) {
		if len(d.h.connects) <= index {
			return false, ""
		}
		attempt := d.h.connects[index]
		if attempt.url != step.URL {
			return false, fmt.Sprintf("the connector dialed\n  %s\nwant the mint's url verbatim\n  %s", attempt.url, step.URL)
		}
		if attempt.kind == "accept" {
			if attempt.peer == nil {
				return false, ""
			}
			d.peer = attempt.peer
		}
		d.connectsTaken++
		return true, ""
	})
}

func (d *driver) serve(step *serveStep) error {
	if d.peer == nil {
		return errors.New("no cable connection is open to serve on")
	}
	frame, err := serveFrameBytes(step, d.identifier)
	if err != nil {
		return err
	}
	return d.h.writeFrame(d.peer, frame)
}

func (d *driver) serverClose(step *serverCloseStep) error {
	if d.peer == nil {
		return errors.New("no cable connection is open to close")
	}
	return d.h.closePeer(d.peer, step.Code, step.Reason)
}

// sever drops the TCP connection abruptly: no close frame, no disconnect
// frame.
func (d *driver) sever() error {
	if d.peer == nil {
		return errors.New("no cable connection is open to sever")
	}
	return d.peer.conn.CloseNow()
}

func (d *driver) expectSubscribe(step *expectSubscribeStep) error {
	frame, err := d.nextClientFrame("the subscribe command")
	if err != nil {
		return err
	}
	if frame.kind != "text" {
		return fmt.Errorf("the connector closed the socket where the script expects a subscribe command")
	}
	var command struct {
		Command    string `json:"command"`
		Identifier string `json:"identifier"`
	}
	if err := json.Unmarshal(frame.data, &command); err != nil {
		return fmt.Errorf("the outbound frame is not a cable command: %w", err)
	}
	if command.Command != "subscribe" {
		return fmt.Errorf("the connector sent a %q command; only `subscribe` is ever sent", command.Command)
	}
	identifier := map[string]string{}
	if err := json.Unmarshal([]byte(command.Identifier), &identifier); err != nil {
		return fmt.Errorf("the subscribe identifier is not a JSON object: %w", err)
	}
	want := map[string]string{"channel": step.Channel}
	maps.Copy(want, step.Params)
	if !maps.Equal(identifier, want) {
		return fmt.Errorf("subscribe identifier %v, want %v", identifier, want)
	}
	if step.IdenticalToPrevious != nil {
		if err := d.assertIdenticalSubscribe(frame.data); err != nil {
			return err
		}
	}
	d.identifier = command.Identifier
	return nil
}

// assertIdenticalSubscribe pins byte-identity with the previous subscribe on
// this connection (the retransmit-absorption case).
func (d *driver) assertIdenticalSubscribe(current []byte) error {
	d.h.mu.Lock()
	defer d.h.mu.Unlock()
	for i := d.peer.taken - 2; i >= 0; i-- {
		previous := d.peer.frames[i]
		if previous.kind != "text" {
			continue
		}
		if !slices.Equal(previous.data, current) {
			return fmt.Errorf("the retransmitted subscribe differs from the previous one:\n  %s\n  %s", previous.data, current)
		}
		return nil
	}
	return errors.New("identicalToPrevious has no previous subscribe on this connection")
}

func (d *driver) expectPoll(step *expectPollStep) error {
	var call *pollCall
	err := d.h.await("a poll seam call", func() (bool, string) {
		if len(d.h.pendingPolls) == 0 {
			return false, ""
		}
		call = d.h.pendingPolls[0]
		d.h.pendingPolls = d.h.pendingPolls[1:]
		return true, ""
	})
	if err != nil {
		return err
	}
	if err := assertPollTarget(step, call); err != nil {
		return err
	}
	outcome, err := d.pollOutcomeFrom(step.Respond)
	if err != nil {
		return err
	}
	call.release <- outcome
	return nil
}

// assertPollTarget pins what the poll requested: an absolute continuation or
// resume URL followed verbatim, or the derived query params.
func assertPollTarget(step *expectPollStep, call *pollCall) error {
	if step.URL != "" {
		if call.cursor.PageURL != step.URL {
			return fmt.Errorf("the poll targeted %+v, want the URL followed verbatim: %s", call.cursor, step.URL)
		}
		return nil
	}
	if call.cursor.PageURL != "" {
		return fmt.Errorf("the poll followed the continuation %s where the script pins a query", call.cursor.PageURL)
	}
	got := derivedQuery(call)
	if step.exactPin {
		if !maps.Equal(got, step.params) {
			return fmt.Errorf("poll query %v, want exactly %v", got, step.params)
		}
		return nil
	}
	for name, want := range step.params {
		if got[name] != want {
			return fmt.Errorf("poll query %v, want %s=%s", got, name, want)
		}
	}
	return nil
}

// derivedQuery renders the query the poll seam call would carry on the wire:
// the cursor's position/since plus the canonical comma-joined filter params.
func derivedQuery(call *pollCall) map[string]string {
	query := map[string]string{}
	if call.cursor.Position != "" {
		query["position"] = call.cursor.Position
	}
	if call.cursor.Since != "" {
		query["since"] = call.cursor.Since
	}
	if len(call.filters.Types) > 0 {
		query["types"] = strings.Join(call.filters.Types, ",")
	}
	if len(call.filters.Buckets) > 0 {
		query["buckets"] = joinIDs(call.filters.Buckets)
	}
	if len(call.filters.Creators) > 0 {
		query["creators"] = joinIDs(call.filters.Creators)
	}
	return query
}

func joinIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func (d *driver) expectClientClose() error {
	frame, err := d.nextClientFrame("the connector's explicit close of the open socket")
	if err != nil {
		return err
	}
	if frame.kind != "close" {
		return fmt.Errorf("the connector sent a frame where the script expects its close: %s", frame.data)
	}
	return nil
}

// nextClientFrame pops the next outbound action observed on the current
// socket, in order.
func (d *driver) nextClientFrame(what string) (clientFrame, error) {
	if d.peer == nil {
		return clientFrame{}, errors.New("no cable connection is open")
	}
	var frame clientFrame
	err := d.h.await(what, func() (bool, string) {
		if d.peer.taken >= len(d.peer.frames) {
			if d.peer.dead {
				return false, "the socket died with no further outbound frame"
			}
			return false, ""
		}
		frame = d.peer.frames[d.peer.taken]
		d.peer.taken++
		return true, ""
	})
	return frame, err
}

// --- time ----------------------------------------------------------------

func (d *driver) fireTimer(step *fireTimerStep) error {
	if err := d.awaitTimerArmed(step.Kind); err != nil {
		return err
	}
	delay, ok := d.h.clock.FireTimer(step.Kind)
	if !ok {
		return fmt.Errorf("no %s timer is outstanding: outstanding %v", step.Kind, d.h.clock.Outstanding())
	}
	if step.AssertDelayMs == nil {
		return nil
	}
	low, high := millis(step.AssertDelayMs.Min), millis(step.AssertDelayMs.Max)
	if delay < low || delay > high {
		return fmt.Errorf("the %s timer was armed for %s, want [%s, %s]", step.Kind, delay, low, high)
	}
	return nil
}

// awaitTimerArmed waits for a timer of the kind to exist. Timers are armed on
// the connector's goroutine, so a fireTimer directive that races the arming
// would otherwise fail spuriously.
func (d *driver) awaitTimerArmed(kind string) error {
	return d.pollUntil(fmt.Sprintf("the %s timer to be armed", kind), func() bool {
		return slices.Contains(d.h.clock.Outstanding(), kind)
	})
}

// awaitTimers waits for the exact outstanding-timer set.
func (d *driver) awaitTimers(want map[string]int) error {
	var got map[string]int
	err := d.pollUntil(fmt.Sprintf("the outstanding timer set %v", want), func() bool {
		got = timerCounts(d.h.clock)
		return maps.Equal(got, want)
	})
	if err != nil {
		return fmt.Errorf("outstanding timers %v, want exactly %v", got, want)
	}
	return nil
}

func timerCounts(clock *feedtest.Clock) map[string]int {
	counts := map[string]int{}
	for _, name := range clock.Outstanding() {
		counts[name]++
	}
	return counts
}

// pollUntil is the bounded wall-clock watchdog for the clock registry, which
// is not part of the harness's broadcast.
func (d *driver) pollUntil(what string, satisfied func() bool) error {
	deadline := time.Now().Add(scenarioWatchdog)
	for {
		if satisfied() {
			return nil
		}
		if err := d.h.violation(); err != nil {
			return fmt.Errorf("while waiting for %s: %w", what, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s\n%s", scenarioWatchdog, what, d.h.describe())
		}
		time.Sleep(time.Millisecond)
	}
}

// --- observations --------------------------------------------------------

func (d *driver) expectDelivered(want []int64) error {
	err := d.h.await(fmt.Sprintf("delivered ids %v", want), func() (bool, string) {
		if !prefixOfIDs(d.h.delivered, want) {
			return false, fmt.Sprintf("delivered %v, want exactly %v", d.h.delivered, want)
		}
		return len(d.h.delivered) == len(want), ""
	})
	if err != nil {
		return err
	}
	d.requiredDeliveries = len(want)
	return nil
}

func (d *driver) expectCheckpoint(position string) error {
	return d.h.await(fmt.Sprintf("a checkpoint save of %s", position), func() (bool, string) {
		if d.h.savesTaken >= len(d.h.saves) {
			return false, ""
		}
		save := d.h.saves[d.h.savesTaken]
		if save.position != position {
			return false, fmt.Sprintf("saved %q, want %q", save.position, position)
		}
		if save.deliveredAt < d.requiredDeliveries {
			return false, fmt.Sprintf(
				"the save of %q arrived after %d deliveries, but the script requires %d first — delivery strictly precedes that page's checkpoint",
				save.position, save.deliveredAt, d.requiredDeliveries)
		}
		d.h.savesTaken++
		return true, ""
	})
}

func (d *driver) awaitState(want string) error {
	return d.h.await(fmt.Sprintf("state %q", want), func() (bool, string) {
		return d.h.state == want, ""
	})
}

func (d *driver) awaitTerminal(reason string) error {
	return d.h.await(fmt.Sprintf("Terminal(%s)", reason), func() (bool, string) {
		if d.h.terminal == nil {
			return false, ""
		}
		if string(d.h.terminal.Reason) != reason {
			return false, fmt.Sprintf("terminal reason %q, want %q (%v)", d.h.terminal.Reason, reason, d.h.terminal)
		}
		return true, ""
	})
}

// awaitIterationEnded waits for the consumer's range over Events to return of
// its own accord — the observable half of "the error element ends iteration".
// Bounded by the same watchdog as every other await, so a connector that stays
// open fails here instead of hanging the scenario.
func (d *driver) awaitIterationEnded() error {
	return d.h.await("the iteration to end on its final error element", func() (bool, string) {
		return d.h.iterDone, ""
	})
}

func (d *driver) expectGap(epochAfterID int64) error {
	return d.h.await(fmt.Sprintf("Observer.gap(%d)", epochAfterID), func() (bool, string) {
		if d.h.gapsTaken >= len(d.h.gaps) {
			return false, ""
		}
		if got := d.h.gaps[d.h.gapsTaken]; got != epochAfterID {
			return false, fmt.Sprintf("gap epoch_after_id %d, want %d", got, epochAfterID)
		}
		d.h.gapsTaken++
		return true, ""
	})
}

// expectBuffered waits for the state-machine-owned live buffer's CURRENT
// occupancy — events admitted minus events dropped — to equal count. Every
// change is recorded, so a value reached and left again while the driver was
// scheduled out still satisfies the rendezvous.
func (d *driver) expectBuffered(count int) error {
	// The scan boundary is the step pointer's era, not this function's start:
	// occupancy reached and left between the pointer entering this step and
	// the driver getting here is this step's to see (excluding it flaked),
	// while anything earlier belongs to a previous era and must not satisfy.
	d.h.mu.Lock()
	from := d.h.occupancyEra
	d.h.mu.Unlock()
	return d.h.await(fmt.Sprintf("live buffer occupancy %d", count), func() (bool, string) {
		if d.h.occupancy == count {
			return true, ""
		}
		return slices.Contains(d.h.occupancyHistory[from:], count), ""
	})
}

func (d *driver) expectSignal(step *expectSignalStep) error {
	return d.h.await(fmt.Sprintf("the %s signal", step.Kind), func() (bool, string) {
		if d.h.signalsTaken >= len(d.h.signals) {
			return false, ""
		}
		signal := d.h.signals[d.h.signalsTaken]
		if bad := matchSignal(step, signal); bad != "" {
			return false, bad
		}
		d.h.signalsTaken++
		return true, ""
	})
}

// matchSignal compares one raised signal against its expectation, returning a
// failure description or "".
func matchSignal(step *expectSignalStep, signal eventfeed.Signal) string {
	switch got := signal.(type) {
	case eventfeed.BufferOverflow:
		if step.Kind != "bufferOverflow" {
			return fmt.Sprintf("raised %T, want a %s signal", got, step.Kind)
		}
		if !slices.Equal(got.DroppedIDs, step.DroppedIDs) {
			return fmt.Sprintf("dropped ids %v, want exactly %v", got.DroppedIDs, step.DroppedIDs)
		}
		if got.DroppedCount != *step.DroppedCount {
			return fmt.Sprintf("dropped count %d, want %d", got.DroppedCount, *step.DroppedCount)
		}
	case eventfeed.FeedGap:
		if step.Kind != "feedGap" {
			return fmt.Sprintf("raised %T, want a %s signal", got, step.Kind)
		}
		if got.EpochAfterID != *step.EpochAfterID {
			return fmt.Sprintf("epoch_after_id %d, want %d", got.EpochAfterID, *step.EpochAfterID)
		}
	default:
		return fmt.Sprintf("unknown signal type %T", got)
	}
	return ""
}

func (d *driver) expectInvocations(want []invocation) error {
	return d.h.await(fmt.Sprintf("handler invocations %v", want), func() (bool, string) {
		if !prefixOfInvocations(d.h.invocations, want) {
			return false, fmt.Sprintf("handler invocations %v, want exactly %v", d.h.invocations, want)
		}
		return len(d.h.invocations) == len(want), ""
	})
}

func (d *driver) expectSaveFailed() error {
	return d.h.await("Observer.checkpoint_save_failed", func() (bool, string) {
		if d.h.saveFailuresTaken >= d.h.saveFailures {
			return false, ""
		}
		d.h.saveFailuresTaken++
		return true, ""
	})
}

func (d *driver) expectPositionRejected(kind string) error {
	return d.h.await(fmt.Sprintf("Observer.position_rejected(%s)", kind), func() (bool, string) {
		if d.h.posRejectedTaken >= len(d.h.positionRejected) {
			return false, ""
		}
		if got := d.h.positionRejected[d.h.posRejectedTaken]; got != kind {
			return false, fmt.Sprintf("position_rejected kind %q, want %q", got, kind)
		}
		d.h.posRejectedTaken++
		return true, ""
	})
}

func (d *driver) expectInvalidFrameDisconnect() error {
	return d.h.await("Observer.disconnected carrying the invalid-frame indication", func() (bool, string) {
		if d.h.invalidFramesTaken >= d.h.invalidFrames {
			return false, ""
		}
		d.h.invalidFramesTaken++
		return true, ""
	})
}

// --- finally -------------------------------------------------------------

// runFinally evaluates the end-of-scenario assertions, then the residue
// checks that make the strict interleave total: a parked seam call never
// matched, an outbound frame never matched, a save never matched, an
// unconsumed store script, an unmatched dial, or any recorded violation.
func (d *driver) runFinally() error {
	fin := d.sc.Finally
	if err := d.awaitState(fin.State); err != nil {
		return fmt.Errorf("finally.state: %w", err)
	}
	if fin.Error != nil {
		if err := d.awaitTerminal(fin.Error.Reason); err != nil {
			return fmt.Errorf("finally.error: %w", err)
		}
		// The element is only the FINAL one if iteration actually ended on it.
		// Without this, a connector that yields the right terminal and then
		// stays open passes every terminal fixture: the deferred cancel ends
		// the range afterwards, so nothing downstream can tell an iteration
		// that ended by itself from one that had to be cancelled.
		if err := d.awaitIterationEnded(); err != nil {
			return fmt.Errorf("finally.error: %w", err)
		}
	} else if terminal := d.h.terminalError(); terminal != nil {
		return fmt.Errorf("finally: the feed terminated, but the fixture states no error: %w", terminal)
	}
	if fin.Delivered != nil {
		if err := d.expectDelivered(fin.Delivered.Exact); err != nil {
			return fmt.Errorf("finally.delivered: %w", err)
		}
	}
	if fin.HandlerInvocations != nil {
		if err := d.expectInvocations(fin.HandlerInvocations.Exact); err != nil {
			return fmt.Errorf("finally.handlerInvocations: %w", err)
		}
	}
	if fin.Checkpoints != nil {
		if err := d.assertCheckpointLedger(fin.Checkpoints.Exact); err != nil {
			return fmt.Errorf("finally.checkpoints: %w", err)
		}
	}
	if fin.Timers != nil {
		if err := d.awaitTimers(fin.Timers.Exact); err != nil {
			return fmt.Errorf("finally.timers: %w", err)
		}
	}
	if err := d.assertCounts(fin); err != nil {
		return err
	}
	if err := d.assertSocket(fin.Socket); err != nil {
		return fmt.Errorf("finally.socket: %w", err)
	}
	return d.assertNoResidue()
}

func (d *driver) assertCheckpointLedger(want []string) error {
	return d.h.await(fmt.Sprintf("the checkpoint ledger %v", want), func() (bool, string) {
		got := make([]string, len(d.h.saves))
		for i, save := range d.h.saves {
			got[i] = save.position
		}
		if !prefixOfStrings(got, want) {
			return false, fmt.Sprintf("checkpoint ledger %v, want exactly %v", got, want)
		}
		return len(got) == len(want), ""
	})
}

func (d *driver) assertCounts(fin scenarioFinally) error {
	d.h.mu.Lock()
	mints, polls, connects := d.h.mintCalls, d.h.pollCalls, len(d.h.connects)
	d.h.mu.Unlock()
	for _, check := range []struct {
		name string
		want *int
		got  int
	}{
		{"finally.mintCount", fin.MintCount, mints},
		{"finally.connectCount", fin.ConnectCount, connects},
		{"finally.pollCount", fin.PollCount, polls},
	} {
		if check.want != nil && check.got != *check.want {
			return fmt.Errorf("%s: %d seam calls, want %d", check.name, check.got, *check.want)
		}
	}
	return nil
}

func (d *driver) assertSocket(disposition string) error {
	if disposition == "" {
		return nil
	}
	d.h.mu.Lock()
	dialed := len(d.h.connects)
	var last *cablePeer
	if len(d.h.peers) > 0 {
		last = d.h.peers[len(d.h.peers)-1]
	}
	d.h.mu.Unlock()
	switch disposition {
	case "none":
		if dialed != 0 {
			return fmt.Errorf("%d dials happened, want none", dialed)
		}
		return nil
	case "closed":
		if last == nil {
			return errors.New("no socket was ever opened")
		}
		return d.pollUntil("the socket to be closed", func() bool {
			d.h.mu.Lock()
			defer d.h.mu.Unlock()
			return last.dead
		})
	default: // open
		if last == nil {
			return errors.New("no socket was ever opened")
		}
		d.h.mu.Lock()
		defer d.h.mu.Unlock()
		if last.dead {
			return errors.New("the socket is closed, want it still open")
		}
		return nil
	}
}

func (d *driver) assertNoResidue() error {
	d.h.mu.Lock()
	defer d.h.mu.Unlock()
	if len(d.h.violations) > 0 {
		return errors.New(d.h.violations[0])
	}
	if len(d.h.pendingMints) > 0 || len(d.h.pendingPolls) > 0 {
		return fmt.Errorf("%d mint and %d poll seam calls are still parked, unmatched by any expect step",
			len(d.h.pendingMints), len(d.h.pendingPolls))
	}
	if d.connectsTaken != len(d.h.connects) {
		return fmt.Errorf("%d cable dials happened, %d matched by an expectConnect step",
			len(d.h.connects), d.connectsTaken)
	}
	for i, peer := range d.h.peers {
		if peer.taken < len(peer.frames) {
			return fmt.Errorf("socket %d has %d unmatched outbound action(s), first: %s",
				i+1, len(peer.frames)-peer.taken, describeFrame(peer.frames[peer.taken]))
		}
	}
	if d.h.savesTaken != len(d.h.saves) {
		return fmt.Errorf("%d checkpoint saves happened, %d matched by an expectCheckpoint step",
			len(d.h.saves), d.h.savesTaken)
	}
	if d.h.saveScripted && d.h.saveUsed != len(d.h.saveScript) {
		return fmt.Errorf("the scripted store calls %v were consumed %d deep — an unconsumed outcome fails the scenario",
			d.h.saveScript, d.h.saveUsed)
	}
	return nil
}

func describeFrame(frame clientFrame) string {
	if frame.kind == "close" {
		return fmt.Sprintf("a client close (code %d)", frame.code)
	}
	return string(frame.data)
}

// terminalError returns the terminal error element, if the feed yielded one.
func (h *scenarioHarness) terminalError() *eventfeed.TerminalError {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.terminal
}

// violation returns the first recorded harness violation, if any.
func (h *scenarioHarness) violation() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.violations) == 0 {
		return nil
	}
	return errors.New(h.violations[0])
}

// --- scripted responses --------------------------------------------------

// mintOutcomeFrom maps a scripted mint response onto the seam's outcome: a
// ticket, or the classified *MintError the Layer-1 adapter would produce.
// Throttled-vs-transient is keyed on the PRESENCE of a §6-parsed Retry-After
// (SPEC's fixed adapter mapping — "whatever its status"), never on the
// status: a status key would honour the header at one retryable status and
// drop it at another. Unauthorized keeps a parsed value too — row 4 floors
// the below-threshold reconnect delay on it.
func (d *driver) mintOutcomeFrom(respond mintRespond) mintOutcome {
	if respond.Body != nil {
		return mintOutcome{ticket: eventfeed.StreamTicket{
			Ticket:    respond.Body.Ticket,
			ExpiresIn: respond.Body.ExpiresIn,
			URL:       respond.Body.URL,
		}}
	}
	retryAfter, present := d.retryAfterFrom(respond.Headers)
	mintErr := &eventfeed.MintError{RetryAfter: retryAfter, Err: fmt.Errorf("mint responded %d", *respond.Status)}
	switch *respond.Status {
	case 401, 403:
		mintErr.Kind = eventfeed.MintUnauthorized
	case 404, 422:
		mintErr.Kind = eventfeed.MintUnrecoverable
	default:
		if present {
			mintErr.Kind = eventfeed.MintThrottled
		} else {
			mintErr.Kind = eventfeed.MintTransient
		}
	}
	return mintOutcome{err: mintErr}
}

// pollOutcomeFrom maps a scripted poll response onto the seam's outcome: a
// page, or the classified *PollError the Layer-1 adapter would produce.
func (d *driver) pollOutcomeFrom(respond pollRespond) (pollOutcome, error) {
	status := 200
	if respond.Status != nil {
		status = *respond.Status
	}
	switch status {
	case 200:
		return pollPageFrom(respond.Body)
	case 302:
		return d.redirectRefusalFrom(respond.Headers)
	case 400:
		return pollOutcome{}, errors.New("400 poll responses are not modeled: the position-vs-filter discriminating bodies are class-1 literals still PROVISIONAL in the family README's dependency table, so a driver cannot discriminate them without guessing")
	case 409:
		return pollOutcome{err: &eventfeed.PollError{
			Kind: eventfeed.PollFilterChanged,
			Err:  errors.New("poll responded 409"),
		}}, nil
	case 410:
		return goneOutcome(respond.Body)
	case 401, 403:
		return pollOutcome{err: &eventfeed.PollError{
			Kind: eventfeed.PollUnauthorized,
			Err:  errors.New("poll responded 401/403"),
		}}, nil
	default:
		// Presence-keyed, exactly as the mint lane: a retryable outcome with
		// a §6-parsed Retry-After is throttled whatever its status, one
		// without is transient.
		retryAfter, present := d.retryAfterFrom(respond.Headers)
		kind := eventfeed.PollTransient
		if present {
			kind = eventfeed.PollThrottled
		}
		return pollOutcome{err: &eventfeed.PollError{
			Kind:       kind,
			RetryAfter: retryAfter,
			Err:        fmt.Errorf("poll responded %d", status),
		}}, nil
	}
}

func pollPageFrom(body json.RawMessage) (pollOutcome, error) {
	envelope := pollEnvelope{}
	if err := decodeStrict(body, &envelope); err != nil {
		return pollOutcome{}, fmt.Errorf("poll envelope: %w", err)
	}
	page := eventfeed.PollPage{Position: envelope.Position, Next: envelope.Next}
	for _, raw := range envelope.Events {
		row := pollEventRow{}
		if err := decodeStrict(raw, &row); err != nil {
			return pollOutcome{}, fmt.Errorf("poll event row: %w", err)
		}
		createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
		if err != nil {
			return pollOutcome{}, fmt.Errorf("poll event created_at: %w", err)
		}
		page.Events = append(page.Events, eventfeed.Event{
			ID:          row.ID,
			Kind:        row.Kind,
			EventType:   row.EventType,
			Action:      row.Action,
			CreatedAt:   createdAt,
			BucketID:    row.BucketID,
			CreatorID:   row.CreatorID,
			RecordingID: row.RecordingID,
		})
	}
	return pollOutcome{page: page}, nil
}

func goneOutcome(body json.RawMessage) (pollOutcome, error) {
	gone := goneBody{}
	if err := decodeStrict(body, &gone); err != nil {
		return pollOutcome{}, fmt.Errorf("410 body: %w", err)
	}
	return pollOutcome{err: &eventfeed.PollError{
		Kind:         eventfeed.PollGone,
		EpochAfterID: gone.EpochAfterID,
		ResumeURL:    gone.Resume,
		Err:          errors.New("poll responded 410"),
	}}, nil
}

// redirectRefusalFrom models the poll seam's suppressed redirect: the seam
// never follows automatically, and a Location that fails per-hop
// same-origin/no-downgrade validation is refused inside the call, with the
// Location redacted to its origin. No request is ever issued to it — the
// driver is the seam, so the foreign origin is unreachable by construction.
// redirectRefusalFrom models the Layer-1 adapter's per-hop redirect rule,
// and the rule it runs is the SHIPPED one: eventfeed.CheckContinuation is
// the per-hop same-origin/no-downgrade validation the adapter is specified
// to apply to a redirect Location ("a redirect Location failing per-hop
// validation inside a poll seam call"). An earlier revision re-implemented
// the decision as an ad-hoc origin comparison, which diverged at the edges —
// a Location that cannot be reduced to an origin at all failed the SCENARIO,
// as if the fixture were malformed rather than the peer hostile. What tier 2
// verifies through this is the refusal DECISION and the loop's response to
// it; that the adapter's HTTP client makes zero requests to the refused URL
// is a below-the-seam property, owned by Layer-1 adapter conformance.
func (d *driver) redirectRefusalFrom(headers map[string]string) (pollOutcome, error) {
	location := headers["Location"]
	if location == "" {
		return pollOutcome{}, errors.New("a 302 poll response needs a Location header")
	}
	base, err := eventfeed.CanonicalOrigin(d.h.apiOrigin)
	if err != nil {
		return pollOutcome{}, fmt.Errorf("canonicalizing the API origin: %w", err)
	}
	if terr := eventfeed.ExportCheckContinuation(base, location); terr == nil {
		return pollOutcome{}, errors.New("a same-origin 302 is not modeled: the seam would follow it, and no fixture scripts that hop")
	}
	// Redacted to its origin best-effort, exactly as the seam contract
	// requires of the adapter; an unreducible Location carries none.
	locationOrigin := ""
	if origin, oerr := eventfeed.CanonicalOrigin(location); oerr == nil {
		locationOrigin = origin
	}
	return pollOutcome{err: &eventfeed.PollError{
		Kind:           eventfeed.PollRedirectRefused,
		LocationOrigin: locationOrigin,
		Err:            errors.New("poll refused a redirect"),
	}}, nil
}

// retryAfterFrom is SPEC §6's Retry-After parsing algorithm, judged against
// the harness's virtual clock: an integer valid and > 0 is seconds; else a
// valid HTTP-date yields max(0, date − now) with a sub-second remainder
// rounded UP, returned only when > 0; everything else — zero, negatives,
// already-passed dates, malformed values — is undefined. present is whether
// the algorithm returned at all, and it is what keys the seam mapping
// (throttled with a value, transient without); the value is always positive
// when present, because the algorithm cannot yield zero ("zero is read as
// 'no usable value'").
func (d *driver) retryAfterFrom(headers map[string]string) (value time.Duration, present bool) {
	raw, ok := headers["Retry-After"]
	if !ok {
		return 0, false
	}
	// The delta-seconds form is RFC 9110's 1*DIGIT — no sign, no space — so
	// the digits are checked before ParseInt, which would honour a leading
	// "+" or "-". Parsed through int64 so the verdict cannot vary with the
	// platform's int width (Atoi's range is int's), with digits beyond int64
	// malformed, and a representable value clamped to the same portable
	// MaxInt32-seconds ceiling the SDK's parseRetryAfter uses — the naive
	// duration multiply would overflow into garbage for anything past
	// ~292 years.
	if isDigits(raw) {
		if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds > 0 {
			if seconds > math.MaxInt32 {
				seconds = math.MaxInt32
			}
			return time.Duration(seconds) * time.Second, true
		}
		return 0, false
	}
	date, err := http.ParseTime(raw)
	if err != nil {
		return 0, false
	}
	remainder := date.Sub(d.h.clock.Now())
	if remainder <= 0 {
		return 0, false
	}
	seconds := int64(remainder / time.Second)
	if remainder%time.Second != 0 {
		seconds++
	}
	return time.Duration(seconds) * time.Second, true
}

// --- server frames -------------------------------------------------------

// isDigits reports RFC 9110's 1*DIGIT and nothing else, mirroring the SDK
// parser's isDelaySeconds (go/pkg/basecamp/client.go).
func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// serveFrameBytes renders one scripted server frame. confirm/reject/message
// auto-echo the captured subscribe identifier unless the fixture overrides it.
func serveFrameBytes(step *serveStep, identifier string) ([]byte, error) {
	echo := identifier
	if step.Identifier != nil {
		echo = *step.Identifier
	}
	switch step.Frame {
	case "welcome":
		return []byte(`{"type":"welcome"}`), nil
	case "ping":
		if step.Message == nil {
			return []byte(`{"type":"ping"}`), nil
		}
		return json.Marshal(map[string]any{"type": "ping", "message": *step.Message})
	case "confirm", "reject":
		if echo == "" {
			return nil, fmt.Errorf("no subscribe identifier has been captured to echo")
		}
		frameType := "confirm_subscription"
		if step.Frame == "reject" {
			frameType = "reject_subscription"
		}
		return json.Marshal(map[string]any{"type": frameType, "identifier": echo})
	case "disconnect":
		return json.Marshal(map[string]any{
			"type":      "disconnect",
			"reason":    step.Reason,
			"reconnect": *step.Reconnect,
		})
	case "message":
		if echo == "" {
			return nil, fmt.Errorf("no subscribe identifier has been captured to echo")
		}
		return json.Marshal(map[string]any{"identifier": echo, "message": step.Event})
	default: // raw
		return []byte(step.Text), nil
	}
}

// --- small helpers -------------------------------------------------------

func prefixOfIDs(got, want []int64) bool {
	return len(got) <= len(want) && slices.Equal(got, want[:len(got)])
}

func prefixOfStrings(got, want []string) bool {
	return len(got) <= len(want) && slices.Equal(got, want[:len(got)])
}

func prefixOfInvocations(got, want []invocation) bool {
	return len(got) <= len(want) && slices.Equal(got, want[:len(got)])
}
