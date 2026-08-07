#!/usr/bin/env ruby
# frozen_string_literal: true

# Doc-constant drift gate + writer.
#
# Three constants are restated in prose and drift silently from their sources:
#
#   @api-version      openapi.json  .info.version
#   @bc3-pin          spec/api-provenance.json  .bc3.revision / .bc3.date
#   @assertion-types  conformance/schema.json
#   @operation-count  openapi.json  count of path × HTTP-method pairs
#                       .properties.assertions.items.properties.type.enum
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
#                      Only the two scalar constants are writable — an
#                      assertion-type row needs a human-written description,
#                      so --write never touches @assertion-types and never
#                      fails on it. `make doc-constants-check` is what catches
#                      that one; keeping it out of the writer keeps a schema
#                      edit from breaking every `make generate`.
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

LINE_KINDS  = %w[api-version bc3-pin operation-count].freeze

# The OpenAPI verbs an operation can be keyed under. Anything else in a path
# item (parameters, servers, summary) is not an operation and must not count.
HTTP_METHODS = %w[get put post delete patch head options trace].freeze
BLOCK_KINDS = %w[assertion-types].freeze
KNOWN_KINDS = (LINE_KINDS + BLOCK_KINDS).freeze

MARKER_RE   = /<!--\s*@([a-z0-9][a-z0-9-]*)(?::(begin|end))?\s*-->/
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
  lines = File.readlines(File.join(ROOT, file), chomp: true, encoding: UTF8)
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

  covered = Set.new
  spans.each { |s| (s.line_no...(s.line_no + s.lines.length)).each { |n| covered << n } }
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
  paths.sum { |_path, ops| ops.count { |method, _| HTTP_METHODS.include?(method) } }
end

# A code span whose ENTIRE content is digits. Bare prose integers are not
# candidates, because the marked lines are full of them — SECURITY.md's states
# 125 GETs and 83 mutations in the same sentence as the total — and a checker
# that read those as the claim would fail on numbers it has no source for.
# Backticks are how the prose says "this one is the derived constant", the same
# device @bc3-pin uses for the SHA.
TICKED_INT_RE = /`(\d+)`/

def check_operation_count(span, count, source)
  ints = span.text.scan(TICKED_INT_RE).flatten.uniq

  if ints.empty?
    return ["#{span.location}: @operation-count span states no backticked integer — " \
            "write the count as `#{count}` so the writer can find it"]
  end

  # Exactly one, so the writer never has to guess which integer to rewrite. A
  # line that needs another backticked integer cannot carry this marker; put the
  # claim on a line of its own instead.
  if ints.length > 1
    return ["#{span.location}: @operation-count span has #{ints.length} backticked integers " \
            "(#{ints.join(', ')}) — exactly one is required, so the writer knows which is the count"]
  end

  return [] if ints.first == count.to_s

  ["#{span.location}: @operation-count says `#{ints.first}`, #{source} has #{count} operations"]
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

# --- writer ------------------------------------------------------------------

def rewrite_line(kind, line, api_version:, revision:, date:, operation_count_value:)
  case kind
  when "api-version"
    line.gsub(ISO_DATE_RE, api_version)
  when "operation-count"
    line.gsub(TICKED_INT_RE) { "`#{operation_count_value.call}`" }
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
    written = []
    declined = []
    spans.group_by(&:file).each do |file, file_spans|
      writable = file_spans.reject(&:block).select { |s| LINE_KINDS.include?(s.kind) }
      next if writable.empty?

      path = File.join(ROOT, file)
      original = File.read(path, encoding: UTF8)
      lines = original.lines
      writable.each do |span|
        index = span.line_no - 1
        body = lines[index].chomp("\n")
        newline = lines[index].end_with?("\n") ? "\n" : ""
        lines[index] = rewrite_line(span.kind, body,
                                    api_version: api_version, revision: revision, date: date,
                                    operation_count_value: op_count) + newline
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
      puts "Doc constants already in sync (#{spans.count { |s| LINE_KINDS.include?(s.kind) }} marked spans)."
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

    # Structural marker problems are still fatal in --write: they mean the
    # writer could not see a span it was supposed to maintain.
    unless errors.empty?
      warn "ERROR: doc-constant markers are malformed; nothing could be synced for these:"
      errors.each { |e| warn "  #{e}" }
      return 1
    end
    return 0
  end

  schema = read_json("conformance/schema.json")
  schema_types = dig!(schema, "conformance/schema.json",
                      "properties", "assertions", "items", "properties", "type", "enum")

  spans.each do |span|
    errors.concat(
      case span.kind
      when "api-version"     then check_api_version(span, api_version, openapi)
      when "bc3-pin"         then check_bc3_pin(span, revision, date)
      when "assertion-types" then check_assertion_types(span, schema_types)
      when "operation-count" then check_operation_count(span, op_count.call, openapi)
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
