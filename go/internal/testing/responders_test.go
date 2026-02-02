package testing

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestStringResponse(t *testing.T) {
	responder := StringResponse("hello world")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Errorf("expected body 'hello world', got %q", body)
	}
}

func TestStatusStringResponse(t *testing.T) {
	responder := StatusStringResponse(http.StatusCreated, "created")
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}
}

func TestRespondJSON(t *testing.T) {
	data := map[string]interface{}{
		"id":   123,
		"name": "Test Project",
	}
	responder := RespondJSON(data)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["name"] != "Test Project" {
		t.Errorf("expected name 'Test Project', got %v", result["name"])
	}
}

func TestStatusJSON(t *testing.T) {
	data := map[string]string{"message": "created"}
	responder := StatusJSON(http.StatusCreated, data)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

func TestRespondError(t *testing.T) {
	responder := RespondError(http.StatusBadRequest, "Invalid request")
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", resp.StatusCode)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["error"] != "Invalid request" {
		t.Errorf("expected error 'Invalid request', got %q", result["error"])
	}
}

func TestRespondNotFound(t *testing.T) {
	responder := RespondNotFound()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestRespondRateLimit(t *testing.T) {
	responder := RespondRateLimit(30)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", resp.StatusCode)
	}

	if ra := resp.Header.Get("Retry-After"); ra != "30" {
		t.Errorf("expected Retry-After '30', got %q", ra)
	}
}

func TestWithHeader(t *testing.T) {
	responder := WithHeader(StringResponse("body"), "X-Custom", "value123")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if h := resp.Header.Get("X-Custom"); h != "value123" {
		t.Errorf("expected X-Custom 'value123', got %q", h)
	}
}

func TestWithPagination(t *testing.T) {
	responder := WithPagination(RespondJSON([]int{1, 2, 3}), "https://api.example.com/test?page=2")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	link := resp.Header.Get("Link")
	if link != `<https://api.example.com/test?page=2>; rel="next"` {
		t.Errorf("unexpected Link header: %q", link)
	}
}

func TestWithPaginationEmpty(t *testing.T) {
	// Empty nextURL should return the original responder unchanged
	responder := WithPagination(RespondJSON([]int{1, 2, 3}), "")
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if link := resp.Header.Get("Link"); link != "" {
		t.Errorf("expected no Link header, got %q", link)
	}
}

func TestRESTPayload(t *testing.T) {
	var capturedPayload map[string]interface{}

	responder := RESTPayload(http.StatusCreated, `{"id": 1}`, func(payload map[string]interface{}) {
		capturedPayload = payload
	})

	body := `{"name": "Test", "description": "A test project"}`
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "https://api.example.com/test", strings.NewReader(body))

	resp, err := responder(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", resp.StatusCode)
	}

	if capturedPayload["name"] != "Test" {
		t.Errorf("expected captured name 'Test', got %v", capturedPayload["name"])
	}
	if capturedPayload["description"] != "A test project" {
		t.Errorf("expected captured description 'A test project', got %v", capturedPayload["description"])
	}
}

func TestSequence(t *testing.T) {
	responder := Sequence(
		StatusStringResponse(http.StatusOK, "first"),
		StatusStringResponse(http.StatusCreated, "second"),
		StatusStringResponse(http.StatusAccepted, "third"),
	)
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api.example.com/test", nil)

	// First call
	resp, _ := responder(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("first call: expected status 200, got %d", resp.StatusCode)
	}

	// Second call
	resp, _ = responder(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("second call: expected status 201, got %d", resp.StatusCode)
	}

	// Third call
	resp, _ = responder(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("third call: expected status 202, got %d", resp.StatusCode)
	}

	// Fourth call (should repeat last)
	resp, _ = responder(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("fourth call: expected status 202 (last response), got %d", resp.StatusCode)
	}
}

func TestSequence_PanicOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected Sequence to panic with no responders")
		}
	}()

	Sequence()
}
