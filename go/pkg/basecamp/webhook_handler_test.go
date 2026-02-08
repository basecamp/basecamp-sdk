package basecamp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookReceiver_ExactKindRouting(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1, // disable dedup for this test
	})

	var received []string
	receiver.On("todo_created", func(event *WebhookEvent) error {
		received = append(received, event.Kind)
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 handler call, got %d", len(received))
	}
	if received[0] != "todo_created" {
		t.Errorf("expected kind 'todo_created', got %q", received[0])
	}
}

func TestWebhookReceiver_GlobPrefixPattern(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	var received []string
	receiver.On("todo_*", func(event *WebhookEvent) error {
		received = append(received, event.Kind)
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 handler call, got %d", len(received))
	}
}

func TestWebhookReceiver_GlobSuffixPattern(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	var received []string
	receiver.On("*_created", func(event *WebhookEvent) error {
		received = append(received, event.Kind)
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 handler call, got %d", len(received))
	}
}

func TestWebhookReceiver_GlobDoesNotMatchWrongKind(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	var received []string
	receiver.On("message_*", func(event *WebhookEvent) error {
		received = append(received, event.Kind)
		return nil
	})

	// This event is todo_created, should not match message_*
	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 0 {
		t.Errorf("expected 0 handler calls, got %d", len(received))
	}
}

func TestWebhookReceiver_OnAnyHandler(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	var received []string
	receiver.OnAny(func(event *WebhookEvent) error {
		received = append(received, event.Kind)
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 handler call, got %d", len(received))
	}
	if received[0] != "todo_created" {
		t.Errorf("expected kind 'todo_created', got %q", received[0])
	}
}

func TestWebhookReceiver_OnAnyWithSpecific(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	var all []string
	var specific []string

	receiver.OnAny(func(event *WebhookEvent) error {
		all = append(all, event.Kind)
		return nil
	})
	receiver.On("todo_created", func(event *WebhookEvent) error {
		specific = append(specific, event.Kind)
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(all) != 1 {
		t.Errorf("expected 1 OnAny call, got %d", len(all))
	}
	if len(specific) != 1 {
		t.Errorf("expected 1 specific call, got %d", len(specific))
	}
}

func TestWebhookReceiver_UnknownKindNoError(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	// Register handlers that won't match the unknown event kind
	receiver.On("todo_*", func(event *WebhookEvent) error {
		t.Error("unexpected handler call for todo_*")
		return nil
	})

	data := loadWebhooksFixture(t, "event-unknown-future.json")
	event, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.Kind != "new_thing_activated" {
		t.Errorf("expected kind 'new_thing_activated', got %q", event.Kind)
	}
}

func TestWebhookReceiver_UnknownKindWithCatchAll(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	var received []string
	receiver.OnAny(func(event *WebhookEvent) error {
		received = append(received, event.Kind)
		return nil
	})

	data := loadWebhooksFixture(t, "event-unknown-future.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 handler call, got %d", len(received))
	}
	if received[0] != "new_thing_activated" {
		t.Errorf("expected kind 'new_thing_activated', got %q", received[0])
	}
}

func TestWebhookReceiver_Dedup(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: 100,
	})

	callCount := 0
	receiver.OnAny(func(event *WebhookEvent) error {
		callCount++
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")

	// First delivery
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error on first delivery: %v", err)
	}

	// Second delivery (duplicate)
	event, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error on second delivery: %v", err)
	}

	// Event should still be returned even though it's a duplicate
	if event == nil {
		t.Fatal("expected non-nil event for duplicate")
	}

	// But handler should only fire once
	if callCount != 1 {
		t.Errorf("expected handler to be called once, got %d", callCount)
	}
}

func TestWebhookReceiver_DedupWindowEviction(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: 2, // Tiny window
	})

	callCount := 0
	receiver.OnAny(func(event *WebhookEvent) error {
		callCount++
		return nil
	})

	// Send 3 different events to fill and overflow the window
	events := []string{
		`{"id":1,"kind":"a","details":{},"created_at":"2022-01-01T00:00:00Z","recording":{"id":1},"creator":{"id":1}}`,
		`{"id":2,"kind":"b","details":{},"created_at":"2022-01-01T00:00:00Z","recording":{"id":2},"creator":{"id":1}}`,
		`{"id":3,"kind":"c","details":{},"created_at":"2022-01-01T00:00:00Z","recording":{"id":3},"creator":{"id":1}}`,
	}

	for _, e := range events {
		_, err := receiver.HandleRequest([]byte(e), noHeaders)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if callCount != 3 {
		t.Fatalf("expected 3 handler calls, got %d", callCount)
	}

	// Event ID 1 should have been evicted, so re-sending it should trigger the handler
	_, err := receiver.HandleRequest([]byte(events[0]), noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if callCount != 4 {
		t.Errorf("expected 4 handler calls (evicted ID re-delivered), got %d", callCount)
	}

	// But event ID 2 should still be in the window (ID 3 and ID 1-again are the two recent)
	// Actually after eviction: window has [2, 3], then we add 1 which evicts 2 -> [3, 1]
	// So ID 2 should now trigger
	_, err = receiver.HandleRequest([]byte(events[1]), noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 5 {
		t.Errorf("expected 5 handler calls, got %d", callCount)
	}
}

func TestWebhookReceiver_Middleware(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	var order []string

	receiver.Use(func(event *WebhookEvent, next func() error) error {
		order = append(order, "mw1-before")
		err := next()
		order = append(order, "mw1-after")
		return err
	})

	receiver.Use(func(event *WebhookEvent, next func() error) error {
		order = append(order, "mw2-before")
		err := next()
		order = append(order, "mw2-after")
		return err
	})

	receiver.OnAny(func(event *WebhookEvent) error {
		order = append(order, "handler")
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}

func TestWebhookReceiver_MiddlewareCanShortCircuit(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	shortCircuitErr := errors.New("blocked by middleware")

	receiver.Use(func(event *WebhookEvent, next func() error) error {
		return shortCircuitErr
	})

	handlerCalled := false
	receiver.OnAny(func(event *WebhookEvent) error {
		handlerCalled = true
		return nil
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if !errors.Is(err, shortCircuitErr) {
		t.Errorf("expected short-circuit error, got %v", err)
	}
	if handlerCalled {
		t.Error("expected handler not to be called")
	}
}

func TestWebhookReceiver_HandlerError(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	handlerErr := errors.New("handler failed")
	receiver.On("todo_created", func(event *WebhookEvent) error {
		return handlerErr
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	_, err := receiver.HandleRequest(data, noHeaders)
	if !errors.Is(err, handlerErr) {
		t.Errorf("expected handler error, got %v", err)
	}
}

func TestWebhookReceiver_SignatureVerification(t *testing.T) {
	secret := "test-secret"
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		Secret:          secret,
		DedupWindowSize: -1,
	})

	receiver.OnAny(func(event *WebhookEvent) error { return nil })

	data := loadWebhooksFixture(t, "event-todo-created.json")
	sig := ComputeWebhookSignature(data, secret)

	// Valid signature
	_, err := receiver.HandleRequest(data, func(key string) string {
		if key == "X-Basecamp-Signature" {
			return sig
		}
		return ""
	})
	if err != nil {
		t.Fatalf("expected no error with valid signature, got %v", err)
	}

	// Invalid signature
	_, err = receiver.HandleRequest(data, func(key string) string {
		if key == "X-Basecamp-Signature" {
			return "bad-signature"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected error with invalid signature")
	}
	var verErr *WebhookVerificationError
	if !errors.As(err, &verErr) {
		t.Errorf("expected WebhookVerificationError, got %T: %v", err, err)
	}

	// Missing signature
	_, err = receiver.HandleRequest(data, noHeaders)
	if err == nil {
		t.Fatal("expected error with missing signature")
	}
	if !errors.As(err, &verErr) {
		t.Errorf("expected WebhookVerificationError, got %T: %v", err, err)
	}
}

func TestWebhookReceiver_ServeHTTP_ValidPost(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	receiver.OnAny(func(event *WebhookEvent) error { return nil })

	data := loadWebhooksFixture(t, "event-todo-created.json")
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	receiver.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestWebhookReceiver_ServeHTTP_MethodNotAllowed(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{})

	req := httptest.NewRequest(http.MethodGet, "/webhooks", nil)
	rr := httptest.NewRecorder()
	receiver.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestWebhookReceiver_ServeHTTP_UnauthorizedBadSig(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		Secret:          "test-secret",
		DedupWindowSize: -1,
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Basecamp-Signature", "wrong-sig")

	rr := httptest.NewRecorder()
	receiver.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestWebhookReceiver_ServeHTTP_InternalServerError(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	receiver.OnAny(func(event *WebhookEvent) error {
		return errors.New("handler failed")
	})

	data := loadWebhooksFixture(t, "event-todo-created.json")
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	receiver.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestWebhookReceiver_InvalidJSON(t *testing.T) {
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		DedupWindowSize: -1,
	})

	_, err := receiver.HandleRequest([]byte(`not json`), noHeaders)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		// It should be a wrapped parse error
		if !strings.Contains(err.Error(), "failed to parse webhook event") {
			t.Errorf("expected parse error, got %v", err)
		}
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		// Exact match
		{"todo_created", "todo_created", true},
		{"todo_created", "todo_completed", false},

		// Prefix glob
		{"todo_*", "todo_created", true},
		{"todo_*", "todo_completed", true},
		{"todo_*", "message_created", false},

		// Suffix glob
		{"*_created", "todo_created", true},
		{"*_created", "message_created", true},
		{"*_created", "todo_completed", false},

		// Middle glob
		{"todo_*_done", "todo_something_done", true},
		{"todo_*_done", "todo_done", false},

		// Double glob
		{"*_*", "todo_created", true},
		{"*_*", "a_b", true},

		// Wildcard matches everything
		{"*", "todo_created", true},
		{"*", "anything", true},

		// No wildcards, no match
		{"todo", "todo_created", false},
		{"todo_created", "todo", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.value, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestWebhookReceiver_CustomSignatureHeader(t *testing.T) {
	secret := "test-secret"
	receiver := NewWebhookReceiver(WebhookReceiverConfig{
		Secret:          secret,
		SignatureHeader: "X-Custom-Signature",
		DedupWindowSize: -1,
	})

	receiver.OnAny(func(event *WebhookEvent) error { return nil })

	data := loadWebhooksFixture(t, "event-todo-created.json")
	sig := ComputeWebhookSignature(data, secret)

	// Using custom header
	_, err := receiver.HandleRequest(data, func(key string) string {
		if key == "X-Custom-Signature" {
			return sig
		}
		return ""
	})
	if err != nil {
		t.Fatalf("expected no error with valid signature on custom header, got %v", err)
	}
}

// noHeaders is a header getter that always returns empty string.
func noHeaders(string) string { return "" }
