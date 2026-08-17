package eventfeed

import (
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
