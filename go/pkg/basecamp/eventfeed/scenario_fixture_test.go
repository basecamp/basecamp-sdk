// Tier-2 conformance: the scenario fixture model.
//
// This file loads `conformance/event-feed/fixtures/*.json` — merged contract,
// never edited here — into typed directives. Loading is STRICT in both
// directions: an unknown step directive, an unknown object key, a construct
// the driver does not model, or a fixture-level invariant the JSON Schema
// cannot express (droppedCount vs droppedIds, a fireTimer envelope with
// min > max, a name that disagrees with the filename) fails the fixture at
// load with a named error. Nothing is ever silently ignored: that is what
// keeps the driver honest as PR 4 adds fixtures and schema constructs.
package eventfeed_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

// scenario is one loaded tier-2 fixture.
type scenario struct {
	Name        string
	Description string
	Config      scenarioConfig
	Steps       []scenarioStep
	Finally     scenarioFinally
}

// scenarioStep is one directive: its schema key plus the decoded payload.
type scenarioStep struct {
	Kind    string
	Payload any
}

// scenarioConfig is the schema's `config` object — the connector construction
// options a scenario selects.
type scenarioConfig struct {
	Types    []string `json:"types"`
	Buckets  []int64  `json:"buckets"`
	Creators []int64  `json:"creators"`
	Position string   `json:"position"`
	// The five durations are THREE-STATE because presence is meaning twice
	// over: a plain int64 read an explicit zero as "absent, use the
	// default", and a pointer read an explicit JSON null the same way —
	// each accepting a value the schema rejects ("minimum": 1 for zero,
	// "type": "integer" for null) and silently substituting another. The
	// driver enforces the schema's judgments portably, so all three states
	// the wire distinguishes are preserved: absent, null, and value.
	ConfirmationDeadlineMs optionalMs        `json:"confirmationDeadlineMs"`
	RepairPollBaseMs       optionalMs        `json:"repairPollBaseMs"`
	BackoffBaseMs          optionalMs        `json:"backoffBaseMs"`
	BackoffCapMs           optionalMs        `json:"backoffCapMs"`
	StalenessMs            optionalMs        `json:"stalenessMs"`
	LiveBufferCapacity     int               `json:"liveBufferCapacity"`
	DedupeCapacity         int               `json:"dedupeCapacity"`
	SignalDisposition      map[string]string `json:"signalDisposition"`
	CheckpointStore        *storeScript      `json:"checkpointStore"`
}

// optionalMs is one config duration in the three JSON states the schema
// distinguishes: absent (the zero optionalMs — use the default), JSON null
// (set, null — rejected, "type": "integer" refuses it), and a value (set,
// ranged). encoding/json calls a value type's UnmarshalJSON for null where it
// short-circuits a pointer's, which is exactly why this is not a *int64.
type optionalMs struct {
	set  bool
	null bool
	v    int64
}

func (o *optionalMs) UnmarshalJSON(data []byte) error {
	o.set = true
	if string(data) == "null" {
		o.null = true
		return nil
	}
	v, err := parseIntegralMs(data)
	o.v = v
	return err
}

// parseIntegralMs parses one JSON number the way draft 2020-12's "integer"
// judges it: by MATHEMATICAL value, not spelling — 1000.0 and 1e3 are integer
// instances a schema-valid fixture may carry, and only this driver was
// refusing them (the float-spelled-int class FlexInt absorbs on the
// rich-text lane). Integrality is a fact about the TEXT, which json.Number
// preserves: a float64 detour rounds 1000.00000000000001 to exactly 1000 —
// and 315575999999.99999 to exactly the maximum — before any check can look,
// so the literal is judged exactly, with big.Rat. Two gates come first: a
// quoted "1000" is a STRING instance the schema refuses, though json.Number's
// own Unmarshal would take it; and an exponent is read as a NUMBER'S text
// before anything is materialized, so an exponent bomb (1e999999999) is
// refused for its magnitude, never expanded.
func parseIntegralMs(data []byte) (int64, error) {
	trimmed := strings.TrimLeft(string(data), " \t\r\n")
	if strings.HasPrefix(trimmed, `"`) {
		return 0, fmt.Errorf("%s is a string: the schema's type is integer — quote-wrapping a number makes it a different instance", trimmed)
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return 0, err
	}
	lit := n.String()
	// A bomb is refused before anything is materialized, and by the value's
	// EFFECTIVE magnitude, never the exponent's spelling alone — draft
	// 2020-12 constrains the mathematical value, and a significand can
	// offset any exponent (1e44-digits × e-41 is exactly 1000). Two string
	// judgments suffice, both exact:
	//   - the most significant nonzero digit's decimal place caps the
	//     value: above place 13 nothing fits [0, maxScenarioMs] (12 digits);
	//   - a least significant nonzero digit below the units place makes the
	//     value non-integral outright (decimal digits do not carry), which
	//     refuses 1e-999999999 without a 10^999999999 denominator.
	// A length cap comes first so no multi-megabyte literal is ever walked
	// into a rational — the schema's top-level description sanctions exactly
	// this bound (a resource limit on spellings, not a value constraint), so
	// refusing "1" + 100k zeros + e-100000 (mathematically 1) is conformant.
	if len(lit) > 100000 {
		return 0, fmt.Errorf("a %d-character number is beyond any modeled ms value (literals are capped at 100000 characters)", len(lit))
	}
	mant, expText := lit, ""
	if i := strings.IndexAny(lit, "eE"); i >= 0 {
		mant = lit[:i]
		expText = strings.TrimPrefix(lit[i+1:], "+")
	}
	digits := strings.TrimPrefix(mant, "-")
	point := strings.IndexByte(digits, '.')
	intLen := len(digits)
	if point >= 0 {
		intLen = point
		digits = digits[:point] + digits[point+1:]
	}
	firstNZ, lastNZ := -1, -1
	for i := 0; i < len(digits); i++ {
		if digits[i] >= '1' && digits[i] <= '9' {
			if firstNZ < 0 {
				firstNZ = i
			}
			lastNZ = i
		}
	}
	if firstNZ < 0 {
		// Zero, however spelled (0, 0.000, 0e200001): integral, the range
		// judgment's to refuse, and decided before the exponent is even
		// parsed — an exponent multiplies a significand, and this one is
		// zero.
		return 0, nil
	}
	exp := 0
	if expText != "" {
		e, err := strconv.Atoi(expText)
		if err != nil {
			return 0, fmt.Errorf("%s is not a number this driver can read", lit)
		}
		// Bound the exponent before any place arithmetic: at the platform's
		// integer extremes, intLen - firstNZ + exp wraps and the magnitude
		// judgments below judge garbage. The literal cap above bounds the
		// significand at 100000 digits, so no in-range value needs an
		// exponent beyond ±200000 to spell.
		if e > 200000 || e < -200000 {
			return 0, fmt.Errorf("%s is beyond any modeled ms value", lit)
		}
		exp = e
	}
	{
		// Digit i occupies decimal place intLen - i + exp (units = 1).
		if msd := intLen - firstNZ + exp; msd > 13 {
			return 0, fmt.Errorf("%s is beyond any modeled ms value", lit)
		}
		if lsd := intLen - lastNZ + exp; lsd < 1 {
			return 0, fmt.Errorf("%s is not an integer: the schema's type is integer — a number whose mathematical value is integral", lit)
		}
	}
	r, ok := new(big.Rat).SetString(lit)
	if !ok {
		return 0, fmt.Errorf("%s is not a number this driver can read", lit)
	}
	if !r.IsInt() {
		return 0, fmt.Errorf("%s is not an integer: the schema's type is integer — a number whose mathematical value is integral", lit)
	}
	num := r.Num()
	if !num.IsInt64() {
		return 0, fmt.Errorf("%s is beyond any modeled ms value", lit)
	}
	return num.Int64(), nil
}

// scenarioMs is a required ms value under the same number model.
type scenarioMs int64

func (m *scenarioMs) UnmarshalJSON(data []byte) error {
	v, err := parseIntegralMs(data)
	*m = scenarioMs(v)
	return err
}

// storeScript is the schema's scripted CheckpointStore.
type storeScript struct {
	Load string   `json:"load"`
	Save []string `json:"save"`
}

// scenarioFinally is the schema's `finally` object.
type scenarioFinally struct {
	State              string            `json:"state"`
	MintCount          *int              `json:"mintCount"`
	ConnectCount       *int              `json:"connectCount"`
	PollCount          *int              `json:"pollCount"`
	Timers             *timerSet         `json:"timers"`
	Socket             string            `json:"socket"`
	Delivered          *exactIDs         `json:"delivered"`
	Checkpoints        *exactPositions   `json:"checkpoints"`
	Error              *errorExpect      `json:"error"`
	HandlerInvocations *exactInvocations `json:"handlerInvocations"`
}

type timerSet struct {
	Exact map[string]int `json:"exact"`
}

type exactIDs struct {
	Exact []int64 `json:"exact"`
}

type exactPositions struct {
	Exact []string `json:"exact"`
}

type exactInvocations struct {
	Exact []invocation `json:"exact"`
}

type invocation struct {
	Kind        string `json:"kind"`
	Disposition string `json:"disposition"`
}

type errorExpect struct {
	Reason   string `json:"reason"`
	Category string `json:"category"`
}

// --- step payloads -------------------------------------------------------

type expectMintStep struct {
	Respond mintRespond `json:"respond"`
}

type mintRespond struct {
	Status  *int              `json:"status"`
	Body    *mintBody         `json:"body"`
	Headers map[string]string `json:"headers"`
}

type mintBody struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int    `json:"expires_in"`
	URL       string `json:"url"`
}

type expectConnectStep struct {
	URL     string          `json:"url"`
	Outcome *connectOutcome `json:"outcome"`
}

type connectOutcome struct {
	Accept     *bool  `json:"accept"`
	Refuse     string `json:"refuse"`
	RedirectTo string `json:"redirectTo"`
}

type serveStep struct {
	Frame      string          `json:"frame"`
	Message    *int            `json:"message"`
	Identifier *string         `json:"identifier"`
	Reason     string          `json:"reason"`
	Reconnect  *bool           `json:"reconnect"`
	Event      json.RawMessage `json:"event"`
	Text       string          `json:"text"`
}

type serverCloseStep struct {
	Code   int    `json:"code"`
	Reason string `json:"reason"`
}

type expectSubscribeStep struct {
	Channel             string            `json:"channel"`
	Params              map[string]string `json:"params"`
	IdenticalToPrevious *bool             `json:"identicalToPrevious"`
}

type expectPollStep struct {
	URL     string          `json:"url"`
	Query   json.RawMessage `json:"query"`
	Respond pollRespond     `json:"respond"`

	// query, decoded: exactPin selects equality over the whole derived query.
	params   map[string]string
	exactPin bool
}

type pollRespond struct {
	Status  *int              `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

type pollEnvelope struct {
	Events   []json.RawMessage `json:"events"`
	Position string            `json:"position"`
	Next     string            `json:"next"`
}

type pollEventRow struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	EventType   string `json:"event_type"`
	Action      string `json:"action"`
	CreatedAt   string `json:"created_at"`
	BucketID    int64  `json:"bucket_id"`
	CreatorID   int64  `json:"creator_id"`
	RecordingID int64  `json:"recording_id"`
}

type goneBody struct {
	Error        string `json:"error"`
	EpochAfterID int64  `json:"epoch_after_id"`
	Resume       string `json:"resume"`
}

type advanceStep struct {
	Ms scenarioMs `json:"ms"`
}

type fireTimerStep struct {
	Kind          string           `json:"kind"`
	AssertDelayMs optionalEnvelope `json:"assertDelayMs"`
}

// delayEnvelope's members are three-state for the same reason the config
// durations are: min and max are schema-REQUIRED integers, and a plain int64
// read an absent or null member as 0 — inside the allowed range, silently
// converting the authored envelope into a different one.
type delayEnvelope struct {
	Min optionalMs `json:"min"`
	Max optionalMs `json:"max"`
}

// optionalEnvelope is assertDelayMs in the three JSON states: absent (no
// assertion — the zero value), null (set, null — refused, the schema's type
// is object), and a value (set, decoded strictly: the outer decoder's
// DisallowUnknownFields does not reach inside a custom unmarshaler, so the
// strictness is re-established here).
type optionalEnvelope struct {
	set  bool
	null bool
	env  delayEnvelope
}

func (o *optionalEnvelope) UnmarshalJSON(data []byte) error {
	o.set = true
	if string(data) == "null" {
		o.null = true
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(&o.env)
}

type expectCheckpointStep struct {
	Position string `json:"position"`
}

type expectStateStep struct {
	Is string `json:"is"`
}

type expectGapStep struct {
	EpochAfterID int64 `json:"epochAfterId"`
}

type expectBufferedStep struct {
	Count int `json:"count"`
}

type expectSignalStep struct {
	Kind         string  `json:"kind"`
	DroppedIDs   []int64 `json:"droppedIds"`
	DroppedCount *int    `json:"droppedCount"`
	EpochAfterID *int64  `json:"epochAfterId"`
}

type expectPositionRejectedStep struct {
	Kind string `json:"kind"`
}

// --- loading -------------------------------------------------------------

// maxScenarioMs is the schema's shared `maximum` for every ms field: 10
// virtual years. It is a DOMAIN bound — scripts age tickets by minutes to
// days, so the largest value today (~11 days) has 300× headroom — chosen over
// the representation-derived 9,223,372,036,854 (the largest ms count whose
// int64-nanosecond product does not overflow) because the schema should say
// what a script can MEAN, not restate one language's integer layout. It sits
// ~29× under that overflow line, so no conforming driver's duration
// representation can overflow — the failure this bound exists to make a
// fixture error rather than a representation accident: one past the int64
// line, time.Duration(ms)*time.Millisecond goes negative and an accepted
// advance would silently REWIND virtual time.
const maxScenarioMs int64 = 315_576_000_000

// checkScenarioMs enforces the schema's [floor, maxScenarioMs] range on one
// ms field at load, so every driver rejects the same values for the same
// stated reason. ms values are int64 END TO END (fixture structs, this check,
// millis): the maximum exceeds MaxInt32, so a platform-width int fails to
// compile on 32-bit (an int64-typed constant alone would instead make
// schema-valid values above MaxInt32 fail decode into int structs).
func checkScenarioMs(what string, v, floor int64) error {
	if v < floor || v > maxScenarioMs {
		return fmt.Errorf("%s must be in [%d, %d] (10 virtual years): got %d", what, floor, maxScenarioMs, v)
	}
	return nil
}

// parseScenario decodes one substituted fixture, failing on anything the
// driver does not model.
func parseScenario(raw []byte, file string) (*scenario, error) {
	top, err := objectKeys(raw)
	if err != nil {
		return nil, err
	}
	if err := allowedKeys("fixture", top, "name", "description", "config", "steps", "finally"); err != nil {
		return nil, err
	}
	sc := &scenario{}
	if err := decodeStrict(top["name"], &sc.Name); err != nil {
		return nil, fmt.Errorf("name: %w", err)
	}
	if err := decodeStrict(top["description"], &sc.Description); err != nil {
		return nil, fmt.Errorf("description: %w", err)
	}
	if want := strings.TrimSuffix(file, ".json"); sc.Name != want {
		return nil, fmt.Errorf("fixture name %q must match its filename %q", sc.Name, want)
	}
	if cfgRaw, ok := top["config"]; ok {
		if err := decodeStrict(cfgRaw, &sc.Config); err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
		if err := validateConfig(sc.Config); err != nil {
			return nil, fmt.Errorf("config: %w", err)
		}
	}
	var steps []json.RawMessage
	if err := decodeStrict(top["steps"], &steps); err != nil {
		return nil, fmt.Errorf("steps: %w", err)
	}
	for i, rawStep := range steps {
		step, err := parseStep(rawStep)
		if err != nil {
			return nil, fmt.Errorf("step %d: %w", i+1, err)
		}
		sc.Steps = append(sc.Steps, step)
	}

	// An advance is deterministic only from a scripted rendezvous point, and
	// the rendezvous is TWO steps: expectState, then expectTimers. An
	// action's completion can precede the timer arms its transition causes —
	// expectConnect returns when the dial is recorded, while the handshake
	// deadline arms on the connector's goroutine after — and a set match
	// ALONE can coincide with a transient mid-surgery set (the welcome
	// transition stops handshake-deadline and arms confirmation-deadline in
	// separate clock acquisitions, so an authored set can exist in between).
	// The state announcement bounds the surgery: expectState blocks until
	// the transition announces, and in every announced state any timer still
	// unarmed at the announcement is exactly what the following exact-set
	// match waits for — so the pair settles where either alone races. Both
	// blocks fail loudly on the watchdog when the authored state or set is
	// wrong; nothing diverges silently.
	for i, step := range sc.Steps {
		if step.Kind != "advance" || i == 0 {
			continue
		}
		if i < 2 || sc.Steps[i-1].Kind != "expectTimers" || sc.Steps[i-2].Kind != "expectState" {
			return nil, fmt.Errorf("step %d: an advance must be the scenario's first step or immediately follow "+
				"an expectState + expectTimers rendezvous — an action's completion can precede the timer arms "+
				"its transition causes, and a set match alone can coincide with a transient mid-surgery set; "+
				"the state announcement bounds the surgery and the exact-set match settles what follows it", i+1)
		}
		// The match must be able to MEAN settled: an empty set can never
		// contain an arm of the preceding transition, so it matches before
		// that transition is processed (a released failed mint has not armed
		// backoff yet) exactly as if the rendezvous were absent. The limit
		// this cannot close is stated in the contract: the authored set must
		// include an arm of the preceding transition, and a same-kind rearm
		// is invisible to set matching — such scripts use fireTimer.
		if rv, ok := sc.Steps[i-1].Payload.(*timerSet); ok && len(rv.Exact) == 0 {
			return nil, fmt.Errorf("step %d: an empty rendezvous orders nothing — an expectTimers set with no "+
				"timers cannot contain an arm of the preceding transition, so its match cannot prove the "+
				"transition settled; a scenario with nothing yet armed advances as its first step", i+1)
		}
	}
	finRaw, ok := top["finally"]
	if !ok {
		return nil, fmt.Errorf("fixture is missing its `finally` block")
	}
	if err := decodeStrict(finRaw, &sc.Finally); err != nil {
		return nil, fmt.Errorf("finally: %w", err)
	}
	if err := validateFinally(sc.Finally); err != nil {
		return nil, fmt.Errorf("finally: %w", err)
	}
	return sc, nil
}

// parseStep decodes one step's single directive.
func parseStep(raw json.RawMessage) (scenarioStep, error) {
	keys, err := objectKeys(raw)
	if err != nil {
		return scenarioStep{}, err
	}
	if len(keys) != 1 {
		return scenarioStep{}, fmt.Errorf("a step carries exactly one directive, got %d", len(keys))
	}
	for kind, body := range keys {
		payload, err := decodeDirective(kind, body)
		if err != nil {
			return scenarioStep{}, fmt.Errorf("%s: %w", kind, err)
		}
		return scenarioStep{Kind: kind, Payload: payload}, nil
	}
	return scenarioStep{}, fmt.Errorf("unreachable")
}

// decodeDirective decodes one directive body. An unrecognized directive key is
// a hard failure — never a skip — so a fixture using a construct this driver
// has not implemented fails loudly instead of passing vacuously.
//
// dispatch table is exactly the shape that makes an unimplemented directive
// impossible to miss.
//
//nolint:gocyclo,cyclop // one arm per schema directive: a flat, exhaustive
func decodeDirective(kind string, body json.RawMessage) (any, error) {
	switch kind {
	case "expectMint":
		step := &expectMintStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, validateMintRespond(step.Respond)
	case "expectConnect":
		step := &expectConnectStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, validateConnectOutcome(step.Outcome)
	case "serve":
		step := &serveStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, validateServe(step)
	case "serverClose":
		step := &serverCloseStep{}
		return step, decodeStrict(body, step)
	case "sever":
		var flag bool
		if err := decodeStrict(body, &flag); err != nil {
			return nil, err
		}
		if !flag {
			return nil, fmt.Errorf("only `true` is valid")
		}
		return &severStep{}, nil
	case "expectSubscribe":
		step := &expectSubscribeStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		if step.Channel != "EventsChannel" {
			return nil, fmt.Errorf("channel %q is not EventsChannel", step.Channel)
		}
		return step, nil
	case "expectPoll":
		step := &expectPollStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, decodePollQuery(step)
	case "expectClientClose":
		var empty struct{}
		return &expectClientCloseStep{}, decodeStrict(body, &empty)
	case "advance":
		step := &advanceStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, checkScenarioMs("advance ms", int64(step.Ms), 1)
	case "fireTimer":
		step := &fireTimerStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		if step.AssertDelayMs.set {
			if step.AssertDelayMs.null {
				return nil, fmt.Errorf("assertDelayMs supplied as JSON null: the schema's type is object and null is not one — omit the key to fire without a delay assertion")
			}
			env := step.AssertDelayMs.env
			for _, m := range []struct {
				name string
				o    optionalMs
			}{{"min", env.Min}, {"max", env.Max}} {
				if !m.o.set {
					return nil, fmt.Errorf("assertDelayMs needs both min and max — the schema requires them, and an absent %s is a different envelope than the one authored", m.name)
				}
				if m.o.null {
					return nil, fmt.Errorf("assertDelayMs %s supplied as JSON null: the schema's type is integer and null is not one", m.name)
				}
				if err := checkScenarioMs("assertDelayMs "+m.name, m.o.v, 0); err != nil {
					return nil, err
				}
			}
			if env.Min.v > env.Max.v {
				return nil, fmt.Errorf("assertDelayMs min %d exceeds max %d", env.Min.v, env.Max.v)
			}
		}
		return step, validateTimerKind(step.Kind)
	case "expectDelivered":
		step := &exactIDs{}
		return step, decodeStrict(body, step)
	case "expectCheckpoint":
		step := &expectCheckpointStep{}
		return step, decodeStrict(body, step)
	case "expectTimers":
		step := &timerSet{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, validateTimerSet(*step)
	case "expectState":
		step := &expectStateStep{}
		return step, decodeStrict(body, step)
	case "expectError":
		step := &errorExpect{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, validateErrorExpect(*step)
	case "expectGap":
		step := &expectGapStep{}
		return step, decodeStrict(body, step)
	case "expectBuffered":
		step := &expectBufferedStep{}
		return step, decodeStrict(body, step)
	case "expectSignal":
		step := &expectSignalStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, validateSignalExpect(step)
	case "expectHandlerInvocations":
		step := &exactInvocations{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		return step, validateInvocations(step.Exact)
	case "expectSaveFailed":
		var flag bool
		if err := decodeStrict(body, &flag); err != nil {
			return nil, err
		}
		if !flag {
			return nil, fmt.Errorf("only `true` is valid")
		}
		return &expectSaveFailedStep{}, nil
	case "expectPositionRejected":
		step := &expectPositionRejectedStep{}
		if err := decodeStrict(body, step); err != nil {
			return nil, err
		}
		if step.Kind != "position_invalid" && step.Kind != "filter_changed" {
			return nil, fmt.Errorf("unknown kind %q", step.Kind)
		}
		return step, nil
	case "expectDisconnectedInvalidFrame":
		var flag bool
		if err := decodeStrict(body, &flag); err != nil {
			return nil, err
		}
		if !flag {
			return nil, fmt.Errorf("only `true` is valid")
		}
		return &expectDisconnectedInvalidFrameStep{}, nil
	default:
		return nil, fmt.Errorf("unknown step directive %q: the driver refuses to skip a directive it does not model", kind)
	}
}

// Parameterless directives, each a distinct type so the executor's type switch
// stays exhaustive.
type (
	severStep                          struct{}
	expectClientCloseStep              struct{}
	expectSaveFailedStep               struct{}
	expectDisconnectedInvalidFrameStep struct{}
)

// --- validation ----------------------------------------------------------

func validateConfig(cfg scenarioConfig) error {
	// Absence means "default"; everything SUPPLIED is judged — a value is
	// ranged (explicit zero included) and null is refused outright.
	for _, f := range []struct {
		name string
		o    optionalMs
	}{
		{"confirmationDeadlineMs", cfg.ConfirmationDeadlineMs},
		{"repairPollBaseMs", cfg.RepairPollBaseMs},
		{"stalenessMs", cfg.StalenessMs},
	} {
		switch {
		case !f.o.set:
		case f.o.null:
			return fmt.Errorf("%s supplied as JSON null: the schema's type is integer and null is not one — omit the key for the default", f.name)
		default:
			if err := checkScenarioMs(f.name, f.o.v, 1); err != nil {
				return err
			}
		}
	}
	if cfg.BackoffBaseMs.set || cfg.BackoffCapMs.set {
		return fmt.Errorf("backoffBaseMs/backoffCapMs are not modeled: SPEC §23 pins the Go connector's full-jitter base and cap as constants, with no construction option to override")
	}
	for kind, disposition := range cfg.SignalDisposition {
		if kind != "bufferOverflow" && kind != "feedGap" {
			return fmt.Errorf("signalDisposition: unknown signal kind %q", kind)
		}
		if disposition != "accept" && disposition != "terminate" {
			return fmt.Errorf("signalDisposition[%s]: unknown disposition %q", kind, disposition)
		}
	}
	if cfg.CheckpointStore != nil {
		load := cfg.CheckpointStore.Load
		switch {
		case load == "missing", load == "failed", strings.HasPrefix(load, "loaded:"):
		default:
			return fmt.Errorf("checkpointStore.load %q is not `loaded:<position>`, `missing`, or `failed`", load)
		}
		for _, outcome := range cfg.CheckpointStore.Save {
			if outcome != "saved" && outcome != "failed" {
				return fmt.Errorf("checkpointStore.save: unknown outcome %q", outcome)
			}
		}
		if cfg.Position != "" {
			return fmt.Errorf("position and checkpointStore are mutually exclusive")
		}
	}
	return nil
}

func validateFinally(fin scenarioFinally) error {
	if err := validateStateName(fin.State); err != nil {
		return err
	}
	switch fin.Socket {
	case "", "none", "open", "closed":
	default:
		return fmt.Errorf("unknown socket disposition %q", fin.Socket)
	}
	if fin.Timers != nil {
		if err := validateTimerSet(*fin.Timers); err != nil {
			return err
		}
	}
	if fin.Error != nil {
		if err := validateErrorExpect(*fin.Error); err != nil {
			return err
		}
	}
	if fin.HandlerInvocations != nil {
		return validateInvocations(fin.HandlerInvocations.Exact)
	}
	return nil
}

func validateMintRespond(r mintRespond) error {
	if r.Body != nil {
		if r.Status != nil && *r.Status != 200 {
			return fmt.Errorf("a mint body with status %d is not a modeled response", *r.Status)
		}
		if r.Body.Ticket == "" || r.Body.URL == "" || r.Body.ExpiresIn <= 0 {
			return fmt.Errorf("a successful mint body needs ticket, expires_in, and url")
		}
		return nil
	}
	if r.Status == nil {
		return fmt.Errorf("a mint response needs a body or a status")
	}
	switch *r.Status {
	case 401, 403, 404, 422, 429, 500, 502, 503, 504:
		return nil
	default:
		return fmt.Errorf("unknown mint status %d", *r.Status)
	}
}

func validateConnectOutcome(o *connectOutcome) error {
	if o == nil {
		return nil
	}
	set := 0
	if o.Accept != nil {
		if !*o.Accept {
			return fmt.Errorf("outcome.accept: only `true` is valid")
		}
		set++
	}
	if o.Refuse != "" {
		if o.Refuse != "upgrade-failure" {
			return fmt.Errorf("outcome.refuse %q is not `upgrade-failure`", o.Refuse)
		}
		set++
	}
	if o.RedirectTo != "" {
		set++
	}
	if set != 1 {
		return fmt.Errorf("outcome names exactly one of accept/refuse/redirectTo")
	}
	return nil
}

func validateServe(s *serveStep) error {
	switch s.Frame {
	case "welcome", "ping", "confirm", "reject":
		return nil
	case "disconnect":
		if s.Reason == "" || s.Reconnect == nil {
			return fmt.Errorf("a disconnect frame needs reason and reconnect")
		}
		return nil
	case "message":
		if len(s.Event) == 0 {
			return fmt.Errorf("a message frame needs an event")
		}
		return validatePushEvent(s.Event)
	case "raw":
		return nil
	default:
		return fmt.Errorf("unknown frame %q", s.Frame)
	}
}

// validatePushEvent pins the 9-key push payload: a fixture's event body is
// forwarded to the connector VERBATIM, so the driver only checks that the
// contract's keys are all present (the schema requires them; a driver that
// never looked would let a rewritten fixture through).
func validatePushEvent(raw json.RawMessage) error {
	keys, err := objectKeys(raw)
	if err != nil {
		return err
	}
	for _, key := range []string{
		"id", "kind", "event_type", "action", "created_at",
		"bucket_id", "creator_id", "recording_id", "visible_to_clients",
	} {
		if _, ok := keys[key]; !ok {
			return fmt.Errorf("push event is missing required key %q", key)
		}
	}
	return allowedKeys("push event", keys,
		"id", "kind", "event_type", "action", "created_at",
		"bucket_id", "creator_id", "recording_id", "visible_to_clients")
}

// decodePollQuery decodes expectPoll.query into its subset/exact form.
func decodePollQuery(step *expectPollStep) error {
	if step.URL == "" && len(step.Query) == 0 {
		return fmt.Errorf("a poll expectation pins nothing: give url or query")
	}
	if len(step.Query) == 0 {
		return nil
	}
	keys, err := objectKeys(step.Query)
	if err != nil {
		return err
	}
	body := step.Query
	if exact, ok := keys["exact"]; ok {
		if len(keys) != 1 {
			return fmt.Errorf("query.exact is exclusive with subset pins")
		}
		step.exactPin = true
		body = exact
	}
	params := map[string]string{}
	if err := decodeStrict(body, &params); err != nil {
		return err
	}
	for name := range params {
		switch name {
		case "position", "since", "types", "buckets", "creators":
		default:
			return fmt.Errorf("unknown query param %q", name)
		}
	}
	step.params = params
	return nil
}

func validateTimerKind(kind string) error {
	switch kind {
	case "handshake-deadline", "confirmation-deadline", "backoff",
		"staleness", "repair-poll", "poll-retry":
		return nil
	default:
		return fmt.Errorf("unknown timer kind %q", kind)
	}
}

func validateTimerSet(set timerSet) error {
	for kind, count := range set.Exact {
		if err := validateTimerKind(kind); err != nil {
			return err
		}
		if count < 1 {
			return fmt.Errorf("timer %q count %d must be positive", kind, count)
		}
	}
	return nil
}

func validateStateName(state string) error {
	switch state {
	case "idle", "backoff", "minting", "connecting", "awaiting_welcome",
		"awaiting_confirmation", "catching_up", "draining", "streaming",
		"terminal", "closed":
		return nil
	default:
		return fmt.Errorf("unknown state %q", state)
	}
}

func validateErrorExpect(e errorExpect) error {
	switch e.Reason {
	case "subscription_rejected", "protocol_fatal", "filter_invalid",
		"authorization_failed", "checkpoint_load", "usage", "buffer_overflow",
		"feed_gap", "invalid_continuation", "poll_failed", "mint_failed",
		"invalid_cable_url":
	default:
		return fmt.Errorf("unknown terminal reason %q", e.Reason)
	}
	if e.Category != "" {
		return fmt.Errorf("error.category is not modeled: the Go TerminalError carries a reason, not a §6 category")
	}
	return nil
}

func validateSignalExpect(s *expectSignalStep) error {
	switch s.Kind {
	case "bufferOverflow":
		if s.DroppedCount == nil || len(s.DroppedIDs) == 0 {
			return fmt.Errorf("a bufferOverflow signal needs droppedIds and droppedCount")
		}
		if *s.DroppedCount != len(s.DroppedIDs) {
			return fmt.Errorf("droppedCount %d disagrees with %d droppedIds", *s.DroppedCount, len(s.DroppedIDs))
		}
		if s.EpochAfterID != nil {
			return fmt.Errorf("a bufferOverflow signal carries no epochAfterId")
		}
	case "feedGap":
		if s.EpochAfterID == nil {
			return fmt.Errorf("a feedGap signal needs epochAfterId")
		}
		if len(s.DroppedIDs) > 0 || s.DroppedCount != nil {
			return fmt.Errorf("a feedGap signal carries no dropped ids")
		}
	default:
		return fmt.Errorf("unknown signal kind %q", s.Kind)
	}
	return nil
}

func validateInvocations(records []invocation) error {
	for _, rec := range records {
		if rec.Kind != "bufferOverflow" && rec.Kind != "feedGap" {
			return fmt.Errorf("unknown handler invocation kind %q", rec.Kind)
		}
		if rec.Disposition != "accept" && rec.Disposition != "terminate" {
			return fmt.Errorf("unknown handler disposition %q", rec.Disposition)
		}
	}
	return nil
}

// --- decoding helpers ----------------------------------------------------

// decodeStrict decodes exactly one JSON value into dst, rejecting unknown
// object keys and trailing content.
func decodeStrict(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return fmt.Errorf("missing value")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("trailing JSON content")
	}
	return nil
}

// objectKeys decodes a JSON object into its raw members.
func objectKeys(raw json.RawMessage) (map[string]json.RawMessage, error) {
	keys := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("not a JSON object: %w", err)
	}
	return keys, nil
}

// allowedKeys rejects any key outside the allowed set.
func allowedKeys(what string, keys map[string]json.RawMessage, allowed ...string) error {
	for key := range keys {
		found := false
		for _, ok := range allowed {
			if key == ok {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s: unknown key %q", what, key)
		}
	}
	return nil
}

// --- placeholder substitution --------------------------------------------

// placeholderPattern matches the family's `{{NAME}}` / `{{NAME:n}}` tokens.
var placeholderPattern = regexp.MustCompile(`\{\{([A-Z_]+)(?::(\d+))?\}\}`)

// substitutePlaceholders replaces every fixture placeholder with this
// harness's own loopback origins and tokens, guaranteeing distinctness across
// indices ({{CABLE_URL:1}} ≠ {{CABLE_URL:2}} is what gives the fresh-ticket
// assertions teeth). Literal foreign origins are deliberately left alone. An
// unknown placeholder fails the fixture rather than surviving into a URL.
func substitutePlaceholders(raw []byte, h *scenarioHarness) ([]byte, error) {
	var failure error
	out := placeholderPattern.ReplaceAllFunc(raw, func(token []byte) []byte {
		groups := placeholderPattern.FindSubmatch(token)
		name := string(groups[1])
		index := 0
		if len(groups[2]) > 0 {
			index, _ = strconv.Atoi(string(groups[2]))
		}
		switch name {
		case "API_ORIGIN":
			return []byte(h.apiOrigin)
		case "TICKET":
			return []byte(ticketToken(index))
		case "CABLE_URL":
			return []byte(h.cableURL(index))
		case "POS":
			return []byte(fmt.Sprintf("pos-%d", index))
		case "NEXT":
			return []byte(fmt.Sprintf("%s/events.json?continuation=%d", h.apiOrigin, index))
		default:
			if failure == nil {
				failure = fmt.Errorf("unknown placeholder %s", token)
			}
			return token
		}
	})
	return out, failure
}

func ticketToken(index int) string { return fmt.Sprintf("ticket-%d", index) }
