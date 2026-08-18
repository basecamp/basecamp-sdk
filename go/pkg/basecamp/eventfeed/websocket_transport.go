package eventfeed

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// WebSocketTransport is the default CableTransport (design Decision 6):
// github.com/coder/websocket behind the cable seam. Dial applies the
// cable-URL policy before any network I/O, performs the handshake under the
// caller's ctx (the connector owns the handshake deadline), negotiates
// subprotocol "actioncable-v1-json" and refuses a handshake that did not
// select it, sends no Origin header, refuses
// redirects, and enforces the dial's maxFrameBytes while reading via the
// socket read limit — an over-limit message aborts the read without being
// materialized. No error it returns ever carries the dialed URL's query
// string: the ticket rides in it.
type WebSocketTransport struct {
	// HTTPClient performs the WebSocket handshake. nil means
	// http.DefaultClient. Dial only — frames ride the upgraded connection.
	// Its redirect policy is overridden for the handshake: redirects are
	// refused per SPEC §23 Security Invariants.
	HTTPClient *http.Client
}

var _ CableTransport = (*WebSocketTransport)(nil)

// cableSubprotocol is the Action Cable subprotocol the dial negotiates.
const cableSubprotocol = "actioncable-v1-json"

// errRedirectRefused is the CheckRedirect sentinel: any redirect during the
// handshake refuses the dial as a policy failure (a fresh mint returns the
// same redirecting URL, so retrying cannot help).
var errRedirectRefused = errors.New("eventfeed: cable dial redirect refused")

// errCableConnClosed is what reads and writes return after a local Close — a
// plain error, deliberately not a *CloseError, which is reserved for a peer
// close.
var errCableConnClosed = errors.New("eventfeed: cable connection closed")

// Dial implements CableTransport: policy pre-check, handshake, read limit.
func (t *WebSocketTransport) Dial(ctx context.Context, wsURL string, maxFrameBytes int64) (CableConn, error) {
	if derr := checkCableURL(wsURL); derr != nil {
		return nil, derr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	base := t.HTTPClient
	if base == nil {
		base = http.DefaultClient
	}
	// Shallow copy so the caller's client keeps its own redirect policy
	// elsewhere; the handshake itself always refuses redirects.
	// coder/websocket clones this client again and chains to our
	// CheckRedirect, so the sentinel survives its defaults.
	client := *base
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errRedirectRefused
	}
	//nolint:bodyclose // coder/websocket owns the handshake response ("You
	// never need to close resp.Body yourself" — Dial docs): on failure it
	// reads and replaces the body itself, and on success the body is the
	// upgraded connection, closed via CableConn.Close.
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient:   &client,
		Subprotocols: []string{cableSubprotocol},
	})
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if errors.Is(err, errRedirectRefused) {
			// Never attach the underlying chain: its *url.Error renders the
			// redirect target, which can carry the ticket too.
			return nil, &DialError{Kind: DialPolicy, Reason: "cable URL redirected; redirects are refused"}
		}
		return nil, &DialError{Kind: DialTransient, Err: dialFailure(err, resp)}
	}
	if negotiated := conn.Subprotocol(); !strings.EqualFold(negotiated, cableSubprotocol) {
		// coder/websocket verifies the server's selection only when it made
		// one: verifySubprotocol (dial.go) returns nil for an ABSENT
		// Sec-WebSocket-Protocol, so a 101 that selected nothing arrives here
		// as a healthy connection that never agreed to speak Action Cable. A
		// mismatched selection the library already rejects, which leaves the
		// empty case as the only one this can see — the comparison is
		// case-insensitive to match the library's own (it accepts a
		// case-varying echo), so nothing the library passed is refused here
		// for spelling alone.
		//
		// CloseNow, not a graceful close: no Action Cable session exists to
		// close down (nothing was subscribed), and the dial is on the
		// connector's handshake deadline, which a close handshake with an
		// already-misbehaving peer would eat.
		_ = conn.CloseNow()
		// Policy, not transient. The classification asks whether a retry can
		// differ, and a fresh mint returns a URL pointing at the same server,
		// which selects the same nothing — so this is the redirect case, not
		// the refused-connection case: Terminal(invalid_cable_url) surfaces a
		// server that cannot speak the protocol instead of reconnecting
		// against it forever. The counterargument — a proxy stripping the
		// header transiently — argues for a loud stop even harder: an
		// operator can see a terminal, but not an endless backoff cycle.
		return nil, &DialError{Kind: DialPolicy, Reason: "cable server did not negotiate the " + cableSubprotocol + " subprotocol"}
	}
	if maxFrameBytes > 0 {
		conn.SetReadLimit(maxFrameBytes)
	} else {
		conn.SetReadLimit(-1)
	}
	return &wsConn{conn: conn}, nil
}

// dialFailure renders a transient dial failure WITHOUT forwarding any text
// the library or the peer produced.
//
// This replaced a redactor, and the reason is the contract rather than a bug
// count. SPEC.md §23 declares the ticket an "opaque bearer credential; never
// logged", riding in the mint URL's query string. OPAQUE is the operative
// word: to strip a credential out of arbitrary text you must MODEL it — its
// length, its encoding, whether it is a query value, a bare token, or a key —
// and every such model is precisely the assumption the contract forbids.
// Three review rounds found three different spellings that slipped a model in
// turn: a value below a length threshold, a percent-encoded form, and a query
// carrying the credential with no "=" in it at all. That is not three bugs, it
// is one control that has to anticipate its own input, and the peer chooses
// the input — the reflection surface is any header the handshake quotes back.
//
// So nothing peer-influenced is forwarded. The classification the connector
// actually acts on (DialTransient, and the reconnect cycle behind it) is
// unchanged; only the human-readable cause narrows, drawn from a CLOSED
// vocabulary keyed on error TYPES — ours and the standard library's — never on
// rendered text. An unrecognized cause degrades to the generic message, so the
// failure direction is "less diagnostic", never "leaks".
//
// resp is the handshake response when there was one. Its status code is the
// one genuinely useful diagnostic that is structurally incapable of carrying a
// credential: an integer, not a text channel.
func dialFailure(err error, resp *http.Response) error {
	cause := dialFailureCause(err)
	if resp != nil && resp.StatusCode != 0 {
		return fmt.Errorf("eventfeed: cable dial failed: %s (server answered HTTP %d)", cause, resp.StatusCode)
	}
	return fmt.Errorf("eventfeed: cable dial failed: %s", cause)
}

// dialFailureCause maps a dial error onto the closed vocabulary. Every arm
// matches on a TYPE or a sentinel, never on message text, so no arm can be
// widened by something a peer wrote.
func dialFailureCause(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "the handshake deadline lapsed"
	case errors.Is(err, context.Canceled):
		return "the dial was cancelled"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "the cable host did not resolve"
	}
	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return "the cable server's TLS certificate was not verified"
	}
	var recordErr tls.RecordHeaderError
	if errors.As(err, &recordErr) {
		return "the cable server did not speak TLS"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		// Op is one of the standard library's own verbs ("dial", "read",
		// "write") — not peer text.
		return "the connection failed during " + opErr.Op
	}
	return "the handshake did not complete"
}

// wsConn adapts one coder/websocket connection to the CableConn seam.
type wsConn struct {
	conn *websocket.Conn

	mu     sync.Mutex
	closed bool
}

var _ CableConn = (*wsConn)(nil)

func (c *wsConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// ReadFrame implements CableConn: the next text frame verbatim.
// coder/websocket answers WS-level ping/pong internally during reads; Action
// Cable pings are TEXT frames and flow through like everything else. Error
// precedence: cancellation, local close, peer close (*CloseError), then the
// raw read failure (which covers the read-limit abort — coder/websocket
// rejects an over-limit message during the read, without materializing it,
// and the connection is dead from then on).
func (c *wsConn) ReadFrame(ctx context.Context) ([]byte, error) {
	if c.isClosed() {
		return nil, errCableConnClosed
	}
	typ, data, err := c.conn.Read(ctx)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if c.isClosed() {
			return nil, errCableConnClosed
		}
		var ce websocket.CloseError
		if errors.As(err, &ce) {
			return nil, &CloseError{Code: int(ce.Code), Reason: ce.Reason}
		}
		return nil, err
	}
	if typ != websocket.MessageText {
		// Action Cable is text-only; a binary frame is a peer protocol
		// violation the connector dispatches as a socket failure.
		return nil, errors.New("eventfeed: unexpected binary cable frame")
	}
	return data, nil
}

// WriteFrame implements CableConn: one text frame. A done context, a local
// Close, and a dead socket each fail the write.
func (c *wsConn) WriteFrame(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.isClosed() {
		return errCableConnClosed
	}
	return c.conn.Write(ctx, websocket.MessageText, data)
}

// closeGraceBudget bounds how long Close waits on the graceful close
// handshake before returning to its caller.
//
// coder/websocket's Conn.Close is two phases with hardcoded timeouts and no
// way to bound them separately (close.go: writeClose under a 5s context, then
// waitCloseHandshake under another 5s). Only the first phase is contractual —
// §23 requires the peer to SEE a close frame, and liveConn.dispose closes
// before cancelling the attempt precisely so it does — while the second is
// politeness the connector never reads. Left synchronous, that politeness puts
// up to ten seconds in front of caller cancellation, Connector.Close, every
// terminal outcome and every reconnect, all of which must reach the universal
// Closed edge promptly.
//
// One second is the budget because the phase that must complete is a control
// frame write to an open socket — microseconds locally, and bounded by the
// kernel's send buffer, not by the peer — while the phase worth allowing for
// is one close-reply round trip, which a WAN peer answers well inside a
// second. So a live, well-behaved peer still completes the whole handshake
// synchronously, and a peer that ignores it costs a tenth of the library's
// worst case.
const closeGraceBudget = time.Second

// errCloseNotAcknowledged is what Close reports when the peer did not answer
// the close handshake within closeGraceBudget. The close frame was still
// written (it is the first thing the handshake does), so this is a slow or
// silent peer, not a failure to close.
var errCloseNotAcknowledged = errors.New("eventfeed: cable close handshake not acknowledged within the close budget")

// Close implements CableConn: idempotent, safe from any goroutine, unblocks
// ReadFrame and WriteFrame. The first call runs the graceful close handshake
// and waits at most closeGraceBudget for it; repeats are no-ops.
//
// Past the budget the handshake is left to finish off-caller. That is not a
// leak and not an abandoned close: coder/websocket's Conn.Close tears the
// underlying socket down unconditionally once its handshake attempt ends
// (close.go calls c.close() whether the handshake succeeded or not), within
// its own 5s+5s ceiling, so the socket always dies and any read still blocked
// inside the library unblocks with it. What the caller trades for returning
// early is the last word on whether the peer acknowledged — and, if the
// attempt's context is cancelled immediately afterward (dispose does exactly
// that), the tail of a close frame a peer that already ignored it for a second
// might not have flushed anyway.
func (c *wsConn) Close(code int, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	// Deliberately no CloseNow fallback on error: it would be a no-op here.
	// Conn.Close has already closed the socket by the time it returns, and
	// CloseNow while a Close is still in flight takes the library's
	// already-closing branch, which only waits (up to 15s) for that Close's
	// goroutines — the opposite of a prompt teardown.
	done := make(chan error, 1)
	go func() { done <- c.conn.Close(websocket.StatusCode(code), reason) }()
	select {
	case err := <-done:
		return err
	case <-time.After(closeGraceBudget):
		return errCloseNotAcknowledged
	}
}
