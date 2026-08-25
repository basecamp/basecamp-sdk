package eventfeed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParseFrame_Welcome(t *testing.T) {
	f, err := parseFrame([]byte(`{"type":"welcome"}`))
	if err != nil {
		t.Fatalf("parseFrame(welcome): %v", err)
	}
	if f.kind != frameWelcome {
		t.Errorf("kind = %v, want %v", f.kind, frameWelcome)
	}
}

func TestParseFrame_PingBothForms(t *testing.T) {
	// SPEC §23 "Cable Protocol Details": ping parsing accepts both
	// {"type":"ping"} and the stock epoch-bearing form.
	for _, raw := range []string{
		`{"type":"ping"}`,
		`{"type":"ping","message":1754306400}`,
	} {
		f, err := parseFrame([]byte(raw))
		if err != nil {
			t.Fatalf("parseFrame(%s): %v", raw, err)
		}
		if f.kind != framePing {
			t.Errorf("parseFrame(%s) kind = %v, want %v", raw, f.kind, framePing)
		}
	}
}

func TestParseFrame_ConfirmSubscription(t *testing.T) {
	id := `{"channel":"EventsChannel","types":"message.created"}`
	raw := []byte(`{"identifier":"{\"channel\":\"EventsChannel\",\"types\":\"message.created\"}","type":"confirm_subscription"}`)
	f, err := parseFrame(raw)
	if err != nil {
		t.Fatalf("parseFrame(confirm): %v", err)
	}
	if f.kind != frameConfirm {
		t.Errorf("kind = %v, want %v", f.kind, frameConfirm)
	}
	// Correlation is exact string equality against the connector's own
	// identifier, so the extracted identifier must be verbatim.
	if f.identifier != id {
		t.Errorf("identifier = %q, want %q", f.identifier, id)
	}
}

func TestParseFrame_RejectSubscription(t *testing.T) {
	raw := []byte(`{"identifier":"{\"channel\":\"EventsChannel\"}","type":"reject_subscription"}`)
	f, err := parseFrame(raw)
	if err != nil {
		t.Fatalf("parseFrame(reject): %v", err)
	}
	if f.kind != frameReject {
		t.Errorf("kind = %v, want %v", f.kind, frameReject)
	}
	if want := `{"channel":"EventsChannel"}`; f.identifier != want {
		t.Errorf("identifier = %q, want %q", f.identifier, want)
	}
}

func TestParseFrame_DisconnectMatrix(t *testing.T) {
	// The §23 disconnect matrix's frames, plus the unknown-reason and
	// absent-reconnect cases. reconnect is tri-state: absent is not false.
	boolPtr := func(v bool) *bool { return &v }
	cases := []struct {
		name      string
		raw       string
		reason    string
		reconnect *bool
	}{
		{"unauthorized", `{"type":"disconnect","reason":"unauthorized","reconnect":false}`, "unauthorized", boolPtr(false)},
		{"remote", `{"type":"disconnect","reason":"remote","reconnect":true}`, "remote", boolPtr(true)},
		{"protocol fatal", `{"type":"disconnect","reason":"invalid_event_stream_command","reconnect":false}`, "invalid_event_stream_command", boolPtr(false)},
		{"unknown reason", `{"type":"disconnect","reason":"server_restart","reconnect":true}`, "server_restart", boolPtr(true)},
		{"absent reconnect", `{"type":"disconnect","reason":"remote"}`, "remote", nil},
		{"absent reason", `{"type":"disconnect"}`, "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parseFrame([]byte(tc.raw))
			if err != nil {
				t.Fatalf("parseFrame: %v", err)
			}
			if f.kind != frameDisconnect {
				t.Fatalf("kind = %v, want %v", f.kind, frameDisconnect)
			}
			if f.reason != tc.reason {
				t.Errorf("reason = %q, want %q", f.reason, tc.reason)
			}
			switch {
			case tc.reconnect == nil && f.reconnect != nil:
				t.Errorf("reconnect = %v, want absent", *f.reconnect)
			case tc.reconnect != nil && f.reconnect == nil:
				t.Errorf("reconnect absent, want %v", *tc.reconnect)
			case tc.reconnect != nil && *f.reconnect != *tc.reconnect:
				t.Errorf("reconnect = %v, want %v", *f.reconnect, *tc.reconnect)
			}
		})
	}
}

func TestParseFrame_MessageFrame(t *testing.T) {
	payload := `{"id":105,"kind":"message","event_type":"message.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"recording_id":900,"visible_to_clients":false}`
	raw := []byte(`{"identifier":"{\"channel\":\"EventsChannel\"}","message":` + payload + `}`)
	f, err := parseFrame(raw)
	if err != nil {
		t.Fatalf("parseFrame(message): %v", err)
	}
	if f.kind != frameMessage {
		t.Fatalf("kind = %v, want %v", f.kind, frameMessage)
	}
	if want := `{"channel":"EventsChannel"}`; f.identifier != want {
		t.Errorf("identifier = %q, want %q", f.identifier, want)
	}
	// The payload rides through verbatim; decoding is a separate, post-
	// correlation step.
	if !bytes.Equal(f.message, []byte(payload)) {
		t.Errorf("message = %s, want %s", f.message, payload)
	}
	ev, err := decodeMessageEvent(f.message)
	if err != nil {
		t.Fatalf("decodeMessageEvent: %v", err)
	}
	if ev.ID != 105 || ev.Kind != "message" || ev.EventType != "message.created" ||
		ev.Action != "created" || ev.BucketID != 2 || ev.CreatorID != 3 || ev.RecordingID != 900 {
		t.Errorf("decoded event = %+v", ev)
	}
	if want := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC); !ev.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", ev.CreatedAt, want)
	}
	if ev.VisibleToClients == nil {
		t.Error("VisibleToClients = nil, want present false (push rows carry it)")
	} else if *ev.VisibleToClients {
		t.Error("*VisibleToClients = true, want false")
	}
}

func TestParseFrame_UnknownTypesUpdateLivenessOnly(t *testing.T) {
	// Parseable JSON objects the connector doesn't recognize are frameUnknown:
	// liveness updates, otherwise ignored — never the invalid-frame class.
	for _, raw := range []string{
		`{"type":"telemetry","payload":{"x":1}}`,
		`{}`,
		`{"type":null}`,
		`{"identifier":"{\"channel\":\"EventsChannel\"}"}`, // no message key: not a broadcast
	} {
		f, err := parseFrame([]byte(raw))
		if err != nil {
			t.Fatalf("parseFrame(%s): %v", raw, err)
		}
		if f.kind != frameUnknown {
			t.Errorf("parseFrame(%s) kind = %v, want %v", raw, f.kind, frameUnknown)
		}
	}
}

func TestParseFrame_InvalidFrames(t *testing.T) {
	// Shape 1 of the §23 invalid-frame class: a frame that fails to parse as
	// a frame envelope. A malformed envelope (non-object JSON, wrong-typed
	// envelope fields) is the same peer protocol violation as unparseable
	// bytes: the frame stream has stopped meaning anything.
	cases := []struct {
		name string
		raw  string
	}{
		{"not json", `disconnect!`},
		{"empty", ``},
		{"truncated", `{"type":"wel`},
		{"top-level array", `[1,2,3]`},
		{"top-level string", `"welcome"`},
		{"wrong-typed type", `{"type":123}`},
		{"wrong-typed identifier", `{"identifier":42,"type":"confirm_subscription"}`},
		{"wrong-typed reconnect", `{"type":"disconnect","reason":"remote","reconnect":"yes"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseFrame([]byte(tc.raw))
			if err == nil {
				t.Fatal("parseFrame succeeded, want invalid-frame error")
			}
			var ife *invalidFrameError
			if !errors.As(err, &ife) {
				t.Fatalf("error = %T (%v), want *invalidFrameError", err, err)
			}
		})
	}
}

func TestParseFrame_NullIsInvalid(t *testing.T) {
	// `null` is the one non-object JSON that unmarshals into the envelope
	// struct WITHOUT error, so it must be rejected explicitly: classified as
	// frameUnknown it would be liveness-only, and a peer sending nothing but
	// `null` frames could hold the socket open indefinitely (the pump resets
	// staleness before the frame is parsed) while delivering no protocol
	// traffic at all.
	for _, raw := range []string{`null`, "  null\n", `NULL_NOT_JSON`} {
		f, err := parseFrame([]byte(raw))
		if err == nil {
			t.Fatalf("parseFrame(%q) = %v, want an invalid-frame error", raw, f.kind)
		}
		var ife *invalidFrameError
		if !errors.As(err, &ife) {
			t.Fatalf("parseFrame(%q) error = %T (%v), want *invalidFrameError", raw, err, err)
		}
		if ife.shape != invalidFrameParse {
			t.Errorf("parseFrame(%q) shape = %q, want %q", raw, ife.shape, invalidFrameParse)
		}
	}
}

// TestParseFrame_NullTypeIsNeverABroadcast pins the classification a nil
// *string could not express. `{"type":null}` is frameUnknown above —
// liveness-only — so adding the two correlation fields must not flip the SAME
// key's value into a delivered event. A `*string` type field decodes an ABSENT
// key and a JSON `null` to the same nil, which put one wire value in two
// classes depending on its siblings, and made the second class a delivery.
//
// The broadcast shape is "no type key at all" (§23: a correlated broadcast
// carries no "type" on the wire); a type key that is present but carries no
// type to recognize is the unrecognized-type case — liveness-only, never
// invalid, so a frame the protocol says to ignore never tears the socket down.
func TestParseFrame_NullTypeIsNeverABroadcast(t *testing.T) {
	payload := `{"id":105,"kind":"message","event_type":"message.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"recording_id":900,"visible_to_clients":false}`
	for _, raw := range []string{
		`{"type":null,"identifier":"{\"channel\":\"EventsChannel\"}","message":` + payload + `}`,
		// Key order must not decide it either.
		`{"identifier":"{\"channel\":\"EventsChannel\"}","message":` + payload + `,"type":null}`,
	} {
		f, err := parseFrame([]byte(raw))
		if err != nil {
			t.Fatalf("parseFrame(%s) = %v, want frameUnknown — a present-but-null type is liveness-only, not an invalid frame", raw, err)
		}
		if f.kind != frameUnknown {
			t.Errorf("parseFrame(%s) kind = %v, want %v", raw, f.kind, frameUnknown)
		}
		if f.identifier != "" {
			t.Errorf("parseFrame(%s) identifier = %q, want empty: an ignored frame correlates with nothing", raw, f.identifier)
		}
		if f.message != nil {
			t.Errorf("parseFrame(%s) message = %s, want nil: an ignored frame carries no deliverable payload", raw, f.message)
		}
	}
}

// frameCanary is a SHORT peer-supplied marker. Its shortness is the whole
// instrument: the predecessor of this test planted 4096 bytes and asserted the
// rendering neither exceeded §9's 500-byte cap nor contained the marker, both
// of which the cap alone made true. It therefore passed against code that
// concatenated the cause verbatim, which is precisely the leak it was written
// to catch. A marker that fits well inside the cap survives truncation, so
// only the absence of concatenation can make the assertion hold.
const frameCanary = "CANARY-8f3a"

// TestInvalidFrameErrorRendersShapeOnly: an invalid-frame error renders its
// SHAPE and nothing else (SPEC §23 "Security Invariants").
//
// The failure it forecloses is concrete. time.Time's UnmarshalJSON quotes its
// input verbatim — `parsing time "<peer bytes>" as "2006-01-02T15:04:05Z07:00"`
// — so a malformed created_at on a correlated broadcast put up to ~430 bytes
// of peer-chosen text (500-byte cap less the fixed prose) into whatever
// Observer.Disconnected forwards it to: a log, an error tracker, a metrics
// label. Bounding that text is not redacting it.
//
// The assertion is EXACT EQUALITY rather than containment, because a
// containment check is what failed here before: `!strings.Contains(rendered,
// marker)` is satisfiable by truncating the marker away, whereas equality with
// the shape rendering is satisfiable only by rendering the shape alone.
func TestInvalidFrameErrorRendersShapeOnly(t *testing.T) {
	for _, tc := range []struct {
		name      string
		err       error
		wantShape string
		// canaryReachable records whether the peer can steer this shape's
		// cause text at all. It can for the decode shape (time.ParseError);
		// it CANNOT for the parse shape, where every reachable cause is one of
		// encoding/json's own errors, and those quote a JSON kind word
		// ("number", "array"), a connector-authored struct field name, a Go
		// type name, or a single offending character — never a peer-chosen
		// value. Saying so here keeps the parse case from reading as an
		// absence proof it structurally cannot be: equality carries it.
		canaryReachable bool
	}{
		{
			name: "event decode shape",
			err: mustErr(t, func() error {
				_, err := decodeMessageEvent([]byte(`{"id":105,"kind":"message","event_type":"message.created","action":"created","created_at":"` +
					frameCanary + `","bucket_id":2,"creator_id":3,"recording_id":900,"visible_to_clients":false}`))
				return err
			}),
			wantShape:       invalidFrameEventDecode,
			canaryReachable: true,
		},
		{
			name: "event decode shape, missing key",
			err: mustErr(t, func() error {
				_, err := decodeMessageEvent([]byte(`{"id":105,"kind":"message","event_type":"message.created","action":"created","bucket_id":2,"creator_id":3,"recording_id":900,"visible_to_clients":false}`))
				return err
			}),
			wantShape: invalidFrameEventDecode,
		},
		{
			name: "parse shape, wrong-typed identifier",
			err: mustErr(t, func() error {
				_, err := parseFrame([]byte(`{"type":"confirm_subscription","identifier":["` + frameCanary + `"]}`))
				return err
			}),
			wantShape: invalidFrameParse,
		},
		{
			name: "parse shape, non-object",
			err: mustErr(t, func() error {
				_, err := parseFrame([]byte(`null`))
				return err
			}),
			wantShape: invalidFrameParse,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := "event feed invalid inbound frame (" + tc.wantShape + ")"
			if got := tc.err.Error(); got != want {
				t.Fatalf("rendered error = %q, want exactly %q", got, want)
			}
			if tc.canaryReachable && strings.Contains(tc.err.Error(), frameCanary) {
				t.Fatalf("the rendering embeds the frame-supplied value %q", frameCanary)
			}
			var ife *invalidFrameError
			if !errors.As(tc.err, &ife) {
				t.Fatalf("errors.As lost the invalid-frame classification of %T", tc.err)
			}
			if ife.shape != tc.wantShape {
				t.Fatalf("shape = %q, want %q", ife.shape, tc.wantShape)
			}
		})
	}
}

// TestInvalidFrameShapeVocabularyIsClosed is what makes the equality assertion
// above a bound on the whole class rather than on the four inputs it names:
// the rendering is a function of `shape` alone, so pinning the vocabulary
// pins every rendering the type can ever produce.
func TestInvalidFrameShapeVocabularyIsClosed(t *testing.T) {
	for _, shape := range []string{invalidFrameParse, invalidFrameEventDecode} {
		err := newInvalidFrameError(shape)
		want := "event feed invalid inbound frame (" + shape + ")"
		if got := err.Error(); got != want {
			t.Fatalf("newInvalidFrameError(%q) renders %q, want %q", shape, got, want)
		}
		if got := len(err.Error()); got > maxErrorMessageBytes {
			t.Fatalf("shape %q renders %d bytes, want at most %d", shape, got, maxErrorMessageBytes)
		}
	}
}

// TestFrameDerivedErrorsAreFlat is the RULE, stated once for the package: an
// error that carries frame-derived, URL-derived, or ticket-adjacent text is
// FLAT — it retains no raw cause an errors.Unwrap / errors.As walk can use to
// recover what the rendering bounded or redacted. redactDialErr is the
// precedent ("the rebuilt error is deliberately flat (no Unwrap): re-exposing
// the original chain would re-expose the unredacted URL"), and truncating only
// the outermost rendering is not a fix — the chain hands the original back.
//
// Typed-kind matching is unaffected: errors.As on the connector's own error
// types still matches, because it is the type that carries the classification
// and only the RAW CAUSE that is dropped.
func TestFrameDerivedErrorsAreFlat(t *testing.T) {
	// The canary is SHORT for the reason frameCanary documents: planted at
	// 4096 bytes, every assertion below is satisfied by the 500-byte cap
	// regardless of what the chain holds.
	raw := []byte(`{"id":105,"kind":"message","event_type":"message.created","action":"created","created_at":"` +
		frameCanary + `","bucket_id":2,"creator_id":3,"recording_id":900,"visible_to_clients":false}`)

	for _, tc := range []struct {
		name string
		err  error
	}{
		{"event decode shape", mustErr(t, func() error { _, err := decodeMessageEvent(raw); return err })},
		{"parse shape", mustErr(t, func() error {
			_, err := parseFrame([]byte(`{"type":"confirm_subscription","identifier":["` + frameCanary + `"]}`))
			return err
		})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			depth := 0
			for err := tc.err; err != nil; err = errors.Unwrap(err) {
				depth++
				if strings.Contains(err.Error(), frameCanary) {
					t.Fatalf("%T in the chain re-exposes the frame-supplied value", err)
				}
				if got := len(err.Error()); got > maxErrorMessageBytes {
					t.Fatalf("%T in the chain renders %d bytes, want at most %d", err, got, maxErrorMessageBytes)
				}
			}
			// Flat means flat: exactly one link. Walking a chain and finding
			// nothing is also what a one-link chain looks like, so the walk
			// above cannot distinguish "the cause was dropped" from "the walk
			// stopped early" without this.
			if depth != 1 {
				t.Fatalf("error chain depth = %d, want 1: a frame-derived error retains no cause", depth)
			}
			// The classification survives the flattening.
			var ife *invalidFrameError
			if !errors.As(tc.err, &ife) {
				t.Fatalf("errors.As lost the invalid-frame classification of %T", tc.err)
			}
		})
	}
}

func mustErr(t *testing.T, f func() error) error {
	t.Helper()
	err := f()
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}

// TestDecodeMessageEvent_RequiresVisibleToClients: decodeMessageEvent decodes
// PUSH payloads only, and conformance/event-feed/schema.json's `pushEvent`
// requires all nine keys ("Push-payload event: all 9 keys required, including
// visible_to_clients (presence-bearing; absent ≠ false)"). The asymmetry is
// the whole point — `pollEvent` requires eight and forbids the ninth
// outright — so a push frame that omits it, or sends JSON null, has broken
// the contract that makes absence meaningful and takes the invalid-frame
// class's socket-failure path. Accepting it would deliver an Event whose
// presence-bearing pointer is nil, i.e. a push row indistinguishable from a
// poll row.
func TestDecodeMessageEvent_RequiresVisibleToClients(t *testing.T) {
	const eightKeys = `"id":7,"kind":"todo","event_type":"todo.completed","action":"completed","created_at":"2026-08-01T12:00:00Z","bucket_id":1,"creator_id":2,"recording_id":3`
	t.Run("absent", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`{`+eightKeys+`}`))
	})
	t.Run("explicit null", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`{`+eightKeys+`,"visible_to_clients":null}`))
	})
	// Present is still presence-bearing on the way out: a false must decode
	// to a non-nil pointer, never a defaulted boolean.
	for _, want := range []bool{false, true} {
		t.Run(fmt.Sprintf("present %v", want), func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{%s,"visible_to_clients":%v}`, eightKeys, want))
			ev, err := decodeMessageEvent(raw)
			if err != nil {
				t.Fatalf("decodeMessageEvent: %v", err)
			}
			if ev.VisibleToClients == nil || *ev.VisibleToClients != want {
				t.Fatalf("VisibleToClients = %v, want a pointer to %v", ev.VisibleToClients, want)
			}
		})
	}
}

func TestDecodeMessageEvent_Failures(t *testing.T) {
	// Shape 3 of the §23 invalid-frame class: a correlated message frame
	// whose payload fails to decode as an Event — a missing required key, a
	// wrong-typed id.
	full := map[string]any{
		"id": int64(105), "kind": "message", "event_type": "message.created",
		"action": "created", "created_at": "2026-08-01T12:00:00Z",
		"bucket_id": int64(2), "creator_id": int64(3), "recording_id": int64(900),
		"visible_to_clients": false,
	}
	requiredKeys := []string{
		"id", "kind", "event_type", "action", "created_at",
		"bucket_id", "creator_id", "recording_id", "visible_to_clients",
	}
	for _, missing := range requiredKeys {
		t.Run("missing "+missing, func(t *testing.T) {
			partial := make(map[string]any, len(full))
			for k, v := range full {
				if k != missing {
					partial[k] = v
				}
			}
			raw, err := json.Marshal(partial)
			if err != nil {
				t.Fatal(err)
			}
			assertEventDecodeFails(t, raw)
		})
	}
	// Both malformed-VALUE cases carry all nine required keys, the flagged
	// value included: with any key missing, the presence check fails the
	// decode on its own and the case passes even when the validation it
	// names has regressed.
	t.Run("wrong-typed id", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`{"id":"911","kind":"message","event_type":"message.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"recording_id":900,"visible_to_clients":false}`))
	})
	t.Run("malformed created_at", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`{"id":105,"kind":"message","event_type":"message.created","action":"created","created_at":"yesterday","bucket_id":2,"creator_id":3,"recording_id":900,"visible_to_clients":false}`))
	})
	t.Run("null payload", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`null`))
	})
	t.Run("array payload", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`[{"id":105}]`))
	})
	t.Run("absent payload", func(t *testing.T) {
		assertEventDecodeFails(t, nil)
	})
}

func assertEventDecodeFails(t *testing.T, raw []byte) {
	t.Helper()
	_, err := decodeMessageEvent(raw)
	if err == nil {
		t.Fatalf("decodeMessageEvent(%s) succeeded, want invalid-frame error", raw)
	}
	var ife *invalidFrameError
	if !errors.As(err, &ife) {
		t.Fatalf("error = %T (%v), want *invalidFrameError", err, err)
	}
}

func TestSubscribeIdentifier_ChannelOnly(t *testing.T) {
	got := subscribeIdentifier(Filters{})
	if want := `{"channel":"EventsChannel"}`; got != want {
		t.Errorf("subscribeIdentifier = %s, want %s", got, want)
	}
}

func TestSubscribeIdentifier_AllFilters(t *testing.T) {
	// Fixed key order channel/types/buckets/creators, comma-joined values in
	// configured order, absent filters omitted (SPEC §23 "Cable Protocol
	// Details"). Hand-built, so the bytes are exact.
	f := Filters{
		Types:    []string{"chat.line.created", "message.created"},
		Buckets:  []int64{2, 1},
		Creators: []int64{3},
	}
	got := subscribeIdentifier(f)
	want := `{"channel":"EventsChannel","types":"chat.line.created,message.created","buckets":"2,1","creators":"3"}`
	if got != want {
		t.Errorf("subscribeIdentifier =\n %s, want\n %s", got, want)
	}
}

func TestSubscribeIdentifier_PartialFilters(t *testing.T) {
	got := subscribeIdentifier(Filters{Buckets: []int64{5951425}})
	if want := `{"channel":"EventsChannel","buckets":"5951425"}`; got != want {
		t.Errorf("subscribeIdentifier = %s, want %s", got, want)
	}
}

func TestSubscribeCommand_ByteExactness(t *testing.T) {
	id := subscribeIdentifier(Filters{Types: []string{"message.created"}})
	got := subscribeCommand(id)
	want := `{"command":"subscribe","identifier":"{\"channel\":\"EventsChannel\",\"types\":\"message.created\"}"}`
	if string(got) != want {
		t.Errorf("subscribeCommand =\n %s, want\n %s", got, want)
	}
	// The command must round-trip as JSON whose identifier is the exact
	// identifier string — that string equality is what confirm/reject
	// correlation depends on.
	var cmd struct {
		Command    string `json:"command"`
		Identifier string `json:"identifier"`
	}
	if err := json.Unmarshal(got, &cmd); err != nil {
		t.Fatalf("subscribe command is not valid JSON: %v", err)
	}
	if cmd.Command != "subscribe" {
		t.Errorf("command = %q, want %q", cmd.Command, "subscribe")
	}
	if cmd.Identifier != id {
		t.Errorf("identifier = %q, want %q", cmd.Identifier, id)
	}
	// Any retransmit is byte-identical: the server absorbs identical
	// resubscribes and rejects different ones.
	if again := subscribeCommand(id); !bytes.Equal(got, again) {
		t.Errorf("retransmit differs: %s vs %s", got, again)
	}
}

func TestUnsubscribeCommand(t *testing.T) {
	got := unsubscribeCommand(`{"channel":"EventsChannel"}`)
	want := `{"command":"unsubscribe","identifier":"{\"channel\":\"EventsChannel\"}"}`
	if string(got) != want {
		t.Errorf("unsubscribeCommand = %s, want %s", got, want)
	}
}

func TestInvalidFrameError_RenderingCarriesNoFrameContent(t *testing.T) {
	// §23 Security Invariants: an invalid-frame error renders no frame
	// contents AT ALL — the rendering names the shape and nothing else.
	// That absence is the required rule, not a courtesy past a cap:
	// bounding is not redacting, and the cap governs only the package's
	// other renderings.
	secret := `{"type":"disconnect","token":"SECRET-DO-NOT-RENDER`
	_, err := parseFrame([]byte(secret))
	if err == nil {
		t.Fatal("parseFrame succeeded, want error")
	}
	if strings.Contains(err.Error(), "SECRET-DO-NOT-RENDER") {
		t.Errorf("error rendering leaks frame content: %v", err)
	}
}

// TestParseFrame_UnknownTypeIgnoresUnrelatedFields is SPEC.md §23's
// forward-compatibility rule: "Unknown frame types — parseable JSON whose
// `type` the connector doesn't recognize — update liveness and are otherwise
// ignored."
//
// Validating the whole envelope before classifying broke it. A future frame
// carrying a field of a different type failed the envelope unmarshal on a
// field its own kind does not have, and was dispatched as a socket failure —
// so a server that started sending such a frame would tear down and reconnect
// every connected client, indefinitely, over a frame the protocol says to
// ignore. The kind is selected first; only the fields that kind READS are
// validated.
func TestParseFrame_UnknownTypeIgnoresUnrelatedFields(t *testing.T) {
	for _, raw := range []string{
		`{"type":"future","identifier":1}`,
		`{"type":"future","reason":{"code":7},"reconnect":"maybe"}`,
		`{"type":"future","message":[1,2,3],"identifier":{"a":1}}`,
	} {
		f, err := parseFrame([]byte(raw))
		if err != nil {
			t.Errorf("parseFrame(%s) = %v, want frameUnknown — an unrecognized type is liveness-only", raw, err)
			continue
		}
		if f.kind != frameUnknown {
			t.Errorf("parseFrame(%s) kind = %v, want %v", raw, f.kind, frameUnknown)
		}
	}
	// A recognized kind still validates what IT reads, so the wrong-typed
	// cases that matter stay fatal. This is the discriminator: the fix must
	// not widen into "wrong types are fine".
	for _, raw := range []string{
		`{"type":"confirm_subscription","identifier":1}`,
		`{"type":"reject_subscription","identifier":[]}`,
		`{"type":"disconnect","reason":"remote","reconnect":"yes"}`,
		`{"identifier":42,"message":{"id":1}}`,
	} {
		if _, err := parseFrame([]byte(raw)); err == nil {
			t.Errorf("parseFrame(%s) = nil error, want the invalid-frame parse shape", raw)
		}
	}
}
