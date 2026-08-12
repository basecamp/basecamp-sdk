package eventfeed

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The Action Cable protocol layer (SPEC.md §23 "Cable Protocol Details",
// internal): frame envelope parsing and classification, subscribe/unsubscribe
// command marshaling, and the EventsChannel subscription identifier. The
// connector parses every frame; the transport moves bytes — that split is
// what makes the raw disconnect frame (and its reason) interceptable by
// contract. Everything here is pure functions over raw text frames; the run
// loop owns goroutines and state.

// Wire literals (class-1, SPEC.md §23 "Provenance").
const (
	frameTypeWelcome    = "welcome"
	frameTypePing       = "ping"
	frameTypeConfirm    = "confirm_subscription"
	frameTypeReject     = "reject_subscription"
	frameTypeDisconnect = "disconnect"

	channelName = "EventsChannel"

	commandSubscribe   = "subscribe"
	commandUnsubscribe = "unsubscribe"
)

// frameKind classifies one inbound Action Cable text frame.
type frameKind int

const (
	// frameWelcome — {"type":"welcome"}: send subscribe.
	frameWelcome frameKind = iota + 1
	// framePing — {"type":"ping"} or {"type":"ping","message":<epoch>}:
	// liveness only.
	framePing
	// frameConfirm — {"type":"confirm_subscription","identifier":...}.
	frameConfirm
	// frameReject — {"type":"reject_subscription","identifier":...}: always
	// terminal.
	frameReject
	// frameDisconnect — {"type":"disconnect","reason":...,"reconnect":...}:
	// a TEXT frame, not a WebSocket close; dispatch is on the reason string.
	frameDisconnect
	// frameMessage — {"identifier":...,"message":<event payload>}: a
	// correlated broadcast (no "type" key on the wire).
	frameMessage
	// frameUnknown — parseable JSON object the connector doesn't recognize:
	// updates liveness, otherwise ignored.
	frameUnknown
)

// String returns the kind's name.
func (k frameKind) String() string {
	switch k {
	case frameWelcome:
		return frameTypeWelcome
	case framePing:
		return frameTypePing
	case frameConfirm:
		return frameTypeConfirm
	case frameReject:
		return frameTypeReject
	case frameDisconnect:
		return frameTypeDisconnect
	case frameMessage:
		return "message"
	case frameUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("frameKind(%d)", int(k))
	}
}

// frame is one parsed inbound Action Cable text frame.
type frame struct {
	// kind classifies the frame.
	kind frameKind
	// identifier is the frame's subscription identifier, verbatim
	// (confirm/reject/message). Correlation is exact string equality against
	// the connector's own identifier; frames carrying other identifiers are
	// ignored.
	identifier string
	// reason is the disconnect frame's reason string (disconnect only).
	reason string
	// reconnect is the disconnect frame's reconnect flag, tri-state: absent
	// is not false (disconnect only).
	reconnect *bool
	// message is the correlated broadcast's payload, verbatim (message
	// only). Decoding is a separate, post-correlation step
	// (decodeMessageEvent) because only a CORRELATED message frame's decode
	// failure is an invalid-frame violation.
	message json.RawMessage
}

// Invalid-frame shapes the connector detects (SPEC.md §23 "Cable Protocol
// Details"). The class's third shape — an over-limit frame — binds inside the
// transport (the max_frame_bytes dial parameter) and never reaches the codec.
const (
	invalidFrameParse       = "frame_parse"
	invalidFrameEventDecode = "event_decode"
)

// invalidFrameError is a §23 invalid-frame-class violation the connector
// itself detects: an inbound frame that fails to parse as a frame envelope,
// or a correlated message frame whose payload fails to decode as an Event.
// One disposition: a peer protocol violation dispatched as a SOCKET FAILURE —
// full teardown through the current state's socket-failure edge, never
// terminal (a garbled frame is transport-level corruption, unlike the
// server's own invalid_event_stream_command verdict), never a silent skip.
//
// The type is deliberately FLAT — the rule for every error in this package
// that can carry frame-derived, URL-derived, or ticket-adjacent text, of
// which redactDialErr is the precedent. The connector never renders frame
// contents itself, but a decoder underneath it can: encoding/json's own
// errors carry offsets and type names only, while a type's UnmarshalJSON can
// quote the offending input — time.Time's does, so an attacker-chosen
// created_at reaches Observer.Disconnected at frame scale. The message is
// therefore composed and bounded ONCE, at construction, by §9's
// MAX_ERROR_MESSAGE_LENGTH; retaining the decoder's error as a cause would
// hand the unbounded original straight back through errors.Unwrap and undo
// the bound the rendering applied. Typed-kind matching is unaffected:
// errors.As matches on this type, which is what carries the classification.
type invalidFrameError struct {
	// shape names the violation shape (invalidFrameParse /
	// invalidFrameEventDecode).
	shape string
	// msg is the fully composed, bounded rendering, fixed at construction.
	msg string
}

// newInvalidFrameError composes the bounded rendering, dropping the cause.
func newInvalidFrameError(shape string, cause error) *invalidFrameError {
	msg := "event feed invalid inbound frame (" + shape + ")"
	if cause != nil {
		msg += ": " + cause.Error()
	}
	return &invalidFrameError{shape: shape, msg: truncateErrorText(msg)}
}

// Error implements the error interface. It renders the message composed at
// construction; there is deliberately no Unwrap (see the type's doc).
func (e *invalidFrameError) Error() string {
	return e.msg
}

// parseFrame parses and classifies one raw inbound text frame. Dispatch is on
// the "type" key; a type the connector doesn't recognize is frameUnknown
// (liveness only). A frame with no type is a correlated broadcast when it
// carries both "identifier" and "message", else frameUnknown. A frame that
// fails to parse as a JSON object envelope — unparseable bytes, non-object
// JSON, wrong-typed envelope fields — is the invalid-frame class's parse
// shape.
func parseFrame(data []byte) (frame, error) {
	var env struct {
		Type       *string         `json:"type"`
		Identifier string          `json:"identifier"`
		Message    json.RawMessage `json:"message"`
		Reason     string          `json:"reason"`
		Reconnect  *bool           `json:"reconnect"`
	}
	if !isJSONObject(data) {
		// Non-object JSON is the parse shape by the same reasoning as
		// unparseable bytes — the frame stream has stopped meaning anything —
		// but only `null` needs saying: arrays, strings and numbers already
		// fail the unmarshal below, while `null` unmarshals into the envelope
		// struct WITHOUT error and would classify as frameUnknown. A peer
		// sending nothing but `null` frames would then hold the socket open
		// indefinitely (the pump re-arms staleness before parsing) while
		// delivering no protocol traffic at all.
		return frame{}, newInvalidFrameError(invalidFrameParse, errors.New("frame is not a JSON object"))
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return frame{}, newInvalidFrameError(invalidFrameParse, err)
	}
	if env.Type != nil {
		switch *env.Type {
		case frameTypeWelcome:
			return frame{kind: frameWelcome}, nil
		case frameTypePing:
			return frame{kind: framePing}, nil
		case frameTypeConfirm:
			return frame{kind: frameConfirm, identifier: env.Identifier}, nil
		case frameTypeReject:
			return frame{kind: frameReject, identifier: env.Identifier}, nil
		case frameTypeDisconnect:
			return frame{kind: frameDisconnect, reason: env.Reason, reconnect: env.Reconnect}, nil
		default:
			return frame{kind: frameUnknown}, nil
		}
	}
	if env.Identifier != "" && len(env.Message) > 0 {
		return frame{kind: frameMessage, identifier: env.Identifier, message: env.Message}, nil
	}
	return frame{kind: frameUnknown}, nil
}

// isJSONObject reports whether data's first non-whitespace byte opens a JSON
// object. Paired with the unmarshal that follows — which rejects trailing
// content — it is exactly "data is a JSON object": RFC 8259 whitespace is the
// four bytes below, and a value's type is decided by its first byte.
func isJSONObject(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

// decodeMessageEvent decodes a correlated message frame's payload as an
// Event. The eight always-present keys are required with correct types; a
// missing required key or a wrong-typed value is the invalid-frame class's
// decode shape. visible_to_clients is presence-bearing, never required —
// absent decodes as a nil pointer, not a violation.
func decodeMessageEvent(raw json.RawMessage) (Event, error) {
	var p struct {
		ID               *int64     `json:"id"`
		Kind             *string    `json:"kind"`
		EventType        *string    `json:"event_type"`
		Action           *string    `json:"action"`
		CreatedAt        *time.Time `json:"created_at"`
		BucketID         *int64     `json:"bucket_id"`
		CreatorID        *int64     `json:"creator_id"`
		RecordingID      *int64     `json:"recording_id"`
		VisibleToClients *bool      `json:"visible_to_clients"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Event{}, newInvalidFrameError(invalidFrameEventDecode, err)
	}
	for _, req := range []struct {
		key     string
		present bool
	}{
		{"id", p.ID != nil},
		{"kind", p.Kind != nil},
		{"event_type", p.EventType != nil},
		{"action", p.Action != nil},
		{"created_at", p.CreatedAt != nil},
		{"bucket_id", p.BucketID != nil},
		{"creator_id", p.CreatorID != nil},
		{"recording_id", p.RecordingID != nil},
	} {
		if !req.present {
			return Event{}, newInvalidFrameError(invalidFrameEventDecode,
				fmt.Errorf("event payload missing required key %q", req.key))
		}
	}
	return Event{
		ID:               *p.ID,
		Kind:             *p.Kind,
		EventType:        *p.EventType,
		Action:           *p.Action,
		CreatedAt:        *p.CreatedAt,
		BucketID:         *p.BucketID,
		CreatorID:        *p.CreatorID,
		RecordingID:      *p.RecordingID,
		VisibleToClients: p.VisibleToClients,
	}, nil
}

// subscribeIdentifier builds the EventsChannel subscription identifier: the
// JSON encoding of an ORDERED object
// {"channel":"EventsChannel"[,"types":"a,b"][,"buckets":"1,2"][,"creators":"3"]}
// — fixed key order, comma-joined values in configured order, absent filters
// omitted. Hand-built rather than map-marshaled so any retransmit is
// byte-identical: the server absorbs identical resubscribes and rejects
// different ones. Built once per connection by the run loop; quote/comma/
// whitespace-hostile inputs were already rejected by Filters.Validate, so
// the minimal RFC 8259 escaping is a no-op in practice but correct by
// construction.
func subscribeIdentifier(f Filters) string {
	var b strings.Builder
	b.WriteString(`{"channel":`)
	writeJSONString(&b, channelName)
	if len(f.Types) > 0 {
		b.WriteString(`,"types":`)
		writeJSONString(&b, strings.Join(f.Types, ","))
	}
	if len(f.Buckets) > 0 {
		b.WriteString(`,"buckets":`)
		writeJSONString(&b, joinIDs(f.Buckets))
	}
	if len(f.Creators) > 0 {
		b.WriteString(`,"creators":`)
		writeJSONString(&b, joinIDs(f.Creators))
	}
	b.WriteByte('}')
	return b.String()
}

// subscribeCommand marshals the subscribe command as an exact byte string:
// {"command":"subscribe","identifier":"<json-escaped identifier>"}.
func subscribeCommand(identifier string) []byte {
	return cableCommand(commandSubscribe, identifier)
}

// unsubscribeCommand marshals the unsubscribe command:
// {"command":"unsubscribe","identifier":"<json-escaped identifier>"}.
func unsubscribeCommand(identifier string) []byte {
	return cableCommand(commandUnsubscribe, identifier)
}

// cableCommand hand-builds one Action Cable command frame. The identifier is
// embedded as a JSON string with the same minimal RFC 8259 escaping the
// digest uses — no language's default JSON emitter is load-bearing.
func cableCommand(command, identifier string) []byte {
	var b strings.Builder
	b.WriteString(`{"command":`)
	writeJSONString(&b, command)
	b.WriteString(`,"identifier":`)
	writeJSONString(&b, identifier)
	b.WriteByte('}')
	return []byte(b.String())
}

// joinIDs comma-joins ids in configured order, canonical base-10 rendering.
func joinIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}
