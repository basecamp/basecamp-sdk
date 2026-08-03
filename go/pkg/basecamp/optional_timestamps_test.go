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
// alone; catching that class needs the (wrapper, generated) pairing that
// scripts/check-wrapper-drift already computes for tag names but not for
// types. That gap is reported on the PR, not closed here.
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
