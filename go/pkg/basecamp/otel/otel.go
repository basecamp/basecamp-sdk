// Package otel provides OpenTelemetry integration for the Basecamp SDK.
//
// It implements the basecamp.Hooks interface to provide distributed tracing
// and metrics for all HTTP operations.
//
// # Usage
//
//	import (
//	    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
//	    basecampotel "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/otel"
//	)
//
//	hooks := basecampotel.NewHooks()
//	client := basecamp.NewClient(cfg, tokenProvider, basecamp.WithHooks(hooks))
package otel

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

const (
	// instrumentationName is the name used for the tracer and meter.
	instrumentationName = "github.com/basecamp/basecamp-sdk/go"

	// Semantic convention attributes for Basecamp operations
	attrBasecampService      = "basecamp.service"
	attrBasecampOperation    = "basecamp.operation"
	attrBasecampResourceType = "basecamp.resource_type"
	attrBasecampIsMutation   = "basecamp.is_mutation"
	attrBasecampMethod       = "basecamp.method"
	attrBasecampURL          = "basecamp.url"
	attrBasecampAttempt      = "basecamp.attempt"
	attrBasecampStatus       = "basecamp.status"
	attrBasecampCached       = "basecamp.cached"
	attrHTTPMethod           = "http.method"
	attrHTTPURL              = "http.url"
	attrHTTPStatusCode       = "http.status_code"
)

// Hooks implements basecamp.Hooks using OpenTelemetry for tracing and metrics.
type Hooks struct {
	tracer            trace.Tracer
	meter             metric.Meter
	operationDuration metric.Float64Histogram
	requestDuration   metric.Float64Histogram
	requests          metric.Int64Counter
	retries           metric.Int64Counter
}

// operationSpanKey is the context key for operation spans.
type operationSpanKey struct{}

// Ensure Hooks implements basecamp.Hooks at compile time.
var _ basecamp.Hooks = (*Hooks)(nil)

// Option configures Hooks.
type Option func(*Hooks)

// WithTracerProvider sets a custom TracerProvider.
func WithTracerProvider(tp trace.TracerProvider) Option {
	return func(h *Hooks) {
		h.tracer = tp.Tracer(instrumentationName)
	}
}

// WithMeterProvider sets a custom MeterProvider.
func WithMeterProvider(mp metric.MeterProvider) Option {
	return func(h *Hooks) {
		h.meter = mp.Meter(instrumentationName)
	}
}

// NewHooks creates a new OpenTelemetry-based Hooks implementation.
// Uses the global TracerProvider and MeterProvider by default.
func NewHooks(opts ...Option) *Hooks {
	h := &Hooks{
		tracer: otel.Tracer(instrumentationName),
		meter:  otel.Meter(instrumentationName),
	}

	for _, opt := range opts {
		opt(h)
	}

	// Initialize metrics
	var err error

	h.operationDuration, err = h.meter.Float64Histogram(
		"basecamp.operation.duration",
		metric.WithDescription("Duration of Basecamp SDK operations in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		otel.Handle(err)
	}

	h.requestDuration, err = h.meter.Float64Histogram(
		"basecamp.request.duration",
		metric.WithDescription("Duration of Basecamp HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		otel.Handle(err)
	}

	h.requests, err = h.meter.Int64Counter(
		"basecamp.requests",
		metric.WithDescription("Total number of Basecamp HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		otel.Handle(err)
	}

	h.retries, err = h.meter.Int64Counter(
		"basecamp.retries",
		metric.WithDescription("Total number of Basecamp request retries"),
		metric.WithUnit("{retry}"),
	)
	if err != nil {
		otel.Handle(err)
	}

	return h
}

// spanKey is the context key for the span.
type spanKey struct{}

// OnOperationStart creates a new span for the semantic SDK operation.
func (h *Hooks) OnOperationStart(ctx context.Context, op basecamp.OperationInfo) context.Context {
	spanName := op.Service + "." + op.Operation
	ctx, span := h.tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrBasecampService, op.Service),
			attribute.String(attrBasecampOperation, op.Operation),
			attribute.String(attrBasecampResourceType, op.ResourceType),
			attribute.Bool(attrBasecampIsMutation, op.IsMutation),
		),
	)

	if op.ResourceID != 0 {
		span.SetAttributes(attribute.Int64("basecamp.resource_id", op.ResourceID))
	}

	return context.WithValue(ctx, operationSpanKey{}, span)
}

// OnOperationEnd records the operation result and ends the span.
func (h *Hooks) OnOperationEnd(ctx context.Context, op basecamp.OperationInfo, err error, duration time.Duration) {
	span, ok := ctx.Value(operationSpanKey{}).(trace.Span)
	if !ok || span == nil {
		return
	}
	defer span.End()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	// Record operation duration metric
	if h.operationDuration != nil {
		h.operationDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(
			attribute.String(attrBasecampService, op.Service),
			attribute.String(attrBasecampOperation, op.Operation),
		))
	}
}

// OnRequestStart creates a new span for the HTTP request.
func (h *Hooks) OnRequestStart(ctx context.Context, info basecamp.RequestInfo) context.Context {
	ctx, span := h.tracer.Start(ctx, "basecamp.request",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(attrHTTPMethod, info.Method),
			attribute.String(attrHTTPURL, info.URL),
			attribute.String(attrBasecampMethod, info.Method),
			attribute.String(attrBasecampURL, info.URL),
			attribute.Int(attrBasecampAttempt, info.Attempt),
		),
	)

	return context.WithValue(ctx, spanKey{}, span)
}

// OnRequestEnd records the request result and ends the span.
func (h *Hooks) OnRequestEnd(ctx context.Context, info basecamp.RequestInfo, result basecamp.RequestResult) {
	span, ok := ctx.Value(spanKey{}).(trace.Span)
	if !ok || span == nil {
		return
	}
	defer span.End()

	// Add result attributes
	attrs := []attribute.KeyValue{
		attribute.Bool(attrBasecampCached, result.FromCache),
	}

	if result.StatusCode > 0 {
		attrs = append(attrs, attribute.Int(attrHTTPStatusCode, result.StatusCode))
		attrs = append(attrs, attribute.Int(attrBasecampStatus, result.StatusCode))
	}

	span.SetAttributes(attrs...)

	// Record error if present
	if result.Error != nil {
		span.RecordError(result.Error)
		span.SetStatus(codes.Error, result.Error.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	// Record metrics
	metricAttrs := metric.WithAttributes(
		attribute.String(attrHTTPMethod, info.Method),
		attribute.Bool(attrBasecampCached, result.FromCache),
	)

	if h.requestDuration != nil {
		h.requestDuration.Record(ctx, result.Duration.Seconds(), metricAttrs)
	}

	if h.requests != nil {
		statusAttr := attribute.Int(attrHTTPStatusCode, result.StatusCode)
		if result.Error != nil && result.StatusCode == 0 {
			statusAttr = attribute.String("error", "connection_failed")
		}
		h.requests.Add(ctx, 1, metric.WithAttributes(
			attribute.String(attrHTTPMethod, info.Method),
			statusAttr,
		))
	}
}

// OnRetry records a retry attempt.
func (h *Hooks) OnRetry(ctx context.Context, info basecamp.RequestInfo, attempt int, err error) {
	span, ok := ctx.Value(spanKey{}).(trace.Span)
	if ok && span != nil {
		span.AddEvent("retry",
			trace.WithAttributes(
				attribute.Int("attempt", attempt),
				attribute.String("error", err.Error()),
			),
		)
	}

	if h.retries != nil {
		h.retries.Add(ctx, 1, metric.WithAttributes(
			attribute.String(attrHTTPMethod, info.Method),
			attribute.Int("attempt", attempt),
		))
	}
}
