package main

// Wire-replay runner for the Go SDK.
//
// Mode-gate: this entrypoint runs only when WIRE_REPLAY_DIR is set. The
// existing main.go handles mock-mode and remains untouched aside from a
// thin gate at the top of main() that delegates here.
//
// We import go/pkg/generated directly. The unexported mappers
// (projectFromGenerated, etc.) intentionally stay private; this runner
// consumes the same internal generated types oapi-codegen produces. Do
// not "fix" this import — it is a deliberate seam for canary purposes.
// Per the BC5-readiness plan §"Per-language additions" (PR 3): the Go
// runner uses `json.Unmarshal(bodyBytes, &generated.<Type>{})` for the
// typed-decode boundary and walks the same bytes parsed into `any` for
// extras detection.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	generated "github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

const ReplaySchemaVersion = 1

// Decoders map operation_id -> decoder fn. Each decoder unmarshals
// bodyText into the generated response type produced by oapi-codegen
// (the same types the SDK uses internally) and returns the unmarshal
// error, or nil on success. Keep this table in sync with
// LIVE_OPERATIONS in conformance/runner/typescript/live-dispatch.ts —
// the coverage gate enforces parity.
var decoders = map[string]func(bodyText string) error{
	"ListProjects": func(bt string) error {
		var v generated.ListProjectsResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"GetProject": func(bt string) error {
		var v generated.GetProjectResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"GetMyAssignments": func(bt string) error {
		var v generated.GetMyAssignmentsResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"GetMyCompletedAssignments": func(bt string) error {
		var v generated.GetMyCompletedAssignmentsResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"GetMyDueAssignments": func(bt string) error {
		var v generated.GetMyDueAssignmentsResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"GetMyNotifications": func(bt string) error {
		var v generated.GetMyNotificationsResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"GetMyProfile": func(bt string) error {
		var v generated.GetMyProfileResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"GetTodoset": func(bt string) error {
		var v generated.GetTodosetResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"ListTodolists": func(bt string) error {
		var v generated.ListTodolistsResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
	"ListTodos": func(bt string) error {
		var v generated.ListTodosResponseContent
		return json.Unmarshal([]byte(bt), &v)
	},
}

var safeNameRE = regexp.MustCompile(`(?i)[^a-z0-9_-]+`)

func safeName(s string) string {
	return safeNameRE.ReplaceAllString(s, "_")
}

// ReplayPage mirrors the per-page output schema documented in the
// BC5-readiness plan §"Snapshot output". Slices are emitted as `[]` (not
// null) in the JSON output for downstream comparator stability — see
// emptyIfNil before serialization.
type ReplayPage struct {
	Decoded         bool     `json:"decoded"`
	DecodeError     *string  `json:"decode_error"`
	MissingRequired []string `json:"missing_required"`
	ExtrasSeen      []string `json:"extras_seen"`
}

type ReplayResult struct {
	SchemaVersion int          `json:"schema_version"`
	Operation     string       `json:"operation"`
	Pages         []ReplayPage `json:"pages"`
}

type fixtureTest struct {
	Mode      string `json:"mode"`
	Name      string `json:"name"`
	Operation string `json:"operation"`
}

type wirePage struct {
	Status   int               `json:"status"`
	Headers  map[string]string `json:"headers"`
	Body     any               `json:"body"`
	BodyText *string           `json:"bodyText"`
	URL      string            `json:"url"`
}

type wireSnapshot struct {
	Operation  string     `json:"operation"`
	Pages      []wirePage `json:"pages"`
	PagesCount int        `json:"pages_count"`
}

type ReplayRunner struct {
	replayDir   string
	backend     string
	fixturePath string
	walker      *SchemaWalker
	fixture     []fixtureTest
}

func NewReplayRunner(replayDir, backend, fixturePath, openapiPath string) (*ReplayRunner, error) {
	walker, err := NewSchemaWalker(openapiPath)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, err
	}
	var all []fixtureTest
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	live := make([]fixtureTest, 0, len(all))
	for _, t := range all {
		if t.Mode == "live" {
			live = append(live, t)
		}
	}
	return &ReplayRunner{replayDir, backend, fixturePath, walker, live}, nil
}

// coverageGate enforces the three startup checks documented in PR 3
// §"Coverage gate":
//  1. every fixture op has a decoder
//  2. every fixture op has a snapshot file
//  3. every snapshot's operation field is in the fixture
//
// Any failure returns a non-empty list of human-readable messages; the
// caller prints them and exits non-zero.
func (r *ReplayRunner) coverageGate() []string {
	var msgs []string
	fixtureOps := map[string]bool{}
	for _, t := range r.fixture {
		fixtureOps[t.Operation] = true
	}

	// 1. Decoder coverage
	var missing []string
	for op := range fixtureOps {
		if _, ok := decoders[op]; !ok {
			missing = append(missing, op)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		msgs = append(msgs, fmt.Sprintf(
			"Go replay runner missing decoders for: %v. Add to decoders map in replay_runner.go.",
			missing,
		))
	}

	// 2. Snapshot completeness
	wireDir := filepath.Join(r.replayDir, r.backend, "wire")
	for _, t := range r.fixture {
		f := filepath.Join(wireDir, safeName(t.Name)+".json")
		if _, err := os.Stat(f); err != nil {
			msgs = append(msgs, fmt.Sprintf(
				"Snapshot missing for operation %s (test %q); expected at %s. Re-run TS live capture or check skip status.",
				t.Operation, t.Name, f,
			))
		}
	}

	// 3. Snapshot recognition
	if entries, err := os.ReadDir(wireDir); err == nil {
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(wireDir, e.Name()))
			if err != nil {
				continue
			}
			var snap struct {
				Operation string `json:"operation"`
			}
			if err := json.Unmarshal(data, &snap); err != nil {
				continue
			}
			if snap.Operation == "" {
				msgs = append(msgs, fmt.Sprintf(
					"Snapshot %s is missing the top-level `operation` field. Re-run the TS live canary; pre-PR3 snapshots are no longer supported.",
					e.Name(),
				))
				continue
			}
			if !fixtureOps[snap.Operation] {
				msgs = append(msgs, fmt.Sprintf(
					"Unknown operation %q in snapshot %s; TS dispatch table appears to have drifted from live-my-surface.json.",
					snap.Operation, e.Name(),
				))
			}
		}
	}

	return msgs
}

func (r *ReplayRunner) Run() int {
	if msgs := r.coverageGate(); len(msgs) > 0 {
		for _, m := range msgs {
			fmt.Fprintln(os.Stderr, m)
		}
		return 1
	}

	outDir := filepath.Join(r.replayDir, r.backend, "decode", "go")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	failures := 0
	for _, t := range r.fixture {
		snap, err := r.readSnapshot(t.Name)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failures++
			continue
		}
		result := r.decodeSnapshot(snap)
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			failures++
			continue
		}
		outPath := filepath.Join(outDir, safeName(t.Name)+".json")
		if err := os.WriteFile(outPath, out, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			failures++
			continue
		}
		for _, p := range result.Pages {
			if !p.Decoded || len(p.MissingRequired) > 0 {
				failures++
				break
			}
		}
	}

	if failures > 0 {
		return 1
	}
	return 0
}

func (r *ReplayRunner) readSnapshot(testName string) (*wireSnapshot, error) {
	path := filepath.Join(r.replayDir, r.backend, "wire", safeName(testName)+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap wireSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

func (r *ReplayRunner) decodeSnapshot(snap *wireSnapshot) ReplayResult {
	decoder := decoders[snap.Operation]
	schema := r.walker.FindResponseSchema(snap.Operation)

	pages := make([]ReplayPage, 0, len(snap.Pages))
	for _, page := range snap.Pages {
		var bodyText string
		switch {
		case page.BodyText != nil:
			bodyText = *page.BodyText
		case page.Body != nil:
			if b, err := json.Marshal(page.Body); err == nil {
				bodyText = string(b)
			}
		}

		rp := ReplayPage{Decoded: false}
		if err := decoder(bodyText); err == nil {
			rp.Decoded = true
		} else {
			s := err.Error()
			rp.DecodeError = &s
		}

		if schema != nil {
			// Per the TS validator: walk against parsed JSON, not the
			// SDK-decoded structure (decoders may drop unknown fields).
			// A page whose body is not parseable JSON gets empty arrays
			// here — the decode_error above already captures that failure.
			var body any
			if json.Unmarshal([]byte(bodyText), &body) == nil {
				rp.MissingRequired = r.walker.MissingRequired(body, schema)
				rp.ExtrasSeen = r.walker.ExtrasSeen(body, schema)
			}
		}
		// Emit `[]` rather than `null` for stable cross-language
		// comparator output (PR 4 jq diffs would otherwise false-fire).
		if rp.MissingRequired == nil {
			rp.MissingRequired = []string{}
		}
		if rp.ExtrasSeen == nil {
			rp.ExtrasSeen = []string{}
		}
		pages = append(pages, rp)
	}

	return ReplayResult{
		SchemaVersion: ReplaySchemaVersion,
		Operation:     snap.Operation,
		Pages:         pages,
	}
}
