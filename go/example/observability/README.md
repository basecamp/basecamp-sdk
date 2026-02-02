# Observability Example

This example demonstrates how to monitor and trace Basecamp SDK operations using OpenTelemetry and Prometheus.

## What it demonstrates

- Setting up OpenTelemetry hooks for distributed tracing
- Setting up Prometheus hooks for metrics collection
- Chaining multiple hooks together
- Understanding the hook interface
- Exporting metrics and traces

## Prerequisites

1. A Basecamp account
2. An access token (see [simple example](../simple/))
3. Your Basecamp account ID

## Running the example

```bash
export BASECAMP_TOKEN="your-access-token"
export BASECAMP_ACCOUNT_ID="12345"
go run main.go
```

Then visit http://localhost:9090/metrics to see Prometheus metrics.

## Available hooks

The SDK provides two built-in hook implementations:

### Prometheus Hooks

```go
import basecampprom "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/prometheus"

hooks := basecampprom.NewHooks(prometheus.DefaultRegisterer)
client := basecamp.NewClient(cfg, tokenProvider, basecamp.WithHooks(hooks))
```

Metrics exposed:

| Metric | Type | Description |
|--------|------|-------------|
| `basecamp_operation_duration_seconds` | Histogram | Duration of SDK operations |
| `basecamp_operations_total` | Counter | Total operations by status |
| `basecamp_http_requests_total` | Counter | HTTP requests by method/status |
| `basecamp_retries_total` | Counter | Request retry attempts |
| `basecamp_cache_operations_total` | Counter | Cache hits and misses |
| `basecamp_errors_total` | Counter | Errors by type |

### OpenTelemetry Hooks

```go
import basecampotel "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/otel"

hooks := basecampotel.NewHooks()
client := basecamp.NewClient(cfg, tokenProvider, basecamp.WithHooks(hooks))
```

Spans created:

| Span Name | Description |
|-----------|-------------|
| `{Service}.{Operation}` | SDK operation (e.g., `Projects.List`) |
| `basecamp.request` | HTTP request (child of operation) |

Span attributes:

| Attribute | Description |
|-----------|-------------|
| `basecamp.service` | Service name (e.g., `Projects`) |
| `basecamp.operation` | Operation name (e.g., `List`) |
| `basecamp.resource_type` | Resource type (e.g., `project`) |
| `basecamp.is_mutation` | Whether operation modifies state |
| `http.method` | HTTP method |
| `http.url` | Request URL |
| `http.status_code` | Response status code |

## Chaining hooks

Combine multiple hooks for comprehensive observability:

```go
import (
    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
    basecampotel "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/otel"
    basecampprom "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/prometheus"
)

promHooks := basecampprom.NewHooks(prometheus.DefaultRegisterer)
otelHooks := basecampotel.NewHooks()

// Chain hooks together
hooks := basecamp.NewChainHooks(promHooks, otelHooks)

client := basecamp.NewClient(cfg, tokenProvider, basecamp.WithHooks(hooks))
```

Hook execution order:
- **Start events**: Called in order (first to last)
- **End events**: Called in reverse order (last to first)

This ensures proper nesting of spans and metrics.

## Custom hooks

Implement the `Hooks` interface for custom observability:

```go
type Hooks interface {
    // Called when a semantic SDK operation starts
    OnOperationStart(ctx context.Context, op OperationInfo) context.Context

    // Called when an operation completes
    OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration)

    // Called before an HTTP request is sent
    OnRequestStart(ctx context.Context, info RequestInfo) context.Context

    // Called after an HTTP request completes
    OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult)

    // Called before a retry attempt
    OnRetry(ctx context.Context, info RequestInfo, attempt int, err error)
}
```

Example custom hook for logging:

```go
type LoggingHooks struct {
    logger *slog.Logger
}

func (h *LoggingHooks) OnOperationStart(ctx context.Context, op basecamp.OperationInfo) context.Context {
    h.logger.Info("operation started",
        "service", op.Service,
        "operation", op.Operation,
    )
    return ctx
}

func (h *LoggingHooks) OnOperationEnd(ctx context.Context, op basecamp.OperationInfo, err error, d time.Duration) {
    if err != nil {
        h.logger.Error("operation failed",
            "service", op.Service,
            "operation", op.Operation,
            "duration", d,
            "error", err,
        )
    } else {
        h.logger.Info("operation completed",
            "service", op.Service,
            "operation", op.Operation,
            "duration", d,
        )
    }
}

// ... implement other methods
```

## Production setup

### OpenTelemetry with OTLP exporter

```go
import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

func initTracing() func() {
    ctx := context.Background()
    exporter, _ := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint("localhost:4317"),
        otlptracegrpc.WithInsecure(),
    )

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName("my-app"),
        )),
    )

    otel.SetTracerProvider(tp)

    return func() { tp.Shutdown(context.Background()) }
}
```

### Prometheus with push gateway

```go
import "github.com/prometheus/client_golang/prometheus/push"

pusher := push.New("http://pushgateway:9091", "my-app").
    Gatherer(prometheus.DefaultGatherer)

// Push metrics periodically
go func() {
    for range time.Tick(15 * time.Second) {
        pusher.Push()
    }
}()
```

### Kubernetes service monitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
  - port: metrics
    path: /metrics
    interval: 30s
```

## Debugging with slog

Enable debug logging for additional visibility:

```go
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

client := basecamp.NewClient(cfg, tokenProvider,
    basecamp.WithLogger(logger),
    basecamp.WithHooks(hooks),
)
```

This logs HTTP request/response details useful for debugging.
