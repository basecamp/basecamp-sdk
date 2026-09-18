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
# Paths default to the repo layout but honour the REQUIRED_TAGS_OPENAPI and
# REQUIRED_TAGS_ALLOWLIST env overrides so the negative-case self-test
# (scripts/test-check-required-tags.rb) can point it at crafted inputs. Stdlib
# only, wired into `make check`.

require "json"
require "set"

PROJECT_ROOT = File.expand_path("..", __dir__)
OPENAPI_FILE = ENV.fetch("REQUIRED_TAGS_OPENAPI", File.join(PROJECT_ROOT, "openapi.json"))

# All eight HTTP methods an OpenAPI Path Item Object may carry. Anything else at
# that level (`parameters`, `summary`, `servers`, `$ref`, `x-*`) is not an
# operation. The list has to be complete or an operation on an unlisted verb
# slips past a gate whose whole claim is "every operation"; it matches
# scripts/check-projected-examples.rb.
HTTP_METHODS = %w[get put post delete options head patch trace].freeze

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
  seen = 0

  paths.each do |path, methods|
    next unless methods.is_a?(Hash)

    methods.each do |method, operation|
      next unless HTTP_METHODS.include?(method.to_s.downcase)
      next unless operation.is_a?(Hash)

      seen += 1
      op_id = operation["operationId"] || "#{method.upcase} #{path}"
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
  die "openapi file declared no HTTP operations" if seen.zero?

  problems = []
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
    exit 1
  end

  puts "required-tags: #{seen} operations, all carry exactly one tag"
end

main if $PROGRAM_NAME == __FILE__
