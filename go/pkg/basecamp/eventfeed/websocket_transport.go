package eventfeed

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
//
// It is deliberately configuration-free. A host that needs different dial
// behavior — a custom root pool for a self-hosted install, a bespoke proxy
// policy — implements CableTransport itself; that is what the seam is for. It
// must not be done by handing this transport an *http.Client, for the reason
// in cableHTTPClient.
type WebSocketTransport struct{}

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

// proxyFromEnvironment is net/http's environment proxy resolution, indirected
// only so cableProxy's rule can be tested deterministically:
// http.ProxyFromEnvironment reads HTTP_PROXY and friends behind a package-wide
// sync.Once, so a test that sets them is silently vacuous whenever anything
// earlier in the binary already resolved a proxy.
var proxyFromEnvironment = http.ProxyFromEnvironment

// cableProxy resolves the handshake's proxy, and refuses to use one for a
// cleartext dial.
//
// A wss:// handshake reaches a proxy as CONNECT host:port — the proxy learns
// which host is being reached and nothing more, so the ticket stays inside the
// tunnel. A ws:// handshake is forwarded in absolute form, which puts
// "/cable?ticket=…" in the proxy's request line, its access log, and every hop
// in between, in the clear.
//
// That combination is reachable rather than theoretical. SPEC.md §9's carve-out
// admits ws:// for *.localhost, and net/http's proxy rules exempt the literal
// "localhost" and loopback IPs but NOT *.localhost subdomains — so
// "ws://app.localhost:3000/cable?ticket=…" with HTTP_PROXY set in the
// environment is proxied, cleartext, ticket included.
func cableProxy(req *http.Request) (*url.URL, error) {
	if req.URL.Scheme != "https" {
		return nil, nil
	}
	return proxyFromEnvironment(req)
}

// cableHTTPClient performs every cable handshake. It is package-owned and
// takes nothing from the caller, which is the point.
//
// The cable origin is chosen by the SERVER: the mint returns a url and the
// connector dials it verbatim, cross-host by design (SPEC.md §23 "Classification:
// Infrastructure, Not a Composite"). The short-lived ticket in its query is
// the only credential that origin is entitled to. An *http.Client accepted
// from the caller carries three more, none of them visible at the call site: a
// RoundTripper may inject Authorization, a Jar attaches cookies, and a
// TLSClientConfig may present a client certificate. Falling back to
// http.DefaultClient is the same hazard with no call site at all — any library
// in the process can have installed a credential-bearing default, and the
// handshake would forward it to a host named by a response.
//
// A RoundTripper is opaque, so a caller-supplied client cannot be inspected
// and rejected at runtime. Owning the client outright is the only form this
// boundary can take.
//
// Field by field: Proxy is cableProxy, not http.ProxyFromEnvironment. No Jar,
// so no cookie is ever attached. No TLSClientConfig, so verification uses the
// system roots and no client certificate is presented. The timeouts and idle
// bounds mirror http.DefaultTransport's, which they exist to replace rather
// than to tune — and the idle bounds must be spelled out, because the zero
// values are UNBOUNDED, not defaults: the cable origin is server-selected
// per dial, so a rotating or hostile mint topology hands every reconnect a
// fresh host, each failed (non-101) handshake parks reusable idle
// connections, and with no cap and no timeout nothing ever closes them —
// file-descriptor exhaustion on a long-running feed.
var cableHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: cableProxy,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// The handshake must be HTTP/1.1: the upgrade depends on hijacking
		// the connection, which HTTP/2 does not offer.
		ForceAttemptHTTP2: false,
	},
	// Every redirect is refused (a fresh mint returns the same redirecting
	// URL, so retrying cannot help), and refusing them is also what stops a
	// redirect from carrying the ticket to a second, unvetted origin.
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return errRedirectRefused
	},
}

// Dial implements CableTransport: policy pre-check, handshake, read limit.
func (t *WebSocketTransport) Dial(ctx context.Context, wsURL string, maxFrameBytes int64) (CableConn, error) {
	// Checked before anything else, ctx included: a configuration bug should
	// surface as itself, not be masked by whichever transient condition also
	// held. There is no unlimited mode to fall into — the parameter exists to
	// bind the cap inside the WebSocket stack (SPEC.md §23), and passing the
	// library's "no limit" sentinel instead would fail open on exactly the
	// property the parameter enforces.
	if maxFrameBytes <= 0 {
		return nil, usageError("cable dial max frame bytes must be positive")
	}
	if derr := checkCableURL(wsURL); derr != nil {
		return nil, derr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// cableHTTPClient, never the caller's and never http.DefaultClient.
	// coder/websocket shallow-copies it and chains to its CheckRedirect, so
	// the redirect sentinel survives the library's own defaults.
	//nolint:bodyclose // coder/websocket owns the handshake response ("You
	// never need to close resp.Body yourself" — Dial docs): on failure it
	// reads and replaces the body itself, and on success the body is the
	// upgraded connection, closed via CableConn.Close.
	conn, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPClient:   cableHTTPClient,
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
		if resp != nil && resp.StatusCode >= 300 && resp.StatusCode < 400 {
			// A 3xx with NO Location never invokes CheckRedirect — net/http
			// hands it back as a normal answer — so the sentinel above
			// cannot see it. It is still the redirect class, and permanence
			// is what decides the kind: a fresh mint returns the same
			// redirecting endpoint, so transient would re-mint forever. (A
			// 3xx whose Location fails to PARSE errors inside net/http
			// before CheckRedirect, untyped and with no response retained;
			// classifying it would take message-text matching, which the
			// closed vocabulary forbids — that one shape stays transient,
			// bounded by the reconnect cycle.)
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
	conn.SetReadLimit(maxFrameBytes)
	lifetime, endLifetime := context.WithCancel(context.Background())
	return &wsConn{conn: conn, lifetime: lifetime, endLifetime: endLifetime}, nil
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
// one genuinely useful diagnostic drawn from a FINITE, semantically defined
// set — which is what the closed-vocabulary policy asks of a rendering — and
// the range guard is what closes the set by construction: net/http accepts
// any three-character Atoi-parseable status line, so a hostile server can
// answer 999 or a zero-padded 007, and only 100-599 is rendered; anything
// else collapses to a fixed digit-free marker. A three-digit coincidence
// with an opaque ticket reconstructs nothing — the ticket is a long opaque
// string, and the server answering already holds it — while which status
// refused the handshake (401 vs 429 vs 503) is real operational triage.
func dialFailure(err error, resp *http.Response) error {
	cause := dialFailureCause(err)
	if resp != nil && resp.StatusCode != 0 {
		if resp.StatusCode >= 100 && resp.StatusCode <= 599 {
			return fmt.Errorf("eventfeed: cable dial failed: %s (server answered HTTP %d)", cause, resp.StatusCode)
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

	// lifetime is the connection-owned cancellation path CableConn.Close's
	// "unblocks ReadFrame and WriteFrame" needs, cancelled by endLifetime once
	// Close is done waiting on the graceful handshake. Every read and write
	// runs under a context derived from BOTH it and the caller's, which is
	// what lets a Close that returned on its own budget reach I/O still parked
	// inside the library. See underLifetime.
	lifetime    context.Context
	endLifetime context.CancelFunc

	mu     sync.Mutex
	closed bool
}

var _ CableConn = (*wsConn)(nil)

func (c *wsConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// underLifetime derives ctx so that cancelling the connection's lifetime
// cancels it too, returning the derived context and a release func the caller
// must always run.
//
// This is what makes the library's I/O interruptible AT ALL, not merely
// interruptible sooner. coder/websocket installs its cancellation hook only
// when the operation's context has a Done channel — conn.go's setupReadTimeout
// and setupWriteTimeout both return early on a nil Done — so a
// ReadFrame(context.Background()), which is exactly how a run loop parks a read
// pump, hands the library nothing to cancel and cannot be interrupted by
// anything short of the socket dying. The derived context always has a Done
// channel, so the hook is always installed, and cancelling it closes the
// underlying socket and unblocks the operation.
func (c *wsConn) underLifetime(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)
	// AfterFunc on an already-cancelled lifetime runs immediately, so a read
	// racing a Close that already lapsed is cancelled before it blocks.
	stop := context.AfterFunc(c.lifetime, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}

// ReadFrame implements CableConn: the next text frame verbatim.
// coder/websocket answers WS-level ping/pong internally during reads; Action
// Cable pings are TEXT frames and flow through like everything else. Error
// precedence: cancellation, local close, peer close (*CloseError), the
// read-limit abort (ErrFrameOversize — coder/websocket rejects an over-limit
// message during the read, without materializing it, and the connection is
// dead from then on), then the raw read failure.
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
	// The caller's ctx is what the error precedence below reports on; readCtx
	// only adds the connection's own cancellation.
	readCtx, release := c.underLifetime(ctx)
	defer release()
	typ, data, err := c.conn.Read(readCtx)
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
		if errors.Is(err, websocket.ErrMessageTooBig) {
			// Returned flat: the sentinel is the whole classification, and
			// the frame was never materialized, so the library's rendering
			// adds nothing a caller may act on.
			return nil, ErrFrameOversize
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
	writeCtx, release := c.underLifetime(ctx)
	defer release()
	if err := c.conn.Write(writeCtx, websocket.MessageText, data); err != nil {
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
// Past the budget the handshake is left to finish off-caller, and the
// connection's lifetime is cancelled — which is what makes returning early a
// bound rather than a relocation of the wait. Waiting for coder/websocket to
// tear the socket down on its own (close.go calls c.close() once the handshake
// attempt ends, within its 5s+5s ceiling) does unblock a parked read
// eventually, but four seconds after Close returned, so a run loop that joins
// its read pump pays the same stall this budget was written to remove.
// Cancelling the lifetime closes the socket now instead, through the
// cancellation hook every read and write installs (underLifetime).
//
// Cancelling AFTER the budget, never before, is what keeps the close frame
// intact: writeClose is the first thing the library's Close does, so by the
// time the budget lapses the frame has been written to an open socket
// (microseconds, bounded by the kernel's send buffer, not the peer) and only
// the politeness phase is still running. What the caller trades for returning
// early is the last word on whether the peer acknowledged.
//
// # A concurrent second caller returns nil before the first has finished
//
// The latch is taken before the handshake, so a Close racing another Close
// reports success while the first is still inside its budget. That is the
// intended trade, not an oversight, and the alternative was considered: making
// repeats wait on a shared completion signal would make every extra caller
// block for up to closeGraceBudget, which is precisely the property the layer
// above refuses — Connector.Close is documented as callable from inside a
// consumer callback and must never wait on host I/O.
//
// Nothing is unsound about the early return. The FIRST caller's budget still
// bounds the teardown for everyone: the lifetime is cancelled either way,
// within closeGraceBudget, and every parked read and write is unblocked by
// that cancellation rather than by any caller's return. So what a second
// caller loses is the peer's acknowledgement — the same thing the first caller
// trades away past the budget — and never the teardown itself. The connector
// itself never races here: it closes each attempt's socket once, from the run
// goroutine.
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
		// The library closed the socket itself; cancelling now only releases
		// the lifetime context.
		c.endLifetime()
		return err
	case <-time.After(closeGraceBudget):
		c.endLifetime()
		return errCloseNotAcknowledged
	}
}
