// Tier-2 conformance: the scenario harness.
//
// The Go lane drives the REAL default WebSocket transport over an in-process
// loopback cable server (`httptest` + coder/websocket server-side), with only
// the Clock faked — the lane the family README's consumers table names. The
// mint and poll lanes are SEAM calls (the Layer-1 adapter over the generated
// CreateStreamTicket/PollEvents operations lands with Layer 1), so this
// harness scripts them at `TicketMinter`/`PollSource` with the family's
// parked-call semantics: a seam call blocks inside the seam until the driver
// reaches its matching expect step.
//
// Everything the connector reports — deliveries, saves, signals, handler
// invocations, observer notifications, state transitions, buffer occupancy —
// arrives on the consumer's goroutine (ranging Events IS the state machine),
// so the harness records a single serialized history and the driver asserts
// over it. No callback ever fails a test directly: violations are recorded and
// surfaced by the driver goroutine, which is the only one that reports.
package eventfeed_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// scenarioWatchdog bounds every rendezvous. Virtual time is frozen between
// directives, so this is the only wall clock in the driver. It is a var
// solely so the driver's own tests can shorten it while asserting that a
// rendezvous which can never be satisfied does time out; scenarios always run
// under the full window.
var scenarioWatchdog = 5 * time.Second

// scenarioAccountID is the account every scenario's checkpoint identity binds
// to; scenarioNamespace is the fixed consumer namespace a configured store
// requires.
const (
	scenarioAccountID = "5951425"
	scenarioNamespace = "conformance"
)

// harness owns the scenario's servers, seams, and recorded history. One
// mutex guards everything: the connector's callbacks and the driver goroutine
// are the only writers, and every mutation broadcasts so rendezvous waiters
// wake.
type scenarioHarness struct {
	mu      sync.Mutex
	changed chan struct{}

	clock *feedtest.Clock

	// Arrival strictness. program is the step script a checkpoint save or an
	// outbound frame is judged against when it arrives, and cursor is the step
	// the driver has reached in it. Both live under h.mu because the judgement
	// has to be made in the same critical section that records the action —
	// that is the whole of the family README's "observed while the current
	// step is anything other than their matching expect step".
	program *driver
	cursor  int

	cableSrv  *httptest.Server
	apiSrv    *httptest.Server
	apiOrigin string
	cableBase string

	// connectPlan is the scenario's expectConnect outcomes, in order: the Nth
	// handshake is answered by the Nth plan entry (dials happen strictly in
	// order, so the indices line up).
	connectPlan []connectPlanEntry

	// Seam parking.
	mintCalls    int
	pendingMints []*mintCall
	pollCalls    int
	pendingPolls []*pollCall

	// Cable lane.
	connects []*connectAttempt
	peers    []*cablePeer

	// Checkpoint store script.
	storeLoad    string
	storeLoadOK  bool
	storeLoadErr error
	saveScript   []string
	saveScripted bool
	saveUsed     int

	// Recorded history.
	delivered          []int64
	saves              []saveRecord
	savesTaken         int
	occupancy          int
	occupancyHistory   []int
	signals            []eventfeed.Signal
	signalsTaken       int
	invocations        []invocation
	gaps               []int64
	gapsTaken          int
	saveFailures       int
	saveFailuresTaken  int
	positionRejected   []string
	posRejectedTaken   int
	invalidFrames      int
	invalidFramesTaken int
	state              string
	terminal           *eventfeed.TerminalError
	terminalCount      int
	iterDone           bool
	violations         []string
}

// saveRecord is one CheckpointStore.Save call plus the ordering witness the
// save-ordering assertions need: how many events had been delivered when the
// save was made. A save that arrives before the deliveries its script
// requires is exactly checkpoint-before-handoff.
type saveRecord struct {
	position    string
	deliveredAt int
}

type connectPlanEntry struct {
	kind   string // "accept" | "refuse" | "redirect"
	target string
}

type connectAttempt struct {
	url  string
	kind string
	peer *cablePeer
}

// cablePeer is one accepted loopback cable connection: the server side of the
// socket the connector dialed.
type cablePeer struct {
	conn *websocket.Conn

	frames []clientFrame
	taken  int
	dead   bool
}

// clientFrame is one thing the connector sent: a text frame, or its
// client-initiated close.
type clientFrame struct {
	kind string // "text" | "close"
	data []byte
	code int
}

type mintCall struct{ release chan mintOutcome }

type mintOutcome struct {
	ticket eventfeed.StreamTicket
	err    error
}

type pollCall struct {
	cursor  eventfeed.Cursor
	filters eventfeed.Filters
	release chan pollOutcome
}

type pollOutcome struct {
	page eventfeed.PollPage
	err  error
}

// newScenarioHarness starts the loopback servers and returns a ready harness.
func newScenarioHarness() *scenarioHarness {
	h := &scenarioHarness{
		changed: make(chan struct{}),
		clock:   feedtest.NewClock(),
	}
	h.cableSrv = httptest.NewServer(http.HandlerFunc(h.handleCable))
	h.apiSrv = httptest.NewServer(http.HandlerFunc(h.handleAPI))
	h.apiOrigin = h.apiSrv.URL
	h.cableBase = "ws" + strings.TrimPrefix(h.cableSrv.URL, "http")
	return h
}

// close tears the harness down: live peers first (so hijacked handlers
// return), then the servers.
func (h *scenarioHarness) close() {
	h.mu.Lock()
	peers := slices.Clone(h.peers)
	h.mu.Unlock()
	for _, p := range peers {
		_ = p.conn.CloseNow()
	}
	h.cableSrv.Close()
	h.apiSrv.Close()
}

// cableURL renders the nth cable URL — distinct per index, loopback ws:// per
// the SPEC §9 carve-out, ticket in the query string exactly as a mint's URL
// carries it.
func (h *scenarioHarness) cableURL(index int) string {
	return fmt.Sprintf("%s/cable/%d?ticket=%s", h.cableBase, index, ticketToken(index))
}

// --- broadcast + rendezvous ----------------------------------------------

// notifyLocked wakes every rendezvous waiter. Callers hold h.mu.
func (h *scenarioHarness) notifyLocked() {
	close(h.changed)
	h.changed = make(chan struct{})
}

// violate records a harness-observed contract violation. Callbacks never fail
// the test directly — the driver goroutine surfaces this at its next
// rendezvous or at `finally`.
func (h *scenarioHarness) violate(format string, args ...any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.violations = append(h.violations, fmt.Sprintf(format, args...))
	h.notifyLocked()
}

// --- arrival strictness --------------------------------------------------

// attach binds the driver whose step script the arrival rule judges against.
// Until it is bound the harness has no script — the driver's own unit tests
// construct a bare harness and drive its record paths directly — and the rule
// records nothing.
func (h *scenarioHarness) attach(d *driver) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.program = d
}

// enterStep moves the step pointer onto index. It only ever moves FORWARD:
// `await` hands the pointer to the next step at the instant a rendezvous is
// satisfied, so by the time the step loop reaches that step its own call is
// already stale and must not rewind it.
func (h *scenarioHarness) enterStep(index int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if index > h.cursor {
		h.cursor = index
	}
}

// advanceLocked is the atomic half of the handoff the family README names: it
// runs inside the SAME critical section that observed a rendezvous being
// satisfied, so an action the connector emits the instant it is satisfied is
// judged against the next step rather than against the stale one. Callers hold
// h.mu.
func (h *scenarioHarness) advanceLocked() {
	if h.program != nil && h.cursor < len(h.program.sc.Steps) {
		h.cursor++
	}
}

// arrivalStrictLocked records a violation when an arrival-strict action —
// a checkpoint save, or an outbound frame — is observed while the current step
// is anything other than the expect step that matches it. Callers hold h.mu
// and call this BEFORE recording the action, so the matching step's own
// "an action of mine is waiting" condition is still false and cannot be
// mistaken for the step having already been satisfied.
func (h *scenarioHarness) arrivalStrictLocked(want, what string) {
	if h.program == nil {
		return
	}
	kind, where := h.program.currentStepLocked()
	if kind == want {
		return
	}
	h.violations = append(h.violations, fmt.Sprintf(
		"%s arrived at %s, not at its matching %s step — checkpoint saves and outbound frames are arrival-strict",
		what, where, want))
}

// await blocks until check reports satisfaction, fails immediately when check
// reports an unsatisfiable state, and gives up at the watchdog. check runs
// under h.mu and may consume harness state on the satisfying call.
func (h *scenarioHarness) await(what string, check func() (bool, string)) error {
	deadline := time.Now().Add(scenarioWatchdog)
	for {
		h.mu.Lock()
		violation := ""
		if len(h.violations) > 0 {
			violation = h.violations[0]
		}
		ok, bad := false, ""
		if violation == "" {
			if ok, bad = check(); ok {
				h.advanceLocked()
			}
		}
		wake := h.changed
		h.mu.Unlock()
		switch {
		case violation != "":
			return fmt.Errorf("while waiting for %s: %s", what, violation)
		case bad != "":
			return fmt.Errorf("%s: %s", what, bad)
		case ok:
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("timed out after %s waiting for %s\n%s", scenarioWatchdog, what, h.describe())
		}
		timer := time.NewTimer(remaining)
		select {
		case <-wake:
		case <-timer.C:
		}
		timer.Stop()
	}
}

// describe renders the harness's current history for a timeout message.
func (h *scenarioHarness) describe() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var b strings.Builder
	if h.program != nil {
		_, where := h.program.currentStepLocked()
		fmt.Fprintf(&b, "  current step: %s\n", where)
	}
	fmt.Fprintf(&b, "  state=%s delivered=%v buffered=%d\n", h.state, h.delivered, h.occupancy)
	fmt.Fprintf(&b, "  mintCalls=%d (%d parked) pollCalls=%d (%d parked) connects=%d\n",
		h.mintCalls, len(h.pendingMints), h.pollCalls, len(h.pendingPolls), len(h.connects))
	positions := make([]string, len(h.saves))
	for i, rec := range h.saves {
		positions[i] = rec.position
	}
	fmt.Fprintf(&b, "  saves=%v invocations=%v gaps=%v\n", positions, h.invocations, h.gaps)
	fmt.Fprintf(&b, "  timers=%v terminal=%v iterationEnded=%t\n", h.clock.Outstanding(), h.terminal, h.iterDone)
	return b.String()
}

// --- seams ---------------------------------------------------------------

// MintStreamTicket implements eventfeed.TicketMinter with the family's parked
// semantics: the call is counted and parked, and only the matching
// `expectMint` step releases its scripted response.
func (h *scenarioHarness) MintStreamTicket(ctx context.Context) (eventfeed.StreamTicket, error) {
	call := &mintCall{release: make(chan mintOutcome, 1)}
	h.mu.Lock()
	h.mintCalls++
	h.pendingMints = append(h.pendingMints, call)
	h.notifyLocked()
	h.mu.Unlock()
	select {
	case out := <-call.release:
		return out.ticket, out.err
	case <-ctx.Done():
		return eventfeed.StreamTicket{}, ctx.Err()
	}
}

// Poll implements eventfeed.PollSource with the same parked semantics.
func (h *scenarioHarness) Poll(ctx context.Context, cursor eventfeed.Cursor, filters eventfeed.Filters) (eventfeed.PollPage, error) {
	call := &pollCall{cursor: cursor, filters: filters, release: make(chan pollOutcome, 1)}
	h.mu.Lock()
	h.pollCalls++
	h.pendingPolls = append(h.pendingPolls, call)
	h.notifyLocked()
	h.mu.Unlock()
	select {
	case out := <-call.release:
		return out.page, out.err
	case <-ctx.Done():
		return eventfeed.PollPage{}, ctx.Err()
	}
}

// Load implements eventfeed.CheckpointStore's tri-state load.
func (h *scenarioHarness) Load(_ context.Context, _ eventfeed.CheckpointKey) (string, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.storeLoad, h.storeLoadOK, h.storeLoadErr
}

// Save implements eventfeed.CheckpointStore's save, recording the call with
// its ordering witness and returning the scripted outcome. A save beyond the
// script is a violation — the exact store-call script is what proves a
// subsequent save is attempted after a failure — and so is a save that arrives
// while the script is anywhere but its matching expectCheckpoint step.
func (h *scenarioHarness) Save(_ context.Context, _ eventfeed.CheckpointKey, position string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.arrivalStrictLocked("expectCheckpoint", fmt.Sprintf("the checkpoint save of %q", position))
	h.saves = append(h.saves, saveRecord{position: position, deliveredAt: len(h.delivered)})
	var err error
	if h.saveScripted {
		if h.saveUsed >= len(h.saveScript) {
			h.violations = append(h.violations,
				fmt.Sprintf("save(%s) beyond the scripted store calls %v", position, h.saveScript))
		} else {
			if h.saveScript[h.saveUsed] == "failed" {
				err = errors.New("feedtest: scripted checkpoint save failure")
			}
			h.saveUsed++
		}
	}
	h.notifyLocked()
	return err
}

// --- cable loopback ------------------------------------------------------

// handleCable serves one dialed cable connection, recording the request-line
// URL verbatim first — even when the scenario scripts the dial to be refused.
func (h *scenarioHarness) handleCable(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	plan := connectPlanEntry{kind: "accept"}
	if idx := len(h.connects); idx < len(h.connectPlan) {
		plan = h.connectPlan[idx]
	}
	attempt := &connectAttempt{url: "ws://" + r.Host + r.RequestURI, kind: plan.kind}
	h.connects = append(h.connects, attempt)
	h.notifyLocked()
	h.mu.Unlock()

	switch plan.kind {
	case "refuse":
		w.WriteHeader(http.StatusBadRequest)
		return
	case "redirect":
		http.Redirect(w, r, plan.target, http.StatusFound)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{"actioncable-v1-json"},
	})
	if err != nil {
		h.violate("the cable upgrade failed: %v", err)
		return
	}
	peer := &cablePeer{conn: conn}
	h.mu.Lock()
	attempt.peer = peer
	h.peers = append(h.peers, peer)
	h.notifyLocked()
	h.mu.Unlock()
	//nolint:contextcheck // deliberate: after the upgrade the connection's
	// lifetime is scripted by the scenario, not bound to r.Context().
	h.readPeer(peer)
}

// readPeer records everything the connector sends on one socket, in order: a
// text frame (the subscribe command), or its client-initiated close.
func (h *scenarioHarness) readPeer(peer *cablePeer) {
	for {
		typ, data, err := peer.conn.Read(context.Background())
		if err != nil {
			var closeErr websocket.CloseError
			h.mu.Lock()
			if errors.As(err, &closeErr) {
				h.recordClientFrameLocked(peer, clientFrame{kind: "close", code: int(closeErr.Code)})
			}
			peer.dead = true
			h.notifyLocked()
			h.mu.Unlock()
			return
		}
		if typ != websocket.MessageText {
			h.violate("the connector sent a non-text cable frame")
			continue
		}
		h.recordClientFrame(peer, clientFrame{kind: "text", data: slices.Clone(data)})
	}
}

// recordClientFrame records one outbound action under the arrival rule.
func (h *scenarioHarness) recordClientFrame(peer *cablePeer, frame clientFrame) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recordClientFrameLocked(peer, frame)
	h.notifyLocked()
}

// recordClientFrameLocked is the arrival-strict half: an outbound frame is
// legal only while the script is at the expect step that matches it. The close
// arm shares this path because a close observed early is the same defect as a
// subscribe observed early — the connector acting ahead of the script.
func (h *scenarioHarness) recordClientFrameLocked(peer *cablePeer, frame clientFrame) {
	if frame.kind == "close" {
		h.arrivalStrictLocked("expectClientClose", "the connector's close of the socket")
	} else {
		h.arrivalStrictLocked("expectSubscribe", fmt.Sprintf("the outbound frame %s", frame.data))
	}
	peer.frames = append(peer.frames, frame)
}

// handleAPI is the API origin's sentinel: the connector reaches the mint and
// poll lanes through seams, so ANY HTTP request arriving here is egress the
// scenario never scripted.
func (h *scenarioHarness) handleAPI(w http.ResponseWriter, r *http.Request) {
	h.violate("an unscripted HTTP request reached the API origin: %s %s", r.Method, r.URL)
	w.WriteHeader(http.StatusTeapot)
}

// closePeer closes one socket server-side, with a close frame (contrast
// `sever`, which drops the TCP connection abruptly).
func (h *scenarioHarness) closePeer(peer *cablePeer, code int, reason string) error {
	return peer.conn.Close(websocket.StatusCode(code), reason)
}

// writeFrame sends one server frame to the peer.
func (h *scenarioHarness) writeFrame(peer *cablePeer, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), scenarioWatchdog)
	defer cancel()
	return peer.conn.Write(ctx, websocket.MessageText, data)
}

// --- recording -----------------------------------------------------------

func (h *scenarioHarness) recordDelivered(id int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// The error element is FINAL: nothing may be yielded after it. Recorded as
	// a violation rather than dropped, so the delivered ledger still shows what
	// arrived and the failure names the ordering rather than an exact-set
	// mismatch that reads as a missing event.
	if h.terminalCount > 0 {
		h.violations = append(h.violations, fmt.Sprintf(
			"the feed delivered event %d after its error element (%v); §23's final error element ends iteration",
			id, h.terminal))
	}
	h.delivered = append(h.delivered, id)
	h.notifyLocked()
}

// recordTerminal records one error element yielded by the iteration.
//
// §23 gives the feed EXACTLY ONE final error element, and it ends iteration.
// Both halves are enforced here rather than assumed, because neither is
// observable from an end-state assertion: keeping only the latest terminal —
// which this did — silently accepts a connector that yields the expected
// reason twice, since the second overwrites the first with an equal value and
// every later assertion still passes.
//
// The FIRST element is retained. A second is a violation naming both, and
// violations block every subsequent await, so the scenario fails at its next
// step rather than at the end.
func (h *scenarioHarness) recordTerminal(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var terr *eventfeed.TerminalError
	if !errors.As(err, &terr) {
		h.violations = append(h.violations, fmt.Sprintf("the feed yielded a non-terminal error element: %v", err))
	} else {
		h.terminalCount++
		if h.terminalCount > 1 {
			h.violations = append(h.violations, fmt.Sprintf(
				"the feed yielded %d error elements (%v after %v); §23 gives it exactly one, and it ends iteration",
				h.terminalCount, terr, h.terminal))
		} else {
			h.terminal = terr
		}
	}
	h.notifyLocked()
}

func (h *scenarioHarness) recordIterDone() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.iterDone = true
	h.notifyLocked()
}

func (h *scenarioHarness) recordState(state string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = state
	h.notifyLocked()
}

func (h *scenarioHarness) recordOccupancy(count int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.occupancy = count
	h.occupancyHistory = append(h.occupancyHistory, count)
	h.notifyLocked()
}

func (h *scenarioHarness) recordSignal(signal eventfeed.Signal) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.signals = append(h.signals, signal)
	h.notifyLocked()
}

func (h *scenarioHarness) recordInvocation(rec invocation) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.invocations = append(h.invocations, rec)
	h.notifyLocked()
}

func (h *scenarioHarness) recordGap(epochAfterID int64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gaps = append(h.gaps, epochAfterID)
	h.notifyLocked()
}

func (h *scenarioHarness) recordSaveFailed() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.saveFailures++
	h.notifyLocked()
}

func (h *scenarioHarness) recordPositionRejected(kind string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.positionRejected = append(h.positionRejected, kind)
	h.notifyLocked()
}

func (h *scenarioHarness) recordInvalidFrame() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.invalidFrames++
	h.notifyLocked()
}

// --- connector construction ----------------------------------------------

// newConnector builds the scenario's connector: real transport, virtual
// clock, parked seams, scripted store, recording handler and observer.
//
// Two mappings are worth stating outright. (1) A store is configured for
// EVERY scenario, because `finally.checkpoints` is an assertion about
// CheckpointStore.save calls and the Go connector makes none without one;
// `config.position` therefore enters as the store's Loaded outcome rather
// than through StartAtPosition, which §23 makes mutually exclusive with a
// store. Both resolve to the identical position-resume entry cursor, so the
// scenario means the same thing either way. (2) `config.signalDisposition`
// installs a handler ONLY when the fixture carries the key: an absent key is
// load-bearing (the default-terminal path), so with no dispositions at all no
// handler is registered, and a kind whose key is absent is answered Terminate
// WITHOUT recording an invocation — the no-handler behavior, which is what
// the per-signal default-terminal pins assert.
func (h *scenarioHarness) newConnector(cfg scenarioConfig) (*eventfeed.Connector, error) {
	h.applyStoreScript(cfg)

	opts := []eventfeed.Option{
		eventfeed.WithClock(h.clock),
		eventfeed.WithCheckpointStore(h),
		eventfeed.WithConsumerNamespace(scenarioNamespace),
		eventfeed.WithFilters(eventfeed.Filters{
			Types:    cfg.Types,
			Buckets:  cfg.Buckets,
			Creators: cfg.Creators,
		}),
		eventfeed.WithObserver(eventfeed.Observer{
			Gap:                  func(epochAfterID int64, _ string) { h.recordGap(epochAfterID) },
			CheckpointSaveFailed: func(error) { h.recordSaveFailed() },
			PositionRejected:     func(kind eventfeed.PollErrorKind) { h.recordPositionRejected(kind.String()) },
			Disconnected: func(_ string, err error) {
				if eventfeed.ExportIsInvalidFrameError(err) {
					h.recordInvalidFrame()
				}
			},
		}),
	}
	if cfg.ConfirmationDeadlineMs > 0 {
		opts = append(opts, eventfeed.WithConfirmationDeadline(millis(cfg.ConfirmationDeadlineMs)))
	}
	if cfg.RepairPollBaseMs > 0 {
		opts = append(opts, eventfeed.WithRepairInterval(millis(cfg.RepairPollBaseMs)))
	}
	if cfg.LiveBufferCapacity > 0 {
		opts = append(opts, eventfeed.WithLiveBufferCapacity(cfg.LiveBufferCapacity))
	}
	if cfg.DedupeCapacity > 0 {
		opts = append(opts, eventfeed.WithDedupeCapacity(cfg.DedupeCapacity))
	}
	if len(cfg.SignalDisposition) > 0 {
		opts = append(opts, eventfeed.WithSignalHandler(h.signalHandler(cfg.SignalDisposition)))
	}

	conn, err := eventfeed.New(h.apiOrigin, scenarioAccountID, h, h, opts...)
	if err != nil {
		return nil, err
	}
	if cfg.StalenessMs > 0 {
		conn.SetStaleAfter(millis(cfg.StalenessMs))
	}
	conn.OnStateChanged(h.recordState)
	conn.OnBufferOccupancy(h.recordOccupancy)
	conn.OnSignal(h.recordSignal)
	return conn, nil
}

// applyStoreScript wires config.checkpointStore (or config.position) into the
// harness store.
func (h *scenarioHarness) applyStoreScript(cfg scenarioConfig) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch {
	case cfg.CheckpointStore == nil && cfg.Position != "":
		h.storeLoad, h.storeLoadOK = cfg.Position, true
	case cfg.CheckpointStore == nil:
		// Missing: the entry is the bare present.
	case cfg.CheckpointStore.Load == "failed":
		h.storeLoadErr = errors.New("feedtest: scripted checkpoint load failure")
	case strings.HasPrefix(cfg.CheckpointStore.Load, "loaded:"):
		h.storeLoad, h.storeLoadOK = strings.TrimPrefix(cfg.CheckpointStore.Load, "loaded:"), true
	}
	if cfg.CheckpointStore != nil && len(cfg.CheckpointStore.Save) > 0 {
		h.saveScript, h.saveScripted = cfg.CheckpointStore.Save, true
	}
}

// signalHandler builds the recording handler for the configured dispositions.
func (h *scenarioHarness) signalHandler(dispositions map[string]string) eventfeed.SignalHandler {
	return func(signal eventfeed.Signal) eventfeed.Disposition {
		kind := "feedGap"
		if _, ok := signal.(eventfeed.BufferOverflow); ok {
			kind = "bufferOverflow"
		}
		disposition, configured := dispositions[kind]
		if !configured {
			// No handler exists for this kind: answer exactly as the absent
			// handler would, and record nothing.
			return eventfeed.Terminate
		}
		h.recordInvocation(invocation{Kind: kind, Disposition: disposition})
		if disposition == "accept" {
			return eventfeed.Accept
		}
		return eventfeed.Terminate
	}
}

func millis(ms int) time.Duration { return time.Duration(ms) * time.Millisecond }
