// Copyright 2025 Basecamp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The observability command demonstrates monitoring and tracing with the Basecamp SDK.
// It shows how to:
//   - Set up OpenTelemetry hooks for distributed tracing
//   - Set up Prometheus hooks for metrics collection
//   - Chain multiple hooks together
//   - Monitor API operations and HTTP requests
//
// This is essential for production applications that need visibility into SDK behavior.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
	basecampotel "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/otel"
	basecampprom "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/prometheus"
)

func main() {
	// Get credentials from environment.
	token := os.Getenv("BASECAMP_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: BASECAMP_TOKEN environment variable is required")
		os.Exit(1)
	}

	accountID := os.Getenv("BASECAMP_ACCOUNT_ID")
	if accountID == "" {
		fmt.Fprintln(os.Stderr, "Error: BASECAMP_ACCOUNT_ID environment variable is required")
		os.Exit(1)
	}

	fmt.Println("=== Basecamp SDK Observability Demo ===")
	fmt.Println()

	// Create a logger for structured output.
	// In production, configure this to send logs to your logging system.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create Prometheus metrics registry.
	// Using a custom registry keeps example metrics separate from global metrics.
	registry := prometheus.NewRegistry()

	// Create Prometheus hooks for metrics collection.
	// These hooks expose metrics like:
	//   - basecamp_operation_duration_seconds
	//   - basecamp_operations_total
	//   - basecamp_http_requests_total
	//   - basecamp_retries_total
	//   - basecamp_cache_operations_total
	//   - basecamp_errors_total
	promHooks := basecampprom.NewHooks(registry)

	// Create OpenTelemetry hooks for distributed tracing.
	// These hooks create spans for each SDK operation and HTTP request.
	// In production, configure an exporter (Jaeger, Zipkin, OTLP, etc.).
	otelHooks := basecampotel.NewHooks()

	// Chain the hooks together.
	// When multiple hooks are chained:
	//   - OnOperationStart: called in order (first to last)
	//   - OnOperationEnd: called in reverse order (last to first)
	// This ensures proper nesting of spans and metrics.
	hooks := basecamp.NewChainHooks(promHooks, otelHooks)

	fmt.Println("Hooks configured:")
	fmt.Println("  - Prometheus: metrics collection")
	fmt.Println("  - OpenTelemetry: distributed tracing")
	fmt.Println()

	// Create the SDK client with hooks enabled.
	cfg := basecamp.DefaultConfig()
	tokenProvider := &basecamp.StaticTokenProvider{Token: token}
	client := basecamp.NewClient(cfg, tokenProvider,
		basecamp.WithHooks(hooks),
		basecamp.WithLogger(logger),
	)
	account := client.ForAccount(accountID)

	// Start a Prometheus HTTP server for metrics scraping.
	// In production, integrate with your existing metrics infrastructure.
	metricsServer := startMetricsServer(registry)
	defer func() {
		if err := metricsServer.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing metrics server: %v\n", err)
		}
	}()

	fmt.Println("Prometheus metrics available at: http://localhost:9090/metrics")
	fmt.Println()

	// Make some API calls to generate metrics and traces.
	ctx := context.Background()

	fmt.Println("=== Making API Calls ===")
	fmt.Println()

	// Call 1: List projects
	fmt.Println("1. Listing projects...")
	start := time.Now()
	result, err := account.Projects().List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		fmt.Printf("   Found %d project(s) in %v\n", len(result.Projects), time.Since(start))
	}
	fmt.Println()

	// Call 2: List projects again (may be faster due to caching if enabled)
	fmt.Println("2. Listing projects again...")
	start = time.Now()
	result, err = account.Projects().List(ctx, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	} else {
		fmt.Printf("   Found %d project(s) in %v\n", len(result.Projects), time.Since(start))
	}
	fmt.Println()

	// Call 3: Get a specific project (if any exist)
	if len(result.Projects) > 0 {
		projectID := result.Projects[0].ID
		fmt.Printf("3. Getting project %d...\n", projectID)
		start = time.Now()
		project, err := account.Projects().Get(ctx, projectID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		} else {
			fmt.Printf("   Retrieved '%s' in %v\n", project.Name, time.Since(start))
		}
		fmt.Println()
	}

	// Display collected metrics.
	fmt.Println("=== Collected Metrics ===")
	fmt.Println()
	fmt.Println("Visit http://localhost:9090/metrics to see Prometheus metrics.")
	fmt.Println()
	fmt.Println("Example metrics you'll see:")
	fmt.Println()
	fmt.Println("  # Operation duration histogram")
	fmt.Println("  basecamp_operation_duration_seconds_bucket{operation=\"Projects.List\",le=\"0.1\"} 2")
	fmt.Println()
	fmt.Println("  # Operation counts by status")
	fmt.Println("  basecamp_operations_total{operation=\"Projects.List\",status=\"success\"} 2")
	fmt.Println("  basecamp_operations_total{operation=\"Projects.Get\",status=\"success\"} 1")
	fmt.Println()
	fmt.Println("  # HTTP requests by method and status")
	fmt.Println("  basecamp_http_requests_total{http_method=\"GET\",status_code=\"200\"} 3")
	fmt.Println()
	fmt.Println("  # Cache operations (if caching enabled)")
	fmt.Println("  basecamp_cache_operations_total{result=\"miss\"} 2")
	fmt.Println("  basecamp_cache_operations_total{result=\"hit\"} 1")
	fmt.Println()

	// Display tracing information.
	fmt.Println("=== Distributed Tracing ===")
	fmt.Println()
	fmt.Println("OpenTelemetry spans are created for each operation:")
	fmt.Println()
	fmt.Println("  Span: Projects.List")
	fmt.Println("    Attributes:")
	fmt.Println("      - basecamp.service: Projects")
	fmt.Println("      - basecamp.operation: List")
	fmt.Println("      - basecamp.resource_type: project")
	fmt.Println("      - basecamp.is_mutation: false")
	fmt.Println("    Child Span: basecamp.request")
	fmt.Println("      Attributes:")
	fmt.Println("        - http.method: GET")
	fmt.Println("        - http.url: https://3.basecampapi.com/...")
	fmt.Println("        - http.status_code: 200")
	fmt.Println()
	fmt.Println("To export traces, configure an OTLP exporter:")
	fmt.Println()
	fmt.Println("  import \"go.opentelemetry.io/otel/exporters/otlp/otlptrace\"")
	fmt.Println("  exporter, _ := otlptrace.New(ctx)")
	fmt.Println("  tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))")
	fmt.Println("  otel.SetTracerProvider(tp)")
	fmt.Println()

	// Show hook interface.
	fmt.Println("=== Hook Interface ===")
	fmt.Println()
	fmt.Println("The SDK provides these hook points:")
	fmt.Println()
	fmt.Println("  type Hooks interface {")
	fmt.Println("    // Called when a semantic SDK operation starts (e.g., Projects.List)")
	fmt.Println("    OnOperationStart(ctx, op OperationInfo) context.Context")
	fmt.Println()
	fmt.Println("    // Called when an operation completes")
	fmt.Println("    OnOperationEnd(ctx, op OperationInfo, err error, duration time.Duration)")
	fmt.Println()
	fmt.Println("    // Called before an HTTP request is sent")
	fmt.Println("    OnRequestStart(ctx, info RequestInfo) context.Context")
	fmt.Println()
	fmt.Println("    // Called after an HTTP request completes")
	fmt.Println("    OnRequestEnd(ctx, info RequestInfo, result RequestResult)")
	fmt.Println()
	fmt.Println("    // Called before a retry attempt")
	fmt.Println("    OnRetry(ctx, info RequestInfo, attempt int, err error)")
	fmt.Println("  }")
	fmt.Println()

	// Keep server running briefly to allow metrics scraping.
	fmt.Println("Metrics server running for 10 seconds...")
	fmt.Println("Press Ctrl+C to exit early.")
	time.Sleep(10 * time.Second)
	fmt.Println()
	fmt.Println("Done!")
}

// startMetricsServer starts an HTTP server for Prometheus metrics.
func startMetricsServer(registry *prometheus.Registry) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	}))

	server := &http.Server{
		Addr:              ":9090",
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Metrics server error: %v\n", err)
		}
	}()

	return server
}
