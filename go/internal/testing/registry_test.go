package testing

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRegistry_RoundTrip_MatchesStub(t *testing.T) {
	reg := NewRegistry(t)

	reg.Register(
		REST("GET", "projects.json"),
		RespondJSON(map[string]string{"name": "Test Project"}),
	)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/projects.json", nil)
	resp, err := reg.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Test Project") {
		t.Errorf("expected body to contain 'Test Project', got %s", body)
	}

	if reg.RequestCount() != 1 {
		t.Errorf("expected 1 request, got %d", reg.RequestCount())
	}

	reg.Verify(t)
}

func TestRegistry_RoundTrip_NoMatchingStub(t *testing.T) {
	reg := NewRegistry(t)

	reg.Register(
		REST("GET", "projects.json"),
		RespondJSON(nil),
	)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/users.json", nil)
	resp, err := reg.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("expected error for unmatched request")
	}
	if !strings.Contains(err.Error(), "no registered HTTP stubs matched") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRegistry_Verify_UnmatchedStubs(t *testing.T) {
	// Use a fake t to capture errors without failing this test
	fakeT := &fakeT{}
	reg := NewRegistry(nil)

	reg.Register(
		REST("GET", "projects.json"),
		RespondJSON(nil),
	)

	reg.Verify(fakeT)

	if !fakeT.failed {
		t.Error("expected Verify to fail for unmatched stub")
	}
	// The format string contains %d for the count
	if !strings.Contains(fakeT.errorMsg, "%d HTTP stubs unmatched") {
		t.Errorf("unexpected error message: %s", fakeT.errorMsg)
	}
}

func TestRegistry_Client(t *testing.T) {
	reg := NewRegistry(t)

	reg.Register(
		REST("GET", "test.json"),
		StatusStringResponse(http.StatusCreated, "created"),
	)

	client := reg.Client()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test.json", nil)
	resp, err := client.Do(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	reg.Verify(t)
}

func TestRegistry_Reset(t *testing.T) {
	reg := NewRegistry(t)

	reg.Register(
		REST("GET", "projects.json"),
		RespondJSON(nil),
	)

	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/projects.json", nil)
	resp, _ := reg.RoundTrip(req)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	if reg.RequestCount() != 1 {
		t.Errorf("expected 1 request before reset, got %d", reg.RequestCount())
	}

	reg.Reset()

	if reg.RequestCount() != 0 {
		t.Errorf("expected 0 requests after reset, got %d", reg.RequestCount())
	}
}

func TestRegistry_LastRequest(t *testing.T) {
	reg := NewRegistry(t)

	reg.Register(MatchAny, RespondJSON(nil))
	reg.Register(MatchAny, RespondJSON(nil))

	req1, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/first", nil)
	resp1, _ := reg.RoundTrip(req1)
	if resp1 != nil && resp1.Body != nil {
		resp1.Body.Close()
	}

	req2, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/second", nil)
	resp2, _ := reg.RoundTrip(req2)
	if resp2 != nil && resp2.Body != nil {
		resp2.Body.Close()
	}

	last := reg.LastRequest()
	if last == nil {
		t.Fatal("expected last request to be non-nil")
	}
	if last.URL.Path != "/second" {
		t.Errorf("expected last request path /second, got %s", last.URL.Path)
	}
}

func TestRegistry_LastRequest_Empty(t *testing.T) {
	reg := NewRegistry(t)

	if reg.LastRequest() != nil {
		t.Error("expected LastRequest to return nil when no requests made")
	}
}

// fakeT is a minimal testing.T implementation for testing Verify behavior
type fakeT struct {
	failed   bool
	errorMsg string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(format string, args ...interface{}) {
	f.failed = true
	f.errorMsg = format
}
