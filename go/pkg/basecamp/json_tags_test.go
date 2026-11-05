package basecamp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// Every exported struct the SDK hands callers as data is something they
// marshal — into caches, tool results, CLI --json output — so each exported
// field carries a snake_case json tag matching the API. An untagged field
// marshals under its Go name (BubbleUpsResult once came out as
// {"BubbleUps":…,"Meta":…}).
//
// The check parses the package source rather than reflecting over values,
// because Go can't enumerate a package's types at runtime; a new struct is
// covered the moment it's written.

// jsonTagExemptSuffixes name the kinds of exported struct that are not API
// data: caller-supplied inputs (…Options, …Filters, …Fields, …Config) and
// error values (…Error), none of which the SDK decodes from or encodes as an
// API shape.
var jsonTagExemptSuffixes = []string{"Options", "Filters", "Fields", "Config", "Error"}

// jsonTagExemptTypes are the remaining exported structs that aren't API data.
var jsonTagExemptTypes = map[string]string{
	"BearerAuth":          "auth strategy holding a token provider",
	"StaticTokenProvider": "credential holder; must never be marshaled",
	"FlexTime":            "embeds time.Time and implements its own JSON",
	"Response":            "raw HTTP response wrapper (body bytes, headers)",
	"DownloadResult":      "streams a file body (io.ReadCloser)",
	"OperationInfo":       "observability hook payload, not an API shape",
	"RequestInfo":         "observability hook payload, not an API shape",
	"RequestResult":       "observability hook payload, not an API shape",
	"RecordingRef":        "caller-supplied pointer to a recording (input)",
	"Match":               "local URL-parser result with no API counterpart",
}

var snakeCaseJSONName = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

func jsonTagExempt(typeName string) bool {
	if _, ok := jsonTagExemptTypes[typeName]; ok {
		return true
	}
	for _, s := range jsonTagExemptSuffixes {
		if strings.HasSuffix(typeName, s) {
			return true
		}
	}
	return false
}

func TestExportedStructFieldsHaveSnakeCaseJSONTags(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	structs, fields := 0, 0
	seen := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				return true
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}
			seen[ts.Name.Name] = true
			if jsonTagExempt(ts.Name.Name) {
				return true
			}
			structs++
			for _, f := range st.Fields.List {
				if len(f.Names) == 0 {
					continue // embedded: encoding/json flattens it into the parent
				}
				for _, name := range f.Names {
					if !name.IsExported() {
						continue
					}
					fields++
					tag := ""
					if f.Tag != nil {
						tag = reflect.StructTag(strings.Trim(f.Tag.Value, "`")).Get("json")
					}
					key, _, _ := strings.Cut(tag, ",")
					switch {
					case tag == "":
						t.Errorf("%s: %s.%s has no json tag", fset.Position(name.Pos()), ts.Name.Name, name.Name)
					case key != "-" && !snakeCaseJSONName.MatchString(key):
						t.Errorf("%s: %s.%s json name %q is not snake_case", fset.Position(name.Pos()), ts.Name.Name, name.Name, key)
					}
				}
			}
			return true
		})
	}
	for name := range jsonTagExemptTypes {
		if !seen[name] {
			t.Errorf("jsonTagExemptTypes lists %s, which is no longer an exported struct; drop it", name)
		}
	}
	t.Logf("checked %d fields across %d exported structs", fields, structs)
	if structs < 100 {
		t.Errorf("checked only %d structs; is the test running in the package directory?", structs)
	}
}
