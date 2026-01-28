// Package prometheus provides Prometheus metrics integration for the Basecamp SDK.
//
// It implements the basecamp.Hooks interface to expose standard HTTP client
// metrics in Prometheus format.
//
// # Usage
//
//	import (
//	    "github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
//	    basecampprom "github.com/basecamp/basecamp-sdk/go/pkg/basecamp/prometheus"
//	    "github.com/prometheus/client_golang/prometheus"
//	)
//
//	hooks := basecampprom.NewHooks(prometheus.DefaultRegisterer)
//	client := basecamp.NewClient(cfg, tokenProvider, basecamp.WithHooks(hooks))
package prometheus

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

const (
	namespace = "basecamp"
)

// Hooks implements basecamp.Hooks using Prometheus metrics.
type Hooks struct {
	operationDuration *prometheus.HistogramVec
	operationsTotal   *prometheus.CounterVec
	httpRequestsTotal *prometheus.CounterVec
	retriesTotal      *prometheus.CounterVec
	cacheOpsTotal     *prometheus.CounterVec
	errorsTotal       *prometheus.CounterVec
}

// Ensure Hooks implements basecamp.Hooks at compile time.
var _ basecamp.Hooks = (*Hooks)(nil)

// NewHooks creates a new Prometheus-based Hooks implementation.
// The registerer is used to register the metrics. Use prometheus.DefaultRegisterer
// for the global registry, or pass a custom registry for testing.
// Returns nil if registerer is nil.
func NewHooks(registerer prometheus.Registerer) *Hooks {
	if registerer == nil {
		return nil
	}

	h := &Hooks{
		operationDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "operation_duration_seconds",
				Help:      "Duration of Basecamp API operations in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"operation"}, // Semantic operation name (e.g., "Todos.Complete")
		),
		operationsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "operations_total",
				Help:      "Total number of Basecamp API operations.",
			},
			[]string{"operation", "status"}, // Semantic operation name
		),
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "http_requests_total",
				Help:      "Total number of HTTP requests to Basecamp API.",
			},
			[]string{"http_method", "status_code"}, // HTTP method (GET, POST, etc.)
		),
		retriesTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "retries_total",
				Help:      "Total number of request retries.",
			},
			[]string{"http_method"}, // HTTP method
		),
		cacheOpsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "cache_operations_total",
				Help:      "Total number of cache operations.",
			},
			[]string{"result"}, // hit, miss
		),
		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "errors_total",
				Help:      "Total number of errors.",
			},
			[]string{"http_method", "type"}, // HTTP method
		),
	}

	// Register all metrics
	registerer.MustRegister(
		h.operationDuration,
		h.operationsTotal,
		h.httpRequestsTotal,
		h.retriesTotal,
		h.cacheOpsTotal,
		h.errorsTotal,
	)

	return h
}

// OnOperationStart is called when a semantic SDK operation begins.
// Returns the context unchanged as Prometheus hooks don't need to track state.
func (h *Hooks) OnOperationStart(ctx context.Context, op basecamp.OperationInfo) context.Context {
	return ctx
}

// OnOperationEnd records metrics for a completed SDK operation.
func (h *Hooks) OnOperationEnd(ctx context.Context, op basecamp.OperationInfo, err error, duration time.Duration) {
	// Record operation duration by service and operation name
	operation := op.Service + "." + op.Operation
	h.operationDuration.WithLabelValues(operation).Observe(duration.Seconds())

	// Record operation status
	status := "success"
	if err != nil {
		status = "error"
	}
	h.operationsTotal.WithLabelValues(operation, status).Inc()
}

// OnRequestStart is called before an HTTP request is sent.
// Returns the context unchanged as no pre-processing is needed.
func (h *Hooks) OnRequestStart(ctx context.Context, info basecamp.RequestInfo) context.Context {
	return ctx
}

// OnRequestEnd records HTTP-level metrics for a completed request.
// Operation-level metrics are recorded in OnOperationEnd.
func (h *Hooks) OnRequestEnd(ctx context.Context, info basecamp.RequestInfo, result basecamp.RequestResult) {
	httpMethod := info.Method

	// Record HTTP request count by method and status code
	statusCode := "0"
	if result.StatusCode > 0 {
		statusCode = httpStatusToLabel(result.StatusCode)
	}
	h.httpRequestsTotal.WithLabelValues(httpMethod, statusCode).Inc()

	// Record cache operations for GET requests
	if httpMethod == "GET" {
		if result.FromCache {
			h.cacheOpsTotal.WithLabelValues("hit").Inc()
		} else {
			h.cacheOpsTotal.WithLabelValues("miss").Inc()
		}
	}

	// Record errors by type
	if result.Error != nil {
		errorType := classifyError(result.StatusCode)
		h.errorsTotal.WithLabelValues(httpMethod, errorType).Inc()
	}
}

// OnRetry records a retry attempt.
func (h *Hooks) OnRetry(ctx context.Context, info basecamp.RequestInfo, attempt int, err error) {
	h.retriesTotal.WithLabelValues(info.Method).Inc() // info.Method is the HTTP method
}

// httpStatusToLabel converts an HTTP status code to a string label.
// Groups status codes by class (2xx, 3xx, 4xx, 5xx) for common codes,
// or returns the exact code for less common ones.
func httpStatusToLabel(code int) string {
	switch code {
	case 200:
		return "200"
	case 201:
		return "201"
	case 204:
		return "204"
	case 304:
		return "304"
	case 400:
		return "400"
	case 401:
		return "401"
	case 403:
		return "403"
	case 404:
		return "404"
	case 429:
		return "429"
	case 500:
		return "500"
	case 502:
		return "502"
	case 503:
		return "503"
	case 504:
		return "504"
	default:
		// Group by class for uncommon codes
		switch {
		case code >= 200 && code < 300:
			return "2xx"
		case code >= 300 && code < 400:
			return "3xx"
		case code >= 400 && code < 500:
			return "4xx"
		case code >= 500:
			return "5xx"
		default:
			return "unknown"
		}
	}
}

// classifyError categorizes request failures for metrics labels based on HTTP status code,
// using 0 to represent network errors where no HTTP response was received.
func classifyError(statusCode int) string {
	if statusCode == 0 {
		return "network"
	}
	switch statusCode {
	case 401:
		return "auth"
	case 403:
		return "forbidden"
	case 404:
		return "not_found"
	case 429:
		return "rate_limit"
	case 500:
		return "server"
	case 502, 503, 504:
		return "gateway"
	default:
		if statusCode >= 400 && statusCode < 500 {
			return "client"
		}
		return "unknown"
	}
}
