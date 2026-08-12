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
//  2. By an explicit `directDecodePairs` map covering pairs whose wrappers
//     have no *FromGenerated function for the signature walker to find. The
//     map organizes these into two labeled tiers — tier 2 (direct-decode via
//     json.Unmarshal, including nested wrappers reachable from the same
//     Unmarshal pass) and tier 3 (inline-converted via composite literal
//     inside a *FromGenerated body or service method). Both tiers run the
//     tag-presence check. Tier 3 also runs the population check, sourced from
//     collectCompositeLiteralFields rather than a *FromGenerated body; tier 2
//     is the only tier where the JSON decoder is the population guarantee and
//     tag presence alone is sufficient. See the directDecodePairs declaration
//     below for the full tier model, derivation recipe, and exclusion list.
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
// the right JSON tag yet never be assigned by the wrapper's construction site,
// so it silently stays zero-valued on the wire while the tag-presence check
// passes. The check therefore also confirms the construction site actually
// assigns each tagged wrapper field. Two construction shapes are covered:
//
//   - Tier 1 (*FromGenerated body): for every `<lower>FromGenerated(g
//     generated.X) Y` declaration, the body is AST-walked to collect the
//     assigned wrapper fields from two forms (see collectAssignedFields): the
//     wrapper's own composite literal (`c := Card{Status: ...}`) and
//     selector-target assignments (`c.Creator = ...`, `c.Steps = append(...)`).
//   - Tier 3 (inline composite literal): the wrapper has no *FromGenerated of
//     its own and is built by a `Wrapper{...}` (or `&Wrapper{...}`) composite
//     literal inside some other function — a parent *FromGenerated body or a
//     service method. For each such literal anywhere in go/pkg/basecamp/, the
//     walker collects keys from the literal AND from subsequent selector writes
//     to the local path (bare identifier or selector chain like `q.Schedule`)
//     the literal is bound to. See collectCompositeLiteralFields.
//
// A tag-present-but-never-assigned field is reported as drift.
//
// Scope and limitations of the population check (verified against the current
// go/pkg/basecamp/ corpus, where every wrapper follows one of these shapes):
//
//   - It is a *reachability* check, not a value check: it proves the field is
//     written somewhere in the construction site, not that the written value
//     is correct or that the assignment is unconditional. A field assigned only
//     inside an `if` branch (e.g. nested Creator/Bucket pointers, which are
//     gated on the generated value being non-empty) counts as populated —
//     matching the wrappers' intentional "leave nil when the source is empty"
//     semantics.
//   - One level of nesting only, consistent with the tag check: a parent field
//     assigned via a nested helper (`c.Creator = &creator` where `creator =
//     personFromGenerated(...)`) counts because the parent field is assigned;
//     the nested Person's own fields are verified through the separate
//     Person ↔ generated.Person pair.
//   - Tier-2 wrappers (the json.Unmarshal subset of directDecodePairs) are
//     EXEMPT: they have no *FromGenerated body and no composite literal — they
//     are populated by json.Unmarshal straight onto the struct tags, so tag
//     presence IS population. The tier-3 subset of directDecodePairs DOES get
//     a population check via the composite-literal walker.
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
//
// # Embedded (anonymous) fields
//
// An anonymous field has no tag and no name, so the field walk cannot read a
// JSON tag off it — but its own tagged fields ARE on the wire, promoted into
// the embedding struct. Both sides of every pair are therefore flattened
// before comparison (see flattenEmbedded): each struct's tag set is its own
// tagged fields plus the tagged fields promoted through its embedded types,
// resolved recursively. Without this, a wrapper that embeds a struct reads as
// having none of the promoted fields and every one of them is reported as
// missing (issue #599).
//
// The flattening follows encoding/json's promotion rules, since the wire shape
// is what this check is about:
//
//   - Embedded pointers (`*Base`) promote exactly like value embeds.
//   - Promotion is transitive: a struct embedding a struct that embeds a struct
//     contributes all three levels, breadth-first by depth.
//   - Shallower wins. A JSON tag declared directly on the embedding struct
//     shadows the same tag promoted from an embedded type; a depth-1 promotion
//     shadows the same tag at depth 2.
//   - Two embedded types contributing the SAME tag at the SAME depth cancel
//     each other out — encoding/json emits neither — so the tag counts as
//     absent, and deeper occurrences stay shadowed (matching
//     encoding/json's dominantField).
//   - The same applies to one type reached by two paths at the same depth (the
//     diamond: Outer embeds Left and Right, both embedding Common). Such a
//     type is walked once but its arrival count is remembered, and its fields
//     are entered twice so the rule above annihilates them — mirroring the
//     nextCount/count pair in encoding/json's typeFields, which duplicates the
//     field for exactly this reason. Deduplicating without counting would
//     claim a tag that is not on the wire.
//   - Unexported fields are ignored, whatever their tag says, because
//     encoding/json ignores unexported non-anonymous fields; one cannot
//     satisfy a generated tag. An embedded unexported STRUCT TYPE is the
//     opposite case — its exported fields promote normally.
//   - An anonymous field that carries its own json:"…" tag is NOT promoted:
//     encoding/json treats it as an ordinary named field under that tag, and so
//     does this check — the tag is recorded, with the embedded type's name as
//     its Go field name for the population check. (`json:"-"` is recorded as
//     the literal tag "-", matching how the check already handles a named
//     field spelled that way; see TestExtractJSONTag_DashSentinel.)
//   - Cycles (A embeds B embeds A, or a self-embedding pointer) terminate: each
//     type name is visited once per flattening.
//
// An embedded type the checker cannot resolve — a qualified one from another
// package (`time.Time`), or a name not declared in the parsed sources — is
// reported as drift for any pair that reaches it, rather than skipped. Skipping
// silently is what produced #599 in the first place; the check parses only
// go/pkg/basecamp/*.go and client.gen.go, so it cannot see another package's
// fields and must not pretend they are absent. The report is deferred to the
// pair walk, so an unresolvable embed on a struct outside every pair (today:
// FlexTime embedding time.Time) costs nothing.
//
// An embedded name that resolves to a declared NON-struct type (an interface, a
// map, a slice — today: generated.ClientWithResponses embedding ClientInterface)
// promotes no JSON-tagged fields and contributes nothing. A defined type whose
// underlying type is another name (`type Alias Base`) is followed to that name.
//
// # Population of promoted fields
//
// A promoted field can be assigned by several spellings, and the population
// check accepts each (see populationTargets): the promoted field itself
// (`w.CreatedAt`), any partially or fully qualified path to it
// (`w.Audit.CreatedAt`, `w.Base.Audit.CreatedAt`), or an assignment of any
// embed on the path. That last one is only total coverage when the assigned
// value is OPAQUE — a call or a variable the walker cannot see inside. When it
// is a visible struct literal the walker enumerates its keys instead, so
// `w.Base = Base{ID: g.Id}` populates `id` and nothing else: the fields the
// literal omits stay zero-valued on the wire, and crediting them would let the
// next generated field through in silence.
//
// What this deliberately does NOT check is whether a pointer embed on the path
// was initialized before a promoted write. `w.CreatedAt = …` across a nil
// `*Audit` panics — but that is a loud, immediate crash on the first execution
// of the path, not the silent zero-valued wire data this guard exists to
// catch, and the guard is a reachability check by design (it counts a
// conditional assignment as populated, and does not model statement order).
// Demanding initialization means recognizing every spelling that provides it,
// and a walk that misses one manufactures drift — which is the failure #599
// itself was: it trains people to work around the guard rather than read it.
// Nil-capability analysis is #621's subject; this walk is the input it needs,
// not the place to do it.
//
// `intentionally-omitted` markers are NOT inherited through an embed: a marker
// declares that one wrapper deliberately drops a tag of ITS generated
// counterpart, and each marker is validated against that counterpart, so
// inheriting one would suppress a check in an unrelated pair and would report
// marker/generated mismatches for tags the embedded struct's own pair owns. An
// embedding wrapper that means to drop a tag carries its own marker; the
// failure mode of getting this wrong is a loud missing-tag report, not silence.
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

// directDecodePairs maps the wrapper struct name to the generated struct name
// for wrappers that have a `generated.X` counterpart but no `*FromGenerated`
// function the tier-1 walker discovers — i.e. wrappers populated either by a
// raw json.Unmarshal (tier 2) or by an inline composite literal inside someone
// else's body (tier 3). Tier-3 entries are also listed in tier3Wrappers so the
// population walker knows to scan composite literals for them.
//
// # Coverage model: three tiers
//
// The drift check operates on a UNION of wrapper↔generated pairs derived from
// three sources. Tiers 1 and 3 both get the population check; tier 2 does not
// (it has no in-package literal — the JSON decoder is the population). All
// three live in one tag-presence pass so future contributors see the coverage
// as a single surface.
//
//   - Tier 1: *FromGenerated-backed pairs. Discovered automatically by walking
//     every `<lower>FromGenerated(g generated.X) Y` declaration (see
//     collectFromGeneratedPairs). These get BOTH the tag-presence check AND
//     the population check — the function body is AST-walked to confirm each
//     tagged wrapper field is assigned (see collectAssignedFields). This tier
//     is NOT in this map; the function signature is the contract.
//
//   - Tier 2: direct-decode pairs (raw json.Unmarshal). Wrappers populated by
//     `json.Unmarshal(rawBody, &wrapper)` on a (sometimes pre-normalized) raw
//     response body, with no *FromGenerated function and no in-package
//     composite literal to walk. The JSON decoder writes each generated field
//     straight onto the matching wrapper tag, so tag presence IS population.
//     The wrapper struct's JSON tags are the contract. Includes both top-level
//     raw-body wrappers and the nested public wrapper structs reachable from
//     them that share the same json.Unmarshal pass.
//
//   - Tier 3: inline-converted pairs (composite-literal construction). Wrappers
//     populated by an explicit `Wrapper{Field: g.Field, ...}` composite literal
//     inside a parent `*FromGenerated` body (e.g. CampfireLineAttachment built
//     in campfireLineFromGenerated) OR inside a service method that builds the
//     wrapper directly from a generated response value (e.g. LineupMarker built
//     in LineupService.ListMarkers, SearchMetadata in SearchService.Metadata,
//     UpdateProjectAccessResponse in PeopleService.UpdateProjectAccess). They
//     have no *FromGenerated of their own. These get BOTH checks: the
//     tag-presence check (this map) AND the population check, via
//     collectCompositeLiteralFields which walks every non-test wrapper file for
//     composite literals of any tier3Wrappers type, collecting keys from the
//     literal and from subsequent selector writes to the local path the literal
//     is bound to (`resp := Wrapper{...}`, `q.Schedule = &Wrapper{...}`).
//
// # Derivation recipe
//
// This map is intended to be the COMPLETE set of (wrapper, generated) tier-2
// and tier-3 pairs as of this PR. To re-derive when adding endpoints or to
// audit for a suspected 4th category:
//
//  1. Enumerate every `^type <Name> struct` declared in go/pkg/basecamp/*.go
//     (non-test) AND in go/pkg/generated/client.gen.go.
//  2. Intersect the two type-name sets.
//  3. Subtract pairs already covered by tier 1 (every wrapper with a
//     `<lower>FromGenerated` function) and the design exclusions below. Each
//     remaining shared name is a tier-2 or tier-3 candidate.
//  4. Classify by HOW it is populated:
//     - `json.Unmarshal(rawBody, &<wrapper>)` (or a thin decode helper) →
//     tier 2; add it here, plus every nested PUBLIC wrapper struct
//     reachable from it that shares the same Unmarshal pass.
//     - `Wrapper{...}` composite literal in a *FromGenerated body or a
//     service method, reading fields off a `generated.X` value → tier 3;
//     add it here.
//     - Neither → out of scope (likely a request envelope, a non-spec
//     endpoint type, or a parallel webhook-flavored shape).
//
// # Excluded by design
//
//   - WebhookEvent and its parallel webhook-flavored wrapper types
//     (WebhookEventRecording / WebhookEventPerson / WebhookCopy /
//     WebhookCopyBucket / WebhookDelivery / WebhookDeliveryRequest /
//     WebhookDeliveryResponse): a separate representation, not aligned 1:1
//     with `generated.WebhookEvent`'s nested `Recording` / `Person`. Follow
//     the same precedent as `webhookPersonFromGenerated` (see
//     excludedFromGenerated).
//   - Local request / response envelope structs used to read upstream API
//     errors, the Launchpad authorization endpoint, embedded SDK provenance,
//     and similar non-spec wrappers.
//   - Outgoing request wrappers whose name happens to match a
//     `generated.CreateXRequest` / etc. (e.g. CreatePersonRequest,
//     ScheduleAttributes): data flows wrapper→generated, not generated→
//     wrapper. The tag-presence check still works in principle, but the
//     semantics (caller-driven vs server-driven payloads) and the failure mode
//     (caller cannot supply a new field vs wire data silently dropped) differ
//     enough to warrant a separate tier with its own documentation, deferred
//     to a follow-up.
var directDecodePairs = map[string]string{
	// Tier 2: direct-decode (raw json.Unmarshal on a response body), top-level.
	"Notification":        "Notification",
	"NotificationsResult": "GetMyNotificationsResponseContent",
	"MyAssignment":        "MyAssignment",
	"Gauge":               "Gauge",
	"GaugeNeedle":         "GaugeNeedle",
	"Account":             "Account",
	"Preferences":         "Preferences",
	"OutOfOffice":         "OutOfOffice",
	"MyAssignmentsResult": "GetMyAssignmentsResponseContent",
	// Tier 2: direct-decode nested wrappers (no *FromGenerated; decoded with their parent).
	"PreviewableAttachment": "PreviewableAttachment", // nested in Notification.previewable_attachments
	"MyAssignmentAssignee":  "MyAssignmentAssignee",  // nested in MyAssignment.assignees
	"MyAssignmentBucket":    "MyAssignmentBucket",    // nested in MyAssignment.bucket
	"MyAssignmentParent":    "MyAssignmentParent",    // nested in MyAssignment.parent
	"AccountLogo":           "AccountLogo",           // nested in Account.logo
	"AccountLimits":         "AccountLimits",         // nested in Account.limits
	"AccountSettings":       "AccountSettings",       // nested in Account.settings
	"AccountSubscription":   "AccountSubscription",   // nested in Account.subscription
	"OutOfOfficePerson":     "OutOfOfficePerson",     // nested in OutOfOffice.person
	// Tier 3: inline-converted (composite literal in *FromGenerated body or service method).
	"CampfireLineAttachment":      "CampfireLineAttachment", // composite literal in campfireLineFromGenerated (campfires.go)
	"CardColumnOnHold":            "CardColumnOnHold",       // composite literal in cardColumnFromGenerated (cards.go)
	"ClientApprovalResponse":      "ClientApprovalResponse", // composite literal in clientApprovalFromGenerated (client_approvals.go)
	"ClientCompany":               "ClientCompany",          // composite literal in projectFromGenerated (projects.go)
	"EventDetails":                "EventDetails",           // composite literal in eventFromGenerated (events.go)
	"HillChartDot":                "HillChartDot",           // composite literal in hillChartFromGenerated (hill_charts.go)
	"LineupMarker":                "LineupMarker",           // composite literal in LineupService.ListMarkers (lineup.go)
	"PersonCompany":               "PersonCompany",          // composite literal in personFromGenerated (people.go)
	"QuestionSchedule":            "QuestionSchedule",       // composite literal in questionFromGenerated (checkins.go)
	"SearchMetadata":              "SearchMetadata",         // composite literal in SearchService.Metadata (search.go)
	"SearchType":                  "SearchType",             // composite literal in searchTypesFromGenerated (search.go)
	"UpdateProjectAccessResponse": "ProjectAccessResult",    // composite literal in PeopleService.UpdateProjectAccess (people.go)
}

// tier3Wrappers is the subset of directDecodePairs keys whose wrappers are built
// by inline composite literal (not raw json.Unmarshal). For these, the
// population check is sourced from collectCompositeLiteralFields, which scans
// every non-test wrapper file for composite literals of these types. Keep in
// sync with the tier-3 entries in directDecodePairs.
var tier3Wrappers = map[string]bool{
	"CampfireLineAttachment":      true,
	"CardColumnOnHold":            true,
	"ClientApprovalResponse":      true,
	"ClientCompany":               true,
	"EventDetails":                true,
	"HillChartDot":                true,
	"LineupMarker":                true,
	"PersonCompany":               true,
	"QuestionSchedule":            true,
	"SearchMetadata":              true,
	"SearchType":                  true,
	"UpdateProjectAccessResponse": true,
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
//
// tags and tagToGoField hold the FLATTENED view: the struct's own tagged fields
// after collectStructsAndMarkers, plus the fields promoted through embedded
// types after flattenEmbedded has run over the whole universe. ownFields and
// embeds are the unflattened inputs flattenEmbedded walks; it never reads tags,
// so flattening is order-independent across structs.
type structFields struct {
	tags         map[string]bool
	omitted      map[string]bool
	tagToGoField map[string]string
	declaration  token.Pos

	// ownFields lists this struct's own tagged fields in declaration order.
	ownFields []taggedField
	// embeds lists this struct's anonymous, untagged fields — the ones whose
	// tagged fields are promoted onto this struct by encoding/json.
	embeds []embedRef
	// tagPath maps a PROMOTED tag to the embedded field names traversed to
	// reach it (`{"Base", "Audit"}` for a field promoted through Base's
	// embedded Audit). Own tags are absent. populationTargets expands it into
	// the assignment spellings that count as populating the field.
	tagPath map[string][]string
	// unresolved lists embedded types the checker could not resolve, as
	// human-readable paths ("Task.Meta -> time.Time"). Reported as drift when
	// a pair reaches this struct; see run.
	unresolved []string
}

// populationTargets returns the assignment spellings that count as populating
// tag on this struct, in the dotted-path vocabulary collectAssignedFields
// records (see recordAssignedValue).
//
// A field declared directly on the struct has exactly one spelling: itself.
//
// A PROMOTED field has several, because every embedded field name on its path
// is itself promoted, so each suffix of that path is legal Go:
//
//	w.CreatedAt            — the promoted field
//	w.Audit.CreatedAt      — through the inner embed, itself promoted
//	w.Base.Audit.CreatedAt — fully qualified
//
// and so is assigning any ancestor OPAQUELY — `w.Base = baseFromGenerated(g)`,
// which the walker records as the total marker `Base.*` because it cannot see
// inside the value. Each such ancestor is spelled by any suffix of the path
// that reaches it. An ancestor assigned from a composite literal produces no
// total marker: the walker enumerates that literal's keys instead, so a
// partial literal populates only the fields it names.
func (sf *structFields) populationTargets(tag string) []string {
	goField := sf.tagToGoField[tag]
	if goField == "" {
		return nil
	}
	path := sf.tagPath[tag]
	if len(path) == 0 {
		return []string{goField}
	}
	out := make([]string, 0, len(path)+1+len(path)*(len(path)+1)/2)
	for i := 0; i <= len(path); i++ {
		out = append(out, strings.Join(append(append([]string{}, path[i:]...), goField), "."))
	}
	// Ancestors: every contiguous sub-path path[i:j] names the embed at
	// depth j, so an opaque assignment to it covers everything below.
	for j := 1; j <= len(path); j++ {
		for i := 0; i < j; i++ {
			out = append(out, strings.Join(path[i:j], ".")+".*")
		}
	}
	return out
}

// taggedField is one JSON-tagged field declared directly on a struct.
type taggedField struct {
	tag     string
	goField string
}

// embedRef is one anonymous, untagged field — an embedded type whose tagged
// fields are promoted onto the embedding struct.
type embedRef struct {
	// name is the embedded type's own name: "Base" for `Base` and `*Base`,
	// "Time" for `time.Time`.
	name string
	// qualifier is the package qualifier for a cross-package embed ("time" for
	// `time.Time`), empty for a same-package embed.
	qualifier string
	// display is the source spelling, used in messages ("*Base", "time.Time").
	display string
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

	if err := run(wrapperDir, generatedFile, directDecodePairs, tier3Wrappers, *verbose); err != nil {
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

// run performs the full drift check. directDecode and tier3 are injected
// (rather than read from the package globals) so tests can drive run()
// end-to-end with their own fixtures without dragging in the production
// pair set / tier-3 set, whose generated structs would otherwise have to exist
// in every test fixture. main() passes directDecodePairs and tier3Wrappers.
func run(wrapperDir, generatedFile string, directDecode map[string]string, tier3 map[string]bool, verbose bool) error {
	fset := token.NewFileSet()

	// Parse the generated client.
	genFile, err := parser.ParseFile(fset, generatedFile, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse generated: %w", err)
	}
	genStructs := collectStructsAndMarkers(fset, genFile)
	flattenEmbedded(genStructs, collectTypeDecls(genFile))

	// Parse all wrapper files.
	entries, err := os.ReadDir(wrapperDir)
	if err != nil {
		return fmt.Errorf("read wrapper dir: %w", err)
	}
	wrapperStructs := map[string]*structFields{}
	wrapperTypeDecls := map[string]ast.Expr{}      // every top-level type decl in the wrapper package, for embed resolution
	fromGenPairs := map[string]string{}            // wrapper name -> generated name (derived from *FromGenerated signatures)
	assignedFields := map[string]map[string]bool{} // wrapper name -> set of Go fields written at the wrapper's construction site (tier 1 + tier 3)
	// Tier-3 names sourced from the production tier3Wrappers set. Tests can
	// inject only tier-2 pairs via the directDecode argument; in that case
	// none of them appear in tier3Wrappers and the composite-literal walker
	// is a no-op for them, matching the existing tier-2 semantics.
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
		for k, v := range collectTypeDecls(f) {
			wrapperTypeDecls[k] = v
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
		// Tier-3 wrappers have no *FromGenerated of their own. Collect their
		// assigned fields from inline composite literals (and selector writes
		// against any local path the literal is bound to) anywhere in the
		// non-test wrapper files. Results merge into the same assignedFields
		// map so the population check below is uniform for tier 1 and tier 3.
		for k, fields := range collectCompositeLiteralFields(f, tier3) {
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

	// Every wrapper file is parsed, so embedded types can now be resolved
	// across the package: an embed may name a struct declared in another file.
	flattenEmbedded(wrapperStructs, wrapperTypeDecls)

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

		// An embedded type the checker could not resolve hides an unknown
		// number of promoted fields from both the tag and population checks.
		// Report it instead of comparing against a knowingly-partial tag set —
		// silently dropping embedded fields is the bug this reporting exists
		// to prevent (#599).
		for _, u := range wrap.unresolved {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: unresolvable embedded type in the wrapper: %s. Promoted fields from it are invisible to this check; teach the resolver about it or replace the embed with named fields.", wrapName, genName, u))
		}
		for _, u := range gen.unresolved {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: unresolvable embedded type in the generated struct: %s. Promoted fields from it are invisible to this check; teach the resolver about it.", wrapName, genName, u))
		}

		// The population check runs for tier 1 (assignedFields sourced from
		// the *FromGenerated body) and tier 3 (assignedFields sourced from the
		// inline composite literal walker). Tier 2 is the only path that skips
		// the population check — its wrappers have no in-package literal; the
		// JSON decoder writes straight onto struct tags, so tag presence IS
		// population.
		_, isDirectDecode := directDecode[wrapName]
		isTier2 := isDirectDecode && !tier3[wrapName]
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
			// Tag is declared on the wrapper. For tier-1 and tier-3 pairs,
			// also confirm the construction site actually assigns the field —
			// otherwise a tag-present-but-unassigned field silently stays
			// zero-valued while this check would otherwise pass.
			if !isTier2 {
				totalFieldsPopChecked++
				goField := wrap.tagToGoField[tag]
				// A promoted field accepts more than one spelling at the
				// construction site: the field itself (`w.Name = ...`, legal
				// through promotion) or any embedded struct on the path to it
				// (`w.Base = Base{...}`), which populates everything under it.
				if goField != "" && !assignedAny(assigned, wrap.populationTargets(tag)) {
					unpopulated = append(unpopulated, fmt.Sprintf("%s (field %s)", tag, goField))
				}
			}
		}
		if len(missing) > 0 {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: missing JSON tags %v (add to wrapper struct or mark with `// intentionally-omitted: <tag> - <reason>`)", wrapName, genName, missing))
		}
		if len(unpopulated) > 0 {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: wrapper declares these tags but no %s{...} composite literal or %sFromGenerated body assigns them %v (assign the field at the wrapper's construction site, or mark with `// intentionally-omitted: <tag> - <reason>` if the wrapper field is populated by some other means)", wrapName, genName, wrapName, lowercaseFirst(wrapName), unpopulated))
		}
		if verbose {
			fmt.Printf("  %s ↔ generated.%s (%d generated tags, %d wrapper tags, %d omitted, %d assigned fields, directDecode=%v, tier2=%v)\n",
				wrapName, genName, len(gen.tags), len(wrap.tags), len(wrap.omitted), len(assigned), isDirectDecode, isTier2)
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

	fmt.Printf("Wrapper drift check: %d pairs walked, %d generated fields verified (%d field assignments verified at tier-1 *FromGenerated bodies + tier-3 composite literals)\n", len(pairNames), totalFieldsChecked, totalFieldsPopChecked)

	if len(drift) > 0 {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "=== DRIFT DETECTED ===")
		for _, d := range drift {
			fmt.Fprintln(os.Stderr, "  -", d)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Fix: either propagate the generated field on the wrapper struct + assign it at the wrapper's construction site (the *FromGenerated function for tier 1, or the inline composite literal for tier 3), or add a comment of the form")
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
//
// The returned tag sets cover the struct's OWN tagged fields. Anonymous
// (embedded) fields are recorded in embeds for flattenEmbedded to resolve once
// every file has been parsed — an embedded type may be declared in a different
// file of the same package, so it cannot be resolved here.
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
				tagPath:      map[string][]string{},
				declaration:  ts.Pos(),
			}
			for _, field := range st.Fields.List {
				tag := ""
				if field.Tag != nil {
					tag = extractJSONTag(field.Tag.Value)
				}
				if tag == "" {
					// An anonymous field with no json tag is an embedded type:
					// its own tagged fields are promoted onto this struct by
					// encoding/json. Record it for flattenEmbedded. (A NAMED
					// field with no json tag is invisible on the wire and to
					// this check, as before.)
					if len(field.Names) == 0 {
						sf.embeds = append(sf.embeds, embedRefFromExpr(field.Type))
					}
					continue
				}
				// Record the Go field identifier for this tag. Tagged
				// fields in these structs always have exactly one name;
				// if a field ever had multiple names sharing a tag, the
				// last wins (still correct for membership lookups).
				goField := ""
				for _, fn := range field.Names {
					goField = fn.Name
				}
				if goField != "" && !ast.IsExported(goField) {
					// encoding/json ignores unexported non-anonymous fields
					// whatever their tag says, so this one is not on the wire
					// and must not satisfy a generated tag. (Embedded
					// unexported STRUCT TYPES are a different case: their
					// exported fields do promote, and resolveEmbed handles
					// them.)
					continue
				}
				sf.tags[tag] = true
				if goField == "" {
					// A TAGGED anonymous field is not promoted — encoding/json
					// treats it as an ordinary field under its tag, whose Go
					// field name is the embedded type's name.
					goField = embedRefFromExpr(field.Type).name
				}
				if goField != "" {
					sf.tagToGoField[tag] = goField
				}
				sf.ownFields = append(sf.ownFields, taggedField{tag: tag, goField: goField})
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

// collectTypeDecls returns every top-level `type X <expr>` declaration in the
// file, mapping the type name to its right-hand-side type expression. Struct
// types are included, so the map doubles as the "is this name declared in the
// parsed sources at all?" oracle flattenEmbedded needs: an embedded name absent
// from it is unresolvable (declared elsewhere, or in another package), while an
// embedded name present but non-struct (an interface, a map, a slice) simply
// promotes no JSON-tagged fields.
func collectTypeDecls(f *ast.File) map[string]ast.Expr {
	out := map[string]ast.Expr{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				out[ts.Name.Name] = ts.Type
			}
		}
	}
	return out
}

// embedRefFromExpr decomposes an anonymous field's type expression into an
// embedRef. `Base` -> {name: "Base"}; `*Base` -> {name: "Base", display:
// "*Base"}; `time.Time` -> {name: "Time", qualifier: "time"}. Anything else
// (a generic instantiation, an inline struct type) yields an empty name, which
// flattenEmbedded reports as unresolvable rather than skipping.
func embedRefFromExpr(expr ast.Expr) embedRef {
	ref := embedRef{display: exprDisplay(expr)}
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch e := expr.(type) {
	case *ast.Ident:
		ref.name = e.Name
	case *ast.SelectorExpr:
		if pkg, ok := e.X.(*ast.Ident); ok {
			ref.qualifier = pkg.Name
			ref.name = e.Sel.Name
		}
	}
	return ref
}

// exprDisplay renders a type expression for error messages. It handles the
// shapes an embedded field can take and falls back to a placeholder for
// anything exotic, so messages never come out blank.
func exprDisplay(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprDisplay(e.X)
	case *ast.SelectorExpr:
		if pkg := exprDisplay(e.X); pkg != "" {
			return pkg + "." + e.Sel.Name
		}
		return e.Sel.Name
	case *ast.IndexExpr: // generic instantiation: Base[T]
		return exprDisplay(e.X) + "[…]"
	case *ast.IndexListExpr: // generic instantiation: Base[T, U]
		return exprDisplay(e.X) + "[…]"
	}
	return "<unsupported type expression>"
}

// maxEmbedDepth bounds the promotion walk. The visited-set already terminates
// cycles; this is a second, independent stop so a pathological chain can never
// spin. Real embedding chains in this repo are one level deep.
const maxEmbedDepth = 16

// flattenEmbedded resolves embedded (anonymous) fields for every struct in a
// universe, merging each embedded type's promoted JSON tags into the embedding
// struct's tag set. structs is the struct universe (all structs parsed from one
// package's files, or from client.gen.go); decls is every top-level type
// declaration from the same sources.
//
// The walk is breadth-first by promotion depth so encoding/json's "shallowest
// declaration wins" rule falls out naturally, and two same-depth contributors
// of one tag cancel (encoding/json emits neither). See the package doc for the
// full rule list. Unresolvable embeds are recorded on the embedding struct's
// unresolved list — never skipped — and reported by run when a pair reaches
// them.
func flattenEmbedded(structs map[string]*structFields, decls map[string]ast.Expr) {
	for name, sf := range structs {
		flattenOne(name, sf, structs, decls)
	}
}

// flattenOne performs the breadth-first promotion walk for a single struct.
func flattenOne(rootName string, root *structFields, structs map[string]*structFields, decls map[string]ast.Expr) {
	// A struct reached at depth d contributes its own tagged fields at depth d;
	// path records the embedded field names traversed to reach it, both for
	// population targets and for unresolvable-embed messages.
	type reached struct {
		name string
		sf   *structFields
		path []string
	}

	// Tags declared directly on the root are claimed at depth 0 and shadow
	// anything promoted from below.
	claimed := map[string]bool{}
	for _, f := range root.ownFields {
		claimed[f.tag] = true
	}

	visited := map[string]bool{rootName: true}
	level := []reached{{name: rootName, sf: root}}

	for depth := 1; depth <= maxEmbedDepth && len(level) > 0; depth++ {
		var next []reached
		// reaches counts how many distinct paths arrive at each type AT THIS
		// DEPTH. encoding/json queues such a type once but remembers the count
		// and then duplicates its fields so the annihilation rule sees the
		// conflict (see the nextCount/count pair in encoding/json's
		// typeFields). Deduplicating without counting would promote a tag that
		// two equal-depth paths actually annihilate.
		reaches := map[string]int{}
		for _, parent := range level {
			for _, e := range parent.sf.embeds {
				child, childName, err := resolveEmbed(e, structs, decls)
				if err != "" {
					root.unresolved = append(root.unresolved,
						fmt.Sprintf("%s -> %s (%s)", strings.Join(append([]string{rootName}, parent.path...), "."), e.display, err))
					continue
				}
				if child == nil || visited[childName] {
					// Either the embedded type promotes no JSON-tagged fields
					// (an interface, a map, a slice), or it was already reached
					// at a SHALLOWER depth — encoding/json likewise visits each
					// type once, which also terminates embedding cycles.
					continue
				}
				reaches[childName]++
				if reaches[childName] > 1 {
					// Same type, same depth, second path: counted above, queued
					// once.
					continue
				}
				path := make([]string, 0, len(parent.path)+1)
				path = append(path, parent.path...)
				path = append(path, e.name)
				next = append(next, reached{name: childName, sf: child, path: path})
			}
		}
		// Marking visited only once the whole level is gathered is what keeps
		// the second same-depth path visible to the count above.
		for _, r := range next {
			visited[r.name] = true
		}

		// Gather this depth's contributions before claiming any of them, so
		// same-depth conflicts can be detected.
		type candidate struct {
			field taggedField
			path  []string
		}
		byTag := map[string][]candidate{}
		var order []string
		for _, r := range next {
			for _, f := range r.sf.ownFields {
				if _, seen := byTag[f.tag]; !seen {
					order = append(order, f.tag)
				}
				byTag[f.tag] = append(byTag[f.tag], candidate{field: f, path: r.path})
				if reaches[r.name] > 1 {
					// Reached by more than one path at this depth: every field
					// it contributes is ambiguous with itself, which the
					// same-depth rule below turns into an annihilation.
					byTag[f.tag] = append(byTag[f.tag], candidate{field: f, path: r.path})
				}
			}
		}
		for _, tag := range order {
			if claimed[tag] {
				continue // a shallower declaration wins (or blocked the tag).
			}
			// Claim the tag either way: a same-depth conflict means
			// encoding/json emits nothing for it, and it also stops any
			// deeper occurrence from being promoted.
			claimed[tag] = true
			cands := byTag[tag]
			if len(cands) != 1 {
				continue
			}
			c := cands[0]
			root.tags[tag] = true
			if c.field.goField != "" {
				root.tagToGoField[tag] = c.field.goField
			}
			root.tagPath[tag] = c.path
		}

		level = next
	}

	// Exhausting the depth budget with embeds still unfollowed would hide
	// fields exactly the way #599 did. Report it rather than stopping quietly.
	for _, r := range level {
		if len(r.sf.embeds) > 0 {
			root.unresolved = append(root.unresolved,
				fmt.Sprintf("%s -> %s (embedding chain deeper than %d levels; promotion beyond that was not followed)",
					strings.Join(append([]string{rootName}, r.path...), "."), r.name, maxEmbedDepth))
			break
		}
	}
}

// resolveEmbed resolves one embedded type reference against the parsed
// universe. It returns the embedded struct and the name it resolved to, or a
// non-empty reason string when the type cannot be resolved at all. A nil struct
// with an empty reason means "resolved, but promotes no JSON-tagged fields"
// (an interface, map, slice, func or channel type).
func resolveEmbed(e embedRef, structs map[string]*structFields, decls map[string]ast.Expr) (*structFields, string, string) {
	switch {
	case e.name == "":
		return nil, "", "unsupported embedded type expression"
	case e.qualifier != "":
		return nil, "", "declared in another package, which this check does not parse"
	}
	// Follow `type Alias Base` hops. maxEmbedDepth also bounds this chain, so a
	// self-referential declaration cannot spin.
	name := e.name
	seen := map[string]bool{}
	for hop := 0; hop <= maxEmbedDepth; hop++ {
		if sf, ok := structs[name]; ok {
			return sf, name, ""
		}
		rhs, ok := decls[name]
		if !ok {
			return nil, "", "not declared in the parsed sources"
		}
		if seen[name] {
			return nil, "", "self-referential type declaration"
		}
		seen[name] = true
		switch t := rhs.(type) {
		case *ast.Ident:
			name = t.Name // `type Alias Base` — follow it.
		case *ast.SelectorExpr:
			return nil, "", "defined in terms of another package's type, which this check does not parse"
		default:
			// An interface, map, slice, func, chan or generic type. It is a
			// valid embed and it is NOT absent from the wire — encoding/json
			// treats it as an ordinary field keyed by the Go field name
			// (`struct{ Payload }` emits "Payload") — but it promotes no
			// JSON-TAGGED fields, and tags are this check's entire vocabulary.
			// An untagged field is invisible here whether it is embedded or
			// named (`Plain string` emits "Plain" and is likewise not
			// tracked), so the treatment is uniform and neither side of a pair
			// can drift past it: generated structs are oapi-codegen output,
			// where every field carries a tag, so no generated wire key is
			// ever spelled this way for a wrapper to miss.
			return nil, "", ""
		}
	}
	return nil, "", "type declaration chain is too deep to resolve"
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
						recordAssignedValue(assigned, key.Name, kv.Value)
					}
				}
			case *ast.AssignStmt:
				for i, lhs := range node.Lhs {
					base, path := selectorRootAndPath(lhs)
					if path == "" || !wrapperVars[base] {
						continue
					}
					var value ast.Expr
					if len(node.Lhs) == len(node.Rhs) {
						value = node.Rhs[i]
					}
					recordAssignedValue(assigned, path, value)
				}
			case *ast.IncDecStmt:
				if base, path := selectorRootAndPath(node.X); path != "" && wrapperVars[base] {
					recordAssignedValue(assigned, path, nil)
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

// recordAssignedValue records every population fact implied by assigning value
// to path (a dotted field path relative to the wrapper instance, e.g. "Base" or
// "Base.Audit"). It is the single vocabulary the population check reads through
// populationTargets.
//
// The path itself is always recorded. What the assignment covers BELOW that
// path depends on whether the walker can see inside the value:
//
//   - A struct composite literal (`Base{ID: g.Id}`, `&Base{...}`) is
//     enumerated: each key becomes `path.Key`, recursively. A partial literal
//     therefore covers only the fields it names — assigning an embedded struct
//     with two of its five fields set leaves the other three unpopulated, which
//     is the true wire outcome and must not read as full coverage.
//   - Anything else — a function call, a variable, a slice or map literal — is
//     opaque, and gets the total marker `path.*`: the walker cannot enumerate
//     it, and crediting the whole subtree matches the one-level-nesting
//     doctrine the check already applies to nested wrappers (`c.Creator =
//     &creator` counts, and Person's own fields are verified through Person's
//     own pair).
//
// A nil value means the write had no readable right-hand side (`x.F++`), which
// is treated as opaque.
func recordAssignedValue(assigned map[string]bool, path string, value ast.Expr) {
	if path == "" {
		return
	}
	assigned[path] = true
	if lit := structLiteral(value); lit != nil {
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok {
				recordAssignedValue(assigned, path+"."+key.Name, kv.Value)
			}
		}
		return
	}
	assigned[path+".*"] = true
}

// structLiteral returns the composite literal behind expr when it is a
// bare-identifier-typed struct literal (`Base{...}` or `&Base{...}`), and nil
// otherwise — including for slice, map and qualified-type literals, whose
// contents say nothing about a wrapper's fields.
func structLiteral(expr ast.Expr) *ast.CompositeLit {
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = u.X
	}
	cl, ok := expr.(*ast.CompositeLit)
	if !ok || litTypeName(cl.Type) == "" {
		return nil
	}
	return cl
}

// selectorRootAndPath decomposes an identifier-rooted selector chain into its
// root identifier and the dotted path below it: `t.Base.ID` -> ("t",
// "Base.ID"), `c.Creator` -> ("c", "Creator"). Returns "", "" for anything not
// rooted in a bare identifier, or for a bare identifier with no field selected.
// Keeping the full path (rather than only the final field) is what lets the
// population check recognize a fully-qualified write to a promoted field.
func selectorRootAndPath(expr ast.Expr) (root, path string) {
	full := exprToPath(expr)
	if full == "" {
		return "", ""
	}
	dot := strings.IndexByte(full, '.')
	if dot == -1 {
		return "", ""
	}
	return full[:dot], full[dot+1:]
}

// exprToPath converts an identifier-rooted selector chain into a dotted path
// string. `q` -> "q", `q.Schedule` -> "q.Schedule", `a.b.c` -> "a.b.c". Returns
// "" for anything not rooted in a bare identifier (index expressions, calls,
// type assertions, ...). Used by collectCompositeLiteralFields to key its local
// bindings so subsequent selector writes can be matched.
func exprToPath(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		base := exprToPath(e.X)
		if base == "" {
			return ""
		}
		return base + "." + e.Sel.Name
	}
	return ""
}

// collectCompositeLiteralFields walks every function body in f and, for each
// composite literal whose type is in tier3Wrappers, collects assignment field
// names from two sources, mirroring tier-1's collectAssignedFields contract so
// the population check can treat tier 1 and tier 3 uniformly:
//
//  1. The literal's own KeyValueExpr keys (`&Wrapper{ID: ..., Name: ...}` →
//     ID, Name). Both bare `Wrapper{...}` and pointer `&Wrapper{...}` forms
//     are caught because ast.Inspect descends through the UnaryExpr into the
//     inner CompositeLit, and only the CompositeLit's bare-Ident type matters.
//  2. Selector writes against any local path the literal is bound to. The
//     binding is the LHS path of an assignment whose RHS is the literal —
//     either a bare local (`resp := Wrapper{...}` binds "resp") or a selector
//     chain (`q.Schedule = &Wrapper{...}` binds "q.Schedule"). Subsequent
//     `resp.X = ...` or `q.Schedule.X = ...` writes are then attributed to
//     the wrapper. Selector writes against an unbound path are ignored.
//
// The per-function `bindings` map scopes the binding to one function body, so
// a `resp` local in one function does not contaminate another. This is enough
// for the current corpus, where every tier-3 wrapper is built inside a single
// function. The walker does not require the enclosing function to be a
// *FromGenerated — service methods (LineupService.ListMarkers,
// SearchService.Metadata, PeopleService.UpdateProjectAccess) that build a
// wrapper inline are covered the same way.
//
// Returns wrapper name -> set of assigned Go field names. Wrappers not in
// tier3Wrappers are ignored; if tier3Wrappers is empty the function is a no-op.
func collectCompositeLiteralFields(f *ast.File, tier3 map[string]bool) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if len(tier3) == 0 {
		return out
	}
	addValue := func(wrapper, path string, value ast.Expr) {
		if !tier3[wrapper] {
			return
		}
		set := out[wrapper]
		if set == nil {
			set = map[string]bool{}
			out[wrapper] = set
		}
		recordAssignedValue(set, path, value)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		bindings := map[string]string{} // path -> tier-3 wrapper type bound to it
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				if t := litTypeName(node.Type); tier3[t] {
					for _, elt := range node.Elts {
						kv, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						if key, ok := kv.Key.(*ast.Ident); ok {
							addValue(t, key.Name, kv.Value)
						}
					}
				}
			case *ast.AssignStmt:
				// Record any LHS-path -> tier-3-wrapper binding so subsequent
				// selector writes can be attributed to the wrapper.
				if len(node.Lhs) == len(node.Rhs) {
					for i, rhs := range node.Rhs {
						if t := compositeLitTypeName(rhs); tier3[t] {
							if path := exprToPath(node.Lhs[i]); path != "" {
								bindings[path] = t
							}
						}
					}
				}
				// Attribute selector-target writes to any bound path.
				for i, lhs := range node.Lhs {
					wrapper, rest := boundWrapperAndPath(bindings, lhs)
					if rest == "" {
						continue
					}
					var value ast.Expr
					if len(node.Lhs) == len(node.Rhs) {
						value = node.Rhs[i]
					}
					addValue(wrapper, rest, value)
				}
			case *ast.IncDecStmt:
				if wrapper, rest := boundWrapperAndPath(bindings, node.X); rest != "" {
					addValue(wrapper, rest, nil)
				}
			}
			return true
		})
	}
	return out
}

// boundWrapperAndPath matches a write target against the composite-literal
// bindings recorded in one function body, returning the tier-3 wrapper it
// belongs to and the dotted path of the write RELATIVE to that binding:
// with `resp := Wrapper{...}` bound at "resp", `resp.Base.ID = …` yields
// ("Wrapper", "Base.ID"). The longest matching binding wins, so a nested
// binding is preferred over an enclosing one. Returns "", "" for a write
// against an unbound path.
func boundWrapperAndPath(bindings map[string]string, lhs ast.Expr) (wrapper, path string) {
	full := exprToPath(lhs)
	if full == "" {
		return "", ""
	}
	best := ""
	for prefix := range bindings {
		if strings.HasPrefix(full, prefix+".") && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return "", ""
	}
	return bindings[best], full[len(best)+1:]
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

// assignedAny reports whether the construction site assigned any of the given
// Go field names. The population check in run only consults it for tags whose
// Go field name is known, so an empty target list means unassigned.
func assignedAny(assigned map[string]bool, names []string) bool {
	for _, n := range names {
		if assigned[n] {
			return true
		}
	}
	return false
}

func lowercaseFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
