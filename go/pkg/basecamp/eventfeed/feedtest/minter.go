package feedtest

import (
	"context"
	"errors"
	"sync"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// Minter is a scripted eventfeed.TicketMinter: tests queue mint outcomes
// (tickets and classified errors) in order, and read back how many seam
// calls the connector made. Every call is counted — including stalled and
// cancelled ones — so seam-call counts stay honest (SPEC.md §23 "Seam-Call
// Semantics"). An unscripted call fails visibly rather than fabricating a
// ticket: strictness is the default, as in the tier-2 scenario family.
type Minter struct {
	mu     sync.Mutex
	script []mintResult
	calls  int
	stalls int
}

var _ eventfeed.TicketMinter = (*Minter)(nil)

type mintResult struct {
	ticket eventfeed.StreamTicket
	err    error
}

// NewMinter returns an empty scripted minter: every call fails as unscripted
// until tickets or errors are queued.
func NewMinter() *Minter {
	return &Minter{}
}

// ScriptTicket queues one successful mint (FIFO with ScriptError).
func (m *Minter) ScriptTicket(t eventfeed.StreamTicket) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.script = append(m.script, mintResult{ticket: t})
}

// ScriptError queues one failed mint (FIFO with ScriptTicket). Pass a
// *eventfeed.MintError to exercise the connector's mint classification.
func (m *Minter) ScriptError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.script = append(m.script, mintResult{err: err})
}

// StallNext scripts the next call to block until its context is done and
// then return the context's error — the teardown-mid-mint case.
func (m *Minter) StallNext() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stalls++
}

// Calls returns the number of mint seam calls made so far.
func (m *Minter) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// MintStreamTicket implements eventfeed.TicketMinter: it pops the next
// scripted outcome in order. A stalled call blocks until ctx is done; a done
// ctx returns ctx.Err(); an unscripted call returns an error.
func (m *Minter) MintStreamTicket(ctx context.Context) (eventfeed.StreamTicket, error) {
	m.mu.Lock()
	m.calls++
	stalled := m.stalls > 0
	if stalled {
		m.stalls--
	}
	m.mu.Unlock()
	if stalled {
		<-ctx.Done()
		return eventfeed.StreamTicket{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return eventfeed.StreamTicket{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.script) == 0 {
		return eventfeed.StreamTicket{}, errors.New("feedtest: unscripted mint call")
	}
	r := m.script[0]
	m.script = m.script[1:]
	return r.ticket, r.err
}
