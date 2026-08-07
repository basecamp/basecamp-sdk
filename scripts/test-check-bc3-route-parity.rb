#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-bc3-route-parity.
#
# The gate's own `make check` run only exercises the VALID allowlist, so nothing
# proves its strongest disposition rejects anything. `modeled_as` asserts "we
# already model that bc3 route at its other spelling" — the same class of claim
# that shipped ListForwards and RepositionTodolistGroup as 404s. This test crafts
# each way that claim can be wrong and asserts the gate rejects it (non-zero exit
# + an expected message fragment), driving the checker through its
# BC3_ROUTE_ALLOWLIST env override against tmp allowlists. It also confirms the
# real allowlist still passes (positive control).
#
# openapi.json and spec/bc3-routes.json are NOT overridden: every case is a real
# operationId substituted for another REAL route, so a case that passes proves
# something about the routes this SDK ships rather than about invented ones.
#
# BC3_PARITY_CHECKER names the checker under test (default
# scripts/check-bc3-route-parity), so a deliberately-mutated copy can be driven
# through the same suite to prove the suite is not vacuous:
#
#   cp scripts/check-bc3-route-parity /tmp/m/scripts/   # mutate the COPY
#   BC3_PARITY_CHECKER=/tmp/m/scripts/check-bc3-route-parity ruby scripts/test-...
#
# The copy needs scripts/bc3_route_normalizer.rb and scripts/generate-bc3-routes
# beside it (the generator fingerprint hashes both, off the normalizer's own
# __dir__) and a ROOT holding spec/ and openapi.json — symlinks are fine. Never
# mutate the tracked file.
#
# Every case below is pinned by at least one guard, and every guard by at least
# one case. Removing a guard from the copy must turn exactly these red:
#
#   guard removed in the copy                     cases that must go red
#   ------------------------------------------    ----------------------
#   suffix rule -> subset (the ORIGINAL bug)       1
#   suffix rule deleted outright                   1, 6
#   terminal-segment equality                      5, 7, 8
#   operationId-exists check                       2
#   verb must match (find -> subs.first)           3
#   substitute-sits-at-this-path check             4
#   routes_rb evidence requirement                 9
#
# Cases 5, 7 and 8 are pinned by the MESSAGE fragment, not the exit code: with the
# terminal check gone the suffix rule still rejects them, just with different
# wording. Asserting on the message is what keeps them load-bearing.
#
# Run directly (`ruby scripts/test-check-bc3-route-parity.rb`) or via
# `make test-bc3-route-parity`.

require "yaml"
require "tmpdir"
require "open3"

# Per-case lines go to stdout and the failure report to stderr. Unsynced, stdout
# block-buffers when redirected to a file and the report lands ahead of the cases
# it summarizes — which is exactly when someone is reading the log.
$stdout.sync = true

ROOT = File.expand_path("..", __dir__)
CHECKER = File.expand_path(ENV.fetch("BC3_PARITY_CHECKER", "scripts/check-bc3-route-parity"), ROOT)
REAL_ALLOWLIST = File.join(ROOT, "spec/bc3-route-allowlist.yml")

def read_utf8(path) = File.read(path, encoding: "UTF-8")

# Run the checker against a given allowlist; returns [combined_output, status].
#
# The captured output is forced to UTF-8. Under a non-UTF-8 locale (LC_ALL=C)
# Open3 tags it US-ASCII, and every expected fragment below contains UTF-8
# punctuation — so `out.include?(fragment)` raises Encoding::CompatibilityError
# before it can compare anything, and every case dies for a reason unrelated to
# what it tests while looking like the gate is broken. The bytes are UTF-8 either
# way; only the tag is wrong.
def run_checker(allowlist:)
  out, status = Open3.capture2e({ "BC3_ROUTE_ALLOWLIST" => allowlist }, "ruby", CHECKER)
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

def expect_fail(failures, label, out, status, fragment)
  if status.success?
    puts "  FAIL  #{label}"
    failures << "#{label}: expected FAILURE but checker passed:\n#{out}"
  elsif !out.include?(fragment)
    puts "  FAIL  #{label}"
    failures << "#{label}: failed as expected but message missing #{fragment.inspect}:\n#{out}"
  else
    puts "  PASS  #{label}"
  end
end

# A tmp allowlist: the REAL one, deep-copied, with exactly ONE modeled_as entry
# mutated in the block. The tracked file is never written to — Marshal breaks
# every reference back to the parsed original, so a mutation cannot leak into a
# later case either.
def with_mutated_entry(method, path)
  allow = Marshal.load(Marshal.dump(YAML.safe_load(read_utf8(REAL_ALLOWLIST))))
  entry = allow.fetch("bc3_routes_not_modeled").find do |e|
    e["method"] == method && e["path"] == path && e.key?("modeled_as")
  end
  raise "no modeled_as entry for #{method} #{path} in #{REAL_ALLOWLIST}" if entry.nil?

  yield entry
  Dir.mktmpdir("bc3-route-parity-test") do |dir|
    tmp = File.join(dir, "bc3-route-allowlist.yml")
    File.write(tmp, YAML.dump(allow))
    run_checker(allowlist: tmp)
  end
end

puts "==> bc3 route-parity self-test (checker: #{CHECKER.sub("#{ROOT}/", '')})"

# --- Positive control ----------------------------------------------------------
#
# First, and load-bearing: if the real allowlist does not pass, every negative
# case below fails for a reason that has nothing to do with the mutation.

out, status = run_checker(allowlist: REAL_ALLOWLIST)
expect_pass(failures, "positive control: the real allowlist passes", out, status)

# --- 1. Interior segment dropped -- THE ONE THAT MATTERS -----------------------
#
# /projects/:id/timesheet is not /projects/:id/recordings/:id/timesheet with a
# leading scope removed: it drops `recordings/:id` from the MIDDLE, which selects
# a different timesheet. The first draft of the gate matched flattened segments
# as a SUBSET and accepted exactly this. Only the suffix rule rejects it.

out, status = with_mutated_entry("GET", "/projects/:id/recordings/:id/timesheet") do |e|
  e["modeled_as"] = "GetProjectTimesheet"
end
expect_fail(failures, "1. interior segment dropped: GetProjectTimesheet for the RECORDING timesheet", out, status,
            "`modeled_as: GetProjectTimesheet` is /projects/:id/timesheet, which is not " \
            "/projects/:id/recordings/:id/timesheet with a leading scope removed")

# --- 2. Operation does not exist -----------------------------------------------

out, status = with_mutated_entry("GET", "/projects/:id/gauge/needles/:id") do |e|
  e["modeled_as"] = "NoSuchOperation"
end
expect_fail(failures, "2. nonexistent operationId", out, status,
            "`modeled_as: NoSuchOperation` names no operation in openapi.json")

# --- 3. Right resource, wrong verb ---------------------------------------------
#
# UpdateGaugeNeedle is a real, bc3-documented operation at the very spelling this
# route aliases — but it is PUT, and a PUT does not answer a GET.

out, status = with_mutated_entry("GET", "/projects/:id/gauge/needles/:id") do |e|
  e["modeled_as"] = "UpdateGaugeNeedle"
end
expect_fail(failures, "3. verb mismatch: PUT operation for a GET route", out, status,
            "`modeled_as: UpdateGaugeNeedle` is PUT, not GET — a different verb does not cover this route.")

# --- 4. Alias of itself ---------------------------------------------------------
#
# Point the entry at the path its own substitute occupies. Nothing is being
# aliased, so the entry is stale rather than discharged and must be deleted.

out, status = with_mutated_entry("GET", "/projects/:id/recordings/:id/timesheet") do |e|
  e["path"] = "/recordings/:id/timesheet"
end
expect_fail(failures, "4. same-path alias: the substitute sits at the entry's own path", out, status,
            "`modeled_as: GetRecordingTimesheet` sits at this very path")

# --- 5. Collection standing in for a member ------------------------------------

out, status = with_mutated_entry("GET", "/projects/:id/gauge/needles/:id") do |e|
  e["modeled_as"] = "ListGaugeNeedles"
end
expect_fail(failures, "5. collection for member: ListGaugeNeedles for the needle SHOW route", out, status,
            'which ends in "needles" where this route ends in ":id" — the terminal segment has to match')

# --- 6. Different resource entirely ---------------------------------------------
#
# GetTimesheetEntry is a real, bc3-documented GET ending in :id, so checks 1-3 and
# the collection/member check all wave it through. Only the suffix rule notices it
# addresses a timesheet entry, not a gauge needle.

out, status = with_mutated_entry("GET", "/projects/:id/gauge/needles/:id") do |e|
  e["modeled_as"] = "GetTimesheetEntry"
end
expect_fail(failures, "6. different resource: GetTimesheetEntry for the needle SHOW route", out, status,
            "`modeled_as: GetTimesheetEntry` is /timesheet_entries/:id, which is not " \
            "/projects/:id/gauge/needles/:id with a leading scope removed")

# --- 7. Different noun, same family ---------------------------------------------
#
# ListGauges is a real, bc3-documented GET at /reports/gauges. It is a gauge
# something and it is a collection, so it reads plausibly next to a gauge needle.
# The terminal segment is the only thing that refuses it.

out, status = with_mutated_entry("GET", "/projects/:id/gauge/needles/:id") do |e|
  e["modeled_as"] = "ListGauges"
end
expect_fail(failures, "7. different noun: ListGauges for the needle SHOW route", out, status,
            '`modeled_as: ListGauges` is /reports/gauges, which ends in "gauges" where this route ' \
            'ends in ":id"')

# --- 8. Two collections, different terminal --------------------------------------
#
# Proof that the terminal rule is not just a collection/member test. BOTH of these
# are collection POSTs and both are real bc3-documented creates; only the noun
# differs. The message has to say that without claiming a member is involved.

out, status = with_mutated_entry("POST", "/projects/:id/recordings/:id/timesheet/entries") do |e|
  e["modeled_as"] = "CreateGaugeNeedle"
end
expect_fail(failures, "8. two collections, different terminal: CreateGaugeNeedle for entry create",
            out, status,
            'which ends in "needles" where this route ends in "entries" — the terminal segment has ' \
            'to match')

# --- 9. Missing routes.rb evidence ----------------------------------------------
#
# The one thing the gate cannot check — that both draws name the same controller
# action — is demanded as a citation instead. A requirement with no test is a
# requirement nobody applies.

out, status = with_mutated_entry("GET", "/projects/:id/gauge/needles/:id") { |e| e.delete("routes_rb") }
expect_fail(failures, "9. modeled_as with no routes_rb citation", out, status,
            "`modeled_as` needs `routes_rb:` naming BOTH draws")

# --- Report --------------------------------------------------------------------

if failures.empty?
  puts "==> bc3 route-parity self-test passed — 1 positive + 9 negative cases"
  exit 0
else
  warn "bc3 route-parity self-test FAILED:"
  failures.each { |f| warn "  - #{f}" }
  exit 1
end
