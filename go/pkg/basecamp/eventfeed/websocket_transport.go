package eventfeed

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/coder/websocket"
)

// WebSocketTransport is the default CableTransport (design Decision 6):
// github.com/coder/websocket behind the cable seam. Dial applies the
// cable-URL policy before any network I/O, performs the handshake under the
// caller's ctx (the connector owns the handshake deadline), negotiates
// subprotocol "actioncable-v1-json", sends no Origin header, refuses
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
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
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
		return nil, &DialError{Kind: DialTransient, Err: redactDialErr(err, wsURL)}
	}
	if maxFrameBytes > 0 {
		conn.SetReadLimit(maxFrameBytes)
	} else {
		conn.SetReadLimit(-1)
	}
	return &wsConn{conn: conn}, nil
}

// redactDialErr rebuilds a dial failure's message with the dialed URL's query
// string redacted: handshake failures wrap *url.Error, whose rendering embeds
// the full request URL — and the ticket rides in its query string. The
// rebuilt error is deliberately flat (no Unwrap): re-exposing the original
// chain would re-expose the unredacted URL.
func redactDialErr(err error, wsURL string) error {
	msg := err.Error()
	if u, perr := url.Parse(wsURL); perr == nil && u.RawQuery != "" {
		msg = strings.ReplaceAll(msg, "?"+u.RawQuery, "?[redacted]")
	}
	return errors.New(msg)
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

// Close implements CableConn: idempotent, safe from any goroutine, unblocks
// ReadFrame and WriteFrame. The first call attempts the graceful close
// handshake and falls back to tearing the socket down; repeats are no-ops.
func (c *wsConn) Close(code int, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	if err := c.conn.Close(websocket.StatusCode(code), reason); err != nil {
		// The graceful handshake failed (peer already gone, socket dead);
		// make sure the underlying socket is torn down so pending reads and
		// writes unblock regardless.
		_ = c.conn.CloseNow()
		return err
	}
	return nil
}
