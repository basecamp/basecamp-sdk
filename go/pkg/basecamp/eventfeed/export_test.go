package eventfeed

import (
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

// SetStaleAfter overrides the staleness window — the tier-2 driver's
// stalenessMs scenario config. Deliberately not a public option: SPEC §23
// pins 7500ms.
func (c *Connector) SetStaleAfter(d time.Duration) { c.cfg.staleAfter = d }

// SetRand overrides the uniform [0, 1) jitter source — white-box, no public
// option.
func (c *Connector) SetRand(r func() float64) { c.cfg.rand = r }

// ExportSubscribeIdentifier exposes the exact EventsChannel subscription
// identifier for frame construction in tests.
func ExportSubscribeIdentifier(f Filters) string { return subscribeIdentifier(f) }

// ExportSubscribeFrame exposes the exact subscribe command bytes the connector
// writes.
func ExportSubscribeFrame(f Filters) []byte { return subscribeCommand(subscribeIdentifier(f)) }
