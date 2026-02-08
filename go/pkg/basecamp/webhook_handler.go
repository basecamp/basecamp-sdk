package basecamp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// WebhookEventHandler handles a parsed webhook event.
type WebhookEventHandler func(event *WebhookEvent) error

// WebhookMiddleware wraps webhook event processing.
type WebhookMiddleware func(event *WebhookEvent, next func() error) error

// WebhookReceiverConfig configures a WebhookReceiver.
type WebhookReceiverConfig struct {
	// Secret is the HMAC secret for signature verification. If empty, verification is skipped.
	Secret string
	// SignatureHeader is the HTTP header containing the signature (default: "X-Basecamp-Signature").
	SignatureHeader string
	// MaxBodyBytes limits the request body size (default: 1MB).
	MaxBodyBytes int64
	// DedupWindowSize is the number of recent event IDs to track for deduplication (default: 1000, 0 to disable).
	DedupWindowSize int
}

// WebhookVerificationError indicates a signature verification failure.
type WebhookVerificationError struct {
	Message string
}

func (e *WebhookVerificationError) Error() string {
	return e.Message
}

// WebhookReceiver receives and routes webhook events from Basecamp.
// It implements http.Handler for direct use as an HTTP endpoint.
type WebhookReceiver struct {
	config      WebhookReceiverConfig
	handlers    map[string][]WebhookEventHandler
	anyHandlers []WebhookEventHandler
	middleware  []WebhookMiddleware
	mu          sync.RWMutex

	// dedup
	dedupSet   map[int64]struct{}
	dedupOrder []int64
	dedupMu    sync.Mutex
}

// NewWebhookReceiver creates a new WebhookReceiver with the given config.
func NewWebhookReceiver(config WebhookReceiverConfig) *WebhookReceiver {
	if config.SignatureHeader == "" {
		config.SignatureHeader = "X-Basecamp-Signature"
	}
	if config.MaxBodyBytes == 0 {
		config.MaxBodyBytes = 1 << 20 // 1MB
	}
	if config.DedupWindowSize == 0 {
		config.DedupWindowSize = 1000
	}

	r := &WebhookReceiver{
		config:   config,
		handlers: make(map[string][]WebhookEventHandler),
	}
	if config.DedupWindowSize > 0 {
		r.dedupSet = make(map[int64]struct{})
		r.dedupOrder = make([]int64, 0, config.DedupWindowSize)
	}
	return r
}

// On registers a handler for a specific event kind pattern.
// Patterns support simple glob matching: "todo_*" matches "todo_created", "todo_completed", etc.
// "*_created" matches "todo_created", "message_created", etc.
func (r *WebhookReceiver) On(pattern string, handler WebhookEventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[pattern] = append(r.handlers[pattern], handler)
}

// OnAny registers a handler for all webhook events.
func (r *WebhookReceiver) OnAny(handler WebhookEventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.anyHandlers = append(r.anyHandlers, handler)
}

// Use adds a middleware to the processing chain.
func (r *WebhookReceiver) Use(middleware WebhookMiddleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, middleware)
}

// HandleRequest processes a raw webhook request body and headers.
// Returns the parsed WebhookEvent, or an error if verification/parsing fails.
// Duplicate events (by ID) return the parsed event but do not trigger handlers.
func (r *WebhookReceiver) HandleRequest(body []byte, getHeader func(string) string) (*WebhookEvent, error) {
	// Verify signature if secret is configured.
	if r.config.Secret != "" {
		sig := getHeader(r.config.SignatureHeader)
		if !VerifyWebhookSignature(body, sig, r.config.Secret) {
			return nil, &WebhookVerificationError{Message: "invalid webhook signature"}
		}
	}

	// Parse event.
	var event WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook event: %w", err)
	}

	// Dedup check — only check, don't record yet (record after successful handling).
	if r.isSeen(event.ID) {
		return &event, nil
	}

	// Run middleware and handlers.
	r.mu.RLock()
	middleware := make([]WebhookMiddleware, len(r.middleware))
	copy(middleware, r.middleware)
	r.mu.RUnlock()

	runHandlers := func() error {
		return r.dispatchHandlers(&event)
	}

	// Build middleware chain.
	chain := runHandlers
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		next := chain
		chain = func() error {
			return mw(&event, next)
		}
	}

	if err := chain(); err != nil {
		return &event, err
	}

	// Record in dedup window only after successful handling.
	r.markSeen(event.ID)

	return &event, nil
}

// ServeHTTP implements http.Handler.
func (r *WebhookReceiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read up to MaxBodyBytes+1 to detect oversized requests.
	body, err := io.ReadAll(io.LimitReader(req.Body, r.config.MaxBodyBytes+1))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if int64(len(body)) > r.config.MaxBodyBytes {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	_, err = r.HandleRequest(body, req.Header.Get)
	if err != nil {
		if _, ok := err.(*WebhookVerificationError); ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (r *WebhookReceiver) isSeen(eventID int64) bool {
	if r.config.DedupWindowSize <= 0 || eventID == 0 {
		return false
	}

	r.dedupMu.Lock()
	defer r.dedupMu.Unlock()

	_, exists := r.dedupSet[eventID]
	return exists
}

func (r *WebhookReceiver) markSeen(eventID int64) {
	if r.config.DedupWindowSize <= 0 || eventID == 0 {
		return
	}

	r.dedupMu.Lock()
	defer r.dedupMu.Unlock()

	if _, exists := r.dedupSet[eventID]; exists {
		return
	}

	// Evict oldest if at capacity.
	if len(r.dedupOrder) >= r.config.DedupWindowSize {
		oldest := r.dedupOrder[0]
		r.dedupOrder = r.dedupOrder[1:]
		delete(r.dedupSet, oldest)
	}

	r.dedupSet[eventID] = struct{}{}
	r.dedupOrder = append(r.dedupOrder, eventID)
}

func (r *WebhookReceiver) dispatchHandlers(event *WebhookEvent) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Collect matching handlers.
	var matched []WebhookEventHandler

	for pattern, handlers := range r.handlers {
		if matchPattern(pattern, event.Kind) {
			matched = append(matched, handlers...)
		}
	}

	matched = append(matched, r.anyHandlers...)

	// Execute all matching handlers.
	for _, handler := range matched {
		if err := handler(event); err != nil {
			return err
		}
	}

	return nil
}

// matchPattern matches a webhook event kind against a glob pattern.
// Supports * as a wildcard that matches any sequence of characters.
//
//   - "todo_created" matches "todo_created" (exact)
//   - "todo_*" matches "todo_created", "todo_completed" etc.
//   - "*_created" matches "todo_created", "message_created" etc.
func matchPattern(pattern, value string) bool {
	// Fast path: exact match.
	if pattern == value {
		return true
	}

	// Simple glob: split on * and match segments.
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		// No wildcards, must be exact match (already checked above).
		return false
	}

	remaining := value
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(remaining, part)
		if idx == -1 {
			return false
		}
		// First part must be a prefix.
		if i == 0 && idx != 0 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}

	// Last part must be a suffix (if non-empty).
	lastPart := parts[len(parts)-1]
	if lastPart != "" {
		return strings.HasSuffix(value, lastPart)
	}

	return true
}
