#!/usr/bin/env ruby
# frozen_string_literal: true

# Doc-constant drift gate + writer.
#
# Several constants and rosters are restated in prose and drift silently from
# their sources:
#
#   @api-version          openapi.json  .info.version
#   @bc3-pin              spec/api-provenance.json  .bc3.revision / .bc3.date
#   @assertion-types      conformance/schema.json
#                           .properties.assertions.items.properties.type.enum
#   @operation-count      openapi.json  count of path × HTTP-method pairs
#   @fixture-categories   git ls-files conformance/tests/*.json
#   @fixture-section-map  git ls-files conformance/tests/*.json
#   @zero-skip-roster     spec/zero-skip-roster.yml, RENDERED — see below
#   @service-count        count of the generated AccountClient accessors
#   @account-scoped-services
#                         the generated AccountClient accessors, by name
#                           (kotlin .../generated/ServiceAccessors.kt and
#                            swift .../Generated/AccountClient+Services.swift,
#                            which must agree)
#
# The fixture rosters are the same shape as @assertion-types one level out: a table
# that CLAIMS to account for every conformance fixture, with nothing checking
# it. Both were missed by the last three fixture-adding commits, in three
# different ways — dee221c85 added documents_write.json and updated neither
# table, b238e5ed4 added uploads_write.json with Appendix D rows and no §19
# row, and #726 added search.json's §19 row and missed Appendix D. Two
# half-applications in opposite directions is not carelessness; it is a rule
# nobody can see (CONTRIBUTING.md tells contributors to add conformance tests
# and mentions neither table).
#
# @zero-skip-roster is the odd one out, and deliberately so: it is the first
# span this gate RENDERS rather than reads. Every other block kind carries a
# human-authored column, so --check compares sets and the writer keeps its hands
# off. SPEC §19's Zero-Skip roster carries no such column — every character of it
# is derived from spec/zero-skip-roster.yml — so it is rendered into memory and
# the block is required to be BYTE-IDENTICAL. Nothing parses it, which is the
# whole point: the parser it replaces had its "a misreading always surfaces as a
# mismatch" invariant breached five times, each fix a new selector for a new
# spelling. A byte comparison has no selectors to widen. See
# scripts/zero_skip_roster.rb for the five and for what this does NOT close.
#
# Only spans MARKED with an HTML comment are checked. That is deliberate:
# spec/api-gaps/ legitimately cites ~20 historical bc3 SHAs in narrative
# ("the pin has since advanced to X", "the A..B range contains..."), and a
# naive "every SHA must equal the current pin" gate would rewrite settled
# history into a claim nobody verified. A marker means "this is a claim about
# the CURRENT value" and nothing else.
#
# The two classes, stated once because everything below follows from them:
#
#   (A) A current-value claim — "the pin is X". True only right now. Carries a
#       marker; the writer keeps it current.
#   (B) An as-of fact — "verified at X", "shipped in X", "the A..B range".
#       True forever, because the revision is bound to a fixed observation.
#       Never marked; rewriting one would fabricate an observation.
#
# The gate cannot read tense, so it cannot sort an arbitrary sentence into A or
# B. It enforces the one boundary it can see without guessing: a sentence
# naming the CURRENT pin, unmarked, is rejected (see check_unmarked_pin
# below). That is where every stale restatement in spec/api-gaps/ came from —
# each was written when its SHA was current and correct. A SHA that was never
# the pin, or that stopped being it before this gate existed, is out of reach
# of any cheap check and is governed by the convention in AGENTS.md instead.
#
# WHICH WAY TO BE WRONG. Deciding what counts as prose means approximating
# Markdown, and every approximation is wrong somewhere. Resolve every such
# ambiguity toward treating a line AS PROSE. Over-scanning costs a false alarm:
# loud, in front of the author, disproved in a minute. Under-scanning costs a
# claim nobody ever looked at, reported as success — which is the exact failure
# this gate exists to prevent, now wearing the gate's own green tick. Three
# separate review findings here were all the same bug in that light: a line the
# scanner could not see was a line it silently vouched for. This is a drift
# gate, not a Markdown parser; when the two disagree, prefer the false alarm.
#
# Marker syntax
#   Line span:   <!-- @api-version -->  or  <!-- @bc3-pin -->
#                anywhere on the line; the span is exactly that line.
#   Block span:  <!-- @assertion-types:begin --> ... <!-- @assertion-types:end -->
#                the span is the lines strictly between them.
#
# Deleting a marker to silence the gate is itself caught: spec/doc-constants.json
# commits the EXACT per-file marker count, and any deviation fails — too few
# (a claim was deleted or unmarked), too many (a claim was added without being
# recorded, and so would not be missed if it went), or a marked file the
# inventory does not mention at all.
#
# Known residual, examined and accepted: a count is per file, not per claim, so
# unmarking one @api-version sentence in SPEC.md while marking a different one
# in the same change keeps the total at 2 and passes, leaving the first
# unprotected. Closing it needs per-claim identity — named markers, say
# <!-- @api-version#headers --> with the inventory listing names — which is a
# real option and no more churn than counts, since both move only when claims
# are added or removed. It is not done here because it changes the marker
# grammar that AGENTS.md and SPEC.md document, and it collides with the
# :begin/:end suffix; and because unlike everything else fixed in review, this
# one cannot happen by ordinary authoring. It takes a deliberate relocation of
# a marker within one file, which is a reviewed edit to marked spans by
# definition. Cross-FILE relocation is already caught. If a third @api-version
# claim ever appears, revisit: the argument weakens as the count grows.
#
# Modes
#   --check (default)  Report drift; exit 1 on any error.
#   --write            Rewrite marked spans in place from the sources.
#                      Writable iff the writer can author the whole span. That
#                      is every scalar constant, and exactly one block kind:
#                      @zero-skip-roster, which is rendered from
#                      spec/zero-skip-roster.yml. The other block kinds are
#                      tables whose rows carry a human-authored column (an
#                      assertion type's meaning, a fixture's owning spec
#                      sections, a fixture's case summary), so --write never
#                      touches them and never fails on them.
#                      `make doc-constants-check` is what catches those;
#                      keeping them out of the writer keeps a schema or
#                      fixture edit from breaking every `make generate`.
#                      Files listed in spec/doc-constants.json .writerExcludes
#                      are never rewritten either: this script runs inside
#                      `make generate`, and a document whose marked value is
#                      the heading of a narrative cannot have that value
#                      advanced without the narrative advancing too. Silently
#                      doing it anyway would be the same unreviewed prose
#                      rewrite this gate exists to prevent. The writer warns
#                      and leaves them; --check then fails until a human
#                      answers.
#   --openapi PATH     Read the API version from PATH instead of ./openapi.json.
#                      sync-api-version.sh forwards its own documented
#                      [openapi.json] argument here; without that, one sync
#                      could set the SDK constants from a caller-supplied file
#                      and the prose from the repo's, leaving them disagreeing.
#
# liveAssertions (.properties.liveAssertions...) is deliberately NOT gated:
# SPEC §19 does not tabulate it, and the live canary is governed by
# CONTRIBUTING.md. Add a marked table there first if that changes.

require "json"
require "set"

require_relative "zero_skip_roster"

# The repo this gate reads. DOC_CONSTANTS_ROOT exists so
# scripts/test-doc-constants.rb can point the gate at a crafted tiny repo and
# assert it rejects each failure mode; nothing in normal operation sets it.
ROOT = File.expand_path(ENV["DOC_CONSTANTS_ROOT"] || File.expand_path("..", __dir__))

# Every input here is UTF-8 by construction — openapi.json, the tracked
# Markdown, and this script's own messages all contain non-ASCII. Ruby
# otherwise reads and writes in the locale's encoding, so under LC_ALL=C the
# gate died on the first byte of an em dash instead of checking anything.
# Encodings are pinned explicitly rather than left to the environment.
UTF8 = "UTF-8"
$stdout.set_encoding(UTF8)
$stderr.set_encoding(UTF8)

LINE_KINDS  = %w[api-version bc3-pin operation-count service-count].freeze

# The OpenAPI verbs an operation can be keyed under. Anything else in a path
# item (parameters, servers, summary) is not an operation and must not count.
HTTP_METHODS = %w[get put post delete patch head options trace].freeze
BLOCK_KINDS = %w[assertion-types fixture-categories fixture-section-map
                 account-scoped-services
                 zero-skip-roster].freeze

# The block kinds the writer may author. Most of BLOCK_KINDS is excluded because
# it CANNOT be authored: a table carrying a column only a person can write, so
# rewriting one would mean inventing that column.
#
# @account-scoped-services is the exception to that framing and is excluded
# anyway, which is worth naming so it does not read as an oversight. Every
# character of it is derivable, so the writer COULD author it — but only by
# fixing an order, and check_account_scoped_services deliberately asserts a set
# rather than a sequence (the roster is alphabetical as a courtesy to readers,
# not as a rule this gate holds). Writing it would quietly convert that courtesy
# into an enforced syntax nobody asked for. --check names the exact missing or
# extra service instead, which leaves the fix a one-line edit.
WRITABLE_BLOCK_KINDS = %w[zero-skip-roster].freeze
KNOWN_KINDS = (LINE_KINDS + BLOCK_KINDS).freeze

MARKER_RE   = /<!--\s*@([a-z0-9][a-z0-9-]*)(?::(begin|end))?\s*-->/

# A block marker must be ALONE on its line. Everything else on that line is
# content the checker structurally cannot see: scan_file takes a block's body as
# the lines strictly BETWEEN the markers, so a table row or a roster bullet
# written onto the marker line itself is rendered into the document and compared
# against nothing. `- "stale" — x. <!-- @zero-skip-roster:end -->` reads as an
# extra roster entry, survives --check, and is preserved by --write.
#
# HELD FOR EVERY BLOCK KIND, not for the one it was found in. The hole is in the
# span arithmetic, not in any kind's checker: every block kind compares only
# span.lines — the tables by set, the roster by bytes — so a row riding the
# marker line is equally invisible to all four, and to the fifth. Scoping the
# rule to @zero-skip-roster would fix one and leave the others, which is the
# per-spelling selector this whole change is a reaction to.
#
# It costs nothing to hold generally: all eight block markers in tracked
# Markdown are already standalone, so this pins existing practice rather than
# demanding a migration. Line markers are deliberately exempt — a line marker's
# whole job is to sit at the end of the prose sentence it qualifies.
#
# INDENTATION IS BOUNDED AT THREE SPACES, the same limit and for the same reason
# FENCE_RE below carries it: at four spaces CommonMark reads the line as
# INDENTED CODE, so an indented marker is not an HTML comment at all — it is
# visible literal text in the rendered document. Indent both of a block's
# markers and the count still matches, the body between them is still compared
# byte for byte, and SPEC quietly shows the delimiters it promises are
# invisible. A tab is excluded for the same reason it is there: it counts as
# four columns. Checked against the raw line rather than a stripped one,
# because stripping is what threw the indentation away.
STANDALONE_BLOCK_MARKER_RE = /\A {0,3}<!--\s*@[a-z0-9][a-z0-9-]*:(?:begin|end)\s*-->[ \t]*\z/
# A fenced-code delimiter: 3+ backticks or 3+ tildes, indented at most three
# spaces, with whatever follows captured as the info string (a close must have
# none).
#
# The three-space limit is load-bearing, not pedantry. At four spaces a line is
# an INDENTED code block — "    ```ruby" is how you show a fence without opening
# one — and a permissive \s* took that as an opening. Since the example it
# quotes usually has no matching close, the fence stayed open and every line
# after it was skipped, which is the same fail-open hole the delimiter matching
# was added to close, re-entering through the indentation door. Tabs are
# excluded for the same reason: a tab counts as four columns.
#
# Tracked Markdown indents fences 0, 2 or 3 spaces and never more, so this
# rejects nothing that exists today. A 4-space-indented fence line now reads as
# prose rather than as code, which over-scans rather than under-scans — the
# safe direction, since the cost is a visible false alarm rather than a claim
# nobody checked. Fences nested in deeply indented list items would need real
# container tracking; none exist here, and the gate is not a Markdown parser.
FENCE_RE    = /\A {0,3}(?<delimiter>`{3,}|~{3,})(?<info>.*)\z/

# A fence opening, or nil when the line only looks like one.
#
# A backtick fence's info string may not itself contain a backtick, so
# "```code``` is inline" is a paragraph holding an inline code span, not an
# opening. Accepting it opened a fence that the line never closes, and the rest
# of the file went unscanned — the same fail-open shape once more, so it lands
# on the same side as everything else: not a fence, therefore prose.
# Tilde fences have no such restriction; their info string may contain anything.
def fence_at(line)
  fence = line.match(FENCE_RE)
  return nil if fence.nil?
  return nil if fence[:delimiter].start_with?("`") && fence[:info].include?("`")

  fence
end
ISO_DATE_RE = /\b\d{4}-\d{2}-\d{2}\b/
TICKED_HEX_RE = /`([0-9a-f]{7,40})`/
BACKTICKED_RE = /`[^`]*`/

# Bare (un-backticked) SHA detection, used only INSIDE a @bc3-pin span. A plain
# /\b[0-9a-f]{7,40}\b/ also matches any run of 7+ digits, so a recording id or
# an issue number sitting in a pin sentence was reported as a "bare SHA" —
# fail-loud, never a false green, but a false alarm a human then has to
# disprove. Requiring at least one a-f digit costs nothing real: an
# abbreviation is only all-decimal by chance, and the one all-decimal
# abbreviation that would actually matter is the pin's own, which is caught by
# the second alternative built from the live revision.
BARE_HEX_WITH_LETTER_RE = /\b(?=[0-9a-f]{7,40}\b)[0-9a-f]*[a-f][0-9a-f]*\b/

def bare_sha_re(revision)
  Regexp.union(BARE_HEX_WITH_LETTER_RE, /\b#{Regexp.escape(revision[0, 7])}[0-9a-f]{0,33}\b/)
end

class Failure < StandardError; end

def read_json_at(path, label)
  raise Failure, "missing #{label}" unless File.exist?(path)

  JSON.parse(File.read(path, encoding: UTF8))
rescue JSON::ParserError => e
  raise Failure, "#{label} is not valid JSON: #{e.message}"
end

# Repo-owned inputs: always resolved against the repo root, never the cwd.
def read_json(relative)
  read_json_at(File.join(ROOT, relative), relative)
end

# The one caller-supplied input. Resolved against the cwd so the same string
# means the same file to this script as it did to the shell that passed it,
# absolute or relative.
def read_openapi(path)
  read_json_at(File.expand_path(path, Dir.pwd), path)
end

def dig!(doc, relative, *path)
  value = doc.dig(*path)
  raise Failure, "#{relative}: expected a value at .#{path.join('.')} (schema moved?)" if value.nil?

  value
end

# Markdown files git actually tracks. `git ls-files` (not Dir.glob) so the
# gitignored internal docs/ tree and vendored node_modules never get scanned.
def tracked_markdown
  out = IO.popen(["git", "-C", ROOT, "ls-files", "-z", "--", "*.md"], external_encoding: UTF8, &:read)
  raise Failure, "git ls-files failed (is #{ROOT} a git checkout?)" unless $?.success?

  out.split("\0").reject(&:empty?).sort
end

# The conformance fixtures the two §19/Appendix D rosters claim to account for.
#
# `git ls-files` rather than Dir.glob, for tracked_markdown's reason one step
# further: an untracked scratch fixture sitting in a working checkout must not
# fail that developer's build, and a fixture that IS tracked is exactly the one
# a reviewer will see and expect the tables to cover.
#
# Scoped to conformance/tests/ deliberately, and that scope is the whole of how
# SPEC §23's carve-out is honored. The parallel fixture families —
# conformance/oauth/, conformance/oauth-token/, conformance/event-feed*/ — are
# documented at their own consuming section and directory, "not in §19's
# operation-dispatch category table or Appendix D". They are not absent from
# these rosters by oversight, so the gate must not pull them in.
#
# Named by reference rather than by count on purpose. This comment carried "68
# JSON files" for several commits; the real number is 74 and was 74 when it was
# written. Nothing keeps a restated count current, which is the whole reason
# @operation-count exists two hundred lines up — and a number is not what the
# reader needs here. AGENTS.md: reach for a literal only when the sentence is
# genuinely about the value.
# DIRECT CHILDREN ONLY. Git's pathspec `*` matches across `/`, so the pattern
# alone would also return `conformance/tests/nested/case.json` — which no runner
# discovers, since all six glob non-recursively. Requiring a roster row for a
# fixture nothing executes would be this gate demanding documentation for a
# claim that is not true; the basename collapse would also make
# `nested/auth.json` and `auth.json` indistinguishable.
def tracked_fixtures
  out = IO.popen(["git", "-C", ROOT, "ls-files", "-z", "--", "conformance/tests/*.json"],
                 external_encoding: UTF8, &:read)
  raise Failure, "git ls-files failed (is #{ROOT} a git checkout?)" unless $?.success?

  out.split("\0").reject(&:empty?)
     .select { |path| File.dirname(path) == "conformance/tests" }
     .map { |path| File.basename(path) }.sort
end

# The canonical account-scoped service surface, from the two GENERATED accessor
# files that hang services off AccountClient.
#
# openapi.json plus the generators' TAG_TO_SERVICE / SERVICE_SPLITS tables is the
# true root, and is deliberately NOT the source here: reproducing that split in
# Ruby means a SIXTH hand-copy of those tables. Five already exist, one per
# language, and no gate compares them to each other — so the derivation would
# itself be the drift surface this check exists to remove. These two files are
# regenerated FROM that root and committed, which is close enough to it to be
# checkable and far enough from it to need no second implementation.
#
# They are also the only two artefacts that encode ACCOUNT-SCOPING directly
# rather than leaving it to be inferred: their entire reason to exist is hanging
# services off AccountClient, which is exactly what §5's roster claims. Per-SDK
# client wiring cannot answer the question — it drifts by design (Go folds
# `automation` and `clientVisibility` and spells `timesheets` singular; Python is
# two short; TypeScript has no account-scoped tier at all), so a roster derived
# from any one of them would assert that SDK's gaps as the canonical surface.
KOTLIN_ACCESSORS = "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/ServiceAccessors.kt"
SWIFT_ACCESSORS  = "swift/Sources/Basecamp/Generated/AccountClient+Services.swift"

KOTLIN_ACCESSOR_RE = /^val AccountClient\.([A-Za-z][A-Za-z0-9_]*)\s*:/
SWIFT_ACCESSOR_RE  = /^\s*public var ([A-Za-z][A-Za-z0-9_]*)\s*:\s*[A-Za-z0-9_]+Service\b/

def read_accessors(relative, pattern)
  path = File.join(ROOT, relative)
  unless File.exist?(path)
    raise Failure, "missing #{relative}, which is where the account-scoped service surface is " \
                   "derived from — regenerate it, or the roster in the marked block is vouched " \
                   "for by nothing"
  end

  File.read(path, encoding: UTF8).scan(pattern).flatten
end

# Both accessor files, REQUIRED TO AGREE.
#
# A disagreement is a generator bug and a finding in its own right — never a
# tiebreak to settle by preferring one file. Taking either alone would let a
# generator that dropped a service certify a roster that dropped it too.
#
# WHAT AGREEMENT DOES NOT BUY, because the obvious reading overclaims it and I
# made that mistake here first. These are not two renderings of ONE source: the
# Kotlin and Swift generators carry their own hand-maintained split tables
# (kotlin/generator/.../Config.kt, swift's ServiceGrouper) — two of the five
# copies noted above. So agreement is agreement between two independent
# transcriptions, not confirmation against a root. A service added to the
# TypeScript, Ruby and Python tables but omitted from BOTH of these two leaves
# them agreeing, and this gate certifies the old roster and the old count.
#
# That residue is real and is NOT closed here. The instrument that would close
# it is comparing the GENERATED service inventories of all five SDKs — which is
# not the sixth hand-copy rejected above, since it reads generator OUTPUT rather
# than reimplementing the mapping. Verified viable: the TypeScript, Ruby and
# Python generated service sets already agree exactly with these 53 today, once
# each SDK's index/base files and Python's `_service` filename suffix are
# accounted for. It is left to its own change because that normalization is four
# per-SDK spelling rules, and bolting them onto the gate whose own argument is
# "stop accreting spellings" is the wrong place to introduce them. Codex raised
# this on #745; tracked as the cross-generator comparison follow-up.
#
# Failures here are exit 2, not exit 1: a broken source of truth is not
# documentation drift, and "the roster disagrees with a file that is itself
# wrong" is not a verdict this gate is entitled to render. Same split
# tracked_fixtures makes between `git ls-files` failing (Failure) and matching
# nothing (a checker-level vacuity error, below).
def account_scoped_services
  kotlin = read_accessors(KOTLIN_ACCESSORS, KOTLIN_ACCESSOR_RE)
  swift  = read_accessors(SWIFT_ACCESSORS, SWIFT_ACCESSOR_RE)

  # DUPLICATES FIRST, per file, and the order is the whole point rather than
  # tidiness. A name emitted twice is invisible to every set comparison
  # downstream — Array#- drops all occurrences, so a doubled accessor leaves the
  # roster's `missing` and `extra` both empty and it passes while enumerating
  # something neither file says. Deduplicating instead of refusing would hide the
  # generator bug behind a green tick.
  #
  # Run before the agreement check because otherwise a duplicate in ONE file
  # surfaces as a disagreement whose two diffs are both empty — "Kotlin has 4,
  # Swift has 3, only in Kotlin: (none)" — which is true, unreadable, and points
  # at the wrong defect.
  { KOTLIN_ACCESSORS => kotlin, SWIFT_ACCESSORS => swift }.each do |relative, names|
    dupes = names.tally.select { |_, n| n > 1 }.keys
    next if dupes.empty?

    raise Failure, "#{relative} exposes #{dupes.join(', ')} more than once; a repeated accessor is " \
                   "invisible to a set comparison, so no roster could be checked against this"
  end

  # Compared SORTED-AND-WHOLE rather than as `.uniq` sets, which after the guard
  # above differ only in that this still reports a length mismatch it could not
  # otherwise reach.
  if kotlin.sort != swift.sort
    only_kotlin = kotlin - swift
    only_swift  = swift - kotlin
    raise Failure,
          "the two generated accessor files disagree about the account-scoped service surface " \
          "(#{KOTLIN_ACCESSORS}: #{kotlin.length}, #{SWIFT_ACCESSORS}: #{swift.length}). " \
          "Only in Kotlin: #{only_kotlin.empty? ? '(none)' : only_kotlin.join(', ')}. " \
          "Only in Swift: #{only_swift.empty? ? '(none)' : only_swift.join(', ')}. " \
          "Each is generated from its own generator's split table, so a disagreement means at " \
          "least one of those tables is wrong — fix it before any roster can be checked against " \
          "either."
  end

  # AN EMPTY EXTRACTION IS REFUSED HERE, AT THE SOURCE, AND THAT PLACEMENT IS THE
  # WHOLE POINT. It was originally a vacuity guard inside the roster checker,
  # which looked equivalent and was not: --write RETURNS BEFORE THE PER-KIND
  # CHECKERS RUN, so nothing in check mode's reasoning applies to it.
  #
  # The failure that produced, reproduced before this line existed: change both
  # generators' accessor syntax at once — a rename, a formatting pass, anything
  # that makes both regexes stop matching — and the two extractions agree at
  # zero, so the disagreement check above passes. `make generate` then rewrites
  # every @service-count span in the repo to `0` and EXITS 0. A generation
  # pipeline reporting success while writing a count nobody derived, with only a
  # later `make check` to notice, and the corruption already in the tree by then.
  #
  # Two regexes that stopped matching is an extraction failure, never a fact
  # about the SDK, and it cannot be allowed to reach either mode. Raising here
  # covers --check and --write with one rule rather than one guard per mode —
  # which is what the checker-side version silently was. Copilot found it; I had
  # dismissed it once by reasoning about --check alone.
  if kotlin.empty?
    raise Failure,
          "#{KOTLIN_ACCESSORS} and #{SWIFT_ACCESSORS} named no services between them. Both " \
          "extractions are empty, so they agree vacuously and there is no source of truth for " \
          "the roster or for @service-count — the accessor syntax these are matched by has " \
          "almost certainly changed. Refusing rather than writing a count derived from nothing."
  end

  kotlin.sort
end

Span = Struct.new(:kind, :file, :line_no, :lines, :block, keyword_init: true) do
  # 1-based inclusive range of content lines, for error messages.
  def location
    block ? "#{file}:#{line_no}-#{line_no + lines.length - 1}" : "#{file}:#{line_no}"
  end

  def text
    lines.join("\n")
  end
end

# Returns [spans, errors, prose_lines]. Line markers yield a one-line span;
# block markers yield the lines strictly between :begin and :end. prose_lines
# is every [line_no, text] pair that is neither fenced code nor inside a marked
# span — i.e. the text check_unmarked_pin is entitled to judge.
def scan_file(file)
  spans = []
  errors = []
  prose = []
  raw = File.read(File.join(ROOT, file), encoding: UTF8)
  lines = raw.lines.map(&:chomp)
  open_block = nil
  open_fence = nil

  lines.each_with_index do |line, index|
    line_no = index + 1

    # Fences are matched, not toggled. A fence closes only on the same
    # character, at least as long, with nothing after it (CommonMark), so a
    # ````-fence may quote a ```-fence — which is how you write a Markdown
    # example about Markdown, and how AGENTS.md documents this convention.
    #
    # Toggling got this wrong in the direction that matters. Each inner fence
    # flipped the flag, so a block containing an odd number of them ended with
    # the flag still set and every following line silently treated as code —
    # including an unmarked restatement of the pin, which the gate would then
    # never see. Fail-open, and invisible: the check would report success.
    if (fence = fence_at(line))
      delimiter = fence[:delimiter]

      if open_fence.nil?
        open_fence = [delimiter[0], delimiter.length]
      elsif delimiter[0] == open_fence[0] &&
            delimiter.length >= open_fence[1] &&
            fence[:info].strip.empty?
        open_fence = nil
      end
      next
    end
    next if open_fence

    prose << [line_no, line]

    # A marker inside an inline code span (or a fenced block) is documentation
    # OF the convention — AGENTS.md explains it — not a use of it. Markdown
    # renders it as literal text, so it is not an HTML comment at all. Mask
    # code spans before looking for markers; value extraction still reads the
    # raw line, where the backticked SHAs live.
    line.gsub(BACKTICKED_RE, " ").scan(MARKER_RE) do
      kind = Regexp.last_match(1)
      form = Regexp.last_match(2)

      unless KNOWN_KINDS.include?(kind)
        errors << "#{file}:#{line_no}: unknown doc-constant marker @#{kind} " \
                  "(known: #{KNOWN_KINDS.join(', ')})"
        next
      end

      if form.nil?
        unless LINE_KINDS.include?(kind)
          errors << "#{file}:#{line_no}: @#{kind} is a block marker; use " \
                    "<!-- @#{kind}:begin --> / <!-- @#{kind}:end -->"
          next
        end
        spans << Span.new(kind: kind, file: file, line_no: line_no, lines: [line], block: false)
      else
        unless BLOCK_KINDS.include?(kind)
          errors << "#{file}:#{line_no}: @#{kind} is a line marker; drop the :#{form} suffix"
          next
        end

        unless line.match?(STANDALONE_BLOCK_MARKER_RE)
          errors << "#{file}:#{line_no}: @#{kind}:#{form} must be alone on its line and indented " \
                    "at most three spaces. The body is the lines BETWEEN the markers, so anything " \
                    "else on the marker line renders into the document and is compared against " \
                    "nothing; and at four spaces the marker is indented code rather than an HTML " \
                    "comment, so it renders as visible text instead of delimiting anything."
          next
        end

        if form == "begin"
          if open_block
            errors << "#{file}:#{line_no}: @#{kind}:begin inside an unclosed " \
                      "@#{open_block[:kind]}:begin from line #{open_block[:line_no]}"
            next
          end
          open_block = { kind: kind, line_no: line_no }
        else
          if open_block.nil? || open_block[:kind] != kind
            errors << "#{file}:#{line_no}: @#{kind}:end without a matching :begin"
            next
          end
          body_start = open_block[:line_no] + 1
          body = lines[body_start - 1..line_no - 2] || []
          spans << Span.new(kind: kind, file: file, line_no: body_start, lines: body, block: true)
          open_block = nil
        end
      end
    end
  end

  if open_block
    errors << "#{file}:#{open_block[:line_no]}: @#{open_block[:kind]}:begin never closed"
  end

  # A marked file must be LF-only, and this is what makes "byte-identical" an
  # honest description of @zero-skip-roster's comparison rather than an
  # approximation of it.
  #
  # Spans are chomped, and `chomp` eats LF, CRLF and a lone CR alike — so a
  # block converted to CRLF compares EQUAL to the renderer's LF output and
  # --check passes over a block that is not byte-for-byte anything, while
  # --write would rewrite the same lines and produce a diff. Check green,
  # generate dirty: the drift-gate failure mode, inside the gate.
  #
  # Two ways to close it: carry terminators through every span, which changes
  # what every block checker sees for one file nobody has; or refuse the
  # terminator that creates the ambiguity. With no CR in the file, "the lines
  # match" and "the bytes match" are the same sentence, so the cheaper one is
  # also the one that makes the claim true. Scoped to files that actually carry
  # a span, since an unmarked file's line endings are nobody's business here,
  # and free today: no tracked Markdown contains a CR.
  if !spans.empty? && raw.include?("\r")
    line_no = raw[0...raw.index("\r")].count("\n") + 1
    errors << "#{file}:#{line_no}: carries a CR line ending, and the file holds marked span(s). " \
              "Spans are compared after chomping, which treats CRLF and LF as equal, so a " \
              "CRLF block would pass --check byte-unequal and then be rewritten by --write. " \
              "Convert the file to LF."
  end

  # Only LINE spans leave the prose pool. A line span IS the marked claim — the
  # writer rewrites it from the source on every sync, so judging it as prose
  # would flag the very restatement the marker sanctions.
  #
  # BLOCK bodies stay eligible, and the difference is not cosmetic. The writer
  # rewrites line spans only (see the --write pass), and the block checkers read
  # nothing but the `|` rows, so an ordinary sentence inside a roster or
  # assertion-types block survives both untouched. Excluding the whole body
  # would let "verified against <current pin>" sit inside a table block with no
  # marker and no grant, invisible to check_unmarked_pin and silently stale at
  # the next repin — the exact claim class this gate exists to catch, hidden by
  # the gate's own span bookkeeping. No block kind legitimately restates a SHA:
  # they hold assertion-type names and fixture filenames.
  covered = Set.new
  spans.each do |s|
    next if s.block

    (s.line_no...(s.line_no + s.lines.length)).each { |n| covered << n }
  end
  prose.reject! { |line_no, _| covered.include?(line_no) }

  [spans, errors, prose]
end

# --- per-kind checkers -------------------------------------------------------

def check_api_version(span, api_version, source)
  dates = span.text.scan(ISO_DATE_RE)
  return ["#{span.location}: @api-version span states no YYYY-MM-DD version"] if dates.empty?

  dates.uniq.reject { |d| d == api_version }.map do |bad|
    "#{span.location}: @api-version says #{bad}, #{source} .info.version is #{api_version}"
  end
end

# The count restated by @operation-count spans: every (path, HTTP method) pair
# in openapi.json, which is the same arithmetic AGENTS.md documents and the same
# number the generators report.
def operation_count(doc, source)
  paths = dig!(doc, source, "paths")
  count = paths.sum { |_path, ops| ops.count { |method, _| HTTP_METHODS.include?(method) } }

  # ZERO IS AN EXTRACTION FAILURE, NOT A COUNT, and it is refused here for the
  # reason spelled out at account_scoped_services: --write returns before the
  # per-kind checkers run, so a checker-side objection protects --check and
  # leaves the writer — the caller that edits files — free to act on it.
  #
  # Reproduced before this guard existed: point the gate at an openapi.json whose
  # `.paths` is `{}` (a truncated build, a mapper that emitted an envelope and no
  # operations) and `--write` rewrote all six @operation-count spans across three
  # files to `0` and exited 0. `dig!` above already refuses a document with no
  # `.paths` key at all; `{}` slipped past it as a legitimate empty sum.
  #
  # Guarding one of the two TICKED_INT_KINDS and not the other would make the
  # rule read as a special case for @service-count. It is not: both restate a
  # count derived from a generated artifact, and for both, "the artifact yielded
  # nothing" is a broken input rather than news about the API.
  if count.zero?
    raise Failure, "#{source} declares no operations at all (.paths is empty). That is a broken " \
                   "or truncated document, not an API with zero operations — refusing rather " \
                   "than rewriting every @operation-count span to `0`."
  end

  count
end

# A code span whose ENTIRE content is digits. Bare prose integers are not
# candidates, because the marked lines are full of them — SECURITY.md's states
# 125 GETs and 83 mutations in the same sentence as the total — and a checker
# that read those as the claim would fail on numbers it has no source for.
# Backticks are how the prose says "this one is the derived constant", the same
# device @bc3-pin uses for the SHA.
TICKED_INT_RE = /`(\d+)`/

# The single backticked integer an @operation-count span carries, or nil when the
# span is ambiguous. OCCURRENCES, not distinct values: two spans both reading
# `250` today would both be rewritten the day the count moves, and only one of
# them is the count.
#
# Both the checker and the writer go through here, and that is the point rather
# than tidiness. --write returns before the per-kind checkers run, so a writer
# that rewrote what the checker would have rejected corrupts the file first and
# is then certified by a check that can no longer see the damage.
def sole_ticked_int(text)
  ints = text.scan(TICKED_INT_RE).flatten
  ints.length == 1 ? ints.first : nil
end

# The line kinds whose claim is a single backticked integer. One checker and one
# writer branch serve both, because they are the same claim shape over different
# sources — @operation-count counts (path, method) pairs in openapi.json,
# @service-count counts the generated account-scoped accessors. A second copy of
# this would be a second place for the sole-ticked-int rule to be got wrong.
TICKED_INT_KINDS = %w[operation-count service-count].freeze

def check_ticked_count(span, count, source_says)
  ints = span.text.scan(TICKED_INT_RE).flatten

  if ints.empty?
    return ["#{span.location}: @#{span.kind} span states no backticked integer — " \
            "write the count as `#{count}` so the writer can find it"]
  end

  # A line that needs a second backticked integer cannot carry this marker. The
  # writer refuses such a span rather than rewriting every integer on it: the
  # sentence in SECURITY.md states 125 GETs and 83 mutations beside the total,
  # and a blanket gsub would turn both into the operation count.
  if ints.length > 1
    return ["#{span.location}: @#{span.kind} span has #{ints.length} backticked integers " \
            "(#{ints.join(', ')}) — exactly one is required, so the writer knows which is the " \
            "count. Put the claim on a line of its own, or unticket the others."]
  end

  return [] if ints.first == count.to_s

  ["#{span.location}: @#{span.kind} says `#{ints.first}`, #{source_says}"]
end

def check_bc3_pin(span, revision, date)
  errors = []
  text = span.text

  hexes = text.scan(TICKED_HEX_RE).flatten
  if hexes.empty?
    errors << "#{span.location}: @bc3-pin span states no backticked bc3 SHA"
  else
    hexes.uniq.reject { |h| revision.start_with?(h) }.each do |bad|
      errors << "#{span.location}: @bc3-pin says `#{bad}`, spec/api-provenance.json " \
                ".bc3.revision is #{revision}"
    end
  end

  # A SHA that lost its backticks is invisible to both the check above and the
  # writer's rewrite, so it would sit stale forever while the gate reported
  # green. Reject it outright.
  text.gsub(BACKTICKED_RE, " ").scan(bare_sha_re(revision)).flatten.compact.uniq.each do |bare|
    errors << "#{span.location}: bare SHA #{bare} in a @bc3-pin span — backtick it " \
              "so the gate can read it as part of the pin claim"
  end

  # A span that states no date is not "nothing to check" — the sync date is
  # half the pin claim, and the writer can only substitute dates it can see, so
  # a dropped date would never come back. Same rule as the missing-SHA case.
  dates = text.scan(ISO_DATE_RE)
  if dates.empty?
    errors << "#{span.location}: @bc3-pin span states no YYYY-MM-DD sync date; " \
              "a pin claim names both the revision and #{date}"
  else
    dates.uniq.reject { |d| d == date }.each do |bad|
      errors << "#{span.location}: @bc3-pin says #{bad}, spec/api-provenance.json " \
                ".bc3.date is #{date}"
    end
  end

  errors
end

# Unmarked prose must not restate TODAY's pin.
#
# This is the birth-time half of the A/B rule at the top of the file. Every
# stale pin restatement in spec/api-gaps/ — "the provenance pin (`c3086931`)",
# "at the pinned `dffa7e11b3`" — was written on the day its SHA was the pin,
# was true that day, and rotted at the next repin with nothing to notice. This
# fires on exactly that sentence, on the day it is written, while the author is
# still in the file and can say which kind of claim they meant.
#
# It deliberately does NOT fire on a SHA that is not the current pin. Deciding
# whether an arbitrary revision is a stale class-A claim or a legitimate
# class-B citation needs the pin's whole history, and reading that means
# walking git log of spec/api-provenance.json — slow, and wrong under the
# shallow clones CI uses. So this catches the claim as it is made, and AGENTS.md
# governs the ones already settled.
#
# Every occurrence counts, wherever it sits. This match is built from the live
# revision's own prefix, so unlike the generic bare-SHA heuristic it cannot
# collide with an ordinary number, and that is what lets it read the raw line
# instead of a filtered one.
#
# It got there in two steps, both of which were holes:
#
#   - Backticks-only missed "the provenance pin is d0edc128", a one-character
#     way around the rule.
#   - Masking code spans before the bare scan then missed the pin inside a
#     COMPOUND span. `d0edc1283b..2c0dafba13` is not a lone SHA, so the
#     backticked pattern skipped it, and blanking the span hid it from the
#     bare pass too. Range notation is the single most common way this file
#     names a revision, so that blind spot sat exactly where the traffic is.
#
# Scanning the raw line subsumes both. A marked span's line never reaches here
# (scan_file drops it from prose), and neither does anything fenced.
#
# It returns EVERY occurrence, not the first. The count that bounds a grant is
# a count of claims, and claims are not one per line: append "…, which is the
# provenance pin `X`" to a line that already ends a range at `X` and the line
# now makes two, while a first-match-only scan would still report one and keep
# the grant satisfied.
#
# Case-insensitive, because 2C0DAFBA and 2c0dafba are the same revision and a
# lowercase-only scan would wave the first one through. Only this scan is
# relaxed: a MARKED span is still required to spell the SHA in the lowercase
# form the writer emits, and an uppercase one there is reported as "states no
# backticked bc3 SHA" — actionable, and it keeps the writer from having to
# guess which spelling to preserve.
def current_pin_citations(line, revision)
  line.scan(/\b#{Regexp.escape(revision[0, 7])}[0-9a-f]{0,33}\b/i)
end

# "lines 103, 107" normally; "line 103 (x2)" when one line carries two claims,
# because a bare repeated line number reads as a bug in the message rather than
# the thing it is reporting.
def citation_locations(hits)
  tallied = hits.map(&:first).tally
  label = tallied.length == 1 ? "line" : "lines"
  listed = tallied.map { |line_no, n| n > 1 ? "#{line_no} (x#{n})" : line_no.to_s }

  "#{label} #{listed.join(', ')}"
end

# A grant is bounded by a COUNT, for the same reason markerFloors became
# markerCounts. spec/api-gaps/README.md has to be granted — its job is
# recording ranges, and a range's endpoint is by definition the pin that repin
# set, so it names today's pin every time. But it is also the single likeliest
# place for a genuine class-A sentence ("the provenance pin is X") to be
# written, and an unbounded file grant would wave that through forever. The
# count says "this file carries exactly N as-of citations of today's pin, all
# reviewed"; the N+1th fails until someone looks at it and re-states the number.
def check_unmarked_pin(file, prose, revision, allowed)
  hits = prose.flat_map do |line_no, line|
    current_pin_citations(line, revision).map { |hit| [line_no, hit] }
  end

  grant = allowed[file]
  claims = grant.nil? ? [] : class_a_claims(file, prose, revision)

  if claims.any?
    claims
  elsif grant.nil?
    hits.map { |line_no, hit|
      "#{file}:#{line_no}: `#{hit}` is the current provenance pin, restated outside a " \
        "@bc3-pin span. Either mark the line <!-- @bc3-pin --> (and name the sync date) so " \
        "`make sync-api-version` keeps it current, or bind the SHA to what makes it permanently " \
        "true — the PR that shipped it, the verification it backs — and record the file in " \
        "spec/doc-constants.json .unmarkedPinCitations with that reason and a count."
    }.uniq
  elsif hits.length == grant["count"]
    []
  elsif hits.length > grant["count"]
    ["#{file}: spec/doc-constants.json grants #{grant['count']} unmarked citation(s) of the " \
     "current pin, found #{hits.length} (#{citation_locations(hits)}). The grant covers " \
     "reviewed as-of facts, not whatever the file grows next — read the new one, confirm it " \
     "binds the SHA to a fixed observation rather than claiming today's pin, and raise the " \
     "count in the same commit."]
  else
    # Fewer than granted: the entry has outlived what it described. Left alone
    # it is a standing permission nobody reviewed, pre-authorising the next
    # unmarked restatement in this file.
    ["#{file}: spec/doc-constants.json grants #{grant['count']} unmarked citation(s) of the " \
     "current pin, found #{hits.length} — the entry no longer describes the file. Lower the " \
     "count, or drop the entry."]
  end
end

# The one thing a grant never covers: a sentence in the class-A grammar.
#
# A grant is a statement about KIND — "the citations in this file are as-of
# facts" — enforced by a count, which is a statement about QUANTITY. Swap one
# granted citation for "the provenance pin is X" in the same change and the
# quantity is unchanged, so the count alone lets a current-value claim in under
# a reason that does not describe it.
#
# Binding the grant to occurrence identity would close that completely, and it
# was considered: line numbers churn on every unrelated edit, and hashing the
# matched text churns on every rewording of the triage prose. Either way the
# grant would need bumping constantly, and an inventory that gets bumped
# reflexively is not an inventory — the same argument that makes markerCounts
# work only because it moves when the number of claims moves, which is rare and
# meaningful.
#
# So instead of identity, this checks for the one shape that is never an as-of
# fact. AGENTS.md already names it as the failure to avoid — "writing the second
# in the grammar of the first" — and a sentence in that grammar wants a marker
# whether or not its file holds a grant. Deliberately narrow: it is a floor
# under the grant, not a tense parser, and the residual is recorded in the PR.
CLASS_A_GRAMMAR_RE = /\b(?:pin|revision)\s+is\b|\bat\s+the\s+pinned\b|\bprovenance\s+pin\s*\(/i

def class_a_claims(file, prose, revision)
  prose.filter_map do |line_no, line|
    next if current_pin_citations(line, revision).empty?
    next unless line.match?(CLASS_A_GRAMMAR_RE)

    "#{file}:#{line_no}: names the current pin in the grammar of a current-value claim " \
      "(\"the pin is X\", \"at the pinned X\"). spec/doc-constants.json .unmarkedPinCitations " \
      "grants as-of FACTS, and never this shape — a grant cannot make a class-A sentence " \
      "permanently true. Mark the line <!-- @bc3-pin --> (and name the sync date), or reword it " \
      "to say what binds the revision."
  end
end

# The grant shape is load-bearing: a bare string (the shape before counts) or a
# missing count would otherwise read as "grant everything" — a silent hole in
# exactly the check this list exists to bound.
#
# Returns [errors, well_formed_grants]. A malformed grant is reported and then
# dropped rather than being handed to check_unmarked_pin, which would compare
# a count against nil and die with a backtrace instead of the message that says
# what to fix. Its file is skipped for the citation scan too: the shape error is
# the actionable one, and also listing every citation in the file as unmarked
# would bury it.
def validate_citation_grants(allowed)
  errors = []
  valid = {}

  allowed.each do |file, grant|
    where = ".unmarkedPinCitations[#{file.inspect}]"
    problem =
      if !grant.is_a?(Hash)
        "#{where} must be an object with \"count\" and \"reason\", got #{grant.class}"
      elsif !grant["count"].is_a?(Integer) || !grant["count"].positive?
        "#{where}.count must be a positive integer, got #{grant['count'].inspect}"
      elsif !grant["reason"].is_a?(String) || grant["reason"].strip.empty?
        "#{where} needs a non-empty \"reason\" — the reason is what a reviewer checks the " \
          "citation against"
      end

    if problem
      errors << "spec/doc-constants.json: #{problem}"
    else
      valid[file] = grant
    end
  end

  [errors, valid]
end

def check_assertion_types(span, schema_types)
  documented = span.lines.filter_map do |line|
    next unless line.lstrip.start_with?("|")

    cell = line.split("|")[1]
    next if cell.nil?

    match = cell.strip.match(/\A`([A-Za-z][A-Za-z0-9_]*)`\z/)
    match && match[1]
  end

  errors = []
  duplicated = documented.tally.select { |_, n| n > 1 }.keys
  duplicated.each { |name| errors << "#{span.location}: `#{name}` is tabulated twice" }

  missing = schema_types - documented
  extra   = documented - schema_types

  unless missing.empty?
    errors << "#{span.location}: conformance/schema.json defines #{schema_types.length} " \
              "assertion types, the table documents #{documented.uniq.length}; missing: " \
              "#{missing.map { |n| "`#{n}`" }.join(', ')}"
  end
  unless extra.empty?
    errors << "#{span.location}: table documents assertion types absent from " \
              "conformance/schema.json: #{extra.map { |n| "`#{n}`" }.join(', ')}"
  end

  errors
end

# --- conformance fixture rosters ---------------------------------------------

# A Markdown alignment cell: `---`, `:---`, `---:` or `:---:`.
SEPARATOR_CELL_RE = /\A:?-+:?\z/

# The cells of one Markdown table row, stripped. The leading empty string before
# the opening pipe is dropped; Ruby's split already drops the trailing one.
def table_cells(line)
  # Split on UNESCAPED pipes only. `\|` inside a cell is ordinary Markdown —
  # "supports A \| B" is a legitimate test summary — and a raw split would treat
  # the text after it as the next column, shifting every cell along. A row whose
  # Primary section is actually blank then presents a non-empty cell there and
  # satisfies the blank-cell guard.
  parts = line.split(/(?<!\\)\|/)
  parts.shift
  parts.map { |cell| cell.strip.gsub("\\|", "|") }
end

# The DATA rows of the table inside a marked span, as [line_no, cells] pairs.
#
# check_assertion_types identifies a row by whether its first cell already
# parses, and filter_maps the rest away. That is safe there: a row it drops
# reappears as a `missing:` assertion type, because the schema is the source of
# truth and every name in it must be found somewhere. The fixture rosters get
# the stricter form — a row is whatever follows the |---|---| separator, and a
# row whose file cell does not parse is REPORTED rather than skipped. Same rule
# the fence handling follows: a row the parser cannot see is a row it silently
# vouches for, and over-scanning costs a false alarm while under-scanning costs
# a claim nobody checked, wearing this gate's own green tick.
def table_rows(span)
  numbered = span.lines.each_with_index
                 .map { |line, index| [span.line_no + index, line.strip] }
                 .select { |_, line| line.start_with?("|") }

  separator = numbered.index do |_, line|
    cells = table_cells(line)
    !cells.empty? && cells.all? { |cell| cell.match?(SEPARATOR_CELL_RE) }
  end

  if separator.nil?
    return [["#{span.location}: the @#{span.kind} span holds no Markdown table (no |---|---| " \
             "separator row) — the marker pair is around the wrong lines"], []]
  end

  [[], numbered[(separator + 1)..].map { |line_no, line| [line_no, table_cells(line)] }]
end

# A table cell naming a fixture file: a single backticked *.json and nothing else.
FIXTURE_CELL_RE = /\A`([A-Za-z0-9][A-Za-z0-9_.-]*\.json)`\z/

# The section numbers a document actually defines, from its `## §N.` headings.
def spec_sections(file)
  @spec_sections ||= {}
  @spec_sections[file] ||= begin
    found = File.readlines(File.join(ROOT, file), chomp: true, encoding: UTF8)
               .filter_map { |line| line[/\A## §(\d+)\./, 1] }.to_set
    if found.empty?
      raise Failure, "#{file}: no `## §N.` headings found, so section references in its tables " \
                     "cannot be validated — this check has no source of truth"
    end

    found
  end
end

# Section references in a table cell that resolve to no heading.
#
# Catches a typo (`§99`) and, more usefully, a reference that RESOLVED when it
# was written and stopped resolving when a section was renumbered — the case a
# reviewer reading the same PR cannot see, unlike an obviously wrong number.
#
# Deliberately does NOT require a row to carry a section reference at all. That
# would catch a `TBD` placeholder, but only with a carve-out for the row that
# legitimately has none — `live-my-surface.json`, attributed to external
# governance — and the carve-out list is the part that grows.
def unresolved_sections(cell, file)
  cell.scan(/§(\d+)/).flatten.uniq.reject { |number| spec_sections(file).include?(number) }
end

# Vacuity guards, stated explicitly rather than left to emerge.
#
# Mostly this buys the MESSAGE, but not entirely, and the difference is the
# reason it cannot be deleted.
#
# When only ONE side is empty the set comparison does catch it — an empty table
# makes every fixture `missing`, an empty fixture list makes every row `extra`.
# There this guard only improves the report: a regex that stopped matching reads
# as 22 individually plausible drift findings instead of the one thing actually
# wrong. House standard since check-search-fixture-copy.py.
#
# When BOTH sides are empty it is the only thing standing. `missing` and `extra`
# are both empty, the bidirectional comparison is trivially satisfied, and the
# gate reports success over a roster checked against nothing — the vacuous pass
# this whole file exists to refuse. `git ls-files` matching nothing is an
# extraction failure, never a fact about the repo.
#
# Do not reduce this to a message-formatting nicety on the strength of the
# one-sided reasoning. Both self-test cases are committed ("no tracked fixtures
# at all" and "both sides empty is not agreement"); the second one is what
# fails if this guard goes.
def roster_vacuity(span, fixtures, rows)
  if fixtures.empty?
    ["#{span.location}: internal error: `git ls-files conformance/tests/*.json` matched nothing, " \
     "so this check has no source of truth and cannot vouch for the table"]
  elsif rows.empty?
    ["#{span.location}: the @#{span.kind} table has no data rows"]
  else
    []
  end
end

# The category slug a fixture's filename dictates: basename minus `.json`, with
# `_` written as `-`. Verified to hold across every row of the table with zero
# violations, which is what makes it an invariant to assert rather than a
# convention to hope for.
def category_slug(file)
  File.basename(file, ".json").tr("_", "-")
end

# SPEC §19's Test Categories table: exactly one row per tracked fixture, and the
# category slug is the filename. The invariant is a bijection, so all of it is
# asserted — both directions plus the slug.
def check_fixture_categories(span, fixtures)
  errors, rows = table_rows(span)
  return errors unless errors.empty?

  vacuity = roster_vacuity(span, fixtures, rows)
  return vacuity unless vacuity.empty?

  documented = []
  rows.each do |line_no, cells|
    # EXACTLY three, not at least three. An extra cell is not a harmless
    # surplus: a raw pipe in the attribution shifts the real section into a
    # fourth cell and puts the fragment before it into cells[2], where
    # non-`§` attributions are legitimately allowed — so the gate validates the
    # wrong cell and a `§99` in the actual section position is never checked,
    # while the row renders with a column GFM drops.
    #
    # This is deliberately a REFUSAL, not another spelling rule. Teaching the
    # splitter more Markdown (separator widths, backslash parity) is the
    # treadmill declined elsewhere in this PR; rejecting a row whose shape the
    # parser does not recognise is the direction this file already argues for —
    # "a row the parser cannot see is a row it silently vouches for". It closes
    # the pipe class as a class: however a stray pipe was spelled, the cell
    # count is wrong and the row fails loudly instead of being mis-parsed
    # quietly.
    if cells.length != 3
      errors << "#{span.file}:#{line_no}: category row has #{cells.length} cell(s), not 3; the " \
                "shape is | Category | `file.json` | Owning Spec Section(s) |. An unescaped `|` " \
                "in a cell splits the row — write it as `\\|`."
      next
    end

    # The table's claim is that every fixture HAS an owning spec section. A row
    # with an empty third cell satisfies presence and states nothing, so it
    # would let a new fixture through with the attribution the row exists to
    # carry left blank — the gate reporting the claim as kept while it is
    # vacuous. Both PR bots flagged this on both tables.
    if cells[2].empty?
      errors << "#{span.file}:#{line_no}: category row for #{cells[1]} names no owning spec " \
                "section. That attribution is the whole claim of this table — a row without it " \
                "records that the fixture exists, not that anything in the spec owns it."
      next
    end

    match = cells[1].match(FIXTURE_CELL_RE)
    if match.nil?
      errors << "#{span.file}:#{line_no}: the Files cell #{cells[1].inspect} is not a single " \
                "backticked fixture filename (`something.json`)"
      next
    end

    documented << [cells[0], match[1], line_no]

    unresolved = unresolved_sections(cells[2], span.file)
    unless unresolved.empty?
      errors << "#{span.file}:#{line_no}: owning section(s) #{unresolved.map { |n| "§#{n}" }.join(', ')} " \
                "do not resolve to a `## §N.` heading in #{span.file} — a renumbered section " \
                "leaves a reference that reads fine and points nowhere"
    end

    next if cells[0] == category_slug(match[1])

    errors << "#{span.file}:#{line_no}: category `#{cells[0]}` does not match its file — " \
              "`#{match[1]}` dictates the slug `#{category_slug(match[1])}` (basename, `_` as `-`)"
  end

  listed = documented.map { |_, file, _| file }
  listed.tally.select { |_, n| n > 1 }.each_key do |file|
    errors << "#{span.location}: `#{file}` is tabulated on more than one row; one fixture, one " \
              "category"
  end

  # Tallying FILES catches a fixture listed twice; it does not catch two
  # different fixtures deriving the same category. `_` and `-` collapse to the
  # same slug, so `foo_bar.json` and `foo-bar.json` each satisfy the per-row
  # slug check above and both claim the category `foo-bar`. The table is then
  # not the bijection its own heading asserts, and the category label is
  # ambiguous about which fixture it names. The slug is what the table is keyed
  # by, so the slug is what has to be unique.
  # Keyed on the DERIVED slug, not the declared cell. A row whose category cell
  # is simply wrong is already reported above and still reaches here, so
  # grouping by what the row claims would both miss real collisions and invent
  # false ones. The filename is what dictates the slug, so the filename is what
  # this groups by. Only DISTINCT files count — one file on two rows is the
  # tally above, and reporting it twice would just be noise.
  documented.group_by { |_, file, _| category_slug(file) }
            .each do |slug, rows|
    files = rows.map { |_, file, _| file }.uniq
    next if files.length < 2

    errors << "#{span.location}: category `#{slug}` is dictated by #{files.map { |f| "`#{f}`" }.join(' and ')} " \
              "— distinct fixtures deriving one slug (`_` and `-` collapse to the same category). " \
              "The table is keyed by category, so this is not the bijection it claims; rename one."
  end

  missing = fixtures - listed
  extra   = listed - fixtures

  unless missing.empty?
    errors << "#{span.location}: conformance/tests holds #{fixtures.length} tracked fixture(s), " \
              "the table categorises #{listed.uniq.length}; missing: " \
              "#{missing.map { |f| "`#{f}`" }.join(', ')}. Every fixture needs a row naming the " \
              "spec section that owns it — add one, or the fixture is a test nothing in the spec " \
              "claims."
  end
  unless extra.empty?
    errors << "#{span.location}: the table categorises files git does not track under " \
              "conformance/tests/: #{extra.map { |f| "`#{f}`" }.join(', ')}"
  end

  errors
end

# SPEC Appendix D's Conformance Test → Spec Section mapping: every tracked
# fixture appears on at least one row, and no row names a file that is not one.
#
# Deliberately weaker than the categories check, because the artifact is
# weaker. Its rows are curated free-form summaries — one row bundles eleven
# schedule-entry cases, four rows split uploads_write.json by theme — so
# "exactly one row per fixture" is not true of it and never was. Asserting
# coverage is what the table actually claims; asserting shape would be
# asserting an invariant nobody wrote.
def check_fixture_section_map(span, fixtures)
  errors, rows = table_rows(span)
  return errors unless errors.empty?

  vacuity = roster_vacuity(span, fixtures, rows)
  return vacuity unless vacuity.empty?

  covered = []
  rows.each do |line_no, cells|
    # Exactly three, for the reason given on the categories table above: a raw
    # pipe in a free-form test summary shifts the real section into an ignored
    # fourth cell, and this table's summaries are the likeliest place in the
    # repo for someone to write `supports A | B`.
    if cells.length != 3
      errors << "#{span.file}:#{line_no}: mapping row has #{cells.length} cell(s), not 3; the " \
                "shape is | `file.json` | Test name | Primary section |. An unescaped `|` in a " \
                "cell splits the row — write it as `\\|`."
      next
    end

    # Coverage by a row that names neither the cases nor a section is coverage
    # in name only. This table is already the weaker of the two — it asserts
    # that a fixture appears, not that every case does — so a blank row would
    # reduce it to asserting nothing at all.
    blank = [["Test name", cells[1]], ["Primary section", cells[2]]].select { |_, cell| cell.empty? }
    unless blank.empty?
      errors << "#{span.file}:#{line_no}: mapping row for #{cells[0]} leaves " \
                "#{blank.map(&:first).join(' and ')} empty — a row that summarises no cases and " \
                "names no section covers the fixture only nominally."
      next
    end

    match = cells[0].match(FIXTURE_CELL_RE)
    if match.nil?
      errors << "#{span.file}:#{line_no}: the Test file cell #{cells[0].inspect} is not a single " \
                "backticked fixture filename (`something.json`)"
      next
    end

    covered << match[1]

    unresolved = unresolved_sections(cells[2], span.file)
    next if unresolved.empty?

    errors << "#{span.file}:#{line_no}: primary section(s) #{unresolved.map { |n| "§#{n}" }.join(', ')} " \
              "do not resolve to a `## §N.` heading in #{span.file}"
  end

  missing = fixtures - covered
  extra   = covered.uniq - fixtures

  unless missing.empty?
    errors << "#{span.location}: no row maps these tracked fixtures to a primary section: " \
              "#{missing.map { |f| "`#{f}`" }.join(', ')}. One row per theme is enough — the " \
              "table claims coverage, not a case-by-case index."
  end
  unless extra.empty?
    errors << "#{span.location}: rows name files git does not track under conformance/tests/: " \
              "#{extra.map { |f| "`#{f}`" }.join(', ')}"
  end

  errors
end

# SPEC §19's Zero-Skip roster: rendered from spec/zero-skip-roster.yml and
# required to match BYTE FOR BYTE.
#
# This is the only check in this file with no parser behind it, and that is the
# entire design. Its predecessor read the roster back out of SPEC's prose, on the
# argument that a misreading always surfaces as a set mismatch and never as a
# passing comparison. That argument failed five times — a list marker it did not
# recognise, a duplicate block, a duplicate entry, a payload on a blockquote or
# table row, and an ASCII-only quote test that curly quotes and backticks walk
# straight through. Each fix was one more selector. String equality has none:
# every one of those spellings is now just text inside a generated block, and any
# text the renderer did not produce is a diff.
#
# WHAT IT DOES AND DOES NOT BUY. It makes SPEC's block faithful to the YAML,
# nothing more. Whether the YAML is faithful to the RUNNERS is a different claim,
# checked by scripts/check-fixture-execution.rb against the execution manifests —
# which needs a conformance run, which is why that half cannot live here.
#
# VACUITY, and why there is no guard for it here. "Both sides empty is agreement"
# is the hole every set comparison in this file has to plug explicitly. This one
# cannot reach it: ZeroSkipRoster.load requires all six runner sections and
# render_lines emits at least a heading for each, so the rendered side is never
# empty and an emptied block never matches it. An emptied YAML fails in the
# loader instead, before this is called. Committed as self-test cases rather than
# asserted here, because a guard that cannot fire is a guard nobody can test.
def check_zero_skip_roster(span, expected)
  actual = span.lines
  return [] if actual == expected

  index = (0...[actual.length, expected.length].max).find { |n| actual[n] != expected[n] }
  line_no = span.line_no + index

  # Name the first differing line and show both sides whole. Truncating would
  # hide exactly the cases worth reporting — a trailing space, a straightened
  # quote, an em dash written as a hyphen — since the visible halves of two such
  # lines are identical.
  ["#{span.file}:#{line_no}: the @#{span.kind} block does not match " \
   "#{ZeroSkipRoster::RELATIVE_PATH} (#{actual.length} line(s) in SPEC, #{expected.length} " \
   "rendered; first difference on this line).\n" \
   "      rendered: #{(expected[index] || '(end of block)').inspect}\n" \
   "      found:    #{(actual[index] || '(end of block)').inspect}\n" \
   "      This block is generated. Edit #{ZeroSkipRoster::RELATIVE_PATH} and run " \
   "`make sync-api-version`; editing SPEC by hand is what this replaced."]
end

# A service name as the roster may spell it: lowerCamelCase, no separators.
# Every one of the generated accessors matches this, so a roster entry that does
# not is not a service name at all — it is a stray comma, a wrapped word, a bit
# of prose that drifted inside the markers, or a name written in another SDK's
# casing.
SERVICE_NAME_RE = /\A[a-z][A-Za-z0-9]*\z/

# SPEC §5's (and Appendix B's) account-scoped service roster: a comma-separated
# prose line, checked for set equality against the generated accessors.
#
# WHY THIS STAYS A COMMA LIST rather than reusing `table_rows`. That machinery is
# table-only, so reusing it means 53 rows of exactly three cells each — and the
# only derivable column is the name. The other two would be new hand-written
# content, i.e. new drift surface, introduced by the change whose entire point is
# removing drift surface. Restating less beats reusing machinery here.
#
# WHY THE BESPOKE PARSER IS ALLOWED TO BE BESPOKE. #740 declined to teach this
# file more GFM because a mis-parse there is SILENT — it validates the wrong cell
# and reports success. The shape below has no such reading. It refuses anything
# it does not recognise instead of skipping it, so the only outcomes are a set
# mismatch or an explicit shape refusal; there is no input it reads as agreement
# it has not actually checked. #736 asserted that property in a comment and had
# it breached three times, so here it is a committed self-test case
# ("misreadings surface, never pass") rather than a claim in prose.
#
# Order is deliberately NOT asserted. The roster's claim is a set, the file is
# alphabetical only as a courtesy to readers, and a sortedness rule would be this
# gate constraining syntax it was not asked to hold. Noted so it is not
# re-litigated as an oversight.
def check_account_scoped_services(span, services)
  # SHAPE FIRST, and this one rule closes the whole list-shaped class that #736
  # needed an inverted default for. Reformat the roster as bullets, wrap it over
  # two lines, or leave a sentence inside the markers, and the block no longer
  # holds exactly one line — which is an error, not a partial read of whichever
  # line the parser happened to recognise. An emptied block lands here too, as 0.
  lines = span.lines.reject { |line| line.strip.empty? }
  if lines.length != 1
    return ["#{span.location}: the @#{span.kind} block holds #{lines.length} non-empty line(s), " \
            "not 1. The roster is ONE comma-separated line of service names — bullets, a wrapped " \
            "line, or prose inside the markers would each leave the parser guessing which line " \
            "is the claim."]
  end

  # `-1` keeps trailing empty fields, so `a, b,` yields a third entry of `""`
  # that fails SERVICE_NAME_RE below. Dropped, a stray comma would read as a
  # clean list and the shape would go unremarked.
  names = lines.first.strip.split(",", -1).map(&:strip)

  malformed = names.reject { |name| name.match?(SERVICE_NAME_RE) }
  unless malformed.empty?
    return ["#{span.location}: #{malformed.length} roster entry/entries are not service names: " \
            "#{malformed.map(&:inspect).join(', ')}. Each must be lowerCamelCase " \
            "(#{SERVICE_NAME_RE.source}), separated by `, ` — an empty entry is a stray or " \
            "trailing comma."]
  end

  # No vacuity guard here, deliberately, and this is the second version of this
  # comment — the first one argued for keeping a guard that turned out to be in
  # the wrong place entirely.
  #
  # An empty extraction is now refused by account_scoped_services itself, which
  # both modes reach. A guard here could only ever have protected --check, since
  # --write returns before the per-kind checkers run; it read as the vacuity
  # backstop while covering exactly one of the two callers, and the writer was
  # the caller that could do damage. Duplicating it here would restate a rule the
  # source already enforces and re-suggest that this is the layer that owns it.
  errors = []

  # Array#- removes EVERY occurrence, so a name listed twice is invisible to the
  # comparison below — both directions come back empty and the gate passes over a
  # roster that is not the enumeration it claims to be. #736 shipped this bug and
  # had to be told about it.
  dupes = names.tally.select { |_, n| n > 1 }.keys
  unless dupes.empty?
    errors << "#{span.location}: the roster names #{dupes.map(&:inspect).join(', ')} more than " \
              "once; a repeat is invisible to the set comparison, so it would pass unnoticed"
  end

  missing = services - names
  extra   = names - services

  unless missing.empty?
    errors << "#{span.location}: the generated accessors expose #{services.length} account-scoped " \
              "service(s), the roster names #{names.uniq.length}; missing: #{missing.join(', ')}. " \
              "Add them — a service the spec's own surface section omits is one no reader of the " \
              "spec knows exists."
  end
  unless extra.empty?
    errors << "#{span.location}: the roster names service(s) no generated accessor exposes: " \
              "#{extra.join(', ')}. Either the service was removed and the roster kept it, or the " \
              "name is misspelled."
  end

  errors
end

# --- writer ------------------------------------------------------------------

# Every argument here is a VALUE, never a thunk, and that is load-bearing rather
# than stylistic: this runs inside the write loop, so anything that computes
# lazily here can raise after earlier files are already on disk. Passing
# resolved values makes "the loop cannot raise for a reason we have not already
# found" a property of the signature instead of a discipline someone has to
# remember at the next call site.
#
# `ticked_counts` is that rule applied to a set of kinds rather than one: a hash
# of kind => already-computed Integer, not of kind => thunk. Keying it means a
# third ticked-int kind is an entry at the call site instead of another keyword
# here, and holding VALUES means adding one cannot reintroduce the deferred
# computation this signature exists to keep out of the loop.
def rewrite_line(kind, line, api_version:, revision:, date:, ticked_counts:)
  case kind
  when "api-version"
    line.gsub(ISO_DATE_RE, api_version)
  when *TICKED_INT_KINDS
    # Refuse an ambiguous span instead of rewriting every integer on it. Left
    # untouched, it fails the next --check with a message naming the problem;
    # rewritten, it would be silently corrupt and then pass.
    sole_ticked_int(line) ? line.sub(TICKED_INT_RE, "`#{ticked_counts.fetch(kind)}`") : line
  when "bc3-pin"
    # Preserve the abbreviation length the prose already chose.
    line
      .gsub(TICKED_HEX_RE) { "`#{revision[0, Regexp.last_match(1).length]}`" }
      .gsub(ISO_DATE_RE, date)
  else
    line
  end
end

# --- main --------------------------------------------------------------------

def run(mode, openapi)
  openapi_doc = read_openapi(openapi)
  api_version = dig!(openapi_doc, openapi, "info", "version")

  # Derived lazily, and only when an @operation-count span actually needs it.
  # The gate's own fixtures are minimal OpenAPI documents with no .paths at all,
  # and a document without operations is a real failure only for a file that
  # claims to count them — computing it eagerly turned every such fixture into
  # an error about a constant it never mentions.
  op_count_memo = nil
  op_count = lambda do
    op_count_memo ||= operation_count(openapi_doc, openapi)
  end

  # Lazy and memoised for the same reason once more, and this one is needed in
  # BOTH modes: --write renders the roster block, --check compares against it.
  # A repo whose SPEC never carries the marker never loads the file, which is
  # what lets the gate's own crafted fixtures stay minimal.
  #
  # A malformed roster is a Failure (exit 2), not a drift error (exit 1): the
  # gate has no source of truth, so it cannot vouch for the block either way.
  # Reporting "the block does not match" would name the wrong file.
  roster_memo = nil
  roster = lambda do
    roster_memo ||= begin
      ZeroSkipRoster.load(ROOT)
    rescue ZeroSkipRoster::Malformed => e
      raise Failure, e.message
    end
  end
  roster_lines = -> { ZeroSkipRoster.render_lines(roster.call) }

  # kind => the exact lines a writable block must hold. Keyed by kind so adding
  # a second rendered block is a line here rather than a branch in the writer.
  block_bodies = { "zero-skip-roster" => roster_lines }

  # Memoised and lazy for the same reason once more, one step more sharply: this
  # one reads two generated SDK files that the gate's own crafted fixtures have
  # no business carrying, so computing it eagerly would fail every case that
  # never mentions a service count.
  services_memo = nil
  services = lambda do
    services_memo ||= account_scoped_services
  end

  # kind => how to compute the sole backticked integer that kind's spans state.
  # Still thunks HERE, because --check must be able to ask for one without
  # paying for the other; the write path resolves the ones it needs to VALUES in
  # its preflight, which is what keeps rewrite_line unable to compute anything.
  ticked_count_sources = {
    "operation-count" => op_count,
    "service-count"   => -> { services.call.length }
  }

  provenance = read_json("spec/api-provenance.json")
  revision = dig!(provenance, "spec/api-provenance.json", "bc3", "revision")
  date     = dig!(provenance, "spec/api-provenance.json", "bc3", "date")

  unless revision.match?(/\A[0-9a-f]{40}\z/)
    raise Failure, "spec/api-provenance.json .bc3.revision is not a full 40-char SHA: #{revision}"
  end

  config = read_json("spec/doc-constants.json")
  declared = dig!(config, "spec/doc-constants.json", "markerCounts")
  writer_excludes = config.fetch("writerExcludes", {})
  pin_citations   = config.fetch("unmarkedPinCitations", {})

  errors = []
  spans = []
  prose_by_file = {}
  tracked_markdown.each do |file|
    file_spans, file_errors, file_prose = scan_file(file)
    spans.concat(file_spans)
    errors.concat(file_errors)
    prose_by_file[file] = file_prose
  end

  # Marker inventory: spec/doc-constants.json states EXACTLY how many marked
  # claims each file carries, and every deviation fails.
  #
  # This was a floor — a minimum — which left a hole worth more than the
  # convenience it bought. Under a minimum, a claim added to a file that
  # already met its count is unprotected forever: add a third @api-version to
  # SPEC.md against a floor of 2, delete it later, and two markers remain, so
  # the gate passes while the value it used to guard drifts. The whole point of
  # this section is that deleting a marked claim fails, so it has to hold for
  # every claim, not just the first N. An exact count costs one line in the
  # same commit that adds the marker, and buys an inventory that is true.
  counts = Hash.new(0)
  spans.each { |s| counts[[s.kind, s.file]] += 1 }

  declared_pairs = []
  declared.each do |kind, per_file|
    unless KNOWN_KINDS.include?(kind)
      errors << "spec/doc-constants.json: count declared for unknown marker @#{kind}"
      next
    end
    per_file.each do |file, expected|
      declared_pairs << [kind, file]
      found = counts[[kind, file]]
      next if found == expected

      errors <<
        if found < expected
          "#{file}: spec/doc-constants.json declares #{expected} @#{kind} marker(s), found " \
            "#{found}. A current-value claim was deleted or unmarked; if it genuinely moved, " \
            "move the count in the same commit."
        else
          "#{file}: spec/doc-constants.json declares #{expected} @#{kind} marker(s), found " \
            "#{found}. A marked claim was added without recording it — set the count to " \
            "#{found} in the same commit, so deleting it later fails too."
        end
    end
  end

  # A marker in a file nobody declared is equally unprotected: it could be
  # removed with nothing to notice.
  (counts.keys - declared_pairs).each do |kind, file|
    errors << "#{file}: carries #{counts[[kind, file]]} @#{kind} marker(s) that " \
              "spec/doc-constants.json does not declare — add the count so deleting them fails."
  end

  if mode == :write
    # STRUCTURAL PROBLEMS ABORT BEFORE ANYTHING IS WRITTEN, and the ordering is
    # the whole point: this check used to run after the write loop, so a file
    # with damaged markers was rewritten and THEN reported as "nothing could be
    # synced" — a message the code made false by writing first.
    #
    # It mattered little while every writable span was a single line rewritten
    # in place. It matters now: a writable BLOCK is spliced, its length can
    # change, and the span offsets come from the same scan that recorded the
    # error. Two roster blocks where the inventory declares one, or a marker
    # left nested by a merge, would be spliced from indices the errors say not
    # to trust — inside `make generate`, against a tracked file.
    #
    # So the rule is the plain one a generation pass should already obey: a run
    # that cannot vouch for what it read changes nothing on disk.
    unless errors.empty?
      warn "ERROR: doc-constant markers are malformed; nothing was synced:"
      errors.each { |e| warn "  #{e}" }
      return 1
    end

    # ...and EVERY DEFERRED INPUT THE WRITE LOOP CAN DEMAND is resolved here,
    # before any file is opened, for exactly the same reason one paragraph up.
    #
    # This is the hole the structural preflight above did NOT close, and it is
    # worth naming rather than quietly fixing: the bodies were lambdas called
    # inside the splice, so a malformed roster raised in the middle of the write
    # loop. Files are walked in scan order, so COORDINATION.md's stale pin was
    # already rewritten by the time SPEC.md's block asked for a roster that
    # would not load — a partially modified checkout from a run that reported
    # failure. Moving one check earlier answered the errors scan_file records
    # and left every failure that only appears when the body is demanded.
    #
    # Forcing them up front turns that class into nothing: after this line the
    # write loop cannot raise for a reason it has not already discovered, so
    # "aborted" and "nothing written" mean the same thing again.
    #
    # Only what is actually MARKED is forced. A repo whose SPEC carries no roster
    # marker must not be made to load the roster file, and one with no
    # @operation-count span must not be made to count operations in an OpenAPI
    # document it never makes a claim about — which is what keeps this gate's own
    # crafted fixtures minimal.
    #
    # This is the third report of one defect, which is why it is written as a
    # place rather than as two fixes. The block bodies were the reported
    # instance; `op_count` was the next one, raising from inside `rewrite_line`
    # after an earlier stale file had been written. A fourth deferred input would
    # have gone the same way, so it now has an obvious home, and `rewrite_line`
    # takes values rather than thunks so the loop cannot reach a computation at
    # all.
    #
    # @service-count is the fourth, and it arrived as an entry here rather than
    # as a fifth way to raise mid-loop — which is the whole return on writing
    # this as a place. It reads two generated SDK files that a crafted fixture
    # need not carry, so it is forced only when a span actually claims it, by
    # exactly the same marked-only rule as the rest.
    marked_writable_blocks = spans.select(&:block).map(&:kind).uniq & WRITABLE_BLOCK_KINDS
    rendered_blocks = marked_writable_blocks.to_h { |kind| [kind, block_bodies.fetch(kind).call] }
    marked_ticked_kinds = spans.reject(&:block).map(&:kind).uniq & TICKED_INT_KINDS
    ticked_counts = marked_ticked_kinds.to_h { |kind| [kind, ticked_count_sources.fetch(kind).call] }

    written = []
    declined = []
    spans.group_by(&:file).each do |file, file_spans|
      writable = file_spans.reject(&:block).select { |s| LINE_KINDS.include?(s.kind) }
      writable_blocks = file_spans.select(&:block)
                                  .select { |s| WRITABLE_BLOCK_KINDS.include?(s.kind) }
      next if writable.empty? && writable_blocks.empty?

      path = File.join(ROOT, file)
      original = File.read(path, encoding: UTF8)
      lines = original.lines
      writable.each do |span|
        index = span.line_no - 1
        body = lines[index].chomp("\n")
        newline = lines[index].end_with?("\n") ? "\n" : ""
        lines[index] = rewrite_line(span.kind, body,
                                    api_version: api_version, revision: revision, date: date,
                                    ticked_counts: ticked_counts) + newline
      end

      # Blocks are spliced LAST and from the bottom up. A rendered block rarely
      # has the same line count as the one it replaces, so every span below it
      # in the same file would be off by the difference — including the line
      # spans just rewritten by index above. Descending order means each splice
      # only ever moves lines that have already been written.
      #
      # "Last" is what the self-test exercises; the ORDER is not, because one
      # writable block exists and a single splice cannot be out of order. It is
      # written this way so the second one does not have to notice.
      writable_blocks.sort_by { |span| -span.line_no }.each do |span|
        start = span.line_no - 1
        lines[start, span.lines.length] = rendered_blocks.fetch(span.kind).map { |l| "#{l}\n" }
      end

      updated = lines.join
      next if updated == original

      # Excluded files are diffed but never written. Reaching here means the
      # file IS stale, so say so loudly — this runs inside `make generate`, and
      # a silent skip would read as "nothing to do" right up until `make check`
      # failed for reasons the generate output never mentioned.
      if writer_excludes.key?(file)
        declined << file
        next
      end

      File.write(path, updated, encoding: UTF8)
      written << file
    end

    if written.empty?
      writable_count = spans.count do |s|
        s.block ? WRITABLE_BLOCK_KINDS.include?(s.kind) : LINE_KINDS.include?(s.kind)
      end
      puts "Doc constants already in sync (#{writable_count} marked spans)."
    else
      written.each { |file| puts "Rewrote marked doc constants in #{file}" }
    end

    declined.each do |file|
      warn ""
      warn "NOTE: #{file} has stale marked doc constants and was NOT rewritten."
      warn "      Reason it is in spec/doc-constants.json .writerExcludes:"
      warn "        #{writer_excludes[file]}"
      warn "      Update it by hand, along with the prose that depends on it."
      warn "      `make doc-constants-check` stays red until you do."
    end

    return 0
  end

  schema = read_json("conformance/schema.json")
  schema_types = dig!(schema, "conformance/schema.json",
                      "properties", "assertions", "items", "properties", "type", "enum")

  # Derived lazily for op_count's reason: a repo carrying no fixture roster has
  # no business shelling out to git for a list it never consults.
  fixtures_memo = nil
  fixtures = lambda do
    fixtures_memo ||= tracked_fixtures
  end

  spans.each do |span|
    errors.concat(
      case span.kind
      when "api-version"         then check_api_version(span, api_version, openapi)
      when "bc3-pin"             then check_bc3_pin(span, revision, date)
      when "assertion-types"     then check_assertion_types(span, schema_types)
      when "operation-count"
        check_ticked_count(span, op_count.call, "#{openapi} has #{op_count.call} operations")
      when "service-count"
        check_ticked_count(span, services.call.length,
                           "the generated accessors expose #{services.call.length} " \
                           "account-scoped services")
      when "fixture-categories"  then check_fixture_categories(span, fixtures.call)
      when "fixture-section-map" then check_fixture_section_map(span, fixtures.call)
      when "zero-skip-roster"    then check_zero_skip_roster(span, roster_lines.call)
      when "account-scoped-services" then check_account_scoped_services(span, services.call)
      else []
      end
    )
  end

  grant_errors, grants = validate_citation_grants(pin_citations)
  errors.concat(grant_errors)
  malformed = pin_citations.keys - grants.keys

  # Every granted file is checked even if git no longer tracks it: a grant for
  # a deleted file is dead weight that would spring back to life, unreviewed,
  # the day someone recreates the path.
  ((prose_by_file.keys | grants.keys) - malformed).each do |file|
    errors.concat(check_unmarked_pin(file, prose_by_file.fetch(file, []), revision, grants))
  end

  if errors.empty?
    puts "Doc constants match their sources (#{spans.length} marked spans across " \
         "#{spans.map(&:file).uniq.length} files)."
    puts "  api-version      #{api_version}"
    puts "  bc3-pin          #{revision[0, 8]} (#{date})"
    puts "  assertion-types  #{schema_types.length}"
    puts "  operation-count  #{op_count.call}" if spans.any? { |s| s.kind == "operation-count" }
    if spans.any? { |s| %w[service-count account-scoped-services].include?(s.kind) }
      puts "  services         #{services.call.length} account-scoped " \
           "(Kotlin and Swift accessors agree)"
    end
    if spans.any? { |s| %w[fixture-categories fixture-section-map].include?(s.kind) }
      puts "  fixtures         #{fixtures.call.length} tracked under conformance/tests/"
    end
    if spans.any? { |s| s.kind == "zero-skip-roster" }
      skips = roster.call.values.sum { |section| section.skips.length }
      puts "  zero-skip-roster #{skips} skip(s) across " \
           "#{ZeroSkipRoster::RUNNERS.length} runners (block rendered from " \
           "#{ZeroSkipRoster::RELATIVE_PATH})"
    end
    0
  else
    warn "ERROR: documentation constants have drifted from their sources."
    errors.each { |e| warn "  #{e}" }
    warn ""
    warn "  Scalar constants: run `make sync-api-version` to rewrite marked spans."
    # Naming the excluded files here matters: for those, "run make
    # sync-api-version" is advice that silently does nothing, and a developer
    # who follows it and sees no diff learns to distrust the gate.
    writer_excludes.each do |file, reason|
      warn "    — except #{file}, which the writer never edits (#{reason});"
      warn "      fix it by hand."
    end
    warn "  Assertion types:  edit SPEC.md §19's marked table by hand — a new row " \
         "needs a description only a human can write."
    warn "  Fixture rosters:  a new conformance/tests fixture needs a row in BOTH SPEC.md §19's " \
         "Test Categories"
    warn "                    table and Appendix D. Neither is auto-written: the owning-section " \
         "and case-summary"
    warn "                    columns are attributions only the fixture's author can make."
    warn "  Zero-skip roster: SPEC §19's marked block is RENDERED from " \
         "#{ZeroSkipRoster::RELATIVE_PATH}."
    warn "                    Edit the YAML, then `make sync-api-version` to rewrite the block. " \
         "It is the"
    warn "                    one block the writer authors, because none of it is hand-written."
    warn "  Service roster:   SPEC §5's and Appendix B's marked rosters are one comma-separated " \
         "line each,"
    warn "                    regenerated from the generated accessors:"
    warn "                      rg -o 'public var (\\w+): \\w+Service' -r '$1' \\"
    warn "                        #{SWIFT_ACCESSORS} | paste -sd, - | sed 's/,/, /g'"
    warn "  Unmarked pin:     see the A/B rule in AGENTS.md §Provenance is Mandatory."
    1
  end
end

mode = :check
openapi = "openapi.json"
args = ARGV.dup
until args.empty?
  case args.shift
  when "--check" then mode = :check
  when "--write" then mode = :write
  when "--openapi"
    openapi = args.shift
    if openapi.nil? || openapi.empty?
      warn "ERROR: --openapi needs a path"
      exit 2
    end
  else
    warn "usage: #{$PROGRAM_NAME} [--check|--write] [--openapi PATH]"
    exit 2
  end
end

begin
  exit run(mode, openapi)
rescue Failure => e
  warn "ERROR: #{e.message}"
  exit 2
end
