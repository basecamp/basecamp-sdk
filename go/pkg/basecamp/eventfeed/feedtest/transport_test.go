package feedtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

func TestTransport_RecordsDialsVerbatimAndHandsOutFreshConns(t *testing.T) {
	tr := NewTransport()
	ctx := context.Background()
	c1, err := tr.Dial(ctx, "wss://28.cable.basecamp.com/cable?ticket=t-1", 1<<20)
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}
	c2, err := tr.Dial(ctx, "wss://28.cable.basecamp.com/cable?ticket=t-2", 512)
	if err != nil {
		t.Fatalf("dial 2: %v", err)
	}
	if c1 == c2 {
		t.Error("dials shared a conn, want one fresh conn per dial")
	}
	urls := tr.DialedURLs()
	want := []string{
		"wss://28.cable.basecamp.com/cable?ticket=t-1",
		"wss://28.cable.basecamp.com/cable?ticket=t-2",
	}
	if len(urls) != 2 || urls[0] != want[0] || urls[1] != want[1] {
		t.Errorf("DialedURLs = %q, want %q", urls, want)
	}
	dials := tr.Dials()
	if dials[0].MaxFrameBytes != 1<<20 || dials[1].MaxFrameBytes != 512 {
		t.Errorf("recorded MaxFrameBytes = %d, %d; want %d, %d", dials[0].MaxFrameBytes, dials[1].MaxFrameBytes, 1<<20, 512)
	}
	if got := len(tr.Conns()); got != 2 {
		t.Errorf("len(Conns) = %d, want 2", got)
	}
	if tr.LastConn() != c2 {
		t.Error("LastConn is not the second conn")
	}
}

func TestTransport_FailNextDialStillRecords(t *testing.T) {
	tr := NewTransport()
	scripted := errors.New("connection refused")
	tr.FailNextDial(scripted)
	if _, err := tr.Dial(context.Background(), "wss://a/cable", 1<<20); !errors.Is(err, scripted) {
		t.Fatalf("dial error = %v, want %v", err, scripted)
	}
	if got := len(tr.DialedURLs()); got != 1 {
		t.Errorf("failed dial not recorded: %d dials", got)
	}
	if tr.LastConn() != nil {
		t.Error("failed dial produced a conn")
	}
	// The failure script is consumed: the next dial succeeds.
	if _, err := tr.Dial(context.Background(), "wss://a/cable", 1<<20); err != nil {
		t.Fatalf("dial after consumed failure: %v", err)
	}
}

func TestTransport_StallNextDialReturnsOnCancel(t *testing.T) {
	tr := NewTransport()
	tr.StallNextDial()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := tr.Dial(ctx, "wss://a/cable", 1<<20)
		done <- err
	}()
	// The dial is recorded even while stalled.
	deadline := time.After(5 * time.Second)
	for len(tr.DialedURLs()) == 0 {
		select {
		case <-deadline:
			t.Fatal("stalled dial never recorded")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("stalled dial error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stalled dial did not return promptly on cancel")
	}
}

func TestConn_ServesFramesThenScriptedTail(t *testing.T) {
	tr := NewTransport()
	conn, _ := tr.Dial(context.Background(), "wss://a/cable", 1<<20)
	c := tr.LastConn()
	c.Serve([]byte(`{"type":"welcome"}`))
	c.Serve([]byte(`{"type":"ping"}`))
	c.ServeClose(1006, "abnormal closure")

	f1, err := conn.ReadFrame(context.Background())
	if err != nil || string(f1) != `{"type":"welcome"}` {
		t.Fatalf("frame 1 = %s, %v", f1, err)
	}
	f2, err := conn.ReadFrame(context.Background())
	if err != nil || string(f2) != `{"type":"ping"}` {
		t.Fatalf("frame 2 = %s, %v", f2, err)
	}
	// Queued frames drain before the scripted close, which surfaces as a
	// peer-close *eventfeed.CloseError.
	_, err = conn.ReadFrame(context.Background())
	var ce *eventfeed.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("read past tail = %v, want *eventfeed.CloseError", err)
	}
	if ce.Code != 1006 || ce.Reason != "abnormal closure" {
		t.Errorf("CloseError = %d %q, want 1006 %q", ce.Code, ce.Reason, "abnormal closure")
	}
}

func TestConn_OversizeFrameMatchesSentinelAndLatches(t *testing.T) {
	tr := NewTransport()
	conn, _ := tr.Dial(context.Background(), "wss://a/cable", 8)
	tr.LastConn().Serve([]byte(`{"type":"welcome"}`)) // 18 bytes > the 8-byte cap

	_, err := conn.ReadFrame(context.Background())
	// The seam contract, not just an error: the run loop classifies an
	// over-limit read by errors.Is against the sentinel, and a fake that
	// returned an untyped error would keep every loop test green against a
	// classification the real transport defeats.
	if !errors.Is(err, eventfeed.ErrFrameOversize) {
		t.Fatalf("oversize read = %v, want errors.Is ErrFrameOversize", err)
	}
	// The violation latches, as with a real dead connection.
	if _, err2 := conn.ReadFrame(context.Background()); !errors.Is(err2, eventfeed.ErrFrameOversize) {
		t.Fatalf("second read = %v, want the latched sentinel", err2)
	}
}

func TestConn_WritesRecordedVerbatimAndCopied(t *testing.T) {
	tr := NewTransport()
	conn, _ := tr.Dial(context.Background(), "wss://a/cable", 1<<20)
	buf := []byte(`{"command":"subscribe","identifier":"x"}`)
	if err := conn.WriteFrame(context.Background(), buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf[0] = '!' // mutate after write; the record must be a copy
	writes := tr.LastConn().Writes()
	if len(writes) != 1 || string(writes[0]) != `{"command":"subscribe","identifier":"x"}` {
		t.Errorf("Writes = %q", writes)
	}
}

func TestConn_FailWrites(t *testing.T) {
	tr := NewTransport()
	conn, _ := tr.Dial(context.Background(), "wss://a/cable", 1<<20)
	scripted := errors.New("broken pipe")
	tr.LastConn().FailWrites(scripted)
	if err := conn.WriteFrame(context.Background(), []byte("x")); !errors.Is(err, scripted) {
		t.Errorf("write error = %v, want %v", err, scripted)
	}
}

func TestConn_CloseRecordsFirstCallOnly(t *testing.T) {
	tr := NewTransport()
	conn, _ := tr.Dial(context.Background(), "wss://a/cable", 1<<20)
	if err := conn.Close(1000, "done"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := conn.Close(1001, "again"); err != nil {
		t.Fatalf("repeat close: %v", err)
	}
	c := tr.LastConn()
	if !c.Closed() || c.CloseCalls() != 2 || c.CloseCode() != 1000 || c.CloseReason() != "done" {
		t.Errorf("close record = closed %v, calls %d, code %d, reason %q",
			c.Closed(), c.CloseCalls(), c.CloseCode(), c.CloseReason())
	}
}
