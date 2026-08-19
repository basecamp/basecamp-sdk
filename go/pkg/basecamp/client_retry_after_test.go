package basecamp

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// The raw GET retry loop computes a delay, logs it, fires OnRetry, and only
// then sleeps. Those first three are enough to pin the delay exactly, so the
// tests below never spend one — no seam in production code, no wall clock, and
// no dependence on how long anything took. Measuring elapsed time is the test
// shape this repo is retiring (#783), and it is the wrong instrument anyway:
// the question is whether the loop chose the server's number or its own backoff
// curve, which is an equality, not a range.

// delayRecorder reads the computed delay off the loop's own "retrying request"
// log record — a structured attribute the SDK already emits to any logger a
// caller installs.
type delayRecorder struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (d *delayRecorder) Enabled(context.Context, slog.Level) bool { return true }
func (d *delayRecorder) WithAttrs([]slog.Attr) slog.Handler       { return d }
func (d *delayRecorder) WithGroup(string) slog.Handler            { return d }

func (d *delayRecorder) Handle(_ context.Context, r slog.Record) error {
	if r.Message != "retrying request" {
		return nil
	}
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "delay" {
			if delay, ok := a.Value.Any().(time.Duration); ok {
				d.mu.Lock()
				d.delays = append(d.delays, delay)
				d.mu.Unlock()
			}
		}
		return true
	})
	return nil
}

func (d *delayRecorder) snapshot() []time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]time.Duration(nil), d.delays...)
}

// cancelOnRetryHooks cancels the request at the one instant that makes these
// tests free: the loop fires OnRetry with the delay already computed and
// logged, and the very next thing it does is wait it out. Cancelling here means
// the wait returns at once however long the delay was.
type cancelOnRetryHooks struct {
	NoopHooks
	cancel context.CancelFunc
}

func (h *cancelOnRetryHooks) OnRetry(context.Context, RequestInfo, int, error) {
	h.cancel()
}

// retryAfterProbe drives one GET against a handler, cancelling at the retry
// boundary, and returns the delays the loop computed plus the resulting error.
// The client's backoff curve is set to milliseconds, so any delay at or above a
// second can only have come from the Retry-After header.
func retryAfterProbe(t *testing.T, handler http.HandlerFunc) ([]time.Duration, error) {
	t.Helper()
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	delays := &delayRecorder{}
	client := NewClient(&Config{BaseURL: server.URL, CacheEnabled: false}, &StaticTokenProvider{Token: "test-token"})
	client.httpOpts.MaxRetries = 3
	client.httpOpts.BaseDelay = time.Millisecond
	client.httpOpts.MaxJitter = time.Millisecond
	client.logger = slog.New(delays)
	client.hooks = &cancelOnRetryHooks{cancel: cancel}

	// Not the assertion — the assertions are all exact equalities below. This
	// is the guard that keeps the whole file honest: if the loop's wait ever
	// stopped observing cancellation, every case here would spend its full
	// Retry-After instead of returning at once, and this says so in one line
	// rather than hanging until the package test timeout.
	//
	// Get runs in a goroutine so the guard can actually fire (review
	// follow-up, Copilot). Called inline, a wait that ignored cancellation
	// would simply never return, and an elapsed-time check placed after it is
	// unreachable — most obviously now that an over-range Retry-After
	// saturates at ~292 years rather than wrapping to a negative delay. The
	// abandoned goroutine outlives the test; that is the cost of reporting a
	// hang instead of becoming one.
	type outcome struct {
		delays []time.Duration
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		_, err := client.Get(ctx, "/test.json")
		done <- outcome{delays: delays.snapshot(), err: err}
	}()

	select {
	case got := <-done:
		return got.delays, got.err
	case <-time.After(5 * time.Second):
		t.Fatalf("Get had not returned 5s after cancellation at the retry boundary; "+
			"the retry wait is not interruptible (delays computed so far: %v)", delays.snapshot())
		return nil, nil
	}
}

func rateLimited(retryAfter string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}
}

// TestClient_RetryAfterReplacesBackoff is the regression test for the raw GET
// retry loop silently discarding a server-specified Retry-After. The loop used
// to read the delay off a *retryableError that nothing in the package ever
// constructed, so the branch was unreachable and every 429 fell through to the
// backoff curve. Against that code this observes ~1ms, wants 2s, and fails.
func TestClient_RetryAfterReplacesBackoff(t *testing.T) {
	delays, err := retryAfterProbe(t, rateLimited("2"))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get returned %v, want context.Canceled", err)
	}
	if len(delays) != 1 {
		t.Fatalf("loop computed %d delays (%v), want exactly 1", len(delays), delays)
	}
	if delays[0] != 2*time.Second {
		t.Errorf("computed a %v retry delay, want 2s from the Retry-After header "+
			"(the backoff curve here is ~1ms, so this is the backoff, not the server's number)", delays[0])
	}
}

// TestClient_RetryAfterHTTPDateReplacesBackoff covers the header's other wire
// form. parseRetryAfter resolves it to whole seconds relative to now, so the
// assertion is a tight window rather than an equality — but one nowhere near
// the millisecond backoff it has to be told apart from.
func TestClient_RetryAfterHTTPDateReplacesBackoff(t *testing.T) {
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	delays, err := retryAfterProbe(t, rateLimited(future))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get returned %v, want context.Canceled", err)
	}
	if len(delays) != 1 {
		t.Fatalf("loop computed %d delays (%v), want exactly 1", len(delays), delays)
	}
	if delays[0] < 80*time.Second || delays[0] > 90*time.Second {
		t.Errorf("computed a %v retry delay for an HTTP-date 90s out, want ~90s", delays[0])
	}
}

// TestClient_RetryAfterAbsentOrUnusableKeepsBackoff is the other half of the
// contract, and the half that stops the tests above from passing for the wrong
// reason: a "fix" that always slept a whole number of seconds would satisfy
// them. Only a header the server actually sent, and that SPEC §6 accepts, may
// displace the curve.
func TestClient_RetryAfterAbsentOrUnusableKeepsBackoff(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
	}{
		{"absent", ""},
		{"unparseable", "sometime next week"},
		{"partly numeric", "120junk"},
		{"zero", "0"},
		{"negative", "-5"},
		{"http-date in the past", "Wed, 09 Jun 2021 10:18:14 GMT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			delays, err := retryAfterProbe(t, rateLimited(tc.header))

			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Get returned %v, want context.Canceled", err)
			}
			if len(delays) != 1 {
				t.Fatalf("loop computed %d delays (%v), want exactly 1", len(delays), delays)
			}
			// backoffDelay(1) is BaseDelay + [0, MaxJitter), both 1ms here.
			if delays[0] < time.Millisecond || delays[0] >= 2*time.Millisecond {
				t.Errorf("computed a %v retry delay, want the backoff curve's [1ms, 2ms) — "+
					"an unusable Retry-After must not displace it", delays[0])
			}
		})
	}
}

// TestClient_RetryAfterSaturatesAtDurationCeiling is the regression test for a
// server turning the retry loop into a tight loop with a syntactically valid
// header. time.Duration counts nanoseconds in an int64, so an unclamped
// `Retry-After: 9223372036854775807` multiplied by time.Second wraps to -1s,
// and time.After on a non-positive duration fires immediately: the loop would
// burn its whole attempt budget back-to-back against a peer that just asked it
// to wait. Against the unclamped code this observes -1s and fails.
//
// The delay is asserted, not the elapsed time — a clamped wait is ~68 years,
// which is precisely why nothing here may sleep it.
//
// Both headers saturate, and the second is the point: `…808` is one past the
// largest int64, so ParseInt reports it as out of range. Honouring `…807` and
// discarding `…808` would put a cliff between two values one digit apart, for
// no reason a server could see — RFC 9110 asks only for `1*DIGIT` (review
// follow-up, Codex).
func TestClient_RetryAfterSaturatesAtDurationCeiling(t *testing.T) {
	for _, header := range []string{"9223372036854775807", "9223372036854775808", "99999999999999999999"} {
		t.Run(header, func(t *testing.T) { assertSaturatedRetryAfter(t, header) })
	}
}

func assertSaturatedRetryAfter(t *testing.T, header string) {
	t.Helper()
	delays, err := retryAfterProbe(t, rateLimited(header))

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get returned %v, want context.Canceled", err)
	}
	if len(delays) != 1 {
		t.Fatalf("loop computed %d delays (%v), want exactly 1", len(delays), delays)
	}
	if delays[0] <= 0 {
		t.Fatalf("computed a %v retry delay from an over-range Retry-After; a non-positive "+
			"delay makes time.After fire at once, so the server-directed wait becomes a tight loop", delays[0])
	}
	if want := time.Duration(maxRetryAfterSeconds) * time.Second; delays[0] != want {
		t.Errorf("computed a %v retry delay, want %v — an over-range Retry-After saturates at "+
			"the honoured ceiling rather than falling back to the ~1ms backoff curve", delays[0], want)
	}
}

// TestParseRetryAfter_FarFutureHTTPDateSaturates covers the header's other wire
// form at the same boundary. A server may legally name a date beyond anything a
// Duration can hold — RFC 7231 puts no bound on it, and year-9999 dates are
// real — for which time.Until saturates at ~292 years. That must saturate the
// parsed delay too, not be discarded onto the millisecond backoff curve.
//
// Stated honestly (review follow-up, Codex/Copilot): the defect this guards is
// 32-bit-only and therefore cannot be executed here. `int(d.Seconds())` narrows
// a float64, and Go leaves an out-of-range float→int conversion
// implementation-defined; where int is 32 bits that produced a non-positive
// value and the header was dropped. The remedy is to remove the narrowing —
// whole seconds now come from integer division in the Duration domain — so this
// pins the resulting contract on every platform rather than proving the fix on
// the one that had the bug.
func TestParseRetryAfter_FarFutureHTTPDateSaturates(t *testing.T) {
	seconds := parseRetryAfter("Fri, 31 Dec 9999 23:59:59 GMT")

	if seconds <= 0 {
		t.Fatalf("parseRetryAfter(a year-9999 HTTP-date) = %d, want a positive saturated delay — "+
			"a non-positive result reads as 'no delay' and drops the server's wait onto the backoff curve", seconds)
	}
	if int64(seconds) > maxRetryAfterSeconds {
		t.Errorf("parseRetryAfter(a year-9999 HTTP-date) = %d, want at most %d", seconds, maxRetryAfterSeconds)
	}
	if delay := time.Duration(seconds) * time.Second; delay <= 0 {
		t.Errorf("time.Duration(%d) * time.Second = %v, want a positive duration", seconds, delay)
	}
}

// TestParseRetryAfter_HTTPDateRoundsUp covers the other end of the same branch.
// A remainder truncated toward zero turns the shortest honoured delay into "no
// delay at all" — a date under a second out becomes 0, which every caller reads
// as absent, so the request falls onto the millisecond backoff curve instead of
// waiting. It also retries before the moment the server named. TypeScript
// (`Math.ceil`), Kotlin (`(remainingMs + 999) / 1000`) and Swift
// (`.rounded(.up)`) all round up for those two reasons; Go now joins them, and
// SPEC §6 step 2 says so. Python and Ruby still truncate (#799).
//
// The assertion is ONE-SIDED, which is what keeps it out of #783's territory
// (review follow-up, Codex). An equality against a literal cannot work here:
// the wire form carries whole seconds, so the remaining time is
// `offset - frac(now) - scheduling delay`, and the room before the answer drops
// by one is `1 - frac(now)` — arbitrarily small, entirely at the clock's
// discretion. Instead the bound is re-measured AFTER the parse, making it a
// lower bound on what the parser itself saw: scheduling delay can only weaken
// this toward vacuity, never turn it red.
func TestParseRetryAfter_HTTPDateRoundsUp(t *testing.T) {
	// The next whole second is the shortest future date the wire form can
	// express, and the case with the widest gap between the two roundings:
	// under a second remains, which truncation reports as 0 — discarded — and
	// rounding up reports as 1.
	target := time.Now().Truncate(time.Second).Add(time.Second)

	seconds := parseRetryAfter(target.UTC().Format(http.TimeFormat))

	remaining := time.Until(target)
	if remaining <= 0 {
		// The target second passed between formatting and measuring, so there
		// is nothing left to be right or wrong about: a parser returning 0 for
		// a past date is correct, and asserting anything here would be
		// asserting the scheduler.
		t.Skip("the target second elapsed during the test; nothing to assert")
	}
	if delay := time.Duration(seconds) * time.Second; delay < remaining {
		t.Errorf("parseRetryAfter(a date %v away) = %ds, shorter than the wait the server named — "+
			"truncating a sub-second remainder yields 0, which reads as absent and drops the "+
			"request onto the backoff curve; any truncation retries early", remaining, seconds)
	}
}

// TestErrRateLimit_NormalizesRetryAfter covers the one door onto Error.RetryAfter
// the wire parser does not guard: an exported constructor taking a bare int.
// The field documents zero as "no delay", so a negative argument must not reach
// it — and the hint, which has always read non-positive as absent, must keep
// saying the same thing the field does.
func TestErrRateLimit_NormalizesRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name       string
		retryAfter int
		want       int
		wantHint   string
	}{
		{"positive", 42, 42, "Try again in 42 seconds"},
		{"zero", 0, 0, "Try again later"},
		{"negative", -5, 0, "Try again later"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ErrRateLimit(tc.retryAfter)
			if err.RetryAfter != tc.want {
				t.Errorf("ErrRateLimit(%d).RetryAfter = %d, want %d", tc.retryAfter, err.RetryAfter, tc.want)
			}
			if err.Hint != tc.wantHint {
				t.Errorf("ErrRateLimit(%d).Hint = %q, want %q", tc.retryAfter, err.Hint, tc.wantHint)
			}
		})
	}

	t.Run("beyond duration range", func(t *testing.T) {
		err := ErrRateLimit(math.MaxInt)
		want := maxRetryAfterSeconds
		if err.RetryAfter != want {
			t.Errorf("ErrRateLimit(math.MaxInt).RetryAfter = %d, want %d (saturated, so the "+
				"retry loop's seconds→Duration conversion stays positive)", err.RetryAfter, want)
		}
		if delay := time.Duration(err.RetryAfter) * time.Second; delay <= 0 {
			t.Errorf("time.Duration(RetryAfter) * time.Second = %v, want a positive duration", delay)
		}
	})
}

// TestClient_RetryAfterErrorCarriesSeconds pins the value onto the error the
// caller sees, not just onto the loop's internal delay: an application that
// gives up and reschedules the work itself needs the number too.
func TestClient_RetryAfterErrorCarriesSeconds(t *testing.T) {
	server := httptest.NewServer(rateLimited("42"))
	defer server.Close()

	client := NewClient(&Config{BaseURL: server.URL, CacheEnabled: false}, &StaticTokenProvider{Token: "test-token"})
	client.httpOpts.MaxRetries = 1 // one attempt, no wait, error straight back

	_, err := client.Get(context.Background(), "/test.json")
	if err == nil {
		t.Fatal("expected an error from a 429, got nil")
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error %T is not an *Error: %v", err, err)
	}
	if apiErr.RetryAfter != 42 {
		t.Errorf("Error.RetryAfter = %d, want 42", apiErr.RetryAfter)
	}
}

// TestCheckResponse_CarriesRetryAfter covers the other door onto *Error: every
// generated service method maps its 429 through checkResponse, so leaving the
// field unset there would make it trustworthy on the raw path and zero on the
// typed one.
func TestCheckResponse_CarriesRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header string
		want   int
	}{
		{name: "seconds", header: "17", want: 17},
		{name: "absent", header: "", want: 0},
		{name: "unparseable", header: "whenever", want: 0},
		// The typed path builds its *Error directly from the parsed header —
		// it never passes through ErrRateLimit — so this is the case that
		// holds the parser's own clamp. Without it a generated service method
		// hands the caller a RetryAfter that overflows the moment anyone
		// multiplies it by time.Second, which is what downloadURL, the
		// resilience hook's rate-limiter block, and any caller rescheduling
		// off err.RetryAfter all do.
		{name: "beyond duration range", header: "9223372036854775807", want: maxRetryAfterSeconds},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
			if tc.header != "" {
				resp.Header.Set("Retry-After", tc.header)
			}
			err := checkResponse(resp, nil)
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("checkResponse returned %T, want *Error", err)
			}
			if apiErr.RetryAfter != tc.want {
				t.Errorf("Error.RetryAfter = %d, want %d", apiErr.RetryAfter, tc.want)
			}
		})
	}
}
