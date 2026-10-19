package prometheus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

func TestHooksImplementsInterface(t *testing.T) {
	// Compile-time check that Hooks implements basecamp.Hooks
	var _ basecamp.Hooks = (*Hooks)(nil)
}

func TestNewHooksNilRegisterer(t *testing.T) {
	hooks := NewHooks(nil)
	if hooks != nil {
		t.Error("NewHooks(nil) should return nil")
	}
}

func TestNewHooks(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	if hooks == nil {
		t.Fatal("NewHooks returned nil")
	}
}

func TestOnOperationStartEnd(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	ctx := context.Background()

	op := basecamp.OperationInfo{
		Service:   "Todos",
		Operation: "Complete",
	}

	// Start operation (no-op for prometheus, just returns context)
	ctx = hooks.OnOperationStart(ctx, op)

	// End operation
	hooks.OnOperationEnd(ctx, op, nil, 100*time.Millisecond)

	// Verify metrics were recorded
	expectedMetric := `
		# HELP basecamp_operation_duration_seconds Duration of Basecamp API operations in seconds.
		# TYPE basecamp_operation_duration_seconds histogram
	`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(expectedMetric),
		"basecamp_operation_duration_seconds"); err != nil {
		// Just verify the metric exists (histogram buckets make exact comparison difficult)
		count := testutil.CollectAndCount(reg, "basecamp_operation_duration_seconds")
		if count == 0 {
			t.Error("expected operation_duration metric to be recorded")
		}
	}

	// Check operations_total counter
	count := testutil.CollectAndCount(reg, "basecamp_operations_total")
	if count == 0 {
		t.Error("expected operations_total metric to be recorded")
	}
}

func TestOnOperationEndWithError(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	ctx := context.Background()

	op := basecamp.OperationInfo{
		Service:   "Todos",
		Operation: "Get",
	}

	ctx = hooks.OnOperationStart(ctx, op)
	hooks.OnOperationEnd(ctx, op, errors.New("not found"), 50*time.Millisecond)

	// The operation should be recorded with status=error
	count := testutil.CollectAndCount(reg, "basecamp_operations_total")
	if count == 0 {
		t.Error("expected operations_total metric to be recorded")
	}
}

func TestOnRequestStartEnd(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
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

	// Verify HTTP request metrics
	count := testutil.CollectAndCount(reg, "basecamp_http_requests_total")
	if count == 0 {
		t.Error("expected http_requests_total metric to be recorded")
	}
}

func TestOnRequestEndWithCache(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	ctx := context.Background()

	info := basecamp.RequestInfo{
		Method:  "GET",
		URL:     "https://example.com/api/todos",
		Attempt: 1,
	}

	ctx = hooks.OnRequestStart(ctx, info)
	hooks.OnRequestEnd(ctx, info, basecamp.RequestResult{
		StatusCode: 200,
		Duration:   5 * time.Millisecond,
		FromCache:  true,
	})

	// Verify cache metrics
	count := testutil.CollectAndCount(reg, "basecamp_cache_operations_total")
	if count == 0 {
		t.Error("expected cache_operations_total metric to be recorded")
	}
}

func TestOnRequestEndWithError(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	ctx := context.Background()

	info := basecamp.RequestInfo{
		Method:  "GET",
		URL:     "https://example.com/api/todos",
		Attempt: 1,
	}

	ctx = hooks.OnRequestStart(ctx, info)
	hooks.OnRequestEnd(ctx, info, basecamp.RequestResult{
		StatusCode: 404,
		Duration:   50 * time.Millisecond,
		Error:      errors.New("not found"),
	})

	// Verify error metrics
	count := testutil.CollectAndCount(reg, "basecamp_errors_total")
	if count == 0 {
		t.Error("expected errors_total metric to be recorded")
	}
}

func TestOnRetry(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	ctx := context.Background()

	info := basecamp.RequestInfo{
		Method:  "GET",
		URL:     "https://example.com/api/todos",
		Attempt: 1,
	}

	hooks.OnRetry(ctx, info, 2, errors.New("timeout"))

	// Verify retry metrics
	count := testutil.CollectAndCount(reg, "basecamp_retries_total")
	if count == 0 {
		t.Error("expected retries_total metric to be recorded")
	}
}

func TestHttpStatusToLabel(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{200, "200"},
		{201, "201"},
		{204, "204"},
		{304, "304"},
		{400, "400"},
		{401, "401"},
		{403, "403"},
		{404, "404"},
		{429, "429"},
		{500, "500"},
		{502, "502"},
		{503, "503"},
		{504, "504"},
		{202, "2xx"},    // Uncommon 2xx
		{301, "3xx"},    // Uncommon 3xx
		{418, "4xx"},    // Uncommon 4xx (I'm a teapot)
		{599, "5xx"},    // Uncommon 5xx
		{0, "unknown"},  // Invalid
		{99, "unknown"}, // Invalid
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := httpStatusToLabel(tt.code)
			if result != tt.expected {
				t.Errorf("httpStatusToLabel(%d) = %q, want %q", tt.code, result, tt.expected)
			}
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		statusCode int
		expected   string
	}{
		{0, "network"},
		{401, "auth"},
		{403, "forbidden"},
		{404, "not_found"},
		{429, "rate_limit"},
		{500, "server"},
		{502, "gateway"},
		{503, "gateway"},
		{504, "gateway"},
		{400, "client"},
		{422, "client"},
		{999, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := classifyError(tt.statusCode)
			if result != tt.expected {
				t.Errorf("classifyError(%d) = %q, want %q", tt.statusCode, result, tt.expected)
			}
		})
	}
}

func TestMetricLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	ctx := context.Background()

	// Test various status codes to verify label cardinality
	statusCodes := []int{200, 201, 400, 401, 403, 404, 429, 500, 502, 503}
	for _, code := range statusCodes {
		info := basecamp.RequestInfo{Method: "GET", URL: "https://example.com", Attempt: 1}
		ctx = hooks.OnRequestStart(ctx, info)
		hooks.OnRequestEnd(ctx, info, basecamp.RequestResult{
			StatusCode: code,
			Duration:   10 * time.Millisecond,
		})
	}

	// Should have metrics for each status code
	count := testutil.CollectAndCount(reg, "basecamp_http_requests_total")
	if count < len(statusCodes) {
		t.Errorf("expected at least %d metric series, got %d", len(statusCodes), count)
	}
}

// TestExpositionShape pins, byte for byte, the text exposition a scraper
// receives for every metric family the hooks register: names, HELP, TYPE, label
// sets, counter values, and the histogram's bucket boundaries, sum and count.
// Dashboards and alerts key on exactly this output, so a client_golang or
// prometheus/common change to it must fail here rather than slip through the
// presence checks above.
//
// It serves the registry through promhttp.HandlerFor and compares the response
// body, rather than using testutil.GatherAndCompare: that parses the expected
// text and re-encodes both sides through the same encoder before comparing, so
// an encoder change moves both sides together and passes. promhttp is also the
// path an application exposes, and it exists on every client_golang this module
// supports (the go.mod floor and the "latest dependencies" CI job), unlike
// testutil.GatherAndFormat.
func TestExpositionShape(t *testing.T) {
	reg := prometheus.NewRegistry()
	hooks := NewHooks(reg)
	ctx := context.Background()

	op := basecamp.OperationInfo{Service: "Todos", Operation: "Complete"}
	hooks.OnOperationEnd(ctx, op, nil, 100*time.Millisecond)
	hooks.OnOperationEnd(ctx, op, errors.New("boom"), 100*time.Millisecond)

	get := basecamp.RequestInfo{Method: "GET", URL: "https://example.com/api/todos", Attempt: 1}
	post := basecamp.RequestInfo{Method: "POST", URL: "https://example.com/api/todos", Attempt: 1}
	hooks.OnRequestEnd(ctx, get, basecamp.RequestResult{StatusCode: 200, FromCache: true})
	hooks.OnRequestEnd(ctx, post, basecamp.RequestResult{StatusCode: 404, Error: errors.New("not found")})
	hooks.OnRequestEnd(ctx, get, basecamp.RequestResult{Error: errors.New("dial tcp: refused")})
	hooks.OnRetry(ctx, get, 2, errors.New("timeout"))

	expected := `
# HELP basecamp_cache_operations_total Total number of cache operations.
# TYPE basecamp_cache_operations_total counter
basecamp_cache_operations_total{result="hit"} 1
basecamp_cache_operations_total{result="miss"} 1
# HELP basecamp_errors_total Total number of errors.
# TYPE basecamp_errors_total counter
basecamp_errors_total{http_method="GET",type="network"} 1
basecamp_errors_total{http_method="POST",type="not_found"} 1
# HELP basecamp_http_requests_total Total number of HTTP requests to Basecamp API.
# TYPE basecamp_http_requests_total counter
basecamp_http_requests_total{http_method="GET",status_code="0"} 1
basecamp_http_requests_total{http_method="GET",status_code="200"} 1
basecamp_http_requests_total{http_method="POST",status_code="404"} 1
# HELP basecamp_operation_duration_seconds Duration of Basecamp API operations in seconds.
# TYPE basecamp_operation_duration_seconds histogram
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="0.005"} 0
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="0.01"} 0
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="0.025"} 0
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="0.05"} 0
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="0.1"} 2
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="0.25"} 2
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="0.5"} 2
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="1"} 2
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="2.5"} 2
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="5"} 2
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="10"} 2
basecamp_operation_duration_seconds_bucket{operation="Todos.Complete",le="+Inf"} 2
basecamp_operation_duration_seconds_sum{operation="Todos.Complete"} 0.2
basecamp_operation_duration_seconds_count{operation="Todos.Complete"} 2
# HELP basecamp_operations_total Total number of Basecamp API operations.
# TYPE basecamp_operations_total counter
basecamp_operations_total{operation="Todos.Complete",status="error"} 1
basecamp_operations_total{operation="Todos.Complete",status="success"} 1
# HELP basecamp_retries_total Total number of request retries.
# TYPE basecamp_retries_total counter
basecamp_retries_total{http_method="GET"} 1
`
	expected = strings.TrimPrefix(expected, "\n")

	// Pin the negotiation: ask for the classic text format (0.0.4), uncompressed,
	// so the body does not depend on a default Accept or on OpenMetrics or
	// compression support in the client_golang under test. No metric-name filter:
	// an extra or missing family fails too.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept", "text/plain; version=0.0.4")
	req.Header.Set("Accept-Encoding", "identity")
	rec := httptest.NewRecorder()
	promhttp.HandlerFor(reg, promhttp.HandlerOpts{ErrorHandling: promhttp.PanicOnError}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Fatalf("Content-Type = %q, want the text/plain version=0.0.4 format", ct)
	}
	if ce := rec.Header().Get("Content-Encoding"); ce != "" {
		t.Fatalf("Content-Encoding = %q, want none", ce)
	}
	if got := rec.Body.String(); got != expected {
		t.Fatalf("exposition differs from the pinned bytes\n--- got\n%s--- want\n%s", got, expected)
	}
}
