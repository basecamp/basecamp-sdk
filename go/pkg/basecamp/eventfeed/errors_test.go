package eventfeed

import (
	"errors"
	"strings"
	"testing"
)

func TestTerminalReason_WireStrings(t *testing.T) {
	// The reason strings are SPEC.md §23's terminal taxonomy, verbatim —
	// cross-language contract values, never paraphrased. auth_revoked is
	// deliberately reserved, not defined.
	want := map[TerminalReason]string{
		ReasonSubscriptionRejected: "subscription_rejected",
		ReasonProtocolFatal:        "protocol_fatal",
		ReasonFilterInvalid:        "filter_invalid",
		ReasonAuthorizationFailed:  "authorization_failed",
		ReasonCheckpointLoad:       "checkpoint_load",
		ReasonUsage:                "usage",
		ReasonBufferOverflow:       "buffer_overflow",
		ReasonFeedGap:              "feed_gap",
		ReasonInvalidContinuation:  "invalid_continuation",
		ReasonPollFailed:           "poll_failed",
		ReasonMintFailed:           "mint_failed",
		ReasonInvalidCableURL:      "invalid_cable_url",
	}
	if len(want) != 12 {
		t.Fatalf("taxonomy carries %d reasons, want 12", len(want))
	}
	for reason, s := range want {
		if string(reason) != s {
			t.Errorf("reason = %q, want %q", string(reason), s)
		}
	}
}

func TestTerminalError_MessageComposition(t *testing.T) {
	cases := []struct {
		name string
		err  *TerminalError
		want string
	}{
		{
			"reason only",
			&TerminalError{Reason: ReasonProtocolFatal},
			"event feed terminal: protocol_fatal",
		},
		{
			"reason and msg",
			&TerminalError{Reason: ReasonFilterInvalid, Msg: "buckets contains an unknown id"},
			"event feed terminal: filter_invalid: buckets contains an unknown id",
		},
		{
			"reason, msg, and cause",
			&TerminalError{Reason: ReasonCheckpointLoad, Msg: "loading checkpoint", Err: errors.New("disk gone")},
			"event feed terminal: checkpoint_load: loading checkpoint: disk gone",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTerminalError_UnwrapReachesTheCause(t *testing.T) {
	cause := errors.New("generated operation failed")
	err := &TerminalError{Reason: ReasonPollFailed, Err: cause}
	if !errors.Is(err, cause) {
		t.Error("errors.Is(err, cause) = false, want true")
	}
	if (&TerminalError{Reason: ReasonUsage}).Unwrap() != nil {
		t.Error("Unwrap() with no cause != nil")
	}
}

func TestSeamErrorKind_Strings(t *testing.T) {
	mintKinds := map[MintErrorKind]string{
		MintTransient:     "transient",
		MintThrottled:     "throttled",
		MintUnauthorized:  "unauthorized",
		MintUnrecoverable: "unrecoverable",
	}
	for kind, s := range mintKinds {
		if kind.String() != s {
			t.Errorf("MintErrorKind.String() = %q, want %q", kind.String(), s)
		}
	}
	pollKinds := map[PollErrorKind]string{
		PollTransient:       "transient",
		PollThrottled:       "throttled",
		PollPositionInvalid: "position_invalid",
		PollFilterInvalid:   "filter_invalid",
		PollFilterChanged:   "filter_changed",
		PollGone:            "gone",
		PollUnauthorized:    "unauthorized",
		PollRedirectRefused: "redirect_refused",
		PollUnrecoverable:   "unrecoverable",
	}
	for kind, s := range pollKinds {
		if kind.String() != s {
			t.Errorf("PollErrorKind.String() = %q, want %q", kind.String(), s)
		}
	}
	dialKinds := map[DialErrorKind]string{
		DialTransient: "transient",
		DialPolicy:    "policy",
	}
	for kind, s := range dialKinds {
		if kind.String() != s {
			t.Errorf("DialErrorKind.String() = %q, want %q", kind.String(), s)
		}
	}
}

func TestSeamErrors_UnwrapReachesTheCause(t *testing.T) {
	cause := errors.New("boom")
	for _, err := range []error{
		&MintError{Kind: MintTransient, Err: cause},
		&PollError{Kind: PollUnrecoverable, Err: cause},
		&DialError{Kind: DialTransient, Err: cause},
	} {
		if !errors.Is(err, cause) {
			t.Errorf("errors.Is(%T, cause) = false, want true", err)
		}
	}
}

// TestCloseError_MessageWithholdsThePeerReason inverts what this test asserted.
// It previously required Error() to RENDER Reason, bounded by §9's cap; that
// pinned the wrong contract, so nothing short of inverting it is honest.
//
// Reason is peer-supplied and this error reaches Observer.Disconnected, which
// hosts log. The cable server is precisely the party that knows the ticket — it
// was dialed with it — so a server echoing the URL it was dialed with puts the
// ticket in the host's logs, and §23 says the ticket is never logged. Bounding
// limits how much of a credential escapes, not whether any does.
func TestCloseError_MessageWithholdsThePeerReason(t *testing.T) {
	const canary = "sekrit-ticket-value"
	withReason := &CloseError{Code: 1008, Reason: "closing wss://x/cable?ticket=" + canary}
	got := withReason.Error()
	if got != "cable connection closed by peer: code 1008 (peer reason withheld)" {
		t.Errorf("Error() = %q, want the fixed withheld marker", got)
	}
	if strings.Contains(got, canary) || strings.Contains(got, "ticket=") {
		t.Errorf("Error() echoed peer text: %q", got)
	}
	// The code survives: an integer cannot carry a credential, and RFC 6455
	// close codes are what an operator classifies on.
	bare := &CloseError{Code: 1006}
	if got := bare.Error(); got != "cable connection closed by peer: code 1006" {
		t.Errorf("Error() = %q", got)
	}
	// Reason remains a FIELD, so a host that has decided its server is
	// trustworthy can read it deliberately.
	if withReason.Reason == "" {
		t.Error("Reason must remain readable on the struct")
	}
}

// TestCloseError_RenderingWithholdsPeerReason pins the last rendering in the
// value-invariant class: the peer chooses its close reason and can reflect
// the mint ticket into it, and RFC 6455's 123-byte reason cap fits a ticket
// well inside §9's 500-byte truncation — bounding is not redacting. The
// canary is deliberately SHORT for the same reason frameCanary is: a marker
// that survives truncation is what proves the absence of concatenation
// rather than the presence of the cap. The type must also stay flat, or a
// chain walk recovers what the rendering withheld.
func TestCloseError_RenderingWithholdsPeerReason(t *testing.T) {
	const canary = "sekrit-ticket"
	e := &CloseError{Code: 1008, Reason: "refused ?ticket=" + canary}
	if strings.Contains(e.Error(), canary) {
		t.Errorf("Error() = %q echoes the peer's reason", e.Error())
	}
	if !strings.Contains(e.Error(), "1008") {
		t.Errorf("Error() = %q lost the structural close code", e.Error())
	}
	if errors.Unwrap(e) != nil {
		t.Errorf("CloseError unwraps to %v; the type must stay flat", errors.Unwrap(e))
	}
}

// TestDialError_MessageIsBounded pins §9's MAX_ERROR_MESSAGE_LENGTH on
// DialError renderings, CloseError-style. checkCableURL's own reasons are a
// closed vocabulary now, so the cap binds on what the TYPE can carry — a
// custom CableTransport composes its own DialError, and nothing bounds the
// Reason or cause it chooses.
func TestDialError_MessageIsBounded(t *testing.T) {
	derr := &DialError{Kind: DialPolicy, Reason: strings.Repeat("a", 2*maxErrorMessageBytes)}
	msg := derr.Error()
	if len(msg) > maxErrorMessageBytes {
		t.Errorf("Error() is %d bytes, want at most %d", len(msg), maxErrorMessageBytes)
	}
	if !strings.HasSuffix(msg, "...") {
		t.Errorf("Error() ends %q, want §9's truncation marker", msg[max(0, len(msg)-16):])
	}
}

// TestSeamErrors_RenderingsAreBounded closes the class the DialError case
// above opened: every seam error that composes server-derived text —
// TerminalError (the poll lane's filter-invalid message is preserved
// verbatim per §23), MintError (a TicketMinter's cause), PollError (the
// server's message verbatim) — renders under §9's cap.
// checkContinuation's rejections no longer appear here because they no
// longer carry an unbounded input at all: the messages are a closed
// vocabulary, pinned by TestCheckContinuation_RejectionsNeverEchoTheURL.
func TestSeamErrors_RenderingsAreBounded(t *testing.T) {
	long := strings.Repeat("a", 2*maxErrorMessageBytes)
	for _, err := range []error{
		&TerminalError{Reason: ReasonFilterInvalid, Msg: long},
		&MintError{Kind: MintUnrecoverable, Err: errors.New(long)},
		&PollError{Kind: PollFilterInvalid, Msg: long},
	} {
		msg := err.Error()
		if len(msg) > maxErrorMessageBytes {
			t.Errorf("%T renders %d bytes, want at most %d", err, len(msg), maxErrorMessageBytes)
		}
		if !strings.HasSuffix(msg, "...") {
			t.Errorf("%T rendering ends %q, want §9's truncation marker", err, msg[max(0, len(msg)-16):])
		}
	}
}

// TestPollError_RenderingOmitsLocationOrigin pins the data-not-rendering
// line on the redirect field: a hostile redirect can reflect the caller's
// bearer into a host label, so the origin is readable data on the error and
// never part of any rendering.
func TestPollError_RenderingOmitsLocationOrigin(t *testing.T) {
	e := &PollError{Kind: PollRedirectRefused, LocationOrigin: "https://sekrit-bearer.invalid"}
	if strings.Contains(e.Error(), "sekrit-bearer") {
		t.Errorf("Error() = %q renders LocationOrigin", e.Error())
	}
}
