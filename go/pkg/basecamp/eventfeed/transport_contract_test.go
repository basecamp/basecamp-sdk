// The CableTransport contract suite (SPEC.md §23 "Seam Contracts",
// design §12 tier 3): one set of behavioral assertions every transport
// implementation must satisfy, run here against the feedtest fake and reused
// verbatim by the real WebSocket transport's harness when it lands — the
// point being that the fake the tier-2 scenarios trust and the transport
// production uses satisfy the same contract. It lives in the external test
// package so nothing leaks into the exported surface; the suite drives the
// transport only through the exported seam types.
package eventfeed_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed/feedtest"
)

// contractWatchdog bounds every blocking assertion in the suite. Generous on
// purpose: it exists to fail hung contracts, not to race slow CI.
const contractWatchdog = 5 * time.Second

// contractPeerConn is the peer side of one dialed connection: the scripting
// surface a harness exposes so the suite can drive frames from the far end.
type contractPeerConn interface {
	// Serve queues one text frame for delivery to the client conn.
	Serve(frame []byte)
	// Close closes the connection from the peer side with a WebSocket
	// status, after any queued frames drain.
	Close(code int, reason string)
	// Received reports the text frames the peer has received from the
	// client, verbatim, in order.
	Received() [][]byte
}

// contractHarness wires the suite to one CableTransport implementation and
// its scriptable peer.
type contractHarness interface {
	// Dial dials target — a path-and-query suffix such as
	// "/cable?ticket=t-1" — through the transport under test, carrying
	// maxFrameBytes, and returns the client conn plus the peer controls.
	Dial(ctx context.Context, target string, maxFrameBytes int64) (eventfeed.CableConn, contractPeerConn, error)
	// DialedTargets reports the path-and-query of every dial the peer
	// observed, verbatim.
	DialedTargets() []string
}

// runTransportContract runs the shared CableTransport contract against one
// implementation. newHarness is called per subtest for isolation.
func runTransportContract(t *testing.T, newHarness func(t *testing.T) contractHarness) {
	t.Run("dial passes the URL through verbatim", func(t *testing.T) {
		h := newHarness(t)
		// The ticket rides in the query string; encoded characters and
		// parameter order must survive untouched.
		target := "/cable?ticket=t-1%2Fabc&b=2&a=1"
		conn, _, err := h.Dial(context.Background(), target, 1<<20)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close(1000, "")
		targets := h.DialedTargets()
		if len(targets) != 1 || targets[0] != target {
			t.Errorf("DialedTargets = %q, want [%q]", targets, target)
		}
	})

	t.Run("frames arrive verbatim", func(t *testing.T) {
		h := newHarness(t)
		conn, peer, err := h.Dial(context.Background(), "/cable", 1<<20)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close(1000, "")
		// A disconnect frame with hostile spacing and non-ASCII content:
		// the terminal/non-terminal distinction lives only in this raw
		// frame, so the transport must not normalize, reformat, or swallow
		// it.
		raw := "{ \"type\" : \"disconnect\",\n\t\"reason\": \"invalid_event_stream_command\", \"note\": \"göodbye\", \"reconnect\": false }"
		peer.Serve([]byte(raw))
		got := mustReadFrame(t, conn)
		if string(got) != raw {
			t.Errorf("frame = %q, want %q", got, raw)
		}
	})

	t.Run("writes arrive at the peer verbatim", func(t *testing.T) {
		h := newHarness(t)
		conn, peer, err := h.Dial(context.Background(), "/cable", 1<<20)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close(1000, "")
		cmd := `{"command":"subscribe","identifier":"{\"channel\":\"EventsChannel\"}"}`
		if err := conn.WriteFrame(context.Background(), []byte(cmd)); err != nil {
			t.Fatalf("write: %v", err)
		}
		deadline := time.Now().Add(contractWatchdog)
		for {
			if got := peer.Received(); len(got) == 1 {
				if string(got[0]) != cmd {
					t.Errorf("peer received %q, want %q", got[0], cmd)
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("peer never received the written frame")
			}
			time.Sleep(time.Millisecond)
		}
	})

	t.Run("max frame bytes is enforced while reading", func(t *testing.T) {
		h := newHarness(t)
		const frameCap = 64
		conn, peer, err := h.Dial(context.Background(), "/cable", frameCap)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close(1000, "")
		// An at-limit frame passes.
		const prefix, suffix = `{"type":"ping","pad":"`, `"}`
		small := []byte(prefix + strings.Repeat("x", frameCap-len(prefix)-len(suffix)) + suffix)
		if int64(len(small)) != frameCap {
			t.Fatalf("test bug: small frame is %d bytes, want %d", len(small), frameCap)
		}
		peer.Serve(small)
		if got := mustReadFrame(t, conn); string(got) != string(small) {
			t.Errorf("under-limit frame = %q, want %q", got, small)
		}
		// An over-limit frame is rejected as an error, never delivered. The
		// rejection is not a peer close and not a cancellation.
		peer.Serve([]byte(`{"type":"ping","pad":"` + strings.Repeat("x", frameCap) + `"}`))
		_, err = readFrameWithin(context.Background(), t, conn)
		if err == nil {
			t.Fatal("over-limit frame was delivered, want read error")
		}
		var ce *eventfeed.CloseError
		if errors.As(err, &ce) {
			t.Errorf("over-limit rejection = *CloseError %v, want a plain read error", ce)
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("over-limit rejection = %v, want a non-cancellation error", err)
		}
	})

	t.Run("peer close surfaces as CloseError", func(t *testing.T) {
		h := newHarness(t)
		conn, peer, err := h.Dial(context.Background(), "/cable", 1<<20)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close(1000, "")
		peer.Close(4401, "unauthorized")
		_, err = readFrameWithin(context.Background(), t, conn)
		var ce *eventfeed.CloseError
		if !errors.As(err, &ce) {
			t.Fatalf("read after peer close = %v, want *eventfeed.CloseError", err)
		}
		if ce.Code != 4401 || ce.Reason != "unauthorized" {
			t.Errorf("CloseError = %d %q, want 4401 %q", ce.Code, ce.Reason, "unauthorized")
		}
	})

	t.Run("close is idempotent and safe from any goroutine", func(t *testing.T) {
		h := newHarness(t)
		conn, _, err := h.Dial(context.Background(), "/cable", 1<<20)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		if err := conn.Close(1000, "done"); err != nil {
			t.Fatalf("first close: %v", err)
		}
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Repeat closes must return (any error is tolerable, a hang
				// or panic is not).
				_ = conn.Close(1000, "again")
			}()
		}
		waitDone(t, &wg, "concurrent repeat closes")
	})

	t.Run("close unblocks a pending read", func(t *testing.T) {
		h := newHarness(t)
		conn, _, err := h.Dial(context.Background(), "/cable", 1<<20)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		read := make(chan error, 1)
		go func() {
			_, err := conn.ReadFrame(context.Background())
			read <- err
		}()
		time.Sleep(10 * time.Millisecond) // let the read block
		if err := conn.Close(1000, "teardown"); err != nil {
			t.Fatalf("close: %v", err)
		}
		select {
		case err := <-read:
			if err == nil {
				t.Fatal("read returned a frame after local close, want error")
			}
			var ce *eventfeed.CloseError
			if errors.As(err, &ce) {
				t.Errorf("local close read error = *CloseError %v; CloseError is reserved for a peer close", ce)
			}
		case <-time.After(contractWatchdog):
			t.Fatal("close did not unblock the pending read")
		}
		// Reads and writes after close fail promptly.
		if _, err := readFrameWithin(context.Background(), t, conn); err == nil {
			t.Error("read after close succeeded, want error")
		}
		if err := conn.WriteFrame(context.Background(), []byte("x")); err == nil {
			t.Error("write after close succeeded, want error")
		}
	})

	t.Run("context cancellation unblocks a pending read", func(t *testing.T) {
		h := newHarness(t)
		conn, _, err := h.Dial(context.Background(), "/cable", 1<<20)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close(1000, "")
		ctx, cancel := context.WithCancel(context.Background())
		read := make(chan error, 1)
		go func() {
			_, err := conn.ReadFrame(ctx)
			read <- err
		}()
		time.Sleep(10 * time.Millisecond) // let the read block
		cancel()
		select {
		case err := <-read:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("cancelled read error = %v, want context.Canceled", err)
			}
		case <-time.After(contractWatchdog):
			t.Fatal("cancellation did not unblock the pending read")
		}
	})

	t.Run("a done context fails dial promptly", func(t *testing.T) {
		h := newHarness(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan error, 1)
		go func() {
			_, _, err := h.Dial(ctx, "/cable", 1<<20)
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("dial with a done context succeeded, want error")
			}
			if !errors.Is(err, context.Canceled) {
				t.Errorf("dial error = %v, want context.Canceled", err)
			}
		case <-time.After(contractWatchdog):
			t.Fatal("dial with a done context did not return promptly")
		}
	})
}

// mustReadFrame reads one frame under the watchdog.
func mustReadFrame(t *testing.T, conn eventfeed.CableConn) []byte {
	t.Helper()
	frame, err := readFrameWithin(context.Background(), t, conn)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	return frame
}

// readFrameWithin performs one ReadFrame bounded by the contract watchdog so
// a violating implementation hangs the subtest, not the suite.
func readFrameWithin(ctx context.Context, t *testing.T, conn eventfeed.CableConn) ([]byte, error) {
	t.Helper()
	type result struct {
		frame []byte
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		frame, err := conn.ReadFrame(ctx)
		ch <- result{frame, err}
	}()
	select {
	case r := <-ch:
		return r.frame, r.err
	case <-time.After(contractWatchdog):
		t.Fatal("ReadFrame did not return under the contract watchdog")
		return nil, nil
	}
}

// waitDone waits for wg under the watchdog.
func waitDone(t *testing.T, wg *sync.WaitGroup, what string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(contractWatchdog):
		t.Fatalf("%s did not complete", what)
	}
}

// fakeHarnessOrigin is the fake peer's origin; targets are appended to it,
// so trimming it back off recovers the dialed path-and-query verbatim.
const fakeHarnessOrigin = "wss://cable.example.test"

// fakeHarness adapts feedtest.Transport to the contract harness.
type fakeHarness struct {
	tr *feedtest.Transport
}

func newFakeHarness(*testing.T) contractHarness {
	return &fakeHarness{tr: feedtest.NewTransport()}
}

func (h *fakeHarness) Dial(ctx context.Context, target string, maxFrameBytes int64) (eventfeed.CableConn, contractPeerConn, error) {
	conn, err := h.tr.Dial(ctx, fakeHarnessOrigin+target, maxFrameBytes)
	if err != nil {
		return nil, nil, err
	}
	return conn, &fakePeerConn{c: h.tr.LastConn()}, nil
}

func (h *fakeHarness) DialedTargets() []string {
	urls := h.tr.DialedURLs()
	targets := make([]string, len(urls))
	for i, u := range urls {
		targets[i] = strings.TrimPrefix(u, fakeHarnessOrigin)
	}
	return targets
}

// fakePeerConn is the far end of one feedtest.Conn.
type fakePeerConn struct {
	c *feedtest.Conn
}

func (p *fakePeerConn) Serve(frame []byte) {
	p.c.Serve(frame)
}

func (p *fakePeerConn) Close(code int, reason string) {
	p.c.ServeClose(code, reason)
}

func (p *fakePeerConn) Received() [][]byte {
	return p.c.Writes()
}

// TestFeedtestTransport_SatisfiesCableTransportContract runs the shared
// contract against the scripted fake — the same suite the real WebSocket
// transport runs when it lands.
func TestFeedtestTransport_SatisfiesCableTransportContract(t *testing.T) {
	runTransportContract(t, newFakeHarness)
}
