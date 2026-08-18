package feedtest

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// Polls is a scripted eventfeed.PollSource: tests queue poll outcomes (pages
// and classified errors) in order and read back the full call log — cursor
// and filters per seam call. Every call is logged, including stalled and
// cancelled ones (SPEC.md §23 "Seam-Call Semantics"), and an unscripted call
// fails visibly rather than fabricating a page.
type Polls struct {
	mu     sync.Mutex
	script []pollResult
	calls  []PollCall
	stalls int
	onCall func(PollCall)
}

var _ eventfeed.PollSource = (*Polls)(nil)

// PollCall records one PollSource.Poll seam call.
type PollCall struct {
	// Cursor is the cursor the connector polled at.
	Cursor eventfeed.Cursor
	// Filters are the filters the connector polled under.
	Filters eventfeed.Filters
}

type pollResult struct {
	page eventfeed.PollPage
	err  error
}

// NewPolls returns an empty scripted poll source: every call fails as
// unscripted until pages or errors are queued.
func NewPolls() *Polls {
	return &Polls{}
}

// ScriptPage queues one successful poll page (FIFO with ScriptError).
func (p *Polls) ScriptPage(page eventfeed.PollPage) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.script = append(p.script, pollResult{page: page})
}

// ScriptError queues one failed poll (FIFO with ScriptPage). Pass a
// *eventfeed.PollError to exercise the connector's poll classification.
func (p *Polls) ScriptError(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.script = append(p.script, pollResult{err: err})
}

// OnCall registers a callback invoked inside every Poll call, after the call
// is logged and before its scripted outcome is produced. It is what lets a
// test act while the connector is blocked in the poll seam — serving a live
// frame that must land in the entry window, or severing the socket mid-walk.
func (p *Polls) OnCall(fn func(PollCall)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onCall = fn
}

// StallNext scripts the next call to block until its context is done and
// then return the context's error — the teardown-mid-poll case.
func (p *Polls) StallNext() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stalls++
}

// Calls returns every recorded poll seam call, in order.
func (p *Polls) Calls() []PollCall {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.calls)
}

// CallCount returns the number of poll seam calls made so far.
func (p *Polls) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.calls)
}

// Poll implements eventfeed.PollSource: it logs the call and pops the next
// scripted outcome in order. A stalled call blocks until ctx is done; a done
// ctx returns ctx.Err(); an unscripted call returns an error.
func (p *Polls) Poll(ctx context.Context, cursor eventfeed.Cursor, filters eventfeed.Filters) (eventfeed.PollPage, error) {
	p.mu.Lock()
	call := PollCall{Cursor: cursor, Filters: filters}
	p.calls = append(p.calls, call)
	stalled := p.stalls > 0
	if stalled {
		p.stalls--
	}
	onCall := p.onCall
	p.mu.Unlock()
	if onCall != nil {
		onCall(call)
	}
	if stalled {
		<-ctx.Done()
		return eventfeed.PollPage{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return eventfeed.PollPage{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.script) == 0 {
		return eventfeed.PollPage{}, errors.New("feedtest: unscripted poll call")
	}
	r := p.script[0]
	p.script = p.script[1:]
	return r.page, r.err
}
