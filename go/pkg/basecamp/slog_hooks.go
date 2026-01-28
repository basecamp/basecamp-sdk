package basecamp

import (
	"context"
	"log/slog"
	"time"
)

// SlogHooks is a Hooks implementation that logs to a *slog.Logger.
// It provides structured logging for all SDK operations (both semantic and HTTP).
type SlogHooks struct {
	logger *slog.Logger
	level  slog.Level
}

// Ensure SlogHooks implements Hooks at compile time.
var _ Hooks = (*SlogHooks)(nil)

// SlogHooksOption configures a SlogHooks instance.
type SlogHooksOption func(*SlogHooks)

// WithLevel sets the log level for SlogHooks.
// Default is slog.LevelDebug.
func WithLevel(level slog.Level) SlogHooksOption {
	return func(h *SlogHooks) {
		h.level = level
	}
}

// NewSlogHooks creates a new SlogHooks that logs to the given logger.
// If logger is nil, uses slog.Default().
func NewSlogHooks(logger *slog.Logger, opts ...SlogHooksOption) *SlogHooks {
	if logger == nil {
		logger = slog.Default()
	}
	h := &SlogHooks{
		logger: logger,
		level:  slog.LevelDebug,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// OnOperationStart logs the start of a semantic SDK operation.
func (h *SlogHooks) OnOperationStart(ctx context.Context, op OperationInfo) context.Context {
	h.logger.Log(ctx, h.level, "basecamp operation start",
		slog.String("service", op.Service),
		slog.String("operation", op.Operation),
		slog.String("resource_type", op.ResourceType),
		slog.Bool("is_mutation", op.IsMutation),
	)
	return ctx
}

// OnOperationEnd logs the completion of a semantic SDK operation.
func (h *SlogHooks) OnOperationEnd(ctx context.Context, op OperationInfo, err error, duration time.Duration) {
	attrs := []slog.Attr{
		slog.String("service", op.Service),
		slog.String("operation", op.Operation),
		slog.Duration("duration", duration),
	}

	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
		h.logger.LogAttrs(ctx, h.level, "basecamp operation failed", attrs...)
	} else {
		h.logger.LogAttrs(ctx, h.level, "basecamp operation complete", attrs...)
	}
}

// OnRequestStart logs the start of an HTTP request.
func (h *SlogHooks) OnRequestStart(ctx context.Context, info RequestInfo) context.Context {
	h.logger.Log(ctx, h.level, "basecamp request start",
		slog.String("method", info.Method),
		slog.String("url", info.URL),
		slog.Int("attempt", info.Attempt),
	)
	return ctx
}

// OnRequestEnd logs the completion of an HTTP request.
func (h *SlogHooks) OnRequestEnd(ctx context.Context, info RequestInfo, result RequestResult) {
	attrs := []slog.Attr{
		slog.String("method", info.Method),
		slog.String("url", info.URL),
		slog.Duration("duration", result.Duration),
	}

	if result.Error != nil {
		attrs = append(attrs,
			slog.Any("error", result.Error),
			slog.Bool("retryable", result.Retryable),
		)
		h.logger.LogAttrs(ctx, h.level, "basecamp request failed", attrs...)
	} else {
		attrs = append(attrs,
			slog.Int("status", result.StatusCode),
			slog.Bool("from_cache", result.FromCache),
		)
		h.logger.LogAttrs(ctx, h.level, "basecamp request complete", attrs...)
	}
}

// OnRetry logs a retry attempt.
func (h *SlogHooks) OnRetry(ctx context.Context, info RequestInfo, attempt int, err error) {
	h.logger.Log(ctx, h.level, "basecamp request retry",
		slog.String("method", info.Method),
		slog.String("url", info.URL),
		slog.Int("attempt", attempt),
		slog.Any("error", err),
	)
}
