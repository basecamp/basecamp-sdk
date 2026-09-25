package eventfeed

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/basecamp/actioncable-go"
)

// WebSocketTransport is the default CableTransport: actioncable-go's
// WebSocketTransport behind the cable seam. Dial applies the cable-URL policy
// before any network I/O, performs the handshake under the caller's ctx (the
// connector owns the handshake deadline), negotiates subprotocol
// "actioncable-v1-json" and refuses a handshake that did not select it, sends
// no Origin header, refuses redirects, and enforces the dial's maxFrameBytes
// while reading — an over-limit message is refused on its length, before any
// of it is read in. No error it returns ever carries the dialed URL's query
// string: the ticket rides in it.
//
// It is deliberately configuration-free. A host that needs different dial
// behavior — a custom root pool for a self-hosted install, a bespoke proxy
// policy — implements CableTransport itself; that is what the seam is for.
//
// The cable origin is chosen by the SERVER: the mint returns a url and the
// connector dials it verbatim, cross-host by design (SPEC.md §23
// "Classification: Infrastructure, Not a Composite"). The short-lived ticket
// in its query is the only credential that origin is entitled to, which is
// why the dial goes through actioncable-go's own RFC 6455 client rather than
// any *http.Client: a RoundTripper may inject Authorization, a Jar attaches
// cookies, a TLSClientConfig may present a client certificate, and
// http.DefaultClient can have been given any of those by any library in the
// process. actioncable-go's transport opens a TCP connection, speaks TLS with
// the system roots and no client certificate, and writes the upgrade request
// itself, so nothing ambient can ride along. It consults no proxy for the
// same reason: a CONNECT target is a server-selected host, and any
// server-controlled component can be the ticket.
type WebSocketTransport struct{}

var _ CableTransport = (*WebSocketTransport)(nil)

// cableSubprotocol is the Action Cable subprotocol the dial negotiates.
const cableSubprotocol = actioncable.SubprotocolV1JSON

// cableHandshakeTimeout bounds a dial whose ctx carries no deadline of its
// own. The connector's ctx never does — its handshake deadline is a timer on
// the injected Clock that cancels the ctx when it fires — so this only has to
// be longer than that deadline for the connector's to stay the one that
// decides.
const cableHandshakeTimeout = 30 * time.Second

// errCableConnClosed is what reads and writes return after a local Close — a
// plain error, deliberately not a *CloseError, which is reserved for a peer
// close.
var errCableConnClosed = errors.New("eventfeed: cable connection closed")

// Dial implements CableTransport: policy pre-check, handshake, read limit.
func (t *WebSocketTransport) Dial(ctx context.Context, wsURL string, maxFrameBytes int64) (CableConn, error) {
	// Checked before anything else, ctx included: a configuration bug should
	// surface as itself, not be masked by whichever transient condition also
	// held. There is no unlimited mode to fall into — the parameter exists to
	// bind the cap inside the WebSocket stack (SPEC.md §23).
	if maxFrameBytes <= 0 {
		return nil, usageError("cable dial max frame bytes must be positive")
	}
	if derr := checkCableURL(wsURL); derr != nil {
		return nil, derr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// A fresh transport per dial, because the frame cap is the dial's.
	dialer := &actioncable.WebSocketTransport{
		HandshakeTimeout: cableHandshakeTimeout,
		MaxMessageSize:   maxFrameBytes,
	}
	// No Header at all: an Origin is what a browser sends, and the connector
	// is not one (SPEC.md §23 "Cable Protocol Details").
	conn, err := dialer.Dial(ctx, wsURL, actioncable.DialOptions{
		Subprotocols: []string{cableSubprotocol},
	})
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		var refused *actioncable.HandshakeError
		if errors.As(err, &refused) {
			if isRedirectStatus(refused.StatusCode) {
				// Any 3xx — with a valid, absent, or malformed Location
				// alike; the header is never parsed. A fresh mint returns the
				// same redirecting URL, so retrying cannot help.
				return nil, &DialError{Kind: DialPolicy, Reason: "cable URL redirected; redirects are refused"}
			}
			return nil, &DialError{Kind: DialTransient, Err: dialFailure(err, refused.StatusCode)}
		}
		return nil, &DialError{Kind: DialTransient, Err: dialFailure(err, 0)}
	}
	// Exact equality: RFC 6455 subprotocol tokens are case-sensitive, and a
	// case-folded match would treat a protocol this client never offered
	// (say "ActionCable-V1-Json") as successfully negotiated. actioncable-go
	// records what the server selected without judging it, so a server that
	// selected nothing, or something never offered, arrives here as a live
	// connection that never agreed to speak Action Cable.
	//
	// Policy, not transient. The classification asks whether a retry can
	// differ, and a fresh mint returns a URL pointing at the same server,
	// which selects the same thing — so this is the redirect case, not the
	// refused-connection case: Terminal(invalid_cable_url) surfaces a server
	// that cannot speak the protocol instead of reconnecting against it
	// forever. The selected value is peer-controlled text and is never
	// rendered.
	if conn.Subprotocol() != cableSubprotocol {
		_ = conn.Close()
		return nil, &DialError{Kind: DialPolicy, Reason: "cable server did not negotiate the " + cableSubprotocol + " subprotocol"}
	}
	return &wsConn{conn: conn}, nil
}

// dialFailure renders a transient dial failure WITHOUT forwarding any text
// the library or the peer produced.
//
// SPEC.md §23 declares the ticket an "opaque bearer credential; never
// logged", riding in the mint URL's query string. OPAQUE is the operative
// word: to strip a credential out of arbitrary text you must MODEL it — its
// length, its encoding, whether it is a query value, a bare token, or a key —
// and every such model is precisely the assumption the contract forbids. The
// peer chooses the input, and the reflection surface is any header the
// handshake quotes back.
//
// So nothing peer-influenced is forwarded. The classification the connector
// actually acts on (DialTransient, and the reconnect cycle behind it) is
// unchanged; only the human-readable cause narrows, drawn from a CLOSED
// vocabulary keyed on error TYPES — ours and the standard library's — never on
// rendered text. An unrecognized cause degrades to the generic message, so the
// failure direction is "less diagnostic", never "leaks".
//
// status is the handshake response's code when there was one, else 0. It is
// the one genuinely useful diagnostic drawn from a FINITE, semantically
// defined set — which is what the closed-vocabulary policy asks of a
// rendering — and the range guard is what closes the set by construction:
// net/http accepts any three-character Atoi-parseable status line, so a
// hostile server can answer 999 or a zero-padded 007, and only 100-599 is
// rendered; anything else collapses to a fixed digit-free marker. A
// three-digit coincidence with an opaque ticket reconstructs nothing — the
// ticket is a long opaque string, and the server answering already holds it
// — while which status refused the handshake (401 vs 429 vs 503) is real
// operational triage.
func dialFailure(err error, status int) error {
	cause := dialFailureCause(err)
	if status != 0 {
		if status >= 100 && status <= 599 {
			return fmt.Errorf("eventfeed: cable dial failed: %s (server answered HTTP %d)", cause, status)
		}
		return fmt.Errorf("eventfeed: cable dial failed: %s (server answered an HTTP status outside the standard range)", cause)
	}
	return fmt.Errorf("eventfeed: cable dial failed: %s", cause)
}

// dialFailureCause maps a dial error onto the closed vocabulary. Every arm
// matches on a TYPE or a sentinel, never on message text, so no arm can be
// widened by something a peer wrote.
func dialFailureCause(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, os.ErrDeadlineExceeded):
		return "the handshake deadline lapsed"
	case errors.Is(err, context.Canceled):
		return "the dial was cancelled"
	}
	var refused *actioncable.HandshakeError
	if errors.As(err, &refused) {
		return "the server refused the upgrade"
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

// wsConn adapts one actioncable-go connection to the CableConn seam.
type wsConn struct {
	conn actioncable.Conn

	mu     sync.Mutex
	closed bool
}

var _ CableConn = (*wsConn)(nil)

func (c *wsConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// ReadFrame implements CableConn: the next text frame verbatim. actioncable-go
// answers WebSocket pings itself during reads; Action Cable pings are TEXT
// frames and flow through like everything else. Error precedence:
// cancellation, local close, peer close (*CloseError), the size-limit refusal
// (ErrFrameOversize — refused on the frame's length, before the payload is
// read, and the connection is dead from then on), then the raw read failure.
//
// A binary frame reaches the connector as its bytes, since the library does
// not say which opcode carried a message. Action Cable is text-only, so those
// bytes fail to parse as a frame and take the invalid-frame socket-failure
// dispatch — the same disposition a refusal here would have taken.
func (c *wsConn) ReadFrame(ctx context.Context) ([]byte, error) {
	// The precedence holds on entry too, not only on the way out. Checking the
	// local close first reversed it whenever both were already true — the
	// shutdown a run loop performs, cancel then close — and reported a
	// cancelled read as a connection failure. WriteFrame checks the context
	// first; the two must not disagree.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.isClosed() {
		return nil, errCableConnClosed
	}
	data, err := c.conn.Read(ctx)
	if err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if c.isClosed() {
			return nil, errCableConnClosed
		}
		var ce *actioncable.CloseError
		if errors.As(err, &ce) {
			return nil, &CloseError{Code: ce.Code, Reason: ce.Reason}
		}
		if errors.Is(err, actioncable.ErrMessageTooBig) {
			// Returned flat: the sentinel is the whole classification, and
			// the frame was never materialized, so the library's rendering
			// adds nothing a caller may act on.
			return nil, ErrFrameOversize
		}
		// The raw fallthrough is deliberately NOT flattened, and the
		// asymmetry with dialFailure is the point. A dial error can render
		// the full ticket-bearing URL; a post-handshake read error cannot
		// render any dialed-URL component by construction: this conn retains
		// no URL, a TCP error's address is the RESOLVED IP plus the connected
		// port — a number in 1-65535, which an opaque ticket cannot be — and
		// every peer-chosen text channel is mapped above (close reasons, the
		// read limit). Flattening would spend the one genuinely diagnostic
		// cause — reset vs timeout vs EOF — to remove text that cannot carry
		// a credential; the run loop's observer vocabulary reduces it for
		// logging surfaces regardless.
		return nil, err
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
	if err := c.conn.Write(ctx, data); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if c.isClosed() {
			return errCableConnClosed
		}
		return err
	}
	return nil
}

// Close implements CableConn: idempotent, safe from any goroutine, unblocks
// ReadFrame and WriteFrame. The first call writes the close frame and closes
// the socket; repeats are no-ops.
//
// The close is bounded by construction. §23 requires the peer to SEE a close
// frame — liveConn.dispose closes before cancelling the attempt precisely so
// it does — and actioncable-go writes that frame under a one-second deadline
// and then closes the socket without waiting for the peer's answer, which
// the connector never reads. So a peer that ignores the close handshake costs
// at most that second, and closing the socket is what unblocks a read or
// write parked inside the library.
func (c *wsConn) Close(code int, reason string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	if closer, ok := c.conn.(actioncable.StatusCloser); ok {
		return closer.CloseWithStatus(code, reason)
	}
	return c.conn.Close()
}
