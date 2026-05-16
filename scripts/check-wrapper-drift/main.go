// Command check-wrapper-drift performs a field-level drift check between the
// hand-written wrappers in go/pkg/basecamp/ and the generated types in
// go/pkg/generated/.
//
// Discovery
//
// The script walks (wrapper, generated) pairs in two ways:
//
//   1. By signature reading every `<lower>FromGenerated(g generated.X) Y`
//      function declaration in go/pkg/basecamp/*.go (non-test). The argument
//      type names the generated struct; the return type names the wrapper
//      struct. (`webhookPersonFromGenerated` is special-cased and excluded
//      from the *FromGenerated convention check below — it is a parallel
//      mapping for WebhookEventPerson, not a Person wrapper.)
//
//   2. By a small explicit `directDecodePairs` map covering wrappers that
//      decode straight from JSON bytes (Notification, NotificationsResult,
//      MyAssignment, Gauge, GaugeNeedle). These do not have *FromGenerated
//      declarations; the JSON tags on the wrapper struct fields are what the
//      JSON decoder uses to read the wire payload.
//
// Check
//
// For each pair, the script compares JSON tag names (not Go field names —
// shape-equivalent tag collisions like wrapper `URL string \`json:"url"\``
// vs generated `Url string \`json:"url"\`` are handled correctly because the
// match is keyed on the `json:"…"` tag value, e.g. "url"). For every JSON
// tag declared on the generated struct, the wrapper must either:
//
//   - declare a field with the same JSON tag, or
//   - carry an `// intentionally-omitted: <tag> - <reason>` marker (ASCII
//     hyphen, matching the repo's default comment convention) anywhere
//     inside the wrapper struct's definition block.
//
// If neither is present, the script reports drift and exits 1.
//
// The wrapper may declare additional fields not in the generated struct
// (e.g. SystemLabel on Person, BillableStatus on TimesheetEntry); these are
// not flagged.
//
// Nested struct checks terminate at one level: TodoBucket fields are not
// compared against Bucket wrapper fields recursively. Each (wrapper,
// generated) pair is walked independently. This means a wrapper missing a
// nested struct entirely (e.g. dropping `bucket`) would surface as a missing
// tag on the parent, while a partial nested copy (where the nested wrapper
// itself drifts) would surface only if that nested wrapper has its own pair
// in the map.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// directDecodePairs maps the wrapper struct name to the generated struct
// name for §C-2 wrappers that decode via json.Unmarshal on the (sometimes
// pre-normalized) raw body. Their *FromGenerated function does not exist;
// the wrapper struct's JSON tags are the contract.
var directDecodePairs = map[string]string{
	"Notification":        "Notification",
	"NotificationsResult": "GetMyNotificationsResponseContent",
	"MyAssignment":        "MyAssignment",
	"Gauge":               "Gauge",
	"GaugeNeedle":         "GaugeNeedle",
}

// excludedFromGenerated lists *FromGenerated functions whose argument type
// is not the structurally-aligned generated struct of their return type
// (e.g. webhookPersonFromGenerated maps generated.Person → WebhookEventPerson,
// which is a parallel webhook-flavored representation, not a Person wrapper).
// Such pairs are exempt from the field-level check.
var excludedFromGenerated = map[string]bool{
	"webhookPersonFromGenerated": true,
}

// markerRe matches the wrapper-side opt-out comment. The reason is
// required: `// intentionally-omitted: <tag> - <reason text>`. The tag
// portion is captured for matching; the reason portion is validated as
// non-empty but otherwise free-form.
var markerRe = regexp.MustCompile(`intentionally-omitted:\s*([a-zA-Z0-9_]+)\s*-\s*\S`)

// structFields captures the JSON tag set of a struct plus the
// intentionally-omitted markers associated with it. Tag is the JSON tag
// (the part before any comma, e.g. "tagline" from `json:"tagline,omitempty"`).
type structFields struct {
	tags        map[string]bool
	omitted     map[string]bool
	declaration token.Pos
}

func main() {
	verbose := flag.Bool("v", false, "verbose output (list every pair walked)")
	root := flag.String("root", "", "repo root (default: walk up from cwd until go/pkg/basecamp/ is found)")
	flag.Parse()

	repoRoot, err := resolveRoot(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(2)
	}

	wrapperDir := filepath.Join(repoRoot, "go", "pkg", "basecamp")
	generatedFile := filepath.Join(repoRoot, "go", "pkg", "generated", "client.gen.go")

	if err := run(wrapperDir, generatedFile, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// resolveRoot finds the repo root. If root is set, use it directly. Otherwise
// walk up from cwd looking for go/pkg/basecamp/.
func resolveRoot(root string) (string, error) {
	if root != "" {
		return root, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		marker := filepath.Join(dir, "go", "pkg", "basecamp")
		if info, err := os.Stat(marker); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repo root (no go/pkg/basecamp/ in any ancestor of %s)", cwd)
		}
		dir = parent
	}
}

func run(wrapperDir, generatedFile string, verbose bool) error {
	fset := token.NewFileSet()

	// Parse the generated client.
	genFile, err := parser.ParseFile(fset, generatedFile, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse generated: %w", err)
	}
	genStructs := collectStructsAndMarkers(fset, genFile)

	// Parse all wrapper files.
	entries, err := os.ReadDir(wrapperDir)
	if err != nil {
		return fmt.Errorf("read wrapper dir: %w", err)
	}
	wrapperStructs := map[string]*structFields{}
	fromGenPairs := map[string]string{} // wrapper name -> generated name (derived from *FromGenerated signatures)
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(wrapperDir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for k, v := range collectStructsAndMarkers(fset, f) {
			wrapperStructs[k] = v
		}
		for k, v := range collectFromGeneratedPairs(f) {
			if !excludedFromGenerated[k+"FromGenerated"] && !excludedFromGenerated[lowercaseFirst(k)+"FromGenerated"] {
				fromGenPairs[k] = v
			}
		}
	}

	// Build the final pair list: union of fromGen + directDecode.
	pairs := map[string]string{}
	for k, v := range fromGenPairs {
		pairs[k] = v
	}
	for k, v := range directDecodePairs {
		pairs[k] = v
	}

	// Check each pair.
	pairNames := make([]string, 0, len(pairs))
	for k := range pairs {
		pairNames = append(pairNames, k)
	}
	sort.Strings(pairNames)

	var drift []string
	totalFieldsChecked := 0
	for _, wrapName := range pairNames {
		genName := pairs[wrapName]
		gen := genStructs[genName]
		wrap := wrapperStructs[wrapName]
		if gen == nil {
			drift = append(drift, fmt.Sprintf("PAIR ERROR: wrapper %s expects generated %s but it was not found in client.gen.go", wrapName, genName))
			continue
		}
		if wrap == nil {
			drift = append(drift, fmt.Sprintf("PAIR ERROR: wrapper %s referenced in %sFromGenerated or directDecodePairs but the wrapper struct was not found in go/pkg/basecamp/", wrapName, lowercaseFirst(wrapName)))
			continue
		}

		// Walk every JSON tag declared on the generated struct.
		tags := make([]string, 0, len(gen.tags))
		for t := range gen.tags {
			tags = append(tags, t)
		}
		sort.Strings(tags)

		var missing []string
		for _, tag := range tags {
			totalFieldsChecked++
			if wrap.tags[tag] {
				continue
			}
			if wrap.omitted[tag] {
				continue
			}
			missing = append(missing, tag)
		}
		if len(missing) > 0 {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: missing JSON tags %v (add to wrapper struct or mark with `// intentionally-omitted: <tag> - <reason>`)", wrapName, genName, missing))
		}
		if verbose {
			fmt.Printf("  %s ↔ generated.%s (%d generated tags, %d wrapper tags, %d omitted)\n",
				wrapName, genName, len(gen.tags), len(wrap.tags), len(wrap.omitted))
		}
	}

	// Validate any intentionally-omitted markers point at real generated tags.
	// This catches typos where a wrapper claims to omit "foo" but the generated
	// type emits "foo_bar".
	for _, wrapName := range pairNames {
		genName := pairs[wrapName]
		gen := genStructs[genName]
		wrap := wrapperStructs[wrapName]
		if gen == nil || wrap == nil {
			continue
		}
		for t := range wrap.omitted {
			if !gen.tags[t] {
				drift = append(drift, fmt.Sprintf("%s: intentionally-omitted marker for %q does not match any field in generated.%s", wrapName, t, genName))
			}
		}
	}

	fmt.Printf("Wrapper drift check: %d pairs walked, %d generated fields verified\n", len(pairNames), totalFieldsChecked)

	if len(drift) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "=== DRIFT DETECTED ===")
		for _, d := range drift {
			fmt.Fprintln(os.Stderr, "  -", d)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Fix: either propagate the generated field on the wrapper struct + the FromGenerated function, or add a comment of the form")
		fmt.Fprintln(os.Stderr, "     `// intentionally-omitted: <tag> - <reason>` inside the wrapper struct's declaration.")
		return fmt.Errorf("wrapper drift: %d issue(s)", len(drift))
	}

	return nil
}

// collectStructsAndMarkers walks the AST and returns a map of struct name
// -> tag/omitted info. Only top-level type X struct {…} declarations are
// collected. Intentionally-omitted markers are scraped from ALL comments
// that fall within the struct's source range (between the opening { and
// closing }), so markers don't need to be attached to a specific field —
// they can sit on their own line inside the struct body.
func collectStructsAndMarkers(fset *token.FileSet, f *ast.File) map[string]*structFields {
	out := map[string]*structFields{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			sf := &structFields{
				tags:        map[string]bool{},
				omitted:     map[string]bool{},
				declaration: ts.Pos(),
			}
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				tagVal := field.Tag.Value
				if tag := extractJSONTag(tagVal); tag != "" {
					sf.tags[tag] = true
				}
			}
			// Scan every comment inside the struct body for opt-out markers.
			// (Field-attached comments are duplicates of these for our purposes;
			// scanning the full range catches free-standing marker lines too.)
			start := st.Fields.Opening
			end := st.Fields.Closing
			for _, cg := range f.Comments {
				if cg.Pos() < start || cg.End() > end {
					continue
				}
				for _, c := range cg.List {
					if m := markerRe.FindStringSubmatch(c.Text); m != nil {
						sf.omitted[m[1]] = true
					}
				}
			}
			out[ts.Name.Name] = sf
		}
	}
	return out
}

// collectFromGeneratedPairs walks the AST for function declarations of the form
//
//	func xFromGenerated(g generated.Y) X
//
// and returns a map of wrapper struct name -> generated struct name. The
// function name does not need to match anything specific; the type signature
// is authoritative.
func collectFromGeneratedPairs(f *ast.File) map[string]string {
	out := map[string]string{}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil {
			continue
		}
		if !strings.HasSuffix(fd.Name.Name, "FromGenerated") {
			continue
		}
		if excludedFromGenerated[fd.Name.Name] {
			continue
		}
		// Need exactly one param and one result.
		if fd.Type.Params == nil || len(fd.Type.Params.List) != 1 {
			continue
		}
		if fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
			continue
		}
		paramType := extractGeneratedTypeName(fd.Type.Params.List[0].Type)
		if paramType == "" {
			continue
		}
		resultType := extractLocalTypeName(fd.Type.Results.List[0].Type)
		if resultType == "" {
			continue
		}
		out[resultType] = paramType
	}
	return out
}

// extractGeneratedTypeName recognizes `generated.X` (SelectorExpr) and returns
// X. Returns "" otherwise.
func extractGeneratedTypeName(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "generated" {
		return ""
	}
	return sel.Sel.Name
}

// extractLocalTypeName recognizes a bare identifier (the wrapper struct
// returned by FromGenerated) and returns its name.
func extractLocalTypeName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

// extractJSONTag pulls the tag name from a struct tag literal like
// "`json:\"foo,omitempty\"`". Returns "" if no json tag is present.
func extractJSONTag(tagLiteral string) string {
	// Strip the surrounding backticks.
	if len(tagLiteral) < 2 || tagLiteral[0] != '`' || tagLiteral[len(tagLiteral)-1] != '`' {
		return ""
	}
	inner := tagLiteral[1 : len(tagLiteral)-1]
	// Use reflect-style key-value parsing. Tags look like `json:"foo,omitempty" xml:"bar"`.
	for inner != "" {
		// Skip leading spaces.
		i := 0
		for i < len(inner) && inner[i] == ' ' {
			i++
		}
		inner = inner[i:]
		if inner == "" {
			break
		}
		// Find key (up to ':').
		colon := strings.IndexByte(inner, ':')
		if colon == -1 {
			break
		}
		key := inner[:colon]
		// Value must start with a quote.
		if colon+1 >= len(inner) || inner[colon+1] != '"' {
			break
		}
		// Find closing quote (Go struct tags don't escape quotes in values).
		end := strings.IndexByte(inner[colon+2:], '"')
		if end == -1 {
			break
		}
		val := inner[colon+2 : colon+2+end]
		if key == "json" {
			// Take everything before the first comma.
			comma := strings.IndexByte(val, ',')
			if comma == -1 {
				return val
			}
			return val[:comma]
		}
		inner = inner[colon+3+end:]
	}
	return ""
}

func lowercaseFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
