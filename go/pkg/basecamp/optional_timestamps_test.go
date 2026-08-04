package basecamp

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/basecamp/basecamp-sdk/go/pkg/generated"
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
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatal("parsed zero non-test .go files — the guard is scanning the wrong directory")
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
				if !isValueTime(f.Type) {
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
					fmt.Sprintf("%s: %s time.Time `%s`", fset.Position(f.Pos()), fieldNames(f), tag))
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
			"(`,omitempty` is inert for a struct, so the key is emitted as 0001-01-01T00:00:00Z): %s", v)
	}
}

// timestampField is one (struct, json key) timestamp declaration, recorded from
// either the wrapper surface or the generated client.
type timestampField struct {
	pointer bool
	field   string
	file    string
}

// collectTimestampFields keys every time.Time / *time.Time struct field by
// (struct name, json key). The json key is the pairing axis rather than the Go
// field name: the wrapper renames fields (Id → ID) but the wire key is the
// thing both sides must agree about.
func collectTimestampFields(t *testing.T, files map[string]*ast.File) map[[2]string]timestampField {
	t.Helper()
	out := map[[2]string]timestampField{}
	for path, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, f := range st.Fields.List {
				if f.Tag == nil || len(f.Names) == 0 {
					continue
				}
				value := isValueTime(f.Type)
				star, isStar := f.Type.(*ast.StarExpr)
				pointer := isStar && isValueTime(star.X)
				if !value && !pointer {
					continue
				}
				raw, err := strconv.Unquote(f.Tag.Value)
				if err != nil {
					continue
				}
				jsonTag, ok := reflect.StructTag(raw).Lookup("json")
				if !ok {
					continue
				}
				key := strings.Split(jsonTag, ",")[0]
				if key == "" || key == "-" {
					continue
				}
				out[[2]string{ts.Name.Name, key}] = timestampField{
					pointer: pointer,
					field:   f.Names[0].Name,
					file:    filepath.Base(path),
				}
			}
			return true
		})
	}
	return out
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

	wrapper := collectTimestampFields(t, parseGoFiles(t, wrapperPaths))
	gen := collectTimestampFields(t, parseGoFiles(t, []string{
		filepath.Join("..", "generated", "client.gen.go"),
	}))

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
			"%s.%s (json:%q, %s) is time.Time but generated.%s.%s is *time.Time",
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
