package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The hook-facing copy of the header (RequestResult.RetryAfter) is populated
// at every status, as the error's field is (SPEC §6) — a 404 carrying
// Retry-After reaches OnRequestEnd with the value, where the transport used
// to parse it for 429 and 503 alone. The resilience hook's own 429/503 policy
// is a separate decision and is pinned in resilience_test.go.
func TestTransport_RequestResultCarriesRetryAfterAtEveryStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "17")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	hooks := &recordingHooks{}
	client := NewClient(&Config{BaseURL: server.URL, CacheEnabled: false}, &StaticTokenProvider{Token: "test-token"})
	client.hooks = hooks

	// Driven through a generated operation, as a caller would, rather than
	// the raw transport path.
	if _, err := client.ForAccount("12345").Projects().Get(context.Background(), 999999999); err == nil {
		t.Fatal("Projects.Get succeeded, want a 404 error")
	}
	if len(hooks.endCalls) != 1 {
		t.Fatalf("OnRequestEnd fired %d times, want 1", len(hooks.endCalls))
	}
	if got := hooks.endCalls[0].RetryAfter; got != 17 {
		t.Errorf("RequestResult.RetryAfter = %d on a 404, want 17", got)
	}
}
