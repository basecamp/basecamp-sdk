// Command gen-catalog distills the SDK's authoritative tool Catalog from its
// own spec artifacts and writes it as go/pkg/basecamp/catalog/catalog.json.
//
// # What it is
//
// The Catalog is the join, done once at the source, of the three build
// products the generate pipeline already produces:
//
//   - openapi.json         — identity and wire shape: operationId, tag,
//     HTTP method, path, description, path/query
//     parameters, and the request-body JSON schema.
//   - behavior-model.json  — per-operation traits distilled from the Smithy
//     model: readonly, idempotent, retry, pagination,
//     write-semantics (and, when the model grows it,
//     the destructive trait — see the gap note below).
//   - (url-routes.json is the sibling distillation of openapi.json for URL
//     recognition; the Catalog is its operation-shaped
//     counterpart and needs the same openapi.json, so
//     it reads openapi.json directly rather than the
//     already-distilled routes.)
//
// Downstream catalog consumers (basecamp-mcp-server and any other) read the
// single embedded catalog.json from the pinned SDK dependency instead of
// vendoring openapi.json + behavior-model.json and re-joining them at every
// startup. The distilled artifact is smaller, self-contained (request-body
// $refs are inlined so no components section is needed), and stable.
//
// # It is the tag view, not a service map
//
// The catalog groups operations by their single OpenAPI tag — the operation
// view a domain-gateway consumer (basecamp-mcp-server) is built around. It is
// deliberately NOT a claim about any per-language SDK's internal service
// grouping: those derive from each SDK's own split tables, and Rust
// (basecamp-sdk#928) consults names.toml overrides BEFORE the tag, so
// tag-derived is not the same as any one SDK's service map.
//
// # The join is strict, and bounded to the generated surface
//
// Every operation must appear in BOTH inputs and carry exactly one non-blank
// OpenAPI tag; a torn regeneration or an untagged/blank-tagged operation fails
// loudly here rather than shipping a half-built catalog. The generator does not
// trust CI for this: the require-tags gate (Spec Gates) is not in main's
// required contexts, so it is loud, not enforced — the invariant is checked at
// generation time and refuses to emit from a violating document.
//
// The catalog is also bounded to the GENERATED surface. The per-language SDK
// generators emit only a fixed set of HTTP methods, declared once in
// spec/generated-verbs.json, but Smithy's @http.method is free-form, so
// openapi.json can carry HEAD/OPTIONS/TRACE operations no client exposes.
// Rather than list phantom operations a consumer cannot dispatch, the generator
// walks each path item by exclusion (skipping its structural members and x-*
// extensions) and refuses any remaining slot whose verb is outside that
// declared set (basecamp-sdk#925) — the same fail-closed shape as #922's
// require-tags gate. The bound is DERIVED from spec/generated-verbs.json rather
// than copied here, so this generator and the six SDK generators can never
// disagree about the surface (basecamp-sdk#935; the failure that motivates it
// is #925).
//
// The strict two-way join mirrors the toolkit loader's contract
// (github.com/basecamp/mcp/catalog) so a consumer's expectations and the SDK's
// guarantees are the same shape.
//
// # The destructive-trait gap
//
// The Catalog carries `destructive` as a tri-state pointer sourced ONLY from
// the behavior model's destructive trait. That trait does not exist in the
// Smithy model yet, so today the field is absent on every operation and a
// consumer keeps bridging from the action name (mcp/catalog.BridgeDestructive)
// until the trait lands. It is deliberately NOT derived from the HTTP method:
// Basecamp uses DELETE for reversible toggles (UncompleteTodo, UnpinMessage,
// DisableTool, ...), so "DELETE == destructive" would misclassify nine
// reversible operations, and one genuinely destructive operation
// (TrashRecording) is a PUT. Destructive is a semantic judgment that belongs
// in a curated Smithy trait, not in a method- or name-derived rule. See the
// package doc and the PR for the exact upstream fix.
//
// # Usage
//
//	go run ./scripts/gen-catalog [openapi.json] [behavior-model.json] [output.json]
//
// Run from the repository root; all three arguments default to their
// repo-relative locations. `make catalog` regenerates in place; `make
// catalog-check` regenerates to a temp file and diffs, byte-for-byte.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

const (
	defaultOpenAPI  = "openapi.json"
	defaultBehavior = "behavior-model.json"
	defaultOutput   = "go/pkg/basecamp/catalog/catalog.json"

	// defaultGeneratedVerbs is the single shared declaration of the HTTP
	// methods the SDK generators emit; the emission bound is derived from it
	// rather than hardcoded here. Repo-relative, read from the repository root
	// like the other inputs.
	//
	// Deliberately a constant, not a flag or a run() parameter (unlike the
	// other three input paths): an overridable bound is what produced two of
	// #933's findings, where the gate validated one file while the generators
	// read another. Un-overridable means the bound cannot be pointed somewhere
	// the generators aren't looking — don't "helpfully" add a flag later.
	defaultGeneratedVerbs = "spec/generated-verbs.json"

	catalogSchema  = "https://basecamp.com/schemas/catalog.json"
	catalogVersion = "1.0.0"

	// maxRefDepth caps request-body $ref inlining. Deeper (usually recursive)
	// structures terminate in a self-contained truncation stub rather than
	// expanding forever. Matches the toolkit loader's cap.
	maxRefDepth = 8
)

func main() {
	openapiPath := arg(1, defaultOpenAPI)
	behaviorPath := arg(2, defaultBehavior)
	outputPath := arg(3, defaultOutput)

	if err := run(openapiPath, behaviorPath, outputPath); err != nil {
		fmt.Fprintln(os.Stderr, "gen-catalog:", err)
		os.Exit(1)
	}
}

func arg(i int, def string) string {
	if len(os.Args) > i && os.Args[i] != "" {
		return os.Args[i]
	}
	return def
}

func run(openapiPath, behaviorPath, outputPath string) error {
	var oa openapiDoc
	if err := readJSON(openapiPath, &oa); err != nil {
		return err
	}
	var bm behaviorModel
	if err := readJSON(behaviorPath, &bm); err != nil {
		return err
	}

	verbs, err := loadGeneratedVerbs(defaultGeneratedVerbs)
	if err != nil {
		return err
	}

	cat, err := Build(&oa, &bm, verbs)
	if err != nil {
		return err
	}

	out, err := marshalCatalog(cat)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}

	fmt.Printf("Generated %s\n", outputPath)
	fmt.Printf("  Operations: %d (across %d tags)\n", len(cat.Operations), countTags(cat.Operations))
	return nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// marshalCatalog renders the catalog deterministically: HTML escaping off (so
// descriptions keep their literal <, >, &), two-space indent, trailing
// newline. Map keys in inlined body schemas are sorted by encoding/json, so
// the output is a pure function of the inputs — which is what the byte-for-byte
// drift gate depends on.
func marshalCatalog(cat *Catalog) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cat); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func countTags(ops []*Operation) int {
	seen := map[string]bool{}
	for _, op := range ops {
		seen[op.Tag] = true
	}
	return len(seen)
}

// ---------------------------------------------------------------------------
// Input shapes (the subset of each artifact the join reads)
// ---------------------------------------------------------------------------

type behaviorModel struct {
	Version    string                    `json:"version"`
	Operations map[string]behaviorTraits `json:"operations"`
}

type behaviorTraits struct {
	ReadOnly   bool `json:"readonly"`
	Idempotent bool `json:"idempotent"`
	// Destructive is tri-state: absent (nil) means the model does not declare
	// the trait for this operation. See the destructive-trait gap note above.
	Destructive *bool           `json:"destructive"`
	Retry       *Retry          `json:"retry"`
	Pagination  *Pagination     `json:"pagination"`
	Write       *WriteSemantics `json:"write"`
}

type openapiDoc struct {
	// Paths maps each path to its Path Item Object, kept as raw messages so the
	// generator walks the item by EXCLUSION (structural keys below) rather than
	// a verb allow-list — the same shape as #922's require-tags gate, so it
	// cannot fail open on a key it does not recognize.
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]map[string]any `json:"schemas"`
	} `json:"components"`
}

// loadGeneratedVerbs derives the emission bound — the set of HTTP methods the
// SDK generators emit — from the single shared declaration in
// spec/generated-verbs.json, rather than repeating it as a literal here. That
// literal was a known copy of the same fact six other generators read; deriving
// it means this generator and those six can never disagree about the surface
// (basecamp-sdk#935), which is exactly the failure #925 was.
//
// The read is deliberately narrow — it asserts one fact about this process's own
// state ("I obtained a usable set to bound on, or I stop"), not the file's
// well-formedness. It adds no BOM/stream/shape opinions of its own — NOT because
// something upstream validates the file (nothing does: #933, which would have
// added such a gate, was closed unmerged, and it proved a "prerequisite of every
// generate target" unreachable — make -j schedules that gate concurrently with
// the checks it was meant to precede). A seventh JSON opinion is exactly what
// #933 showed cannot define validity for the other six. Nothing guarantees the
// file is well-formed before this function sees it — that is WHY fail-loud is the
// contract, not a reason the read can be relaxed. It only:
//
//   - reads and JSON-unmarshals the file, PERMISSIVELY (unknown keys ignored),
//   - keeps the verb strings that are non-empty and non-whitespace, and
//   - FAILS LOUD if the file is unreadable/unparseable OR if it yields no usable
//     verb at all (verbs absent, [], or nothing but blanks).
//
// There is intentionally NO fallback to a hardcoded default: a silent fallback
// is precisely how a stale superset (e.g. one still naming `patch`) would
// survive the file dropping it, so an empty bound must STOP the build, never be
// read as "no restriction." There is likewise no len==0 short-circuit anywhere
// downstream that could turn an empty bound into "allow everything."
//
// Kept in ONE small function on purpose: basecamp-sdk#935 may replace the JSON
// read with a generated Go constant, and that should be a one-function swap.
func loadGeneratedVerbs(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read generated-verbs %s: %w", path, err)
	}
	var decl struct {
		Verbs []string `json:"verbs"`
	}
	if err := json.Unmarshal(data, &decl); err != nil {
		return nil, fmt.Errorf("parse generated-verbs %s: %w", path, err)
	}
	verbs := map[string]bool{}
	for _, v := range decl.Verbs {
		if strings.TrimSpace(v) != "" {
			verbs[v] = true
		}
	}
	if len(verbs) == 0 {
		return nil, fmt.Errorf("generated-verbs %s declares no usable verb (the emission bound would be empty): an absent, empty, or all-blank `verbs` array must stop the build, not be read as 'no restriction' (fix the `verbs` array in %s)", path, path)
	}
	return verbs, nil
}

// structuralPathKeys are the non-operation members of an OpenAPI Path Item
// Object. Everything in a path item that is not one of these (and not an x-*
// extension) is treated as an operation slot and MUST be a generated verb —
// walking by exclusion means an unrecognized key fails loudly rather than
// being silently skipped.
var structuralPathKeys = map[string]bool{
	"$ref":        true,
	"summary":     true,
	"description": true,
	"servers":     true,
	"parameters":  true,
}

// isStructuralPathKey reports whether a path-item key is a known non-operation
// member (or a specification extension) rather than an HTTP-method operation.
func isStructuralPathKey(key string) bool {
	return structuralPathKeys[key] || strings.HasPrefix(key, "x-")
}

type openapiOperation struct {
	OperationID string   `json:"operationId"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
	Parameters  []Param  `json:"parameters"`
	RequestBody *struct {
		Ref      string `json:"$ref"`
		Required bool   `json:"required"`
		Content  map[string]struct {
			Schema map[string]any `json:"schema"`
		} `json:"content"`
	} `json:"requestBody"`
}

// ---------------------------------------------------------------------------
// Output shapes — these mirror go/pkg/basecamp/catalog exactly. The catalog
// package's roundtrip test (DisallowUnknownFields over the committed
// catalog.json) fails if this emitter ever grows a field the loader cannot
// read, keeping the two definitions in agreement without importing across the
// module boundary.
// ---------------------------------------------------------------------------

// Catalog is the distilled, versioned tool catalog.
type Catalog struct {
	Schema       string       `json:"$schema"`
	Version      string       `json:"version"`
	ModelVersion string       `json:"modelVersion"`
	Generated    bool         `json:"generated"`
	Operations   []*Operation `json:"operations"`
}

// Operation is one API operation joined across openapi.json and
// behavior-model.json.
type Operation struct {
	ID          string          `json:"operationId"`
	Tag         string          `json:"tag"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Doc         string          `json:"doc,omitempty"`
	ReadOnly    bool            `json:"readonly"`
	Idempotent  bool            `json:"idempotent"`
	Destructive *bool           `json:"destructive,omitempty"`
	Retry       *Retry          `json:"retry,omitempty"`
	Pagination  *Pagination     `json:"pagination,omitempty"`
	Write       *WriteSemantics `json:"write,omitempty"`
	Params      []Param         `json:"params,omitempty"`
	// Body is the request-body JSON schema with $refs inlined, present only
	// for application/json bodies. Nil for operations with no body or a
	// non-JSON body (see BodyMediaType).
	Body map[string]any `json:"body,omitempty"`
	// BodyMediaType names the sole request-body media type when it is not
	// application/json (e.g. "multipart/form-data", "application/octet-stream"
	// for the binary-upload operations). Empty for JSON or bodyless operations.
	BodyMediaType string `json:"bodyMediaType,omitempty"`
	// BodyRequired reports whether the request body itself must be supplied
	// (OpenAPI requestBody.required).
	BodyRequired bool `json:"bodyRequired,omitempty"`
}

// Retry mirrors the behavior model's retry policy verbatim.
type Retry struct {
	Max              int    `json:"max"`
	BaseDelayMs      int    `json:"base_delay_ms,omitempty"`
	BaseDelaySeconds int    `json:"base_delay_seconds,omitempty"`
	Backoff          string `json:"backoff,omitempty"`
	RetryOn          []int  `json:"retry_on,omitempty"`
}

// Pagination mirrors the behavior model's pagination descriptor verbatim.
type Pagination struct {
	Style       string `json:"style"`
	MaxPageSize int    `json:"maxPageSize,omitempty"`
	Key         string `json:"key,omitempty"`
}

// WriteSemantics mirrors the behavior model's write descriptor verbatim.
type WriteSemantics struct {
	Mode                string   `json:"mode"`
	ClearsOmitted       bool     `json:"clearsOmitted,omitempty"`
	PreservedOnOmission []string `json:"preservedOnOmission,omitempty"`
}

// Param is one path or query parameter, schema inlined.
type Param struct {
	Ref         string         `json:"$ref,omitempty"`
	Name        string         `json:"name"`
	In          string         `json:"in"`
	Required    bool           `json:"required,omitempty"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

// ---------------------------------------------------------------------------
// The join
// ---------------------------------------------------------------------------

// Build joins the two model files into a Catalog. The join is strict both
// ways: an operation in one file but not the other, an operation without
// exactly one tag, or an unresolvable body $ref is a hard error. verbs is the
// emission bound (see loadGeneratedVerbs); an operation slot on any verb outside
// it is refused by name (basecamp-sdk#925), never silently skipped.
func Build(oa *openapiDoc, bm *behaviorModel, verbs map[string]bool) (*Catalog, error) {
	ops := make([]*Operation, 0, len(bm.Operations))
	seen := map[string]bool{}

	for path, item := range oa.Paths {
		for key, raw := range item {
			// Walk the path item by exclusion: skip its structural members and
			// extensions; everything else is an operation slot.
			if isStructuralPathKey(key) {
				continue
			}
			method := strings.ToLower(key)
			// Bound the catalog to the generated surface. A non-generated verb
			// (HEAD/OPTIONS/TRACE, or any unrecognized method key) is an
			// operation no SDK client exposes; emitting or silently skipping it
			// both misrepresent the surface, so refuse to build (basecamp-sdk#925).
			// The bound is the set declared in spec/generated-verbs.json.
			if !verbs[method] {
				return nil, fmt.Errorf("path %s: operation slot %q uses a verb the SDK generators don't emit — it is outside the set declared in spec/generated-verbs.json, so a catalog built from it would list an operation no client can call. This build is bounded to the generated surface on purpose (basecamp-sdk#925): if the Smithy model now legitimately serves this verb, add it to spec/generated-verbs.json only once every SDK runtime can actually serve it, don't emit it here alone", path, key)
			}
			if len(raw) == 0 || string(raw) == "null" {
				return nil, fmt.Errorf("%s %s: null operation", strings.ToUpper(method), path)
			}
			var op openapiOperation
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("%s %s: %w", strings.ToUpper(method), path, err)
			}
			if op.OperationID == "" {
				return nil, fmt.Errorf("%s %s: missing operationId", strings.ToUpper(method), path)
			}
			if seen[op.OperationID] {
				return nil, fmt.Errorf("duplicate operationId %q", op.OperationID)
			}
			seen[op.OperationID] = true

			// The catalog is the tag-based operation view: each operation is
			// grouped by its single OpenAPI tag, which is the domain a
			// tool-catalog consumer gates on. Require exactly one, and a
			// non-blank one — an empty or whitespace tag reads as "grouped"
			// while grouping nothing, and has split SDKs before (#922).
			if len(op.Tags) != 1 {
				return nil, fmt.Errorf("operation %q: expected exactly one tag, got %v", op.OperationID, op.Tags)
			}
			if strings.TrimSpace(op.Tags[0]) == "" {
				return nil, fmt.Errorf("operation %q: tag is empty or blank (%q)", op.OperationID, op.Tags[0])
			}
			if op.RequestBody != nil && op.RequestBody.Ref != "" {
				return nil, fmt.Errorf("operation %q: requestBody $ref is not supported (inline the body in the SDK export)", op.OperationID)
			}

			traits, ok := bm.Operations[op.OperationID]
			if !ok {
				return nil, fmt.Errorf("operation %q in openapi.json but not behavior-model.json", op.OperationID)
			}

			params, err := resolveParams(op.Parameters, oa)
			if err != nil {
				return nil, fmt.Errorf("operation %q: %w", op.OperationID, err)
			}
			body, mediaType, err := resolveBody(&op, oa)
			if err != nil {
				return nil, fmt.Errorf("operation %q: %w", op.OperationID, err)
			}
			if traits.ReadOnly && traits.Destructive != nil && *traits.Destructive {
				return nil, fmt.Errorf("operation %q is declared both readonly and destructive", op.OperationID)
			}

			ops = append(ops, &Operation{
				ID:            op.OperationID,
				Tag:           op.Tags[0],
				Method:        strings.ToUpper(method),
				Path:          path,
				Doc:           op.Description,
				ReadOnly:      traits.ReadOnly,
				Idempotent:    traits.Idempotent,
				Destructive:   traits.Destructive,
				Retry:         traits.Retry,
				Pagination:    traits.Pagination,
				Write:         traits.Write,
				Params:        params,
				Body:          body,
				BodyMediaType: mediaType,
				BodyRequired:  op.RequestBody != nil && op.RequestBody.Required,
			})
		}
	}

	for id := range bm.Operations {
		if !seen[id] {
			return nil, fmt.Errorf("operation %q in behavior-model.json but not openapi.json", id)
		}
	}

	sort.Slice(ops, func(i, j int) bool { return ops[i].ID < ops[j].ID })

	return &Catalog{
		Schema:       catalogSchema,
		Version:      catalogVersion,
		ModelVersion: bm.Version,
		Generated:    true,
		Operations:   ops,
	}, nil
}

// resolveParams copies parameters, inlining any schema $refs and refusing the
// Reference-Object parameter form the SDK exports never emit.
func resolveParams(params []Param, oa *openapiDoc) ([]Param, error) {
	if len(params) == 0 {
		return nil, nil
	}
	out := make([]Param, len(params))
	for i, p := range params {
		if p.Ref != "" {
			return nil, fmt.Errorf("parameter $ref %q is not supported (inline parameters in the SDK export)", p.Ref)
		}
		if p.Name == "" || p.In == "" {
			return nil, fmt.Errorf("parameter (name %q, in %q) is missing its identity", p.Name, p.In)
		}
		if p.Schema != nil {
			resolved, err := resolveRefs(p.Schema, oa, 0)
			if err != nil {
				return nil, fmt.Errorf("parameter %q: %w", p.Name, err)
			}
			p.Schema = asSchema(resolved)
		}
		out[i] = p
	}
	return out, nil
}

// resolveBody returns the request-body schema with $refs inlined for
// application/json bodies. For a single non-JSON body it returns a nil schema
// and the media type; for no body it returns nil, "".
func resolveBody(op *openapiOperation, oa *openapiDoc) (map[string]any, string, error) {
	if op.RequestBody == nil {
		return nil, "", nil
	}
	if content, ok := op.RequestBody.Content["application/json"]; ok {
		resolved, err := resolveRefs(content.Schema, oa, 0)
		if err != nil {
			return nil, "", fmt.Errorf("request body: %w", err)
		}
		schema := asSchema(resolved)
		if len(schema) == 0 {
			return nil, "", fmt.Errorf("request body has no schema (the SDK exports always constrain JSON bodies)")
		}
		return schema, "", nil
	}
	// A non-JSON body: capture its single media type (multipart/form-data or
	// application/octet-stream for the binary uploads) and carry no schema.
	types := make([]string, 0, len(op.RequestBody.Content))
	for mt := range op.RequestBody.Content {
		types = append(types, mt)
	}
	sort.Strings(types)
	if len(types) != 1 {
		return nil, "", fmt.Errorf("request body has %d media types %v (expected exactly one)", len(types), types)
	}
	return nil, types[0], nil
}

// resolveRefs inlines "#/components/schemas/*" references so bodies are
// self-contained, and drops "x-go-*" codegen extensions, which are noise to a
// catalog consumer. Sibling keys alongside a $ref are overlaid on the resolved
// target. A depth cap terminates recursive structures in a self-contained
// stub. Ported from github.com/basecamp/mcp/catalog so the SDK's inlining and
// the toolkit's agree.
func resolveRefs(v any, oa *openapiDoc, depth int) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		if ref, ok := t["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if name == ref {
				return nil, fmt.Errorf("unsupported $ref %q (only #/components/schemas/* is emitted by the SDK exports)", ref)
			}
			target, found := oa.Components.Schemas[name]
			if !found {
				return nil, fmt.Errorf("unresolvable $ref %q", ref)
			}
			if depth >= maxRefDepth {
				return map[string]any{
					"type":        "object",
					"description": fmt.Sprintf("truncated: recursive reference to %s", name),
				}, nil
			}
			rv, err := resolveRefs(deepCopy(target), oa, depth+1)
			if err != nil {
				return nil, err
			}
			resolved, _ := rv.(map[string]any)
			for k, val := range t {
				if k == "$ref" || strings.HasPrefix(k, "x-go-") {
					continue
				}
				sv, err := resolveRefs(val, oa, depth)
				if err != nil {
					return nil, err
				}
				resolved[k] = sv
			}
			return resolved, nil
		}
		out := make(map[string]any, len(t))
		for k, val := range t {
			if strings.HasPrefix(k, "x-go-") {
				continue
			}
			sv, err := resolveRefs(val, oa, depth)
			if err != nil {
				return nil, err
			}
			out[k] = sv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			sv, err := resolveRefs(val, oa, depth)
			if err != nil {
				return nil, err
			}
			out[i] = sv
		}
		return out, nil
	default:
		return v, nil
	}
}

func deepCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case map[string]any:
			out[k] = deepCopy(t)
		case []any:
			cp := make([]any, len(t))
			for i, e := range t {
				if em, ok := e.(map[string]any); ok {
					cp[i] = deepCopy(em)
				} else {
					cp[i] = e
				}
			}
			out[k] = cp
		default:
			out[k] = v
		}
	}
	return out
}

func asSchema(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}
