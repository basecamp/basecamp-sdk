package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
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

// captureStderr runs fn with os.Stderr redirected and returns what it wrote.
// run() reports drift to stderr, so this is how a test asserts on the
// DIAGNOSIS rather than only on the exit path.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var buf strings.Builder
		io.Copy(&buf, r)
		done <- buf.String()
	}()
	fn()
	os.Stderr = saved
	w.Close()
	out := <-done
	r.Close()
	return out
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

// TestSelectorRootAndPath verifies the decomposition the population walker uses
// to attribute a write to the wrapper instance and the dotted field path below
// it. Retaining the whole path is what makes a fully-qualified write to a
// promoted field (`n.Base.ID`) recognizable.
func TestSelectorRootAndPath(t *testing.T) {
	cases := []struct {
		src      string
		wantRoot string
		wantPath string
	}{
		{"x.Y", "x", "Y"},
		{"x.Y.Z", "x", "Y.Z"},
		{"a.b.c.d", "a", "b.c.d"},
		{"x", "", ""},      // bare ident — nothing selected
		{"f().Y", "", ""},  // call-rooted — no path
		{"a[0].Y", "", ""}, // index-rooted — no path
	}
	for _, c := range cases {
		expr, err := parser.ParseExpr(c.src)
		if err != nil {
			t.Fatalf("parse %q: %v", c.src, err)
		}
		root, path := selectorRootAndPath(expr)
		if root != c.wantRoot || path != c.wantPath {
			t.Errorf("selectorRootAndPath(%q) = (%q, %q), want (%q, %q)",
				c.src, root, path, c.wantRoot, c.wantPath)
		}
	}
}

// TestBoundWrapperAndPath verifies that a write is attributed to the innermost
// binding that encloses it, and that an unbound path is ignored.
func TestBoundWrapperAndPath(t *testing.T) {
	bindings := map[string]string{"q": "Outer", "q.Schedule": "QuestionSchedule"}
	cases := []struct {
		src         string
		wantWrapper string
		wantPath    string
	}{
		{"q.Schedule.WeekInstance", "QuestionSchedule", "WeekInstance"},
		{"q.Schedule.Base.ID", "QuestionSchedule", "Base.ID"},
		{"q.Title", "Outer", "Title"},
		{"other.Title", "", ""},
		{"q", "", ""},
	}
	for _, c := range cases {
		expr, err := parser.ParseExpr(c.src)
		if err != nil {
			t.Fatalf("parse %q: %v", c.src, err)
		}
		wrapper, path := boundWrapperAndPath(bindings, expr)
		if wrapper != c.wantWrapper || path != c.wantPath {
			t.Errorf("boundWrapperAndPath(%q) = (%q, %q), want (%q, %q)",
				c.src, wrapper, path, c.wantWrapper, c.wantPath)
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
	jsonMethods, ifaceEmbeds := collectJSONMethodTypes(f)
	closeJSONMethodTypes(jsonMethods, ifaceEmbeds)
	flattenEmbedded(structs, collectTypeDecls(f), jsonMethods)
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
//
// The pointer embed is initialized before the writes that go through it: a
// promoted write across a nil pointer panics, so a fixture that omitted the
// initialization would be pinning a construction that cannot run.
const embeddingWrapperSrc = `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Task struct {
	Meta
	Status string ~json:"status"~
	Title  string ~json:"title"~
}

func taskFromGenerated(g generated.Task) Task {
	t := Task{Status: g.Status}
	t.Audit = &Audit{}
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
	// A depth-1 promotion is populated by the promoted field, by its qualified
	// spelling, or by an opaque assignment of the embed — and by nothing else.
	assertTargets(t, outer, "kind", []string{"Kind", "Mid.Kind", "Mid.*"})
	// A depth-2 promotion adds the intermediate spellings, since `Deep` is
	// itself promoted onto Outer. The promoted field here is ALSO named Deep,
	// and that collision is instructive: `outer.Deep` resolves to the embedded
	// Deep struct at depth 1, not to the string field at depth 2, so the bare
	// spelling is absent from the target set — crediting it would attribute a
	// write that lands somewhere else entirely.
	assertTargets(t, outer, "deep_only", []string{
		"Deep.Deep", "Mid.Deep.Deep", "Mid.*", "Deep.*", "Mid.Deep.*",
	})
}

// assertTargets compares populationTargets against an expected set, order
// independent. Both directions matter: a missing spelling reports false drift
// on a valid construction, and an extra one credits a construction that does
// not actually populate the field.
func assertTargets(t *testing.T, sf *structFields, tag string, want []string) {
	t.Helper()
	got := sf.populationTargets(tag)
	gotSet := map[string]bool{}
	for _, g := range got {
		gotSet[g] = true
	}
	wantSet := map[string]bool{}
	for _, w := range want {
		wantSet[w] = true
		if !gotSet[w] {
			t.Errorf("populationTargets(%q): missing spelling %q, got %v", tag, w, got)
		}
	}
	for _, g := range got {
		if !wantSet[g] {
			t.Errorf("populationTargets(%q): unexpected spelling %q, got %v", tag, g, got)
		}
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
	jsonMethods, ifaceEmbeds := collectJSONMethodTypes(f)
	closeJSONMethodTypes(jsonMethods, ifaceEmbeds)
	// flattenEmbedded runs on its own goroutine so a walk that fails to
	// terminate fails the test instead of hanging the whole package.
	done := make(chan struct{})
	go func() {
		flattenEmbedded(structs, decls, jsonMethods)
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
	// The interface and map embeds are not absent from the wire —
	// encoding/json emits them under their Go field names ("Doer",
	// "Payload"). They carry no json TAG, which is this check's whole
	// vocabulary, so they are invisible here in exactly the way an untagged
	// NAMED field is. Asserting the parallel pins that the treatment is
	// uniform rather than an embed-specific hole.
	if outer.tags["Doer"] || outer.tags["Payload"] {
		t.Errorf("untagged fields are outside this check's tag vocabulary, got %v", outer.tags)
	}
	plain := flattenFixture(t, src(`package fixture

type Outer struct {
	Plain string
	Name  string ~json:"name"~
}
`))["Outer"]
	if plain.tags["Plain"] || len(plain.tags) != 1 {
		t.Errorf("an untagged NAMED field is equally invisible; got %v", plain.tags)
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

// ---------------------------------------------------------------------------
// Review follow-ups (PR #721): cases where the first cut of the promotion walk
// claimed a tag encoding/json would not put on the wire, or credited a
// construction that does not populate what it appears to.
// ---------------------------------------------------------------------------

// TestFlattenEmbedded_DiamondAnnihilates is the diamond case: Outer embeds Left
// and Right, both of which embed Common. encoding/json reaches Common twice at
// the same depth and annihilates its tags (typeFields duplicates the field so
// dominantField sees the conflict), so the tag is NOT on the wire and must not
// count as present. Deduplicating the second path without counting it — the
// walk's first cut — silently claimed the tag instead.
func TestFlattenEmbedded_DiamondAnnihilates(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Common struct {
	Shared string ~json:"shared"~
}

type Left struct {
	Common
	Only string ~json:"left_only"~
}

type Right struct {
	Common
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
	if outer.tags["shared"] {
		t.Error("a tag reached twice at the same depth is annihilated by encoding/json; it must not count as present")
	}
	if !outer.tags["left_only"] || !outer.tags["right_only"] {
		t.Errorf("the unambiguous depth-1 tags must still promote, got %v", outer.tags)
	}
}

// TestFlattenEmbedded_DiamondResolvedByShadowing is the diamond's counterpart:
// the same shape, but Outer declares the contested tag itself. The depth-0
// declaration is unambiguous, so the tag IS on the wire and must count. Without
// this pair, the annihilation rule could be "fixed" by dropping the tag
// wholesale.
func TestFlattenEmbedded_DiamondResolvedByShadowing(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Common struct {
	Shared string ~json:"shared"~
}

type Left struct {
	Common
}

type Right struct {
	Common
}

type Outer struct {
	Left
	Right
	Shared string ~json:"shared"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if !outer.tags["shared"] {
		t.Error("the struct's own declaration is unambiguous and wins; the tag is on the wire")
	}
	assertTargets(t, outer, "shared", []string{"Shared"})
}

// TestFlattenEmbedded_UnexportedPromotedFieldIgnored covers the export rule:
// encoding/json ignores unexported non-anonymous fields whatever their tag
// says, so promoting one would let a wrapper satisfy a generated tag with a
// field that never reaches the wire. An embedded UNEXPORTED STRUCT TYPE is the
// opposite case — its exported fields do promote — and both are asserted here
// so a fix for one cannot silently break the other.
func TestFlattenEmbedded_UnexportedPromotedFieldIgnored(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Base struct {
	Visible string ~json:"visible"~
	hidden  string ~json:"hidden"~
}

type unexportedBase struct {
	Promoted string ~json:"promoted"~
}

type Outer struct {
	Base
	unexportedBase
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["hidden"] {
		t.Error("an unexported field is not on the wire and must not satisfy a generated tag")
	}
	if !outer.tags["visible"] {
		t.Errorf("expected the exported promoted field, got %v", outer.tags)
	}
	if !outer.tags["promoted"] {
		t.Errorf("an embedded unexported STRUCT TYPE still promotes its exported fields, got %v", outer.tags)
	}
}

// TestRun_UnexportedFieldDoesNotSatisfyGeneratedTag is the end-to-end half of
// the rule: the wrapper "declares" the tag, but on an unexported field, so the
// generated tag is unsatisfied and must be reported.
func TestRun_UnexportedFieldDoesNotSatisfyGeneratedTag(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id     int64  ~json:"id"~
	Secret string ~json:"secret"~
}
`)
	wrapperSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Note struct {
	ID     int64  ~json:"id"~
	secret string ~json:"secret"~
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = g.Id
	n.secret = g.Secret
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: an unexported field carrying the tag must not satisfy it, got nil")
	}
}

// TestRun_PartialEmbeddedLiteralIsNotFullCoverage is the population half of the
// same principle. `n.Base = Base{ID: g.Id}` leaves Content zero-valued on the
// wire; crediting every field under an assigned embed would let a newly added
// generated field pass silently, which is the failure this whole check exists
// to prevent. The paired positive — the same literal, completed — must pass,
// so the rule cannot be satisfied by refusing wholesale assignment outright.
func TestRun_PartialEmbeddedLiteralIsNotFullCoverage(t *testing.T) {
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
	n.Base = Base{ID: g.Id}
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: a partial literal on an embedded field must not credit the fields it omits, got nil")
	}

	completed := strings.Replace(wrapperSrc, "Base{ID: g.Id}", "Base{ID: g.Id, Content: g.Content}", 1)
	if !strings.Contains(completed, "Content: g.Content") {
		t.Fatal("fixture setup: the literal was not completed")
	}
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": completed})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: a complete literal on an embedded field populates everything it names, got %v", err)
	}
}

// TestRun_OpaqueEmbeddedAssignmentIsFullCoverage pins the other side of the
// literal rule: when the walker cannot see inside the assigned value it credits
// the whole subtree, matching the one-level-nesting doctrine the check already
// applies to nested wrappers. Narrowing that would report drift on every
// wrapper that delegates to a helper.
func TestRun_OpaqueEmbeddedAssignmentIsFullCoverage(t *testing.T) {
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

func baseFrom(g generated.Note) Base {
	return Base{ID: g.Id, Content: g.Content}
}

type Note struct {
	Base
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.Base = baseFrom(g)
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: an opaque assignment to an embedded field credits its subtree, got %v", err)
	}
}

// TestRun_QualifiedPromotedAssignment covers the fully-qualified write
// `n.Base.ID = g.Id`, which is valid Go and does populate the promoted field.
// The one-level selector walk could not see it and reported false drift — the
// over-reporting failure #599 is about, hit by the first person to embed.
func TestRun_QualifiedPromotedAssignment(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id      int64  ~json:"id"~
	Content string ~json:"content"~
}
`)
	wrapperSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Inner struct {
	Content string ~json:"content"~
}

type Base struct {
	Inner
	ID int64 ~json:"id"~
}

type Note struct {
	Base
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.Base.ID = g.Id
	n.Base.Inner.Content = g.Content
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: a fully-qualified write to a promoted field must count as population, got %v", err)
	}

	// Paired negative: drop one of the two qualified writes and the check must
	// bite again, so recognizing the spelling did not blunt it.
	dropped := strings.Replace(wrapperSrc, "\tn.Base.Inner.Content = g.Content\n", "", 1)
	if strings.Contains(dropped, "Inner.Content = g.Content") {
		t.Fatal("fixture setup: the write was not dropped")
	}
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": dropped})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: expected population drift once the qualified write is removed, got nil")
	}
}

// TestCollectAssignedFields_QualifiedAndLiteralCoverage pins the assignment
// vocabulary directly: full selector paths are retained, struct literals are
// enumerated key by key, and only opaque values get the `.*` total marker.
func TestCollectAssignedFields_QualifiedAndLiteralCoverage(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "fixture.go", src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

func wrapFromGenerated(g generated.Wrap) Wrap {
	w := Wrap{Base: Base{ID: g.Id}}
	w.Meta.Note = g.Note
	w.Other = helper(g)
	w.Items = []string{g.Item}
	return w
}
`), parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assigned := collectAssignedFields(f)["Wrap"]
	for _, want := range []string{"Base", "Base.ID", "Meta.Note", "Other", "Other.*", "Items", "Items.*"} {
		if !assigned[want] {
			t.Errorf("expected %q to be recorded, got %v", want, assigned)
		}
	}
	// The literal is visible, so it must NOT be credited as total coverage.
	if assigned["Base.*"] {
		t.Error("a visible struct literal must be enumerated, not credited wholesale")
	}
	// A slice literal says nothing about a wrapper's fields, so it stays opaque
	// rather than being enumerated as if its elements were field keys.
	if assigned["Items.0"] {
		t.Error("a slice literal must not be enumerated as struct keys")
	}
}

// TestRun_OuterGoNameShadowsPromotedField is the second-round shadowing case,
// and it is subtle enough to be worth spelling out. The wrapper embeds Base
// (`ID` tagged `id`) and declares its OWN `ID` tagged `other_id`. Both keys are
// on the wire — the Go names collide but the JSON names do not — while
// `n.ID = …` writes the outer field only:
//
//	json.Marshal(b) where b.ID = 7  =>  {"id":0,"other_id":7}
//
// So crediting the bare `ID` spelling to the promoted `id` would pass a wrapper
// that ships `"id": 0` on every response. The qualified spelling still counts,
// which is the paired positive.
func TestRun_OuterGoNameShadowsPromotedField(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id      int64 ~json:"id"~
	OtherId int64 ~json:"other_id"~
}
`)
	wrapperSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Base struct {
	ID int64 ~json:"id"~
}

type Note struct {
	Base
	ID int64 ~json:"other_id"~
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = g.OtherId
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: `n.ID` writes the outer field, so the promoted `id` is unpopulated and must be reported")
	}

	qualified := strings.Replace(wrapperSrc, "\tn.ID = g.OtherId\n", "\tn.ID = g.OtherId\n\tn.Base.ID = g.Id\n", 1)
	if !strings.Contains(qualified, "n.Base.ID") {
		t.Fatal("fixture setup: the qualified write was not added")
	}
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": qualified})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: the qualified write reaches the promoted field and must count, got %v", err)
	}
}

// TestFlattenEmbedded_DuplicateOwnTagAnnihilates covers the depth-0 case of the
// conflict rule that already governed promotions: two fields on ONE struct
// sharing a json tag are both dropped by encoding/json —
// `json.Marshal(struct{A,B string "same"; C string "c"}{...})` emits only "c" —
// so the tag is not on the wire and cannot satisfy a generated field. The
// conflict also blocks a deeper promotion of the same tag, matching
// dominantField, which gives up as soon as its two shallowest entries tie.
func TestFlattenEmbedded_DuplicateOwnTagAnnihilates(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Base struct {
	FromEmbed string ~json:"same"~
}

type Outer struct {
	Base
	A string ~json:"same"~
	B string ~json:"same"~
	C string ~json:"c"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["same"] {
		t.Error("one tag on two fields of the same struct is annihilated; it must not count as present")
	}
	if !outer.tags["c"] {
		t.Errorf("the unambiguous tag must survive, got %v", outer.tags)
	}
}

// TestFlattenEmbedded_TaggedUnexportedNonStructEmbedIgnored covers the last of
// encoding/json's embed carve-outs: an anonymous field of an unexported
// NON-STRUCT type is dropped even when tagged (`struct{ hidden "json:secret" }`
// marshals without "secret"), while an unexported STRUCT type keeps its tag,
// because typeFields only skips the non-struct case.
func TestFlattenEmbedded_TaggedUnexportedNonStructEmbedIgnored(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type hidden string

type hiddenStruct struct {
	Inner string ~json:"inner"~
}

type Outer struct {
	hidden       ~json:"secret"~
	hiddenStruct ~json:"nested"~
	Name         string ~json:"name"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["secret"] {
		t.Error("an anonymous unexported non-struct field is dropped by encoding/json even when tagged")
	}
	if !outer.tags["nested"] {
		t.Errorf("an anonymous unexported STRUCT type keeps its tag, got %v", outer.tags)
	}
	if outer.tags["inner"] {
		t.Error("a tagged anonymous field is not a promotion source")
	}
	if !outer.tags["name"] {
		t.Errorf("expected the plain field to survive, got %v", outer.tags)
	}
}

// TestFlattenEmbedded_GroupedDeclarationAnnihilates covers the grouped
// declaration `A, B string ~json:"x"~`, which is two fields to encoding/json
// sharing one tag at one depth — so they annihilate and the tag is not on the
// wire, whether the struct is the wrapper itself or something it embeds.
func TestFlattenEmbedded_GroupedDeclarationAnnihilates(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Base struct {
	A, B string ~json:"x"~
	C    string ~json:"c"~
}

type Outer struct {
	Base
}

type Direct struct {
	A, B string ~json:"x"~
	C    string ~json:"c"~
}
`))
	for _, name := range []string{"Outer", "Direct"} {
		sf := structs[name]
		if sf == nil {
			t.Fatalf("%s not collected", name)
		}
		if sf.tags["x"] {
			t.Errorf("%s: a grouped declaration is two fields sharing a tag; they annihilate", name)
		}
		if !sf.tags["c"] {
			t.Errorf("%s: the ungrouped tag must survive, got %v", name, sf.tags)
		}
	}
}

// TestRecordAssignedValue_ValueShapes pins how each right-hand side shape is
// classified, since the difference between "enumerated" and "total coverage"
// is what stops a partial assignment from certifying fields it never sets.
func TestRecordAssignedValue_ValueShapes(t *testing.T) {
	cases := []struct {
		name       string
		expr       string
		wantKeys   []string
		wantTotal  bool
		wantAbsent []string
	}{
		{"keyed literal", "Base{ID: g.Id}", []string{"Base.ID"}, false, []string{"Base.*"}},
		{"parenthesized keyed literal", "(Base{ID: g.Id})", []string{"Base.ID"}, false, []string{"Base.*"}},
		{"pointer keyed literal", "&Base{ID: g.Id}", []string{"Base.ID"}, false, []string{"Base.*"}},
		// A positional literal must list every field, so it covers all of them.
		{"positional literal", "Base{g.Id, g.Name}", nil, true, nil},
		// A positional element CAN be a nil embedded pointer, whose promoted
		// fields are then absent from the wire. The walker cannot tell which
		// element is which field, so a nil element withdraws the claim.
		{"positional literal with a nil element", "Base{nil, g.Name}", nil, false, []string{"Base.*"}},
		// Any statically zero element has the same effect: whatever it lands
		// on contributes nothing, so the literal cannot claim the subtree.
		{"positional literal with an empty nested literal", "Base{Audit{}, g.Name}", nil, false, []string{"Base.*"}},
		{"positional literal with new(T)", "Base{new(Audit), g.Name}", nil, false, []string{"Base.*"}},
		{"positional literal with a typed nil", "Base{(*Audit)(nil), g.Name}", nil, false, []string{"Base.*"}},
		// A populated nested literal is content the walker can SEE but cannot
		// attribute to a field without the declaration order — it fills some
		// of Audit and leaves the rest zero — so the subtree claim is dropped
		// rather than assumed. A positional literal of opaque elements still
		// claims it, which is the row above.
		{"positional literal with a populated nested literal", "Base{Audit{At: g.At}, g.Name}", nil, false, []string{"Base.*"}},
		{"empty literal", "Base{}", nil, false, []string{"Base.*"}},
		{"opaque call", "baseFrom(g)", nil, true, nil},
		// Statically zero-producing values carry nothing in, so they cannot
		// populate what they claim.
		{"new(T)", "new(Base)", nil, false, []string{"Base.*"}},
		{"typed nil conversion", "(*Base)(nil)", nil, false, []string{"Base.*"}},
		// An ordinary call that happens to take nil is NOT a conversion; only
		// the type checker could tell `Base(nil)` from a call, so neither is
		// claimed and both stay opaque.
		{"call with a nil argument", "baseFrom(nil)", nil, true, nil},
		// nil populates nothing: a nil embedded pointer emits none of its fields.
		{"nil", "nil", nil, false, []string{"Base.*"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, err := parser.ParseExpr(c.expr)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			assigned := map[string]bool{}
			recordAssignedValue(assigned, "Base", expr)
			if !assigned["Base"] {
				t.Error("the assigned path itself must always be recorded")
			}
			for _, k := range c.wantKeys {
				if !assigned[k] {
					t.Errorf("expected %q, got %v", k, assigned)
				}
			}
			if assigned["Base.*"] != c.wantTotal {
				t.Errorf("total coverage = %v, want %v (got %v)", assigned["Base.*"], c.wantTotal, assigned)
			}
			for _, a := range c.wantAbsent {
				if assigned[a] {
					t.Errorf("did not expect %q, got %v", a, assigned)
				}
			}
		})
	}
}

// TestRun_NilEmbedAssignmentPopulatesNothing is the end-to-end half: a
// converter that nils out a pointer embed emits none of its promoted fields, so
// the guard must not pass it. Paired with the same converter assigning a real
// value, which must pass.
func TestRun_NilEmbedAssignmentPopulatesNothing(t *testing.T) {
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
	*Base
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.Base = nil
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: a nil embed emits none of its promoted fields and must not count as population")
	}

	real := strings.Replace(wrapperSrc, "n.Base = nil", "n.Base = &Base{ID: g.Id, Content: g.Content}", 1)
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": real})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: a real pointer literal populates its fields, got %v", err)
	}
}

// TestFlattenEmbedded_MethodBearingEmbedIsRejected covers the failure that
// invalidates flattening wholesale rather than one tag: embedding a type with
// MarshalJSON promotes the method, so the EMBEDDING type implements
// json.Marshaler and encoding/json calls it instead of walking any of these
// fields. The tag set then describes nothing, so the walk refuses to judge and
// reports instead — the same fail-loud path as an unresolvable embed. The
// package already has a method-bearing type (FlexTime).
func TestFlattenEmbedded_MethodBearingEmbedIsRejected(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Stamp struct {
	At string ~json:"at"~
}

func (s Stamp) MarshalJSON() ([]byte, error) { return nil, nil }

type Plain struct {
	Name string ~json:"name"~
}

type Outer struct {
	Stamp
	Plain
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if len(outer.unresolved) == 0 {
		t.Error("embedding a type whose method is promoted must be reported, not silently flattened")
	}
	if outer.tags["at"] {
		t.Error("the method-bearing embed's tags must not be certified")
	}
}

// TestFlattenEmbedded_IgnoredEmbedDoesNotAnnihilate is the ordering case: a
// field encoding/json never sees cannot annihilate one it does. An anonymous
// unexported non-struct is dropped before dominance is resolved, so a real
// field sharing its tag stays on the wire — counting the hidden one first would
// report phantom drift on a wrapper that is correct.
func TestFlattenEmbedded_IgnoredEmbedDoesNotAnnihilate(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type hidden string

type Outer struct {
	hidden ~json:"secret"~
	Real   string ~json:"secret"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if !outer.tags["secret"] {
		t.Error("the real field survives: the ignored embed never reaches dominance")
	}
	if got := outer.tagToGoField["secret"]; got != "Real" {
		t.Errorf("expected the tag to resolve to the real field, got %q", got)
	}
}

// TestPopulationTargets_SameNamedEmbedIsNotTheLeaf checks a review claim that
// assigning an embed could be credited to a same-named promoted leaf. It
// cannot: `w.Deep = Deep{}` is recorded as the bare path "Deep", which is not
// a target of the leaf (the shadowing rule excludes the bare spelling, since
// `w.Deep` resolves to the embed), and a keyed literal grants no total marker.
// The ancestor spelling `Deep.*` requires an OPAQUE assignment of that embed,
// which genuinely does populate everything under it.
func TestPopulationTargets_SameNamedEmbedIsNotTheLeaf(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Deep struct {
	Deep string ~json:"deep_only"~
}

type Mid struct {
	Deep
}

type Outer struct {
	Mid
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	targets := map[string]bool{}
	for _, tgt := range outer.populationTargets("deep_only") {
		targets[tgt] = true
	}
	if targets["Deep"] {
		t.Error("the bare embed spelling must not be a target of the leaf it shadows")
	}
	if !targets["Deep.Deep"] || !targets["Mid.Deep.Deep"] {
		t.Errorf("the qualified spellings must be targets, got %v", outer.populationTargets("deep_only"))
	}
	// And the assignment side agrees: a keyed literal on the embed records the
	// path and its keys, never the bare-leaf spelling.
	expr, err := parser.ParseExpr("Deep{}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assigned := map[string]bool{}
	recordAssignedValue(assigned, "Deep", expr)
	populated := false
	for _, tgt := range outer.populationTargets("deep_only") {
		if assigned[tgt] {
			populated = true
		}
	}
	if populated {
		t.Error("assigning the embed with an empty literal must not populate the leaf")
	}
}

// TestFlattenEmbedded_IgnoredFieldNotPromotedFromDescendant is the depth-N half
// of the ignored-field rule: a tagged anonymous field of an unexported
// non-struct type is invisible to encoding/json wherever it is declared, so
// embedding the struct that declares it must not promote its tag either.
// Filtering only the root would have promoted a key that is on no wire.
func TestFlattenEmbedded_IgnoredFieldNotPromotedFromDescendant(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type hidden string

type Mid struct {
	hidden ~json:"ghost"~
	Real   string ~json:"real"~
}

type Outer struct {
	Mid
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["ghost"] {
		t.Error("a field encoding/json ignores cannot be promoted onto an embedding struct")
	}
	if !outer.tags["real"] {
		t.Errorf("the visible field must still promote, got %v", outer.tags)
	}
}

// TestFlattenEmbedded_MethodBearingInterfaceEmbedIsRejected covers the other
// way a custom marshaller reaches the embedding type: an embedded INTERFACE
// carrying MarshalJSON in its method set promotes it exactly as a struct's
// method would, and the interface has no fields for the walk to notice. Also
// covers an interface that gets the method by embedding another interface,
// since a method set is transitive.
func TestFlattenEmbedded_MethodBearingInterfaceEmbedIsRejected(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

type Fancy interface {
	Marshaler
	Extra()
}

type Plain interface {
	Do()
}

type Base struct {
	ID int64 ~json:"id"~
}

type Direct struct {
	Marshaler
	Base
}

type Transitive struct {
	Fancy
	Base
}

type Harmless struct {
	Plain
	Base
}
`))
	for _, name := range []string{"Direct", "Transitive"} {
		sf := structs[name]
		if sf == nil {
			t.Fatalf("%s not collected", name)
		}
		if len(sf.unresolved) == 0 {
			t.Errorf("%s: embedding an interface whose method set carries MarshalJSON must be reported", name)
		}
		if sf.tags["id"] {
			t.Errorf("%s: no tag may be certified once a custom marshaller is promoted", name)
		}
	}
	// An ordinary interface embed promotes no method that redirects the
	// encoder, so it stays the harmless no-op it always was.
	if h := structs["Harmless"]; h == nil || len(h.unresolved) != 0 || !h.tags["id"] {
		t.Errorf("a plain interface embed must not be rejected, got %+v", structs["Harmless"])
	}
}

// TestFlattenEmbedded_DiamondDoesNotPropagateDeeper pins a subtlety two
// reviewers read the other way, so it is worth stating precisely. In
// `Outer -> Left/Right -> Common -> Deep`, Common is reached twice and its own
// tags annihilate — but Deep is reached once, from the single queued Common,
// and its tags SURVIVE. That is not an accident of this walk; it is what the
// marshaller does:
//
//	json.Marshal(Outer{})  =>  {"deep_field":""}
//
// encoding/json's typeFields does `nextCount[ft]++` per arrival at a level, not
// `+= count[f.typ]`, so multiplicity does not cascade to a duplicated type's
// own embeds. Mirroring the mechanism gives this for free; "keep everything
// below a duplicate ambiguous" would drop a key that is really on the wire.
func TestFlattenEmbedded_DiamondDoesNotPropagateDeeper(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Deep struct {
	DeepField string ~json:"deep_field"~
}

type Common struct {
	Deep
	Shared string ~json:"shared"~
}

type Left struct{ Common }
type Right struct{ Common }

type Outer struct {
	Left
	Right
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["shared"] {
		t.Error("the duplicated type's OWN tags annihilate")
	}
	if !outer.tags["deep_field"] {
		t.Error("a tag below the duplicated type is still emitted by encoding/json and must be certified")
	}
}

// TestFlattenEmbedded_NameChainToMarshallerIsRefused records a deliberate
// over-approximation, and the reasoning belongs with it.
//
// Go distinguishes the two declaration forms: `type Same = Stamp` IS Stamp,
// methods included, while `type Safe Stamp` takes its fields with an empty
// method set — so Go would walk a struct embedding Safe. The walk refuses both.
//
// Telling them apart means tracking which declaration form each hop used and
// keeping that straight through interfaces, chains of hops and re-aliasing —
// the surface that produced four consecutive rounds of review findings, each
// about the previous round's rule. Both forms are name edges in one closure
// instead, which costs an over-report on a shape Go would accept and closes
// the class. The over-report names the embed and says what it will not vouch
// for; the failure it prevents is certifying tags a promoted marshaller never
// emits.
func TestFlattenEmbedded_NameChainToMarshallerIsRefused(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Stamp struct {
	At string ~json:"at"~
}

func (s Stamp) MarshalJSON() ([]byte, error) { return nil, nil }

type Safe Stamp
type Same = Stamp

type UsesDefined struct {
	Safe
}

type UsesAlias struct {
	Same
}
`))
	for _, name := range []string{"UsesDefined", "UsesAlias"} {
		sf := structs[name]
		if sf == nil {
			t.Fatalf("%s not collected", name)
		}
		if len(sf.unresolved) == 0 {
			t.Errorf("%s: a name chain reaching a marshaller is refused, whichever declaration form it uses", name)
		}
	}
}

// TestFlattenEmbedded_NameChainToOrdinaryTypeIsFine is the paired positive that
// keeps the rule above from becoming "refuse every name chain": a chain to an
// ordinary struct still flattens, so the over-approximation is scoped to
// chains that actually reach a method.
func TestFlattenEmbedded_NameChainToOrdinaryTypeIsFine(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Base struct {
	ID int64 ~json:"id"~
}

type Defined Base
type Aliased = Base

type UsesDefined struct{ Defined }
type UsesAlias struct{ Aliased }
`))
	for _, name := range []string{"UsesDefined", "UsesAlias"} {
		sf := structs[name]
		if sf == nil {
			t.Fatalf("%s not collected", name)
		}
		if len(sf.unresolved) != 0 {
			t.Errorf("%s: a chain to an ordinary struct is vouched, got %v", name, sf.unresolved)
		}
		if !sf.tags["id"] {
			t.Errorf("%s: its fields promote, got %v", name, sf.tags)
		}
	}
}

// TestFlattenEmbedded_TaggedMethodBearingEmbedIsRejected covers the gap a json
// tag opens in the method guard: the tag stops FIELD promotion, and nothing
// else. The struct still implements json.Marshaler through the promoted
// method, so the encoder never walks these fields.
func TestFlattenEmbedded_TaggedMethodBearingEmbedIsRejected(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Stamp struct {
	At string ~json:"at"~
}

func (s *Stamp) UnmarshalJSON(b []byte) error { return nil }

type Outer struct {
	Stamp ~json:"stamp"~
	Name  string ~json:"name"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if len(outer.unresolved) == 0 {
		t.Error("a tagged embed still promotes the type's methods and must be reported")
	}
}

// TestCollectJSONMethodTypes_ClosureIsPackageWide pins that the interface
// method-set closure runs over the merged package. The declaring interface and
// the one embedding it habitually live in different files, and closing per file
// would mark the first and never the second.
func TestCollectJSONMethodTypes_ClosureIsPackageWide(t *testing.T) {
	parse := func(source string) *ast.File {
		t.Helper()
		f, err := parser.ParseFile(token.NewFileSet(), "f.go", source, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		return f
	}
	a := parse(`package fixture

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}
`)
	b := parse(`package fixture

type Fancy interface {
	Marshaler
	Extra()
}
`)
	merged := map[string]bool{}
	graph := map[string][]string{}
	for _, f := range []*ast.File{b, a} { // reversed: the embedder is parsed first
		direct, edges := collectJSONMethodTypes(f)
		for k := range direct {
			merged[k] = true
		}
		for k, v := range edges {
			graph[k] = append(graph[k], v...)
		}
	}
	if merged["Fancy"] {
		t.Fatal("fixture setup: Fancy must only become a marshaler through the closure")
	}
	closeJSONMethodTypes(merged, graph)
	if !merged["Fancy"] {
		t.Error("an interface embedding a marshaler from ANOTHER file carries the method too")
	}
}

// TestRun_UnresolvableTaggedEmbedIsReported closes the tagged half of the
// vouching rule. A tag stops FIELD promotion and nothing else, so an
// unresolvable tagged embed can still promote a custom marshaller —
// `time.Time` is exactly such a type — and the walk cannot see either its
// fields or its methods. It reports instead of certifying the tags around it.
func TestRun_UnresolvableTaggedEmbedIsReported(t *testing.T) {
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

type Note struct {
	time.Time ~json:"at"~
	ID        int64 ~json:"id"~
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = g.Id
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": wrapperSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: an unresolvable TAGGED embed can promote a marshaller and must be reported")
	}
}

// TestFlattenEmbedded_AliasToMethodBearingInterfaceIsRejected covers a method
// set reached through an alias to an interface — the embed's own name is the
// alias, and the interface resolves to no struct, so neither key alone finds
// it. The vouching rule catches it because an alias carries the method set and
// resolution reports that it travelled.
func TestFlattenEmbedded_AliasToMethodBearingInterfaceIsRejected(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

type Alias = Marshaler

type Base struct {
	ID int64 ~json:"id"~
}

type Outer struct {
	Alias
	Base
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if len(outer.unresolved) == 0 {
		t.Error("an alias to a method-bearing interface promotes the method and must be reported")
	}
	if outer.tags["id"] {
		t.Error("a sibling embed's tags are just as meaningless once the encoder is redirected")
	}
}

// TestFlattenEmbedded_InheritedMarshallerIsRejected covers method promotion
// arriving through a chain rather than a declaration. Mid does not declare
// MarshalJSON — it embeds Wire, which does — so Mid promotes it and anything
// embedding Mid promotes it in turn. A json tag on that embed changes nothing,
// since tags govern field selection and not method sets, and the tag is
// precisely what stops the field walk from ever reaching Wire.
func TestFlattenEmbedded_InheritedMarshallerIsRejected(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Wire struct {
	Raw string ~json:"raw"~
}

func (w Wire) MarshalJSON() ([]byte, error) { return nil, nil }

type Mid struct {
	Wire
	Extra string ~json:"extra"~
}

type Base struct {
	ID int64 ~json:"id"~
}

type TaggedParent struct {
	Mid ~json:"mid"~
	Base
}

type UntaggedParent struct {
	Mid
	Base
}
`))
	for _, name := range []string{"TaggedParent", "UntaggedParent"} {
		sf := structs[name]
		if sf == nil {
			t.Fatalf("%s not collected", name)
		}
		if len(sf.unresolved) == 0 {
			t.Errorf("%s: a marshaller inherited through an embed is promoted just as far and must be reported", name)
		}
		if sf.tags["id"] {
			t.Errorf("%s: no promoted tag survives a redirected encoder", name)
		}
	}
}

// TestFlattenEmbedded_UnexportedPointerEmbedIsDecodeUnsafe covers a failure
// that is real for ONE direction only: encoding/json cannot allocate an
// embedded pointer to an unexported type ("cannot set embedded pointer to
// unexported struct"), so a decoder never populates its promoted fields —
// while marshalling and in-Go construction are unaffected. It is therefore
// recorded separately from the refusals, and run() reports it for tier-2 pairs
// alone, whose whole premise is that the decoder does the populating.
func TestFlattenEmbedded_UnexportedPointerEmbedIsDecodeUnsafe(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type hidden struct {
	ID int64 ~json:"id"~
}

type ByPointer struct {
	*hidden
}

type ByValue struct {
	hidden
}
`))
	p := structs["ByPointer"]
	if p == nil {
		t.Fatal("ByPointer not collected")
	}
	if len(p.decodeUnsafe) == 0 {
		t.Error("an unexported pointer embed must be recorded as decode-unsafe")
	}
	if len(p.unresolved) != 0 {
		t.Errorf("it is not a refusal: marshalling and construction work, got %v", p.unresolved)
	}
	if !p.tags["id"] {
		t.Error("the fields still promote — the problem is decoding, not the shape")
	}
	if v := structs["ByValue"]; v == nil || len(v.decodeUnsafe) != 0 || !v.tags["id"] {
		t.Errorf("the value form has no decode problem at all, got %+v", structs["ByValue"])
	}
}

// TestRun_UnexportedPointerEmbedReportedForDirectDecodeOnly is the pairing that
// makes the scoping meaningful: the SAME wrapper shape is reported when the
// pair is direct-decode and accepted when it is built by a *FromGenerated.
func TestRun_UnexportedPointerEmbedReportedForDirectDecodeOnly(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id int64 ~json:"id"~
}
`)
	tier2Src := src(`package basecamp

type hidden struct {
	ID int64 ~json:"id"~
}

type Note struct {
	*hidden
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": tier2Src})
	if err := run(wrapperDir, generatedFile, map[string]string{"Note": "Note"}, nil, false); err == nil {
		t.Error("run: a direct-decode pair relies on the decoder, which cannot allocate this embed")
	}

	tier1Src := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type hidden struct {
	ID int64 ~json:"id"~
}

type Note struct {
	*hidden
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.hidden = &hidden{}
	n.ID = g.Id
	return n
}
`)
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": tier1Src})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: a constructed wrapper initializes the embed itself and is unaffected, got %v", err)
	}
}

// TestIsJSONMethod_SignatureMatters guards the guard: only a method that
// actually satisfies json.Marshaler / json.Unmarshaler redirects the encoder,
// so a same-named method with a different signature must not make the walk
// refuse a struct encoding/json walks normally.
func TestIsJSONMethod_SignatureMatters(t *testing.T) {
	cases := []struct {
		decl string
		want bool
	}{
		{"func (s Stamp) MarshalJSON() ([]byte, error) { return nil, nil }", true},
		{"func (s *Stamp) UnmarshalJSON(b []byte) error { return nil }", true},
		{"func (s Stamp) MarshalJSON(n int) []byte { return nil }", false},
		{"func (s Stamp) MarshalJSON() []byte { return nil }", false},
		{"func (s *Stamp) UnmarshalJSON(b string) error { return nil }", false},
		{"func (s *Stamp) UnmarshalJSON(b []byte) (int, error) { return 0, nil }", false},
		// encoding/json falls back to encoding.TextMarshaler, so a promoted
		// MarshalText redirects the encoder too.
		{"func (s Stamp) MarshalText() ([]byte, error) { return nil, nil }", true},
		{"func (s *Stamp) UnmarshalText(b []byte) error { return nil }", true},
		{"func (s Stamp) MarshalText() string { return \"\" }", false},
		{"func (s Stamp) String() string { return \"\" }", false},
		// A signature spelled through a local alias cannot be evaluated here,
		// and the conservative reading is the one that does not certify a
		// wrapper whose marshaller was hidden behind a name.
		{"func (s Stamp) MarshalJSON() (Bytes, error) { return nil, nil }", true},
	}
	for _, c := range cases {
		source := "package fixture" + "\n\n" + c.decl + "\n"
		f, err := parser.ParseFile(token.NewFileSet(), "f.go", source, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %q: %v", c.decl, err)
		}
		got, _ := collectJSONMethodTypes(f)
		if got["Stamp"] != c.want {
			t.Errorf("%s: recognized = %v, want %v", c.decl, got["Stamp"], c.want)
		}
	}
}

// TestFlattenEmbedded_DefinedTypeOverInterfaceKeepsMethods separates the two
// defined-type cases. `type Safe Stamp` over a STRUCT starts with an empty
// method set, so it is safe to walk; `type Safe Marshaler` over an INTERFACE is
// still an interface with that method set, so embedding it redirects the
// encoder. Treating them alike in either direction is wrong in one of them.
func TestFlattenEmbedded_DefinedTypeOverInterfaceKeepsMethods(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

type SafeIface Marshaler

type Base struct {
	ID int64 ~json:"id"~
}

type Outer struct {
	SafeIface
	Base
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if len(outer.unresolved) == 0 {
		t.Error("a defined type over an interface keeps its method set and must be reported")
	}
}

// TestFlattenEmbedded_QualifiedInterfaceEmbedCounts covers an interface whose
// method set comes from another package (`type Local interface { json.Marshaler }`).
// The check does not parse that package, so the method set is unknown — and
// unknown is answered with a refusal rather than an assumption.
func TestFlattenEmbedded_QualifiedInterfaceEmbedCounts(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

import "encoding/json"

type Local interface {
	json.Marshaler
}

type Base struct {
	ID int64 ~json:"id"~
}

type Outer struct {
	Local
	Base
}
`))
	if o := structs["Outer"]; o == nil || len(o.unresolved) == 0 {
		t.Errorf("an interface embedding another package's must not be assumed method-free, got %+v", structs["Outer"])
	}
}

// TestFlattenEmbedded_BuiltinBackedTypeResolves guards the other direction of
// that rule: a defined type over a BUILTIN is fully accounted for — no fields,
// no method set of its own — and must not be refused as unresolvable. Treating
// it as unknown would report drift on an ordinary wrapper.
func TestFlattenEmbedded_BuiltinBackedTypeResolves(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Kind string

type Outer struct {
	Kind
	Name string ~json:"name"~
}
`))
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if len(outer.unresolved) != 0 {
		t.Errorf("a builtin-backed defined type is resolved, not unknown, got %v", outer.unresolved)
	}
	if !outer.tags["name"] {
		t.Errorf("its sibling fields must be judged normally, got %v", outer.tags)
	}
}

// TestFlattenEmbedded_AliasedInterfaceInGraph covers a method set reached
// through an aliased interface name inside the interface graph itself
// (`type M interface{ MarshalJSON() … }; type A = M; type B interface { A }`).
// Since every single-identifier declaration is an edge, the closure marks A and
// then B, and a struct embedding B is refused.
func TestFlattenEmbedded_AliasedInterfaceInGraph(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type M interface {
	MarshalJSON() ([]byte, error)
}

type A = M

type B interface {
	A
	Other()
}

type Base struct {
	ID int64 ~json:"id"~
}

type Outer struct {
	B
	Base
}
`))
	if o := structs["Outer"]; o == nil || len(o.unresolved) == 0 {
		t.Errorf("an interface embedding an ALIASED marshaler still promotes the method, got %+v", structs["Outer"])
	}
}

// TestFlattenEmbedded_MethodOnIntermediateNameIsRefused covers the other order:
// the method is declared on a name in the middle of the chain rather than at
// either end (`type Custom Wire; func (Custom) MarshalJSON(); type Alias =
// Custom`). Closing over name edges catches it wherever in the chain it sits,
// which is the point of doing this with a closure rather than by inspecting
// the two endpoints.
func TestFlattenEmbedded_MethodOnIntermediateNameIsRefused(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Wire struct {
	Raw string ~json:"raw"~
}

type Custom Wire

func (c Custom) MarshalJSON() ([]byte, error) { return nil, nil }

type Alias = Custom

type Outer struct {
	Alias
}
`))
	if o := structs["Outer"]; o == nil || len(o.unresolved) == 0 {
		t.Errorf("a method declared mid-chain is still promoted and must be refused, got %+v", structs["Outer"])
	}
}

// TestFlattenEmbedded_TaggedUnexportedPointerIsDecodeUnsafe pairs with the
// untagged case: a json tag changes which KEY the field appears under, not
// whether the decoder can allocate it.
func TestFlattenEmbedded_TaggedUnexportedPointerIsDecodeUnsafe(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type hidden struct {
	ID int64 ~json:"id"~
}

type W struct {
	*hidden ~json:"hidden"~
}
`))
	w := structs["W"]
	if w == nil {
		t.Fatal("W not collected")
	}
	if len(w.decodeUnsafe) == 0 {
		t.Error("a TAGGED unexported pointer embed is just as undecodable as an untagged one")
	}
}

// TestFlattenEmbedded_ParenthesizedDeclarationsFollowed covers legal but
// unusual `type Alias = (Base)` syntax. The parentheses are not part of the
// type, and dropping the declaration on the floor because of them is the
// silent-skip pattern this whole check exists to remove — here it would lose a
// name edge and certify a wrapper whose promoted marshaller sat behind it.
func TestFlattenEmbedded_ParenthesizedDeclarationsFollowed(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Stamp struct {
	At string ~json:"at"~
}

func (s Stamp) MarshalJSON() ([]byte, error) { return nil, nil }

type Alias = (Stamp)

type Plain struct {
	ID int64 ~json:"id"~
}

type PlainAlias = (Plain)

type Outer struct {
	Alias
}

type Harmless struct {
	PlainAlias
}
`))
	if o := structs["Outer"]; o == nil || len(o.unresolved) == 0 {
		t.Errorf("a parenthesized alias to a marshaller is still an edge to it, got %+v", structs["Outer"])
	}
	// And the paired positive: parentheses alone must not cause a refusal.
	h := structs["Harmless"]
	if h == nil {
		t.Fatal("Harmless not collected")
	}
	if len(h.unresolved) != 0 {
		t.Errorf("a parenthesized alias to an ordinary struct is vouched, got %v", h.unresolved)
	}
	if !h.tags["id"] {
		t.Errorf("and its fields promote, got %v", h.tags)
	}
}

// TestFlattenEmbedded_UnexportedNonStructPointerIsNotDecodeUnsafe narrows the
// allocation rule to what encoding/json actually does. An anonymous field of an
// unexported NON-struct type is ignored outright — never traversed, never
// allocated — so there is no field the decoder failed to populate, and
// reporting one would fail a direct-decode wrapper that is fine. The struct
// form is the real case and must keep failing.
func TestFlattenEmbedded_UnexportedNonStructPointerIsNotDecodeUnsafe(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type hiddenStr string

type hiddenStruct struct {
	ID int64 ~json:"id"~
}

type NonStruct struct {
	*hiddenStr
	Name string ~json:"name"~
}

type TaggedNonStruct struct {
	*hiddenStr ~json:"h"~
	Name       string ~json:"name"~
}

type Struct struct {
	*hiddenStruct
}
`))
	for _, name := range []string{"NonStruct", "TaggedNonStruct"} {
		sf := structs[name]
		if sf == nil {
			t.Fatalf("%s not collected", name)
		}
		if len(sf.decodeUnsafe) != 0 {
			t.Errorf("%s: encoding/json ignores this field rather than failing to allocate it, got %v", name, sf.decodeUnsafe)
		}
	}
	if sf := structs["Struct"]; sf == nil || len(sf.decodeUnsafe) == 0 {
		t.Errorf("the struct form is the real allocation failure and must still be reported, got %+v", structs["Struct"])
	}
}

// TestRun_UnrecognizedAssignmentShapeIsReported is the flipped default, which
// is the same correction as the original #599 bug one layer down: the first
// walker met an anonymous field it did not understand and dropped it; this one
// met a value shape it did not understand and CREDITED it. Both are silent
// assumptions, and both are now reports.
//
// The paired positive is the shape the walker does understand — the same
// wrapper with a literal — so "report the unknown" cannot be satisfied by
// reporting everything.
func TestRun_UnrecognizedAssignmentShapeIsReported(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id      int64  ~json:"id"~
	Content string ~json:"content"~
}
`)
	// A binary expression is not a shape the classifier models.
	oddSrc := src(`package basecamp

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
	n.Base = *baseOf(g) + *baseOf(g)
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": oddSrc})
	stderr := captureStderr(t, func() {
		if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
			t.Error("run: a value shape the walker cannot interpret must be reported, not credited")
		}
	})
	// The diagnosis matters as much as the failure: "unpopulated" would send
	// the reader looking for a missing assignment that is right there.
	if !strings.Contains(stderr, "shape this walker does not interpret") {
		t.Errorf("expected the report to name the uninterpreted shape, got:\n%s", stderr)
	}

	knownSrc := strings.Replace(oddSrc, "*baseOf(g) + *baseOf(g)", "Base{ID: g.Id, Content: g.Content}", 1)

	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": knownSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: a shape the walker DOES interpret must still pass, got %v", err)
	}
}

// TestRecordAssignedValue_ElidedNestedLiteral covers the elided form Go permits
// for a nested literal. `Base{Audit: {At: g.At}}` sets exactly one field of
// Audit; treating the inner literal as opaque would credit every field under
// it — the silent direction — so it is enumerated like any other keyed literal.
func TestRecordAssignedValue_ElidedNestedLiteral(t *testing.T) {
	expr, err := parser.ParseExpr("Base{Audit: {At: g.At}, ID: g.Id}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assigned := map[string]bool{}
	recordAssignedValue(assigned, "Base", expr)
	for _, want := range []string{"Base", "Base.ID", "Base.Audit", "Base.Audit.At"} {
		if !assigned[want] {
			t.Errorf("expected %q, got %v", want, assigned)
		}
	}
	for _, absent := range []string{"Base.*", "Base.Audit.*"} {
		if assigned[absent] {
			t.Errorf("an enumerated literal must not grant %q, got %v", absent, assigned)
		}
	}
	// An elided EMPTY literal contributes nothing, exactly like a typed one.
	expr2, err := parser.ParseExpr("Base{Audit: {}}")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assigned2 := map[string]bool{}
	recordAssignedValue(assigned2, "Base", expr2)
	if assigned2["Base.Audit.*"] {
		t.Errorf("an empty elided literal populates nothing, got %v", assigned2)
	}
}

// TestExtractJSONTag_InterpretedLiteral covers the interpreted (double-quoted)
// struct tag Go also accepts. Reading it as untagged is not a cosmetic miss: on
// an ANONYMOUS field it flips the field from an ordinary member into a
// promotion source, certifying members that are not on the wire.
func TestExtractJSONTag_InterpretedLiteral(t *testing.T) {
	if got := extractJSONTag(`"json:\"base\""`); got != "base" {
		t.Errorf("interpreted tag literal: got %q, want \"base\"", got)
	}
	if got := extractJSONTag("`json:\"base\"`"); got != "base" {
		t.Errorf("raw tag literal: got %q, want \"base\"", got)
	}
	if got := extractJSONTag(`"not a tag`); got != "" {
		t.Errorf("malformed literal must yield no tag, got %q", got)
	}
}

// TestFlattenEmbedded_InterpretedTagStopsPromotion is the consequence of the
// above at the walk level: a tagged anonymous field is an ordinary field
// whichever literal form its tag uses.
func TestFlattenEmbedded_InterpretedTagStopsPromotion(t *testing.T) {
	structs := flattenFixture(t, `package fixture

type Base struct {
	ID int64 `+"`json:\"id\"`"+`
}

type Outer struct {
	Base "json:\"base\""
}
`)
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["id"] {
		t.Error("a field tagged with an interpreted literal is not a promotion source")
	}
	if !outer.tags["base"] {
		t.Errorf("it registers under its own tag, got %v", outer.tags)
	}
}

// TestCollectJSONMethodTypes_ParenthesizedReceiver covers `func (m (M)) …`.
// Legal, essentially unwritten — but missing it drops M from the method set,
// which is a SILENT miss: a wrapper embedding M would be certified while the
// encoder is redirected. One unwrap, because the failure is in the silent class.
func TestCollectJSONMethodTypes_ParenthesizedReceiver(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "f.go", `package fixture

type M struct{}

func (m (M)) MarshalJSON() ([]byte, error) { return nil, nil }
`, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, _ := collectJSONMethodTypes(f)
	if !got["M"] {
		t.Error("a parenthesized receiver still declares the method")
	}
}

// TestExtractJSONTag_EscapedQuoteInAnotherTag covers a tag body whose EARLIER
// value contains an escaped quote. The hand-rolled scanner that used to live
// here stopped at it and reported no json tag, which on an anonymous field
// silently turns a tagged member into a promotion source. Parsing is now
// reflect's job — the same parser encoding/json consults — so the two cannot
// disagree.
func TestExtractJSONTag_EscapedQuoteInAnotherTag(t *testing.T) {
	// `xml:"a\"b" json:"base"` as it appears in source, inside a raw literal.
	literal := "`" + `xml:"a\"b" json:"base"` + "`"
	if got := extractJSONTag(literal); got != "base" {
		t.Errorf("escaped quote in a preceding tag: got %q, want \"base\"", got)
	}
	// And the consequence at the walk level: such a field is NOT a promotion
	// source, so the embedded type's tags must not appear on the outer struct.
	structs := flattenFixture(t, "package fixture\n\ntype Base struct {\n\tID int64 `json:\"id\"`\n}\n\ntype Outer struct {\n\tBase "+literal+"\n}\n")
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if outer.tags["id"] {
		t.Error("a tagged anonymous field is not a promotion source, whatever the earlier tags contain")
	}
	if !outer.tags["base"] {
		t.Errorf("it registers under its own json tag, got %v", outer.tags)
	}
}

// TestFlattenEmbedded_GenericInstantiationIsReported covers a name declared
// over an instantiated generic (`type A G[int]`). Such a type keeps G's fields
// — and an alias to one keeps its method set — so treating it as fieldless
// would drop those tags from BOTH sides of a pair and let a wrapper omit them
// with no report at all. Modelling instantiation is the type checker's job, so
// the walk reports instead.
func TestFlattenEmbedded_GenericInstantiationIsReported(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type G[T any] struct {
	Value T ~json:"value"~
}

type Defined G[int]
type Aliased = G[int]

type UsesDefined struct {
	Defined
}

type UsesAlias struct {
	Aliased
}
`))
	for _, name := range []string{"UsesDefined", "UsesAlias"} {
		sf := structs[name]
		if sf == nil {
			t.Fatalf("%s not collected", name)
		}
		if len(sf.unresolved) == 0 {
			t.Errorf("%s: an instantiated generic is not modelled and must be reported, not treated as fieldless", name)
		}
	}
}

// TestFlattenEmbedded_DecodeUnsafeTravelsToTheEmbeddingStruct is the promotion
// half of the allocation rule. `Base` holds a tagged anonymous `*hidden`; a
// wrapper embedding `Base` promotes that field under the same key, and the
// decoder can no more allocate it there than it could in Base. Recording it
// only while Base is the struct being judged would leave every wrapper that
// embeds Base silently unpopulated — and for a tier-2 pair, tag presence IS
// the population claim.
func TestFlattenEmbedded_DecodeUnsafeTravelsToTheEmbeddingStruct(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type hidden struct {
	ID int64 ~json:"id"~
}

type Base struct {
	*hidden ~json:"hidden"~
	Name    string ~json:"name"~
}

type Outer struct {
	Base
}
`))
	if b := structs["Base"]; b == nil || len(b.decodeUnsafe) == 0 {
		t.Fatalf("Base itself must record it, got %+v", structs["Base"])
	}
	outer := structs["Outer"]
	if outer == nil {
		t.Fatal("Outer not collected")
	}
	if len(outer.decodeUnsafe) == 0 {
		t.Error("the record must travel to the struct that promotes the field")
	}
	if !outer.tags["name"] {
		t.Errorf("the rest of Base still promotes normally, got %v", outer.tags)
	}
}

// TestIsJSONMethod_ByteAliasSpelling covers `[]uint8`. byte IS uint8, so
// `MarshalJSON() ([]uint8, error)` implements json.Marshaler and its promotion
// redirects the encoder — missing it is a silent certification, not a cosmetic
// mismatch.
func TestIsJSONMethod_ByteAliasSpelling(t *testing.T) {
	for _, decl := range []string{
		"func (s Stamp) MarshalJSON() ([]uint8, error) { return nil, nil }",
		"func (s *Stamp) UnmarshalJSON(b []uint8) error { return nil }",
	} {
		f, err := parser.ParseFile(token.NewFileSet(), "f.go", "package fixture\n\ntype Stamp struct{}\n\n"+decl+"\n", parser.ParseComments)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		got, _ := collectJSONMethodTypes(f)
		if !got["Stamp"] {
			t.Errorf("%s: []uint8 is []byte and must be recognized", decl)
		}
	}
}

// TestFlattenEmbedded_RootMarshallerWithEmbedIsRefused covers the combination
// that is silent: a wrapper that declares its own MarshalJSON AND promotes
// fields through an embed certifies tags against an encoder that bypasses them.
//
// The pairing matters. A root method ALONE is not disqualifying — every wrapper
// in this repo that declares one (Upload, SearchResult, RichTextAttachment, …)
// is a decode adapter that unmarshals into the generated type and hands off to
// the *FromGenerated conversion, so the pairing is what it is built on. The
// second case asserts that shape keeps working.
func TestFlattenEmbedded_RootMarshallerWithEmbedIsRefused(t *testing.T) {
	structs := flattenFixture(t, src(`package fixture

type Base struct {
	ID int64 ~json:"id"~
}

type WithEmbed struct {
	Base
	Name string ~json:"name"~
}

func (w WithEmbed) MarshalJSON() ([]byte, error) { return nil, nil }

type Adapter struct {
	ID   int64  ~json:"id"~
	Name string ~json:"name"~
}

func (a *Adapter) UnmarshalJSON(b []byte) error { return nil }
`))
	we := structs["WithEmbed"]
	if we == nil {
		t.Fatal("WithEmbed not collected")
	}
	if len(we.unresolved) == 0 {
		t.Error("a root marshaller over promoted fields certifies tags the encoder bypasses; it must be reported")
	}
	if we.tags["id"] {
		t.Error("the promoted tag must not be certified")
	}
	if !we.tags["name"] {
		t.Errorf("its OWN declarations are still its own, got %v", we.tags)
	}
	// The decode-adapter shape — a root method and no embed — is untouched.
	ad := structs["Adapter"]
	if ad == nil {
		t.Fatal("Adapter not collected")
	}
	if len(ad.unresolved) != 0 {
		t.Errorf("a root method with no promotion is the repo's own adapter shape and must pass, got %v", ad.unresolved)
	}
	if !ad.tags["id"] || !ad.tags["name"] {
		t.Errorf("its tags are judged normally, got %v", ad.tags)
	}
}

// TestCollectJSONMethodTypes_UnknownRHSCounts covers the method graph's half of
// the "unrecognised is never credited" invariant. A type declaration whose
// right-hand side this check cannot evaluate — a qualified name, a generic
// instantiation — has an UNKNOWN method set, and unknown counts as carrying.
// Skipping it silently is how a struct embedding the alias gets its sibling
// tags certified while a promoted MarshalJSON redirects the encoder.
//
// The paired negatives matter as much: a type literal has no method set of its
// own, so `type A = struct{…}` and friends must NOT be marked, or the rule
// degenerates into refusing every declaration.
func TestCollectJSONMethodTypes_UnknownRHSCounts(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "f.go", `package fixture

import "encoding/json"

type QualifiedAlias = json.Marshaler
type Generic[T any] struct{ V T }
type GenericAlias = Generic[int]

type ViaInterface interface {
	QualifiedAlias
}

type PlainStruct = struct{ ID int64 }
type PlainMap map[string]string
type PlainFunc func() error
`, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	direct, graph := collectJSONMethodTypes(f)
	closeJSONMethodTypes(direct, graph)
	for _, name := range []string{"QualifiedAlias", "GenericAlias"} {
		if !direct[name] {
			t.Errorf("%s: an unevaluable right-hand side has an unknown method set and must count as carrying", name)
		}
	}
	// And it propagates: an interface embedding the unknown alias carries too.
	if !direct["ViaInterface"] {
		t.Error("an interface embedding a name with an unknown method set carries it as well")
	}
	for _, name := range []string{"PlainStruct", "PlainMap", "PlainFunc"} {
		if direct[name] {
			t.Errorf("%s: a type literal has no method set of its own and must not be marked", name)
		}
	}
}

// TestCollectJSONMethodTypes_GenericReceiver covers the other silent skip in
// the method graph: `func (b Base[T]) MarshalJSON()` declares the method on
// Base, and reading the receiver as unrecognized left Base unmarked — so a
// wrapper embedding it would be certified while the encoder is redirected.
func TestCollectJSONMethodTypes_GenericReceiver(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "f.go", `package fixture

type Base[T any] struct{ V T }

func (b Base[T]) MarshalJSON() ([]byte, error) { return nil, nil }

type Two[A any, B any] struct{}

func (t *Two[A, B]) UnmarshalJSON(b []byte) error { return nil }
`, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got, _ := collectJSONMethodTypes(f)
	if !got["Base"] {
		t.Error("a generic receiver declares the method on its base type")
	}
	if !got["Two"] {
		t.Error("the multi-parameter receiver form counts too")
	}
}

// TestRun_OrdinaryFieldValuesAreNotReported is the regression for an
// over-report the unrecognised-shape rule introduced: `recordAssignedValue`
// runs for EVERY field, so a literal or an expression assigned to an ordinary
// scalar — `n.ID = 1`, `n.Name = g.Name + "!"` — was being reported as
// uninterpretable.
//
// Every expression yields a value of its field's type; an unclassifiable one
// only matters when the walker would otherwise credit what is UNDER it. On a
// scalar there is nothing underneath, so the report is scoped to embeds on a
// promoted field's path — which the second half of this test still catches.
func TestRun_OrdinaryFieldValuesAreNotReported(t *testing.T) {
	genSrc := src(`package generated

type Note struct {
	Id   int64  ~json:"id"~
	Name string ~json:"name"~
}
`)
	scalarSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Note struct {
	ID   int64  ~json:"id"~
	Name string ~json:"name"~
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.ID = 1
	n.Name = g.Name + "!"
	return n
}
`)
	wrapperDir, generatedFile := writeDriftFixtures(t, genSrc, map[string]string{"note.go": scalarSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err != nil {
		t.Errorf("run: ordinary field assignments must not be reported as uninterpretable, got %v", err)
	}

	// The same unclassifiable shape assigned to an EMBED still reports: there
	// the walker would have credited every field underneath.
	embedSrc := src(`package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Base struct {
	ID   int64  ~json:"id"~
	Name string ~json:"name"~
}

type Note struct {
	Base
}

func noteFromGenerated(g generated.Note) Note {
	n := Note{}
	n.Base = *baseOf(g) + *baseOf(g)
	return n
}
`)
	wrapperDir, generatedFile = writeDriftFixtures(t, genSrc, map[string]string{"note.go": embedSrc})
	if err := run(wrapperDir, generatedFile, nil, nil, false); err == nil {
		t.Error("run: an uninterpretable value assigned to an embed must still be reported")
	}
}
