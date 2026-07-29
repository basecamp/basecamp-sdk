package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// These tests pin Gate 3's STATUS SET, the half the per-operation ceiling work
// did not cover. behavior-model.json declares retry_on: [429, 503] for all 226
// operations; the generated client previously used a global
// {429, 500, 502, 503, 504} allowlist, so it retried three statuses the spec
// never declared retryable. The declared set is now emitted as
// operationRetryOn and consulted by operationId.
//
// ParseHTTPError still classifies 500/502/503/504 as retryable errors for the
// caller's benefit. That is a caller-facing hint, and these tests exist partly
// to pin that it does NOT widen the transport's gate.
//
// They stay outside pkg/generated per the repo rule that generated code carries
// no hand-written tests.

func statusRetryConfig(maxRetries int) generated.RetryConfig {
	return generated.RetryConfig{
		MaxRetries: maxRetries,
		BaseDelay:  time.Millisecond,
		MaxDelay:   2 * time.Millisecond,
		Multiplier: 2.0,
	}
}

func countingStatusHandler(status int, counter *int32) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(counter, 1)
		w.WriteHeader(status)
	}
}

func TestGeneratedRetry_HonorsDeclaredStatusSet(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		wantAttempts int32
	}{
		{"429 is declared retryable", http.StatusTooManyRequests, 3},
		{"503 is declared retryable", http.StatusServiceUnavailable, 3},
		{"500 is not declared retryable", http.StatusInternalServerError, 1},
		{"502 is not declared retryable", http.StatusBadGateway, 1},
		{"504 is not declared retryable", http.StatusGatewayTimeout, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(countingStatusHandler(tc.status, &attempts))
			defer server.Close()

			client, err := generated.NewClient(server.URL, generated.WithRetryConfig(statusRetryConfig(3)))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			// CompleteTodo is an idempotent POST with maxAttempts 3, so the only
			// thing varying across these cases is the status.
			resp, err := client.CompleteTodo(context.Background(), "99999", 100)
			if err != nil {
				t.Fatalf("CompleteTodo returned error: %v", err)
			}
			if resp.StatusCode != tc.status {
				t.Errorf("got status %d, want %d", resp.StatusCode, tc.status)
			}
			_ = resp.Body.Close()

			if got := atomic.LoadInt32(&attempts); got != tc.wantAttempts {
				t.Errorf("made %d attempts for status %d, want %d", got, tc.status, tc.wantAttempts)
			}
		})
	}
}

// The read path must be gated too: a GET is retry-eligible by method, so
// without the status gate a 500 would retry to exhaustion.
func TestGeneratedRetry_ReadPathHonorsDeclaredStatusSet(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(countingStatusHandler(http.StatusInternalServerError, &attempts))
	defer server.Close()

	client, err := generated.NewClient(server.URL, generated.WithRetryConfig(statusRetryConfig(3)))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := client.GetAccount(context.Background(), "99999")
	if err != nil {
		t.Fatalf("GetAccount returned error: %v", err)
	}
	_ = resp.Body.Close()

	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("made %d attempts on a 500, want 1 (500 is not in the declared retry_on set)", got)
	}
}

// The generated table must carry the declared set verbatim. This is the raw
// metadata half; the behavioral half is above.
func TestGeneratedRetryOn_MatchesDeclaredSet(t *testing.T) {
	for _, operation := range []string{"CompleteTodo", "GetAccount", "UpdateGaugeNeedle"} {
		retryOn, ok := generated.GetOperationRetryOn(operation)
		if !ok {
			t.Errorf("%s missing from operationRetryOn", operation)
			continue
		}
		if len(retryOn) != 2 || retryOn[0] != 429 || retryOn[1] != 503 {
			t.Errorf("%s retryOn = %v, want [429 503]", operation, retryOn)
		}
	}
}

// An operation's declared set is authoritative in both directions. A
// present-but-EMPTY retryOn means "never retry on any status"; only an absent
// entry falls back to the default. isRetryableStatus is unexported, so this
// exercises the exported accessor plus the documented fallback rule.
func TestGeneratedRetryOn_EmptySetIsNotAbsent(t *testing.T) {
	// Every generated operation carries a set today, so the fallback is only
	// reachable for an unknown id. Pin that it IS the default, so the two cases
	// stay distinguishable if an operation ever declares retryOn: [].
	if _, ok := generated.GetOperationRetryOn("NoSuchOperation"); ok {
		t.Fatal("an unknown operation must not be present in operationRetryOn")
	}

	// And pin that no shipped operation declares an empty set, which is what
	// makes the fallback unreachable from a generated call site today.
	for _, operation := range []string{"CompleteTodo", "GetAccount", "UpdateGaugeNeedle"} {
		retryOn, ok := generated.GetOperationRetryOn(operation)
		if !ok {
			t.Fatalf("%s missing from operationRetryOn", operation)
		}
		if len(retryOn) == 0 {
			t.Errorf("%s declares an empty retryOn; it would never retry on any status", operation)
		}
	}
}

// The accessor must hand out a COPY. It reads a package-level map that
// isRetryableStatus consults on every response, so returning the stored slice
// lets any caller rewrite the retry policy for every client in the process —
// and race with in-flight requests while doing it. Mutating what the accessor
// returned must not be observable through a second lookup, nor change behavior.
func TestGeneratedRetryOn_ReturnsCopy(t *testing.T) {
	first, ok := generated.GetOperationRetryOn("GetAccount")
	if !ok {
		t.Fatal("GetAccount missing from operationRetryOn")
	}
	if len(first) != 2 {
		t.Fatalf("GetAccount retryOn = %v, want 2 statuses", first)
	}
	first[0] = 599
	first[1] = 599

	second, ok := generated.GetOperationRetryOn("GetAccount")
	if !ok {
		t.Fatal("GetAccount missing from operationRetryOn after mutation")
	}
	if second[0] != 429 || second[1] != 503 {
		t.Fatalf("accessor leaked the shared slice: after mutating the first result, "+
			"a second lookup returns %v, want [429 503]", second)
	}

	// The behavioral half: the poisoned set must not have disabled retries.
	// 429 is declared retryable for GetAccount, so the client must still retry.
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client, err := generated.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.GetAccount(ctx, "1")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	_ = resp.Body.Close()

	if got := atomic.LoadInt32(&attempts); got < 2 {
		t.Errorf("made %d attempts on a 429 after a caller mutated the accessor result, "+
			"want the declared retry policy to be intact (>=2)", got)
	}
}
