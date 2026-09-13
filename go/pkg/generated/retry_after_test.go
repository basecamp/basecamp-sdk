package generated_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// The generated client's retry loop, driven through its public surface the
// way auth_transport_test.go drives the transport: nothing here names an
// unexported symbol, so a regeneration cannot leave it testing a shape the
// template no longer emits. Before #855 the loop did a bare strconv.Atoi
// behind a 429 gate (#798): no HTTP-date form, a 503's header ignored, and a
// seconds×time.Second product that wrapped negative for the largest int64 —
// an already-expired timer, so a typed operation burned its whole attempt
// budget back to back against an origin that had asked it to wait.

// retryAfterClient answers every request with the given handler and retries
// on a millisecond curve, so any delay at or above a second can only have
// come from the Retry-After header.
func retryAfterClient(t *testing.T, handler http.HandlerFunc) *generated.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := generated.NewClient(server.URL, generated.WithRetryConfig(generated.RetryConfig{
		MaxRetries: 3,
		BaseDelay:  time.Millisecond,
		MaxDelay:   time.Millisecond,
		Multiplier: 2,
	}))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func serviceUnavailableThenOK(retryAfter string, attempts *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", retryAfter)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}
}

// A 503's Retry-After governs the wait (SPEC §6 "Retry-After Honouring"), in
// both wire forms, and is waited exactly rather than as a floor under the
// jittered curve — the elapsed floor is the header's own value.
func TestGeneratedClient_HonoursRetryAfterAt503(t *testing.T) {
	for _, tc := range []struct {
		name        string
		header      func() string
		wantAtLeast time.Duration
	}{
		{"delta-seconds", func() string { return "2" }, 2 * time.Second},
		// Minted inside the subtest, not before the table: the delta-seconds
		// case spends two seconds, and a date minted before it would be a
		// second nearer by the time this case reads it. A whole-second date,
		// because the wire form carries whole seconds: the remainder at parse
		// time is in (2s, 3s] and rounds up to 3.
		{"http-date", func() string {
			return time.Now().Truncate(time.Second).Add(3 * time.Second).UTC().Format(http.TimeFormat)
		}, 2 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			client := retryAfterClient(t, serviceUnavailableThenOK(tc.header(), &attempts))

			start := time.Now()
			resp, err := client.GetProject(context.Background(), "999", 1)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("GetProject: %v", err)
			}
			_ = resp.Body.Close()
			if got := attempts.Load(); got != 2 {
				t.Errorf("made %d requests, want 2", got)
			}
			if elapsed < tc.wantAtLeast {
				t.Errorf("retried after %v, want at least %v — the 503's Retry-After must replace the millisecond curve", elapsed, tc.wantAtLeast)
			}
		})
	}
}

// The regression for the wrapped conversion. Against the old Atoi loop the
// largest int64 became a -1s timer that fired at once, and the whole attempt
// budget went out inside the deadline; against the saturating parser the loop
// is still waiting when the deadline lands, having sent exactly one request.
func TestGeneratedClient_OverRangeRetryAfterSaturatesRatherThanWrapping(t *testing.T) {
	for _, header := range []string{"9223372036854775807", "99999999999999999999", "2147483648"} {
		t.Run(header, func(t *testing.T) {
			var attempts atomic.Int32
			client := retryAfterClient(t, serviceUnavailableThenOK(header, &attempts))

			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			resp, err := client.GetProject(ctx, "999", 1)
			if resp != nil {
				_ = resp.Body.Close()
			}

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("GetProject returned %v, want context.DeadlineExceeded — the loop must still be waiting out the (saturated) header when the deadline lands", err)
			}
			if got := attempts.Load(); got != 1 {
				t.Errorf("made %d requests inside the deadline, want exactly 1 — a wrapped delay retries at once", got)
			}
		})
	}
}
