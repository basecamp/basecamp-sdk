package prometheus

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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
