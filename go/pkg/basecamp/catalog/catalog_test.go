package catalog

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c == nil {
		t.Fatal("Load returned nil catalog")
	}
	if len(c.Operations) == 0 {
		t.Fatal("catalog has no operations")
	}
	if c.Version == "" {
		t.Error("catalog is missing a version")
	}
	if c.ModelVersion == "" {
		t.Error("catalog is missing a modelVersion")
	}
	if !c.Generated {
		t.Error("catalog is not marked generated")
	}
}

// TestLoadIsCached checks Load returns the same shared instance.
func TestLoadIsCached(t *testing.T) {
	a, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	b, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a != b {
		t.Error("Load returned distinct instances; expected a cached shared catalog")
	}
}

// TestEveryOperationIsWellFormed asserts the invariant a consumer relies on:
// every operation carries the identity a tool catalog needs.
func TestEveryOperationIsWellFormed(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	seen := map[string]bool{}
	var prev string
	for i, op := range c.Operations {
		if op.ID == "" {
			t.Fatalf("operation %d has no operationId", i)
		}
		if seen[op.ID] {
			t.Errorf("operation %q appears more than once", op.ID)
		}
		seen[op.ID] = true
		if prev != "" && op.ID < prev {
			t.Errorf("operations not sorted by ID: %q before %q", prev, op.ID)
		}
		prev = op.ID

		if op.Tag == "" {
			t.Errorf("operation %q has no tag", op.ID)
		}
		if !validMethods[op.Method] {
			t.Errorf("operation %q has invalid method %q", op.ID, op.Method)
		}
		if !strings.HasPrefix(op.Path, "/") {
			t.Errorf("operation %q has non-absolute path %q", op.ID, op.Path)
		}
		if op.ReadOnly && op.IsDestructive() {
			t.Errorf("operation %q is both readonly and destructive", op.ID)
		}
	}
}

// TestBodiesAreSelfContained asserts the distillation inlined every request
// body: no unresolved component $ref survives into the embedded catalog, so a
// consumer never needs the openapi.json components section.
func TestBodiesAreSelfContained(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, op := range c.Operations {
		if op.Body != nil {
			assertNoRef(t, op.ID, "body", op.Body)
		}
		for _, p := range op.Params {
			if p.Schema != nil {
				assertNoRef(t, op.ID, "param "+p.Name, p.Schema)
			}
		}
	}
}

func assertNoRef(t *testing.T, opID, where string, v any) {
	t.Helper()
	switch m := v.(type) {
	case map[string]any:
		if ref, ok := m["$ref"]; ok {
			t.Errorf("operation %q %s retains unresolved $ref %v", opID, where, ref)
		}
		for _, val := range m {
			assertNoRef(t, opID, where, val)
		}
	case []any:
		for _, e := range m {
			assertNoRef(t, opID, where, e)
		}
	}
}

// TestByTagCoversEveryOperation checks the ByTag grouping loses nothing.
func TestByTagCoversEveryOperation(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	total := 0
	for tag, ops := range c.ByTag() {
		if tag == "" {
			t.Error("ByTag produced an empty tag key")
		}
		total += len(ops)
	}
	if total != len(c.Operations) {
		t.Errorf("ByTag covered %d operations, want %d", total, len(c.Operations))
	}
}

// TestOperationLookup exercises the ID lookup both ways.
func TestOperationLookup(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	first := c.Operations[0]
	got, ok := c.Operation(first.ID)
	if !ok || got != first {
		t.Errorf("Operation(%q) = %v, %v; want the first operation", first.ID, got, ok)
	}
	if _, ok := c.Operation("NoSuchOperationXYZ"); ok {
		t.Error("Operation returned ok for an unknown id")
	}
}
