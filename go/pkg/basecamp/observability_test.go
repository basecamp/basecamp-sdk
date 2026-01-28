package basecamp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNoopHooks(t *testing.T) {
	var hooks Hooks = NoopHooks{}

	// Should not panic and should return same context
	ctx := context.Background()
	info := RequestInfo{Method: "GET", URL: "https://example.com", Attempt: 1}
	op := OperationInfo{Service: "Todos", Operation: "List", ResourceType: "todo"}

	// Test operation-level hooks
	opCtx := hooks.OnOperationStart(ctx, op)
	if opCtx != ctx {
		t.Error("NoopHooks.OnOperationStart should return the same context")
	}
	hooks.OnOperationEnd(ctx, op, nil, time.Second)

	// Test request-level hooks
	returnedCtx := hooks.OnRequestStart(ctx, info)
	if returnedCtx != ctx {
		t.Error("NoopHooks.OnRequestStart should return the same context")
	}

	// Should not panic
	hooks.OnRequestEnd(ctx, info, RequestResult{StatusCode: 200})
	hooks.OnRetry(ctx, info, 2, nil)
}

func TestChainHooks_Empty(t *testing.T) {
	// Empty chain should return NoopHooks
	hooks := NewChainHooks()
	if _, ok := hooks.(NoopHooks); !ok {
		t.Error("NewChainHooks with no hooks should return NoopHooks")
	}

	// Chain with only nil hooks should return NoopHooks
	hooks = NewChainHooks(nil, nil)
	if _, ok := hooks.(NoopHooks); !ok {
		t.Error("NewChainHooks with only nil hooks should return NoopHooks")
	}

	// Chain with only NoopHooks should return NoopHooks
	hooks = NewChainHooks(NoopHooks{}, NoopHooks{})
	if _, ok := hooks.(NoopHooks); !ok {
		t.Error("NewChainHooks with only NoopHooks should return NoopHooks")
	}
}

func TestChainHooks_Single(t *testing.T) {
	recorder := &recordingHooks{}
	hooks := NewChainHooks(recorder)

	// Single hook should return that hook directly
	if hooks != recorder {
		t.Error("NewChainHooks with single hook should return that hook directly")
	}
}

func TestChainHooks_Order(t *testing.T) {
	recorder1 := &recordingHooks{id: "1"}
	recorder2 := &recordingHooks{id: "2"}
	recorder3 := &recordingHooks{id: "3"}

	hooks := NewChainHooks(recorder1, recorder2, recorder3)

	ctx := context.Background()
	info := RequestInfo{Method: "GET", URL: "https://example.com", Attempt: 1}

	// OnRequestStart should be called in order
	hooks.OnRequestStart(ctx, info)

	if len(recorder1.startCalls) != 1 || len(recorder2.startCalls) != 1 || len(recorder3.startCalls) != 1 {
		t.Error("OnRequestStart should be called on all hooks")
	}

	// OnRequestEnd should be called in reverse order
	hooks.OnRequestEnd(ctx, info, RequestResult{StatusCode: 200})

	// Check that all hooks were called
	if len(recorder1.endCalls) != 1 || len(recorder2.endCalls) != 1 || len(recorder3.endCalls) != 1 {
		t.Error("OnRequestEnd should be called on all hooks")
	}
}

func TestChainHooks_ContextPropagation(t *testing.T) {
	type keyType string
	key1 := keyType("key1")
	key2 := keyType("key2")

	hook1 := &contextAddingHooks{key: key1, value: "value1"}
	hook2 := &contextAddingHooks{key: key2, value: "value2"}

	hooks := NewChainHooks(hook1, hook2)

	ctx := context.Background()
	info := RequestInfo{Method: "GET", URL: "https://example.com", Attempt: 1}

	resultCtx := hooks.OnRequestStart(ctx, info)

	// Both values should be in the context
	if resultCtx.Value(key1) != "value1" {
		t.Error("Context should contain value from first hook")
	}
	if resultCtx.Value(key2) != "value2" {
		t.Error("Context should contain value from second hook")
	}
}

func TestChainHooks_OnRetry(t *testing.T) {
	recorder1 := &recordingHooks{id: "1"}
	recorder2 := &recordingHooks{id: "2"}

	hooks := NewChainHooks(recorder1, recorder2)

	ctx := context.Background()
	info := RequestInfo{Method: "GET", URL: "https://example.com", Attempt: 1}

	hooks.OnRetry(ctx, info, 2, nil)

	if len(recorder1.retryCalls) != 1 || len(recorder2.retryCalls) != 1 {
		t.Error("OnRetry should be called on all hooks")
	}
}

func TestChainHooks_OperationLevel(t *testing.T) {
	recorder1 := &recordingHooks{id: "1"}
	recorder2 := &recordingHooks{id: "2"}
	recorder3 := &recordingHooks{id: "3"}

	hooks := NewChainHooks(recorder1, recorder2, recorder3)

	ctx := context.Background()
	op := OperationInfo{
		Service:      "Todos",
		Operation:    "Complete",
		ResourceType: "todo",
		IsMutation:   true,
		BucketID:     123,
		ResourceID:   456,
	}

	// OnOperationStart should be called in order
	hooks.OnOperationStart(ctx, op)

	if len(recorder1.opStartCalls) != 1 || len(recorder2.opStartCalls) != 1 || len(recorder3.opStartCalls) != 1 {
		t.Error("OnOperationStart should be called on all hooks")
	}

	// OnOperationEnd should be called in reverse order
	hooks.OnOperationEnd(ctx, op, nil, time.Second)

	if len(recorder1.opEndCalls) != 1 || len(recorder2.opEndCalls) != 1 || len(recorder3.opEndCalls) != 1 {
		t.Error("OnOperationEnd should be called on all hooks")
	}
}

func TestChainHooks_OnOperationGate(t *testing.T) {
	ctx := context.Background()
	op := OperationInfo{
		Service:      "Todos",
		Operation:    "List",
		ResourceType: "todo",
	}

	t.Run("no gating hooks", func(t *testing.T) {
		// Chain with regular hooks (non-gating) should pass through
		recorder := &recordingHooks{id: "1"}
		hooks := NewChainHooks(recorder)

		chain, ok := hooks.(*ChainHooks)
		if !ok {
			// Single hook, test that we can cast and call OnOperationGate
			// For single hooks, we need to wrap in chain to test
			chain = &ChainHooks{hooks: []Hooks{recorder}}
		}

		resultCtx, err := chain.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("OnOperationGate should return nil for non-gating hooks: %v", err)
		}
		if resultCtx != ctx {
			t.Error("OnOperationGate should return original context when no gater")
		}
	})

	t.Run("gating hook allows", func(t *testing.T) {
		gater := &gatingHooks{allowAll: true}
		hooks := NewChainHooks(gater)

		chain, ok := hooks.(*ChainHooks)
		if !ok {
			chain = &ChainHooks{hooks: []Hooks{gater}}
		}

		_, err := chain.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("OnOperationGate should return nil when gater allows: %v", err)
		}
		if gater.gateCalls != 1 {
			t.Errorf("OnOperationGate should call gater once: got %d", gater.gateCalls)
		}
	})

	t.Run("gating hook rejects", func(t *testing.T) {
		gater := &gatingHooks{allowAll: false, rejectErr: ErrCircuitOpen}
		hooks := NewChainHooks(gater)

		chain, ok := hooks.(*ChainHooks)
		if !ok {
			chain = &ChainHooks{hooks: []Hooks{gater}}
		}

		_, err := chain.OnOperationGate(ctx, op)
		if err != ErrCircuitOpen {
			t.Errorf("OnOperationGate should return ErrCircuitOpen: got %v", err)
		}
	})

	t.Run("mixed hooks with gater first rejecting", func(t *testing.T) {
		gater := &gatingHooks{allowAll: false, rejectErr: ErrBulkheadFull}
		recorder := &recordingHooks{id: "1"}

		chain := &ChainHooks{hooks: []Hooks{gater, recorder}}

		_, err := chain.OnOperationGate(ctx, op)
		if err != ErrBulkheadFull {
			t.Errorf("OnOperationGate should return ErrBulkheadFull: got %v", err)
		}
	})

	t.Run("only first gater is called", func(t *testing.T) {
		// ChainHooks should only call the FIRST gater, not all of them
		gater1 := &gatingHooks{allowAll: true}
		gater2 := &gatingHooks{allowAll: true}

		chain := &ChainHooks{hooks: []Hooks{gater1, gater2}}

		_, err := chain.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("OnOperationGate should succeed: %v", err)
		}
		if gater1.gateCalls != 1 {
			t.Errorf("First gater should be called: got %d", gater1.gateCalls)
		}
		if gater2.gateCalls != 0 {
			t.Errorf("Second gater should NOT be called (only first gater): got %d", gater2.gateCalls)
		}
	})

	t.Run("gater returns modified context", func(t *testing.T) {
		type ctxKey string
		key := ctxKey("test-key")
		gater := &gatingHooks{allowAll: true, ctxKey: key, ctxValue: "test-value"}

		chain := &ChainHooks{hooks: []Hooks{gater}}

		resultCtx, err := chain.OnOperationGate(ctx, op)
		if err != nil {
			t.Errorf("OnOperationGate should succeed: %v", err)
		}
		if resultCtx.Value(key) != "test-value" {
			t.Errorf("OnOperationGate should return context with gater's value")
		}
	})
}

// gatingHooks is a test implementation of GatingHooks
type gatingHooks struct {
	NoopHooks
	allowAll  bool
	rejectErr error
	gateCalls int
	ctxKey    any
	ctxValue  any
}

var _ GatingHooks = (*gatingHooks)(nil)

func (h *gatingHooks) OnOperationGate(ctx context.Context, op OperationInfo) (context.Context, error) {
	h.gateCalls++
	if !h.allowAll {
		return ctx, h.rejectErr
	}
	// Add value to context if configured
	if h.ctxKey != nil {
		ctx = context.WithValue(ctx, h.ctxKey, h.ctxValue)
	}
	return ctx, nil
}

// recordingHooks records all hook calls for testing
type recordingHooks struct {
	id           string
	startCalls   []RequestInfo
	endCalls     []RequestResult
	retryCalls   []int
	opStartCalls []OperationInfo
	opEndCalls   []OperationInfo
}

func (h *recordingHooks) OnOperationStart(ctx context.Context, op OperationInfo) context.Context {
	h.opStartCalls = append(h.opStartCalls, op)
	return ctx
}

func (h *recordingHooks) OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration) {
	h.opEndCalls = append(h.opEndCalls, op)
}

func (h *recordingHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	h.startCalls = append(h.startCalls, info)
	return ctx
}

func (h *recordingHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
	h.endCalls = append(h.endCalls, result)
}

func (h *recordingHooks) OnRetry(ctx context.Context, info RequestInfo, attempt int, err error) {
	h.retryCalls = append(h.retryCalls, attempt)
}

// contextAddingHooks adds a value to context in OnRequestStart
type contextAddingHooks struct {
	key   any
	value any
}

func (h *contextAddingHooks) OnOperationStart(ctx context.Context, op OperationInfo) context.Context {
	return ctx
}

func (h *contextAddingHooks) OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration) {
}

func (h *contextAddingHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, h.key, h.value)
}

func (h *contextAddingHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
}

func (h *contextAddingHooks) OnRetry(ctx context.Context, info RequestInfo, attempt int, err error) {}

// contextCapturingHooks captures the context passed to OnRequestEnd
// to verify hook context propagation
type contextCapturingHooks struct {
	NoopHooks
	key          any
	value        any
	capturedCtx  context.Context
	endCallCount int
}

func (h *contextCapturingHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	return context.WithValue(ctx, h.key, h.value)
}

func (h *contextCapturingHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
	h.capturedCtx = ctx
	h.endCallCount++
}

// contextCheckingTransport is a transport that verifies context values are present.
type contextCheckingTransport struct {
	inner       http.RoundTripper
	key         any
	expectedVal any
	sawValue    bool
	capturedCtx context.Context
}

func (t *contextCheckingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.capturedCtx = req.Context()
	if req.Context().Value(t.key) == t.expectedVal {
		t.sawValue = true
	}
	return t.inner.RoundTrip(req)
}

// TestHookContextPropagation verifies that the context returned by OnRequestStart
// is used for the HTTP request, enabling trace propagation and context values.
func TestHookContextPropagation(t *testing.T) {
	type ctxKey string
	key := ctxKey("trace-id")
	expectedValue := "abc123"

	// Test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	// Custom transport to verify context propagation within the client
	checkingTransport := &contextCheckingTransport{
		inner:       http.DefaultTransport,
		key:         key,
		expectedVal: expectedValue,
	}

	// Hook that adds a value to the context
	hooks := &contextCapturingHooks{
		key:   key,
		value: expectedValue,
	}

	// Create client with the hook and custom transport
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	token := &StaticTokenProvider{Token: "test-token"}
	client := NewClient(cfg, token,
		WithHooks(hooks),
		WithTransport(checkingTransport),
	)

	// Make a request
	ctx := context.Background()
	_, err := client.ForAccount("12345").Projects().List(ctx, nil)
	if err != nil {
		t.Logf("Request error (expected in some cases): %v", err)
	}

	// Verify the hook was called
	if hooks.endCallCount == 0 {
		t.Fatal("OnRequestEnd was not called")
	}

	// Verify the context passed to OnRequestEnd contains our value
	if hooks.capturedCtx == nil {
		t.Fatal("capturedCtx is nil")
	}
	if hooks.capturedCtx.Value(key) != expectedValue {
		t.Errorf("OnRequestEnd context missing hook value: got %v, want %v",
			hooks.capturedCtx.Value(key), expectedValue)
	}

	// Verify the hook context was propagated to the transport layer
	// This is the critical check - it ensures hookCtx is used for the actual HTTP request
	if !checkingTransport.sawValue {
		t.Error("Hook context value was not propagated to transport layer")
		t.Logf("Transport saw context value: %v (expected: %v)",
			checkingTransport.capturedCtx.Value(key), expectedValue)
	}
}
