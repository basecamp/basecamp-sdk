package feedtest

import (
	"context"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp/eventfeed"
)

func TestPolls_RecordsCallsAndPopsScriptInOrder(t *testing.T) {
	p := NewPolls()
	p.ScriptPage(eventfeed.PollPage{Position: "pos-1"})
	scripted := &eventfeed.PollError{Kind: eventfeed.PollTransient}
	p.ScriptError(scripted)

	cursor := eventfeed.Cursor{Since: "now"}
	filters := eventfeed.Filters{Types: []string{"message.created"}}
	page, err := p.Poll(context.Background(), cursor, filters)
	if err != nil || page.Position != "pos-1" {
		t.Fatalf("first poll = %+v, %v; want the scripted page", page, err)
	}
	if _, err := p.Poll(context.Background(), eventfeed.Cursor{}, eventfeed.Filters{}); err != scripted {
		t.Fatalf("second poll error = %v, want the scripted error", err)
	}

	calls := p.Calls()
	if len(calls) != 2 || calls[0].Cursor != cursor || len(calls[0].Filters.Types) != 1 {
		t.Fatalf("Calls() = %+v, want the two calls with cursor and filters recorded", calls)
	}
	if got := p.CallCount(); got != 2 {
		t.Fatalf("CallCount() = %d, want 2", got)
	}
}

func TestPolls_UnscriptedCallFails(t *testing.T) {
	p := NewPolls()
	if _, err := p.Poll(context.Background(), eventfeed.Cursor{}, eventfeed.Filters{}); err == nil {
		t.Fatal("unscripted poll succeeded, want a visible failure")
	}
	if got := p.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1 — every seam call counts", got)
	}
}

func TestPolls_StallNextBlocksUntilCancelled(t *testing.T) {
	p := NewPolls()
	p.StallNext()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := p.Poll(ctx, eventfeed.Cursor{}, eventfeed.Filters{})
		done <- err
	}()
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("stalled poll returned %v, want context.Canceled", err)
	}
	if got := p.CallCount(); got != 1 {
		t.Fatalf("CallCount() = %d, want 1 — a stalled call still counts", got)
	}
}
