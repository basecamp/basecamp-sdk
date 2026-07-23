package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractJSONTag(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"`json:\"foo\"`", "foo"},
		{"`json:\"foo,omitempty\"`", "foo"},
		{"`json:\"foo,omitempty\" xml:\"bar\"`", "foo"},
		{"`xml:\"bar\" json:\"foo\"`", "foo"},
		{"`json:\"-\"`", "-"},
		{"`xml:\"bar\"`", ""},
		{"", ""},
		{"`json:\"\"`", ""},
	}
	for _, c := range cases {
		got := extractJSONTag(c.in)
		if got != c.want {
			t.Errorf("extractJSONTag(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCollectStructs_TagsAndOmittedMarkers(t *testing.T) {
	src := `package fixture

// Wrapper has two fields and two intentionally-omitted markers sitting on
// their own lines inside the struct body.
type Wrapper struct {
	Foo string ` + "`json:\"foo\"`" + `
	// intentionally-omitted: secret_field - never expose
	// intentionally-omitted: another_field - not user-visible
	Bar int ` + "`json:\"bar,omitempty\"`" + `
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fixture.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	structs := collectStructsAndMarkers(fset, f)
	w, ok := structs["Wrapper"]
	if !ok {
		t.Fatal("expected Wrapper struct to be collected")
	}
	if !w.tags["foo"] || !w.tags["bar"] {
		t.Errorf("expected tags foo+bar, got %v", w.tags)
	}
	if !w.omitted["secret_field"] {
		t.Errorf("expected omitted secret_field, got %v", w.omitted)
	}
	if !w.omitted["another_field"] {
		t.Errorf("expected omitted another_field, got %v", w.omitted)
	}
}

func TestCollectFromGeneratedPairs(t *testing.T) {
	// Each exclusion case uses a DISTINCT return type so its absence from the
	// pair map is a meaningful assertion: a regression that started accepting the
	// excluded shape would surface that type as a key. (The previous version
	// asserted on "Foo", a type no fixture produced, so it could never fail.)
	src := `package fixture

import "generated"

// barFromGenerated maps generated.Bar to Bar. This is the one valid pair.
func barFromGenerated(g generated.Bar) Bar { return Bar{} }

// receiverFnFromGenerated is a method returning Recv, must be skipped.
func (s *Service) receiverFnFromGenerated(g generated.Recv) Recv { return Recv{} }

// unqualifiedParamFromGenerated has an unqualified (non-generated.X) param and
// returns the distinct type Unqualified, so its exclusion is observable.
func unqualifiedParamFromGenerated(g Unqualified) Unqualified { return Unqualified{} }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fixture.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pairs := collectFromGeneratedPairs(f)
	if got := pairs["Bar"]; got != "Bar" {
		t.Errorf("expected Bar -> Bar pair, got %q", got)
	}
	if _, ok := pairs["Recv"]; ok {
		t.Error("method receiver fn must be excluded from pair extraction")
	}
	if _, ok := pairs["Unqualified"]; ok {
		t.Error("function with a non-generated.X param must be excluded from pair extraction")
	}
	// The only pair must be Bar; nothing leaked from the excluded shapes.
	if len(pairs) != 1 {
		t.Errorf("expected exactly one pair (Bar), got %d: %v", len(pairs), pairs)
	}
}

func TestExtractJSONTag_MultipleKeysIntermixed(t *testing.T) {
	// Defensive: a tag that uses an exotic ordering should still resolve.
	got := extractJSONTag("`xml:\"x_bar\" json:\"the_json,omitempty\" yaml:\"yam\"`")
	if got != "the_json" {
		t.Errorf("expected the_json, got %q", got)
	}
}

func TestMarkerRegex_RequiresReason(t *testing.T) {
	cases := []struct {
		in      string
		match   bool
		capture string
	}{
		{"// intentionally-omitted: foo - because", true, "foo"},
		{"// intentionally-omitted: foo - x", true, "foo"},
		{"// intentionally-omitted: foo -", false, ""},
		{"// intentionally-omitted: foo  ", false, ""},
		{"// not-the-marker: foo - reason", false, ""},
	}
	for _, c := range cases {
		m := markerRe.FindStringSubmatch(c.in)
		if c.match {
			if m == nil {
				t.Errorf("expected match for %q", c.in)
				continue
			}
			if m[1] != c.capture {
				t.Errorf("for %q expected capture %q, got %q", c.in, c.capture, m[1])
			}
		} else if m != nil {
			t.Errorf("expected no match for %q, got %v", c.in, m)
		}
	}
}

// writeDriftFixtures writes a generated client file and one or more wrapper
// files into a temp tree laid out the way run() expects (a wrapper dir + a
// separate generated file path) and returns the two paths. This lets tests
// drive the real run() entry point end-to-end instead of reimplementing the
// check's internals.
func writeDriftFixtures(t *testing.T, genSrc string, wrapperSrcByName map[string]string) (wrapperDir, generatedFile string) {
	t.Helper()
	root := t.TempDir()
	wrapperDir = filepath.Join(root, "wrappers")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatalf("mkdir wrappers: %v", err)
	}
	generatedFile = filepath.Join(root, "client.gen.go")
	if err := os.WriteFile(generatedFile, []byte(genSrc), 0o644); err != nil {
		t.Fatalf("write generated: %v", err)
	}
	for name, src := range wrapperSrcByName {
		if err := os.WriteFile(filepath.Join(wrapperDir, name), []byte(src), 0o644); err != nil {
			t.Fatalf("write wrapper %s: %v", name, err)
		}
	}
	return wrapperDir, generatedFile
}

// TestRun_InSync drives the real run() over a tree where every generated tag is
// either propagated + assigned or intentionally-omitted. run() must return nil.
func TestRun_InSync(t *testing.T) {
	genSrc := `package generated

type Foo struct {
	Id     int64  ` + "`json:\"id\"`" + `
	Title  string ` + "`json:\"title\"`" + `
	Hidden string ` + "`json:\"hidden,omitempty\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Foo struct {
	ID    int64  ` + "`json:\"id\"`" + `
	Title string ` + "`json:\"title\"`" + `
	// intentionally-omitted: hidden - internal echo, not part of the public surface
	internalNote string
}

func fooFromGenerated(g generated.Foo) Foo {
	f := Foo{Title: g.Title}
	f.ID = g.Id
	return f
}
`
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"foo.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: expected no drift, got %v", err)
	}
}

// TestRun_MissingTag drives run() over a wrapper missing a generated tag with no
// marker. run() must return a drift error.
func TestRun_MissingTag(t *testing.T) {
	genSrc := `package generated

type Bar struct {
	Id       int64  ` + "`json:\"id\"`" + `
	Name     string ` + "`json:\"name\"`" + `
	NewField string ` + "`json:\"new_field,omitempty\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Bar struct {
	ID   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func barFromGenerated(g generated.Bar) Bar {
	b := Bar{Name: g.Name}
	b.ID = g.Id
	return b
}
`
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"bar.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected drift on missing tag new_field, got nil")
	}
}

// TestRun_TagPresentButUnassigned is the P1 regression: a wrapper that DECLARES
// the right tag but whose *FromGenerated never assigns the field must still be
// caught. This is exactly the case the tag-only check let through.
func TestRun_TagPresentButUnassigned(t *testing.T) {
	genSrc := `package generated

type Baz struct {
	Id      int64  ` + "`json:\"id\"`" + `
	Tagline string ` + "`json:\"tagline\"`" + `
}
`
	// Tagline carries the right tag but bazFromGenerated never assigns it.
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Baz struct {
	ID      int64  ` + "`json:\"id\"`" + `
	Tagline string ` + "`json:\"tagline\"`" + `
}

func bazFromGenerated(g generated.Baz) Baz {
	b := Baz{}
	b.ID = g.Id
	return b
}
`
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"baz.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected population drift on unassigned Tagline, got nil")
	}
}

// TestRun_HelperLocalDoesNotMaskDrift is the end-to-end soundness regression for
// the scoped population walk. The wrapper declares the `name` tag but its
// *FromGenerated never assigns the wrapper's Name field — it only writes
// `child.Name` on a helper local that happens to share the field name. The old
// broad walk attributed `child.Name` to the wrapper and let this pass; the
// scoped walk must report `name` as unpopulated drift.
func TestRun_HelperLocalDoesNotMaskDrift(t *testing.T) {
	genSrc := `package generated

type Wrap struct {
	Id   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Child struct {
	Name string ` + "`json:\"name\"`" + `
}

type Wrap struct {
	ID    int64  ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Child *Child ` + "`json:\"child,omitempty\"`" + `
}

func wrapFromGenerated(g generated.Wrap) Wrap {
	w := Wrap{}
	w.ID = g.Id
	// Only the helper local's Name is written, never w.Name.
	child := Child{}
	child.Name = g.Name
	w.Child = &child
	return w
}
`
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"wrap.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected population drift on Wrap.Name (only a helper local assigns name), got nil")
	}
}

// TestRun_AssignedViaSelectorAndCompositeLit confirms both assignment forms the
// population walker recognizes count: a field set in the composite literal and a
// field set via a later `x.Field = ...` statement.
func TestRun_AssignedViaSelectorAndCompositeLit(t *testing.T) {
	genSrc := `package generated

type Qux struct {
	Id    int64  ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Title string ` + "`json:\"title\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Qux struct {
	ID    int64  ` + "`json:\"id\"`" + `
	Name  string ` + "`json:\"name\"`" + `
	Title string ` + "`json:\"title\"`" + `
}

func quxFromGenerated(g generated.Qux) Qux {
	q := Qux{Name: g.Name}
	q.ID = g.Id
	q.Title = g.Title
	return q
}
`
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"qux.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: expected no drift (all fields assigned), got %v", err)
	}
}

// TestRun_OmitMarkerMismatch confirms run() flags an intentionally-omitted
// marker that names a tag the generated struct does not emit.
func TestRun_OmitMarkerMismatch(t *testing.T) {
	genSrc := `package generated

type Foo struct {
	Id int64 ` + "`json:\"id\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Foo struct {
	ID int64 ` + "`json:\"id\"`" + `
	// intentionally-omitted: not_a_real_tag - typo that should be flagged
	note string
}

func fooFromGenerated(g generated.Foo) Foo {
	f := Foo{}
	f.ID = g.Id
	return f
}
`
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"foo.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected drift on stale omit marker not_a_real_tag, got nil")
	}
}

// TestRun_DirectDecodeRenamedPair drives run() with a direct-decode pair whose
// wrapper name differs from the generated type — the shape used by the
// MyAssignmentsResult ↔ GetMyAssignmentsResponseContent and similar entries in
// the production directDecodePairs map. Two assertions matter: (1) the pair is
// walked via the injected directDecode map even with no *FromGenerated function,
// and (2) the tag-presence check fires on a missing generated tag.
func TestRun_DirectDecodeRenamedPair(t *testing.T) {
	genSrc := `package generated

type GetMyAssignmentsResponseContent struct {
	NonPriorities []MyAssignment ` + "`json:\"non_priorities,omitempty\"`" + `
	Priorities    []MyAssignment ` + "`json:\"priorities,omitempty\"`" + `
}
type MyAssignment struct {
	Id int64 ` + "`json:\"id\"`" + `
}
`
	// Wrapper has both tags — clean run.
	wrapperOK := `package basecamp

type MyAssignment struct {
	ID int64 ` + "`json:\"id\"`" + `
}
type MyAssignmentsResult struct {
	Priorities    []MyAssignment ` + "`json:\"priorities,omitempty\"`" + `
	NonPriorities []MyAssignment ` + "`json:\"non_priorities,omitempty\"`" + `
}
`
	pairs := map[string]string{
		"MyAssignmentsResult": "GetMyAssignmentsResponseContent",
		"MyAssignment":        "MyAssignment",
	}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"my_assignments.go": wrapperOK})
	if err := run(wrapperDir, generatedFile, pairs, nil, false); err != nil {
		t.Errorf("run (in-sync renamed direct-decode pair): expected no drift, got %v", err)
	}

	// Wrapper drops the non_priorities tag with no marker — drift expected.
	wrapperMissing := `package basecamp

type MyAssignment struct {
	ID int64 ` + "`json:\"id\"`" + `
}
type MyAssignmentsResult struct {
	Priorities []MyAssignment ` + "`json:\"priorities,omitempty\"`" + `
}
`
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"my_assignments.go": wrapperMissing})
	if err := run(wrapperDir, generatedFile, pairs, nil, false); err == nil {
		t.Error("run (renamed direct-decode pair missing non_priorities): expected drift, got nil")
	}
}

// TestRun_InlineConvertedPair drives run() with a tier-3 pair: the wrapper has
// no *FromGenerated of its own and is populated by a composite literal inside a
// parent's *FromGenerated body — the shape the production directDecodePairs
// tier-3 entries (CampfireLineAttachment, EventDetails, etc.) follow. Two
// assertions matter: (1) the pair is walked via the injected directDecode map
// despite no *FromGenerated function for the inline-converted wrapper, and (2)
// the tag-presence check fires on a missing generated tag — exactly the
// regression the tier exists to catch (parent's body silently dropping a new
// generated field).
func TestRun_InlineConvertedPair(t *testing.T) {
	genSrc := `package generated

type Parent struct {
	Id     int64 ` + "`json:\"id\"`" + `
	Nested Nested ` + "`json:\"nested,omitempty\"`" + `
}
type Nested struct {
	Name  string ` + "`json:\"name\"`" + `
	Color string ` + "`json:\"color\"`" + `
}
`
	// In-sync wrapper: Nested carries both tags and the parent's *FromGenerated
	// builds it inline. Only Parent has a *FromGenerated; Nested is tier 3.
	wrapperOK := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Nested struct {
	Name  string ` + "`json:\"name\"`" + `
	Color string ` + "`json:\"color\"`" + `
}
type Parent struct {
	ID     int64   ` + "`json:\"id\"`" + `
	Nested *Nested ` + "`json:\"nested,omitempty\"`" + `
}

func parentFromGenerated(g generated.Parent) Parent {
	p := Parent{}
	p.ID = g.Id
	p.Nested = &Nested{Name: g.Nested.Name, Color: g.Nested.Color}
	return p
}
`
	pairs := map[string]string{"Nested": "Nested"}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperOK})
	if err := run(wrapperDir, generatedFile, pairs, nil, false); err != nil {
		t.Errorf("run (in-sync inline-converted pair): expected no drift, got %v", err)
	}

	// Wrapper drops the `color` tag with no marker — drift expected.
	wrapperMissing := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Nested struct {
	Name string ` + "`json:\"name\"`" + `
}
type Parent struct {
	ID     int64   ` + "`json:\"id\"`" + `
	Nested *Nested ` + "`json:\"nested,omitempty\"`" + `
}

func parentFromGenerated(g generated.Parent) Parent {
	p := Parent{}
	p.ID = g.Id
	p.Nested = &Nested{Name: g.Nested.Name}
	return p
}
`
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperMissing})
	if err := run(wrapperDir, generatedFile, pairs, nil, false); err == nil {
		t.Error("run (inline-converted pair missing nested color tag): expected drift, got nil")
	}
}

// TestCollectAssignedFields verifies the walker collects fields from both the
// wrapper composite literal and selector assignments, and does NOT collect keys
// from nested helper literals (Parent/Bucket) — the one-level-nesting boundary.
func TestCollectAssignedFields(t *testing.T) {
	src := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Thing struct {
	ID     int64
	Status string
	Parent *Parent
}

func thingFromGenerated(g generated.Thing) Thing {
	t := Thing{Status: g.Status}
	t.ID = g.Id
	if g.Parent.Id != 0 {
		t.Parent = &Parent{ID: g.Parent.Id, Title: g.Parent.Title}
	}
	return t
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "wrapper.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := collectAssignedFields(f)["Thing"]
	for _, want := range []string{"Status", "ID", "Parent"} {
		if !got[want] {
			t.Errorf("expected %q to be collected as assigned, got %v", want, got)
		}
	}
	// Title is a key on the nested &Parent{} literal, NOT a Thing field — it
	// must not leak into Thing's assigned set.
	if got["Title"] {
		t.Errorf("nested literal key Title must not be attributed to Thing: %v", got)
	}
}

// TestCollectAssignedFields_HelperLocalSelectorExcluded is the soundness
// regression for the scoped population walk. A *FromGenerated body routinely
// builds a nested helper value via its own local and writes that local's fields
// by selector (here `child.Name = ...` on a `child := Child{}`). Those writes
// must NOT be attributed to the wrapper, even when the helper local shares a
// field name with the wrapper (`Name`). Under the old broad walk — which
// counted every `x.Field = ...` regardless of base — `Name` would be falsely
// marked assigned on the wrapper, masking the fact that the wrapper itself never
// assigns it.
func TestCollectAssignedFields_HelperLocalSelectorExcluded(t *testing.T) {
	src := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Wrap struct {
	ID    int64
	Name  string
	Child *Child
}

func wrapFromGenerated(g generated.Wrap) Wrap {
	w := Wrap{}
	w.ID = g.Id
	// Helper local of a different type; its Name field shares the wrapper's
	// field name but must not count toward the wrapper.
	child := Child{}
	child.Name = g.Child.Name
	w.Child = &child
	return w
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "wrapper.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := collectAssignedFields(f)["Wrap"]
	if !got["ID"] {
		t.Errorf("expected ID (written on the wrapper var) to be collected, got %v", got)
	}
	if !got["Child"] {
		t.Errorf("expected Child (written on the wrapper var) to be collected, got %v", got)
	}
	// The wrapper never assigns its own Name; only the helper local does. The
	// scoped walk must not attribute the helper-local write to the wrapper.
	if got["Name"] {
		t.Errorf("helper-local selector write (child.Name) must not count as wrapper Wrap.Name: %v", got)
	}
}

// TestExcludedFromGenerated verifies that the special-cased mapping
// (webhookPersonFromGenerated → WebhookEventPerson, NOT Person) is skipped
// during automatic pair discovery so the drift check doesn't double-count
// generated.Person as the parent for two unrelated wrappers.
func TestExcludedFromGenerated(t *testing.T) {
	src := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

func webhookPersonFromGenerated(g generated.Person) WebhookEventPerson {
	return WebhookEventPerson{}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "wrapper.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pairs := collectFromGeneratedPairs(f)
	if _, ok := pairs["WebhookEventPerson"]; ok {
		t.Error("webhookPersonFromGenerated should be excluded from auto-discovered pairs")
	}
}

// TestExtractJSONTag_DashSentinel covers the edge case of `json:"-"`, which
// reflect treats as "skip this field". The drift check matches on the literal
// tag value, so `-` is treated like any other JSON tag name. The check still
// holds: a generated struct field with tag `-` would not normally exist
// (oapi-codegen doesn't emit them), but the parser must not crash on it.
func TestExtractJSONTag_DashSentinel(t *testing.T) {
	if !strings.HasPrefix(extractJSONTag("`json:\"-,omitempty\"`"), "-") {
		t.Error("expected `-` to be captured from `json:\"-,omitempty\"`")
	}
}

// TestRun_Tier3PointerLiteralInSync drives run() with a tier-3 pair populated
// by the pointer `Field: &Wrapper{...}` form inside a parent *FromGenerated.
// Every generated tag on the tier-3 wrapper is assigned by the composite
// literal, so the population check must pass.
func TestRun_Tier3PointerLiteralInSync(t *testing.T) {
	genSrc := `package generated

type Parent struct {
	Id     int64  ` + "`json:\"id\"`" + `
	OnHold OnHold ` + "`json:\"on_hold,omitempty\"`" + `
}
type OnHold struct {
	Id     int64  ` + "`json:\"id\"`" + `
	Status string ` + "`json:\"status\"`" + `
	Title  string ` + "`json:\"title\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type OnHold struct {
	ID     int64  ` + "`json:\"id\"`" + `
	Status string ` + "`json:\"status\"`" + `
	Title  string ` + "`json:\"title\"`" + `
}
type Parent struct {
	ID     int64   ` + "`json:\"id\"`" + `
	OnHold *OnHold ` + "`json:\"on_hold,omitempty\"`" + `
}

func parentFromGenerated(g generated.Parent) Parent {
	p := Parent{}
	p.ID = g.Id
	if g.OnHold.Id != 0 {
		p.OnHold = &OnHold{
			ID:     g.OnHold.Id,
			Status: g.OnHold.Status,
			Title:  g.OnHold.Title,
		}
	}
	return p
}
`
	pairs := map[string]string{"OnHold": "OnHold"}
	tier3 := map[string]bool{"OnHold": true}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, pairs, tier3, false); err != nil {
		t.Errorf("run (tier-3 pointer literal in sync): expected no drift, got %v", err)
	}
}

// TestRun_Tier3PointerLiteralDroppedAssignment is the teeth proof for the
// composite-literal walker: the wrapper declares the right tags, but the
// inline `&OnHold{...}` in the parent's body silently drops one assignment.
// The new tier-3 population check must catch it. Before this change the same
// fixture would have passed (tier-3 was tag-presence-only / reviewer-enforced).
func TestRun_Tier3PointerLiteralDroppedAssignment(t *testing.T) {
	genSrc := `package generated

type Parent struct {
	Id     int64  ` + "`json:\"id\"`" + `
	OnHold OnHold ` + "`json:\"on_hold,omitempty\"`" + `
}
type OnHold struct {
	Id     int64  ` + "`json:\"id\"`" + `
	Status string ` + "`json:\"status\"`" + `
	Title  string ` + "`json:\"title\"`" + `
}
`
	// Title tag is declared on the wrapper but the composite literal omits it.
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type OnHold struct {
	ID     int64  ` + "`json:\"id\"`" + `
	Status string ` + "`json:\"status\"`" + `
	Title  string ` + "`json:\"title\"`" + `
}
type Parent struct {
	ID     int64   ` + "`json:\"id\"`" + `
	OnHold *OnHold ` + "`json:\"on_hold,omitempty\"`" + `
}

func parentFromGenerated(g generated.Parent) Parent {
	p := Parent{}
	p.ID = g.Id
	if g.OnHold.Id != 0 {
		p.OnHold = &OnHold{
			ID:     g.OnHold.Id,
			Status: g.OnHold.Status,
		}
	}
	return p
}
`
	pairs := map[string]string{"OnHold": "OnHold"}
	tier3 := map[string]bool{"OnHold": true}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperSrc})
	err := run(wrapperDir, generatedFile, pairs, tier3, false)
	if err == nil {
		t.Fatal("run (tier-3 dropped assignment): expected population drift on Title, got nil")
	}
	if !strings.Contains(err.Error(), "wrapper drift") {
		t.Errorf("run: expected wrapper drift error, got %v", err)
	}
}

// TestRun_Tier3BareLiteralInSync covers the bare `Wrapper{...}` (non-pointer)
// construction form — the shape LineupMarker, HillChartDot, SearchType, and
// CampfireLineAttachment take inside an append/index-assign. Every generated
// tag is assigned by the literal, so the population check must pass.
func TestRun_Tier3BareLiteralInSync(t *testing.T) {
	genSrc := `package generated

type ParentList struct {
	Markers []Marker ` + "`json:\"markers,omitempty\"`" + `
}
type Marker struct {
	Id   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Marker struct {
	ID   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
type ParentList struct {
	Markers []Marker ` + "`json:\"markers,omitempty\"`" + `
}

func parentListFromGenerated(g generated.ParentList) ParentList {
	pl := ParentList{}
	for _, gm := range g.Markers {
		pl.Markers = append(pl.Markers, Marker{
			ID:   gm.Id,
			Name: gm.Name,
		})
	}
	return pl
}
`
	pairs := map[string]string{"Marker": "Marker"}
	tier3 := map[string]bool{"Marker": true}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, pairs, tier3, false); err != nil {
		t.Errorf("run (tier-3 bare literal in sync): expected no drift, got %v", err)
	}
}

// TestRun_Tier3BareLiteralDroppedAssignment proves the bare-literal form also
// catches dropped assignments — a regression check independent of the pointer
// form. The wrapper declares both tags but the inline literal in the for-loop
// silently drops Name.
func TestRun_Tier3BareLiteralDroppedAssignment(t *testing.T) {
	genSrc := `package generated

type ParentList struct {
	Markers []Marker ` + "`json:\"markers,omitempty\"`" + `
}
type Marker struct {
	Id   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Marker struct {
	ID   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}
type ParentList struct {
	Markers []Marker ` + "`json:\"markers,omitempty\"`" + `
}

func parentListFromGenerated(g generated.ParentList) ParentList {
	pl := ParentList{}
	for _, gm := range g.Markers {
		pl.Markers = append(pl.Markers, Marker{
			ID: gm.Id,
		})
	}
	return pl
}
`
	pairs := map[string]string{"Marker": "Marker"}
	tier3 := map[string]bool{"Marker": true}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, pairs, tier3, false); err == nil {
		t.Error("run (tier-3 bare literal missing Name): expected population drift, got nil")
	}
}

// TestRun_Tier3LocalBoundSelectorWrites covers the shape that
// ClientApprovalResponse and UpdateProjectAccessResponse take in the real
// corpus: the wrapper is bound to a local via `resp := Wrapper{...}` and then
// fields are written by subsequent `resp.X = ...` selector statements. The
// walker must attribute those writes to the wrapper, so every generated tag
// counts as populated.
func TestRun_Tier3LocalBoundSelectorWrites(t *testing.T) {
	genSrc := `package generated

type ParentList struct {
	Items []Item ` + "`json:\"items,omitempty\"`" + `
}
type Item struct {
	Id     int64  ` + "`json:\"id\"`" + `
	Status string ` + "`json:\"status\"`" + `
	Title  string ` + "`json:\"title\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Item struct {
	ID     int64  ` + "`json:\"id\"`" + `
	Status string ` + "`json:\"status\"`" + `
	Title  string ` + "`json:\"title\"`" + `
}
type ParentList struct {
	Items []Item ` + "`json:\"items,omitempty\"`" + `
}

func parentListFromGenerated(g generated.ParentList) ParentList {
	pl := ParentList{}
	for _, gi := range g.Items {
		resp := Item{Status: gi.Status}
		resp.ID = gi.Id
		resp.Title = gi.Title
		pl.Items = append(pl.Items, resp)
	}
	return pl
}
`
	pairs := map[string]string{"Item": "Item"}
	tier3 := map[string]bool{"Item": true}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, pairs, tier3, false); err != nil {
		t.Errorf("run (tier-3 local-bound + selector writes): expected no drift, got %v", err)
	}
}

// TestRun_Tier3SelectorChainBoundWrites covers the shape QuestionSchedule
// takes: the wrapper is bound to a selector chain (`q.Schedule =
// &Wrapper{...}`) and conditional `q.Schedule.X = ...` writes set the
// remaining fields. The walker must track selector-chain bindings, not just
// bare-identifier locals.
func TestRun_Tier3SelectorChainBoundWrites(t *testing.T) {
	genSrc := `package generated

type Parent struct {
	Schedule Schedule ` + "`json:\"schedule,omitempty\"`" + `
}
type Schedule struct {
	Frequency    string ` + "`json:\"frequency\"`" + `
	WeekInstance int32  ` + "`json:\"week_instance,omitempty\"`" + `
}
`
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Schedule struct {
	Frequency    string ` + "`json:\"frequency\"`" + `
	WeekInstance *int   ` + "`json:\"week_instance,omitempty\"`" + `
}
type Parent struct {
	Schedule *Schedule ` + "`json:\"schedule,omitempty\"`" + `
}

func parentFromGenerated(g generated.Parent) Parent {
	q := Parent{}
	if g.Schedule.Frequency != "" {
		q.Schedule = &Schedule{
			Frequency: g.Schedule.Frequency,
		}
		if g.Schedule.WeekInstance != 0 {
			wi := int(g.Schedule.WeekInstance)
			q.Schedule.WeekInstance = &wi
		}
	}
	return q
}
`
	pairs := map[string]string{"Schedule": "Schedule"}
	tier3 := map[string]bool{"Schedule": true}
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"parent.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, pairs, tier3, false); err != nil {
		t.Errorf("run (tier-3 selector-chain binding): expected no drift, got %v", err)
	}
}

// TestCollectCompositeLiteralFields verifies the walker collects keys from
// both bare and pointer composite literals, attributes subsequent selector
// writes against bound locals (`resp.X`) and bound selector chains
// (`q.Schedule.X`), and ignores composite literals of types not in tier3.
func TestCollectCompositeLiteralFields(t *testing.T) {
	src := `package basecamp

type Marker struct{ ID int64; Name string }
type Item struct{ ID int64; Status string; Title string }
type Schedule struct{ Frequency string; WeekInstance *int }
type Other struct{ X string } // not in tier3; must be ignored
type wrap struct{ M *Marker }
type parent struct{ Schedule *Schedule }

func _build() {
	// Bare literal inside a slice — keys must be collected.
	_ = []Marker{{ID: 1, Name: "n"}}

	// Pointer literal as a field assignment.
	w := wrap{}
	w.M = &Marker{ID: 2, Name: "n2"}

	// Local-bound + selector writes.
	resp := Item{Status: "active"}
	resp.ID = 3
	resp.Title = "t"
	_ = resp

	// Selector-chain binding + chain selector writes.
	q := parent{}
	q.Schedule = &Schedule{Frequency: "daily"}
	wi := 1
	q.Schedule.WeekInstance = &wi

	// Other type — must not appear in output.
	_ = Other{X: "ignored"}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "wrapper.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tier3 := map[string]bool{"Marker": true, "Item": true, "Schedule": true}
	got := collectCompositeLiteralFields(f, tier3)
	for _, want := range []string{"ID", "Name"} {
		if !got["Marker"][want] {
			t.Errorf("Marker: expected %q in assigned set, got %v", want, got["Marker"])
		}
	}
	for _, want := range []string{"ID", "Status", "Title"} {
		if !got["Item"][want] {
			t.Errorf("Item: expected %q in assigned set, got %v", want, got["Item"])
		}
	}
	for _, want := range []string{"Frequency", "WeekInstance"} {
		if !got["Schedule"][want] {
			t.Errorf("Schedule: expected %q in assigned set, got %v", want, got["Schedule"])
		}
	}
	if _, ok := got["Other"]; ok {
		t.Errorf("Other is not in tier3 and must not be in the output: %v", got["Other"])
	}
}

// TestCollectCompositeLiteralFields_EmptyTier3 confirms the walker is a no-op
// when tier3 is empty — preserves the tier-2-only semantics callers rely on
// when they don't want any composite-literal sourcing.
func TestCollectCompositeLiteralFields_EmptyTier3(t *testing.T) {
	src := `package basecamp

type Marker struct{ ID int64 }

func _f() { _ = []Marker{{ID: 1}} }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "wrapper.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := collectCompositeLiteralFields(f, nil); len(got) != 0 {
		t.Errorf("expected empty output with nil tier3, got %v", got)
	}
	if got := collectCompositeLiteralFields(f, map[string]bool{}); len(got) != 0 {
		t.Errorf("expected empty output with empty tier3, got %v", got)
	}
}

// TestExprToPath verifies the dotted-path conversion the composite-literal
// walker uses to key its bindings. Identifier roots are preserved, deeper
// chains are joined with dots, and anything not identifier-rooted returns "".
func TestExprToPath(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{"x", "x"},
		{"x.y", "x.y"},
		{"x.y.z", "x.y.z"},
		{"f()", ""},    // call — not identifier-rooted
		{"a[0]", ""},   // index — not identifier-rooted
		{"a[0].b", ""}, // index inside a chain
	}
	for _, c := range cases {
		expr, err := parser.ParseExpr(c.src)
		if err != nil {
			t.Fatalf("parse %q: %v", c.src, err)
		}
		if got := exprToPath(expr); got != c.want {
			t.Errorf("exprToPath(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

// TestPathPrefixAndField verifies the decomposition the walker uses to
// attribute selector writes (`q.Schedule.WeekInstance`) to a previously
// recorded binding (`q.Schedule`).
func TestPathPrefixAndField(t *testing.T) {
	cases := []struct {
		src        string
		wantPrefix string
		wantField  string
	}{
		{"x.Y", "x", "Y"},
		{"x.Y.Z", "x.Y", "Z"},
		{"a.b.c.d", "a.b.c", "d"},
		{"x", "", ""},      // bare ident — no selector
		{"f().Y", "", ""},  // call-rooted — no path
		{"a[0].Y", "", ""}, // index-rooted — no path
	}
	for _, c := range cases {
		expr, err := parser.ParseExpr(c.src)
		if err != nil {
			t.Fatalf("parse %q: %v", c.src, err)
		}
		prefix, field := pathPrefixAndField(expr)
		if prefix != c.wantPrefix || field != c.wantField {
			t.Errorf("pathPrefixAndField(%q) = (%q, %q), want (%q, %q)",
				c.src, prefix, field, c.wantPrefix, c.wantField)
		}
	}
}
