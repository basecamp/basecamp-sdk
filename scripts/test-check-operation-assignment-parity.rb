#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-operation-assignment-parity.
#
# The gate's own `make check` run only ever exercises the PASSING case, so nothing
# there proves it rejects anything. This repo treats an ungated gate as no gate,
# and a parity gate that cannot fail is worse than none: it converts "nobody
# compared them" into "the gate says they agree".
#
# Every case drives the real checker through its one env seam
# (OPERATION_ASSIGNMENT_ROOT) against a synthetic repository tree in a tmpdir. The
# tracked tree is never written to.
#
# THE SYNTHETIC TREE IS ITSELF UNDER TEST, the same way
# test-check-service-inventory-parity.rb's is. It is built by INVERTING what the
# checker reads: five class-declaration syntaxes, every request helper the checker
# declares a pattern for, and all four path-interpolation spellings (`{p}`,
# `${p}`, `#{p}`, `\(p)`). A builder that spelled anything differently from the
# real generators would make every negative case below fail for a reason
# unrelated to its mutation, so the synthetic positive control runs second and is
# as load-bearing as the real one.
#
# THE ROSTER IS SCRAPED, NOT WRITTEN DOWN. operationId -> (method, path) comes
# from openapi.json and operationId -> service from the real Swift service
# filenames plus their `operation:` strings. A literal would be a sixth hand-copy
# of the very table this gate exists because there are already five of. The scrape
# is deliberately NOT the checker's own extraction: it keys on operationId, which
# the checker never reads out of an SDK, so the builder and the thing it drives
# do not share a reader.
#
# EXERCISING EVERY DECLARED HELPER IS THE POINT OF THE ROTATION below. The
# checker declares 13 request-call patterns across the five sources; a pattern
# that matches nothing is a dead rule, indistinguishable from a stale carve-out.
# The builder rotates helper choice by the operation's index so each pattern is
# hit, which makes the synthetic positive control fail if any of them is deleted
# or misspelled.
#
# OPERATION_ASSIGNMENT_CHECKER names the checker under test (default
# scripts/check-operation-assignment-parity), so a deliberately-mutated copy can
# be driven through the same suite to prove the suite is not vacuous:
#
#   cp scripts/check-operation-assignment-parity /tmp/m       # mutate the COPY
#   OPERATION_ASSIGNMENT_CHECKER=/tmp/m ruby scripts/test-check-operation-assignment-parity.rb
#
# The copy resolves its sources from OPERATION_ASSIGNMENT_ROOT, which every case
# sets, so it needs nothing beside it. Never mutate the tracked file.
#
# Every guard in the checker is pinned by at least one case, and the mapping below
# is MEASURED — mutate the guard in a copy, record which cases go red — rather
# than reasoned about. Removing a guard from the copy must turn exactly this red:
#
#   guard mutated in the copy                       cases that go red
#   ---------------------------------------------   -----------------------------
#   the parity comparison                           1, 2, 3
#   parity keyed on GROUPING, not on the name       3 (checker PASSES; see below)
#   the completeness comparison (missing)           4, 5
#   the completeness comparison (extra)             6
#   the duplicate-key check                         7
#   Python sync/async agreement check               8
#   source-directory-exists check                   9
#   openapi.json-exists check                       10
#   account-prefix assertion                        11
#   MIN_OPERATIONS floor on the anchor              12
#   completeness reported BEFORE parity             4, 5
#   the TypeScript baseUrl-concatenation pattern    both controls, 1, 2, 3, 13
#   any other request-call pattern (10 measured)    both controls, 1, 2, 3, 13
#   the `?`-suffix strip in normalize_path          both controls, 1, 2, 3, 13
#   any of the four interpolation substitutions     both controls, 1, 2, 3, 13
#   `sub` -> `gsub` on the service-suffix strip     NOTHING — see below
#
# THE BLUNT ROWS, AND WHY THEY ARE NOT THE PINS. Deleting any request-call pattern
# turns both controls red plus 1, 2, 3 and 13, because a checker that cannot read
# one SDK completely fails every case that expected some other verdict. Those
# rows are smoke alarms, not pins. The pins are the sharp rows above them, and
# they were MEASURED to be sharp: removing the parity comparison turns EXACTLY 1,
# 2 and 3 red and nothing else; removing the duplicate-key check turns EXACTLY 7
# red. What the blunt rows do buy is dead-rule detection — a pattern that matches
# nothing is indistinguishable from a stale carve-out, and the rotation in the
# builder is what makes all 13 of them load-bearing.
#
# ONE MUTATION SURVIVES ON PURPOSE, and it is recorded rather than papered over.
# Changing the service-suffix strip from `sub` to `gsub` turns no case red, because
# the strip is cosmetic: all five generators spell the class identically, so any
# consistent rule leaves all five agreeing. No case is added for it, because a
# case that cannot distinguish the two would be decoration. The checker's header
# says the same thing at the code.
#
# TWO CASES PIN A MESSAGE RATHER THAN A VERDICT (4 and 5), which is deliberate. An
# operation read out of four SDKs and not the fifth is an extraction or emission
# problem, and describing it as an assignment disagreement would send the reader
# to five generator configs when the thing to look at is one regex. Both are
# pinned as ABSENCES (`expect_fail_without`), because the only way to hold a
# diagnostic honest is to assert what it must NOT say. Case 13 is the mirror: it
# expects a PASS, and pins that the gate answers ONE question — agreement — and
# does not smuggle in a fixed service roster, which is
# check-service-inventory-parity's question and not this one.
#
# Cases 1, 2 and 3 share the parity comparison and are not redundant: a
# single-SDK reassignment, a two-SDK reassignment (which is what the reported
# grouping has to render), and two classes whose contents were swapped wholesale.
#
# The third is the discriminator, and the grouping row above is its proof — read
# with care, because all three go red under that mutation and only one of them is
# the signal. Rekey the parity comparison on how operations are GROUPED rather
# than on the service NAME (the cheaper instrument someone will propose) and 1 and
# 2 go red on the MESSAGE — they still fail, the spread just no longer names the
# services — while case 3 goes red because the mutated checker PASSES OUTRIGHT.
# That is the only row in this file where a mutation makes the checker miss a real
# defect, and case 3 is the only case that catches it. Both blocks and the
# service-name set are unchanged in a wholesale swap, and
# check-service-inventory-parity compares name SETS rather than names to contents,
# so nothing else in `make` would see it.
#
# WHAT THIS SUITE IS NOT. The suite passing means these thirteen things are
# checked. It has never meant the list is complete.
#
# Run directly (`ruby scripts/test-check-operation-assignment-parity.rb`) or via
# `make test-check-operation-assignment-parity`.

require "fileutils"
require "json"
require "open3"
require "tmpdir"

# Per-case lines go to stdout and the failure report to stderr. Unsynced, stdout
# block-buffers when redirected to a file and the report lands ahead of the cases
# it summarizes — which is exactly when someone is reading the log.
$stdout.sync = true

ROOT = File.expand_path("..", __dir__)
CHECKER = File.expand_path(
  ENV.fetch("OPERATION_ASSIGNMENT_CHECKER", "scripts/check-operation-assignment-parity"), ROOT
)

TS_DIR = "typescript/src/generated/services"
RB_DIR = "ruby/lib/basecamp/generated/services"
PY_DIR = "python/src/basecamp/generated/services"
KT_DIR = "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/services"
SW_DIR = "swift/Sources/Basecamp/Generated/Services"
SW_REAL = File.join(ROOT, SW_DIR)
OPENAPI = "openapi.json"

def read_utf8(path) = File.read(path, encoding: "UTF-8")

def snake_case(name)
  name.gsub(/([a-z0-9])([A-Z])/, '\1_\2').gsub(/([A-Z]+)([A-Z][a-z])/, '\1_\2').downcase
end

def lower_camel(name)
  parts = snake_case(name).split("_")
  ([parts.first] + parts.drop(1).map(&:capitalize)).join
end

# --- The scraped roster --------------------------------------------------------
#
# operationId -> { method:, path:, service: }. `service` is the Pascal service
# name (`CardTables`), which is what every generator builds its class name from.

spec = JSON.parse(read_utf8(File.join(ROOT, OPENAPI)))
wire = {}
spec.fetch("paths").each do |path, item|
  item.each do |verb, operation|
    next unless %w[get put post delete patch].include?(verb)
    id = operation["operationId"]
    raise "openapi.json has an operation with no operationId at #{verb} #{path}" if id.nil?
    raise "openapi.json path #{path} is not account-scoped" unless path.start_with?("/{accountId}")
    wire[id] = { method: verb.upcase, path: path.sub("/{accountId}", "") }
  end
end

# Service assignment, read off the real Swift tree by operationId — a key the
# checker never extracts from any SDK, so this scrape shares no reader with it.
ROSTER = {}
Dir.children(SW_REAL).select { |f| f.end_with?("Service.swift") }.sort.each do |file|
  service = File.basename(file, "Service.swift")
  read_utf8(File.join(SW_REAL, file)).scan(/operation: "(\w+)"/).flatten.uniq.each do |id|
    raise "#{id} appears on two Swift services" if ROSTER.key?(id)
    raise "#{id} is on a Swift service but not in openapi.json" unless wire.key?(id)
    ROSTER[id] = wire.fetch(id).merge(service: service)
  end
end
ROSTER.freeze

raise "roster came back with #{ROSTER.length} operations; the real tree changed shape" \
  if ROSTER.length < 200
raise "roster is short of openapi.json (#{ROSTER.length} vs #{wire.length})" \
  unless ROSTER.length == wire.length

# The operations the cases below move around. Both services must survive the move
# with at least one operation left, or the mutation would also delete a service —
# a different failure, and one check-service-inventory-parity owns.
MOVED = "GetCard"
MOVED_TO = "CardTables"
raise "case setup needs #{MOVED} to exist" unless ROSTER.key?(MOVED)
FROM_SERVICE = ROSTER.fetch(MOVED).fetch(:service)
raise "#{MOVED} is already on #{MOVED_TO}" if FROM_SERVICE == MOVED_TO
raise "moving #{MOVED} would empty #{FROM_SERVICE}" \
  unless ROSTER.count { |_, o| o[:service] == FROM_SERVICE } > 1

def canonical(service) = service.downcase.gsub(/[^a-z0-9]/, "").sub(/service\z/, "")

# --- Synthetic tree builder ----------------------------------------------------
#
# One emitter per source, each inverting what the checker reads out of that SDK.
# `index` is the operation's position in the sorted roster and drives the helper
# rotation described in the header.

def interpolate(path, style)
  path.gsub(/\{(\w+)\}/) do
    param = Regexp.last_match(1)
    case style
    when :braces then "{#{param}}"
    when :dollar then "${#{param}}"
    when :hash   then "\#{#{snake_case(param)}}"
    when :paren  then "\\(#{param})"
    when :fstr   then "{#{snake_case(param)}}"
    else raise "unknown interpolation style #{style.inspect}"
    end
  end
end

def write_file(root, rel, body)
  path = File.join(root, rel)
  FileUtils.mkdir_p(File.dirname(path))
  File.write(path, body)
end

# The one generated TypeScript method that concatenates its URL instead of
# handing a literal to openapi-fetch. Named here for the same reason the checker
# names it: it is the only such site, and the pattern that reads it can only be
# pinned by a case that emits it.
TS_CONCAT_OPS = %w[UpdateAccountLogo].freeze

def ts_method(id, op, index)
  path = interpolate(op[:path], :braces)
  name = lower_camel(id)
  if TS_CONCAT_OPS.include?(id)
    <<~TS
        async #{name}(file: Blob): Promise<void> {
          const url = `${this.baseUrl}` +
            `#{path}`;
          return this.requestMultipartUpload(
            {
              service: "#{op[:service]}",
              operation: "#{id}",
              isMutation: true,
            },
            url,
            "#{op[:method]}",
            file,
          );
        }
    TS
  else
    <<~TS
        async #{name}(): Promise<unknown> {
          return this.request(
            { service: "#{op[:service]}", operation: "#{id}", isMutation: #{index.even?} },
            () => this.client.#{op[:method]}("#{path}", { params: {} })
          );
        }
    TS
  end
end

def rb_method(id, op, index)
  path = interpolate(op[:path], :hash)
  call =
    case [op[:method], index % 4]
    when ["GET", 0] then %(paginate("#{path}", operation: "#{id}"))
    when ["GET", 1] then %(paginate_wrapped("#{path}", "events", operation: "#{id}"))
    when ["POST", 0] then %(http_post_raw("#{path}?name=\#{URI.encode_www_form_component(name.to_s)}", body: nil))
    when ["PUT", 0] then %(http_put_multipart("#{path}", io: io, field: "logo"))
    else
      case op[:method]
      when "GET" then %(http_get("#{path}", operation: "#{id}"))
      when "PUT" then %(http_put("#{path}", body: nil))
      when "POST" then %(http_post("#{path}", body: nil))
      when "DELETE" then %(http_delete("#{path}"))
      when "PATCH" then %(http_patch("#{path}", body: nil))
      else raise "unhandled method #{op[:method]}"
      end
    end
  <<~RUBY
        def #{snake_case(id)}
          #{call}
        end
  RUBY
end

def py_body(id, op, index, indent)
  path = interpolate(op[:path], :fstr)
  info = %(OperationInfo(service="#{canonical(op[:service])}", operation="#{snake_case(id)}", is_mutation=#{index.even? ? 'True' : 'False'}))
  call =
    case [op[:method], index % 4]
    when ["GET", 0] then %(_request_paginated(\n    #{info},\n    f"#{path}",\n    operation="#{id}",\n))
    when ["GET", 1] then %(_request_paginated_wrapped(\n    #{info},\n    f"#{path}",\n    "events",\n    operation="#{id}",\n))
    when ["GET", 2] then %(_request_list(\n    #{info},\n    "#{path}",\n    operation="#{id}",\n))
    when ["POST", 0] then %(_request_raw(\n    #{info},\n    f"#{path}",\n    content=b"",\n    operation="#{id}",\n))
    when ["PUT", 0] then %(_request_multipart_void(\n    #{info},\n    "PUT",\n    f"#{path}",\n    field="logo",\n))
    when ["DELETE", 0], ["DELETE", 1], ["DELETE", 2], ["DELETE", 3]
      %(_request_void(\n    #{info},\n    "DELETE",\n    f"#{path}",\n))
    else
      %(_request(\n    #{info},\n    "#{op[:method]}",\n    f"#{path}",\n    operation="#{id}",\n))
    end
  call.lines.map { |l| l == "\n" ? l : "#{indent}#{l}" }.join
end

def py_method(id, op, index, async:)
  kw = async ? "async def" : "def"
  await = async ? "await " : ""
  <<~PY
      #{kw} #{snake_case(id)}(self):
          return #{await}self.#{py_body(id, op, index, '        ').lstrip}    )
  PY
end

def kt_method(id, op, index)
  path = interpolate(op[:path], :dollar)
  call =
    case [op[:method], index % 4]
    when ["GET", 0] then %(httpGet("#{path}" + qs, operationName = info.operation))
    when ["POST", 0] then %(httpPostBinary("#{path}", data, contentType))
    when ["PUT", 0] then %(httpPutMultipart("#{path}", data, contentType))
    else %(http#{op[:method].capitalize}("#{path}", operationName = info.operation))
    end
  <<~KT
        suspend fun #{lower_camel(id)}(): JsonElement {
            val info = OperationInfo(
                service = "#{op[:service]}",
                operation = "#{id}",
                isMutation = #{index.even?},
            )
            return request(info, {
                #{call}
            }) { body -> json.parseToJsonElement(body) }
        }
  KT
end

def sw_method(id, op, index)
  path = interpolate(op[:path], :paren)
  info = %(OperationInfo(service: "#{op[:service]}", operation: "#{id}", isMutation: #{index.even?}))
  call =
    case [op[:method], index % 4]
    when ["GET", 0]
      %(requestPaginated(\n            #{info},\n            path: "#{path}"\n        ))
    when ["GET", 1]
      %(requestPaginatedWrapped(\n            #{info},\n            path: "#{path}",\n            key: "events"\n        ))
    when ["DELETE", 0], ["DELETE", 1], ["DELETE", 2], ["DELETE", 3]
      %(requestVoid(\n            #{info},\n            method: "#{op[:method]}",\n            path: "#{path}"\n        ))
    else
      %(request(\n            #{info},\n            method: "#{op[:method]}",\n            path: "#{path}"\n        ))
    end
  <<~SW
        public func #{lower_camel(id)}() async throws -> Data {
            return try await #{call}
        }
  SW
end

# `plan` is a list of [operationId, op, index] already resolved to its
# (possibly reassigned) service, grouped by the class it must be emitted into.
def group(plan)
  plan.group_by { |_id, op, _i| op[:service] }.transform_values { |v| v.sort_by { |id, _, _| id } }
       .sort.to_h
end

def build_root(dir, plan:, async_plan:, openapi:)
  write_file(dir, OPENAPI, JSON.pretty_generate(openapi))

  group(plan).each do |service, ops|
    snake = snake_case(service)

    write_file(dir, "#{TS_DIR}/#{snake.tr('_', '-')}.ts", <<~TS)
      import { BaseService } from "../../services/base.js";

      export class #{service}Service extends BaseService {
      #{ops.map { |id, op, i| ts_method(id, op, i) }.join("\n")}
      }
    TS

    write_file(dir, "#{RB_DIR}/#{snake}_service.rb", <<~RUBY)
      # frozen_string_literal: true

      module Basecamp
        module Services
          class #{service}Service < BaseService
      #{ops.map { |id, op, i| rb_method(id, op, i) }.join("\n")}
          end
        end
      end
    RUBY

    write_file(dir, "#{KT_DIR}/#{snake.tr('_', '-')}.kt", <<~KT)
      package com.basecamp.sdk.generated.services

      class #{service}Service(client: AccountClient) : BaseService(client) {
      #{ops.map { |id, op, i| kt_method(id, op, i) }.join("\n")}
      }
    KT

    write_file(dir, "#{SW_DIR}/#{service}Service.swift", <<~SW)
      // @generated — do not edit
      import Foundation

      public final class #{service}Service: BaseService, @unchecked Sendable {
      #{ops.map { |id, op, i| sw_method(id, op, i) }.join("\n")}
      }
    SW
  end

  # Python is emitted from its own plan so a case can drift the async twin alone.
  sync = group(plan)
  async_ = group(async_plan)
  (sync.keys | async_.keys).sort.each do |service|
    snake = snake_case(service)
    body = +"# @generated — do not edit\n\nfrom basecamp.hooks import OperationInfo\n\n\n"
    body << "class #{service}Service(BaseService):\n"
    ops = sync.fetch(service, [])
    body << (ops.empty? ? "    pass\n" : ops.map { |id, op, i| py_method(id, op, i, async: false) }.join("\n"))
    body << "\n\nclass Async#{service}Service(AsyncBaseService):\n"
    aops = async_.fetch(service, [])
    body << (aops.empty? ? "    pass\n" : aops.map { |id, op, i| py_method(id, op, i, async: true) }.join("\n"))
    write_file(dir, "#{PY_DIR}/#{snake}.py", body)
  end

  # The non-service files each source drops, present so the drop rules are
  # exercised rather than merely declared.
  write_file(dir, "#{TS_DIR}/index.ts", "export * from \"./account.js\";\n")
  write_file(dir, "#{RB_DIR}/base_service.rb", <<~RUBY)
    module Basecamp
      module Services
        class BaseService
          # @!method http_get(path, params: {})
          # @!method http_put(path, body: nil)
          def paginate(...) = @client.paginate(...)
        end
      end
    end
  RUBY
  write_file(dir, "#{PY_DIR}/__init__.py", "from basecamp.generated.services.account import AccountService\n")
  write_file(dir, "#{PY_DIR}/_base.py", "class BaseService:\n    def _paginate(self, path, **kw):\n        return None\n")
  write_file(dir, "#{PY_DIR}/_async_base.py", "class AsyncBaseService:\n    async def _paginate(self, path, **kw):\n        return None\n")
  write_file(dir, "#{KT_DIR}/Types.kt", "package com.basecamp.sdk.generated.services\n")
end

# Builds the per-SDK plans. `reassign` moves an operation to another service in
# the named SDKs only; `reassign_async` does it in Python's async classes only;
# `duplicate` ALSO emits it on a second service; `omit` drops it entirely.
def plans(reassign: {}, reassign_async: {}, duplicate: {}, omit: {}, extra: {})
  base = ROSTER.keys.sort.each_with_index.map { |id, i| [id, ROSTER.fetch(id), i] }
  sdk_plan = lambda do |sdk|
    rows = base.reject { |id, _, _| Array(omit[sdk]).include?(id) }
    rows = rows.map do |id, op, i|
      moved = (reassign[sdk] || {})[id]
      [id, moved ? op.merge(service: moved) : op, i]
    end
    (duplicate[sdk] || {}).each { |id, service| rows << [id, ROSTER.fetch(id).merge(service: service), 0] }
    (extra[sdk] || []).each { |row| rows << row }
    rows
  end
  {
    plan: sdk_plan,
    async: lambda do |sdk|
      rows = sdk_plan.call(sdk)
      rows.map do |id, op, i|
        moved = (reassign_async[sdk] || {})[id]
        [id, moved ? op.merge(service: moved) : op, i]
      end
    end,
  }
end

# Every source is written from the SAME plan unless a case names an SDK, so the
# builder writes one tree and the checker's five readers must agree on it. That
# is what makes the synthetic positive control a round-trip proof.
def with_root(reassign: {}, reassign_async: {}, duplicate: {}, omit: {}, extra: {}, openapi: nil)
  built = plans(reassign: reassign, reassign_async: reassign_async, duplicate: duplicate,
                omit: omit, extra: extra)
  Dir.mktmpdir("operation-assignment-parity-test") do |dir|
    spec_paths = {}
    ROSTER.each do |id, op|
      (spec_paths["/{accountId}#{op[:path]}"] ||= {})[op[:method].downcase] = { "operationId" => id }
    end
    doc = openapi || { "openapi" => "3.0.3", "paths" => spec_paths }

    # The five sources are built one at a time so a case can name a single SDK.
    # Each writer only touches its own directory, so writing five trees into one
    # root and keeping the last of each is exactly the same as five roots.
    build_root(dir, plan: built[:plan].call("typescript"), async_plan: built[:async].call("python"),
                    openapi: doc)
    %w[ruby kotlin swift python].each do |sdk|
      rebuild_source(dir, sdk, built[:plan].call(sdk), built[:async].call(sdk))
    end

    yield dir if block_given?
    run_checker(dir)
  end
end

# Rewrites one SDK's directory from its own plan, after build_root laid down all
# five from the TypeScript plan. Removing the directory first is what makes an
# `omit` or a reassignment that empties a service actually disappear.
def rebuild_source(dir, sdk, plan, async_plan)
  target = { "ruby" => RB_DIR, "kotlin" => KT_DIR, "swift" => SW_DIR, "python" => PY_DIR }.fetch(sdk)
  FileUtils.rm_rf(File.join(dir, target))
  Dir.mktmpdir("operation-assignment-parity-one") do |scratch|
    build_root(scratch, plan: plan, async_plan: async_plan, openapi: { "paths" => {} })
    FileUtils.mkdir_p(File.dirname(File.join(dir, target)))
    FileUtils.cp_r(File.join(scratch, target), File.join(dir, target))
  end
end

def run_checker(root)
  out, status = Open3.capture2e({ "OPERATION_ASSIGNMENT_ROOT" => root }, "ruby", CHECKER)
  # Under LC_ALL=C the captured output comes back tagged US-ASCII, and expected
  # fragments can contain UTF-8 punctuation — so `out.include?` would raise
  # Encoding::CompatibilityError before comparing anything. The bytes are UTF-8
  # either way; only the tag is wrong.
  [out.dup.force_encoding("UTF-8"), status]
end

failures = []

def expect_pass(failures, label, out, status)
  if status.success?
    puts "  PASS  #{label}"
  else
    puts "  FAIL  #{label}"
    failures << "#{label}: expected PASS but checker failed:\n#{out}"
  end
end

def expect_fail(failures, label, out, status, *fragments)
  missing = fragments.reject { |f| out.include?(f) }
  if status.success?
    puts "  FAIL  #{label}"
    failures << "#{label}: expected FAILURE but checker passed:\n#{out}"
  elsif !missing.empty?
    puts "  FAIL  #{label}"
    failures << "#{label}: failed as expected but message missing #{missing.map(&:inspect).join(', ')}:\n#{out}"
  else
    puts "  PASS  #{label}"
  end
end

# Fails, AND the message must NOT contain `forbidden`. For diagnostics: a gate
# that fails for the right reason while telling the reader the wrong place to look
# is still a defect, and only an absence assertion can pin that.
def expect_fail_without(failures, label, out, status, fragment, forbidden)
  if status.success?
    puts "  FAIL  #{label}"
    failures << "#{label}: expected FAILURE but checker passed:\n#{out}"
  elsif !out.include?(fragment)
    puts "  FAIL  #{label}"
    failures << "#{label}: failed as expected but message missing #{fragment.inspect}:\n#{out}"
  elsif out.include?(forbidden)
    puts "  FAIL  #{label}"
    failures << "#{label}: failed correctly but MISDIAGNOSED — output contains " \
                "#{forbidden.inspect}, which names a cause this failure does not have:\n#{out}"
  else
    puts "  PASS  #{label}"
  end
end

puts "==> operation assignment parity self-test (checker: #{CHECKER.sub("#{ROOT}/", '')})"

# --- Positive controls ---------------------------------------------------------
#
# Both load-bearing. The first says the real tree agrees; the second says the
# synthetic builder spells all five renderings — and every request helper, and
# all four interpolation styles — the way the real generators do, without which
# no negative case below means anything.

out, status = run_checker(ROOT)
expect_pass(failures, "positive control: the real tree passes", out, status)

out, status = with_root
expect_pass(failures,
            "positive control: the synthetic tree passes (builder round-trips #{ROSTER.length} operations)",
            out, status)

# --- 1. THE #756 SCENARIO -------------------------------------------------------
#
# One generator routes an operation to a DIFFERENT BUT ALREADY-EXISTING service.
# The service-name union is unchanged, so check-service-inventory-parity sees
# nothing; the global operationId set is unchanged, so every per-SDK
# check-*-service-drift sees nothing. This is the case the gate exists for.

out, status = with_root(reassign: { "typescript" => { MOVED => MOVED_TO } })
expect_fail(failures, "1. one SDK assigns an operation to a different existing service", out, status,
            "#{MOVED} (#{ROSTER.fetch(MOVED)[:method]} #{ROSTER.fetch(MOVED)[:path]}) is assigned to different services",
            "`#{canonical(MOVED_TO)}` in typescript",
            "`#{canonical(FROM_SERVICE)}` in kotlin, python, ruby, swift")

# --- 2. Two SDKs on the minority side -------------------------------------------
#
# The reported grouping has to render a split rather than a single odd-one-out,
# and the majority has to come first so the reader sees which way the weight lies.

out, status = with_root(reassign: { "typescript" => { MOVED => MOVED_TO },
                                    "kotlin" => { MOVED => MOVED_TO } })
expect_fail(failures, "2. two SDKs reassign the same operation", out, status,
            "`#{canonical(FROM_SERVICE)}` in python, ruby, swift; `#{canonical(MOVED_TO)}` in kotlin, typescript")

# --- 3. Two classes whose contents were swapped wholesale ------------------------
#
# THE CASE A PARTITION COMPARISON WOULD PASS. Every block is unchanged and the
# service-name set is unchanged — and check-service-inventory-parity compares name
# SETS, not names to contents — so nothing but a name-aware comparison of the
# assignment can see it. Users reach the wrong methods on both services.

swapped = ROSTER.select { |_, op| [FROM_SERVICE, MOVED_TO].include?(op[:service]) }
                .transform_values { |op| op[:service] == FROM_SERVICE ? MOVED_TO : FROM_SERVICE }
out, status = with_root(reassign: { "swift" => swapped })
expect_fail(failures, "3. one SDK swaps two classes' contents wholesale", out, status,
            "is assigned to different services",
            "`#{canonical(MOVED_TO)}` in swift")

# --- 4. An operation missing from one SDK ---------------------------------------
#
# An extraction or emission problem, NOT an assignment disagreement. Pinned as an
# absence: the gate must report it against openapi.json and must not send the
# reader to five generator configs. It is also why completeness is reported and
# exits BEFORE the parity diff — a key one SDK never emits would otherwise show
# up as a nil service in the spread.

out, status = with_root(omit: { "ruby" => [MOVED] })
expect_fail_without(failures, "4. an operation one SDK never emits is reported as such", out, status,
                    "ruby never emits #{MOVED}",
                    "is assigned to different services")

# --- 5. A request-call pattern that stopped matching ----------------------------
#
# Specifically the TypeScript baseUrl-concatenation pattern, which is the one
# recorded oddity in the generated tree — the sole `this.baseUrl` site, and the
# only generated method that does not hand a literal path to openapi-fetch.
# Recorded as an extra PATTERN rather than as an exemption precisely so this case
# can exist: exempting the operation would mean the gate could never see it move.
# Rewriting the emission shape must be LOUD and must name the operation.

out, status = with_root do |dir|
  path = File.join(dir, TS_DIR, "account.ts")
  body = read_utf8(path)
  raise "the synthetic TypeScript account service has no baseUrl concatenation" \
    unless body.include?("${this.baseUrl}")
  File.write(path, body.gsub("`${this.baseUrl}` +", "this.resolveUrl("))
end
expect_fail_without(failures, "5. the TypeScript URL-concatenation pattern going blind is loud",
                    out, status,
                    "typescript never emits #{TS_CONCAT_OPS.first}",
                    "is assigned to different services")

# --- 6. An operation openapi.json does not declare ------------------------------

phantom = ["ListFanfares", { method: "GET", path: "/fanfares.json", service: MOVED_TO }, 0]
out, status = with_root(extra: { "kotlin" => [phantom] })
expect_fail(failures, "6. an SDK emitting an operation the spec does not declare", out, status,
            "kotlin emits GET /fanfares.json on `#{canonical(MOVED_TO)}`, and openapi.json declares no such operation")

# --- 7. One operation on two services inside a single SDK -----------------------
#
# Both split entries list it, so the SDK emits the method twice. Building the
# assignment map by assignment alone would silently keep whichever came last, and
# every other gate is blind: the SDK is internally consistent with its own
# generator either way, and the operationId set is unchanged.

out, status = with_root(duplicate: { "swift" => { MOVED => MOVED_TO } })
expect_fail(failures, "7. one SDK emitting an operation on two services", out, status,
            "swift emits #{ROSTER.fetch(MOVED)[:method]}", "on BOTH")

# --- 8. Python's async twin drifting from its sync class ------------------------
#
# Every Python service is emitted twice. The async class is a second
# transcription of the same assignment by the same generator, and a user who
# reaches for the async client gets the drifted one. Reported as itself rather
# than folded into the cross-SDK diff, which would describe it as Python
# disagreeing with the other four when Python disagrees with itself.

out, status = with_root(reassign_async: { "python" => { MOVED => MOVED_TO } })
expect_fail(failures, "8. Python's async class assigning an operation elsewhere", out, status,
            "python: #{ROSTER.fetch(MOVED)[:method]}",
            "`#{canonical(FROM_SERVICE)}` in the sync class but `#{canonical(MOVED_TO)}` in the Async class")

# --- 9. A rendering that is not there -------------------------------------------

out, status = with_root { |dir| FileUtils.rm_rf(File.join(dir, KT_DIR)) }
expect_fail(failures, "9. missing input is a build problem, not a parity verdict", out, status,
            "#{KT_DIR} not found")

# --- 10. The anchor missing ------------------------------------------------------

out, status = with_root { |dir| FileUtils.rm(File.join(dir, OPENAPI)) }
expect_fail(failures, "10. openapi.json missing is named for what it is", out, status,
            "openapi.json not found")

# --- 11. A path the account prefix no longer covers -----------------------------
#
# The SDKs put the account in the base URL, so the prefix is stripped before
# comparing. A convention change would otherwise surface as all 250 operations
# missing from all five SDKs — 1250 true statements describing the wrong problem.

bare = { "openapi" => "3.0.3", "paths" => { "/fanfares.json" => { "get" => { "operationId" => "ListFanfares" } } } }
out, status = with_root(openapi: bare)
expect_fail(failures, "11. an openapi path that is not account-scoped", out, status,
            "is not account-scoped")

# --- 12. Anchor collapse ---------------------------------------------------------
#
# A regex or a walk that stops matching returns nothing rather than an error,
# which reads as "all clear". Here the walk is over openapi.json, and the floor
# turns a collapsed anchor into one legible failure instead of 1250 lines.

first = ROSTER.first(3).to_h { |id, op| ["/{accountId}#{op[:path]}", { op[:method].downcase => { "operationId" => id } }] }
out, status = with_root(openapi: { "openapi" => "3.0.3", "paths" => first })
expect_fail(failures, "12. extraction floor catches a collapsed anchor", out, status,
            "the anchor collapsed")

# --- 13. A service this gate has never heard of ---------------------------------
#
# ALL FIVE move an operation onto a brand-new service. This gate's question is
# agreement, and only agreement: which services exist is
# check-service-inventory-parity's question, and answering both here is the
# failure mode #755 documents. So a roster change every SDK made together must
# PASS — a rule that exists to prevent a false positive can only be pinned by a
# case that is supposed to pass.

everywhere = %w[typescript ruby python kotlin swift].to_h { |sdk| [sdk, { MOVED => "Fanfares" }] }
out, status = with_root(reassign: everywhere, reassign_async: everywhere)
expect_pass(failures, "13. a new service all five SDKs agree on is not this gate's business",
            out, status)

# --- Report --------------------------------------------------------------------

puts
if failures.empty?
  puts "==> operation assignment parity self-test: all cases passed"
  exit 0
else
  warn "==> operation assignment parity self-test: #{failures.length} case(s) failed"
  warn ""
  failures.each { |f| warn "  #{f}\n\n" }
  exit 1
end
