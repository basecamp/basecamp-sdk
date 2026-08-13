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
// # CAN / CANNOT
//
// The honest boundary of the embedded-field support, and the invariant that
// makes the boundary safe. Read this before adding a rule to either list.
//
// CAN resolve and judge:
//
//   - An embed of a locally-declared struct, by value or pointer, from any
//     file of the same package, at any depth, including through a chain of
//     `type A B` / `type A = B` declarations.
//   - Promotion under encoding/json's rules: shallowest declaration wins,
//     same-depth conflicts annihilate (including a type reached twice at one
//     depth), unexported fields are ignored, a tagged anonymous field is an
//     ordinary field rather than a promotion source.
//   - Population of a promoted field through any legal spelling of it —
//     `w.Field`, `w.Embed.Field`, or an assignment of any embed on its path —
//     with the value classified exhaustively: literals are enumerated key by
//     key, statically zero values credit nothing.
//
// CANNOT, and therefore REPORTS rather than assumes:
//
//   - A type from another package, or a name not declared in the parsed
//     sources. Its fields and its method set are both invisible.
//   - A type whose method set carries MarshalJSON/UnmarshalJSON/MarshalText/
//     UnmarshalText by any route. Promotion redirects the encoder away from
//     every field, so no tag on the struct is trustworthy.
//   - An assignment whose right-hand side is a shape not in the classification
//     above.
//   - An embedded pointer to an unexported type, for direct-decode pairs only:
//     encoding/json cannot allocate it, so the decoder never populates it.
//
// Known to OVER-report, and deliberately left that way, because each fails
// loudly and naming the shape is cheaper than modelling it:
//
//   - A promoted field written through the second path of a diamond
//     (`w.Right.Common.Field` where the walk retained the Left path, which is
//     also the one encoding/json indexes) reads as unpopulated.
//   - A defined type whose name chain reaches a marshaller is refused even
//     where Go would walk its fields, because the closure does not model
//     which declaration form carries a method set.
//   - The decode-allocation report fires for an embed whose fields would
//     have annihilated anyway.
//   - A skipped-segment selector (`w.Base.Field` where an embed sits between)
//     is not among the recognised spellings.
//
// The invariant: ANYTHING UNRECOGNISED IS REPORTED. Never credited, never
// skipped. Both failure modes this walk has actually had — the original #599
// bug and every regression found while fixing it — were a silent assumption
// about something the walker did not understand.
//
// # An honest tally, for whoever adds the next rule
//
// This support was reviewed in ten rounds and grew a rule in almost every one:
// promotion, annihilation, same-depth multiplicity, export visibility, literal
// enumeration, qualified assignment paths, Go-name shadowing, depth-0
// conflicts, method promotion (declared, inherited, interface, aliased, text),
// decode allocation, value classification. The call sites that support covers
// grew exactly once — at the original fix — and remain at ZERO: no wrapper in
// a checked pair embeds a struct, and the gate's output on the real corpus has
// been byte-identical throughout.
//
// The rules that earned their place are the ones that turn a silent assumption
// into a report. A rule that only makes an already-loud report more precise, on
// a shape with no call site, is the kind this file has enough of. Prefer
// widening the reported set over modelling another corner of encoding/json or
// of Go's type identity — those are the type checker's job, and every attempt
// here to do a piece of it produced the next round's finding about itself.
//
// # What the walk will and will not judge
//
// Flattening happens only through embeds the walk can fully vouch for (see
// vouch). Anything else is REPORTED — the struct is then judged on its own
// declarations alone, and any pair that reaches it fails loudly. Two things
// fail to vouch:
//
//   - A type this check does not parse: qualified from another package
//     (`time.Time`), or a name not declared in the parsed sources. It cannot
//     be assumed field-less, and it cannot be assumed method-less either.
//
//   - A type whose method set carries MarshalJSON/UnmarshalJSON — or
//     MarshalText/UnmarshalText, which encoding/json falls back to — by any
//     route: its own declaration, a type it embeds, an interface, a name
//     chain. Promoting such a method makes the EMBEDDING struct implement the
//     interface, so encoding/json calls it and never walks any of these
//     fields; every promoted tag on the struct is then meaningless, not just
//     that embed's. A json tag on the embed does not change this: a tag
//     governs field selection, while method promotion is a language rule that
//     never consults tags.
//
//     "By any route" is computed as one closure over name edges — every
//     `type A B` and `type A = B`, every embedded type, every interface
//     method set — rather than by modelling which routes carry a method set
//     and which do not. Go distinguishes them (a defined type over a struct
//     starts empty; an alias does not), and this walk deliberately does not:
//     that distinction was the source of four consecutive rounds of review
//     findings, each about the previous round's rule. The closure
//     over-approximates, refusing a name chain that reaches a marshaller even
//     where Go would walk the fields, and in exchange the class is closed.
//     Signatures ARE checked, so an unrelated method that merely shares a
//     name does not trigger it — but a signature spelled through a name this
//     check cannot evaluate counts as a match, since the alternative is to
//     certify a wrapper whose marshaller was hidden behind an alias.
//
// This is a whitelist on purpose, and it is where the design stops mirroring
// encoding/json. Field promotion is a bounded, syntactic problem this walk can
// model faithfully. Method promotion and cross-package types are not: they are
// decided by the type checker, travel through interfaces, aliases and other
// packages, and every rule that models one spelling of them invites the next.
// Reporting is the bound. A new shape in that class belongs here as a refusal,
// not as another mirror of the standard library.
//
// The report is deferred to the pair walk, so an unvouched embed on a struct
// outside every pair (today: FlexTime embedding time.Time) costs nothing.
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
	"reflect"
	"regexp"
	"sort"
	"strconv"
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
	// ignoredOwn marks ownFields slots encoding/json never sees — a tagged
	// anonymous field of an unexported non-struct type. Computed once per
	// struct by flattenEmbedded, and skipped at EVERY promotion depth: a field
	// the encoder ignores cannot be promoted onto an embedding struct, nor
	// annihilate or shadow a field the encoder does see.
	ignoredOwn map[int]bool
	// tagPath maps a PROMOTED tag to the embedded field names traversed to
	// reach it (`{"Base", "Audit"}` for a field promoted through Base's
	// embedded Audit). Own tags are absent.
	tagPath map[string][]string
	// tagTargets holds the assignment spellings that count as populating a
	// PROMOTED tag, computed during flattening because the legal spellings
	// depend on Go-name shadowing across the whole promotion tree. Own tags
	// are absent; see populationTargets.
	tagTargets map[string][]string
	// fieldNames lists every field name declared directly on this struct —
	// tagged or not, exported or not, including the embedded fields' own
	// names. Go resolves a selector by NAME with the same shallowest-wins
	// rule, independently of json tags, so this is what decides whether a
	// promoted field can be written by its bare name.
	fieldNames []string
	// taggedEmbeds lists anonymous fields that carry a json tag. They are not
	// promotion sources, but an unexported one is only on the wire if its type
	// is a struct — which needs the universe, so flattenEmbedded settles it.
	taggedEmbeds []taggedEmbed
	// decodeUnsafe lists embeds that break DECODING only: an embedded pointer
	// to an unexported type, which encoding/json refuses to allocate ("cannot
	// set embedded pointer to unexported struct"). Marshalling and in-Go
	// construction are unaffected, so this is reported for tier-2 pairs — the
	// ones whose whole premise is that the decoder populates the tags — and not
	// for wrappers built by a *FromGenerated or a composite literal.
	decodeUnsafe []string
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
	if targets, ok := sf.tagTargets[tag]; ok {
		return targets
	}
	return []string{goField}
}

// promotedTargets enumerates the spellings that populate a promoted field,
// given the depth at which each Go name resolves on the embedding struct
// (nameDepth, with nameConflict marking names that are ambiguous and therefore
// unwritable). path is the chain of embedded field names and goField the
// promoted field.
//
// Every suffix of the path is a candidate spelling, because each embed name is
// itself promoted — but only while the FIRST component of that spelling still
// resolves to this chain. If a shallower field shares that Go name it wins the
// selector, and crediting the spelling would attribute a write that lands on a
// different field: `w.ID = …` on a struct that declares its own ID writes the
// outer one and leaves the promoted `id` zero, with BOTH keys on the wire.
func promotedTargets(path []string, goField string, nameDepth map[string]int, nameConflict map[string]bool) []string {
	resolves := func(name string, depth int) bool {
		d, ok := nameDepth[name]
		return ok && d == depth && !nameConflict[name]
	}
	var out []string
	// The field itself, spelled from each suffix of its path. path[i] is
	// declared at depth i; the field itself at depth len(path).
	for i := 0; i <= len(path); i++ {
		first, firstDepth := goField, len(path)
		if i < len(path) {
			first, firstDepth = path[i], i
		}
		if !resolves(first, firstDepth) {
			continue
		}
		out = append(out, strings.Join(append(append([]string{}, path[i:]...), goField), "."))
	}
	// Ancestors: an opaque assignment to any embed on the path covers
	// everything below it, and each is spelled from any suffix that reaches it.
	for j := 1; j <= len(path); j++ {
		for i := 0; i < j; i++ {
			if !resolves(path[i], i) {
				continue
			}
			out = append(out, strings.Join(path[i:j], ".")+".*")
		}
	}
	return out
}

// taggedEmbed is an anonymous field carrying its own json tag: an ordinary
// field under that tag rather than a promotion source.
type taggedEmbed struct {
	tag string
	ref embedRef
	// ownIndex is this field's slot in ownFields, so flattenEmbedded can take
	// it out of the depth-0 dominance candidates when encoding/json ignores
	// the field outright.
	ownIndex int
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
	// pointer is true for `*Base`. It matters for decoding: encoding/json
	// cannot allocate an embedded pointer to an UNEXPORTED type, so a
	// direct-decode wrapper with one never gets those fields populated.
	pointer bool
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
	genJSONMethods, genIfaceEmbeds := collectJSONMethodTypes(genFile)
	closeJSONMethodTypes(genJSONMethods, genIfaceEmbeds)
	flattenEmbedded(genStructs, collectTypeDecls(genFile), genJSONMethods)

	// Parse all wrapper files.
	entries, err := os.ReadDir(wrapperDir)
	if err != nil {
		return fmt.Errorf("read wrapper dir: %w", err)
	}
	wrapperStructs := map[string]*structFields{}
	wrapperTypeDecls := map[string]typeDecl{}      // every top-level type decl in the wrapper package, for embed resolution
	wrapperJSONMethods := map[string]bool{}        // types declaring MarshalJSON/UnmarshalJSON, whose promotion invalidates flattening
	wrapperIfaceEmbeds := map[string][]string{}    // interface -> embedded interfaces, closed once every file is in
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
		fileJSONMethods, fileIfaceEmbeds := collectJSONMethodTypes(f)
		for k := range fileJSONMethods {
			wrapperJSONMethods[k] = true
		}
		for k, v := range fileIfaceEmbeds {
			wrapperIfaceEmbeds[k] = append(wrapperIfaceEmbeds[k], v...)
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
	// Closed across the whole package: an interface may embed one declared in
	// another file, and the promotion is just as real.
	closeJSONMethodTypes(wrapperJSONMethods, wrapperIfaceEmbeds)
	flattenEmbedded(wrapperStructs, wrapperTypeDecls, wrapperJSONMethods)

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
	embeddedPairs := 0         // pairs whose wrapper actually promotes fields through an embed
	promotedFieldsChecked := 0 // generated tags satisfied by a promoted (not directly declared) field
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

		// Tiering first: it decides which reports apply. Tier 2 is the only
		// path that skips the population check — its wrappers have no
		// in-package literal; the JSON decoder writes straight onto struct
		// tags, so tag presence IS population.
		_, isDirectDecode := directDecode[wrapName]
		isTier2 := isDirectDecode && !tier3[wrapName]
		assigned := assignedFields[wrapName]

		// An embedded type the checker could not resolve hides an unknown
		// number of promoted fields from both the tag and population checks.
		// Report it instead of comparing against a knowingly-partial tag set —
		// silently dropping embedded fields is the bug this reporting exists
		// to prevent (#599).
		for _, u := range wrap.unresolved {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: unresolvable embedded type in the wrapper: %s. Promoted fields from it are invisible to this check; teach the resolver about it or replace the embed with named fields.", wrapName, genName, u))
		}
		if isTier2 {
			for _, u := range wrap.decodeUnsafe {
				drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: %s. This pair is direct-decode, where tag presence IS population, so those fields are silently absent.", wrapName, genName, u))
			}
		}
		// A value shape the walker could not interpret was recorded rather
		// than credited. Report it: an uninterpreted assignment is exactly
		// where a silently-credited field would hide.
		if !isTier2 {
			var unrecognized []string
			for spelling := range assigned {
				if strings.HasPrefix(spelling, unrecognizedPrefix) {
					unrecognized = append(unrecognized, strings.TrimPrefix(spelling, unrecognizedPrefix))
				}
			}
			sort.Strings(unrecognized)
			for _, u := range unrecognized {
				drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: the value assigned to %s is a shape this walker does not interpret, so it credits nothing. Assign the field with a composite literal, a call or a variable, or mark the tags with `// intentionally-omitted: <tag> - <reason>`.", wrapName, genName, u))
			}
		}
		for _, u := range gen.unresolved {
			drift = append(drift, fmt.Sprintf("%s ↔ generated.%s: unresolvable embedded type in the generated struct: %s. Promoted fields from it are invisible to this check; teach the resolver about it.", wrapName, genName, u))
		}

		// Walk every JSON tag declared on the generated struct.
		tags := make([]string, 0, len(gen.tags))
		for t := range gen.tags {
			tags = append(tags, t)
		}
		sort.Strings(tags)

		if len(wrap.tagPath) > 0 {
			embeddedPairs++
		}
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
			if len(wrap.tagPath[tag]) > 0 {
				promotedFieldsChecked++
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

	// Say what the verdict covers. A clean run means "no drift in the shapes
	// this walker resolves", and when no pair embeds anything at all — which
	// is the case in this repo today — the embedded-field machinery verified
	// nothing and the line should not let a reader think otherwise.
	switch {
	case embeddedPairs == 0:
		fmt.Println("  Embedded-field promotion: no pair embeds a struct, so nothing above was verified through it.")
	default:
		fmt.Printf("  Embedded-field promotion: %d of %d pairs embed a struct; %d promoted fields verified through it.\n", embeddedPairs, len(pairNames), promotedFieldsChecked)
	}
	fmt.Println("  Scope: shapes this walker resolves. Unrecognised embeds and assignment shapes are reported as drift, never assumed away — see the CAN/CANNOT list in this file's header.")

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
				tagTargets:   map[string][]string{},
				declaration:  ts.Pos(),
			}
			for _, field := range st.Fields.List {
				// Go resolves selectors by field NAME regardless of tags, so
				// every name counts for shadowing — including untagged and
				// unexported ones, and the embedded fields' own names.
				if len(field.Names) == 0 {
					if n := embedRefFromExpr(field.Type).name; n != "" {
						sf.fieldNames = append(sf.fieldNames, n)
					}
				}
				for _, fn := range field.Names {
					sf.fieldNames = append(sf.fieldNames, fn.Name)
				}
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
				// A grouped declaration (`A, B string ` + "`json:\"x\"`" + `) is TWO
				// fields to encoding/json, which then annihilates them for
				// sharing a tag at one depth. Record one candidate per name so
				// the dominance rules see the conflict.
				if len(field.Names) > 1 {
					for _, fn := range field.Names {
						if !ast.IsExported(fn.Name) {
							continue
						}
						sf.tags[tag] = true
						sf.tagToGoField[tag] = fn.Name
						sf.ownFields = append(sf.ownFields, taggedField{tag: tag, goField: fn.Name})
					}
					continue
				}
				// Record the Go field identifier for this tag.
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
					// field name is the embedded type's name. Whether it is on
					// the wire at all depends on the type when the type name is
					// unexported, which only flattenEmbedded can settle.
					ref := embedRefFromExpr(field.Type)
					goField = ref.name
					sf.taggedEmbeds = append(sf.taggedEmbeds, taggedEmbed{tag: tag, ref: ref, ownIndex: len(sf.ownFields)})
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

// collectJSONMethodTypes returns the names of types in this file that declare
// MarshalJSON or UnmarshalJSON, on either a value or a pointer receiver.
// Embedding such a type promotes the method, which makes the EMBEDDING type
// implement json.Marshaler / json.Unmarshaler too — so encoding/json calls that
// method for the whole struct instead of walking its fields, and no comparison
// of promoted tags describes the wire shape any more.
func collectJSONMethodTypes(f *ast.File) (map[string]bool, map[string][]string) {
	out := map[string]bool{}
	// Methods declared with a receiver.
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 {
			continue
		}
		if !isJSONMethod(fd.Name.Name, fd.Type) {
			continue
		}
		// `func (m (M)) MarshalJSON()` is legal, and reading the receiver as
		// unrecognized would leave M out of the method set — a silent miss, so
		// the parentheses come off even though nobody writes them.
		recv := unparen(fd.Recv.List[0].Type)
		if star, ok := recv.(*ast.StarExpr); ok {
			recv = unparen(star.X)
		}
		if id, ok := recv.(*ast.Ident); ok {
			out[id.Name] = true
		}
	}
	// Interfaces carrying the method in their method set. Embedding one
	// promotes it just as embedding a struct with the method does, and the
	// interface itself has no field to give the walk a second chance.
	ifaceEmbeds := map[string][]string{}
	for name, decl := range collectTypeDecls(f) {
		// `type A = M` and `type A M` both make A a name for M's shape, and
		// the first also for its method set. Both become edges: the closure is
		// deliberately conservative here, because distinguishing them means
		// tracking which declaration form each hop used, and every rule of
		// that kind has produced the next round's finding about itself. A
		// defined type over a method-bearing struct is therefore refused
		// though Go would walk it — one over-report, in exchange for closing
		// the alias/defined surface entirely.
		if id, ok := decl.expr.(*ast.Ident); ok {
			ifaceEmbeds[name] = append(ifaceEmbeds[name], id.Name)
			continue
		}
		it, ok := decl.expr.(*ast.InterfaceType)
		if !ok {
			continue
		}
		for _, m := range it.Methods.List {
			if len(m.Names) == 0 {
				// An embedded interface contributes its whole method set.
				if id, ok := m.Type.(*ast.Ident); ok {
					ifaceEmbeds[name] = append(ifaceEmbeds[name], id.Name)
					continue
				}
				// Qualified (`json.Marshaler`) or otherwise unreadable: this
				// check does not parse the other package, so the method set is
				// unknown. Unknown counts as carrying — the whole point of the
				// vouching rule is that "cannot tell" is answered with a
				// refusal, not an assumption.
				out[name] = true
				continue
			}
			ft, _ := m.Type.(*ast.FuncType)
			for _, n := range m.Names {
				if isJSONMethod(n.Name, ft) {
					out[name] = true
				}
			}
		}
	}
	return out, ifaceEmbeds
}

// closeJSONMethodTypes propagates the method sets through interface embedding
// until stable. It runs over the MERGED package, not one file: the interface
// declaring MarshalJSON and the interface embedding it routinely live in
// different files, and a per-file closure would mark the first and never the
// second. The loop is bounded by the graph size, so a cycle cannot spin.
func closeJSONMethodTypes(direct map[string]bool, ifaceEmbeds map[string][]string) {
	for range ifaceEmbeds {
		changed := false
		for name, embedded := range ifaceEmbeds {
			if direct[name] {
				continue
			}
			for _, e := range embedded {
				if direct[e] {
					direct[name] = true
					changed = true
				}
			}
		}
		if !changed {
			return
		}
	}
}

// vouch is the single gate on whether the walk may flatten through an embed. It
// returns "" when the embed is safe to walk, or the reason it is not.
//
// This is deliberately a WHITELIST, and it is where the design stops chasing
// encoding/json's tail. Two mechanisms decide an embedding struct's wire shape
// that source text cannot settle: promoted METHODS (a promoted MarshalJSON
// redirects the encoder away from every field, and method sets travel through
// interfaces, aliases and other packages) and types this check never parses.
// Each is an unbounded surface for a static walk, and each new rule modelling
// one spelling of it invites the next. So anything not plainly vouched for —
// a type from another package, a name not declared in the parsed sources, a
// type whose method set carries MarshalJSON/UnmarshalJSON by any route — is
// REPORTED rather than modelled, and the struct is judged on its own
// declarations alone.
//
// The method check keys on the embed's own name as well as the resolved
// struct: an embedded interface carries the method in its method set and
// resolves to no struct at all. A name reached through declaration hops counts
// only when the method set travelled with it (every hop an alias), since a
// defined type starts with an empty method set.
func vouch(e embedRef, childName string, methodsTravel bool, resolveErr string, jsonMethods map[string]bool) string {
	if jsonMethods[e.name] || (methodsTravel && childName != "" && jsonMethods[childName]) {
		return "the embedded type's method set carries MarshalJSON/UnmarshalJSON, which the embedding struct promotes; encoding/json then calls it instead of walking any of these fields, so no promoted tag here is trustworthy"
	}
	if resolveErr != "" {
		// An unresolvable type may also be a marshaller — time.Time is one —
		// so this cannot be treated as merely "fields we cannot see".
		return resolveErr + "; its fields AND any custom JSON methods it would promote are invisible here"
	}
	return ""
}

// isJSONMethod reports whether a method is one whose promotion redirects
// encoding/json away from a struct's fields — which takes the right NAME and
// the right SIGNATURE, since only a method that satisfies json.Marshaler or
// json.Unmarshaler is ever called. A `MarshalJSON(int) []byte` implements
// neither, and refusing to judge a struct that embeds its type would
// manufacture drift on a wrapper the encoder walks normally.
//
// Signatures are compared by rendered type expression, which is exact for the
// shapes these two interfaces require:
//
//	MarshalJSON() ([]byte, error)
//	UnmarshalJSON([]byte) error
func isJSONMethod(name string, ft *ast.FuncType) bool {
	if ft == nil {
		return false
	}
	switch name {
	case "MarshalJSON":
		return signatureMatches(ft, nil, []string{"[]byte", "error"})
	case "UnmarshalJSON":
		return signatureMatches(ft, []string{"[]byte"}, []string{"error"})
	case "MarshalText", "UnmarshalText":
		// encoding/json falls back to encoding.TextMarshaler when a type does
		// not implement json.Marshaler, encoding the value as a JSON string —
		// so promoting one redirects the encoder away from the fields just as
		// surely.
		if name == "MarshalText" {
			return signatureMatches(ft, nil, []string{"[]byte", "error"})
		}
		return signatureMatches(ft, []string{"[]byte"}, []string{"error"})
	}
	return false
}

// signatureMatches is signatureIs plus a deliberate bias: a component this
// check cannot resolve to a builtin spelling — a local name like
// `type Bytes = []byte`, or any imported type — counts as a MATCH rather than
// a miss. The alternative is to certify a wrapper whose promoted marshaller
// was spelled through an alias, which is the silent direction. A signature
// built entirely from resolvable spellings is compared exactly, so
// `MarshalJSON(int) []byte` is still correctly not a marshaller.
func signatureMatches(ft *ast.FuncType, params, results []string) bool {
	if signatureIs(ft, params, results) {
		return true
	}
	return signatureHasUnresolvableComponent(ft)
}

// signatureHasUnresolvableComponent reports whether any parameter or result
// type is spelled with a name this check cannot evaluate — anything that is
// not a builtin, a slice of one, or an error.
func signatureHasUnresolvableComponent(ft *ast.FuncType) bool {
	resolvable := func(expr ast.Expr) bool {
		switch e := expr.(type) {
		case *ast.Ident:
			return predeclaredTypes[e.Name]
		case *ast.ArrayType:
			if e.Len != nil {
				return false
			}
			if id, ok := e.Elt.(*ast.Ident); ok {
				return predeclaredTypes[id.Name]
			}
		}
		return false
	}
	for _, fl := range []*ast.FieldList{ft.Params, ft.Results} {
		if fl == nil {
			continue
		}
		for _, f := range fl.List {
			if !resolvable(f.Type) {
				return true
			}
		}
	}
	return false
}

// signatureIs compares a function type's parameter and result types against
// rendered type expressions, treating each declared name in a grouped
// parameter as its own entry.
func signatureIs(ft *ast.FuncType, params, results []string) bool {
	return fieldListIs(ft.Params, params) && fieldListIs(ft.Results, results)
}

func fieldListIs(fl *ast.FieldList, want []string) bool {
	var got []string
	if fl != nil {
		for _, f := range fl.List {
			n := 1
			if len(f.Names) > 0 {
				n = len(f.Names)
			}
			for i := 0; i < n; i++ {
				got = append(got, exprDisplay(f.Type))
			}
		}
	}
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// collectTypeDecls returns every top-level `type X <expr>` declaration in the
// file, mapping the type name to its right-hand-side type expression. Struct
// types are included, so the map doubles as the "is this name declared in the
// parsed sources at all?" oracle flattenEmbedded needs: an embedded name absent
// from it is unresolvable (declared elsewhere, or in another package), while an
// embedded name present but non-struct (an interface, a map, a slice) simply
// promotes no JSON-tagged fields.
func collectTypeDecls(f *ast.File) map[string]typeDecl {
	out := map[string]typeDecl{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				// `type Alias = (Base)` is legal, and the parentheses are not
				// part of the type. Unwrapping here means every consumer —
				// the name-edge closure, embed resolution, the interface scan
				// — sees the same shape without each having to remember.
				out[ts.Name.Name] = typeDecl{expr: unparen(ts.Type), alias: ts.Assign.IsValid()}
			}
		}
	}
	return out
}

// typeDecl is one `type X …` declaration. alias distinguishes `type X = Y`,
// which is the same type as Y and therefore has its methods, from `type X Y`,
// which takes Y's underlying structure but starts with an EMPTY method set.
// That distinction decides whether embedding X promotes Y's MarshalJSON.
type typeDecl struct {
	expr  ast.Expr
	alias bool
}

// embedRefFromExpr decomposes an anonymous field's type expression into an
// embedRef. `Base` -> {name: "Base"}; `*Base` -> {name: "Base", display:
// "*Base"}; `time.Time` -> {name: "Time", qualifier: "time"}. Anything else
// (a generic instantiation, an inline struct type) yields an empty name, which
// flattenEmbedded reports as unresolvable rather than skipping.
func embedRefFromExpr(expr ast.Expr) embedRef {
	ref := embedRef{display: exprDisplay(expr)}
	if star, ok := expr.(*ast.StarExpr); ok {
		ref.pointer = true
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
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprDisplay(e.Elt)
		}
	}
	return "<unsupported type expression>"
}

// predeclaredTypes are Go's builtin type names. A declaration chain ending in
// one is resolved, not unresolvable: it contributes no fields and no method set
// of its own, so it needs neither a refusal nor a walk.
var predeclaredTypes = map[string]bool{
	"any": true, "bool": true, "byte": true, "comparable": true,
	"complex64": true, "complex128": true, "error": true, "float32": true,
	"float64": true, "int": true, "int8": true, "int16": true, "int32": true,
	"int64": true, "rune": true, "string": true, "uint": true, "uint8": true,
	"uint16": true, "uint32": true, "uint64": true, "uintptr": true,
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
func flattenEmbedded(structs map[string]*structFields, decls map[string]typeDecl, jsonMethods map[string]bool) {
	// Settle which fields the encoder ignores before any promotion runs, for
	// every struct rather than just the roots: the walk reads a descendant's
	// ownFields directly, so filtering only at depth 0 would promote a tag out
	// of a field encoding/json never emits.
	// A struct promotes the methods of everything it embeds, so "carries a
	// custom JSON method" is transitive through struct embedding just as it is
	// through interface embedding — and a json TAG on the embed does not stop
	// it. Close over that before any flattening, so an embed of a struct that
	// merely inherits a marshaller is refused as readily as one that declares
	// it. Unresolvable embeds count as carrying: an unparsed type can be a
	// marshaller (time.Time is), and "cannot tell" is answered with a refusal.
	for {
		changed := false
		for name, sf := range structs {
			if jsonMethods[name] {
				continue
			}
			refs := make([]embedRef, 0, len(sf.embeds)+len(sf.taggedEmbeds))
			refs = append(refs, sf.embeds...)
			for _, te := range sf.taggedEmbeds {
				refs = append(refs, te.ref)
			}
			for _, ref := range refs {
				_, childName, methodsTravel, err := resolveEmbedFull(ref, structs, decls)
				if err != "" || jsonMethods[ref.name] || (methodsTravel && childName != "" && jsonMethods[childName]) {
					jsonMethods[name] = true
					changed = true
					break
				}
			}
		}
		if !changed {
			break
		}
	}

	for _, sf := range structs {
		sf.ignoredOwn = map[int]bool{}
		for _, te := range sf.taggedEmbeds {
			if te.ref.name == "" || ast.IsExported(te.ref.name) {
				continue
			}
			if child, _, err := resolveEmbed(te.ref, structs, decls); err == "" && child != nil {
				continue
			}
			sf.ignoredOwn[te.ownIndex] = true
		}
	}
	for name, sf := range structs {
		flattenOne(name, sf, structs, decls, jsonMethods)
	}
}

// flattenOne performs the breadth-first promotion walk for a single struct.
func flattenOne(rootName string, root *structFields, structs map[string]*structFields, decls map[string]typeDecl, jsonMethods map[string]bool) {
	// A struct reached at depth d contributes its own tagged fields at depth d;
	// path records the embedded field names traversed to reach it, both for
	// population targets and for unresolvable-embed messages.
	type reached struct {
		name string
		sf   *structFields
		path []string
	}

	// Tags declared directly on the root are claimed at depth 0 and shadow
	// anything promoted from below — EXCEPT where the root declares one tag
	// twice, which encoding/json annihilates exactly like a same-depth
	// promotion conflict. The tag is then on no field's wire output, so it
	// stays claimed (blocking deeper promotion) but is not present.
	// Rebuild the own-tag view from the surviving fields — deleting the ignored
	// field's tag outright would take a REAL field's tag with it when the two
	// collide, which is the phantom drift this ordering exists to avoid. A tag
	// whose only carrier was an ignored field is simply absent, and stays
	// unclaimed so a promotion can still supply it.
	claimed := map[string]bool{}
	ownCount := map[string]int{}
	root.tags = map[string]bool{}
	root.tagToGoField = map[string]string{}
	for i, f := range root.ownFields {
		if root.ignoredOwn[i] {
			continue
		}
		ownCount[f.tag]++
		claimed[f.tag] = true
		root.tags[f.tag] = true
		if f.goField != "" {
			root.tagToGoField[f.tag] = f.goField
		}
	}
	// Two surviving fields sharing a tag annihilate: encoding/json emits
	// neither, so the tag is not on the wire — but it stays claimed, which
	// blocks a deeper promotion from filling the vacancy, as dominantField
	// gives up on the tie rather than falling through.
	for tag, n := range ownCount {
		if n > 1 {
			delete(root.tags, tag)
			delete(root.tagToGoField, tag)
		}
	}

	// The own-only view, kept so a discovery that invalidates promotion (a
	// promoted custom marshaller) can drop every promoted tag and leave the
	// struct judged on its own declarations alone.
	ownTags := make(map[string]bool, len(root.tags))
	for t := range root.tags {
		ownTags[t] = true
	}
	ownTagToGoField := make(map[string]string, len(root.tagToGoField))
	for t, f := range root.tagToGoField {
		ownTagToGoField[t] = f
	}
	discardPromotions := func() {
		root.tags = ownTags
		root.tagToGoField = ownTagToGoField
		root.tagPath = map[string][]string{}
		root.tagTargets = map[string][]string{}
	}

	// A json tag stops an anonymous field from promoting its FIELDS. It does
	// nothing to method promotion — that is a language rule which never
	// consults tags — so a tagged embed is vouched for on exactly the same
	// terms as an untagged one.
	for _, te := range root.taggedEmbeds {
		child, childName, methodsTravel, err := resolveEmbedFull(te.ref, structs, decls)
		if child != nil && te.ref.pointer && !ast.IsExported(te.ref.name) {
			// Same allocation problem as an untagged one: the tag changes which
			// KEY the field appears under, not whether the decoder can create
			// it. Only for a STRUCT, though — encoding/json ignores an
			// anonymous unexported non-struct field outright, so there is
			// nothing it failed to allocate.
			root.decodeUnsafe = append(root.decodeUnsafe,
				fmt.Sprintf("%s -> %s (encoding/json cannot allocate an embedded pointer to an unexported type, so a decoder never populates it)", rootName, te.ref.display))
		}
		if reason := vouch(te.ref, childName, methodsTravel, err, jsonMethods); reason != "" {
			root.unresolved = append(root.unresolved, fmt.Sprintf("%s -> %s (%s)", rootName, te.ref.display, reason))
			discardPromotions()
			return
		}
	}

	// Go-name resolution, tracked alongside the tag walk: nameDepth records the
	// shallowest depth at which each field NAME appears and nameConflict the
	// names two same-depth structs both declare, which makes the selector
	// ambiguous. promotedTargets needs both to know which spellings of a
	// promoted field actually reach it.
	nameDepth := map[string]int{}
	nameConflict := map[string]bool{}
	recordNames := func(names []string, depth int, seenThisDepth map[string]bool) {
		for _, n := range names {
			if d, ok := nameDepth[n]; ok {
				if d == depth && seenThisDepth[n] {
					nameConflict[n] = true
				}
				continue
			}
			nameDepth[n] = depth
			seenThisDepth[n] = true
		}
	}
	recordNames(root.fieldNames, 0, map[string]bool{})

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
				child, childName, methodsTravel, err := resolveEmbedFull(e, structs, decls)
				if reason := vouch(e, childName, methodsTravel, err, jsonMethods); reason != "" {
					root.unresolved = append(root.unresolved,
						fmt.Sprintf("%s -> %s (%s)", strings.Join(append([]string{rootName}, parent.path...), "."), e.display, reason))
					discardPromotions()
					return
				}
				if child != nil && e.pointer && !ast.IsExported(e.name) {
					// A struct only: an anonymous pointer to an unexported
					// NON-struct is ignored by encoding/json rather than
					// allocated, so there is no field it failed to populate.
					root.decodeUnsafe = append(root.decodeUnsafe,
						fmt.Sprintf("%s -> %s (encoding/json cannot allocate an embedded pointer to an unexported type, so a decoder never populates its promoted fields)",
							strings.Join(append([]string{rootName}, parent.path...), "."), e.display))
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

		// Names first: a spelling's validity depends on what resolves at this
		// depth and shallower, all of which is known once this level is built.
		seenThisDepth := map[string]bool{}
		for _, r := range next {
			recordNames(r.sf.fieldNames, depth, seenThisDepth)
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
			for i, f := range r.sf.ownFields {
				if r.sf.ignoredOwn[i] {
					continue
				}
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
			root.tagTargets[tag] = promotedTargets(c.path, c.field.goField, nameDepth, nameConflict)
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
func resolveEmbed(e embedRef, structs map[string]*structFields, decls map[string]typeDecl) (sf *structFields, name string, reason string) {
	sf, name, _, reason = resolveEmbedFull(e, structs, decls)
	return sf, name, reason
}

// resolveEmbedFull is resolveEmbed plus methodsTravel: whether the resolved
// type's METHOD SET reaches the embedding struct. Every hop must be an alias
// for that to hold — `type Safe Stamp` has Stamp's fields but none of its
// methods, so embedding Safe does not make the outer type a json.Marshaler
// even when Stamp is one.
func resolveEmbedFull(e embedRef, structs map[string]*structFields, decls map[string]typeDecl) (*structFields, string, bool, string) {
	switch {
	case e.name == "":
		return nil, "", false, "unsupported embedded type expression"
	case e.qualifier != "":
		return nil, "", false, "declared in another package, which this check does not parse"
	}
	// Follow `type Alias Base` hops. maxEmbedDepth also bounds this chain, so a
	// self-referential declaration cannot spin.
	name := e.name
	methodsTravel := true
	seen := map[string]bool{}
	for hop := 0; hop <= maxEmbedDepth; hop++ {
		if sf, ok := structs[name]; ok {
			return sf, name, methodsTravel, ""
		}
		decl, ok := decls[name]
		if !ok {
			if predeclaredTypes[name] {
				// A defined type over a builtin (`type hidden string`). It is
				// fully accounted for: no fields to promote, and no method set
				// of its own — any methods would be declared locally and are
				// already in jsonMethods under the defining name.
				return nil, name, methodsTravel, ""
			}
			return nil, "", methodsTravel, "not declared in the parsed sources"
		}
		if seen[name] {
			return nil, "", methodsTravel, "self-referential type declaration"
		}
		seen[name] = true
		switch t := decl.expr.(type) {
		case *ast.Ident:
			// `type Alias = Base` keeps Base's method set; `type Alias Base`
			// starts empty.
			if !decl.alias {
				methodsTravel = false
			}
			name = t.Name
		case *ast.SelectorExpr:
			return nil, "", methodsTravel, "defined in terms of another package's type, which this check does not parse"
		case *ast.IndexExpr, *ast.IndexListExpr:
			// `type A G[int]` keeps G's fields, and an alias to one keeps its
			// method set too. Modelling instantiation is type-checker work;
			// under this file's invariant the answer is to report, not to
			// treat it as fieldless — which would drop its tags from the
			// GENERATED side as well and let a wrapper omit them silently.
			return nil, "", methodsTravel, "declared over an instantiated generic type, whose fields and method set this check does not model"
		default:
			// (fallthrough comment below applies to the default branch)
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
			//
			// The NAME is still returned even though there is no struct: an
			// embedded interface promotes its method set, and vouch has to be
			// able to look that name up.
			//
			// An interface is also the one target where the defined-type rule
			// does NOT apply: `type Safe Marshaler` is still an interface with
			// Marshaler's methods, unlike `type Safe SomeStruct`, which starts
			// with an empty method set.
			if _, isIface := decl.expr.(*ast.InterfaceType); isIface {
				methodsTravel = true
			}
			return nil, name, methodsTravel, ""
		}
	}
	return nil, "", methodsTravel, "type declaration chain is too deep to resolve"
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

// unrecognizedPrefix marks a value shape the walker could not interpret. It is
// stored in the same set as the dotted assignment paths, prefixed with a byte
// that cannot begin a Go identifier, so it can never collide with a real
// spelling. run() turns each into drift for the pair that reaches it.
const unrecognizedPrefix = "?"

// recordAssignedValue records every population fact implied by assigning value
// to path (a dotted field path relative to the wrapper instance, e.g. "Base" or
// "Base.Audit"). It is the single vocabulary the population check reads through
// populationTargets.
//
// The path itself is always recorded. What the assignment covers BELOW it is
// decided by an EXHAUSTIVE classification of the value, because the default for
// an unrecognized shape is the thing this whole check exists to prevent:
// silently crediting fields nobody assigned. The classification is therefore a
// whitelist, and its last branch reports rather than assumes.
//
//   - Statically zero — `nil`, `new(T)`, `(*T)(nil)`, an empty literal —
//     covers NOTHING. None of them carry a value in.
//   - A struct literal with keys, written with its type (`Base{ID: g.Id}`) or
//     with the type elided inside another literal (`Base{Audit: {At: g.At}}`),
//     is ENUMERATED key by key, recursively. A partial literal covers only the
//     fields it names.
//   - A positional struct literal lists every direct field, so it covers the
//     subtree — unless an element is statically zero, which withdraws the
//     claim, since the walker cannot tell which field that element lands on.
//   - A call, a variable, a field selector, a dereference, an index or a type
//     assertion delivers a whole value of the field's type, so it covers the
//     subtree. This is the one-level-nesting doctrine the check has always
//     applied (`c.Creator = &creator` counts, and Person's own fields are
//     verified through Person's own pair).
//   - ANYTHING ELSE is recorded as unrecognized and covers nothing. The pair
//     that reaches it reports, and the shape gets read by a person instead of
//     being credited by a walker that did not understand it.
//
// A nil value means the write had no readable right-hand side (`x.F++`), which
// covers the subtree: the field was written, and there is nothing to look into.
func recordAssignedValue(assigned map[string]bool, path string, value ast.Expr) {
	if path == "" {
		return
	}
	assigned[path] = true
	if value == nil {
		assigned[path+".*"] = true
		return
	}
	if zeroValued(value) {
		return
	}
	if lit := literalToEnumerate(value); lit != nil {
		keyed := false
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyed = true
			key, ok := kv.Key.(*ast.Ident)
			if !ok {
				// A struct literal's keys are field names; anything else is a
				// shape this walker does not model.
				assigned[unrecognizedPrefix+path] = true
				continue
			}
			recordAssignedValue(assigned, path+"."+key.Name, kv.Value)
		}
		if keyed || len(lit.Elts) == 0 {
			return
		}
		// Positional: every direct field is listed, so the subtree is covered
		// unless one of the elements is itself statically zero.
		for _, elt := range lit.Elts {
			if zeroValued(elt) {
				return
			}
		}
		assigned[path+".*"] = true
		return
	}
	if deliversWholeValue(value) {
		assigned[path+".*"] = true
		return
	}
	assigned[unrecognizedPrefix+path] = true
}

// zeroValued reports whether an assigned value is statically known to carry no
// data: the nil identifier, `new(T)`, a conversion of nil such as
// `(*Base)(nil)`, or an empty composite literal. Crediting any of them with
// subtree coverage would pass a converter that emits none of the fields it
// claims.
func zeroValued(expr ast.Expr) bool {
	switch e := unparen(expr).(type) {
	case *ast.Ident:
		return e.Name == "nil"
	case *ast.CompositeLit:
		// An empty literal — `Audit{}`, or `{}` elided inside another —
		// contributes nothing.
		return len(e.Elts) == 0
	case *ast.UnaryExpr:
		return e.Op == token.AND && zeroValued(e.X)
	case *ast.CallExpr:
		// new(T) — the builtin, not a method named new.
		if id, ok := unparen(e.Fun).(*ast.Ident); ok && id.Name == "new" {
			return true
		}
		// A pointer conversion of nil: (*Base)(nil). The parenthesized star is
		// what makes this unambiguous — `baseFrom(nil)` is an ordinary call
		// that may well populate everything, and only the type checker could
		// tell `Base(nil)` from it, so neither is claimed here.
		if _, isStar := unparenOnce(e.Fun).(*ast.StarExpr); isStar && len(e.Args) == 1 {
			if id, ok := unparen(e.Args[0]).(*ast.Ident); ok && id.Name == "nil" {
				return true
			}
		}
	}
	return false
}

// unparen strips redundant parentheses: `(Base{...})` is the same value as
// `Base{...}`, and classifying the parenthesized form differently would credit
// or refuse a subtree on punctuation alone.
func unparen(expr ast.Expr) ast.Expr {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = p.X
	}
}

// literalToEnumerate returns the composite literal to walk key-by-key for a
// value, or nil. It accepts the typed forms (`Base{…}`, `&Base{…}`) and the
// ELIDED form Go permits for a nested literal (`Base{Audit: {At: g.At}}`),
// whose type is implied by the field it initializes. Treating an elided
// literal as opaque would credit every field under it while only the keys
// written are set. Slice and map literals are not struct literals and are
// handled elsewhere.
func literalToEnumerate(expr ast.Expr) *ast.CompositeLit {
	if lit := structLiteral(expr); lit != nil {
		return lit
	}
	expr = unparen(expr)
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = unparen(u.X)
	}
	if cl, ok := expr.(*ast.CompositeLit); ok && cl.Type == nil {
		return cl
	}
	return nil
}

// deliversWholeValue reports whether an expression yields a complete value of
// the assigned field's type — the shapes whose contents the walker cannot see
// but whose result is a whole value, so crediting the subtree is sound under
// the one-level-nesting doctrine. Anything outside this list is reported
// rather than assumed.
func deliversWholeValue(expr ast.Expr) bool {
	switch e := unparen(expr).(type) {
	case *ast.CallExpr, *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr,
		*ast.IndexListExpr, *ast.TypeAssertExpr, *ast.StarExpr:
		return true
	case *ast.CompositeLit:
		// A slice or map literal: not a struct, nothing to enumerate as
		// fields, and a complete value of its own type.
		return true
	case *ast.UnaryExpr:
		return e.Op == token.AND && deliversWholeValue(e.X)
	}
	return false
}

// unparenOnce strips one layer of parentheses, distinguishing `(*T)(nil)` —
// where the parens are part of the conversion syntax — from a bare call.
func unparenOnce(expr ast.Expr) ast.Expr {
	if p, ok := expr.(*ast.ParenExpr); ok {
		return p.X
	}
	return expr
}

// structLiteral returns the composite literal behind expr when it is a
// bare-identifier-typed struct literal (`Base{...}` or `&Base{...}`), and nil
// otherwise — including for slice, map and qualified-type literals, whose
// contents say nothing about a wrapper's fields.
func structLiteral(expr ast.Expr) *ast.CompositeLit {
	expr = unparen(expr)
	if u, ok := expr.(*ast.UnaryExpr); ok && u.Op == token.AND {
		expr = unparen(u.X)
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
	// Go accepts either a raw literal (the universal convention here) or an
	// interpreted one: `Base "json:\"base\""` is a TAGGED field, and reading
	// it as untagged would treat an ordinary field as an embed and promote
	// members that are not on the wire.
	inner := ""
	switch {
	case len(tagLiteral) >= 2 && tagLiteral[0] == '`' && tagLiteral[len(tagLiteral)-1] == '`':
		inner = tagLiteral[1 : len(tagLiteral)-1]
	case len(tagLiteral) >= 2 && tagLiteral[0] == '"' && tagLiteral[len(tagLiteral)-1] == '"':
		unquoted, err := strconv.Unquote(tagLiteral)
		if err != nil {
			return ""
		}
		inner = unquoted
	default:
		return ""
	}
	// The tag body is parsed by reflect, not by hand. A hand-rolled scanner
	// has to re-derive Go's quoted-string rules, and the one that lived here
	// stopped at an escaped quote inside an unrelated tag — reading a TAGGED
	// anonymous field as untagged, which promotes members that are not on the
	// wire. reflect.StructTag is the same parser encoding/json consults, so
	// the two cannot disagree.
	val, ok := reflect.StructTag(inner).Lookup("json")
	if !ok {
		return ""
	}
	if comma := strings.IndexByte(val, ','); comma != -1 {
		return val[:comma]
	}
	return val
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
