package eventfeed

// TerminalReason identifies why the feed terminated (SPEC.md §23 "Terminal
// and Continuable Outcomes"). Terminal reasons end iteration with exactly one
// typed error element; cancellation, close(), and a consumer break end
// iteration with no error element.
//
// `auth_revoked` is deliberately reserved, not defined: the wire carries no
// distinct revocation signal — revocation is one possible cause of repeated
// unauthorized failures, and the error name must not claim certainty the wire
// cannot substantiate. It comes into existence only if bc3 ships an observable
// revocation contract.
type TerminalReason string

const (
	// ReasonSubscriptionRejected — `reject_subscription`: always terminal,
	// first attempt or reconnect; zero reconnect attempts.
	ReasonSubscriptionRejected TerminalReason = "subscription_rejected"
	// ReasonProtocolFatal — raw disconnect frame
	// `reason=invalid_event_stream_command`; terminal from every socket-open
	// state.
	ReasonProtocolFatal TerminalReason = "protocol_fatal"
	// ReasonFilterInvalid — 400-filter from a poll: a configuration error a
	// position reset won't help; the server's message naming the offending
	// list is preserved.
	ReasonFilterInvalid TerminalReason = "filter_invalid"
	// ReasonAuthorizationFailed — 3rd consecutive connection-level
	// authorization failure on the shared counter (unauthorized mint,
	// `unauthorized` disconnect, or unauthorized poll).
	ReasonAuthorizationFailed TerminalReason = "authorization_failed"
	// ReasonCheckpointLoad — CheckpointStore.Load failed at start; zero wire
	// attempts (silently starting at the present would skip history).
	ReasonCheckpointLoad TerminalReason = "checkpoint_load"
	// ReasonUsage — a re-consumed iterator (the one usage condition surfaced
	// as an iteration element); construction-time validation failures raise
	// construction errors carrying the same code.
	ReasonUsage TerminalReason = "usage"
	// ReasonBufferOverflow — live-buffer overflow with no registered handler,
	// or a handler returning Terminate.
	ReasonBufferOverflow TerminalReason = "buffer_overflow"
	// ReasonFeedGap — 410 with no registered handler, or a handler returning
	// Terminate. A 410 never silently auto-continues.
	ReasonFeedGap TerminalReason = "feed_gap"
	// ReasonInvalidContinuation — a `next` or 410 `resume` URL (or a redirect
	// Location) failed same-origin/downgrade validation; no request is issued
	// to the failing URL.
	ReasonInvalidContinuation TerminalReason = "invalid_continuation"
	// ReasonPollFailed — an unrecoverable-kind poll error: a
	// generated-operation outcome outside the feed's 400/409/410 matrix and
	// the retryable classes (e.g. 404, 405, an unexpected shape), passed
	// through with the generated error attached.
	ReasonPollFailed TerminalReason = "poll_failed"
	// ReasonMintFailed — an unrecoverable-kind mint error: a non-retryable
	// CreateStreamTicket outcome other than 401/403 (e.g. 404, 422, a
	// malformed success), passed through with the generated error attached.
	ReasonMintFailed TerminalReason = "mint_failed"
	// ReasonInvalidCableURL — the mint's `url` violates cable-URL policy
	// (non-`wss://` outside localhost, a redirect on dial, unparseable):
	// recurring by construction on every re-mint, so it is surfaced, never
	// retried into.
	ReasonInvalidCableURL TerminalReason = "invalid_cable_url"
)

// TerminalError is a terminal feed error carrying a TerminalReason, mirroring
// the oauth.DeviceFlowError{Reason} precedent. It is the single final error
// element a terminated iteration yields.
type TerminalError struct {
	// Reason identifies the termination class.
	Reason TerminalReason
	// Msg carries condition detail (e.g. the server's filter-invalid message,
	// or a rejected URL redacted to its origin). Never a ticket or a mint
	// URL's query string.
	Msg string
	// Err is the underlying cause, if any (e.g. the generated error behind
	// mint_failed/poll_failed, or the store error behind checkpoint_load).
	Err error
}

// Error implements the error interface. The composed result is bounded by
// §9's MAX_ERROR_MESSAGE_LENGTH like every other rendering here that can
// carry server-derived text: Msg holds the server's filter-invalid message
// verbatim, or a rejected continuation's scheme or origin, and nothing
// upstream bounds their length.
func (e *TerminalError) Error() string {
	msg := "event feed terminal: " + string(e.Reason)
	if e.Msg != "" {
		msg += ": " + e.Msg
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return truncateErrorText(msg)
}

// Unwrap exposes the underlying cause for errors.Is / errors.As traversal.
func (e *TerminalError) Unwrap() error {
	return e.Err
}

// maxErrorMessageBytes bounds every error rendering that can carry
// frame-derived text (SPEC.md §9 "Error Message Truncation",
// MAX_ERROR_MESSAGE_LENGTH = 500; §23's Security Invariants apply it to frame
// contents). Go's unit is bytes, matching the main client's
// MaxErrorMessageBytes.
const maxErrorMessageBytes = 500

// truncateErrorText bounds s to maxErrorMessageBytes with §9's truncation
// semantics: over the limit, the last 3 units become "...", so the result is
// at most the limit. Byte-level truncation can split a codepoint; §9 accepts
// that explicitly for Go.
func truncateErrorText(s string) string {
	if len(s) <= maxErrorMessageBytes {
		return s
	}
	return s[:maxErrorMessageBytes-3] + "..."
}

// usageError builds the usage-coded error construction-time validation
// surfaces (SPEC.md §23 "Consumer Surface"): zero wire attempts, never an
// iteration element.
func usageError(msg string) *TerminalError {
	return &TerminalError{Reason: ReasonUsage, Msg: msg}
}
