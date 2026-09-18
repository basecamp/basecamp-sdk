// Package catalog exposes the Basecamp SDK's authoritative, embedded tool
// Catalog: one operation per API endpoint, joined from the SDK's own spec
// artifacts and shipped inside the module.
//
// The Catalog is the operation-shaped sibling of the embedded URL route table
// (see package basecamp's url.go). Where url-routes.json answers "which
// operation does this URL name?", catalog.json answers "what are all the
// operations, and everything a tool catalog needs to describe and dispatch
// each one?": operationId, tag (domain), HTTP method and path, behavior
// traits (readonly, idempotent, and — when the model carries it — destructive),
// retry and pagination policy, write semantics, and the request's parameter
// and body JSON schemas ($refs inlined so each operation is self-contained).
//
// It is generated, not hand-maintained. `make catalog` distills catalog.json
// from openapi.json + behavior-model.json (scripts/gen-catalog); `make
// catalog-check` fails if the committed artifact is not byte-identical to a
// fresh distillation, so the embedded catalog cannot drift from the spec.
//
// # The tag view of the generated surface
//
// Operation.Tag is the operation's single OpenAPI tag, and the catalog is that
// tag-based operation view — what a domain-gateway consumer groups on. It is
// NOT a claim about any per-language SDK's internal service grouping: those
// derive from each SDK's own split tables (Rust even consults names.toml
// overrides before the tag). The catalog is also bounded to the operations the
// SDKs actually generate — GET/PUT/POST/DELETE/PATCH — so it never lists a
// verb no client exposes (see scripts/gen-catalog for the generation-time
// invariants that enforce this).
//
// # For downstream consumers
//
// A tool-catalog consumer (basecamp-mcp-server and any other) depends on the
// pinned SDK module and calls Load, rather than vendoring openapi.json +
// behavior-model.json and re-joining them itself. Everything a per-operation
// tool definition needs is on Operation; the consumer supplies only the
// product-specific parts the SDK cannot know: the tool-name prefix, the
// tag→domain grouping, and the wire spelling of the action name
// (snake_case of Operation.ID).
//
// # The destructive trait
//
// Operation.Destructive is a tri-state pointer sourced only from the behavior
// model's destructive trait, which the Smithy model does not carry yet — so it
// is nil on every operation today. It is deliberately not derived from the HTTP
// method: Basecamp serves reversible toggles over DELETE (UncompleteTodo,
// UnpinMessage, DisableTool, and six more), so a method rule would misclassify
// them, while one genuinely destructive operation (TrashRecording) is a PUT.
// Destructive is a curated semantic judgment; until the Smithy trait lands, a
// consumer bridges from the action name (github.com/basecamp/mcp/catalog's
// BridgeDestructive). The moment the model declares the trait, Load surfaces it
// here and the consumer's bridge retires. See the PR and scripts/gen-catalog
// for the exact upstream fix.
package catalog

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed catalog.json
var catalogJSON []byte

// Catalog is the full set of SDK operations plus the metadata identifying the
// distillation. Treat a Catalog returned by Load as read-only: Load caches and
// shares a single instance.
type Catalog struct {
	// Schema is the JSON Schema identifier for the catalog artifact.
	Schema string `json:"$schema"`
	// Version is the catalog schema version.
	Version string `json:"version"`
	// ModelVersion is the version of the behavior model the traits came from.
	ModelVersion string `json:"modelVersion"`
	// Generated is always true for the embedded artifact.
	Generated bool `json:"generated"`
	// Operations are sorted by ID.
	Operations []*Operation `json:"operations"`
}

// Operation is one API operation with everything a tool catalog needs to list,
// describe, and dispatch it.
type Operation struct {
	// ID is the SDK operationId, e.g. "CreateTodo".
	ID string `json:"operationId"`
	// Tag is the operation's single OpenAPI tag — its domain, e.g. "Todos".
	Tag string `json:"tag"`
	// Method is the upper-case HTTP method, e.g. "POST".
	Method string `json:"method"`
	// Path is the API path template, e.g. "/{accountId}/buckets/{bucketId}/todos.json".
	Path string `json:"path"`
	// Doc is the operation's OpenAPI description.
	Doc string `json:"doc,omitempty"`
	// ReadOnly reports that the operation does not mutate server state.
	ReadOnly bool `json:"readonly"`
	// Idempotent reports that repeating the operation has the same effect as
	// performing it once.
	Idempotent bool `json:"idempotent"`
	// Destructive reports whether the operation deletes or irreversibly alters
	// data. Tri-state: nil means the behavior model does not declare the trait
	// (the state today — see the package doc). Non-nil is authoritative.
	Destructive *bool `json:"destructive,omitempty"`
	// Retry is the operation's declared retry policy.
	Retry *Retry `json:"retry,omitempty"`
	// Pagination describes how a list operation pages; nil when not paginated.
	Pagination *Pagination `json:"pagination,omitempty"`
	// Write declares how the server interprets the request body on write —
	// most importantly whether omitted fields are cleared.
	Write *WriteSemantics `json:"write,omitempty"`
	// Params are the path and query parameters, with schema $refs inlined.
	Params []Param `json:"params,omitempty"`
	// Body is the request-body JSON schema with $refs inlined, present only for
	// application/json bodies. Nil for a bodyless operation or a non-JSON body
	// (see BodyMediaType). The top level is left as the SDK emitted it — a
	// consumer that wants unknown-property rejection stamps it strict itself.
	Body map[string]any `json:"body,omitempty"`
	// BodyMediaType names the sole request-body media type when it is not
	// application/json (e.g. "multipart/form-data", "application/octet-stream").
	// Empty for JSON or bodyless operations.
	BodyMediaType string `json:"bodyMediaType,omitempty"`
	// BodyRequired reports whether the request body itself must be supplied.
	BodyRequired bool `json:"bodyRequired,omitempty"`
}

// Paginated reports whether the operation pages its results.
func (o *Operation) Paginated() bool { return o != nil && o.Pagination != nil }

// IsDestructive reports the operation's destructive trait as a plain bool,
// treating an undeclared (nil) trait as false. Callers that must distinguish
// "declared not destructive" from "undeclared" read Destructive directly.
func (o *Operation) IsDestructive() bool {
	return o != nil && o.Destructive != nil && *o.Destructive
}

// Retry is an operation's declared retry policy, mirroring the behavior model.
type Retry struct {
	Max              int    `json:"max"`
	BaseDelayMs      int    `json:"base_delay_ms,omitempty"`
	BaseDelaySeconds int    `json:"base_delay_seconds,omitempty"`
	Backoff          string `json:"backoff,omitempty"`
	RetryOn          []int  `json:"retry_on,omitempty"`
}

// Pagination describes how a list operation pages, in the behavior model's
// own vocabulary.
type Pagination struct {
	// Style is the pagination mechanism: "link" (RFC 5988 Link header) or
	// "cursor".
	Style string `json:"style"`
	// MaxPageSize is the server's page-size cap.
	MaxPageSize int `json:"maxPageSize,omitempty"`
	// Key is the response-object key holding the paginated array, when the
	// response is a wrapper object rather than a bare array.
	Key string `json:"key,omitempty"`
}

// WriteSemantics declares how the server interprets a write's request body.
// Mode "replace" with ClearsOmitted true means any writable field omitted from
// the request is cleared server-side, except the fields in PreservedOnOmission.
type WriteSemantics struct {
	Mode                string   `json:"mode"`
	ClearsOmitted       bool     `json:"clearsOmitted,omitempty"`
	PreservedOnOmission []string `json:"preservedOnOmission,omitempty"`
}

// Param is one path or query parameter of an operation, schema inlined.
type Param struct {
	Name        string         `json:"name"`
	In          string         `json:"in"`
	Required    bool           `json:"required,omitempty"`
	Description string         `json:"description,omitempty"`
	Schema      map[string]any `json:"schema,omitempty"`
}

var (
	loaded    *Catalog
	loadOnce  sync.Once
	errLoaded error
)

// Load returns the embedded catalog, parsed once and shared across calls. The
// returned *Catalog is shared; treat it as read-only. Load never fails for the
// embedded artifact in a correctly built module (the catalog-check gate and the
// package tests guarantee it parses), but it returns an error rather than
// panicking so callers can surface a corrupt build cleanly.
func Load() (*Catalog, error) {
	loadOnce.Do(func() {
		loaded, errLoaded = parse(catalogJSON)
	})
	return loaded, errLoaded
}

// parse decodes catalog bytes, rejecting any field the Operation/Catalog types
// do not model so a drifted artifact fails here rather than losing data
// silently.
func parse(data []byte) (*Catalog, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Catalog
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("catalog: parse embedded catalog.json: %w", err)
	}
	if len(c.Operations) == 0 {
		return nil, fmt.Errorf("catalog: embedded catalog.json has no operations")
	}
	return &c, nil
}

// Operations returns the embedded catalog's operations, sorted by ID. It is a
// convenience over Load for callers that need only the slice.
func Operations() ([]*Operation, error) {
	c, err := Load()
	if err != nil {
		return nil, err
	}
	return c.Operations, nil
}

// Operation returns the operation with the given operationId.
func (c *Catalog) Operation(id string) (*Operation, bool) {
	for _, op := range c.Operations {
		if op.ID == id {
			return op, true
		}
	}
	return nil, false
}

// ByTag groups the catalog's operations by tag, each group sorted by ID (the
// order they already hold in Operations).
func (c *Catalog) ByTag() map[string][]*Operation {
	out := make(map[string][]*Operation)
	for _, op := range c.Operations {
		out[op.Tag] = append(out[op.Tag], op)
	}
	return out
}
