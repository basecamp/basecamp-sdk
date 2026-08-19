package basecamp

import (
	"context"
	"errors"
	"log/slog"
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

	start := time.Now()
	_, err := client.Get(ctx, "/test.json")
	// Not the assertion — the assertions are all exact equalities below. This
	// is the guard that keeps the whole file honest: if the loop's wait ever
	// stopped observing cancellation, every case here would spend its full
	// Retry-After instead of returning at once, and this says so in one line
	// rather than hanging until the package test timeout.
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Get took %v after cancellation at the retry boundary; the retry wait is not interruptible", elapsed)
	}
	return delays.snapshot(), err
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
		{"seconds", "17", 17},
		{"absent", "", 0},
		{"unparseable", "whenever", 0},
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
