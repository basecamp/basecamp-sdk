package eventfeed

import (
	"context"
	"errors"
	"slices"
	"time"
)

// export_test.go bridges the external eventfeed_test package (which uses the
// feedtest fakes and therefore cannot live in-package — import cycle) into
// the connector's unexported observation points. Everything here is
// test-binary-only; nothing leaks into the shipped surface.

// CatchUpBoundary is the external-test projection of the internal
// catchUpHandoff — the slice-4b seam a confirmed subscription hands off at.
type CatchUpBoundary struct {
	// Entry is the selected entry cursor.
	Entry Cursor
	// PresentClass reports whether the entry resolves at the present head.
	PresentClass bool
	// Buffered is a snapshot of the live buffer's contents at hand-off.
	Buffered []Event
}

// OnCatchUpEntered registers a hook fired when a confirmed subscription
// reaches the catch-up boundary.
func (c *Connector) OnCatchUpEntered(f func(CatchUpBoundary)) {
	c.hooks.catchUpEntered = func(h catchUpHandoff) {
		f(CatchUpBoundary{
			Entry:        h.entry,
			PresentClass: h.presentClass,
			Buffered:     slices.Clone(h.buffer.events),
		})
	}
}

// OnStateChanged registers a hook receiving each state transition's fixture
// spelling.
func (c *Connector) OnStateChanged(f func(string)) {
	c.hooks.stateChanged = func(s connState) { f(s.String()) }
}

// OnFrameHandled registers a hook receiving each handled inbound frame's
// kind name, fired after the state machine dequeued and classified it.
func (c *Connector) OnFrameHandled(f func(string)) {
	c.hooks.frameHandled = func(k frameKind) { f(k.String()) }
}

// OnPumpBlocked registers a hook fired when a frame pump hand-off finds the
// bounded queue full — the moment the staleness evaluation suspends.
func (c *Connector) OnPumpBlocked(f func()) { c.hooks.pumpBlocked = f }

// OnPumpReleased registers a hook fired once a blocked hand-off completes and
// its release has re-armed the staleness window unsuspended. It is the
// rendezvous for "the suspension has lifted" — the counterpart to
// OnPumpBlocked, and the only observation point that proves the release's
// re-arm has landed rather than still being in flight behind the dequeue.
func (c *Connector) OnPumpReleased(f func()) { c.hooks.pumpReleased = f }

// OnPumpHandedOff registers a hook fired once an item reaches the hand-off
// queue, reporting whether it is the pump's terminating error. It is the
// rendezvous for "the socket's death is queued for the state machine".
func (c *Connector) OnPumpHandedOff(f func(isErr bool)) { c.hooks.pumpHandedOff = f }

// OnPumpRead registers a hook fired after a pump read returns and before its
// hand-off — the window in which a frame is READ but not yet observable
// through the queue. Parking here is the only deterministic way to put the
// protocol-fatal scan and an already-read frame in the same instant.
func (c *Connector) OnPumpRead(f func()) { c.hooks.pumpRead = f }

// OnFrameDeferred registers a hook fired when the in-flight-poll servicing
// parks one receive — always a socket outcome — for the walk's next dispatch
// point. It is the rendezvous for "the walk is now waiting on nothing but the
// seam call": without it, a test that serves the deferring frame and then lets
// the poll return races the state machine's own select between the queued frame
// and the completed call.
func (c *Connector) OnFrameDeferred(f func()) { c.hooks.frameDeferred = f }

// ExportPumpDepth is the frame pump's bounded hand-off queue depth: the
// number of frames a test must serve to make the pump block.
const ExportPumpDepth = pumpDepth

// SetStaleAfter overrides the staleness window — the tier-2 driver's
// stalenessMs scenario config. Deliberately not a public option: SPEC §23
// pins 7500ms.
func (c *Connector) SetStaleAfter(d time.Duration) { c.cfg.staleAfter = d }

// SetRand overrides the uniform [0, 1) jitter source — white-box, no public
// option.
func (c *Connector) SetRand(r func() float64) { c.cfg.rand = r }

// OnBufferOccupancy registers a hook receiving the state-machine-owned live
// buffer's occupancy after every change (events admitted minus events
// dropped) — the tier-2 `expectBuffered` rendezvous.
func (c *Connector) OnBufferOccupancy(f func(int)) { c.hooks.bufferOccupancy = f }

// OnSignal registers a hook receiving every semantic signal as it is raised,
// before its disposition is taken. Unlike the Observer callbacks it carries
// the signal's full payload and fires whether or not a handler is registered —
// which is what makes the default-terminal fixtures' exact `expectSignal`
// assertions (dropped ids included) observable.
func (c *Connector) OnSignal(f func(Signal)) { c.hooks.signalRaised = f }

// ExportIsInvalidFrameError reports whether err is a §23 invalid-frame-class
// violation — the indication Observer.Disconnected carries for the tier-2
// `expectDisconnectedInvalidFrame` rendezvous.
func ExportIsInvalidFrameError(err error) bool {
	var ife *invalidFrameError
	return errors.As(err, &ife)
}

// ExportObservableSocketError exposes the teardown-cause reduction that guards
// Observer.Disconnected's error argument. Tested directly rather than only
// through a socket because the leak it must prevent is in the SHAPE of the
// error handed to it — a seam error WRAPPING a sentinel — and driving every
// such shape through a live teardown proves less about the mapping than
// calling it does.
func ExportObservableSocketError(err error) error { return observableSocketError(err) }

// ExportStaleConnectionErr returns the connector's staleness sentinel, for
// asserting that a wrapped one reduces to the sentinel itself rather than to
// its wrapper. A func rather than a var so the errname linter does not read a
// test-only bridge as a package sentinel needing the ErrXxx spelling.
func ExportStaleConnectionErr() error { return errStaleConnection }

// ExportCableConnClosedErr returns the connector's socket-closed sentinel,
// exposed for the same reason as ExportStaleConnectionErr.
func ExportCableConnClosedErr() error { return errCableConnClosed }

// ExportSocketFailedErr returns the generic cause an unrecognized teardown is
// reported as — what a seam-authored error must reduce to. Exposed so the
// reduction can be asserted by IDENTITY: an assertion that merely checks the
// canary is absent passes for any of several wrong answers, including nil.
func ExportSocketFailedErr() error { return errSocketFailed }

// ExportSubscribeIdentifier exposes the exact EventsChannel subscription
// identifier for frame construction in tests.
func ExportSubscribeIdentifier(f Filters) string { return subscribeIdentifier(f) }

// ExportSubscribeFrame exposes the exact subscribe command bytes the connector
// writes.
func ExportSubscribeFrame(f Filters) []byte { return subscribeCommand(subscribeIdentifier(f)) }

// OnSubscribeWritten registers a hook fired once the subscribe command has
// been written and BEFORE the phase deadline is stopped. It is the only
// rendezvous for the deadline-vs-welcome ordering: from outside, a deadline
// that fires while the welcome frame is already queued leaves the state
// machine's select with two ready cases and no way for a test to say which
// one it must take.
func (c *Connector) OnSubscribeWritten(f func()) { c.hooks.subscribeWritten = f }

// OnRunContext registers a hook receiving the active run's context once its
// cancellation has been REGISTERED — the point from which Close's guarantee is
// a statement about that context. Before it, the run is held by the isClosed
// latch instead, so a context handed out earlier would invite an assertion
// about a window the latch already covers. It is the only way to observe Close's actual guarantee —
// "cancellation is visible before the return" is a statement about that
// context, and any proxy for it (a nil cancel func, a closed latch) is exactly
// the state a broken Close sets too early.
func (c *Connector) OnRunContext(f func(context.Context)) { c.hooks.runContext = f }
