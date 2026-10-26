package eventfeed

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// In-band renewal (SPEC.md §23 "In-Band Renewal"). A server that bounds a
// ticket stream by its token's expiry advertises that deadline in the
// socket's welcome, as stream_expires_at, and closes the socket at it with
// `remote` unless the client first sends a fresh ticket over the open socket:
//
//	{"command":"renew","identifier":<the subscription's identifier>,"ticket":<fresh ticket>}
//
// The server redeems the ticket exactly as a handshake would and answers the
// renewing socket alone, {"type":"renewed","stream_expires_at":…} with the new
// token's expiry or {"type":"renew_refused","reason":…}. Renewal is therefore a
// reconnect that keeps the socket: it grants nothing a reconnect would not, and
// a renewal that fails costs nothing a reconnect does not already pay — the
// lease closes the socket, the reconnect cycle re-mints and re-dials, and the
// catch-up walk repairs the gap. That is what lets every failure here be
// non-terminal and uncounted.
//
// The whole exchange lives in Streaming. The renew-due timer is armed only
// there (stopped across a repair walk, re-armed for the remainder after it),
// and the repair cadence waits while an exchange is in flight, so CatchingUp
// and Draining keep the timer sets they always had and a reply is never
// awaited from inside a walk. A socket whose welcome advertised nothing is
// never renewed: an old server would answer the command, and a server that
// does not bound the stream has nothing to renew.

// Timer kinds the renewal arms (SPEC.md §23 "Clock, Timers, and Virtual
// Time").
const (
	timerRenewDue         = "renew-due"
	timerRenewAckDeadline = "renew-ack-deadline"
)

// Renewal constants (SPEC.md §23 "Constants" / Appendix A's EVENT_FEED_RENEW_*
// rows).
const (
	// renewLead (EVENT_FEED_RENEW_LEAD, 3 min) is how long before the stream
	// deadline a renewal is due, jittered by renewJitter. It sits inside the
	// 5-minute window in which a token provider that refreshes lazily ahead of
	// expiry (the CLI's) has already replaced the token the stream was opened
	// with, so the mint carries a token whose expiry is later — the one thing
	// the server's renewal requires.
	renewLead = 3 * time.Minute
	// renewJitter (EVENT_FEED_RENEW_JITTER, ±30 s) spreads renewals of
	// streams opened together.
	renewJitter = 30 * time.Second
	// renewAckDeadline (EVENT_FEED_RENEW_ACK_DEADLINE, 10 s) bounds one
	// attempt from the renew-due firing to the server's reply: the mint, the
	// write, and the answer.
	renewAckDeadline = 10 * time.Second
	// renewMinInterval (EVENT_FEED_RENEW_MIN_INTERVAL, 30 s) is the shortest
	// gap between the end of one attempt on a socket and the next renew it
	// sends — the server refuses more than one renewal per 30 s — and the
	// delay of the one retry.
	renewMinInterval = 30 * time.Second
	// renewAttempts (EVENT_FEED_RENEW_ATTEMPTS, 2) is the attempts spent
	// against one deadline: the renewal and one retry.
	renewAttempts = 2
)

// errRenewWriteLapsed reports a renew command whose write was still
// outstanding when its acknowledgement deadline lapsed: the peer is not
// reading, which is a socket failure rather than a refused renewal.
var errRenewWriteLapsed = errors.New("event feed renew write did not complete within its acknowledgement deadline")

// RenewFailure classifies one failed renewal attempt for
// Observer.RenewFailed. It is a closed vocabulary on purpose: a refusal's
// reason is peer text and decides nothing, so it is never forwarded.
type RenewFailure int

const (
	// RenewRefused — the server answered renew_refused (for instance, the
	// fresh ticket's token expires no later than the one the stream holds).
	RenewRefused RenewFailure = iota + 1
	// RenewNoReply — the renew command was sent and nothing answered it
	// within the acknowledgement deadline.
	RenewNoReply
	// RenewMintFailed — minting the fresh ticket failed, or did not finish
	// within the acknowledgement deadline. Never counted toward the shared
	// authorization counter: the reconnect cycle's own mint is what judges
	// the bearer.
	RenewMintFailed
	// RenewInvalidReply — the server answered `renewed` without a usable
	// stream deadline later than the one it replaces.
	RenewInvalidReply
)

// String returns the failure's name.
func (k RenewFailure) String() string {
	switch k {
	case RenewRefused:
		return "refused"
	case RenewNoReply:
		return "no_reply"
	case RenewMintFailed:
		return "mint_failed"
	case RenewInvalidReply:
		return "invalid_reply"
	default:
		return fmt.Sprintf("RenewFailure(%d)", int(k))
	}
}

// renewal is one socket's renewal state. It exists only when the socket's
// welcome advertised a usable stream deadline, and it belongs to the attempt:
// a new socket gets its own welcome, and with it its own deadline.
type renewal struct {
	// expiresAt is the socket's stream deadline as last advertised or
	// renewed — the server's clock, compared against the injected Clock's
	// reading (SPEC.md §23: the one absolute comparison the connector makes).
	expiresAt time.Time
	// target is the Clock reading the next renew is due at, valid while
	// scheduled. It is absolute so a repair walk that stops renew-due can
	// re-arm it for the remainder rather than redraw it.
	target    time.Time
	scheduled bool
	// failures counts failed attempts against expiresAt; a renewal resets it.
	failures int
	// due is the armed renew-due timer — Streaming only, nil otherwise.
	due Timer
	// ex is the attempt in flight, nil between attempts.
	ex *renewExchange
}

// renewExchange is one renewal attempt in flight: the mint, then the write,
// then the wait for the reply, all under one acknowledgement deadline.
//
// The deadline is AUTHORITATIVE. When it and a mint's result, a write's
// result or a reply are ready together, the select picks one at random, so
// every arm that would move the attempt forward first asks whether the
// deadline has already fired — Stop's result is the ordering discriminator,
// as it is for the handshake and confirmation deadlines — and an attempt the
// deadline has fired on fails as the deadline says.
//
// An attempt also never ends with its write unobserved. A reply can beat the
// write's completion to the state machine (the server answers a frame the
// moment it has it), so a reply that arrives while the write's outcome is
// still out is PARKED, and taken once that outcome is in: a failed write is
// then the socket's failure whatever the reply said, a write still blocked at
// the deadline is a socket that is not reading, and no second attempt can
// start a write beside one that never returned.
type renewExchange struct {
	// cancel cancels the mint. The write rides the attempt's own context and
	// is never cancelled on its own: a cancelled write may abort the socket
	// it is writing to.
	cancel context.CancelFunc
	// ack is the renew-ack-deadline timer, nil once fired. deadline is the
	// instant it was armed for, so a re-arm keeps the attempt's one bound.
	ack      Timer
	deadline time.Time
	// minted carries the mint's outcome; nil once consumed.
	minted chan renewMint
	// written carries the write's outcome; nil until the write starts and
	// once consumed.
	written chan renewWrite
	// sent reports that the renew command's write has started. From then on
	// a reply is the attempt's.
	sent bool
	// reply is a reply that arrived while the write's outcome was still out,
	// parked until it is in. The first one is kept: there is one renewal in
	// flight, so a second reply answers nothing.
	reply *frame
}

type renewMint struct {
	ticket   StreamTicket
	err      error
	panicked *seamPanic
}

type renewWrite struct {
	err      error
	panicked *seamPanic
}

// newRenewal schedules a socket's first renewal from its welcome's advert.
func (l *loop) newRenewal(expiresAt time.Time) *renewal {
	return &renewal{
		expiresAt: expiresAt,
		target:    l.renewTarget(expiresAt),
		scheduled: true,
	}
}

// renewTarget draws the instant a renewal of deadline is due: renewLead
// before it, ±renewJitter.
func (l *loop) renewTarget(deadline time.Time) time.Time {
	jitter := time.Duration((2*l.cfg.rand() - 1) * float64(renewJitter))
	return deadline.Add(jitter - renewLead)
}

// armRenewDue arms renew-due for the scheduled target — a remainder, floored
// at zero, so a target already behind the clock fires at once. It is a no-op
// unless a renewal is scheduled and not already armed, which is what lets the
// Streaming entry points and the attempt's resolution share it.
func (l *loop) armRenewDue(at *attempt) {
	rn := at.renew
	if rn == nil || !rn.scheduled || rn.due != nil {
		return
	}
	rn.due = l.cfg.clock.NewTimer(max(rn.target.Sub(l.cfg.clock.Now()), 0), timerRenewDue)
}

// stopRenewDue stops an armed renew-due, keeping the schedule: Streaming is
// the only state that arms it, so a repair walk stops it on the way out and
// armRenewDue re-arms the remainder on the way back.
func (l *loop) stopRenewDue(at *attempt) {
	if rn := at.renew; rn != nil && rn.due != nil {
		rn.due.Stop()
		rn.due = nil
	}
}

// stop ends the socket's renewal for good — its timers stopped and its mint
// cancelled. disposeAttempt calls it on every teardown.
func (rn *renewal) stop() {
	if rn.due != nil {
		rn.due.Stop()
		rn.due = nil
	}
	rn.endExchange()
	rn.scheduled = false
}

// endExchange retires the attempt in flight, if any: its deadline stopped and
// its mint cancelled. Outside a teardown it is only ever reached with the
// write's outcome in (renewExchange), so no write is left behind it.
func (rn *renewal) endExchange() {
	ex := rn.ex
	if ex == nil {
		return
	}
	if ex.ack != nil {
		ex.ack.Stop()
	}
	ex.cancel()
	rn.ex = nil
}

// renewSelect is the Streaming select's view of the renewal: the channels of
// whatever is armed or in flight, nil (never ready) otherwise. While an
// exchange is in flight the repair cadence is held — a reply is only ever
// awaited from Streaming — and a cadence that fired meanwhile is taken as
// soon as the exchange ends.
type renewSelect struct {
	repair  <-chan time.Time
	due     <-chan time.Time
	ack     <-chan time.Time
	minted  <-chan renewMint
	written <-chan renewWrite
}

func (l *loop) renewChannels(at *attempt, repair Timer) renewSelect {
	sel := renewSelect{repair: repair.C()}
	rn := at.renew
	if rn == nil {
		return sel
	}
	if rn.due != nil {
		sel.due = rn.due.C()
	}
	if ex := rn.ex; ex != nil {
		sel.repair = nil
		if ex.ack != nil {
			sel.ack = ex.ack.C()
		}
		sel.minted = ex.minted
		sel.written = ex.written
	}
	return sel
}

// renewDue starts an attempt: the renew-due timer fired in Streaming. The
// acknowledgement deadline arms first, and the mint runs on its own goroutine
// so the stream keeps delivering while it is in flight.
func (l *loop) renewDue(at *attempt, pending Timer) (cycleOutcome, bool) {
	rn := at.renew
	rn.due = nil
	// The expired window outranks the renewal exactly as it outranks the
	// repair cadence: a mint for a socket whose staleness verdict is already
	// in is a renewal of a socket about to be torn down.
	if out, done := l.expiredStaleness(at, pending); done {
		return out, true
	}
	if l.runCtx.Err() != nil {
		l.disposeAttempt(at, pending)
		return cycleOutcome{kind: outcomeClosed}, true
	}
	rn.scheduled = false
	ctx, cancel := context.WithCancel(at.ctx)
	minted := make(chan renewMint, 1)
	rn.ex = &renewExchange{
		cancel:   cancel,
		deadline: l.cfg.clock.Now().Add(renewAckDeadline),
		ack:      l.cfg.clock.NewTimer(renewAckDeadline, timerRenewAckDeadline),
		minted:   minted,
	}
	go func() {
		var r renewMint
		r.panicked = capturePanic(func() { r.ticket, r.err = l.cfg.minter.MintStreamTicket(ctx) })
		minted <- r
	}()
	return cycleOutcome{}, false
}

// renewMinted takes the mint's outcome: a failed mint is a failed attempt,
// never terminal and never counted; a ticket is written in the renew command.
func (l *loop) renewMinted(at *attempt, pending Timer, r renewMint) (cycleOutcome, bool) {
	ex := at.renew.ex
	ex.minted = nil
	if r.panicked != nil {
		// Re-raised on the state machine's goroutine; runCycle's recovery
		// disposes the attempt, renewal included.
		panic(r.panicked.value)
	}
	if l.runCtx.Err() != nil {
		l.disposeAttempt(at, pending)
		return cycleOutcome{kind: outcomeClosed}, true
	}
	if out, done := l.expiredStaleness(at, pending); done {
		return out, true
	}
	// A mint that finished behind its deadline starts no write: the deadline
	// had fired, and the attempt is the deadline's.
	if l.ackFired(ex) {
		return l.renewFailed(at, pending, RenewMintFailed)
	}
	if r.err != nil || r.ticket.Ticket == "" {
		return l.renewFailed(at, pending, RenewMintFailed)
	}
	command := renewCommand(l.identifier, r.ticket.Ticket)
	written := make(chan renewWrite, 1)
	ex.written = written
	ex.sent = true
	go func() {
		var w renewWrite
		w.panicked = capturePanic(func() { w.err = at.lc.conn.WriteFrame(at.ctx, command) })
		written <- w
	}()
	if l.hooks.renewSent != nil {
		l.hooks.renewSent()
	}
	return cycleOutcome{}, false
}

// renewWritten takes the write's outcome. A failed write is the socket's
// failure (CableConn: "a write failure takes the current state's
// socket-failure path"), whatever reply may be parked behind it; a completed
// one takes the parked reply, if there is one, or leaves the attempt awaiting
// it.
func (l *loop) renewWritten(at *attempt, pending Timer, w renewWrite) (cycleOutcome, bool) {
	ex := at.renew.ex
	ex.written = nil
	if w.panicked != nil {
		panic(w.panicked.value)
	}
	if l.runCtx.Err() != nil {
		l.disposeAttempt(at, pending)
		return cycleOutcome{kind: outcomeClosed}, true
	}
	if out, done := l.expiredStaleness(at, pending); done {
		return out, true
	}
	if w.err != nil {
		return l.failSocket(at, pending, w.err), true
	}
	if parked := ex.reply; parked != nil {
		ex.reply = nil
		return l.resolveRenewal(at, pending, *parked)
	}
	return cycleOutcome{}, false
}

// renewLapsed is the acknowledgement deadline's arm. Close and an expired
// staleness window outrank it as they outrank every other arm of the select —
// a lapse taken after Close would report a failed renewal and schedule a
// retry for a feed the consumer ended, and one taken over a dead socket's
// verdict would report the wrong one.
func (l *loop) renewLapsed(at *attempt, pending Timer) (cycleOutcome, bool) {
	if l.runCtx.Err() != nil {
		l.disposeAttempt(at, pending)
		return cycleOutcome{kind: outcomeClosed}, true
	}
	if out, done := l.expiredStaleness(at, pending); done {
		return out, true
	}
	at.renew.ex.ack = nil
	return l.renewExpired(at, pending)
}

// renewExpired fails an attempt whose deadline has fired, by how far it got. A
// mint still outstanding is cancelled and JOINED — the seam returns promptly
// on cancellation, and a panic it raised there is re-raised on this goroutine
// as every seam's is — and fails as mint_failed. A write still outstanding is
// a socket that is not reading, unless its outcome is already waiting, in
// which case that outcome is taken first: a write that completed is not a
// stalled socket, and one that failed is the socket's failure. A command
// written and unanswered by the deadline is no_reply, and a reply parked
// behind the write is the deadline's too.
func (l *loop) renewExpired(at *attempt, pending Timer) (cycleOutcome, bool) {
	ex := at.renew.ex
	if ex.minted != nil {
		ex.cancel()
		r := <-ex.minted
		ex.minted = nil
		if r.panicked != nil {
			panic(r.panicked.value)
		}
		return l.renewFailed(at, pending, RenewMintFailed)
	}
	if ex.written != nil {
		select {
		case w := <-ex.written:
			ex.written = nil
			if w.panicked != nil {
				panic(w.panicked.value)
			}
			if w.err != nil {
				return l.failSocket(at, pending, w.err), true
			}
		default:
			return l.failSocket(at, pending, errRenewWriteLapsed), true
		}
	}
	return l.renewFailed(at, pending, RenewNoReply)
}

// ackFired reports whether the attempt's deadline has already fired, taking
// the verdict: Stop's result is the discriminator, not the channel, which a
// real clock fills a moment after Stop starts reporting false. A deadline that
// has not fired is re-armed for what remains of it — the attempt keeps the one
// bound it started with — the new timer armed before the old one is stopped,
// so the set never passes through one without it.
func (l *loop) ackFired(ex *renewExchange) bool {
	if ex.ack == nil {
		return true
	}
	next := l.cfg.clock.NewTimer(max(ex.deadline.Sub(l.cfg.clock.Now()), 0), timerRenewAckDeadline)
	if !ex.ack.Stop() {
		next.Stop()
		ex.ack = nil
		return true
	}
	ex.ack = next
	return false
}

// renewReply takes a `renewed` or `renew_refused` frame. Only an attempt
// whose command has been sent can be answered; any other reply — on a socket
// that advertised nothing, between attempts, after an attempt already ended —
// is liveness, as an unknown frame type is. A late `renewed` is not adopted:
// the attempt it answered was already counted, and the worst that follows is
// the lease closing a socket the connector stopped renewing, which the
// reconnect cycle recovers. A reply that beats its own write's outcome is
// parked until that outcome is in (renewExchange).
func (l *loop) renewReply(at *attempt, pending Timer, f frame) (cycleOutcome, bool) {
	rn := at.renew
	if rn == nil || rn.ex == nil || !rn.ex.sent {
		return cycleOutcome{}, false
	}
	if ex := rn.ex; ex.written != nil {
		if ex.reply == nil {
			ex.reply = &f
		}
		return cycleOutcome{}, false
	}
	return l.resolveRenewal(at, pending, f)
}

// resolveRenewal ends an attempt on its reply, its write's outcome in — unless
// the deadline fired first, in which case the attempt is the deadline's and
// the reply answered nothing.
func (l *loop) resolveRenewal(at *attempt, pending Timer, f frame) (cycleOutcome, bool) {
	rn := at.renew
	// Re-armed rather than merely stopped when it has not fired: the attempt's
	// end below arms the next renew-due before it stops the deadline, and a
	// deadline already stopped here would leave a moment with neither.
	if l.ackFired(rn.ex) {
		return l.renewFailed(at, pending, RenewNoReply)
	}
	switch {
	case f.kind == frameRenewRefused:
		return l.renewFailed(at, pending, RenewRefused)
	case !f.hasExpiresAt || !f.expiresAt.After(rn.expiresAt):
		return l.renewFailed(at, pending, RenewInvalidReply)
	}
	rn.expiresAt = f.expiresAt
	rn.failures = 0
	// The next renewal is drawn from the new deadline, and never sooner than
	// renewMinInterval from now: the server refuses a second renewal inside
	// that window, so a deadline granted close to the present must not turn
	// into a renewal loop.
	rn.target = l.renewTarget(f.expiresAt)
	if floor := l.cfg.clock.Now().Add(renewMinInterval); rn.target.Before(floor) {
		rn.target = floor
	}
	rn.scheduled = true
	// The next renew-due is armed BEFORE the deadline it replaces is stopped,
	// so the outstanding set never passes through one without renew-due — the
	// set a renewal that gave up leaves behind.
	l.armRenewDue(at)
	rn.endExchange()
	if l.cfg.observer.Renewed != nil {
		l.cfg.observer.Renewed(f.expiresAt)
	}
	if l.runCtx.Err() != nil {
		l.disposeAttempt(at, pending)
		return cycleOutcome{kind: outcomeClosed}, true
	}
	return cycleOutcome{}, false
}

// renewFailed ends a failed attempt. The first failure against a deadline is
// retried once, renewMinInterval later; the second gives the deadline up, and
// the socket runs to the server's lease. Neither is terminal and neither
// touches the shared authorization counter: an unrenewed socket closes with
// `remote`, and the reconnect cycle that follows owns every judgment about
// the bearer.
func (l *loop) renewFailed(at *attempt, pending Timer, kind RenewFailure) (cycleOutcome, bool) {
	rn := at.renew
	rn.failures++
	retrying := rn.failures < renewAttempts
	if retrying {
		rn.target = l.cfg.clock.Now().Add(renewMinInterval)
		rn.scheduled = true
		// Armed before the attempt's deadline is stopped, as on success.
		l.armRenewDue(at)
	}
	rn.endExchange()
	if l.cfg.observer.RenewFailed != nil {
		l.cfg.observer.RenewFailed(kind, retrying)
	}
	if l.runCtx.Err() != nil {
		l.disposeAttempt(at, pending)
		return cycleOutcome{kind: outcomeClosed}, true
	}
	return cycleOutcome{}, false
}
