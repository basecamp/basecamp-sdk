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
	src := `package fixture

import "generated"

// barFromGenerated maps generated.Bar to Bar.
func barFromGenerated(g generated.Bar) Bar { return Bar{} }

// receiverFnFromGenerated is a method, must be skipped.
func (s *Service) receiverFnFromGenerated(g generated.X) X { return X{} }

// noGeneratedPrefix is missing the generated. qualifier on the param, skipped.
func wrongParamFromGenerated(g Bar) Bar { return Bar{} }
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
	if _, ok := pairs["X"]; ok {
		t.Error("method receiver fn must be excluded from pair extraction")
	}
	if _, ok := pairs["Foo"]; ok {
		t.Error("non-generated-prefixed param must be excluded")
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
	if err := run(wrapperDir, generatedFile, nil, false); err != nil {
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
	if err := run(wrapperDir, generatedFile, nil, false); err == nil {
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
	if err := run(wrapperDir, generatedFile, nil, false); err == nil {
		t.Error("run: expected population drift on unassigned Tagline, got nil")
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
	if err := run(wrapperDir, generatedFile, nil, false); err != nil {
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
	if err := run(wrapperDir, generatedFile, nil, false); err == nil {
		t.Error("run: expected drift on stale omit marker not_a_real_tag, got nil")
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
