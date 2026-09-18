package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// rawOp marshals an operation into the json.RawMessage form a path item holds.
func rawOp(t *testing.T, op openapiOperation) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("marshal fixture op: %v", err)
	}
	return b
}

func jsonBody(schema map[string]any, required bool) *struct {
	Ref      string `json:"$ref"`
	Required bool   `json:"required"`
	Content  map[string]struct {
		Schema map[string]any `json:"schema"`
	} `json:"content"`
} {
	return &struct {
		Ref      string `json:"$ref"`
		Required bool   `json:"required"`
		Content  map[string]struct {
			Schema map[string]any `json:"schema"`
		} `json:"content"`
	}{
		Required: required,
		Content: map[string]struct {
			Schema map[string]any `json:"schema"`
		}{
			"application/json": {Schema: schema},
		},
	}
}

// fixtureDoc builds a minimal openapi doc with one component-ref body.
func fixtureDoc(t *testing.T) *openapiDoc {
	return &openapiDoc{
		Paths: map[string]map[string]json.RawMessage{
			"/{accountId}/things.json": {
				"get": rawOp(t, openapiOperation{
					OperationID: "ListThings",
					Tags:        []string{"Things"},
					Description: "List things.",
				}),
				"post": rawOp(t, openapiOperation{
					OperationID: "CreateThing",
					Tags:        []string{"Things"},
					Description: "Create a thing.",
					RequestBody: jsonBody(map[string]any{"$ref": "#/components/schemas/CreateThingRequestContent"}, true),
				}),
			},
		},
	}
}

func withSchemas(oa *openapiDoc) *openapiDoc {
	oa.Components.Schemas = map[string]map[string]any{
		"CreateThingRequestContent": {
			"type": "object",
			"properties": map[string]any{
				"name":  map[string]any{"type": "string", "x-go-name": "Name"},
				"child": map[string]any{"$ref": "#/components/schemas/Child"},
			},
			"required": []any{"name"},
		},
		"Child": {"type": "object", "properties": map[string]any{"id": map[string]any{"type": "integer"}}},
	}
	return oa
}

func doc(t *testing.T) *openapiDoc { return withSchemas(fixtureDoc(t)) }

func TestBuildJoinsAndInlines(t *testing.T) {
	cat, err := Build(doc(t), fixtureModel())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := len(cat.Operations); got != 2 {
		t.Fatalf("got %d operations, want 2", got)
	}
	if cat.ModelVersion != "9.9.9" {
		t.Errorf("ModelVersion = %q, want 9.9.9", cat.ModelVersion)
	}
	// Sorted by ID: CreateThing before ListThings.
	if cat.Operations[0].ID != "CreateThing" || cat.Operations[1].ID != "ListThings" {
		t.Errorf("operations not sorted by ID: %q, %q", cat.Operations[0].ID, cat.Operations[1].ID)
	}

	create := cat.Operations[0]
	if create.Method != "POST" || create.Tag != "Things" {
		t.Errorf("CreateThing method/tag = %q/%q", create.Method, create.Tag)
	}
	if !create.BodyRequired {
		t.Error("CreateThing should be BodyRequired")
	}
	// $ref inlined, nested $ref inlined, x-go-* stripped.
	body, _ := json.Marshal(create.Body)
	if strings.Contains(string(body), "$ref") {
		t.Errorf("body retains a $ref: %s", body)
	}
	if strings.Contains(string(body), "x-go-name") {
		t.Errorf("body retains an x-go-* extension: %s", body)
	}
	if !strings.Contains(string(body), `"child"`) || !strings.Contains(string(body), `"id"`) {
		t.Errorf("nested Child schema not inlined: %s", body)
	}
}

func fixtureModel() *behaviorModel {
	tru := true
	return &behaviorModel{
		Version: "9.9.9",
		Operations: map[string]behaviorTraits{
			"ListThings":  {ReadOnly: true, Retry: &Retry{Max: 3, BaseDelayMs: 1000, Backoff: "exponential", RetryOn: []int{429, 503}}},
			"CreateThing": {Idempotent: true, Write: &WriteSemantics{Mode: "replace", ClearsOmitted: tru}},
		},
	}
}

func TestBuildRejectsTornModel(t *testing.T) {
	// Operation in openapi but not behavior model.
	bm := fixtureModel()
	delete(bm.Operations, "ListThings")
	if _, err := Build(doc(t), bm); err == nil || !strings.Contains(err.Error(), "not behavior-model.json") {
		t.Errorf("expected torn-model error, got %v", err)
	}

	// Operation in behavior model but not openapi.
	bm2 := fixtureModel()
	bm2.Operations["Ghost"] = behaviorTraits{}
	if _, err := Build(doc(t), bm2); err == nil || !strings.Contains(err.Error(), "not openapi.json") {
		t.Errorf("expected ghost-op error, got %v", err)
	}
}

func TestBuildRejectsUntagged(t *testing.T) {
	oa := doc(t)
	oa.Paths["/{accountId}/things.json"]["get"] = rawOp(t, openapiOperation{
		OperationID: "ListThings", Description: "List things.", // no tags
	})
	if _, err := Build(oa, fixtureModel()); err == nil || !strings.Contains(err.Error(), "exactly one tag") {
		t.Errorf("expected one-tag error, got %v", err)
	}
}

func TestBuildRejectsMultipleTags(t *testing.T) {
	oa := doc(t)
	oa.Paths["/{accountId}/things.json"]["get"] = rawOp(t, openapiOperation{
		OperationID: "ListThings", Tags: []string{"Things", "Extra"}, Description: "List things.",
	})
	if _, err := Build(oa, fixtureModel()); err == nil || !strings.Contains(err.Error(), "exactly one tag") {
		t.Errorf("expected one-tag error, got %v", err)
	}
}

func TestBuildRejectsBlankTag(t *testing.T) {
	for _, blank := range []string{"", "   ", "\t"} {
		oa := doc(t)
		oa.Paths["/{accountId}/things.json"]["get"] = rawOp(t, openapiOperation{
			OperationID: "ListThings", Tags: []string{blank}, Description: "List things.",
		})
		if _, err := Build(oa, fixtureModel()); err == nil || !strings.Contains(err.Error(), "empty or blank") {
			t.Errorf("tag %q: expected blank-tag error, got %v", blank, err)
		}
	}
}

// TestBuildRejectsNonGeneratedVerb is the #925 guard: an operation on a verb
// no SDK generates must refuse to build, not silently ship a phantom op.
func TestBuildRejectsNonGeneratedVerb(t *testing.T) {
	for _, verb := range []string{"head", "options", "trace"} {
		oa := doc(t)
		oa.Paths["/{accountId}/things.json"][verb] = rawOp(t, openapiOperation{
			OperationID: "PeekThing", Tags: []string{"Things"}, Description: "Peek.",
		})
		_, err := Build(oa, fixtureModel())
		if err == nil || !strings.Contains(err.Error(), "non-generated HTTP verb") || !strings.Contains(err.Error(), "925") {
			t.Errorf("verb %q: expected #925 non-generated-verb error, got %v", verb, err)
		}
	}
}

// TestBuildExcludesStructuralPathKeys checks the exclusion walk: a path item
// carrying structural members and extensions builds cleanly, treating none of
// them as operations.
func TestBuildExcludesStructuralPathKeys(t *testing.T) {
	oa := doc(t)
	item := oa.Paths["/{accountId}/things.json"]
	item["parameters"] = json.RawMessage(`[{"name":"accountId","in":"path"}]`)
	item["summary"] = json.RawMessage(`"Things path"`)
	item["description"] = json.RawMessage(`"Everything things"`)
	item["servers"] = json.RawMessage(`[{"url":"https://example.com"}]`)
	item["x-basecamp-note"] = json.RawMessage(`"hi"`)
	cat, err := Build(oa, fixtureModel())
	if err != nil {
		t.Fatalf("Build with structural keys: %v", err)
	}
	if len(cat.Operations) != 2 {
		t.Errorf("got %d operations, want 2 (structural keys must not become operations)", len(cat.Operations))
	}
}

func TestBuildRejectsReadonlyDestructive(t *testing.T) {
	bm := fixtureModel()
	tru := true
	tr := bm.Operations["ListThings"]
	tr.Destructive = &tru
	bm.Operations["ListThings"] = tr
	if _, err := Build(doc(t), bm); err == nil || !strings.Contains(err.Error(), "readonly and destructive") {
		t.Errorf("expected readonly/destructive contradiction error, got %v", err)
	}
}

func TestBuildCarriesDestructiveTriState(t *testing.T) {
	bm := fixtureModel()
	fal := false
	tr := bm.Operations["CreateThing"]
	tr.Destructive = &fal
	bm.Operations["CreateThing"] = tr
	cat, err := Build(doc(t), bm)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	create := cat.Operations[0]
	if create.Destructive == nil || *create.Destructive != false {
		t.Errorf("Destructive = %v, want a declared false", create.Destructive)
	}
	// ListThings never declared it -> nil.
	list := cat.Operations[1]
	if list.Destructive != nil {
		t.Errorf("ListThings Destructive = %v, want nil (undeclared)", list.Destructive)
	}
}

func TestBuildRejectsUnresolvableRef(t *testing.T) {
	oa := fixtureDoc(t) // no components schemas registered
	if _, err := Build(oa, fixtureModel()); err == nil || !strings.Contains(err.Error(), "unresolvable $ref") {
		t.Errorf("expected unresolvable-ref error, got %v", err)
	}
}

func TestMarshalIsDeterministic(t *testing.T) {
	cat, err := Build(doc(t), fixtureModel())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	a, err := marshalCatalog(cat)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cat2, _ := Build(doc(t), fixtureModel())
	b, err := marshalCatalog(cat2)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(a) != string(b) {
		t.Error("marshalCatalog is not deterministic across builds")
	}
	if !strings.HasSuffix(string(a), "\n") {
		t.Error("output is not newline-terminated")
	}
	if strings.Contains(string(a), "\\u003c") {
		t.Error("HTML escaping is on; expected literal < > &")
	}
}
