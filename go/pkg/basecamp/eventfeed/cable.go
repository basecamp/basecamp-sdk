package eventfeed

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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
// The type is deliberately FLAT and its rendering deliberately CLOSED — the
// rule for every error in this package that can carry frame-derived,
// URL-derived, or ticket-adjacent text, of which redactDialErr is the
// precedent. The connector never renders frame contents itself, but a decoder
// underneath it can: encoding/json's own errors carry offsets, kind words and
// type names only, while a type's UnmarshalJSON can quote the offending input
// — time.Time's does, verbatim — so an attacker-chosen created_at on a
// correlated broadcast reached Observer.Disconnected, a surface whose
// destination the connector knows nothing about.
//
// Bounding that text by §9's MAX_ERROR_MESSAGE_LENGTH, which is what this type
// used to do, is not redacting it: a 500-byte cap still forwards ~430 bytes of
// peer-chosen text. So the cause is dropped from the RENDERING as well as from
// the chain, and the message is a function of `shape` alone — a closed
// two-value vocabulary, fixed in this file. Nothing peer-derived can reach it,
// which is also why no truncation is applied here any more: there is no longer
// an unbounded input to bound.
//
// The trade is diagnostic detail: an operator sees "event_decode", not which
// key was missing or which value failed to parse. That is the intended
// direction — the same "less diagnostic, never leaks" failure direction
// redactURL commits to — and the shape is what decides disposition anyway.
//
// Typed-kind matching is unaffected: errors.As matches on this type, which is
// what carries the classification.
type invalidFrameError struct {
	// shape names the violation shape (invalidFrameParse /
	// invalidFrameEventDecode), and is the whole of the rendering.
	shape string
}

// newInvalidFrameError builds the error for shape. It deliberately takes NO
// cause: a cause parameter is the leak, whether it is rendered or merely
// retained, and a signature that cannot accept one cannot regrow it at a call
// site added later.
func newInvalidFrameError(shape string) *invalidFrameError {
	return &invalidFrameError{shape: shape}
}

// Error implements the error interface. There is deliberately no Unwrap, and
// deliberately nothing here but the shape (see the type's doc).
func (e *invalidFrameError) Error() string {
	return "event feed invalid inbound frame (" + e.shape + ")"
}

// parseFrame parses and classifies one raw inbound text frame. Dispatch is on
// the "type" key; a type the connector doesn't recognize — including a present
// but null one, which names no type to recognize — is frameUnknown (liveness
// only). A frame with NO type key is a correlated broadcast when it carries
// both "identifier" and "message", else frameUnknown. A frame that
// fails to parse as a JSON object envelope — unparseable bytes, non-object
// JSON, a wrong-typed "type", or a wrong-typed field the SELECTED frame kind
// reads — is the invalid-frame class's parse shape.
//
// The type is decoded FIRST, and only the fields the selected kind actually
// uses are decoded after it. Validating the whole envelope up front made a
// forward-compatible extension fatal: SPEC.md §23 says "Unknown frame types —
// parseable JSON whose `type` the connector doesn't recognize — update
// liveness and are otherwise ignored", but `{"type":"future","identifier":1}`
// failed the envelope unmarshal on a field `future` does not have and was
// dispatched as a socket failure. A server that adds such a frame would then
// tear down and reconnect every connected client, indefinitely, over a frame
// the protocol says to ignore. Selecting first is also what keeps the
// wrong-typed cases that DO matter fatal: `confirm_subscription` and
// `reject_subscription` read `identifier`, `disconnect` reads `reason` and
// `reconnect`, and a typeless frame reads both correlation fields, so each is
// still validated against the kind that consumes it.
func parseFrame(data []byte) (frame, error) {
	// Gated BEFORE any unmarshal: encoding/json does not refuse invalid
	// UTF-8, it silently swaps each invalid sequence for U+FFFD. Ungated, a
	// frame whose "type" carried a mangled byte decoded as an UNKNOWN type
	// and was ignored as liveness, and a correlated payload's fields could
	// mutate in place — the filestore refuses the same silent-mutation class
	// on load. Mutated bytes are not a dialect; the frame stream has stopped
	// meaning anything, which is the parse shape.
	if !utf8.Valid(data) {
		return frame{}, newInvalidFrameError(invalidFrameParse)
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
		return frame{}, newInvalidFrameError(invalidFrameParse)
	}
	// The type key's PRESENCE is decoded separately from its value: a `*string`
	// gives the same nil for an absent key and for a JSON `null`, which put one
	// wire value in two classes depending on its siblings — `{"type":null}`
	// alone fell through to frameUnknown, while
	// `{"type":null,"identifier":…,"message":…}` fell through to the
	// correlated-broadcast shape below and was DELIVERED as an event. A raw
	// message distinguishes them, so presence is decided here and the value
	// below.
	var envType struct {
		Type json.RawMessage `json:"type"`
	}
	if err := json.Unmarshal(data, &envType); err != nil {
		return frame{}, newInvalidFrameError(invalidFrameParse)
	}
	if len(envType.Type) > 0 {
		// The key is present. A wrong-typed value (a number, an array) fails
		// this unmarshal and is the parse shape, as it always was; `null`
		// decodes to a nil pointer WITHOUT error and is the unrecognized-type
		// case — a type key carrying no type to recognize is liveness-only, not
		// invalid, so a frame §23 says to ignore never tears the socket down.
		// What it is NOT is a broadcast: that shape is "no type key at all".
		var typ *string
		if err := json.Unmarshal(envType.Type, &typ); err != nil {
			return frame{}, newInvalidFrameError(invalidFrameParse)
		}
		if typ == nil {
			return frame{kind: frameUnknown}, nil
		}
		switch *typ {
		case frameTypeWelcome:
			return frame{kind: frameWelcome}, nil
		case frameTypePing:
			// The optional epoch `message` is never read (§23: both
			// {"type":"ping"} and {"type":"ping","message":<epoch>} are
			// accepted), so it is never validated either.
			return frame{kind: framePing}, nil
		case frameTypeConfirm, frameTypeReject:
			var env struct {
				Identifier string `json:"identifier"`
			}
			if err := json.Unmarshal(data, &env); err != nil {
				return frame{}, newInvalidFrameError(invalidFrameParse)
			}
			kind := frameConfirm
			if *typ == frameTypeReject {
				kind = frameReject
			}
			return frame{kind: kind, identifier: env.Identifier}, nil
		case frameTypeDisconnect:
			var env struct {
				Reason    string `json:"reason"`
				Reconnect *bool  `json:"reconnect"`
			}
			if err := json.Unmarshal(data, &env); err != nil {
				return frame{}, newInvalidFrameError(invalidFrameParse)
			}
			return frame{kind: frameDisconnect, reason: env.Reason, reconnect: env.Reconnect}, nil
		default:
			return frame{kind: frameUnknown}, nil
		}
	}
	// No "type" KEY at all: this is the correlated-broadcast shape, which reads
	// BOTH fields, so both are validated. It is not §23's unrecognized-type
	// case — there is no type to be unrecognized — and a wrong-typed
	// correlation field here still stops the frame stream meaning anything.
	var env struct {
		Identifier string          `json:"identifier"`
		Message    json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return frame{}, newInvalidFrameError(invalidFrameParse)
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
// Event. All NINE push-payload keys are required with correct types; a
// missing required key or a wrong-typed value is the invalid-frame class's
// decode shape.
//
// visible_to_clients is required here and only here. It is presence-bearing
// on the Event — absent ≠ false, which is why it is a *bool — and this
// decoder sees push payloads exclusively, where §23's presence asymmetry says
// the key is always carried ("push payloads carry it, poll rows omit it";
// conformance/event-feed/schema.json requires all 9 keys on pushEvent and
// forbids the key outright on pollEvent). A push frame that omits it, or
// sends JSON null, has erased the distinction the pointer exists to carry:
// the decoded Event would be indistinguishable from a poll row. That is a
// peer protocol violation, so it takes the invalid-frame class's
// socket-failure path rather than being delivered.
//
// The poll lane is untouched by this: poll rows never reach this function —
// they arrive through the PollSource seam as Events — and the plain
// encoding/json decoding of an Event keeps its 8-key tolerance.
func decodeMessageEvent(raw json.RawMessage) (Event, error) {
	// parseFrame's gate already covers a payload sliced from a gated frame;
	// this one binds for any other caller, so the function is total on its
	// own contract: no decoder-mutated routing field ever leaves it.
	if !utf8.Valid(raw) {
		return Event{}, newInvalidFrameError(invalidFrameEventDecode)
	}
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
		return Event{}, newInvalidFrameError(invalidFrameEventDecode)
	}
	// The nine required keys, in conformance/event-feed/schema.json's pushEvent
	// order. The key names ride as comments rather than as data because the
	// error deliberately cannot name the offender (see invalidFrameError): a
	// string field carried here would be a value with no reader, which is
	// exactly how a rendering regrows one.
	for _, present := range []bool{
		p.ID != nil,               // id
		p.Kind != nil,             // kind
		p.EventType != nil,        // event_type
		p.Action != nil,           // action
		p.CreatedAt != nil,        // created_at
		p.BucketID != nil,         // bucket_id
		p.CreatorID != nil,        // creator_id
		p.RecordingID != nil,      // recording_id
		p.VisibleToClients != nil, // visible_to_clients
	} {
		if !present {
			return Event{}, newInvalidFrameError(invalidFrameEventDecode)
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
