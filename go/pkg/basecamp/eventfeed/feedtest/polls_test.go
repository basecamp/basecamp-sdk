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

func TestPolls_LedgerOwnsItsFilterBytes(t *testing.T) {
	p := NewPolls()
	p.ScriptPage(eventfeed.PollPage{})
	types := []string{"message.created"}
	if _, err := p.Poll(context.Background(), eventfeed.Cursor{}, eventfeed.Filters{Types: types}); err != nil {
		t.Fatalf("poll = %v, want the scripted page", err)
	}
	// Neither the caller mutating its slice after the call...
	types[0] = "mutated.after.call"
	if got := p.Calls()[0].Filters.Types[0]; got != "message.created" {
		t.Fatalf("ledger says %q, want what was passed at call time", got)
	}
	// ...nor a test editing a returned snapshot rewrites history...
	p.Calls()[0].Filters.Types[0] = "edited.snapshot"
	if got := p.Calls()[0].Filters.Types[0]; got != "message.created" {
		t.Fatalf("snapshot edit corrupted the ledger: %q", got)
	}
	// ...nor an OnCall callback mutating its argument.
	p.ScriptPage(eventfeed.PollPage{})
	p.OnCall(func(call PollCall) { call.Filters.Types[0] = "mutated.in.callback" })
	if _, err := p.Poll(context.Background(), eventfeed.Cursor{}, eventfeed.Filters{Types: []string{"todo.created"}}); err != nil {
		t.Fatalf("poll = %v, want the scripted page", err)
	}
	if got := p.Calls()[1].Filters.Types[0]; got != "todo.created" {
		t.Fatalf("callback mutation corrupted the ledger: %q", got)
	}
}
