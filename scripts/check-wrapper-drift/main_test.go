package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

// ---------------------------------------------------------------------------
// Embedded (anonymous) field promotion — issue #599.
//
// Before the promotion walk existed, an anonymous field was dropped on the
// floor (no tag, no name), so a wrapper that embedded a struct read as having
// none of its promoted fields and every one of them was reported missing. The
// cases below come in pairs: an embedding wrapper that must PASS, and a
// genuinely-drifted sibling built on the same shape that must still FAIL — a
// fix that simply stopped checking embedding wrappers would pass the first and
// fail the second.
// ---------------------------------------------------------------------------

// src rewrites ~ to a backtick so struct-tag fixtures can be written as raw
// string literals (Go raw strings cannot contain a backtick, and these
// embedding fixtures carry too many tags for the escaped-concatenation style
// used by the older fixtures above).
func src(s string) string { return strings.ReplaceAll(s, "~", "`") }

// flattenFixture parses one source file, collects its structs and resolves
// their embedded types, returning the flattened universe.
func flattenFixture(t *testing.T, source string) map[string]*structFields {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fixture.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	structs := collectStructsAndMarkers(fset, f)
	flattenEmbedded(structs, collectTypeDecls(f))
	return structs
}

// embeddingGenSrc is the generated side shared by the embedding pair below: 14
// tags, 12 of which the wrapper can only satisfy through promotion.
const embeddingGenSrc = `package generated

type Task struct {
	Id               int64  ~json:"id"~
	Status           string ~json:"status"~
	Title            string ~json:"title"~
	Content          string ~json:"content"~
	Type             string ~json:"type"~
	Position         int64  ~json:"position"~
	VisibleToClients bool   ~json:"visible_to_clients"~
	CreatorName      string ~json:"creator_name"~
	CreatorId        int64  ~json:"creator_id"~
	CreatedAt        string ~json:"created_at"~
	UpdatedAt        string ~json:"updated_at"~
	Url              string ~json:"url"~
	AppUrl           string ~json:"app_url"~
	BookmarkUrl      string ~json:"bookmark_url"~
}
`

// embeddingWrapperSrc declares two tags directly and inherits the other twelve
// through a two-level embedding chain (Task embeds Meta by value, Meta embeds
// *Audit by pointer). Meta and Audit live in a SECOND wrapper file, so the
// resolution has to happen after every file is parsed, not per file.
const embeddingWrapperSrc = `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Task struct {
	Meta
	Status string ~json:"status"~
	Title  string ~json:"title"~
}

func taskFromGenerated(g generated.Task) Task {
	t := Task{Status: g.Status}
	t.Title = g.Title
	t.ID = g.Id
	t.Content = g.Content
	t.Type = g.Type
	t.Position = g.Position
	t.VisibleToClients = g.VisibleToClients
	t.CreatorName = g.CreatorName
	t.CreatorID = g.CreatorId
	t.CreatedAt = g.CreatedAt
	t.UpdatedAt = g.UpdatedAt
	t.URL = g.Url
	t.AppURL = g.AppUrl
	t.BookmarkURL = g.BookmarkUrl
	return t
}
`

const embeddingBaseSrc = `package basecamp

type Meta struct {
	*Audit
	ID               int64  ~json:"id"~
	Content          string ~json:"content"~
	Type             string ~json:"type"~
	Position         int64  ~json:"position"~
	VisibleToClients bool   ~json:"visible_to_clients"~
	CreatorName      string ~json:"creator_name"~
	CreatorID        int64  ~json:"creator_id"~
}

type Audit struct {
	CreatedAt   string ~json:"created_at"~
	UpdatedAt   string ~json:"updated_at"~
	URL         string ~json:"url"~
	AppURL      string ~json:"app_url"~
	BookmarkURL string ~json:"bookmark_url"~
}
`

// TestRun_EmbeddedWrapperInSync is the #599 regression. Every generated tag is
// present on the wrapper — two declared directly, twelve promoted through the
// embedding chain — and every one is assigned. Before the promotion walk, this
// reported all twelve promoted tags as missing.
func TestRun_EmbeddedWrapperInSync(t *testing.T) {
	wrapperDir, generatedFile := writeDriftFixtures(t, src(embeddingGenSrc), map[string]string{
		"task.go": src(embeddingWrapperSrc),
		"meta.go": src(embeddingBaseSrc),
	})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: expected no drift for an embedding wrapper, got %v", err)
	}
}

// TestRun_EmbeddedWrapperGenuinelyMissingTag is the paired negative: the same
// embedding shape, plus one generated tag that neither the wrapper nor
// anything it embeds declares. Resolving embedded fields must not blunt the
// check — this must still be reported.
func TestRun_EmbeddedWrapperGenuinelyMissingTag(t *testing.T) {
	genSrc := strings.Replace(embeddingGenSrc,
		"\tBookmarkUrl      string ~json:\"bookmark_url\"~\n",
		"\tBookmarkUrl      string ~json:\"bookmark_url\"~\n\tSubscriptionUrl  string ~json:\"subscription_url\"~\n", 1)
	if !strings.Contains(genSrc, "subscription_url") {
		t.Fatal("fixture setup: subscription_url was not added to the generated struct")
	}
	wrapperDir, generatedFile := writeDriftFixtures(t, src(genSrc), map[string]string{
		"task.go": src(embeddingWrapperSrc),
		"meta.go": src(embeddingBaseSrc),
	})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected drift on subscription_url, which no embedded struct declares, got nil")
	}
}

// TestRun_EmbeddedPromotedFieldUnassigned proves the population check reaches
// through promotion too: the tag is declared (on the embedded struct) but the
// *FromGenerated body never assigns it, in any spelling.
func TestRun_EmbeddedPromotedFieldUnassigned(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id      int64  ~json:"id"~
	Content string ~json:"content"~
}
`)
	wrapperSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Base struct {
	ID      int64  ~json:"id"~
	Content string ~json:"content"~
}

type Note struct {
	Base
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = g.Id
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected population drift on the promoted-but-unassigned Content field, got nil")
	}
}

// TestRun_EmbeddedStructAssignedWholesale covers the other legal spelling:
// assigning the embedded struct itself populates every field promoted through
// it, including fields promoted from a deeper embed.
func TestRun_EmbeddedStructAssignedWholesale(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id        int64  ~json:"id"~
	Content   string ~json:"content"~
	CreatedAt string ~json:"created_at"~
}
`)
	wrapperSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Stamp struct {
	CreatedAt string ~json:"created_at"~
}

type Base struct {
	Stamp
	ID      int64  ~json:"id"~
	Content string ~json:"content"~
}

type Note struct {
	Base
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.Base = Base{ID: g.Id, Content: g.Content, Stamp: Stamp{CreatedAt: g.CreatedAt}}
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: assigning the embedded struct wholesale must populate its promoted fields, got %v", err)
	}
}

// TestRun_GeneratedStructEmbeds covers the other side of the pair: the
// GENERATED struct embeds a struct, so its promoted tags are part of the
// contract the wrapper must satisfy. Dropping them would make the check
// silently under-report.
func TestRun_GeneratedStructEmbeds(t *testing.T) {
	genSrc := src(`package generated

type Timestamps struct {
	CreatedAt string ~json:"created_at"~
	UpdatedAt string ~json:"updated_at"~
}

type Note struct {
	Timestamps
	Id int64 ~json:"id"~
}
`)
	wrapperMissing := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Note struct {
	ID        int64  ~json:"id"~
	CreatedAt string ~json:"created_at"~
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = g.Id
	n.CreatedAt = g.CreatedAt
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperMissing})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected drift on updated_at, promoted onto the generated struct, got nil")
	}

	wrapperComplete := strings.Replace(wrapperMissing,
		"\tn.CreatedAt = g.CreatedAt\n",
		"\tn.CreatedAt = g.CreatedAt\n\tn.UpdatedAt = g.UpdatedAt\n", 1)
	wrapperComplete = strings.Replace(wrapperComplete,
		src("\tCreatedAt string ~json:\"created_at\"~\n"),
		src("\tCreatedAt string ~json:\"created_at\"~\n\tUpdatedAt string ~json:\"updated_at\"~\n"), 1)
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperComplete})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: wrapper covering both promoted generated tags must pass, got %v", err)
	}
}

// TestRun_UnresolvableEmbedIsReportedNotSkipped pins the deliberate choice for
// an embed the checker cannot resolve: report it for any pair that reaches it,
// because the promoted fields are invisible and pretending they are absent (or
// that there are none) is the silent-drop failure mode of #599. Structs
// OUTSIDE every pair keep their unresolvable embeds for free — go/pkg/basecamp
// has one today (FlexTime embeds time.Time).
func TestRun_UnresolvableEmbedIsReportedNotSkipped(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id int64 ~json:"id"~
}
`)
	wrapperSrc := src(`package basecamp

import (
	"time"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
)

// FlexTime is outside every pair: its unresolvable embed must NOT fail the run.
type FlexTime struct {
	time.Time
}

type Note struct {
	ID int64 ~json:"id"~
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = g.Id
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: an unresolvable embed on a struct outside every pair must not fail, got %v", err)
	}

	// Now put the unresolvable embed ON the paired wrapper.
	paired := strings.Replace(wrapperSrc, src("type Note struct {\n\tID int64 ~json:\"id\"~\n}"),
		src("type Note struct {\n\ttime.Time\n\tID int64 ~json:\"id\"~\n}"), 1)
	if !strings.Contains(paired, "\ttime.Time\n\tID") {
		t.Fatal("fixture setup: the embed was not added to the paired wrapper")
	}
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": paired})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: an unresolvable embed on a paired wrapper must be reported, got nil")
	}
}

// TestFlattenEmbedded_Shadowing pins encoding/json's promotion rules: a tag
// declared directly on the struct wins over the same tag promoted from an
// embed, and a depth-1 promotion wins over depth 2.
func TestFlattenEmbedded_Shadowing(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Deep struct {
	Name string ~json:"name"~
	Deep string ~json:"deep_only"~
}

type Mid struct {
	Deep
	Name string ~json:"name"~
	Kind string ~json:"kind"~
}

type Outer struct {
	Mid
	Name string ~json:"name"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	for _, tag := range []string{"name", "kind", "deep_only"} {
		if !outer.tags[tag] {
			t.Errorf("expected Outer to carry tag %q, got %v", tag, outer.tags)
		}
	}
	if got := outer.tagToGoField["name"]; got != "Name" {
		t.Errorf("name should resolve to Outer's own Name field, got %q", got)
	}
	// The own field wins, so its population target is itself alone — not the
	// embedded path that would also have carried a "name".
	if got := outer.populationTargets("name"); len(got) != 1 || got[0] != "Name" {
		t.Errorf("expected population target [Name] for the shadowing own field, got %v", got)
	}
	if got := outer.populationTargets("kind"); len(got) != 2 || got[0] != "Mid" || got[1] != "Kind" {
		t.Errorf("expected population targets [Mid Kind] for the depth-1 promotion, got %v", got)
	}
	if got := outer.populationTargets("deep_only"); len(got) != 3 || got[0] != "Mid" || got[1] != "Deep" || got[2] != "Deep" {
		t.Errorf("expected population targets [Mid Deep Deep] for the depth-2 promotion, got %v", got)
	}
}

// TestFlattenEmbedded_SameDepthConflictCancels pins the other half of
// encoding/json's rule: two embeds contributing the same tag at the same depth
// cancel out, so the tag is NOT on the wire and must not be reported as
// present. Non-conflicting tags from the same two embeds still promote.
func TestFlattenEmbedded_SameDepthConflictCancels(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Left struct {
	Name string ~json:"name"~
	Only string ~json:"left_only"~
}

type Right struct {
	Name string ~json:"name"~
	Only string ~json:"right_only"~
}

type Outer struct {
	Left
	Right
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["name"] {
		t.Error("a tag contributed twice at the same depth is dropped by encoding/json; it must not count as present")
	}
	if !outer.tags["left_only"] || !outer.tags["right_only"] {
		t.Errorf("non-conflicting tags from both embeds must still promote, got %v", outer.tags)
	}
}

// TestFlattenEmbedded_CyclesTerminate covers mutual and self embedding. The
// assertion that matters is that this returns at all; the tag expectations
// pin the fields it still finds on the way round.
func TestFlattenEmbedded_CyclesTerminate(t *testing.T) {
	source := src(`package fixture

type A struct {
	*B
	AField string ~json:"a_field"~
}

type B struct {
	*A
	BField string ~json:"b_field"~
}

type Selfish struct {
	*Selfish
	SelfField string ~json:"self_field"~
}
`)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fixture.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	structs := collectStructsAndMarkers(fset, f)
	decls := collectTypeDecls(f)
	// flattenEmbedded runs on its own goroutine so a walk that fails to
	// terminate fails the test instead of hanging the whole package.
	done := make(chan struct{})
	go func() {
		flattenEmbedded(structs, decls)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("flattenEmbedded did not terminate on an embedding cycle")
	}
	if a := structs["A"]; a == nil || !a.tags["a_field"] || !a.tags["b_field"] {
		t.Errorf("expected A to carry a_field and b_field, got %v", structs["A"])
	}
	if s := structs["Selfish"]; s == nil || !s.tags["self_field"] || len(s.tags) != 1 {
		t.Errorf("expected Selfish to carry exactly self_field, got %v", structs["Selfish"])
	}
}

// TestFlattenEmbedded_NonStructAndAliasEmbeds covers the two resolvable
// non-struct shapes: an embedded interface or map promotes no JSON fields
// (generated.ClientWithResponses embeds ClientInterface today), while a defined
// type standing for a struct is followed to it.
func TestFlattenEmbedded_NonStructAndAliasEmbeds(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Doer interface{ Do() }

type Payload map[string]string

type Base struct {
	ID int64 ~json:"id"~
}

type Alias Base

type Outer struct {
	Doer
	Payload
	Alias
	Name string ~json:"name"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if len(outer.unresolved) != 0 {
		t.Errorf("interface/map/alias embeds all resolve; got unresolved %v", outer.unresolved)
	}
	if !outer.tags["id"] {
		t.Errorf("expected the alias embed to promote id, got %v", outer.tags)
	}
	if !outer.tags["name"] || len(outer.tags) != 2 {
		t.Errorf("expected exactly name+id, got %v", outer.tags)
	}
}

// TestCollectStructs_TaggedAnonymousFieldNotPromoted pins the encoding/json
// carve-out: an anonymous field that carries its own json tag is an ordinary
// field under that tag, not a promotion source.
func TestCollectStructs_TaggedAnonymousFieldNotPromoted(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Base struct {
	ID int64 ~json:"id"~
}

type Outer struct {
	Base ~json:"base"~
	Name string ~json:"name"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["id"] {
		t.Error("a tagged anonymous field must not promote its fields")
	}
	if !outer.tags["base"] {
		t.Errorf("expected the tagged anonymous field to register under its own tag, got %v", outer.tags)
	}
	if got := outer.tagToGoField["base"]; got != "Base" {
		t.Errorf("expected the embedded type name as the Go field name, got %q", got)
	}
}

// TestRun_OmitMarkerNotInheritedThroughEmbed pins the deliberate non-inheritance
// of intentionally-omitted markers. A marker inside an embedded struct belongs
// to that struct's own pair; the embedding wrapper must declare its own, and
// until it does the tag is reported. The failure mode of this choice is a loud
// missing-tag report, never a silent pass.
func TestRun_OmitMarkerNotInheritedThroughEmbed(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id     int64  ~json:"id"~
	Secret string ~json:"secret"~
}
`)
	wrapperSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Base struct {
	// intentionally-omitted: secret - Base's own decision, not Note's
	ID int64 ~json:"id"~
}

type Note struct {
	Base
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = g.Id
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: an embedded struct's intentionally-omitted marker must not suppress the embedding wrapper's check")
	}

	// The embedding wrapper carrying its own marker resolves it.
	withOwn := strings.Replace(wrapperSrc, "type Note struct {\n\tBase\n}",
		"type Note struct {\n\t// intentionally-omitted: secret - Note's own decision\n\tBase\n}", 1)
	if !strings.Contains(withOwn, "Note's own decision") {
		t.Fatal("fixture setup: the marker was not added to the embedding wrapper")
	}
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": withOwn})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: the embedding wrapper's own marker must suppress the tag, got %v", err)
	}
}

// TestFlattenEmbedded_DepthCapIsReportedNotSilent pins the behaviour at the
// depth budget: a chain longer than maxEmbedDepth is truncated (the walk must
// terminate), but the truncation is REPORTED on the root, because a quietly
// truncated chain hides fields exactly the way #599 did.
func TestFlattenEmbedded_DepthCapIsReportedNotSilent(t *testing.T) {
	deepest := maxEmbedDepth + 2
	var b strings.Builder
	b.WriteString("package fixture\n")
	for i := 0; i <= deepest; i++ {
		b.WriteString("\ntype T" + strconv.Itoa(i) + " struct {\n")
		if i < deepest {
			b.WriteString("\tT" + strconv.Itoa(i+1) + "\n")
		}
		b.WriteString("\tF" + strconv.Itoa(i) + " string ~json:\"f" + strconv.Itoa(i) + "\"~\n}\n")
	}
	structs := flattenFixture(t, src(b.String()))
	root := structs["T0"]
	if root == nil {
		t.Fatal("T0 not collected")
	}
	if len(root.unresolved) == 0 {
		t.Error("a chain truncated at the depth cap must be reported, not dropped silently")
	}
	if !root.tags["f"+strconv.Itoa(maxEmbedDepth)] {
		t.Errorf("expected everything within the cap to promote, got %v", root.tags)
	}
	if root.tags["f"+strconv.Itoa(deepest)] {
		t.Error("fixture is not actually exceeding the cap; the test would prove nothing")
	}
}
