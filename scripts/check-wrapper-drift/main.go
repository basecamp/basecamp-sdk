// Command check-wrapper-drift performs a field-level drift check between the
// hand-written wrappers in go/pkg/basecamp/ and the generated types in
// go/pkg/generated/.
//
// # Discovery
//
// The script walks (wrapper, generated) pairs in two ways:
//
//  1. By signature reading every `<lower>FromGenerated(g generated.X) Y`
//     function declaration in go/pkg/basecamp/*.go (non-test). The argument
//     type names the generated struct; the return type names the wrapper
//     struct. (`webhookPersonFromGenerated` is special-cased and excluded
//     from the *FromGenerated convention check below — it is a parallel
//     mapping for WebhookEventPerson, not a Person wrapper.)
//
//  2. By an explicit `directDecodePairs` map covering wrappers that decode
//     straight from JSON bytes (Notification, NotificationsResult, MyAssignment,
//     MyAssignmentsResult, Gauge, GaugeNeedle, Account, Preferences,
//     OutOfOffice) plus the nested public wrapper structs reachable from them
//     (PreviewableAttachment, MyAssignmentAssignee, MyAssignmentBucket,
//     MyAssignmentParent, AccountLogo, AccountLimits, AccountSettings,
//     AccountSubscription, OutOfOfficePerson). These do not have
//     *FromGenerated declarations; the JSON tags on the wrapper struct fields
//     are what the JSON decoder uses to read the wire payload. Listing the
//     nested structs explicitly carries the tag-presence check into them
//     rather than stopping at the parent field. See the directDecodePairs
//     declaration below for the derivation recipe that produced the list.
//
// # Check
//
// For each pair, the script compares JSON tag names (not Go field names —
// shape-equivalent tag collisions like wrapper URL with json tag "url" vs
// generated Url with json tag "url" are handled correctly because the match
// is keyed on the json:"…" tag value, e.g. "url"). For every JSON tag
// declared on the generated struct, the wrapper must either:
//
//   - declare a field with the same JSON tag, or
//   - carry an `// intentionally-omitted: <tag> - <reason>` marker (ASCII
//     hyphen, matching the repo's default comment convention) anywhere
//     inside the wrapper struct's definition block.
//
// If neither is present, the script reports drift and exits 1.
//
// # Population check
//
// Declaring the tag is necessary but not sufficient: a wrapper field can carry
// the right JSON tag yet never be assigned by its *FromGenerated conversion
// function, so it silently stays zero-valued on the wire while the tag-presence
// check passes. For every *FromGenerated-backed pair, the script therefore also
// confirms the conversion body actually assigns each tagged wrapper field. It
// AST-walks the function body and collects assigned wrapper fields from two
// forms (see collectAssignedFields): the wrapper's own composite literal
// (`c := Card{Status: ...}`) and selector-target assignments (`c.Creator =
// ...`, `c.Steps = append(...)`). A tag-present-but-never-assigned field is
// reported as drift.
//
// Scope and limitations of the population check (verified against the current
// go/pkg/basecamp/ corpus, where every *FromGenerated follows this shape):
//
//   - It is a *reachability* check, not a value check: it proves the field is
//     written somewhere in the body, not that the written value is correct or
//     that the assignment is unconditional. A field assigned only inside an
//     `if` branch (e.g. nested Creator/Bucket pointers, which are gated on the
//     generated value being non-empty) counts as populated — matching the
//     wrappers' intentional "leave nil when the source is empty" semantics.
//   - One level of nesting only, consistent with the tag check: a parent field
//     assigned via a nested helper (`c.Creator = &creator` where `creator =
//     personFromGenerated(...)`) counts because the parent field is assigned;
//     the nested Person's own fields are verified through the separate
//     Person ↔ generated.Person pair.
//   - Direct-decode wrappers (the directDecodePairs set) are EXEMPT: they have
//     no *FromGenerated body and are populated by json.Unmarshal straight onto
//     the struct tags, so tag presence already is population for them.
//   - A field genuinely populated by some mechanism the walker cannot see
//     (none exist today) should carry an `// intentionally-omitted` marker with
//     a reason, which suppresses both the tag and population checks for it.
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
//
// The set covers both the top-level raw-body wrappers AND the nested public
// wrapper structs reachable from them that have a generated counterpart but no
// *FromGenerated of their own (they are populated by the same json.Unmarshal
// pass as their parent). Listing the nested structs explicitly extends the
// tag-presence check into them; without these entries the parent check only
// verifies the parent field (e.g. previewable_attachments, assignees, bucket)
// and a future generated-field addition inside the nested struct would slip
// through. Because they are direct-decode, only the tag-presence check applies
// (json.Unmarshal populates them straight from the tags), not the population
// check.
//
// # Completeness
//
// This map is intended to be the COMPLETE set of (wrapper, generated) direct-
// decode pairs as of this PR. The derivation recipe used to build it (and the
// recipe future contributors should re-run when adding endpoints):
//
//  1. Grep go/pkg/basecamp/*.go (non-test) for raw-decode call sites:
//     `json.Unmarshal(... &<local>)`, `json.NewDecoder(...).Decode(&<local>)`,
//     and decode helpers (e.g. `decodeGaugePayload`).
//  2. For each site whose target local is a hand-written WRAPPER struct (not a
//     `generated.X` value routed through a `*FromGenerated` function), check
//     whether a `generated.<Name>` (or close-named) counterpart exists in
//     `go/pkg/generated/client.gen.go`.
//  3. If yes, add (wrapper, generated) here. Also add every nested PUBLIC
//     wrapper struct reachable from it whose fields are populated by the same
//     json.Unmarshal (no *FromGenerated of its own).
//
// Excluded by design:
//   - WebhookEvent and its parallel webhook-flavored wrapper types
//     (WebhookEventRecording / WebhookEventPerson / ...): these are a separate
//     representation, not aligned 1:1 with `generated.WebhookEvent`'s nested
//     `Recording` / `Person`. They follow the same precedent as
//     `webhookPersonFromGenerated` (see excludedFromGenerated).
//   - Local request / response envelope structs used to read upstream API
//     errors, the Launchpad authorization endpoint, embedded SDK provenance,
//     and the like, which are not driven by the OpenAPI spec.
var directDecodePairs = map[string]string{
	"Notification":        "Notification",
	"NotificationsResult": "GetMyNotificationsResponseContent",
	"MyAssignment":        "MyAssignment",
	"Gauge":               "Gauge",
	"GaugeNeedle":         "GaugeNeedle",
	"Account":             "Account",
	"Preferences":         "Preferences",
	"OutOfOffice":         "OutOfOffice",
	"MyAssignmentsResult": "GetMyAssignmentsResponseContent",
	// Nested direct-decode structs (no *FromGenerated; decoded with their parent).
	"PreviewableAttachment": "PreviewableAttachment", // nested in Notification.previewable_attachments
	"MyAssignmentAssignee":  "MyAssignmentAssignee",  // nested in MyAssignment.assignees
	"MyAssignmentBucket":    "MyAssignmentBucket",    // nested in MyAssignment.bucket
	"MyAssignmentParent":    "MyAssignmentParent",    // nested in MyAssignment.parent
	"AccountLogo":           "AccountLogo",           // nested in Account.logo
	"AccountLimits":         "AccountLimits",         // nested in Account.limits
	"AccountSettings":       "AccountSettings",       // nested in Account.settings
	"AccountSubscription":   "AccountSubscription",   // nested in Account.subscription
	"OutOfOfficePerson":     "OutOfOfficePerson",     // nested in OutOfOffice.person
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
//
// tagToGoField maps each JSON tag back to its Go field identifier (e.g.
// "tagline" -> "Tagline"). The population check (see run) uses it to translate
// the set of assigned Go fields collected from a *FromGenerated body into the
// JSON-tag space the rest of the check operates in.
type structFields struct {
	tags         map[string]bool
	omitted      map[string]bool
	tagToGoField map[string]string
	declaration  token.Pos
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

	if err := run(wrapperDir, generatedFile, directDecodePairs, *verbose); err != nil {
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

// run performs the full drift check. directDecode is injected (rather than read
// from the package global) so tests can drive run() end-to-end with their own
// fixtures without dragging in the production direct-decode pair set, whose
// generated structs would otherwise have to exist in every test fixture.
// main() passes the package-level directDecodePairs.
func run(wrapperDir, generatedFile string, directDecode map[string]string, verbose bool) error {
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
	fromGenPairs := map[string]string{}            // wrapper name -> generated name (derived from *FromGenerated signatures)
	assignedFields := map[string]map[string]bool{} // wrapper name -> set of Go fields its *FromGenerated body assigns
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
		// collectFromGeneratedPairs already drops excluded functions by their
		// function name (see excludedFromGenerated check inside it), so no
		// second exclusion is needed here. Re-filtering by wrapper struct name
		// would be dead code: the keys are wrapper struct names (e.g.
		// WebhookEventPerson), not function names (webhookPersonFromGenerated).
		for k, v := range collectFromGeneratedPairs(f) {
			fromGenPairs[k] = v
		}
		for k, fields := range collectAssignedFields(f) {
			set := assignedFields[k]
			if set == nil {
				set = map[string]bool{}
				assignedFields[k] = set
			}
			for fn := range fields {
				set[fn] = true
			}
		}
	}

	// Build the final pair list: union of fromGen + directDecode.
	pairs := map[string]string{}
	for k, v := range fromGenPairs {
		pairs[k] = v
	}
	for k, v := range directDecode {
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
	totalFieldsPopChecked := 0
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

		// Direct-decode wrappers (Notification, MyAssignment, ...) have no
		// *FromGenerated body: the JSON decoder populates them straight from the
		// struct tags, so tag presence IS population. The population check below
		// only applies to *FromGenerated-backed pairs.
		_, isDirectDecode := directDecode[wrapName]
		assigned := assignedFields[wrapName]

		// Walk every JSON tag declared on the generated struct.
		tags := make([]string, 0, len(gen.tags))
		for t := range gen.tags {
			tags = append(tags, t)
		}
		sort.Strings(tags)

		var missing []string
		var unpopulated []string
		for _, tag := range tags {
			totalFieldsChecked++
			if wrap.omitted[tag] {
				continue
			}
			if !wrap.tags[tag] {
				missing = append(missing, tag)
				continue
			}
			// Tag is declared on the wrapper. For *FromGenerated-backed pairs,
			// also confirm the conversion body actually assigns the field —
			// otherwise a tag-present-but-unassigned field silently stays
			// zero-valued while this check would otherwise pass.
			if !isDirectDecode {
				totalFieldsPopChecked++
				goField := wrap.tagToGoField[tag]
				if goField != "" && (assigned == nil || !assigned[goField]) {
					unpopulated = append(unpopulated, fmt.Sprintf("%s (field %s)", tag, goField))
				}
			}
		}
		if len(missing) > 0 {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: missing JSON tags %v (add to wrapper struct or mark with `// intentionally-omitted: <tag> - <reason>`)", wrapName, genName, missing))
		}
		if len(unpopulated) > 0 {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: wrapper declares these tags but %sFromGenerated never assigns them %v (assign the field in the conversion function, or mark with `// intentionally-omitted: <tag> - <reason>` if the wrapper field is populated by some other means)", wrapName, genName, lowercaseFirst(wrapName), unpopulated))
		}
		if verbose {
			fmt.Printf("  %s ↔ generated.%s (%d generated tags, %d wrapper tags, %d omitted, %d assigned fields, directDecode=%v)\n",
				wrapName, genName, len(gen.tags), len(wrap.tags), len(wrap.omitted), len(assigned), isDirectDecode)
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

	fmt.Printf("Wrapper drift check: %d pairs walked, %d generated fields verified (%d field assignments verified in *FromGenerated bodies)\n", len(pairNames), totalFieldsChecked, totalFieldsPopChecked)

	if len(drift) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "=== DRIFT DETECTED ===")
		for _, d := range drift {
			fmt.Fprintln(os.Stderr, "  -", d)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Fix: either propagate the generated field on the wrapper struct + assign it in the *FromGenerated function, or add a comment of the form")
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
				tags:         map[string]bool{},
				omitted:      map[string]bool{},
				tagToGoField: map[string]string{},
				declaration:  ts.Pos(),
			}
			for _, field := range st.Fields.List {
				if field.Tag == nil {
					continue
				}
				tagVal := field.Tag.Value
				if tag := extractJSONTag(tagVal); tag != "" {
					sf.tags[tag] = true
					// Record the Go field identifier for this tag. Tagged
					// fields in these structs always have exactly one name;
					// if a field ever had multiple names sharing a tag, the
					// last wins (still correct for membership lookups).
					for _, fn := range field.Names {
						sf.tagToGoField[tag] = fn.Name
					}
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

// collectAssignedFields walks every non-excluded *FromGenerated function in the
// file and, for each, records the set of wrapper Go fields the body actually
// assigns. Two assignment forms are recognized, which together cover every
// *FromGenerated in go/pkg/basecamp/:
//
//  1. The wrapper's own composite literal, e.g. `c := Card{Status: ..., ...}` —
//     every KeyValueExpr key whose enclosing composite-literal type names the
//     wrapper struct. Nested literals like `&Parent{ID: ...}` and `&Bucket{...}`
//     are correctly ignored because their type identifier is Parent/Bucket, not
//     the wrapper, so only the parent field (`c.Parent = ...`) counts as
//     populated — matching the check's one-level-nesting termination.
//  2. Selector-target assignments to the wrapper instance, e.g. `c.ID = ...`,
//     `c.Creator = &creator`, `c.Assignees = append(...)` — every
//     AssignStmt / IncDecStmt whose LHS is a SelectorExpr rooted in the wrapper
//     variable. The wrapper variable is identified up front (see
//     findWrapperVars): the named result, the local the wrapper composite
//     literal is bound to (`c := Card{...}`), and the operand of `return c`.
//     Selector writes to any OTHER local are ignored — a *FromGenerated body
//     frequently builds nested helper values via their own locals
//     (`creator := personFromGenerated(...)`, `d := WebhookDelivery{...};
//     d.ID = *gd.Id`, `c := &WebhookCopy{...}; c.ID = *ge.Copy.Id`). Counting
//     a `d.ID`/`c.ID` selector write as a wrapper-field write would wrongly
//     mark the wrapper's same-named field populated and mask genuine drift, so
//     only writes whose base identifier is the wrapper instance count.
//
// The result maps wrapper struct name -> set of assigned Go field names. It is
// keyed on the function's *return* type, so it lines up with the wrapper-side of
// each (wrapper, generated) pair. Multiple functions returning the same wrapper
// (across files) accumulate into one set.
func collectAssignedFields(f *ast.File) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Body == nil {
			continue
		}
		if !strings.HasSuffix(fd.Name.Name, "FromGenerated") {
			continue
		}
		if excludedFromGenerated[fd.Name.Name] {
			continue
		}
		if fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
			continue
		}
		wrapper := extractLocalTypeName(fd.Type.Results.List[0].Type)
		if wrapper == "" {
			continue
		}
		assigned := out[wrapper]
		if assigned == nil {
			assigned = map[string]bool{}
			out[wrapper] = assigned
		}
		// Identify the variable(s) that hold the wrapper instance this function
		// builds and returns, so selector-target writes can be scoped to it.
		wrapperVars := findWrapperVars(fd, wrapper)
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				// Only the wrapper's own literal contributes field names;
				// nested helper literals (Parent/Bucket/...) are skipped.
				if litTypeName(node.Type) != wrapper {
					return true
				}
				for _, elt := range node.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if key, ok := kv.Key.(*ast.Ident); ok {
						assigned[key.Name] = true
					}
				}
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if base, name := selectorBaseAndField(lhs); name != "" && wrapperVars[base] {
						assigned[name] = true
					}
				}
			case *ast.IncDecStmt:
				if base, name := selectorBaseAndField(node.X); name != "" && wrapperVars[base] {
					assigned[name] = true
				}
			}
			return true
		})
	}
	return out
}

// findWrapperVars returns the set of local identifier names that hold the
// wrapper instance a *FromGenerated function builds and returns. Selector-target
// assignments (`x.Field = ...`) only count as wrapper-field population when their
// base identifier is in this set; writes to helper locals (a nested Person, a
// WebhookDelivery, a WebhookCopy) are excluded so they cannot masquerade as
// wrapper-field writes and mask drift.
//
// Three sources, covering every shape a *FromGenerated may take:
//
//   - Named result values: `func f(...) (w Wrapper)`. The result identifier is
//     the wrapper instance even before any assignment.
//   - The local bound to the wrapper's composite literal: `c := Card{...}` (or
//     `c := &Card{...}`, or `var c Card`). This is the universal shape in the
//     current corpus (`x := Wrapper{...}; ...; return x`).
//   - The operand of a bare `return c`. Redundant with the composite-literal
//     binding for today's code, but it keeps the var set correct if a body ever
//     constructs the wrapper without a recognizable literal binding.
func findWrapperVars(fd *ast.FuncDecl, wrapper string) map[string]bool {
	vars := map[string]bool{}
	// Named results.
	if fd.Type.Results != nil {
		for _, field := range fd.Type.Results.List {
			for _, name := range field.Names {
				if name.Name != "" && name.Name != "_" {
					vars[name.Name] = true
				}
			}
		}
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// `c := Wrapper{...}` / `c = Wrapper{...}` / `c := &Wrapper{...}`.
			// Bind each LHS identifier whose paired RHS is a composite literal
			// of the wrapper type.
			if len(node.Lhs) == len(node.Rhs) {
				for i, rhs := range node.Rhs {
					if compositeLitTypeName(rhs) == wrapper {
						if id, ok := node.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
							vars[id.Name] = true
						}
					}
				}
			}
		case *ast.ReturnStmt:
			// `return c` — the returned identifier is the wrapper instance.
			for _, res := range node.Results {
				if id, ok := res.(*ast.Ident); ok && id.Name != "_" {
					vars[id.Name] = true
				}
			}
		}
		return true
	})
	return vars
}

// compositeLitTypeName returns the wrapper-type name of a composite-literal
// expression, transparently unwrapping a leading address-of (`&Wrapper{}`).
// Returns "" for anything that is not a bare-identifier-typed composite literal.
func compositeLitTypeName(expr ast.Expr) string {
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = u.X
	}
	cl, ok := expr.(*ast.CompositeLit)
	if !ok {
		return ""
	}
	return litTypeName(cl.Type)
}

// litTypeName returns the type identifier of a composite-literal type
// expression (`Card{}` -> "Card"). Returns "" for non-identifier types
// (slices, maps, qualified types like generated.X).
func litTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// selectorBaseAndField decomposes an `x.Field` selector rooted in a bare
// identifier into its base identifier and field name (`c.Creator` -> "c",
// "Creator"). Returns "", "" for anything else (index expressions, deeper
// chains like `a.b.c`, non-selector expressions). The base lets callers scope
// the write to a known wrapper variable.
func selectorBaseAndField(expr ast.Expr) (base, field string) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", ""
	}
	return ident.Name, sel.Sel.Name
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
