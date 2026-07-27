package basecamp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// doerFunc adapts a function to generated.HttpRequestDoer so a test can inject
// a fake transport.
type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

// These tests pin the generated client's total-attempt retry semantics for
// idempotent operations: RetryConfig.MaxRetries is a TOTAL attempt count (3
// means 3 attempts, not 4), 0 means a single attempt, and a negative value is
// a configuration error. They drive the generated retry loop (doWithRetry)
// directly through an exported idempotent operation (CompleteTodo, a POST
// flagged idempotent in behavior-model.json), staying outside pkg/generated per
// the repo rule that generated code carries no hand-written tests.

// fastRetryConfig returns a RetryConfig with the given total-attempt count and
// tiny delays so multi-attempt retry tests run quickly.
func fastRetryConfig(maxRetries int) generated.RetryConfig {
	return generated.RetryConfig{
		MaxRetries: maxRetries,
		BaseDelay:  time.Millisecond,
		MaxDelay:   2 * time.Millisecond,
		Multiplier: 2.0,
	}
}

// always503 returns a handler that always responds 503 (a retryable status)
// and counts each request it receives.
func always503(counter *int32) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(counter, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}

func TestGeneratedDoWithRetry_TotalAttempts(t *testing.T) {
	cases := []struct {
		name         string
		maxRetries   int
		wantAttempts int32
	}{
		{"zero means a single attempt", 0, 1},
		{"one means a single attempt", 1, 1},
		{"three means three attempts", 3, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(always503(&attempts))
			defer server.Close()

			client, err := generated.NewClient(server.URL, generated.WithRetryConfig(fastRetryConfig(tc.maxRetries)))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			// Exhaustion returns the last 503 response with a nil error.
			resp, err := client.CompleteTodo(context.Background(), "99999", 100)
			if err != nil {
				t.Fatalf("CompleteTodo returned error: %v", err)
			}
			_ = resp.Body.Close()

			if got := atomic.LoadInt32(&attempts); got != tc.wantAttempts {
				t.Errorf("made %d attempts, want %d (MaxRetries is a total attempt count)", got, tc.wantAttempts)
			}
		})
	}
}

func TestGeneratedRetryConfig_NegativeRejectedAtConstruction(t *testing.T) {
	_, err := generated.NewClient(
		"https://example.com",
		generated.WithRetryConfig(generated.RetryConfig{MaxRetries: -1}),
	)
	if err == nil {
		t.Fatal("expected a configuration error for negative MaxRetries, got nil")
	}
}

func TestGeneratedRetryConfig_NegativeRejectedAfterMutation(t *testing.T) {
	// RetryConfig is a public field, so a caller can set an invalid value after
	// construction. doWithRetry must re-validate and surface a configuration
	// error without making any request.
	var attempts int32
	server := httptest.NewServer(always503(&attempts))
	defer server.Close()

	client, err := generated.NewClient(server.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.RetryConfig.MaxRetries = -1

	resp, err := client.CompleteTodo(context.Background(), "99999", 100)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil {
		t.Fatal("expected a configuration error for negative MaxRetries, got nil")
	}
	if got := atomic.LoadInt32(&attempts); got != 0 {
		t.Errorf("made %d requests with an invalid config, want 0", got)
	}
}

func TestGeneratedDoWithRetry_NoSleepAfterFinalAttempt(t *testing.T) {
	// Deterministic (no wall-clock threshold): a fake transport cancels the
	// context on the final attempt. The correct loop returns the final 503
	// before any sleep, so CompleteTodo returns that response with a nil error.
	// A spurious sleep after the final attempt would observe the cancellation in
	// its select and return context.Canceled instead — which this test rejects.
	const maxRetries = 2
	var attempts int32

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	doer := doerFunc(func(_ *http.Request) (*http.Response, error) {
		if int(atomic.AddInt32(&attempts, 1)) == maxRetries {
			cancel() // cancel during the final attempt only
		}
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	client, err := generated.NewClient(
		"https://example.com",
		generated.WithHTTPClient(doer),
		generated.WithRetryConfig(fastRetryConfig(maxRetries)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := client.CompleteTodo(ctx, "99999", 100)
	if err != nil {
		t.Fatalf("CompleteTodo returned %v — a sleep after the final attempt observed the cancellation", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	if got := atomic.LoadInt32(&attempts); got != maxRetries {
		t.Errorf("made %d attempts, want %d", got, maxRetries)
	}
}
