package eventfeed

import (
	"fmt"
	"strconv"
)

// The recovery matrix: the repair poll a fired `repair-poll` timer drives
// (SPEC.md §23 transition 24) and the re-entries a poll error whose
// disposition is a NEW cursor takes (transitions 17/18/19 — 410 `gone`,
// 400-position, 409 `filter_changed`). Both run on the consumer's goroutine,
// inside the walk that reached them, so nothing here is concurrent with a
// delivery and the handler contract's "synchronously, on the consumer's
// execution context" is structural.
//
// The three re-entries share one shape and differ only in where the new
// cursor comes from: a 410's is the resume URL the SERVER provided, gated
// behind the FeedGap semantic signal; a 400-position's and a 409's is the
// poll-lane-only reset cursor below. All three restart the entry boundary —
// the walk drops any held position and re-takes the ownership cut on the new
// entry — because a re-entry IS an entry; only the positions already saved
// survive it.
//
// A re-entry re-polls IMMEDIATELY: §23 models rows 17/18/19 as CatchingUp
// self-loops with no wait of their own (only transients and throttles take
// the `poll-retry` timer), and the tier-2 gap fixtures script the re-entry
// poll with no intervening timer to fire. The connector therefore relies on
// the server not answering a reset cursor with the same rejection that
// produced it — a `since=` cursor carries no position token for a 400-position
// or 409 to be about, and a resume URL is the server's own. No local bound is
// imposed here: one would have to invent a threshold and a terminal reason
// §23 does not publish, and would diverge from the other five SDKs.

// repairWalk is transition 24: one walk from the connector's current
// position, driven by a fired `repair-poll` timer. It is the same walk, drain
// and entry discipline the post-confirmation catch-up runs, so it returns to
// Streaming through Draining — which is what makes a live frame arriving
// mid-walk buffered rather than delivered (CatchingUp delivers poll pages
// only) and replayed on the way back.
//
// It reports done=true with the cycle's outcome when the walk ended the
// cycle; otherwise the caller re-arms the next jittered cadence and
// re-announces Streaming.
func (l *loop) repairWalk(at *attempt) (cycleOutcome, bool) {
	cursor, presentClass := l.repairCursor()
	l.setState(stateCatchingUp)
	if l.cfg.observer.CatchUpStarted != nil {
		l.cfg.observer.CatchUpStarted(cursor)
	}
	return l.walkThenDrain(at, cursor, presentClass)
}

// repairCursor selects the repair walk's entry. The connector's IN-MEMORY
// position is authoritative for repair within the run — the store is never
// re-read after its one load, so a failed save neither regresses nor blanks
// what the repair polls from. With no position at all — an entry whose pages
// have not moved one, or one a 409 discarded — the repair re-enters at the
// present, which is present-class.
func (l *loop) repairCursor() (Cursor, bool) {
	if l.position != "" {
		return Cursor{Position: l.position}, false
	}
	return Cursor{Since: sincePresent}, true
}

// reenterWalk dispatches the poll errors whose disposition is a new cursor
// (transitions 17/18/19). It returns the walk's next step, or done=true with
// the outcome that ended the cycle.
func (l *loop) reenterWalk(at *attempt, pe *PollError) (walkStep, cycleOutcome, bool) {
	switch pe.Kind {
	case PollGone:
		return l.recoverGone(at, pe)
	case PollFilterChanged:
		// Transition 19: the held position is DISCARDED before the re-entry.
		// Its lineage belongs to a filter set that is not this connector's, so
		// it can never be resumed from again — an attempt torn down before the
		// re-entry serves a page must not reconnect back onto it. (Transition
		// 18 deliberately does not discard: §23 gives only the 409 that step,
		// and a re-entry's first accepted page overwrites the in-memory cursor
		// either way.) No store delete is needed or offered: the next page's
		// save overwrites under the same key.
		l.position = ""
		return l.reenterAtResetCursor(pe.Kind), cycleOutcome{}, false
	default: // PollPositionInvalid
		// Transition 18: the position the server refused is replaced by the
		// reset cursor; nothing durable moves and the socket is kept.
		return l.reenterAtResetCursor(pe.Kind), cycleOutcome{}, false
	}
}

// recoverGone dispatches transition 17's FeedGap semantic signal. Observer.Gap
// fires first and unconditionally — observability is not disposition — and a
// REGISTERED handler is then invoked exactly once, synchronously, before its
// disposition takes effect. Accept re-enters via the URL the server provided,
// which the walk validates under §8 before issuing any request to it;
// Terminate and no-handler are both Terminal(feed_gap) with NO save. A 410
// never silently auto-continues: with no handler the feed ends here rather
// than following `resume` on the consumer's behalf.
func (l *loop) recoverGone(at *attempt, pe *PollError) (walkStep, cycleOutcome, bool) {
	signal := FeedGap{EpochAfterID: pe.EpochAfterID, ResumeURL: pe.ResumeURL}
	if l.hooks.signalRaised != nil {
		l.hooks.signalRaised(signal)
	}
	if l.cfg.observer.Gap != nil {
		l.cfg.observer.Gap(pe.EpochAfterID, pe.ResumeURL)
	}
	if l.cfg.handler != nil && l.cfg.handler(signal) == Accept {
		if pe.ResumeURL == "" {
			// Accepting a 410 that carried no resume URL cannot be honored:
			// the disposition names a URL the body did not supply, and a bare
			// present entry is emphatically not what was accepted — it would
			// silently reposition the feed at the present head. An empty
			// string fails the same continuation validation every followed URL
			// passes (it is not an absolute URL), so it takes that edge.
			l.disposeAttempt(at, nil)
			return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
				Reason: ReasonInvalidContinuation,
				Msg:    "the 410 carried no resume URL to continue from",
			}}, true
		}
		// The resume URL is a present-class entry (the server documents it as
		// since=now with the canonical filter set preserved), so the entry
		// boundary's hold-then-save discipline governs the pages it serves —
		// and the old cursor is unusable, which is precisely the durability
		// boundary the invariant's exclusion names.
		return walkStep{
			cursor:       Cursor{PageURL: pe.ResumeURL},
			presentClass: true,
			reentry:      true,
		}, cycleOutcome{}, false
	}
	// Terminate, or no handler: the typed terminal, and no save — the error
	// names the epoch and never the resume URL.
	l.disposeAttempt(at, nil)
	return walkStep{}, cycleOutcome{kind: outcomeTerminal, term: &TerminalError{
		Reason: ReasonFeedGap,
		Msg:    fmt.Sprintf("the feed's served history before event %d is gone", pe.EpochAfterID),
	}}, true
}

// reenterAtResetCursor builds the re-entry rows 18/19 share: `since=<last
// poll-served id>`, or `since=now` — a present-class entry — when the poll
// lane has served no id.
//
// The reset cursor is POLL-LANE-ONLY. A live-delivered id is never one: a
// live id above the durable position would re-enter past the un-polled gap
// behind it and permanently skip everything in between. Empty pages serve no
// ids and advance nothing.
func (l *loop) reenterAtResetCursor(kind PollErrorKind) walkStep {
	if l.cfg.observer.PositionRejected != nil {
		l.cfg.observer.PositionRejected(kind)
	}
	if l.lastPollServedID > 0 {
		return walkStep{
			cursor:  Cursor{Since: strconv.FormatInt(l.lastPollServedID, 10)},
			reentry: true,
		}
	}
	return walkStep{
		cursor:       Cursor{Since: sincePresent},
		presentClass: true,
		reentry:      true,
	}
}
