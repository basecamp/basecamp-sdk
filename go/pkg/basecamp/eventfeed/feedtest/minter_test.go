package feedtest

import (
	"context"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

func TestMinter_ScriptedOutcomesPopInOrder(t *testing.T) {
	m := NewMinter()
	m.ScriptTicket(eventfeed.StreamTicket{Ticket: "t1", ExpiresIn: 120, URL: "wss://one"})
	scripted := &eventfeed.MintError{Kind: eventfeed.MintTransient}
	m.ScriptError(scripted)

	st, err := m.MintStreamTicket(context.Background())
	if err != nil || st.URL != "wss://one" {
		t.Fatalf("first mint = %+v, %v; want the scripted ticket", st, err)
	}
	if _, err := m.MintStreamTicket(context.Background()); err != scripted {
		t.Fatalf("second mint error = %v, want the scripted error", err)
	}
	if got := m.Calls(); got != 2 {
		t.Fatalf("Calls() = %d, want 2", got)
	}
}

func TestMinter_UnscriptedCallFails(t *testing.T) {
	m := NewMinter()
	if _, err := m.MintStreamTicket(context.Background()); err == nil {
		t.Fatal("unscripted mint succeeded, want a visible failure")
	}
	if got := m.Calls(); got != 1 {
		t.Fatalf("Calls() = %d, want 1 — every seam call counts", got)
	}
}

func TestMinter_StallNextBlocksUntilCancelled(t *testing.T) {
	m := NewMinter()
	m.StallNext()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := m.MintStreamTicket(ctx)
		done <- err
	}()
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("stalled mint returned %v, want context.Canceled", err)
	}
	if got := m.Calls(); got != 1 {
		t.Fatalf("Calls() = %d, want 1 — a stalled call still counts", got)
	}
}

func TestMinter_DoneContextReturnsPromptly(t *testing.T) {
	m := NewMinter()
	m.ScriptTicket(eventfeed.StreamTicket{Ticket: "t1", ExpiresIn: 120, URL: "wss://one"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.MintStreamTicket(ctx); err != context.Canceled {
		t.Fatalf("cancelled mint returned %v, want context.Canceled", err)
	}
}
