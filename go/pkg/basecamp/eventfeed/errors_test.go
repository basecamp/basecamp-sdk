package eventfeed

import (
	"errors"
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

func TestCloseError_Message(t *testing.T) {
	withReason := &CloseError{Code: 1008, Reason: "policy violation"}
	if got := withReason.Error(); got != "cable connection closed by peer: code 1008: policy violation" {
		t.Errorf("Error() = %q", got)
	}
	bare := &CloseError{Code: 1006}
	if got := bare.Error(); got != "cable connection closed by peer: code 1006" {
		t.Errorf("Error() = %q", got)
	}
}
