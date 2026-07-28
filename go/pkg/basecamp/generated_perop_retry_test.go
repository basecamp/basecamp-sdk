package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// These tests pin the generated client's per-operation retry ceiling. The
// x-basecamp-retry.maxAttempts value for each operation is emitted into the
// operationRetryMax map (read via GetOperationRetryMax), and doWithRetry applies
// it as a ceiling (upper bound on attempts): effective attempts =
// min(client cap, op retry max). This
// matches how TS/Swift/
// Kotlin drive their retry loops directly from the per-op max, while still
// honoring a Go client that lowered its cap. DisableOutOfOffice carries
// RetryMax:2 (an idempotent account write); CompleteTodo carries RetryMax:3.
// They stay outside pkg/generated per the repo rule.

func TestGeneratedPerOpRetryMax_CeilingBindsBelowClientCap(t *testing.T) {
	cases := []struct {
		name         string
		operation    string
		clientCap    int
		wantAttempts int32
	}{
		// DisableOutOfOffice has RetryMax:2. With the default cap (3) the op
		// ceiling binds: min(3, 2) == 2, not the client-wide 3.
		{"op ceiling below default cap", "DisableOutOfOffice", 3, 2},
		// Raising the client cap cannot exceed the op ceiling: min(5, 2) == 2.
		{"op ceiling below raised cap", "DisableOutOfOffice", 5, 2},
		// Lowering the client cap below the op ceiling is honored: min(1, 2) == 1.
		{"client cap below op ceiling", "DisableOutOfOffice", 1, 1},
		// CompleteTodo has RetryMax:3, equal to the default cap: min(3, 3) == 3
		// (control — unchanged behavior).
		{"op ceiling equals cap", "CompleteTodo", 3, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(always503(&attempts))
			defer server.Close()

			client, err := generated.NewClient(server.URL, generated.WithRetryConfig(fastRetryConfig(tc.clientCap)))
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}

			resp := mustCallIdempotent(t, client, tc.operation)
			if resp != nil {
				_ = resp.Body.Close()
			}

			if got := atomic.LoadInt32(&attempts); got != tc.wantAttempts {
				t.Errorf("%s with client cap %d made %d attempts, want %d (effective = min(cap, RetryMax))",
					tc.operation, tc.clientCap, got, tc.wantAttempts)
			}
		})
	}
}

// TestGeneratedPerOpRetryMax_MetadataMatchesModel is a lightweight guard that
// the emitted per-op retry max is present and positive for the ops under test,
// so the ceiling test above is exercising real per-op data rather than a zero
// default.
func TestGeneratedPerOpRetryMax_MetadataPresent(t *testing.T) {
	for op, want := range map[string]int{"DisableOutOfOffice": 2, "CompleteTodo": 3} {
		got, ok := generated.GetOperationRetryMax(op)
		if !ok {
			t.Fatalf("no retry max for %s", op)
		}
		if got != want {
			t.Errorf("%s retry max = %d, want %d", op, got, want)
		}
	}
}

// mustCallIdempotent invokes an idempotent no-body operation by name so the
// ceiling test can parameterize over operations.
func mustCallIdempotent(t *testing.T, client *generated.Client, operation string) *http.Response {
	t.Helper()
	switch operation {
	case "DisableOutOfOffice":
		r, err := client.DisableOutOfOffice(context.Background(), "99999", 100)
		if err != nil {
			t.Fatalf("DisableOutOfOffice: %v", err)
		}
		return r
	case "CompleteTodo":
		r, err := client.CompleteTodo(context.Background(), "99999", 100)
		if err != nil {
			t.Fatalf("CompleteTodo: %v", err)
		}
		return r
	default:
		t.Fatalf("unknown operation %q", operation)
		return nil
	}
}
