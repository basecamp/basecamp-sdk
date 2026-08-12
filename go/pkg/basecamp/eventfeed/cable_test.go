package eventfeed

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestInvalidFrameErrorRenderingIsBounded(t *testing.T) {
	// SPEC §23 "Security Invariants": bound/truncate any error rendering of
	// frame contents (§9's MAX_ERROR_MESSAGE_LENGTH). time.Time's decoder
	// embeds the offending value in its parse error, so an attacker-chosen
	// created_at would otherwise reach Observer.Disconnected at frame scale.
	oversized := strings.Repeat("a", 4096)
	raw := []byte(`{"id":105,"kind":"message","event_type":"message.created","action":"created","created_at":"` +
		oversized + `","bucket_id":2,"creator_id":3,"recording_id":900}`)
	_, err := decodeMessageEvent(raw)
	if err == nil {
		t.Fatal("decodeMessageEvent succeeded, want an invalid-frame error")
	}
	if got := len(err.Error()); got > maxErrorMessageBytes {
		t.Fatalf("rendered error length = %d bytes, want at most %d", got, maxErrorMessageBytes)
	}
	if strings.Contains(err.Error(), oversized) {
		t.Fatal("the rendered error embeds the frame-supplied value verbatim")
	}
	// The same bound covers the parse shape, whose decoder errors can quote
	// frame bytes too.
	_, perr := parseFrame([]byte(`{"type":"` + oversized + `"`))
	if perr == nil {
		t.Fatal("parseFrame succeeded, want an invalid-frame error")
	}
	if got := len(perr.Error()); got > maxErrorMessageBytes {
		t.Fatalf("rendered parse error length = %d bytes, want at most %d", got, maxErrorMessageBytes)
	}
}

func TestDecodeMessageEvent_VisibleToClientsIsOptional(t *testing.T) {
	// visible_to_clients is presence-bearing, never decode-required: absence
	// must decode with a nil pointer, not trip the invalid-frame class.
	payload := []byte(`{"id":7,"kind":"todo","event_type":"todo.completed","action":"completed","created_at":"2026-08-01T12:00:00Z","bucket_id":1,"creator_id":2,"recording_id":3}`)
	ev, err := decodeMessageEvent(payload)
	if err != nil {
		t.Fatalf("decodeMessageEvent: %v", err)
	}
	if ev.VisibleToClients != nil {
		t.Errorf("VisibleToClients = %v, want absent (nil)", *ev.VisibleToClients)
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
	}
	requiredKeys := []string{"id", "kind", "event_type", "action", "created_at", "bucket_id", "creator_id", "recording_id"}
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
	t.Run("wrong-typed id", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`{"id":"911","kind":"message","event_type":"message.created","action":"created","created_at":"2026-08-01T12:00:00Z","bucket_id":2,"creator_id":3,"recording_id":900}`))
	})
	t.Run("malformed created_at", func(t *testing.T) {
		assertEventDecodeFails(t, []byte(`{"id":105,"kind":"message","event_type":"message.created","action":"created","created_at":"yesterday","bucket_id":2,"creator_id":3,"recording_id":900}`))
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
	// §23 Security Invariants: error renderings of frame contents are
	// bounded. The codec goes further — it never embeds frame bytes at all.
	secret := `{"type":"disconnect","token":"SECRET-DO-NOT-RENDER`
	_, err := parseFrame([]byte(secret))
	if err == nil {
		t.Fatal("parseFrame succeeded, want error")
	}
	if strings.Contains(err.Error(), "SECRET-DO-NOT-RENDER") {
		t.Errorf("error rendering leaks frame content: %v", err)
	}
}
