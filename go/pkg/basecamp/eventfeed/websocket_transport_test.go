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
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

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
}

func newWSPeer(c *websocket.Conn) *wsPeer {
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
// dial's maxFrameBytes and asserts the in-read cap: the read errors without
// delivering the frame, the error is neither a peer-close *CloseError nor a
// cancellation, it never carries the ticket-bearing query, and the
// connection is unusable afterward.
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
