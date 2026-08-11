package feedtest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// Transport is a scripted eventfeed.CableTransport: it records every dial
// (URL verbatim, query string included, plus the max-frame-bytes cap), hands
// out one fresh scriptable Conn per successful dial, and can be scripted to
// fail or stall dials. It honors the seam's cancellation contract — a dial
// whose context is done returns promptly — and, like the whole package, is
// deterministic: nothing here touches the wall clock.
type Transport struct {
	mu       sync.Mutex
	dials    []Dial
	conns    []*Conn
	dialErrs []error
	stalls   int
}

var _ eventfeed.CableTransport = (*Transport)(nil)

// Dial records one CableTransport.Dial call.
type Dial struct {
	// URL is the dialed URL, verbatim.
	URL string
	// MaxFrameBytes is the read cap the dial carried.
	MaxFrameBytes int64
}

// NewTransport returns an empty scripted transport: every dial succeeds with
// a fresh Conn until scripted otherwise.
func NewTransport() *Transport {
	return &Transport{}
}

// Dial implements eventfeed.CableTransport. Every call is recorded first —
// including cancelled and scripted-failure dials, so seam-call counts stay
// honest. A stalled dial blocks until ctx is done; a done ctx returns
// ctx.Err(); a scripted failure pops in FIFO order; otherwise a fresh Conn
// carrying maxFrameBytes is returned.
func (tr *Transport) Dial(ctx context.Context, wsURL string, maxFrameBytes int64) (eventfeed.CableConn, error) {
	tr.mu.Lock()
	tr.dials = append(tr.dials, Dial{URL: wsURL, MaxFrameBytes: maxFrameBytes})
	stalled := tr.stalls > 0
	if stalled {
		tr.stalls--
	}
	tr.mu.Unlock()
	if stalled {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.dialErrs) > 0 {
		err := tr.dialErrs[0]
		tr.dialErrs = tr.dialErrs[1:]
		return nil, err
	}
	conn := newConn(maxFrameBytes)
	tr.conns = append(tr.conns, conn)
	return conn, nil
}

// FailNextDial scripts the next un-stalled dial to fail with err (FIFO when
// queued repeatedly).
func (tr *Transport) FailNextDial(err error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.dialErrs = append(tr.dialErrs, err)
}

// StallNextDial scripts the next dial to block until its context is done and
// then return the context's error — the deadline-expiry-mid-dial case.
func (tr *Transport) StallNextDial() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.stalls++
}

// Dials returns every recorded dial, in order.
func (tr *Transport) Dials() []Dial {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.dials)
}

// DialedURLs returns every recorded dial URL, verbatim, in order.
func (tr *Transport) DialedURLs() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	urls := make([]string, len(tr.dials))
	for i, d := range tr.dials {
		urls[i] = d.URL
	}
	return urls
}

// Conns returns every Conn handed out, in dial order.
func (tr *Transport) Conns() []*Conn {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.conns)
}

// LastConn returns the most recently handed-out Conn, or nil before the
// first successful dial.
func (tr *Transport) LastConn() *Conn {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.conns) == 0 {
		return nil
	}
	return tr.conns[len(tr.conns)-1]
}

// errConnClosed is what reads and writes return after a local Close — a
// plain error, deliberately not an *eventfeed.CloseError, which is reserved
// for a peer close.
var errConnClosed = errors.New("feedtest: cable connection closed")

// Conn is one scripted eventfeed.CableConn. The test side queues inbound
// frames (Serve), scripts the read tail (ServeClose / FailReads), and reads
// back what the connector did (Writes, Closed, CloseCode, CloseReason).
// ReadFrame delivers queued frames in order, enforcing the dial's
// max-frame-bytes cap the way a real transport must — an over-limit frame is
// rejected as an error, never delivered, and the connection is dead from
// then on. Reads and writes honor the seam's cancellation contract: a done
// context and a local Close both unblock them promptly.
type Conn struct {
	mu   sync.Mutex
	cond *sync.Cond

	maxFrameBytes int64
	pending       [][]byte
	finalErr      error // after pending drains: peer close or scripted read failure
	violation     error // latched max-frame-bytes rejection

	writes   [][]byte
	writeErr error

	closed      bool
	closeCode   int
	closeReason string
	closeCalls  int
}

var _ eventfeed.CableConn = (*Conn)(nil)

func newConn(maxFrameBytes int64) *Conn {
	c := &Conn{maxFrameBytes: maxFrameBytes}
	c.cond = sync.NewCond(&c.mu)
	return c
}

// Serve queues one inbound text frame for ReadFrame to deliver.
func (c *Conn) Serve(frame []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending = append(c.pending, slices.Clone(frame))
	c.cond.Broadcast()
}

// ServeClose scripts a peer close: after the queued frames drain, ReadFrame
// returns *eventfeed.CloseError with the given WebSocket status.
func (c *Conn) ServeClose(code int, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.finalErr = &eventfeed.CloseError{Code: code, Reason: reason}
	c.cond.Broadcast()
}

// FailReads scripts an abrupt read failure (a severed TCP connection): after
// the queued frames drain, ReadFrame returns err — deliberately not a
// *eventfeed.CloseError.
func (c *Conn) FailReads(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.finalErr = err
	c.cond.Broadcast()
}

// FailWrites scripts every subsequent WriteFrame to fail with err.
func (c *Conn) FailWrites(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeErr = err
	c.cond.Broadcast()
}

// ReadFrame implements eventfeed.CableConn: the next queued frame, verbatim.
// Precedence at each wakeup: context cancellation, local close, a latched
// frame-size violation, queued frames (checking the cap as each is
// dequeued), then the scripted read tail; otherwise it blocks.
func (c *Conn) ReadFrame(ctx context.Context) ([]byte, error) {
	stop := context.AfterFunc(ctx, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.cond.Broadcast()
	})
	defer stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	for {
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case c.closed:
			return nil, errConnClosed
		case c.violation != nil:
			return nil, c.violation
		case len(c.pending) > 0:
			frame := c.pending[0]
			c.pending = c.pending[1:]
			if c.maxFrameBytes > 0 && int64(len(frame)) > c.maxFrameBytes {
				// Enforced while reading, never materialized to the caller;
				// the connection is dead from here, as with a real
				// read-limit breach.
				c.violation = fmt.Errorf("feedtest: inbound frame of %d bytes exceeds max frame bytes %d", len(frame), c.maxFrameBytes)
				return nil, c.violation
			}
			return frame, nil
		case c.finalErr != nil:
			return nil, c.finalErr
		default:
			c.cond.Wait()
		}
	}
}

// WriteFrame implements eventfeed.CableConn: it records the frame verbatim.
// A done context, a local Close, and a scripted write failure each fail the
// write instead.
func (c *Conn) WriteFrame(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errConnClosed
	}
	if c.writeErr != nil {
		return c.writeErr
	}
	c.writes = append(c.writes, slices.Clone(data))
	c.cond.Broadcast()
	return nil
}

// Close implements eventfeed.CableConn: idempotent, safe from any goroutine,
// unblocks ReadFrame and WriteFrame. The first call's code and reason are
// recorded; repeats are counted no-ops.
func (c *Conn) Close(code int, reason string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeCalls++
	if c.closed {
		return nil
	}
	c.closed = true
	c.closeCode = code
	c.closeReason = reason
	c.cond.Broadcast()
	return nil
}

// Writes returns every frame the connector wrote, verbatim, in order.
func (c *Conn) Writes() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.writes))
	for i, w := range c.writes {
		out[i] = slices.Clone(w)
	}
	return out
}

// Closed reports whether the connector closed the connection.
func (c *Conn) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// CloseCalls returns how many times Close was called (idempotency
// accounting).
func (c *Conn) CloseCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeCalls
}

// CloseCode returns the first Close call's code.
func (c *Conn) CloseCode() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeCode
}

// CloseReason returns the first Close call's reason.
func (c *Conn) CloseReason() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeReason
}

// MaxFrameBytes returns the cap the dial carried.
func (c *Conn) MaxFrameBytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxFrameBytes
}
