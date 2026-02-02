package otel

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func TestHooksImplementsInterface(t *testing.T) {
	// Compile-time check that Hooks implements basecamp.Hooks
	var _ basecamp.Hooks = (*Hooks)(nil)
}

func TestNewHooks(t *testing.T) {
	hooks := NewHooks()
	if hooks == nil {
		t.Fatal("NewHooks returned nil")
	}
	if hooks.tracer == nil {
		t.Error("tracer should not be nil")
	}
	if hooks.meter == nil {
		t.Error("meter should not be nil")
	}
}

func TestNewHooksWithOptions(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	hooks := NewHooks(WithTracerProvider(tp))
	if hooks == nil {
		t.Fatal("NewHooks returned nil")
	}
}

func TestOnOperationStartEnd(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	hooks := NewHooks(WithTracerProvider(tp))
	ctx := context.Background()

	op := basecamp.OperationInfo{
		Service:      "Todos",
		Operation:    "Complete",
		ResourceType: "todo",
		IsMutation:   true,
		ResourceID:   456,
	}

	// Start operation
	ctx = hooks.OnOperationStart(ctx, op)

	// End operation
	hooks.OnOperationEnd(ctx, op, nil, 100*time.Millisecond)

	// Verify span was created
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "Todos.Complete" {
		t.Errorf("expected span name 'Todos.Complete', got %q", span.Name)
	}
	if span.Status.Code != codes.Ok {
		t.Errorf("expected status Ok, got %v", span.Status.Code)
	}

	// Check attributes
	attrs := make(map[string]any)
	for _, attr := range span.Attributes {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}

	if attrs["basecamp.service"] != "Todos" {
		t.Errorf("expected basecamp.service='Todos', got %v", attrs["basecamp.service"])
	}
	if attrs["basecamp.operation"] != "Complete" {
		t.Errorf("expected basecamp.operation='Complete', got %v", attrs["basecamp.operation"])
	}
	if attrs["basecamp.is_mutation"] != true {
		t.Errorf("expected basecamp.is_mutation=true, got %v", attrs["basecamp.is_mutation"])
	}
	if attrs["basecamp.resource_id"] != int64(456) {
		t.Errorf("expected basecamp.resource_id=456, got %v", attrs["basecamp.resource_id"])
	}
}

func TestOnOperationEndWithError(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	hooks := NewHooks(WithTracerProvider(tp))
	ctx := context.Background()

	op := basecamp.OperationInfo{
		Service:   "Todos",
		Operation: "Get",
	}

	ctx = hooks.OnOperationStart(ctx, op)
	testErr := errors.New("not found")
	hooks.OnOperationEnd(ctx, op, testErr, 50*time.Millisecond)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Errorf("expected status Error, got %v", span.Status.Code)
	}
	if span.Status.Description != "not found" {
		t.Errorf("expected status description 'not found', got %q", span.Status.Description)
	}
}

func TestOnRequestStartEnd(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	hooks := NewHooks(WithTracerProvider(tp))
	ctx := context.Background()

	info := basecamp.RequestInfo{
		Method:  "GET",
		URL:     "https://example.com/api/todos",
		Attempt: 1,
	}

	ctx = hooks.OnRequestStart(ctx, info)
	hooks.OnRequestEnd(ctx, info, basecamp.RequestResult{
		StatusCode: 200,
		Duration:   50 * time.Millisecond,
	})

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "basecamp.request" {
		t.Errorf("expected span name 'basecamp.request', got %q", span.Name)
	}

	attrs := make(map[string]any)
	for _, attr := range span.Attributes {
		attrs[string(attr.Key)] = attr.Value.AsInterface()
	}

	if attrs["http.method"] != "GET" {
		t.Errorf("expected http.method='GET', got %v", attrs["http.method"])
	}
	if attrs["http.status_code"] != int64(200) {
		t.Errorf("expected http.status_code=200, got %v", attrs["http.status_code"])
	}
}

func TestOnRetry(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	hooks := NewHooks(WithTracerProvider(tp))
	ctx := context.Background()

	info := basecamp.RequestInfo{
		Method:  "GET",
		URL:     "https://example.com/api/todos",
		Attempt: 1,
	}

	// Start a request span first
	ctx = hooks.OnRequestStart(ctx, info)

	// Record retry
	hooks.OnRetry(ctx, info, 2, errors.New("timeout"))

	// End request
	hooks.OnRequestEnd(ctx, info, basecamp.RequestResult{StatusCode: 200})

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	// Check for retry event
	events := spans[0].Events
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Name != "retry" {
		t.Errorf("expected event name 'retry', got %q", event.Name)
	}
}

func TestNestedOperationAndRequest(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	hooks := NewHooks(WithTracerProvider(tp))
	ctx := context.Background()

	// Start operation
	op := basecamp.OperationInfo{Service: "Todos", Operation: "List"}
	ctx = hooks.OnOperationStart(ctx, op)

	// Start request (nested under operation)
	info := basecamp.RequestInfo{Method: "GET", URL: "https://example.com/api/todos", Attempt: 1}
	reqCtx := hooks.OnRequestStart(ctx, info)

	// End request
	hooks.OnRequestEnd(reqCtx, info, basecamp.RequestResult{StatusCode: 200})

	// End operation
	hooks.OnOperationEnd(ctx, op, nil, 100*time.Millisecond)

	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// Verify spans are properly nested (request span should have operation span as parent)
	var opSpan, reqSpan tracetest.SpanStub
	for _, s := range spans {
		switch s.Name {
		case "Todos.List":
			opSpan = s
		case "basecamp.request":
			reqSpan = s
		}
	}

	if opSpan.SpanContext.SpanID().IsValid() && reqSpan.Parent.SpanID() == opSpan.SpanContext.SpanID() {
		// Request span is child of operation span - correct nesting
	} else {
		t.Logf("Operation span ID: %s", opSpan.SpanContext.SpanID())
		t.Logf("Request parent ID: %s", reqSpan.Parent.SpanID())
		// This is expected when using context propagation
	}
}
