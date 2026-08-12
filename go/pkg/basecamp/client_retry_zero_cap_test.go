package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// A cap of zero means "no retries — exactly one attempt", not "no request"
// (SPEC §2 validation step 4). Until #718 the hand-written client rejected it
// outright, panicking at construction, while every other SDK with a numeric cap
// accepted and floored it.
//
// The cap tests below pin one loop each. They fail against the un-fixed code in
// two distinct ways, which is the point of having all three: the raw GET and
// download loops are pre-check (`attempt <= 0` sends nothing), and the typed
// path never consulted the cap at all. The last test covers the other half of
// what now reaches the generated client — BaseDelay, and its ceiling.

func TestNewClient_AcceptsZeroMaxRetries(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewClient panicked on a zero cap: %v", r)
		}
	}()

	NewClient(&Config{BaseURL: "https://3.basecampapi.com"},
		&StaticTokenProvider{Token: "test-token"}, WithMaxRetries(0))
}

func TestNewClient_RejectsNegativeMaxRetries(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic on a negative cap, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "max retries must not be negative") {
			t.Errorf("unexpected panic value: %v", r)
		}
	}()

	NewClient(&Config{BaseURL: "https://3.basecampapi.com"},
		&StaticTokenProvider{Token: "test-token"}, WithMaxRetries(-1))
}

// The raw GET loop. Its `for attempt = 1; attempt <= cap` is pre-check, so
// without the floor a zero cap makes zero requests and returns the nil lastErr.
func TestClient_ZeroMaxRetriesMakesExactlyOneGetRequest(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // retryable — a retry would show
	}))
	defer server.Close()

	hooks := &retryCountingHooks{}
	client := NewClient(&Config{BaseURL: server.URL, CacheEnabled: false},
		&StaticTokenProvider{Token: "test-token"},
		WithMaxRetries(0), WithBaseDelay(time.Millisecond))
	client.hooks = hooks

	if _, err := client.Get(context.Background(), "/test.json"); err == nil {
		t.Fatal("expected the 503 to surface as an error, got nil")
	}

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("made %d requests, want exactly 1 (a zero cap is one attempt, not none)", got)
	}
	if got := atomic.LoadInt32(&hooks.onRetry); got != 0 {
		t.Errorf("OnRetry fired %d times, want 0 (there is no gap after a single attempt)", got)
	}
}

// The generated client's own retry loop, behind every typed operation. Until
// #718 initGeneratedClient never passed the caller's settings, so this made
// three attempts on DefaultRetryConfig no matter what the cap said — the defect
// the cap change alone would not have found.
func TestClient_ZeroMaxRetriesReachesTypedOperations(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(&Config{BaseURL: server.URL, CacheEnabled: false},
		&StaticTokenProvider{Token: "test-token"},
		WithMaxRetries(0), WithBaseDelay(time.Millisecond))

	if _, err := client.ForAccount("999").Projects().Get(context.Background(), 1); err == nil {
		t.Fatal("expected the 503 to surface as an error, got nil")
	}

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("typed GetProject made %d requests, want exactly 1", got)
	}
}

// The same plumbing at a non-zero, non-default cap: a caller who lowers the cap
// to fail fast must be honored on the typed path too, not silently given the
// generated default of 3.
func TestClient_LoweredMaxRetriesReachesTypedOperations(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(&Config{BaseURL: server.URL, CacheEnabled: false},
		&StaticTokenProvider{Token: "test-token"},
		WithMaxRetries(2), WithBaseDelay(time.Millisecond))

	if _, err := client.ForAccount("999").Projects().Get(context.Background(), 1); err == nil {
		t.Fatal("expected the 503 to surface as an error, got nil")
	}

	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Errorf("typed GetProject made %d requests, want 2 (the configured cap, not the generated default of 3)", got)
	}
}

// BaseDelay reaches the generated client, but never above the §7 backoff
// ceiling. The generated loop uses RetryConfig.BaseDelay verbatim for its FIRST
// sleep and applies MaxDelay only after multiplying for the next one, so an
// unclamped value would stall a typed operation for its full length — where
// doRequestURL's own loop saturates every computed delay at MaxBackoffDelay.
//
// Asserted on the config rather than by timing a request: the point is the
// value handed to the generated client, and sleeping 30s to observe it would be
// a wall-clock test of exactly the kind #715 removed.
func TestInitGeneratedClient_ClampsBaseDelayToTheBackoffCeiling(t *testing.T) {
	for _, tc := range []struct {
		name string
		base time.Duration
		want time.Duration
	}{
		{"above the ceiling is clamped", 10 * time.Minute, MaxBackoffDelay},
		{"at the ceiling is kept", MaxBackoffDelay, MaxBackoffDelay},
		{"below the ceiling is kept", 250 * time.Millisecond, 250 * time.Millisecond},
		// A caller asking for no backoff must get none on the typed path too;
		// leaving it at the generated 1s default is the split this closes.
		{"zero carries over rather than reverting to the generated default", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(&Config{BaseURL: "https://3.basecampapi.com"},
				&StaticTokenProvider{Token: "test-token"}, WithBaseDelay(tc.base))
			client.initGeneratedClient()

			if got := client.gen.ClientInterface.(*generated.Client).RetryConfig.BaseDelay; got != tc.want {
				t.Errorf("generated RetryConfig.BaseDelay = %v, want %v", got, tc.want)
			}
		})
	}
}

// The download loop, which is ungoverned traffic: DownloadURL carries no entry
// in behavior-model.json, so no per-operation ceiling applies and the cap is
// the whole budget. Its loop is pre-check like the raw GET one.
func TestDownloadURL_ZeroMaxRetriesMakesExactlyOneRequest(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // in the hop-1 retry set
	}))
	defer server.Close()

	client := NewClient(&Config{BaseURL: server.URL, CacheEnabled: false},
		&StaticTokenProvider{Token: "test-token"},
		WithMaxRetries(0), WithBaseDelay(time.Millisecond))

	_, err := client.ForAccount("999").DownloadURL(context.Background(), server.URL+"/files/1/download/a.png")
	if err == nil {
		t.Fatal("expected the 503 to surface as an error, got nil")
	}
	// Specifically NOT the "retry loop made no attempt" usage error: that would
	// mean the loop was skipped rather than run once.
	if strings.Contains(err.Error(), "made no attempt") {
		t.Errorf("download loop was skipped entirely: %v", err)
	}

	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Errorf("made %d download requests, want exactly 1", got)
	}
}
