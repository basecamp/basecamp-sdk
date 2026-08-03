#!/usr/bin/env ruby
# frozen_string_literal: true

# Doc-constant drift gate + writer.
#
# Three constants are restated in prose and drift silently from their sources:
#
#   @api-version      openapi.json  .info.version
#   @bc3-pin          spec/api-provenance.json  .bc3.revision / .bc3.date
#   @assertion-types  conformance/schema.json
#                       .properties.assertions.items.properties.type.enum
#
# Only spans MARKED with an HTML comment are checked. That is deliberate:
# spec/api-gaps/ legitimately cites ~20 historical bc3 SHAs in narrative
# ("the pin has since advanced to X", "the A..B range contains..."), and a
# naive "every SHA must equal the current pin" gate would rewrite settled
# history into a claim nobody verified. A marker means "this is a claim about
# the CURRENT value" and nothing else.
#
# Marker syntax
#   Line span:   <!-- @api-version -->  or  <!-- @bc3-pin -->
#                anywhere on the line; the span is exactly that line.
#   Block span:  <!-- @assertion-types:begin --> ... <!-- @assertion-types:end -->
#                the span is the lines strictly between them.
#
# Deleting a marker to silence the gate is itself caught: spec/doc-constants.json
# commits a per-file marker-count FLOOR, and dropping below it fails.
#
# Modes
#   --check (default)  Report drift; exit 1 on any error.
#   --write            Rewrite marked spans in place from the sources.
#
#   --openapi PATH     Read the API version from PATH instead of ./openapi.json.
#                      sync-api-version.sh forwards its own documented
#                      [openapi.json] argument here; without that, one sync
#                      could set the SDK constants from a caller-supplied file
#                      and the prose from the repo's, leaving them disagreeing.
#                      Only the two scalar constants are writable — an
#                      assertion-type row needs a human-written description,
#                      so --write never touches @assertion-types and never
#                      fails on it. `make doc-constants-check` is what catches
#                      that one; keeping it out of the writer keeps a schema
#                      edit from breaking every `make generate`.
#
# liveAssertions (.properties.liveAssertions...) is deliberately NOT gated:
# SPEC §19 does not tabulate it, and the live canary is governed by
# CONTRIBUTING.md. Add a marked table there first if that changes.

require "json"

ROOT = File.expand_path("..", __dir__)

LINE_KINDS  = %w[api-version bc3-pin].freeze
BLOCK_KINDS = %w[assertion-types].freeze
KNOWN_KINDS = (LINE_KINDS + BLOCK_KINDS).freeze

MARKER_RE   = /<!--\s*@([a-z0-9][a-z0-9-]*)(?::(begin|end))?\s*-->/
ISO_DATE_RE = /\b\d{4}-\d{2}-\d{2}\b/
TICKED_HEX_RE = /`([0-9a-f]{7,40})`/
BACKTICKED_RE = /`[^`]*`/

class Failure < StandardError; end

def read_json_at(path, label)
  raise Failure, "missing #{label}" unless File.exist?(path)

  JSON.parse(File.read(path))
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
  out = IO.popen(["git", "-C", ROOT, "ls-files", "-z", "--", "*.md"], &:read)
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

# Returns [spans, errors]. Line markers yield a one-line span; block markers
# yield the lines strictly between :begin and :end.
def scan_file(file)
  spans = []
  errors = []
  lines = File.readlines(File.join(ROOT, file), chomp: true)
  open_block = nil
  in_fence = false

  lines.each_with_index do |line, index|
    line_no = index + 1

    if line.lstrip.start_with?("```", "~~~")
      in_fence = !in_fence
      next
    end
    next if in_fence

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

  [spans, errors]
end

# --- per-kind checkers -------------------------------------------------------

def check_api_version(span, api_version, source)
  dates = span.text.scan(ISO_DATE_RE)
  return ["#{span.location}: @api-version span states no YYYY-MM-DD version"] if dates.empty?

  dates.uniq.reject { |d| d == api_version }.map do |bad|
    "#{span.location}: @api-version says #{bad}, #{source} .info.version is #{api_version}"
  end
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

  # A SHA that lost its backticks is invisible to the writer's rewrite, so it
  # would sit stale forever while the gate reported green. Reject it outright.
  text.gsub(BACKTICKED_RE, " ").scan(/\b[0-9a-f]{7,40}\b/).uniq.each do |bare|
    errors << "#{span.location}: bare SHA #{bare} in a @bc3-pin span — backtick it " \
              "so `make sync-api-version` can rewrite it"
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

def rewrite_line(kind, line, api_version:, revision:, date:)
  case kind
  when "api-version"
    line.gsub(ISO_DATE_RE, api_version)
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
  api_version = dig!(read_openapi(openapi), openapi, "info", "version")

  provenance = read_json("spec/api-provenance.json")
  revision = dig!(provenance, "spec/api-provenance.json", "bc3", "revision")
  date     = dig!(provenance, "spec/api-provenance.json", "bc3", "date")

  unless revision.match?(/\A[0-9a-f]{40}\z/)
    raise Failure, "spec/api-provenance.json .bc3.revision is not a full 40-char SHA: #{revision}"
  end

  config = read_json("spec/doc-constants.json")
  floors = dig!(config, "spec/doc-constants.json", "markerFloors")

  errors = []
  spans = []
  tracked_markdown.each do |file|
    file_spans, file_errors = scan_file(file)
    spans.concat(file_spans)
    errors.concat(file_errors)
  end

  # Marker-count floor: deleting a marked claim to silence the gate fails here.
  counts = Hash.new(0)
  spans.each { |s| counts[[s.kind, s.file]] += 1 }
  floors.each do |kind, per_file|
    unless KNOWN_KINDS.include?(kind)
      errors << "spec/doc-constants.json: floor declared for unknown marker @#{kind}"
      next
    end
    per_file.each do |file, floor|
      found = counts[[kind, file]]
      next if found >= floor

      errors << "#{file}: marker floor violated — spec/doc-constants.json requires at least " \
                "#{floor} @#{kind} marker(s), found #{found}. A current-value claim was deleted " \
                "or unmarked; if it genuinely moved, move the floor in the same commit."
    end
  end

  if mode == :write
    written = []
    spans.group_by(&:file).each do |file, file_spans|
      writable = file_spans.reject(&:block).select { |s| LINE_KINDS.include?(s.kind) }
      next if writable.empty?

      path = File.join(ROOT, file)
      original = File.read(path)
      lines = original.lines
      writable.each do |span|
        index = span.line_no - 1
        body = lines[index].chomp("\n")
        newline = lines[index].end_with?("\n") ? "\n" : ""
        lines[index] = rewrite_line(span.kind, body,
                                    api_version: api_version, revision: revision, date: date) + newline
      end
      updated = lines.join
      next if updated == original

      File.write(path, updated)
      written << file
    end

    if written.empty?
      puts "Doc constants already in sync (#{spans.count { |s| LINE_KINDS.include?(s.kind) }} marked spans)."
    else
      written.each { |file| puts "Rewrote marked doc constants in #{file}" }
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
      else []
      end
    )
  end

  if errors.empty?
    puts "Doc constants match their sources (#{spans.length} marked spans across " \
         "#{spans.map(&:file).uniq.length} files)."
    puts "  api-version      #{api_version}"
    puts "  bc3-pin          #{revision[0, 8]} (#{date})"
    puts "  assertion-types  #{schema_types.length}"
    0
  else
    warn "ERROR: documentation constants have drifted from their sources."
    errors.each { |e| warn "  #{e}" }
    warn ""
    warn "  Scalar constants: run `make sync-api-version` to rewrite marked spans."
    warn "  Assertion types:  edit SPEC.md §19's marked table by hand — a new row " \
         "needs a description only a human can write."
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
