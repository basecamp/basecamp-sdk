package basecamp

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func newTestSlogHooks(buf *bytes.Buffer) *SlogHooks {
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return NewSlogHooks(slog.New(handler))
}

func TestSlogHooks_OnOperationStart(t *testing.T) {
	var buf bytes.Buffer
	h := newTestSlogHooks(&buf)

	op := OperationInfo{
		Service:      "Todos",
		Operation:    "List",
		ResourceType: "todo",
		IsMutation:   false,
	}

	ctx := h.OnOperationStart(context.Background(), op)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	output := buf.String()
	for _, want := range []string{"basecamp operation start", "Todos", "List", "todo"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q: %s", want, output)
		}
	}
}

func TestSlogHooks_OnOperationEnd_Success(t *testing.T) {
	var buf bytes.Buffer
	h := newTestSlogHooks(&buf)

	op := OperationInfo{Service: "Projects", Operation: "Get"}
	h.OnOperationEnd(context.Background(), op, nil, 100*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "basecamp operation complete") {
		t.Errorf("expected 'operation complete', got: %s", output)
	}
	if !strings.Contains(output, "Projects") {
		t.Errorf("expected service name in output: %s", output)
	}
}

func TestSlogHooks_OnOperationEnd_Error(t *testing.T) {
	var buf bytes.Buffer
	h := newTestSlogHooks(&buf)

	op := OperationInfo{Service: "Todos", Operation: "Create"}
	h.OnOperationEnd(context.Background(), op, fmt.Errorf("network error"), 50*time.Millisecond)

	output := buf.String()
	if !strings.Contains(output, "basecamp operation failed") {
		t.Errorf("expected 'operation failed', got: %s", output)
	}
	if !strings.Contains(output, "network error") {
		t.Errorf("expected error in output: %s", output)
	}
}

func TestSlogHooks_OnRequestStart(t *testing.T) {
	var buf bytes.Buffer
	h := newTestSlogHooks(&buf)

	info := RequestInfo{Method: "GET", URL: "https://example.com/todos", Attempt: 1}
	ctx := h.OnRequestStart(context.Background(), info)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	output := buf.String()
	if !strings.Contains(output, "basecamp request start") {
		t.Errorf("expected 'request start', got: %s", output)
	}
	if !strings.Contains(output, "GET") {
		t.Errorf("expected method in output: %s", output)
	}
}

func TestSlogHooks_OnRequestEnd_Success(t *testing.T) {
	var buf bytes.Buffer
	h := newTestSlogHooks(&buf)

	info := RequestInfo{Method: "GET", URL: "https://example.com/todos"}
	result := RequestResult{StatusCode: 200, Duration: 50 * time.Millisecond, FromCache: true}

	h.OnRequestEnd(context.Background(), info, result)

	output := buf.String()
	if !strings.Contains(output, "basecamp request complete") {
		t.Errorf("expected 'request complete', got: %s", output)
	}
	if !strings.Contains(output, "from_cache=true") {
		t.Errorf("expected from_cache in output: %s", output)
	}
}

func TestSlogHooks_OnRequestEnd_Error(t *testing.T) {
	var buf bytes.Buffer
	h := newTestSlogHooks(&buf)

	info := RequestInfo{Method: "POST", URL: "https://example.com/todos"}
	result := RequestResult{Error: fmt.Errorf("timeout"), Duration: 5 * time.Second, Retryable: true}

	h.OnRequestEnd(context.Background(), info, result)

	output := buf.String()
	if !strings.Contains(output, "basecamp request failed") {
		t.Errorf("expected 'request failed', got: %s", output)
	}
	if !strings.Contains(output, "retryable=true") {
		t.Errorf("expected retryable in output: %s", output)
	}
}

func TestSlogHooks_OnRetry(t *testing.T) {
	var buf bytes.Buffer
	h := newTestSlogHooks(&buf)

	info := RequestInfo{Method: "GET", URL: "https://example.com/todos"}
	h.OnRetry(context.Background(), info, 3, fmt.Errorf("connection reset"))

	output := buf.String()
	if !strings.Contains(output, "basecamp request retry") {
		t.Errorf("expected 'request retry', got: %s", output)
	}
	if !strings.Contains(output, "attempt=3") {
		t.Errorf("expected attempt in output: %s", output)
	}
}

func TestSlogHooks_WithLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := NewSlogHooks(slog.New(handler), WithLevel(slog.LevelDebug))

	op := OperationInfo{Service: "Test", Operation: "Op"}
	h.OnOperationStart(context.Background(), op)

	// Debug-level message should be filtered by warn-level handler
	if buf.Len() > 0 {
		t.Errorf("expected no output at warn level, got: %s", buf.String())
	}
}

func TestSlogHooks_NilLogger(t *testing.T) {
	// Should not panic with nil logger
	h := NewSlogHooks(nil)
	op := OperationInfo{Service: "Test", Operation: "Op"}
	ctx := h.OnOperationStart(context.Background(), op)
	if ctx == nil {
		t.Error("expected non-nil context")
	}
}
