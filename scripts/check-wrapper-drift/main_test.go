package main

import (
	"go/parser"
	"go/token"
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

// TestEndToEnd_HappyAndDrift drives the full check against two minimal
// fixtures: one in sync, one with a missing tag and an omitted-marker hit.
func TestEndToEnd_HappyAndDrift(t *testing.T) {
	genSrc := `package generated

type Foo struct {
	Id    int64  ` + "`json:\"id\"`" + `
	Title string ` + "`json:\"title\"`" + `
	Hidden string ` + "`json:\"hidden,omitempty\"`" + `
}

type Bar struct {
	Id   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
	NewField string ` + "`json:\"new_field,omitempty\"`" + `
}
`
	// Wrapper fixture: Foo is in sync (Hidden marked as intentionally-omitted),
	// Bar is drifted (missing NewField).
	wrapperSrc := `package basecamp

import "github.com/basecamp/basecamp-sdk/go/pkg/generated"

type Foo struct {
	ID    int64  ` + "`json:\"id\"`" + `
	Title string ` + "`json:\"title\"`" + `
	// intentionally-omitted: hidden - internal echo, not part of the public surface
	internalNote string
}

func fooFromGenerated(g generated.Foo) Foo { return Foo{} }

type Bar struct {
	ID   int64  ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func barFromGenerated(g generated.Bar) Bar { return Bar{} }
`
	fset := token.NewFileSet()
	genFile, err := parser.ParseFile(fset, "gen.go", genSrc, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse gen: %v", err)
	}
	wrapFile, err := parser.ParseFile(fset, "wrapper.go", wrapperSrc, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse wrap: %v", err)
	}
	genStructs := collectStructsAndMarkers(fset, genFile)
	wrapStructs := collectStructsAndMarkers(fset, wrapFile)
	pairs := collectFromGeneratedPairs(wrapFile)

	// Foo: in sync (Hidden marked as intentionally-omitted).
	fooGen := genStructs["Foo"]
	fooWrap := wrapStructs["Foo"]
	for tag := range fooGen.tags {
		if !fooWrap.tags[tag] && !fooWrap.omitted[tag] {
			t.Errorf("Foo: expected tag %q to be matched or omitted, got drift", tag)
		}
	}

	// Bar: missing new_field, no marker → drift expected.
	barGen := genStructs["Bar"]
	barWrap := wrapStructs["Bar"]
	missing := []string{}
	for tag := range barGen.tags {
		if !barWrap.tags[tag] && !barWrap.omitted[tag] {
			missing = append(missing, tag)
		}
	}
	if len(missing) != 1 || missing[0] != "new_field" {
		t.Errorf("Bar: expected drift on [new_field], got %v", missing)
	}

	// Sanity: pair extraction worked.
	if pairs["Foo"] != "Foo" || pairs["Bar"] != "Bar" {
		t.Errorf("pair extraction: got %v", pairs)
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
