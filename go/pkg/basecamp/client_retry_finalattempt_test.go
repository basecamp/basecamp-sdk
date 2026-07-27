package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// retryCountingHooks counts OnRetry invocations; all other hook methods are
// inherited as no-ops.
type retryCountingHooks struct {
	NoopHooks
	onRetry int32
}

func (h *retryCountingHooks) OnRetry(context.Context, RequestInfo, int, error) {
	atomic.AddInt32(&h.onRetry, 1)
}

// TestClient_NoRetryAfterFinalAttempt pins that the hand-written GET retry loop
// makes exactly MaxRetries attempts (total-attempt semantics) and does NOT
// sleep or fire OnRetry after the final attempt — OnRetry fires once per
// gap between attempts, i.e. MaxRetries-1 times.
func TestClient_NoRetryAfterFinalAttempt(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // retryable
	}))
	defer server.Close()

	cfg := &Config{BaseURL: server.URL, CacheEnabled: false}
	hooks := &retryCountingHooks{}
	client := NewClient(cfg, &StaticTokenProvider{Token: "test-token"})
	client.httpOpts.MaxRetries = 3
	client.httpOpts.BaseDelay = time.Millisecond
	client.hooks = hooks

	if _, err := client.Get(context.Background(), "/test.json"); err == nil {
		t.Fatal("expected an error after retry exhaustion, got nil")
	}

	if got := atomic.LoadInt32(&requests); got != 3 {
		t.Errorf("made %d requests, want 3 (MaxRetries is a total attempt count)", got)
	}
	// One OnRetry per inter-attempt gap: 3 attempts → 2 retries. A third call
	// would mean a wasted sleep + misleading hook after the final attempt.
	if got := atomic.LoadInt32(&hooks.onRetry); got != 2 {
		t.Errorf("OnRetry fired %d times, want 2 (no retry notification after the final attempt)", got)
	}
}
