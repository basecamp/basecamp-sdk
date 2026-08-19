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
// An anonymous embedded field usually carries neither a name nor a tag, and
// promotes its own JSON keys into the enclosing struct. This walk does not
// model promotion, so such an embed goes to assertEmbedPromotesNoTimestamps,
// which fails the test on anything that could promote a timestamp (#722). A
// TAGGED anonymous field is the other case and is not a promotion source at
// all: encoding/json treats it as an ordinary field under its json name, and
// so does this walk — the same reading scripts/check-wrapper-drift pins for
// the shape (see its header, "An anonymous field that carries its own
// json:"…" tag is NOT promoted").
func collectTimestampFields(t fatalReporter, files map[string]*ast.File, wrappers map[string]bool) map[[2]string]timestampField {
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
				key, tagged := jsonKey(f)
				if len(f.Names) == 0 {
					// encoding/json's three readings of an anonymous field:
					// `json:"-"` is dropped outright, a non-empty json name
					// makes it an ordinary field under that name (fall
					// through), and anything else — no tag, no json key, an
					// unparsable tag, or an empty name like `json:",omitempty"`
					// — promotes the embedded type's own fields.
					if tagged && key == "-" {
						continue
					}
					if key == "" {
						assertEmbedPromotesNoTimestamps(t, path, ts.Name.Name, f.Type, files, wrappers)
						continue
					}
				} else if !tagged || key == "" || key == "-" {
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
					field:   goFieldName(f),
					file:    filepath.Base(path),
				}
			}
			return true
		})
	}
	return out
}

// jsonKey returns a field's json name and whether it carries a readable json
// tag at all. A missing tag, an unparsable one, and one without a json key all
// read as untagged — which for an anonymous field means "promotion source",
// the reported side of the choice rather than the credited one.
func jsonKey(f *ast.Field) (string, bool) {
	if f.Tag == nil {
		return "", false
	}
	raw, err := strconv.Unquote(f.Tag.Value)
	if err != nil {
		return "", false
	}
	tag, ok := reflect.StructTag(raw).Lookup("json")
	if !ok {
		return "", false
	}
	return strings.Split(tag, ",")[0], true
}

// goFieldName is the Go identifier a field is reported under. A tagged
// anonymous field has no declared name, so — as encoding/json and
// scripts/check-wrapper-drift both do — it is named by its embedded type, with
// any pointer and package qualifier stripped (`*types.Date` is the field
// `Date`).
func goFieldName(f *ast.Field) string {
	if len(f.Names) > 0 {
		return f.Names[0].Name
	}
	expr := f.Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return embeddedTypeName(f.Type)
}

// assertEmbedPromotesNoTimestamps fails unless an anonymous embedded field is
// one of the shapes that promote no JSON-tagged fields at all.
//
// #722: this walk used to drop an anonymous embed with the same `continue`
// that drops an untagged field — an embed has no Names AND no Tag, so both
// halves of that one guard matched it. Every timestamp the embed promotes then
// fell out of the (struct name, json key) pairing, and the value/pointer
// parity assertion simply never ran on it. That is the UNDER-report direction:
// the suite stays green while the exact class this file exists to catch ships
// unremarked. #721's invariant for the sibling gate holds here too — an embed
// the walker cannot resolve is REPORTED, never skipped.
//
// What this deliberately is NOT, recorded here rather than in a PR nobody
// re-reads: the promotion walk scripts/check-wrapper-drift carries
// (shallowest-wins, same-depth annihilation, transitive embeds, pointer
// embeds, cycle termination). Nothing in this walk's input promotes anything —
// the only anonymous embeds in go/pkg/basecamp and in client.gen.go are the
// two recognised below — so that walk would today be several hundred lines
// certifying an empty set, and #741 inventories the shapes such a walk gets
// confidently wrong. Failing loudly here closes the dangerous direction at
// zero risk and leaves the walk to whoever first needs it, who will have a
// real embed to test it against.
func assertEmbedPromotesNoTimestamps(t fatalReporter, path, owner string, expr ast.Expr, files map[string]*ast.File, wrappers map[string]bool) {
	t.Helper()

	// A time wrapper embedding a value time.Time (FlexTime here,
	// types.FlexibleTime one package over): time.Time's own fields are
	// unexported and it marshals through MarshalJSON, so it promotes nothing.
	if isTimeLike(expr, wrappers) {
		return
	}
	// A name these sources declare as an interface, map, slice, func or chan —
	// today generated.ClientWithResponses embedding ClientInterface. Same
	// carve-out scripts/check-wrapper-drift records, for the same reason: a
	// non-struct has no JSON-tagged fields to promote. A name resolving to
	// another NAME (`type Alias Base`) is not accepted here: it can reach a
	// struct, and a walk that credits it would be back to skipping silently.
	if ident, ok := expr.(*ast.Ident); ok && declaresNonPromotingType(files, ident.Name) {
		return
	}

	t.Fatalf("%s: %s embeds %s, which this walk cannot resolve. encoding/json promotes "+
		"the embedded type's JSON-tagged fields into %s, so any timestamp among them is "+
		"absent from the (struct, json key) pairing below and its value/pointer parity "+
		"goes unchecked — silently, which is why this is fatal rather than skipped. "+
		"Teach this walk encoding/json's promotion rules (scripts/check-wrapper-drift "+
		"implements them) before embedding here",
		path, owner, embeddedTypeName(expr), owner)
}

// declaresNonPromotingType reports whether name is declared in files as a type
// whose underlying type cannot carry a JSON-tagged field. Only the forms that
// are conclusively non-promoting count; anything else — including a name that
// resolves to another name — is left for the caller to report.
func declaresNonPromotingType(files map[string]*ast.File, name string) bool {
	found := false
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name {
				return true
			}
			switch ts.Type.(type) {
			case *ast.InterfaceType, *ast.MapType, *ast.ArrayType, *ast.FuncType, *ast.ChanType:
				found = true
			}
			return true
		})
	}
	return found
}

// embeddedTypeName renders an embedded field's type for a failure message.
func embeddedTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return "*" + embeddedTypeName(e.X)
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		if x, ok := e.X.(*ast.Ident); ok {
			return x.Name + "." + e.Sel.Name
		}
	}
	return "an unrecognised type expression"
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

// TestCollectTimestampFieldsReportsUnresolvableEmbed is the committed proof of
// the #722 branch. The two anonymous embeds in the real corpus (FlexTime
// embedding time.Time, generated.ClientWithResponses embedding ClientInterface)
// only ever reach assertEmbedPromotesNoTimestamps' two ACCEPTING arms, so the
// whole reporting path — the half that closes the under-report — would ship
// with nothing holding it, and reverting it to the `continue` it replaced would
// leave the suite green. That is the same vacuity the guards at the ends of
// TestNoValueTypedOptionalTimestamps and TestNoWrapperTimestampNarrowerThanGenerated
// exist to refuse, one level up: those assert the walk saw something, this
// asserts the walk still objects to something.
func TestCollectTimestampFieldsReportsUnresolvableEmbed(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			// The plain #722 shape: an untagged embed whose type carries a
			// timestamp. `created_at` is promoted onto Wrapper, so pairing
			// Wrapper by (struct, json key) never sees it.
			name: "untagged embed",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit\n\tID string `json:\"id\"`\n}\n",
		},
		{
			// A json tag with an EMPTY name does not stop promotion —
			// encoding/json explores the embed exactly as if untagged — so
			// this must reach the report too, not the ordinary-field path.
			name: "embed tagged with an empty json name",
			src: "package p\n\nimport \"time\"\n\n" +
				"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
				"type Wrapper struct {\n\tAudit `json:\",omitempty\"`\n}\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, collected := collectFromSource(t, c.src, map[string]bool{})
			if !rec.fatal {
				t.Fatalf("collectTimestampFields walked a struct embedding a timestamp-carrying "+
					"struct without reporting it, collecting %d field(s) instead. Promotion is "+
					"deliberately unmodelled here, so a silent skip is precisely the #722 "+
					"under-report: the promoted timestamp drops out of the (struct, json key) "+
					"pairing and its value/pointer parity is never asserted", len(collected))
			}
			for _, want := range []string{"Wrapper", "Audit"} {
				if !strings.Contains(rec.message, want) {
					t.Errorf("report does not name %s, so it cannot be acted on: %s", want, rec.message)
				}
			}
		})
	}
}

// TestCollectTimestampFieldsTreatsTaggedEmbedAsOrdinaryField pins the other
// side of the same branch. A tagged anonymous field is NOT a promotion source
// — encoding/json puts it on the wire under its own json name — so reporting
// it would be a false alarm against a shape that is safe, and the field itself
// still has to be collected with the parity the rest of the walk checks.
func TestCollectTimestampFieldsTreatsTaggedEmbedAsOrdinaryField(t *testing.T) {
	src := "package p\n\nimport \"time\"\n\n" +
		"type Audit struct {\n\tCreatedAt *time.Time `json:\"created_at\"`\n}\n\n" +
		"type Nested struct {\n\tAudit `json:\"audit\"`\n}\n\n" +
		"type Wrapper struct {\n\tFlexTime `json:\"created_at\"`\n}\n\n" +
		"type PointerWrapper struct {\n\t*FlexTime `json:\"updated_at\"`\n}\n\n" +
		"type Dropped struct {\n\tFlexTime `json:\"-\"`\n}\n"

	rec, collected := collectFromSource(t, src, map[string]bool{"FlexTime": true})
	if rec.fatal {
		t.Fatalf("a tagged embed was reported as an unresolvable promotion source, "+
			"but encoding/json treats it as an ordinary field under its json name "+
			"(scripts/check-wrapper-drift pins the same reading): %s", rec.message)
	}

	for _, want := range []struct {
		key     [2]string
		field   string
		pointer bool
	}{
		{key: [2]string{"Wrapper", "created_at"}, field: "FlexTime"},
		{key: [2]string{"PointerWrapper", "updated_at"}, field: "FlexTime", pointer: true},
	} {
		got, ok := collected[want.key]
		if !ok {
			t.Errorf("%s.%s was not collected — a tagged embed is a wire field and must be "+
				"paired like one", want.key[0], want.key[1])
			continue
		}
		if got.field != want.field {
			t.Errorf("%s.%s collected under Go field %q, want %q — a tagged embed's field name "+
				"is its embedded type", want.key[0], want.key[1], got.field, want.field)
		}
		if got.pointer != want.pointer {
			t.Errorf("%s.%s collected with pointer=%v, want %v", want.key[0], want.key[1], got.pointer, want.pointer)
		}
	}

	if _, ok := collected[[2]string{"Dropped", "-"}]; ok {
		t.Error("an embed tagged `json:\"-\"` was collected — it is not on the wire at all")
	}
	// Nested's tagged embed is an ordinary field of a non-timestamp type: it
	// belongs in neither the report nor the pairing. Audit.CreatedAt is Audit's
	// OWN field, nested under `audit` on Nested's wire, so no (Nested, …) key
	// may claim it.
	if _, ok := collected[[2]string{"Nested", "created_at"}]; ok {
		t.Error("a tagged embed's inner timestamp was paired against the EMBEDDING struct — " +
			"a tagged embed nests its fields under its own key, it does not promote them")
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
					if len(f.Names) == 0 && isValueTime(f.Type) {
						out[ts.Name.Name] = true
						discovered++
					}
				}
				return true
			})
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
