// Tier-3 transport tests (design §12): the shared CableTransport contract
// suite from transport_contract_test.go run against the REAL WebSocket
// transport over a local httptest server speaking coder/websocket
// server-side — proving the fake the tier-2 scenarios trust and the transport
// production uses satisfy the same contract — plus the transport-specific
// assertions the contract cannot express: policy refusal before any network
// I/O, handshake headers, and ticket redaction in dial errors. Loopback
// ws:// is allowed by policy per the SPEC §9 carve-out, which is what lets
// the whole tier run against 127.0.0.1.
package eventfeed_test

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

// TestWebSocketTransport_SatisfiesCableTransportContract runs the shared
// contract against the real transport — the same suite the feedtest fake
// runs.
func TestWebSocketTransport_SatisfiesCableTransportContract(t *testing.T) {
	runTransportContract(t, func(t *testing.T) contractHarness {
		return newWSHarness(t)
	})
}

// wsHarness backs the contract suite with a real httptest WebSocket server:
// each accepted connection yields a wsPeer scripting surface, and every
// request's URI is recorded verbatim for DialedTargets.
type wsHarness struct {
	srv       *httptest.Server
	transport *eventfeed.WebSocketTransport
	accepted  chan *wsPeer

	mu      sync.Mutex
	targets []string
	headers []http.Header
	peers   []*wsPeer
}

func newWSHarness(t *testing.T) *wsHarness {
	t.Helper()
	h := &wsHarness{
		transport: &eventfeed.WebSocketTransport{},
		accepted:  make(chan *wsPeer, 8),
	}
	h.srv = httptest.NewServer(http.HandlerFunc(h.handle))
	// LIFO: kill live peer connections first so in-flight (hijacked) handlers
	// return and srv.Close cannot block on them.
	t.Cleanup(h.srv.Close)
	t.Cleanup(h.killPeers)
	return h
}

// handle serves one WebSocket connection until it dies, recording the
// request-line URI verbatim first — even when the upgrade fails.
func (h *wsHarness) handle(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.targets = append(h.targets, r.RequestURI)
	h.headers = append(h.headers, r.Header.Clone())
	h.mu.Unlock()
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols: []string{"actioncable-v1-json"},
	})
	if err != nil {
		return
	}
	peer := newWSPeer(c)
	h.mu.Lock()
	h.peers = append(h.peers, peer)
	h.mu.Unlock()
	h.accepted <- peer
	//nolint:contextcheck // deliberate: the peer outlives the handshake
	// request — after the upgrade the connection's lifetime is scripted by
	// the test (Serve/Close/killPeers), not bound to r.Context().
	peer.run()
}

func (h *wsHarness) killPeers() {
	h.mu.Lock()
	peers := slices.Clone(h.peers)
	h.mu.Unlock()
	for _, p := range peers {
		p.releaseStall()
		_ = p.conn.CloseNow()
	}
}

// Dial implements contractHarness: target is a path-and-query suffix on the
// test server, dialed through the real transport as loopback ws://.
func (h *wsHarness) Dial(ctx context.Context, target string, maxFrameBytes int64) (eventfeed.CableConn, contractPeerConn, error) {
	wsURL := "ws" + strings.TrimPrefix(h.srv.URL, "http") + target
	conn, err := h.transport.Dial(ctx, wsURL, maxFrameBytes)
	if err != nil {
		return nil, nil, err
	}
	select {
	case peer := <-h.accepted:
		return conn, peer, nil
	case <-ctx.Done():
		_ = conn.Close(1000, "")
		return nil, nil, ctx.Err()
	}
}

// DialedTargets implements contractHarness: every request-line URI the
// server saw, verbatim.
func (h *wsHarness) DialedTargets() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.targets)
}

// Headers returns each request's handshake headers, in dial order.
func (h *wsHarness) Headers() []http.Header {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.headers)
}

// wsPeerCmd is one scripted peer action: a frame to send, or (frame == nil) a
// close with code+reason.
type wsPeerCmd struct {
	frame  []byte
	code   websocket.StatusCode
	reason string
}

// wsPeer is the server side of one accepted connection: scripted writes and
// close ride a FIFO channel (so Close happens after queued frames drain, as
// the contract requires) while run's read loop records client frames.
type wsPeer struct {
	conn *websocket.Conn
	cmds chan wsPeerCmd
	dead chan struct{}

	mu       sync.Mutex
	received [][]byte
	stall    chan struct{}
}

func newWSPeer(c *websocket.Conn) *wsPeer {
	// The PEER reads without a limit: the read cap under test is the
	// client's, and the contract's stalled-write cases jam the client with
	// frames past coder/websocket's 32 KiB server-side default.
	c.SetReadLimit(-1)
	return &wsPeer{conn: c, cmds: make(chan wsPeerCmd, 64), dead: make(chan struct{})}
}

// run pumps scripted commands to the client and records inbound frames until
// the connection dies. The read loop also drives coder/websocket's close
// handshake, echoing client closes.
func (p *wsPeer) run() {
	ctx := context.Background()
	go func() {
		for {
			select {
			case cmd := <-p.cmds:
				if cmd.frame != nil {
					if p.conn.Write(ctx, websocket.MessageText, cmd.frame) != nil {
						return
					}
				} else {
					_ = p.conn.Close(cmd.code, cmd.reason)
					return
				}
			case <-p.dead:
				return
			}
		}
	}()
	for {
		if gate := p.stallGate(); gate != nil {
			// A stalled peer stops consuming: parked here, the client's
			// writes back up until its WriteFrame genuinely blocks.
			// releaseStall (test cleanup) reopens the gate so the loop can
			// observe the killed connection and tear down leak-free.
			<-gate
		}
		_, data, err := p.conn.Read(ctx)
		if err != nil {
			break
		}
		p.mu.Lock()
		p.received = append(p.received, data)
		p.mu.Unlock()
	}
	close(p.dead)
	_ = p.conn.CloseNow()
}

// StallWrites implements contractPeerConn: the read loop parks before its
// next Read. The one frame a currently-blocked Read may still consume is
// harmless — the contract's writer loops until it jams.
func (p *wsPeer) StallWrites() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stall == nil {
		p.stall = make(chan struct{})
	}
}

func (p *wsPeer) stallGate() chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stall
}

// releaseStall reopens a stalled peer's gate (idempotent): a closed channel
// lets the read loop run again and die normally with the connection.
func (p *wsPeer) releaseStall() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stall != nil {
		select {
		case <-p.stall:
		default:
			close(p.stall)
		}
	}
}

// Serve implements contractPeerConn.
func (p *wsPeer) Serve(frame []byte) {
	p.cmds <- wsPeerCmd{frame: slices.Clone(frame)}
}

// Close implements contractPeerConn.
func (p *wsPeer) Close(code int, reason string) {
	p.cmds <- wsPeerCmd{code: websocket.StatusCode(code), reason: reason}
}

// Received implements contractPeerConn.
func (p *wsPeer) Received() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i, f := range p.received {
		out[i] = slices.Clone(f)
	}
	return out
}

// TestWebSocketTransport_PolicyRefusalBeforeNetworkIO points a policy-bad URL
// (an http:// scheme) at a live server that counts raw TCP connections and
// asserts the refusal is a policy-kind *DialError issued before ANY network
// I/O — and that the error never carries the ticket-bearing query string.
func TestWebSocketTransport_PolicyRefusalBeforeNetworkIO(t *testing.T) {
	var conns atomic.Int64
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("policy-refused dial reached the HTTP handler")
	}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	tr := &eventfeed.WebSocketTransport{}
	conn, err := tr.Dial(context.Background(), srv.URL+"/cable?ticket=sekrit-ticket-value", 1<<20)
	if conn != nil {
		t.Fatal("policy-bad dial returned a connection")
	}
	var de *eventfeed.DialError
	if !errors.As(err, &de) {
		t.Fatalf("dial error = %v (%T), want *eventfeed.DialError", err, err)
	}
	if de.Kind != eventfeed.DialPolicy {
		t.Errorf("DialError.Kind = %v, want policy", de.Kind)
	}
	if got := conns.Load(); got != 0 {
		t.Errorf("server saw %d connections, want 0 (policy refusal must precede network I/O)", got)
	}
	assertNoTicket(t, err)
}

// TestWebSocketTransport_HandshakeHeaders asserts the dial offers subprotocol
// actioncable-v1-json and sends no Origin header (SPEC §23 Security
// Invariants).
func TestWebSocketTransport_HandshakeHeaders(t *testing.T) {
	h := newWSHarness(t)
	conn, _, err := h.Dial(context.Background(), "/cable", 1<<20)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(1000, "")
	headers := h.Headers()
	if len(headers) != 1 {
		t.Fatalf("server saw %d handshakes, want 1", len(headers))
	}
	if got := headers[0].Get("Sec-WebSocket-Protocol"); !strings.Contains(got, "actioncable-v1-json") {
		t.Errorf("Sec-WebSocket-Protocol = %q, want it to offer actioncable-v1-json", got)
	}
	if got := headers[0].Get("Origin"); got != "" {
		t.Errorf("Origin header = %q, want none", got)
	}
}

// TestWebSocketTransport_OversizedFrameAbortsRead drives a frame past the
// dial's maxFrameBytes and asserts the in-read cap: the read fails as
// ErrFrameOversize — the seam's stable size-violation classification, which
// is what lets the run loop dispatch it as an invalid frame rather than a
// generic socket failure — without delivering the frame, the error is
// neither a peer-close *CloseError nor a cancellation, it never carries the
// ticket-bearing query, and the connection is unusable afterward.
func TestWebSocketTransport_OversizedFrameAbortsRead(t *testing.T) {
	h := newWSHarness(t)
	const frameCap = 64
	conn, peer, err := h.Dial(context.Background(), "/cable?ticket=sekrit-ticket-value", frameCap)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(1000, "")
	peer.Serve([]byte(`{"type":"ping","pad":"` + strings.Repeat("x", frameCap*4) + `"}`))
	_, err = readFrameWithin(context.Background(), t, conn)
	if err == nil {
		t.Fatal("oversized frame was delivered, want read error")
	}
	if !errors.Is(err, eventfeed.ErrFrameOversize) {
		t.Errorf("oversized-frame rejection = %v, want ErrFrameOversize", err)
	}
	var ce *eventfeed.CloseError
	if errors.As(err, &ce) {
		t.Errorf("oversized-frame rejection = *CloseError %v, want a plain read error", ce)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("oversized-frame rejection = %v, want a non-cancellation error", err)
	}
	assertNoTicket(t, err)
	// The connection is dead from here.
	if _, err := readFrameWithin(context.Background(), t, conn); err == nil {
		t.Error("read after an oversized-frame abort succeeded, want error")
	}
}

// TestWebSocketTransport_DialErrorsRedactTicket exercises dial failures that
// happen after real network I/O — a refused TCP connection, a redirecting
// server, and a non-101 handshake response — and asserts none of the
// returned errors carry the ticket-bearing query string (net/http's
// *url.Error renders the full request URL, so the transport must sanitize).
func TestWebSocketTransport_DialErrorsRedactTicket(t *testing.T) {
	tr := &eventfeed.WebSocketTransport{}

	t.Run("connection refused is transient and redacted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/cable?ticket=sekrit-ticket-value"
		srv.Close() // nothing listens on the port anymore
		_, err := tr.Dial(context.Background(), wsURL, 1<<20)
		var de *eventfeed.DialError
		if !errors.As(err, &de) {
			t.Fatalf("dial error = %v (%T), want *eventfeed.DialError", err, err)
		}
		if de.Kind != eventfeed.DialTransient {
			t.Errorf("DialError.Kind = %v, want transient", de.Kind)
		}
		assertNoTicket(t, err)
	})

	t.Run("redirect is refused as policy and redacted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/elsewhere?ticket=sekrit-ticket-value", http.StatusFound)
		}))
		defer srv.Close()
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/cable?ticket=sekrit-ticket-value"
		_, err := tr.Dial(context.Background(), wsURL, 1<<20)
		var de *eventfeed.DialError
		if !errors.As(err, &de) {
			t.Fatalf("dial error = %v (%T), want *eventfeed.DialError", err, err)
		}
		if de.Kind != eventfeed.DialPolicy {
			t.Errorf("DialError.Kind = %v, want policy (redirects are refused)", de.Kind)
		}
		assertNoTicket(t, err)
	})

	t.Run("non-101 handshake response is transient and redacted", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/cable?ticket=sekrit-ticket-value"
		_, err := tr.Dial(context.Background(), wsURL, 1<<20)
		var de *eventfeed.DialError
		if !errors.As(err, &de) {
			t.Fatalf("dial error = %v (%T), want *eventfeed.DialError", err, err)
		}
		if de.Kind != eventfeed.DialTransient {
			t.Errorf("DialError.Kind = %v, want transient", de.Kind)
		}
		assertNoTicket(t, err)
	})
}

// TestWebSocketTransport_RejectsUnnegotiatedSubprotocol closes the gap
// coder/websocket leaves open: its verifySubprotocol (dial.go) returns nil
// when the 101 response carries NO Sec-WebSocket-Protocol at all, so a server
// that selects nothing yields a live connection that never agreed to speak
// actioncable-v1-json. The transport must refuse it — as policy, not
// transient: a fresh mint returns a URL pointing at the same server, which
// will keep selecting nothing, so reconnecting cannot help.
func TestWebSocketTransport_RejectsUnnegotiatedSubprotocol(t *testing.T) {
	accepted := make(chan *websocket.Conn, 1)
	// AcceptOptions with no Subprotocols: the handshake succeeds and selects
	// none, which is exactly the case the library lets through.
	srv := newParkedWSServer(t, accepted, nil)

	tr := &eventfeed.WebSocketTransport{}
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/cable?ticket=sekrit-ticket-value"
	conn, err := tr.Dial(context.Background(), wsURL, 1<<20)
	if conn != nil {
		_ = conn.Close(1000, "")
		t.Fatal("dial returned a connection although the server negotiated no subprotocol")
	}
	var de *eventfeed.DialError
	if !errors.As(err, &de) {
		t.Fatalf("dial error = %v (%T), want *eventfeed.DialError", err, err)
	}
	if de.Kind != eventfeed.DialPolicy {
		t.Errorf("DialError.Kind = %v, want policy (a server that never negotiates the subprotocol is not retryable)", de.Kind)
	}
	assertNoTicket(t, err)

	// The refused connection is torn down rather than leaked: the server side
	// sees its socket die without the test releasing the handler.
	peer := awaitPeer(t, accepted)
	if err := awaitPeerRead(t, peer); err == nil {
		t.Error("the refused connection was left open, want it torn down")
	}
}

// TestWebSocketTransport_RejectsWrongCaseSubprotocol pins that negotiation is
// case-SENSITIVE: RFC 6455 subprotocol tokens are exact, so a server
// selecting "ActionCable-V1-Json" selected a protocol this client never
// offered. A case-folded compare would expose the connection anyway. The
// server side is a raw upgrade handler, since a conforming library will not
// select an unoffered spelling.
func TestWebSocketTransport_RejectsWrongCaseSubprotocol(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Sec-WebSocket-Key")
		h := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("test server cannot hijack")
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		fmt.Fprintf(buf, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\nSec-WebSocket-Protocol: ActionCable-V1-Json\r\n\r\n", base64.StdEncoding.EncodeToString(h[:]))
		_ = buf.Flush()
		// Park until the client tears down.
		_, _ = buf.Read(make([]byte, 1))
	}))
	defer srv.Close()

	tr := &eventfeed.WebSocketTransport{}
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/cable?ticket=sekrit-ticket-value"
	conn, err := tr.Dial(context.Background(), wsURL, 1<<20)
	if conn != nil {
		_ = conn.Close(1000, "")
		t.Fatal("dial returned a connection although the server selected an unoffered spelling")
	}
	if err == nil {
		t.Fatal("dial succeeded, want a refusal")
	}
	assertNoTicket(t, err)
}

// TestWebSocketTransport_UppercaseSchemeDials pins the pairing between the
// cable-URL policy — which compares the scheme case-insensitively — and what
// the library actually dials. net/url.Parse lowercases the scheme before
// coder/websocket's handshake switch sees it (dial.go handshakeRequest, over
// url.Parse's `url.Scheme = strings.ToLower(url.Scheme)`), so a "WS://"
// spelling connects; this test is the regression pin that the two halves stay
// in agreement, and that the ticket-bearing remainder still rides through
// byte-identical.
func TestWebSocketTransport_UppercaseSchemeDials(t *testing.T) {
	h := newWSHarness(t)
	target := "/cable?ticket=t-1%2Fabc&b=2&a=1"
	wsURL := "WS" + strings.TrimPrefix(h.srv.URL, "http") + target
	conn, err := h.transport.Dial(context.Background(), wsURL, 1<<20)
	if err != nil {
		t.Fatalf("dial of an uppercase-scheme URL the policy accepts: %v", err)
	}
	defer conn.Close(1000, "")
	if targets := h.DialedTargets(); len(targets) != 1 || targets[0] != target {
		t.Errorf("DialedTargets = %q, want [%q]", targets, target)
	}
}

// closeBoundWatchdog bounds the teardown assertion below. It sits between the
// transport's own close budget and coder/websocket's unbounded-to-us handshake
// (5s to write the close frame, then 5s waiting for the peer's answer), so it
// fails the stall without racing a slow machine.
const closeBoundWatchdog = 3 * time.Second

// TestWebSocketTransport_CloseIsBoundedAgainstAnUnresponsivePeer drives the
// teardown that stalls: a peer that stays alive and never answers the close
// handshake. coder/websocket's Conn.Close waits 5s for that answer, and
// liveConn.dispose closes BEFORE cancelling the attempt (deliberately — the
// peer must see a close frame), so an unbounded close delays cancellation,
// Connector.Close, every terminal outcome and every reconnect behind it.
// Both halves are asserted: Close returns under the watchdog, AND the close
// frame still reached the peer.
func TestWebSocketTransport_CloseIsBoundedAgainstAnUnresponsivePeer(t *testing.T) {
	accepted := make(chan *websocket.Conn, 1)
	// The handler parks without reading, so the close frame sits unanswered in
	// the peer's socket — coder/websocket only answers a close while reading.
	srv := newParkedWSServer(t, accepted, []string{"actioncable-v1-json"})

	tr := &eventfeed.WebSocketTransport{}
	conn, err := tr.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/cable", 1<<20)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	peer := awaitPeer(t, accepted)

	closed := make(chan error, 1)
	go func() { closed <- conn.Close(1000, "teardown") }()
	select {
	case <-closed:
	case <-time.After(closeBoundWatchdog):
		t.Fatalf("Close blocked past %v against a peer that never answers the close handshake", closeBoundWatchdog)
	}

	// Only now — after the bound was measured — does the peer read, so the
	// close frame it sees was written without the transport waiting for it.
	if err := awaitPeerRead(t, peer); websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Errorf("peer read after teardown = %v, want a %d close frame: the close frame must still be written",
			err, websocket.StatusNormalClosure)
	}
}

// TestWebSocketTransport_CloseUnblocksAPendingReadAgainstASilentPeer is the
// other half of the bound above, and the half a cooperative peer cannot see.
// The contract suite's "close unblocks a pending read" runs against a harness
// peer that answers the close handshake, so coder/websocket's Conn.Close
// returns at once and the socket dies with it. Against a peer that stays alive
// and silent, Close returns on the transport's own budget while the library is
// still inside waitCloseHandshake — and the socket, which is what actually
// unblocks a read, is not torn down until that finishes ~5s later. A pending
// ReadFrame(context.Background()) is doubly stranded there: coder/websocket
// installs its read-cancellation hook only when the read ctx HAS a Done
// channel (conn.go setupReadTimeout returns early otherwise), so a background
// read is not cancellable at all.
//
// That is the CableConn.Close contract — "unblocks ReadFrame and WriteFrame" —
// and the reason closeGraceBudget exists: a teardown the caller waits a second
// for, whose read pump then stalls four more, has moved the stall rather than
// bounded it. The elapsed time is measured from Close's RETURN, so the budget
// itself is not what is being asserted here.
//
// Both halves again: the read unblocks, AND the close frame still reached the
// peer — the discriminator against "cancel everything the moment Close is
// called", which would unblock the read by killing the socket out from under
// the close frame §23 requires the peer to see. That second half catches such a
// regression on most runs rather than every one (moving the cancel ahead of the
// handshake races the close-frame write against the socket teardown, and the
// write sometimes wins); it is a backstop on top of the bound above, not a
// deterministic gate, and there is no synchronization point in the seam that
// would make it one.
func TestWebSocketTransport_CloseUnblocksAPendingReadAgainstASilentPeer(t *testing.T) {
	accepted := make(chan *websocket.Conn, 1)
	srv := newParkedWSServer(t, accepted, []string{"actioncable-v1-json"})

	tr := &eventfeed.WebSocketTransport{}
	conn, err := tr.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/cable", 1<<20)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	peer := awaitPeer(t, accepted)

	read := make(chan error, 1)
	go func() {
		_, err := conn.ReadFrame(context.Background())
		read <- err
	}()
	time.Sleep(50 * time.Millisecond) // let the read block inside the library

	if err := conn.Close(1000, "teardown"); err != nil && !strings.Contains(err.Error(), "not acknowledged") {
		t.Fatalf("close: %v", err)
	}
	closeReturned := time.Now()

	select {
	case err := <-read:
		if err == nil {
			t.Fatal("the pending read returned a frame after close, want an error")
		}
		var ce *eventfeed.CloseError
		if errors.As(err, &ce) {
			t.Errorf("pending read error = *CloseError %v; CloseError is reserved for a PEER close", ce)
		}
		if waited := time.Since(closeReturned); waited > closeBoundWatchdog {
			t.Errorf("the pending read unblocked %v after Close returned, want under %v", waited, closeBoundWatchdog)
		}
	case <-time.After(closeBoundWatchdog):
		t.Fatalf("the pending read stayed blocked more than %v after Close returned, against a peer that never answers the close handshake", closeBoundWatchdog)
	}

	// The close frame must still have been written: unblocking the read must
	// not come from tearing the socket down before the peer could see it.
	if err := awaitPeerRead(t, peer); websocket.CloseStatus(err) != websocket.StatusNormalClosure {
		t.Errorf("peer read after teardown = %v, want a %d close frame: the close frame must still be written",
			err, websocket.StatusNormalClosure)
	}
}

// newParkedWSServer starts a WebSocket server whose handler accepts one
// connection, publishes it on accepted, and then parks WITHOUT READING until
// the test ends — the shape both the subprotocol refusal and the close-bound
// test need, where the peer must stay alive but silent. Parking is what makes
// a close frame go unanswered: coder/websocket answers one only while reading.
func newParkedWSServer(t *testing.T, accepted chan<- *websocket.Conn, subprotocols []string) *httptest.Server {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: subprotocols})
		if err != nil {
			return
		}
		accepted <- c
		<-release
		_ = c.CloseNow()
	}))
	// LIFO: release the parked handler first so srv.Close cannot block on it.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	return srv
}

// awaitPeer takes the accepted server-side connection under the watchdog.
func awaitPeer(t *testing.T, accepted <-chan *websocket.Conn) *websocket.Conn {
	t.Helper()
	select {
	case peer := <-accepted:
		return peer
	case <-time.After(contractWatchdog):
		t.Fatal("the server never accepted a connection")
		return nil
	}
}

// awaitPeerRead performs one server-side read under the watchdog, returning
// its error (a close frame surfaces as websocket.CloseError).
func awaitPeerRead(t *testing.T, peer *websocket.Conn) error {
	t.Helper()
	result := make(chan error, 1)
	go func() {
		_, _, err := peer.Read(context.Background())
		result <- err
	}()
	select {
	case err := <-result:
		return err
	case <-time.After(contractWatchdog):
		t.Fatal("the peer's read never returned")
		return nil
	}
}

// assertNoTicket asserts err's rendering never leaks the ticket-bearing
// query: neither the ticket value nor the query parameter may appear.
func assertNoTicket(t *testing.T, err error) {
	t.Helper()
	msg := err.Error()
	if strings.Contains(msg, "sekrit-ticket-value") {
		t.Errorf("error leaks the ticket value: %q", msg)
	}
	if strings.Contains(msg, "ticket=") {
		t.Errorf("error leaks the ticket query parameter: %q", msg)
	}
}

// rawHandshakeServer answers the WebSocket upgrade itself, so the test can put
// arbitrary bytes in the 101 response — which is what a hostile or merely
// sloppy peer controls, and what coder/websocket then quotes back in its
// verification errors. header is echoed as Sec-WebSocket-Protocol.
func rawHandshakeServer(t *testing.T, header string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("the test server does not support hijacking")
			return
		}
		conn, buf, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
		accept := base64.StdEncoding.EncodeToString(sum[:])
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n" +
			"Upgrade: websocket\r\nConnection: Upgrade\r\n" +
			"Sec-WebSocket-Accept: " + accept + "\r\n" +
			"Sec-WebSocket-Protocol: " + header + "\r\n\r\n")
		_ = buf.Flush()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestWebSocketTransport_DialErrorNeverCarriesTheMintQuery is the invariant a
// redactor could not hold. SPEC §23 calls the ticket an "opaque bearer
// credential; never logged" — opaque, so the transport may assume NOTHING
// about how it is spelled inside the mint URL's query string.
//
// Each case puts the credential somewhere a value-shaped redactor misses, and
// has the peer reflect it into a response header that coder/websocket quotes
// verbatim in its verification error (dial.go's verifySubprotocol renders the
// server's Sec-WebSocket-Protocol with %q). It asserts through the PUBLIC
// Dial, so it is a statement about what reaches an observer's logs rather than
// about any one helper.
func TestWebSocketTransport_DialErrorNeverCarriesTheMintQuery(t *testing.T) {
	tr := &eventfeed.WebSocketTransport{}
	for _, tc := range []struct {
		name   string
		query  string // the mint URL's query, verbatim
		secret string // what the peer reflects back
	}{
		{"bare opaque query, no key=value at all", "SECRET-TICKET-9f3c", "SECRET-TICKET-9f3c"},
		{"credential in the key", "TCKT-8b21ae=", "TCKT-8b21ae"},
		{"short opaque value", "ticket=q7", "q7"},
		{"percent-encoded value", "ticket=a%2Fb%2Bc", "a/b+c"},
		{"percent-encoded value, raw spelling", "ticket=a%2Fb%2Bc", "a%2Fb%2Bc"},
		{"long ordinary value", "ticket=tkt-9f3c1ab27e5d40b6", "tkt-9f3c1ab27e5d40b6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := rawHandshakeServer(t, tc.secret)
			wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/cable?" + tc.query
			_, err := tr.Dial(context.Background(), wsURL, 1<<20)
			if err == nil {
				t.Fatal("dial succeeded, want the unnegotiated-subprotocol refusal")
			}
			if got := err.Error(); strings.Contains(got, tc.secret) {
				t.Fatalf("dial error leaked the credential %q: %s", tc.secret, got)
			}
			// The whole query string is off limits too, not just the secret.
			if got := err.Error(); strings.Contains(got, tc.query) {
				t.Fatalf("dial error leaked the mint query %q: %s", tc.query, got)
			}
		})
	}
}

// TestWebSocketTransport_DialErrorKeepsTheStatusCode pins the diagnostic that
// survives the closed vocabulary. An HTTP status is an integer, structurally
// incapable of carrying a credential, and it is the single most useful thing
// an operator needs off a failed handshake — so narrowing the message must not
// cost it.
func TestWebSocketTransport_DialErrorKeepsTheStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	tr := &eventfeed.WebSocketTransport{}
	_, err := tr.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/cable?ticket=sekrit-ticket-value", 1<<20)
	if err == nil {
		t.Fatal("dial succeeded against a 401, want a transient failure")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("dial error = %q, want the HTTP status preserved", err)
	}
	assertNoTicket(t, err)
}

// ---------------------------------------------------------------------------
// Credential boundary (SPEC.md §23 "Security Invariants").
//
// The cable origin is CROSS-HOST BY DESIGN: the mint returns whatever host the
// server directs the subscription to, and the connector dials it verbatim. The
// ticket in its query string is the only credential that origin is entitled
// to. Anything else the handshake carries — the caller's bearer, a cookie, a
// proxy credential — is a credential sent to a host chosen by the response to
// an earlier request, which is the whole hazard.
//
// These assert the boundary from the origin's side and the proxy's side,
// because that is where a violation is observable. A structural assertion
// ("the transport has no client field") would go stale the moment a credential
// arrived by some other route.
// ---------------------------------------------------------------------------

// injectingRoundTripper is the credential-bearing client an unsuspecting host
// would plausibly install process-wide — an auth-injecting RoundTripper of
// exactly the shape an SDK, a tracing wrapper or a corporate mTLS bootstrap
// installs on http.DefaultClient.
type injectingRoundTripper struct{ base http.RoundTripper }

func (rt injectingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer caller-bearer-token")
	req.Header.Set("Proxy-Authorization", "Basic cHJveHk6c2VjcmV0")
	return rt.base.RoundTrip(req)
}

// TestWebSocketTransport_CableOriginReceivesNoCallerCredential is daybreak
// blocker 1. The cable dial must not inherit process-global HTTP
// configuration: not http.DefaultClient's Transport, not its cookie jar.
//
// This composes two files, which is why eight review rounds walked past it —
// transport.go's URL policy is impeccable in isolation and
// websocket_transport.go's fallback to http.DefaultClient is unremarkable in
// isolation. Together they hand every credential the ambient client carries to
// a host the server nominated.
func TestWebSocketTransport_CableOriginReceivesNoCallerCredential(t *testing.T) {
	var mu sync.Mutex
	var seen []http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{"actioncable-v1-json"},
		})
		if err != nil {
			return
		}
		_ = c.CloseNow()
	}))
	defer srv.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "caller-session-cookie"}})

	// Pollute the process-global client, as a host embedding this SDK
	// alongside its own HTTP stack routinely does. Restored on the way out.
	prev := http.DefaultClient
	http.DefaultClient = &http.Client{
		Transport: injectingRoundTripper{base: http.DefaultTransport},
		Jar:       jar,
	}
	t.Cleanup(func() { http.DefaultClient = prev })

	tr := &eventfeed.WebSocketTransport{}
	conn, derr := tr.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http")+"/cable?ticket=sekrit-ticket-value", 1<<20)
	if derr == nil {
		_ = conn.Close(1000, "")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("the cable origin saw no handshake at all; the test proves nothing")
	}
	for i, h := range seen {
		for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie"} {
			if got := h.Get(header); got != "" {
				t.Errorf("handshake %d sent %s: %q — the cable origin must receive no caller credential", i, header, got)
			}
		}
	}
}

// TestWebSocketTransport_RejectsUserinfoBeforeAnyNetworkIO is the second half
// of the same boundary, and it is not hypothetical: net/http's send() turns
// URL userinfo into a Basic Authorization header
// (`if u := req.URL.User; u != nil && ...`), so a mint URL carrying userinfo
// makes the connector authenticate to the cable origin with credentials the
// SERVER chose. The policy must refuse it before any network I/O — nothing
// about a URL like this becomes acceptable after a TCP connect.
func TestWebSocketTransport_RejectsUserinfoBeforeAnyNetworkIO(t *testing.T) {
	var conns atomic.Int64
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	defer srv.Close()

	hostport := strings.TrimPrefix(srv.URL, "http://")
	for _, tc := range []struct {
		name  string
		wsURL string
	}{
		{"user and password", "ws://attacker:hunter2@" + hostport + "/cable?ticket=sekrit-ticket-value"},
		{"bare user", "ws://attacker@" + hostport + "/cable?ticket=sekrit-ticket-value"},
		{"empty password", "ws://attacker:@" + hostport + "/cable?ticket=sekrit-ticket-value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conns.Store(0)
			tr := &eventfeed.WebSocketTransport{}
			conn, err := tr.Dial(context.Background(), tc.wsURL, 1<<20)
			if err == nil {
				_ = conn.Close(1000, "")
				t.Fatal("dial accepted a cable URL carrying userinfo")
			}
			var derr *eventfeed.DialError
			if !errors.As(err, &derr) || derr.Kind != eventfeed.DialPolicy {
				t.Errorf("err = %v (%T), want a policy-kind *DialError", err, err)
			}
			if n := conns.Load(); n != 0 {
				t.Errorf("the server accepted %d connection(s); the refusal must precede all network I/O", n)
			}
			// The rejection is reported without echoing the credential or the
			// ticket that rode alongside it.
			for _, secret := range []string{"attacker", "hunter2", "sekrit-ticket-value"} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("rejection leaked %q: %s", secret, err)
				}
			}
		})
	}
}

// TestWebSocketTransport_WriteAfterPeerCloseCarriesNoPeerReason is the write
// path's half of the peer-close rule, pinned as a TRIPWIRE. The claimed leak
// does not exist in coder/websocket v1.8.15 — the write path returns
// net.ErrClosed sentinels for every closed-connection shape and consults
// closeReceivedErr only on reads, so the peer's reason structurally cannot
// surface from Write — but that is the library's internals, not its
// contract. This test drives the exact interleaving (a read observes the
// peer's close, then a write fails) and walks the write error's chain for
// the planted reason, so a dependency bump that starts surfacing the
// recorded close from Write goes red here and forces the read path's
// sanitizing treatment onto the write path then.
func TestWebSocketTransport_WriteAfterPeerCloseCarriesNoPeerReason(t *testing.T) {
	const canary = "sekrit-ticket-value"
	h := newWSHarness(t)
	conn, peer, err := h.Dial(context.Background(), "/cable?ticket="+canary, 1<<20)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(1000, "")
	read := make(chan error, 1)
	go func() {
		_, rerr := conn.ReadFrame(context.Background())
		read <- rerr
	}()
	peer.Close(1008, "refused ?ticket="+canary)
	select {
	case rerr := <-read:
		var ce *eventfeed.CloseError
		if !errors.As(rerr, &ce) {
			t.Fatalf("read after peer close = %v (%T), want *CloseError", rerr, rerr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the peer close never reached the read")
	}
	werr := conn.WriteFrame(context.Background(), []byte(`{"command":"subscribe"}`))
	if werr == nil {
		t.Fatal("write after peer close succeeded, want error")
	}
	for e := werr; e != nil; e = errors.Unwrap(e) {
		if strings.Contains(e.Error(), canary) {
			t.Errorf("write error echoes the peer's close reason: %q", e.Error())
		}
	}
}
