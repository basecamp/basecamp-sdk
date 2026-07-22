package oauth

import (
	"github.com/basecamp/basecamp-sdk/go/pkg/basecamp"
)

// DeviceFlowReason identifies why a device flow terminated. The parent
// basecamp.Error category is DERIVED from the reason (SPEC.md §16), so callers
// can branch on the precise reason or on the coarse taxonomy code.
type DeviceFlowReason string

const (
	// DeviceFlowAccessDenied — the user declined the authorization → auth.
	DeviceFlowAccessDenied DeviceFlowReason = "access_denied"
	// DeviceFlowExpired — the device/user code expired before approval → auth.
	DeviceFlowExpired DeviceFlowReason = "expired"
	// DeviceFlowTransport — a network failure ended the flow → network (retryable).
	DeviceFlowTransport DeviceFlowReason = "transport"
	// DeviceFlowUnavailable — the selected AS cannot do device flow → validation.
	DeviceFlowUnavailable DeviceFlowReason = "unavailable"
	// DeviceFlowCancelled — the caller cancelled the flow → native ctx.Err (usage
	// taxonomy code).
	DeviceFlowCancelled DeviceFlowReason = "cancelled"
)

// DeviceFlowError is a terminal device-flow error carrying a DeviceFlowReason.
// Its parent taxonomy category is derived from the reason; for a cancelled flow
// the underlying ctx error (Err) is exposed so errors.Is against
// context.Canceled / context.DeadlineExceeded matches natively.
type DeviceFlowError struct {
	// Reason identifies the termination class.
	Reason DeviceFlowReason
	// Err is the underlying cause, if any (for cancelled: ctx.Err()).
	Err error
}

// Error implements the error interface. The message is derived from the reason.
func (e *DeviceFlowError) Error() string {
	msg := messageForReason(e.Reason)
	if e.Err != nil {
		return msg + ": " + e.Err.Error()
	}
	return msg
}

// Unwrap exposes the derived taxonomy-coded *basecamp.Error (and, through it,
// the underlying cause) for errors.Is / errors.As traversal. Matching a
// *basecamp.Error yields the parent category; matching context.Canceled reaches
// the cancellation cause.
func (e *DeviceFlowError) Unwrap() error {
	return e.category()
}

// Code returns the parent basecamp taxonomy code derived from the reason.
func (e *DeviceFlowError) Code() string {
	return e.category().Code
}

// Retryable reports whether the failure is retryable (transport only).
func (e *DeviceFlowError) Retryable() bool {
	return e.category().Retryable
}

// category maps the reason to its parent taxonomy-coded *basecamp.Error, carrying
// the cause so downstream Unwrap traversal reaches it.
func (e *DeviceFlowError) category() *basecamp.Error {
	switch e.Reason {
	case DeviceFlowAccessDenied, DeviceFlowExpired:
		return &basecamp.Error{Code: basecamp.CodeAuth, Message: e.Error(), Cause: e.Err}
	case DeviceFlowTransport:
		return &basecamp.Error{Code: basecamp.CodeNetwork, Message: e.Error(), Retryable: true, Cause: e.Err}
	case DeviceFlowUnavailable:
		return &basecamp.Error{Code: basecamp.CodeValidation, Message: e.Error(), Cause: e.Err}
	case DeviceFlowCancelled:
		return &basecamp.Error{Code: basecamp.CodeUsage, Message: e.Error(), Cause: e.Err}
	default:
		return &basecamp.Error{Code: basecamp.CodeAPI, Message: e.Error(), Cause: e.Err}
	}
}

func messageForReason(reason DeviceFlowReason) string {
	switch reason {
	case DeviceFlowAccessDenied:
		return "the authorization request was denied"
	case DeviceFlowExpired:
		return "device code expired before authorization completed"
	case DeviceFlowTransport:
		return "device flow transport failure"
	case DeviceFlowUnavailable:
		return "the selected authorization server does not support the device authorization grant"
	case DeviceFlowCancelled:
		return "device flow cancelled"
	default:
		return "device flow failed"
	}
}
