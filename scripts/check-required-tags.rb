#!/usr/bin/env ruby
# frozen_string_literal: true

# Required-tags guard (fail-closed).
#
# Every operation in openapi.json must carry EXACTLY ONE tag. basecamp/mcp's
# catalog.Load groups SDK operations into MCP domain tools by their tag, one tag
# per operation; the SDK generators likewise route service grouping off tags[0].
# An operation that reaches openapi.json with no tag is silently dropped or
# folded into an unrelated domain (the drift that folded the recordings and
# event-feed surfaces into basecamp_admin/basecamp_schedules — #878, #898). An
# operation with two tags is ambiguous for the same consumers.
#
# This is an ACCIDENT-class guard: it stops a new operation added to the Smithy
# model without a matching entry in spec/overlays/tags.smithy from shipping
# untagged, and it stops a tag from being deleted or doubled by an edit nobody
# noticed. There is no adversary — anyone who can add an operation can add a tag;
# the guard exists so they cannot do the first and forget the second.
#
# It reads the COMMITTED openapi.json, the same artifact catalog.Load consumes.
# The smithy-verify workflow's "OpenAPI is up to date" step already proves that
# file is regenerated from the Smithy model, so a model change that drops a tag
# reaches this gate through a regenerated openapi.json rather than hiding behind
# a stale one.
#
# ALLOWLIST: empty. Every Basecamp API operation is a catalog operation and must
# carry a domain tag. If a genuinely tag-less operation ever arises, add its
# operationId to ALLOWLIST below WITH a reason and a tracking reference — do not
# widen the check to accept absent tags in general.
#
# WHAT THIS GATE DOES NOT SEE. It reads `paths` only. A 3.1 document may also
# carry operations under `webhooks` or `components.pathItems`; this artifact has
# neither, and the check would not notice if it grew one. It also does not judge
# whether the single tag is the RIGHT tag, only that there is exactly one. And
# the six per-language generators still iterate a five-verb list of their own, so
# an operation on any other verb would be dropped from the generated services
# outright rather than merely shipping untagged — a larger hole, one cross-SDK
# regeneration away from this file, and deliberately not addressed here.
#
# Paths default to the repo layout but honour the REQUIRED_TAGS_OPENAPI and
# REQUIRED_TAGS_ALLOWLIST env overrides so the negative-case self-test
# (scripts/test-check-required-tags.rb) can point it at crafted inputs. Stdlib
# only, wired into `make check`.

require "json"
require "set"

PROJECT_ROOT = File.expand_path("..", __dir__)
OPENAPI_FILE = ENV.fetch("REQUIRED_TAGS_OPENAPI", File.join(PROJECT_ROOT, "openapi.json"))

# A Path Item Object is identified by what it is NOT. Its non-operation fields
# are a closed, spec-defined set; specification extensions are `x-` prefixed.
# EVERY other field is an operation.
#
# Enumerating the operation verbs instead would fail open in the one dimension
# this gate exists to hold. The list would have to be complete forever: OpenAPI
# 3.0 added `trace`, 3.2 adds `query`, and Smithy's own `@http` trait takes the
# method as a free-form string it "will use literally and will perform no
# validation on" — so this repo's generator can put any key in a path item, and
# a verb list would skip it in silence. Inverted, an unfamiliar field is caught
# and named instead of stepped over.
#
# That inversion is already earning its keep in one known case: OpenAPI 3.2 adds
# `additionalOperations`, a MAP of method to Operation. This artifact declares
# 3.1.0, so the case is unreachable today; were it to appear, the check reads it
# as one untagged operation and fails naming the field, which is the outcome
# worth having. Teach it the map shape then — not now, on spec.
NON_OPERATION_FIELDS = %w[$ref summary description servers parameters].freeze

def non_operation_field?(field)
  NON_OPERATION_FIELDS.include?(field) || field.start_with?("x-")
end

# operationIds permitted to carry no tag. Keep empty; see the header note.
ALLOWLIST = [].freeze

# The self-test extends the allowlist with crafted operationIds to exercise the
# allowlist path; production runs never set this.
def allowlist
  extra = ENV["REQUIRED_TAGS_ALLOWLIST"].to_s.split(",").map(&:strip).reject(&:empty?)
  (ALLOWLIST + extra).to_set
end

def die(message)
  warn "ERROR: #{message}"
  exit 1
end

def main
  die "openapi file not found: #{OPENAPI_FILE} (run 'make smithy-build')" unless File.file?(OPENAPI_FILE)

  spec =
    begin
      JSON.parse(File.read(OPENAPI_FILE, encoding: "UTF-8"))
    rescue JSON::ParserError => e
      die "openapi file is not valid JSON: #{e.message}"
    end

  paths = spec["paths"]
  die "openapi file has no paths object" unless paths.is_a?(Hash) && !paths.empty?

  permitted = allowlist
  untagged = []
  multi_tagged = []
  unreadable = []
  seen = 0

  paths.each do |path, item|
    unless item.is_a?(Hash)
      unreadable << "#{path} (path item is #{item.class}, expected an object)"
      next
    end

    item.each do |field, operation|
      next if non_operation_field?(field.to_s)

      # Not a known non-operation field, so it is an operation. A value that is
      # not an object cannot be one — say so rather than stepping over it.
      unless operation.is_a?(Hash)
        unreadable << "#{path} -> #{field} (#{operation.class}, expected an operation object)"
        next
      end

      seen += 1
      op_id = operation["operationId"] || "#{field.upcase} #{path}"
      tags = operation["tags"]

      if tags.nil? || tags.empty?
        untagged << op_id unless permitted.include?(op_id)
      elsif tags.length > 1
        multi_tagged << "#{op_id} (#{tags.join(', ')})"
      end
    end
  end

  # Fail closed on a spec that yielded no operations at all — a broken or
  # truncated openapi.json must not pass this gate vacuously.
  die "openapi file declared no HTTP operations" if seen.zero? && unreadable.empty?

  problems = []
  unless unreadable.empty?
    problems << "#{unreadable.length} path item field(s) could not be read as an operation:\n" \
                "#{unreadable.sort.map { |o| "  - #{o}" }.join("\n")}"
  end
  unless untagged.empty?
    problems << "#{untagged.length} operation(s) carry no tag (each must have exactly one):\n" \
                "#{untagged.sort.map { |o| "  - #{o}" }.join("\n")}"
  end
  unless multi_tagged.empty?
    problems << "#{multi_tagged.length} operation(s) carry more than one tag (catalog.Load requires exactly one):\n" \
                "#{multi_tagged.sort.map { |o| "  - #{o}" }.join("\n")}"
  end

  unless problems.empty?
    warn "ERROR: required-tags check failed."
    problems.each { |p| warn p }
    warn "\nTag each operation in spec/overlays/tags.smithy and run 'make smithy-build', " \
         "or, for a genuinely tag-less operation, add its operationId to ALLOWLIST in " \
         "scripts/check-required-tags.rb with a reason."
    warn "A field named above that you did not expect to be an operation is either a " \
         "path item verb this spec had not used before (tag it) or a new non-operation " \
         "field from a later OpenAPI version (add it to NON_OPERATION_FIELDS with a reason)."
    exit 1
  end

  puts "required-tags: #{seen} operations, all carry exactly one tag"
end

main if $PROGRAM_NAME == __FILE__
