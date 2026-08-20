package basecamp

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	gotypes "go/types"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
	"github.com/basecamp/basecamp-sdk/go/pkg/types"
)

// Optional timestamps on the hand-written wrapper surface must be *time.Time.
//
// A value-typed time.Time cannot represent absence, and — unlike the string,
// bool and numeric optionals that make up the rest of this surface — it cannot
// even hide that fact behind `omitempty`. encoding/json's "empty" set is
// false / 0 / nil pointer / nil interface / empty array, slice, map, string.
// A struct is never empty, and time.Time is a struct, so `,omitempty` on a
// value-typed time.Time is INERT: the key is emitted on every re-marshal,
// carrying 0001-01-01T00:00:00Z as if the server had sent it.
//
// That is the whole distinction. `Title string \`json:"title,omitempty"\`` is
// lossy in a benign way (absent and "" collapse, and both are then omitted, so
// the value round-trips). `UpdatedAt time.Time \`json:"updated_at,omitempty"\``
// is lossy in a way that fabricates data on the wire.

// timestampCarrier is one (wrapper type, wire JSON) pair whose optional
// timestamp keys must survive a decode/re-encode round trip: absent in, absent
// out; present in, byte-identical out.
type timestampCarrier struct {
	name string
	// decode unmarshals wire into a fresh value of the wrapper type and
	// returns it for re-marshaling. Written as a closure so this table can
	// hold heterogeneous wrapper types without reflection.
	decode func(t *testing.T, wire []byte) any
	// keys are the optional timestamp JSON keys under test.
	keys []string
	// present is a wire body carrying every key in keys.
	present []byte
}

func unmarshalInto[T any](t *testing.T, wire []byte) any {
	t.Helper()
	var v T
	if err := json.Unmarshal(wire, &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return v
}

func timestampCarriers() []timestampCarrier {
	return []timestampCarrier{
		{
			name:    "HillChart",
			decode:  unmarshalInto[HillChart],
			keys:    []string{"updated_at"},
			present: []byte(`{"enabled":true,"stale":false,"updated_at":"2026-07-31T12:34:56Z"}`),
		},
		{
			name:   "Notification",
			decode: unmarshalInto[Notification],
			keys:   []string{"read_at", "unread_at"},
			present: []byte(`{"id":1,"created_at":"2026-07-31T12:34:56Z","updated_at":"2026-07-31T12:34:56Z",` +
				`"read_at":"2026-07-31T12:34:56Z","unread_at":"2026-07-30T01:02:03Z"}`),
		},
		{
			name:   "SearchResult",
			decode: unmarshalInto[SearchResult],
			keys:   []string{"created_at", "updated_at"},
			present: []byte(`{"id":1,"created_at":"2026-07-31T12:34:56Z","updated_at":"2026-07-30T01:02:03Z",` +
				`"content":null,"description":null}`),
		},
		// #620: the five holdouts of the same class. Each was typed value
		// time.Time with a bare `json:"..."` tag — no omitempty to even hint the
		// field was optional — while its generated counterpart was *time.Time.
		{
			name:    "TimelineEvent",
			decode:  unmarshalInto[TimelineEvent],
			keys:    []string{"created_at"},
			present: []byte(`{"id":1,"created_at":"2026-07-31T12:34:56Z","kind":"todo_created"}`),
		},
		{
			name:    "WebhookDelivery",
			decode:  unmarshalInto[WebhookDelivery],
			keys:    []string{"created_at"},
			present: []byte(`{"id":1,"created_at":"2026-07-31T12:34:56Z"}`),
		},
		{
			name:    "QuestionReminder",
			decode:  unmarshalInto[QuestionReminder],
			keys:    []string{"remind_at"},
			present: []byte(`{"remind_at":"2026-07-31T12:34:56Z"}`),
		},
		{
			name:   "ClientApprovalResponse",
			decode: unmarshalInto[ClientApprovalResponse],
			keys:   []string{"created_at", "updated_at"},
			present: []byte(`{"id":1,"created_at":"2026-07-31T12:34:56Z",` +
				`"updated_at":"2026-07-30T01:02:03Z","approved":true}`),
		},
	}
}

// marshalToKeys re-marshals v and returns its top-level object as a key map.
// Keys are compared by exact map lookup rather than substring search: read_at
// is a substring of unread_at, and a bytes.Contains check would silently pass
// whenever either key survived.
func marshalToKeys(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("re-unmarshal %s: %v", b, err)
	}
	return out
}

// TestOptionalTimestampsOmitWhenAbsent is the red proof for #562: an absent
// optional timestamp must not reappear on re-marshal.
//
// Against the un-fixed tree every case fails, reporting the fabricated
// 0001-01-01T00:00:00Z that the value-typed field emitted.
func TestOptionalTimestampsOmitWhenAbsent(t *testing.T) {
	for _, c := range timestampCarriers() {
		t.Run(c.name, func(t *testing.T) {
			got := marshalToKeys(t, c.decode(t, []byte(`{}`)))
			for _, k := range c.keys {
				if raw, ok := got[k]; ok {
					t.Errorf("%s.%s was absent on the wire but re-marshaled as %s; "+
						"a value-typed time.Time cannot represent absence and `,omitempty` never fires for a struct",
						c.name, k, raw)
				}
			}
		})
	}
}

// TestOptionalTimestampsRoundTripWhenPresent is the positive control: the fix
// must not turn a real timestamp into an omitted one.
func TestOptionalTimestampsRoundTripWhenPresent(t *testing.T) {
	for _, c := range timestampCarriers() {
		t.Run(c.name, func(t *testing.T) {
			var in map[string]json.RawMessage
			if err := json.Unmarshal(c.present, &in); err != nil {
				t.Fatalf("fixture is not an object: %v", err)
			}
			got := marshalToKeys(t, c.decode(t, c.present))
			for _, k := range c.keys {
				want, ok := in[k]
				if !ok {
					t.Fatalf("test fixture for %s omits %s, so this case proves nothing", c.name, k)
				}
				raw, ok := got[k]
				if !ok {
					t.Errorf("%s.%s was present on the wire (%s) but did not survive re-marshal", c.name, k, want)
					continue
				}
				if string(raw) != string(want) {
					t.Errorf("%s.%s round-tripped as %s, want %s", c.name, k, raw, want)
				}
			}
		})
	}
}

// TestOptionalTimestampsSurviveGeneratedConversion covers the other ingress:
// the *FromGenerated converters, which read the generated client's *time.Time
// and previously flattened it through deref(). Notification has no converter —
// it is decoded straight from the response body — so it is absent here and
// covered by the wire-decode cases above.
func TestOptionalTimestampsSurviveGeneratedConversion(t *testing.T) {
	cases := []struct {
		name string
		from any
		keys []string
	}{
		{"HillChart", hillChartFromGenerated(generated.HillChart{Enabled: true}), []string{"updated_at"}},
		{"SearchResult", searchResultFromGenerated(generated.SearchResult{}), []string{"created_at", "updated_at"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := marshalToKeys(t, c.from)
			for _, k := range c.keys {
				if raw, ok := got[k]; ok {
					t.Errorf("%s.%s: generated value carried a nil timestamp but the wrapper emitted %s", c.name, k, raw)
				}
			}
		})
	}
}

// TestNilOptionalTimestampPanicsOnValueReceiverCall pins the migration hazard
// that made #562 worth its own PR, as executable documentation.
//
// `hc.UpdatedAt.IsZero()` is the idiomatic pre-fix guard, and it compiles
// UNCHANGED after the field becomes a pointer — Go inserts the dereference for
// a value-receiver method call. So the break is invisible to the compiler and
// surfaces as a nil-pointer panic at runtime. Callers must nil-check first;
// see the migration note on the PR.
//
// Against the un-fixed tree this test fails ("expected a nil-pointer panic"),
// which is exactly the point: nothing panicked, and nothing complained.
func TestNilOptionalTimestampPanicsOnValueReceiverCall(t *testing.T) {
	hc := hillChartFromGenerated(generated.HillChart{Enabled: true})

	defer func() {
		if recover() == nil {
			t.Error("expected a nil-pointer panic from IsZero() on an absent UpdatedAt: " +
				"if this line is reached the field is still value-typed and absence reads as 0001-01-01T00:00:00Z")
		}
	}()

	_ = hc.UpdatedAt.IsZero()
}

// TestNoValueTypedOptionalTimestamps is the regression guard.
//
// scripts/check-go-optional-pointers enforces the same invariant, but only
// over go/pkg/generated/client.gen.go — the hand-written wrapper surface is
// outside its scope by construction, and deliberately so: this package
// flattens optional strings/bools/numerics to value types on purpose, and
// ~300 fields would trip that guard's classifier for no benefit. Timestamps
// are the carve-out, because `,omitempty` cannot save a struct.
//
// Scope, stated honestly rather than implied: this checks value-typed
// time.Time fields carrying `,omitempty`. A value-typed time.Time with NO
// omitempty whose generated counterpart is a pointer — the shape
// SearchResult.CreatedAt had — is NOT detectable from this package's source
// alone; catching that class needs a (wrapper, generated) pairing.
//
// That gap is now closed by TestNoWrapperTimestampNarrowerThanGenerated below,
// which reads both sides and pairs them by struct name + json key (#620). This
// guard stays: it catches an inert `,omitempty` on a wrapper struct that has no
// generated counterpart at all, which the cross-reference cannot see.
func TestNoValueTypedOptionalTimestamps(t *testing.T) {
	wrappers := timeWrapperNames(t,
		parseGoFiles(t, packageGoFiles(t, ".")),
		parseGoFiles(t, packageGoFiles(t, filepath.Join("..", "types"))))

	fset := token.NewFileSet()
	paths := packageGoFiles(t, ".")
	files := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, f)
	}

	var (
		violations []string
		scanned    int
	)
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, f := range st.Fields.List {
				if !isTimeLike(f.Type, wrappers) {
					continue
				}
				scanned++
				if f.Tag == nil {
					continue
				}
				tag, err := strconv.Unquote(f.Tag.Value)
				if err != nil {
					t.Errorf("unparsable struct tag at %s: %s", fset.Position(f.Tag.Pos()), f.Tag.Value)
					continue
				}
				if !hasOmitempty(reflect.StructTag(tag)) {
					continue
				}
				violations = append(violations,
					fmt.Sprintf("%s: %s %s `%s`", fset.Position(f.Pos()), fieldNames(f), timeTypeName(f.Type), tag))
			}
			return true
		})
	}

	// A guard that scans nothing reports success forever. The wrapper surface
	// carries many required value-typed timestamps (Todo.CreatedAt and the
	// like); if none were seen, the walk broke, not the surface.
	if scanned == 0 {
		t.Fatal("scanned zero value-typed time.Time fields — the AST walk is broken, not the package")
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("optional timestamp is value-typed and cannot represent absence "+
			"(`,omitempty` is inert for a struct, so the key is emitted on every re-marshal — "+
			"as a fabricated 0001-01-01T00:00:00Z for time.Time, or at best null for a named wrapper): %s", v)
	}
}

// timestampField is one (struct, json key) timestamp declaration, recorded from
// either the wrapper surface or the generated client.
type timestampField struct {
	pointer bool
	field   string
	file    string
}

// fatalReporter is the slice of *testing.T the AST walk below uses: enough to
// report an embed it cannot resolve, and narrow enough that
// TestCollectTimestampFieldsReportsUnresolvableEmbed can hand it a recorder in
// place of a real T and observe the report instead of dying of it. Without the
// seam the #722 branch is untestable from inside the suite it aborts, and an
// untested fatal is one a later `continue` can delete in silence.
type fatalReporter interface {
	Helper()
	Fatalf(format string, args ...any)
}

// collectTimestampFields keys every timestamp-typed struct field by
// (struct name, json key) — time.Time and the named time wrappers (FlexTime,
// types.FlexibleTime, types.Date), in both value and pointer form. The json
// key is the pairing axis rather than the Go field name: the wrapper renames
// fields (Id → ID) but the wire key is the thing both sides must agree about.
//
// An anonymous embedded field promotes its own JSON keys into the enclosing
// struct, which moves them out from under this pairing. This walk does not
// judge whether a given embed does that; it asks whether a human has cleared
// that exact embed, and fails otherwise (assertEmbedIsAllowed, #722).
func collectTimestampFields(t fatalReporter, files map[string]*ast.File, wrappers map[string]bool) map[[2]string]timestampField {
	t.Helper()
	out := map[[2]string]timestampField{}
	for path, file := range files {
		for _, ts := range packageTypeSpecs(file) {
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, f := range st.Fields.List {
				if len(f.Names) == 0 {
					assertEmbedIsAllowed(t, path, ts.Name.Name, f)
					continue
				}
				key := jsonKey(f)
				if key == "" || key == "-" {
					continue
				}
				value := isTimeLike(f.Type, wrappers)
				star, isStar := f.Type.(*ast.StarExpr)
				pointer := isStar && isTimeLike(star.X, wrappers)
				if !value && !pointer {
					continue
				}
				out[[2]string{ts.Name.Name, key}] = timestampField{
					pointer: pointer,
					field:   f.Names[0].Name,
					file:    filepath.Base(path),
				}
			}
		}
	}
	return out
}

// packageTypeSpecs returns a file's PACKAGE-LEVEL type declarations, and only
// those. A type declared inside a function body is reachable by ast.Inspect
// but is never what a struct field resolves to, and this walk keys everything
// — the (struct, json key) pairing and the embed lookup below — by bare type
// name. A function-local `type Todo struct{…}` walked as if it were the
// package's would overwrite the real Todo's entries with a declaration Go
// never consults. There are none in the walked sources today; this keeps it
// that way by construction rather than by luck.
func packageTypeSpecs(file *ast.File) []*ast.TypeSpec {
	var out []*ast.TypeSpec
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				out = append(out, ts)
			}
		}
	}
	return out
}

// jsonKey returns the json name written on a NAMED field's tag, or "" when
// there is no usable one. It is the pairing axis for the two sides, and
// nothing more: it decides no question about promotion, embedding or wire
// semantics, all of which now belong to allowedEmbeds. A key this cannot read
// means the field is not paired — the same blind spot an untagged field has
// always had, and it cannot hide a promoted timestamp, because nothing
// anonymous reaches here.
func jsonKey(f *ast.Field) string {
	if f.Tag == nil {
		return ""
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return ""
	}
	tag, ok := reflect.StructTag(raw).Lookup("json")
	if !ok {
		return ""
	}
	return strings.Split(tag, ",")[0]
}

// allowedEmbeds is the ENTIRE exemption surface of the embed guard: the
// anonymous embeds in the walked sources that a human has read and cleared.
// Every other anonymous field fails the test, whatever it is embedding and
// whatever tag it carries.
//
// # Why an allowlist and not an analysis
//
// The four rounds of review before this one each found a real hole in a
// classifier that tried to decide, from the AST, whether a given embed could
// promote a timestamp. The holes were not careless — they were encoding/json
// semantics: only the exact tag `"-"` skips a field, so `json:"-,omitempty"`
// is an ordinary field named `-`; a tagged embed still PROMOTES the embedded
// type's MarshalJSON, so a struct embedding FlexTime under a json:"created_at"
// tag marshals as a bare scalar and has no created_at key at all; a selector's
// final name says nothing about which package it came from, so an unrelated
// audit.Date read as the local Date wrapper. Each fix was correct and the next
// round found the next one, which is the signal to reassess the instrument
// rather than write another rule.
//
// So the modelling is gone. This guard now knows NOTHING about encoding/json —
// not tag syntax, not the `-` sentinel, not promotion, not method sets, not
// shadowing. It compares four strings. Everything it cannot match, it
// reports, and a human decides once and records the decision here.
//
// # What the matcher compares
//
// The key is (file base name, enclosing struct name, embedded type, raw
// struct tag) — filepath.Base of the parsed path, the TypeSpec's name, the
// embedded type expression rendered by go/types.ExprString (star, package
// qualifier and any type arguments included, so `Base[int]` and `Base[string]`
// are different keys), and the tag literal exactly as it appears in the
// source, "" when there is none. That is the COMPLETE syntax of an anonymous
// field; nothing outside the field's own line enters the key, and nothing on
// it is left out. All four are syntax, deliberately: an entry records that
// somebody read THAT line in THAT struct in THAT file, not that a type
// resolves some way. Renaming the struct, moving it to another file, changing
// time.Time to *time.Time, or adding or removing a tag all invalidate the
// entry and re-open the question, which is the intended sensitivity. The tag
// is in the key for the same reason the star is: the reading that cleared an
// embed may have depended on it, and the guard has no way to know whether it
// did without modelling the semantics it refuses to model — so it re-asks.
//
// What the key does NOT carry, and will not: anything about the embedded
// type's own declaration. An allowlisted type that later grows a timestamp
// field keeps its key, and the clearance stands until someone re-reads it.
// That is the residual every allowlist has, and it is accepted here on
// purpose: re-verifying the declaration IS the analysis this guard replaced,
// and the reader that did it was retired after four rounds of holes. Today
// neither entry embeds a struct this package declares — one is time.Time, the
// other an interface — so no walked declaration can grow a field under it.
//
// # Adding an entry
//
// A new embed is EXPECTED to fail this suite until someone reads it. Before
// adding it here, establish that it promotes no JSON key that belongs in the
// (struct, json key) pairing — and if it does promote one, do not add it:
// teach the pairing about it, or drop the embed. TestAllowedEmbedsMatchesCorpus
// keeps this list and the corpus in exact correspondence in both directions,
// so a stale entry fails just as loudly as an unreviewed embed.
var allowedEmbeds = map[[4]string]string{
	{"authorization.go", "FlexTime", "time.Time", ""}: "time.Time's own fields are unexported and it " +
		"marshals through its own MarshalJSON, so this embed contributes no JSON key to FlexTime",
	{"client.gen.go", "ClientWithResponses", "ClientInterface", ""}: "an interface has no fields to " +
		"promote, and ClientWithResponses is an HTTP client that is never marshaled",
}

// assertEmbedIsAllowed fails unless this exact anonymous embed is on
// allowedEmbeds. It resolves nothing and infers nothing; see that variable for
// why the guard is shaped this way.
func assertEmbedIsAllowed(t fatalReporter, path, owner string, f *ast.Field) {
	t.Helper()
	key := embedKey(path, owner, f)
	if _, ok := allowedEmbeds[key]; ok {
		return
	}
	t.Fatalf("%s: %s embeds %s, which nobody has cleared. encoding/json promotes an "+
		"embedded type's JSON-tagged fields into %s, so a timestamp among them would sit "+
		"outside the (struct, json key) pairing below and its value/pointer parity would go "+
		"unchecked — silently, which is why this is fatal rather than skipped (#722). This "+
		"guard deliberately models no encoding/json semantics: read the embed, and if it "+
		"promotes no key this pairing needs, add {%q, %q, %q, %q} to allowedEmbeds with the "+
		"reason",
		path, owner, key[2], owner,
		key[0], key[1], key[2], key[3])
}

// embedKey builds the allowlist key for one anonymous field: the complete
// syntax of that field, rendered, and nothing resolved. See allowedEmbeds.
func embedKey(path, owner string, f *ast.Field) [4]string {
	tag := ""
	if f.Tag != nil {
		tag = f.Tag.Value
	}
	return [4]string{filepath.Base(path), owner, gotypes.ExprString(f.Type), tag}
}

// TestEmbedKeyIsTheCompleteFieldSyntax pins the identity contract of the key:
// two anonymous fields that differ anywhere on their own line get different
// keys. An earlier renderer collapsed every generic embed to one shared
// fallback string and read no tag at all, so clearing one `Base[int]` embed
// cleared every `Base[T]` beside it and a tag could come or go without the
// entry noticing. No case here is about what any of these shapes MEANS to
// encoding/json — only that the key tells them apart.
func TestEmbedKeyIsTheCompleteFieldSyntax(t *testing.T) {
	src := "package p\n\ntype W struct {\n" +
		"\tBase[int]\n\tBase[string]\n\tpkg.Base[A, B]\n\t*Base[int]\n" +
		"\tAudit\n\tAudit `json:\"audit\"`\n\tAudit `json:\"-\"`\n}\n"
	file, err := parser.ParseFile(token.NewFileSet(), "w.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	st := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)

	seen := map[[4]string]int{}
	for _, f := range st.Fields.List {
		seen[embedKey("w.go", "W", f)]++
	}
	if len(seen) != len(st.Fields.List) {
		for key, n := range seen {
			if n > 1 {
				t.Errorf("%d distinct embeds share the key %q — clearing one would clear them all", n, key)
			}
		}
	}
	if _, ok := seen[[4]string{"w.go", "W", "pkg.Base[A, B]", ""}]; !ok {
		t.Errorf("a qualified generic embed was not rendered as written; keys: %v", seen)
	}
	if _, ok := seen[[4]string{"w.go", "W", "Audit", "`json:\"audit\"`"}]; !ok {
		t.Errorf("the raw tag literal is not part of the key; keys: %v", seen)
	}
}

// embedRecorder stands in for *testing.T so the meta-tests below can watch
// collectTimestampFields report rather than be killed by the report. Fatalf
// must not return — the walk it interrupts is written on the assumption that
// it doesn't — so it panics with a sentinel the runner recovers, standing in
// for testing's own runtime.Goexit.
type embedRecorder struct {
	fatal   bool
	message string
}

type recordedFatal struct{}

func (r *embedRecorder) Helper() {}

func (r *embedRecorder) Fatalf(format string, args ...any) {
	r.fatal = true
	r.message = fmt.Sprintf(format, args...)
	panic(recordedFatal{})
}

// collectFromSource runs collectTimestampFields over one synthetic file and
// returns both what it reported and what it collected. The collected map is
// nil when a report fired, since the walk is aborted exactly as a real
// t.Fatalf aborts it.
func collectFromSource(t *testing.T, src string, wrappers map[string]bool) (*embedRecorder, map[[2]string]timestampField) {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "synthetic.go", src, 0)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}
	rec := &embedRecorder{}
	var collected map[[2]string]timestampField
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(recordedFatal); !ok {
					panic(r)
				}
			}
		}()
		collected = collectTimestampFields(rec, map[string]*ast.File{"synthetic.go": file}, wrappers)
	}()
	return rec, collected
}

// TestCollectTimestampFieldsReportsUnallowedEmbed is the committed proof of the
// #722 branch. The corpus reaches only the ACCEPTING side of the guard — every
// embed it contains is on allowedEmbeds — so the reporting side, the half that
// closes the under-report, would otherwise ship with nothing holding it and a
// revert to `continue` would leave the suite green. That is the vacuity the
// guards at the ends of TestNoValueTypedOptionalTimestamps and
// TestNoWrapperTimestampNarrowerThanGenerated refuse, one level up: those
// assert the walk saw something, this asserts it still objects to something.
//
// The cases are the shapes four rounds of review found holes in. They are here
// as a record that the answer no longer depends on telling them apart: none is
// on the allowlist, so each one fatals for the same reason, and no reading of
// encoding/json is involved in getting them right.
func TestCollectTimestampFieldsReportsUnallowedEmbed(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			// The plain #722 shape: an untagged embed whose type carries a
			// timestamp, promoted onto Wrapper and therefore invisible to a
			// pairing keyed by (struct, json key).
			name: "untagged embed",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit\n\tID string `json:\"id\"`\n}\n",
		},
		{
			// A json name that looks ordinary. The old classifier called this a
			// non-promoting field and collected it; whether that reading was
			// right no longer matters, because a tag buys no exemption.
			name: "embed under an ordinary json name",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit `json:\"audit\"`\n}\n",
		},
		{
			// `json:"-,omitempty"` is NOT the skip sentinel — only the exact tag
			// `"-"` is — so the old key-only check dropped a field encoding/json
			// keeps. The guard no longer reads the tag at all.
			name: "embed tagged with the near-miss sentinel",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit `json:\"-,omitempty\"`\n}\n",
		},
		{
			// Even the exact sentinel. encoding/json does drop this field, so
			// the old skip was defensible — but it was a semantic judgement,
			// and those are what this guard no longer makes.
			name: "embed tagged with the exact sentinel",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit `json:\"-\"`\n}\n",
		},
		{
			// A name the library refuses resets to empty and promotes. Also no
			// longer a question this guard asks.
			name: "embed tagged with a name encoding/json rejects",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit `json:\"bad\\\\name\"`\n}\n",
		},
		{
			// A TIME WRAPPER embed under a tag. Embedding promotes the type's
			// MarshalJSON to Wrapper, so this marshals as a scalar and has no
			// created_at key — the shape the old isTimeLike arm waved through.
			name: "tagged embed of a time wrapper",
			src: "package p\n\n" +
				"type Wrapper struct {\n\tFlexTime `json:\"created_at\"`\n}\n",
		},
		{
			// An unqualified time wrapper, untagged: exempted before by name
			// alone, whatever package it came from.
			name: "untagged embed of a time wrapper",
			src:  "package p\n\ntype Wrapper struct {\n\tFlexTime\n}\n",
		},
		{
			// A qualified type whose final name collides with a wrapper. The
			// old exemption matched the selector's last segment, so an
			// unrelated external Date passed as the local one.
			name: "embed of an external type whose final name matches a wrapper",
			src:  "package p\n\ntype Wrapper struct {\n\taudit.Date\n}\n",
		},
		{
			// An interface embed, exempted before because a non-struct promotes
			// no fields. True, and still not this guard's call to make.
			name: "embed of a locally declared interface",
			src: "package p\n\ntype Reader interface{ Read() }\n\n" +
				"type Wrapper struct {\n\tReader\n}\n",
		},
		{
			// The type Go resolves in Wrapper is the PACKAGE-LEVEL Audit; a
			// function-local declaration of the same name used to vouch for it.
			name: "function-local type shadowing the embedded name",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit\n}\n\n" +
				"func shadow() {\n\ttype Audit interface{ Foo() }\n\tvar _ Audit\n}\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The wrapper set is deliberately populated: it is what the old
			// classifier consulted to exempt an embed, and it must now buy
			// nothing.
			rec, collected := collectFromSource(t, c.src, map[string]bool{"FlexTime": true, "Date": true})
			if !rec.fatal {
				t.Fatalf("an embed outside allowedEmbeds was walked without a report, collecting "+
					"%d field(s) instead. Whatever this one promotes, nobody has said so in "+
					"allowedEmbeds, and a silent skip is the #722 under-report: a promoted "+
					"timestamp leaves the (struct, json key) pairing and its value/pointer "+
					"parity is never asserted", len(collected))
			}
			if !strings.Contains(rec.message, "Wrapper") {
				t.Errorf("report does not name the embedding struct, so it cannot be acted on: %s", rec.message)
			}
			if !strings.Contains(rec.message, "allowedEmbeds") {
				t.Errorf("report does not say where the decision is recorded: %s", rec.message)
			}
		})
	}
}

// TestCollectTimestampFieldsIgnoresFunctionLocalTypes pins the other half of
// the package-level rule. The lookup that vouches for an embed is not the only
// place a bare type name is resolved — the pairing map is keyed by one too, so
// a function-local declaration walked as if it were the package's overwrites
// the real entry and the parity assertion then judges a type no field has.
func TestCollectTimestampFieldsIgnoresFunctionLocalTypes(t *testing.T) {
	src := "package p\n\nimport \"time\"\n\n" +
		"type Todo struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
		"func helper() {\n\ttype Todo struct {\n\t\tCreatedAt time.Time `json:\"created_at\"`\n\t}\n\tvar _ Todo\n}\n"

	rec, collected := collectFromSource(t, src, map[string]bool{})
	if rec.fatal {
		t.Fatalf("unexpected report: %s", rec.message)
	}
	got, ok := collected[[2]string{"Todo", "created_at"}]
	if !ok {
		t.Fatal("package-level Todo.created_at was not collected at all")
	}
	if !got.pointer {
		t.Error("the function-local Todo overwrote the package-level one, which is the " +
			"declaration Go actually resolves — its value-typed CreatedAt would be " +
			"compared against the generated schema in place of the real field")
	}
}

// TestAllowedEmbedsMatchesCorpus holds the allowlist and the walked sources in
// exact correspondence, in BOTH directions, over the same files the guard
// walks.
//
// Forward: an anonymous embed nobody has cleared fails here as well as in the
// guard, with a message that says what to read.
//
// Backward, and this is the half an allowlist usually lacks: an entry no
// corpus embed matches fails too. Without it a stale exemption survives the
// embed it was written for — the struct is renamed, the file split, the type
// changed to a pointer — and sits there ready to excuse a future embed that
// happens to land on the same four strings. It also makes every entry
// self-evidently non-vacuous: each one is matched by a real line today, or
// this test is red.
func TestAllowedEmbedsMatchesCorpus(t *testing.T) {
	files := parseGoFiles(t, packageGoFiles(t, "."))
	for path, file := range parseGoFiles(t, []string{filepath.Join("..", "generated", "client.gen.go")}) {
		files[path] = file
	}

	found := map[[4]string]string{}
	for path, file := range files {
		for _, ts := range packageTypeSpecs(file) {
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, f := range st.Fields.List {
				if len(f.Names) == 0 {
					found[embedKey(path, ts.Name.Name, f)] = path
				}
			}
		}
	}

	// A corpus with no embeds at all would make the allowlist trivially
	// satisfiable and this test vacuous in the direction that matters.
	if len(found) == 0 {
		t.Fatal("found zero anonymous embeds in the walked sources — the enumeration is " +
			"broken, not the corpus (FlexTime and ClientWithResponses both embed)")
	}

	for key, path := range found {
		if _, ok := allowedEmbeds[key]; !ok {
			t.Errorf("%s: %s embeds %s and is not on allowedEmbeds. Read it: if it promotes no "+
				"JSON key the (struct, json key) pairing needs, add {%q, %q, %q, %q} with the reason; "+
				"if it does promote one, the pairing has to learn about it instead",
				path, key[1], key[2], key[0], key[1], key[2], key[3])
		}
	}
	for key, why := range allowedEmbeds {
		if _, ok := found[key]; !ok {
			t.Errorf("allowedEmbeds carries {%q, %q, %q, %q} (%q) but no embed in the walked sources "+
				"matches it. A stale exemption excuses whatever next lands on those four strings — "+
				"delete it, or correct it to the embed it was meant for", key[0], key[1], key[2], key[3], why)
		}
	}
}

func parseGoFiles(t *testing.T, paths []string) map[string]*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string]*ast.File{}
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		out[p] = f
	}
	return out
}

// TestNoWrapperTimestampNarrowerThanGenerated closes the class #620 named.
//
// A wrapper field typed value time.Time whose generated counterpart is
// *time.Time cannot represent absence, and — this is what makes it invisible —
// NOTHING IN THE WRAPPER SOURCE SAYS SO. The evidence lives in the generated
// client, so the guard has to read both sides.
//
// Why the two neighbouring gates cannot do this:
//
//   - TestNoValueTypedOptionalTimestamps (above) keys on `,omitempty` and
//     `continue`s without it. All five #620 fields were tagged bare
//     `json:"created_at"`, so they were invisible to it. Its own doc comment
//     named this gap and deferred it; this is the close.
//   - scripts/check-wrapper-drift has the (wrapper, generated) pairing but
//     compares tag NAMES only. Teaching it types would still miss
//     WebhookDelivery, which its header excludes BY DESIGN as a parallel
//     webhook-flavored shape — and WebhookDelivery.CreatedAt is one of the five.
//
// So the pairing here is by struct name + json key, which is blind to the
// converter tiers and therefore covers all of them.
//
// Both this guard and TestNoValueTypedOptionalTimestamps originally keyed on
// the literal `time.Time` selector, so the named time wrappers (FlexTime,
// types.FlexibleTime, types.Date) were invisible to them — which is how
// TimelineEventData.StartsAt/EndsAt sat value-typed against a pointer-typed
// generated counterpart with neither guard able to see it (#662). The
// predicate is now isTimeLike, which resolves the wrappers by final type name
// on both sides of the pairing.
//
// Scoped to timestamps deliberately. The wrapper flattens optional
// *string/*bool/*int to value types on purpose; a nil-capability rule applied
// broadly reports ~345 intentional fields. Timestamps are the principled
// carve-out: an absent string marshals away harmlessly, an absent time.Time
// marshals as a wrong value.
func TestNoWrapperTimestampNarrowerThanGenerated(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	var wrapperPaths []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		wrapperPaths = append(wrapperPaths, name)
	}

	wrapperFiles := parseGoFiles(t, wrapperPaths)
	wrappers := timeWrapperNames(t, wrapperFiles,
		parseGoFiles(t, packageGoFiles(t, filepath.Join("..", "types"))))

	wrapper := collectTimestampFields(t, wrapperFiles, wrappers)
	gen := collectTimestampFields(t, parseGoFiles(t, []string{
		filepath.Join("..", "generated", "client.gen.go"),
	}), wrappers)

	if len(wrapper) == 0 || len(gen) == 0 {
		t.Fatalf("collected %d wrapper and %d generated timestamp fields — the walk is broken, not the surface",
			len(wrapper), len(gen))
	}

	var (
		violations []string
		compared   int
	)
	for key, w := range wrapper {
		g, ok := gen[key]
		if !ok {
			continue
		}
		compared++
		if w.pointer || !g.pointer {
			continue
		}
		violations = append(violations, fmt.Sprintf(
			"%s.%s (json:%q, %s) is value-typed but generated.%s.%s is a pointer",
			key[0], w.field, key[1], w.file, key[0], g.field))
	}

	// A name-keyed join that matches nothing passes vacuously.
	if compared == 0 {
		t.Fatal("zero (wrapper, generated) timestamp fields paired — the join key is wrong, not the surface")
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("wrapper timestamp is narrower than the schema it decodes: %s. "+
			"An omitted value decodes to the zero time and re-marshals as a fabricated "+
			"0001-01-01T00:00:00Z, indistinguishable from a real timestamp", v)
	}
}

// TestNamedTimeWrappersMarshalZeroAsNull is the behavioral root guard for the
// class #662 named. AuthorizationInfo.ExpiresAt is exactly the field the two
// AST guards above can never reach — it has no generated counterpart to pair
// with and a bare `json:"expires_at"` tag, so it is structurally invisible to
// both, widened or not. What actually protects it, and every future field like
// it, is the wrappers' own marshaling contract: a named time wrapper's zero
// value means "the wire didn't state one" and must marshal as null, never as a
// fabricated instant (0001-01-01T00:00:00Z, or a 1970 date via the epoch).
//
// The table is cross-checked against the same wrapper discovery the AST guards
// use, so a new embedded-time.Time wrapper type cannot ship without a row here.
func TestNamedTimeWrappersMarshalZeroAsNull(t *testing.T) {
	cases := []struct {
		name string
		zero any
	}{
		{"FlexTime", FlexTime{}},
		{"FlexibleTime", types.FlexibleTime{}},
		{"Date", types.Date{}},
	}

	wrappers := timeWrapperNames(t,
		parseGoFiles(t, packageGoFiles(t, ".")),
		parseGoFiles(t, packageGoFiles(t, filepath.Join("..", "types"))))
	covered := map[string]bool{}
	for _, c := range cases {
		covered[c.name] = true
	}
	for name := range wrappers {
		if !covered[name] {
			t.Errorf("time wrapper %s has no zero-marshals-as-null case — add its zero value to this table", name)
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := json.Marshal(c.zero)
			if err != nil {
				t.Fatalf("marshal zero %s: %v", c.name, err)
			}
			if string(b) != "null" {
				t.Errorf("zero %s marshaled as %s, want null — a zero named time wrapper means "+
					"\"absent on the wire\" and must not re-marshal as a fabricated instant", c.name, b)
			}
		})
	}
}

// isValueTime reports whether expr is the type `time.Time` exactly — not
// *time.Time, not []time.Time, not a named alias.
func isValueTime(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Time" {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == "time"
}

// packageGoFiles returns the non-test .go files in dir.
func packageGoFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var paths []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		paths = append(paths, filepath.Join(dir, name))
	}
	if len(paths) == 0 {
		t.Fatalf("found zero non-test .go files in %s — the guard is scanning the wrong directory", dir)
	}
	return paths
}

// timeWrapperNames discovers the named time-wrapper types of this surface:
// every struct type in the given files that embeds a value time.Time. Today
// that is FlexTime (this package) and types.FlexibleTime. types.Date is added
// explicitly: it does not embed a time.Time — it stores year/month/day ints —
// but it occupies the same optionality class (zero value means "unset",
// marshals as null), so a value-typed Date field is just as much a timestamp
// declaration as the embedded-time wrappers are.
func timeWrapperNames(t *testing.T, fileSets ...map[string]*ast.File) map[string]bool {
	t.Helper()
	out := map[string]bool{"Date": true}
	discovered := 0
	for _, files := range fileSets {
		for _, file := range files {
			// Package-level declarations only. A function-local
			// `type Audit struct { time.Time }` would otherwise put Audit in
			// this set, and every field typed by the PACKAGE-level Audit would
			// then read as a timestamp — the same shadowing hole the embed
			// lookup had, in the one place that still resolves a bare name.
			for _, ts := range packageTypeSpecs(file) {
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, f := range st.Fields.List {
					if len(f.Names) == 0 && isValueTime(f.Type) {
						out[ts.Name.Name] = true
						discovered++
					}
				}
			}
		}
	}
	// A discovery that finds nothing reports an empty wrapper set forever.
	// FlexTime and types.FlexibleTime both embed a time.Time; if neither was
	// seen, the walk broke, not the surface.
	if discovered == 0 {
		t.Fatal("discovered zero embedded-time.Time wrapper types — the walk is broken, not the surface")
	}
	return out
}

// isTimeLike reports whether expr is a value-typed timestamp declaration:
// `time.Time` itself, or one of the named time wrappers by final type name —
// which matches both the same-package spelling (`FlexTime`) and the selector
// spelling (`types.FlexibleTime`), so the generated client's fields resolve
// through the same predicate as the wrapper surface's.
func isTimeLike(expr ast.Expr, wrappers map[string]bool) bool {
	if isValueTime(expr) {
		return true
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return wrappers[e.Name]
	case *ast.SelectorExpr:
		if _, ok := e.X.(*ast.Ident); ok {
			return wrappers[e.Sel.Name]
		}
	}
	return false
}

// timeTypeName renders expr for a violation message.
func timeTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
	}
	return "time.Time"
}

// hasOmitempty reports whether any tag key carries the omitempty option. Every
// key is inspected, not just json: a field tagged form:"x,omitempty" is just as
// optional, and only checking json would exempt it.
func hasOmitempty(tag reflect.StructTag) bool {
	for _, key := range []string{"json", "form", "url", "query", "xml", "yaml"} {
		v, ok := tag.Lookup(key)
		if !ok {
			continue
		}
		parts := strings.Split(v, ",")
		for _, opt := range parts[1:] {
			if opt == "omitempty" {
				return true
			}
		}
	}
	return false
}

func fieldNames(f *ast.Field) string {
	if len(f.Names) == 0 {
		return "<embedded>"
	}
	names := make([]string, 0, len(f.Names))
	for _, n := range f.Names {
		names = append(names, n.Name)
	}
	return strings.Join(names, ", ")
}
