#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-grouped-client-coverage.
#
# The gate's own `make check` run only ever exercises the PASSING case, so nothing
# there proves it rejects anything. This repo treats an ungated gate as no gate,
# and a coverage gate that cannot fail is worse than none: it converts "nobody
# checked" into "the gate says it is fine".
#
# Every case drives the real checker through its four env seams
# (GROUPED_CLIENT_INVENTORY / _TEMPLATE / _OPENAPI / _GENERATED) against synthetic
# inputs in a tmpdir. The tracked tree is never written to.
#
# GROUPED_COVERAGE_CHECKER names the checker under test (default
# scripts/check-grouped-client-coverage), so a deliberately-mutated copy can be
# driven through the same suite to prove the suite is not vacuous:
#
#   mkdir -p /tmp/m/scripts
#   ln -s "$PWD/openapi.json" /tmp/m/openapi.json
#   ln -s "$PWD/go"           /tmp/m/go
#   cp scripts/check-grouped-client-coverage /tmp/m/scripts/   # mutate the COPY
#   GROUPED_COVERAGE_CHECKER=/tmp/m/scripts/check-grouped-client-coverage \
#     ruby scripts/test-check-grouped-client-coverage.rb
#
# The copy resolves ROOT from its own __dir__, so it needs a ROOT holding
# openapi.json and go/ — symlinks are fine. Skip that and every case fails,
# including the positive control, which looks like a devastating mutation and is
# really just a checker that cannot find its inputs. Never mutate the tracked file.
#
# Every guard in the checker is pinned by at least one case, and the mapping below
# is measured — mutate the guard in a copy, record which cases go red — rather than
# reasoned about. It is mostly one-to-one; where it is not, the table says so, and
# the two exceptions were both found by measuring after writing down the tidy
# version. Removing a guard from the copy must turn exactly this red:
#
#   guard removed in the copy                        case that must go red
#   ---------------------------------------------    ---------------------
#   unaccounted-operation check (THE #679 MISS)      1
#   template-arm-not-in-inventory check              2
#   in-both-buckets check                            3
#   stale-inventory-entry check                      4
#   inventory-vs-template service agreement          5
#   `grouped:`-entry-with-no-template-arm check      6
#   arm-emits-unrecorded-method check                7
#   inventory-records-phantom-method check           8 and 13
#   extraction floor on template arms                9
#   duplicate `grouped:` entry check                 10
#   dead-template-arm check                          11
#   dynamic-prefix (typed body variant) check        12
#   generated-not-recorded check (reverse direction) 14
#   duplicate-template-arm check                     15
#   the `explained` filter on the phantom check      positive control
#
# The last two rows are the measured result, not the tidy one I first wrote. The
# phantom check catches both a genuinely invented method (8) and a typed variant
# whose `{{range .Bodies}}` block was deleted (13) — it is one guard with two
# cases, and neutering it turns both red. The `explained` filter beside it does
# the opposite job: it stops the same check from misreporting the 23 legitimate
# typed variants as phantoms, so deleting it breaks the POSITIVE control rather
# than any negative case. A filter that prevents false positives can only be
# pinned by a case that is supposed to pass.
#
# Case 2 is the #679 replay in its realistic form and is caught by the
# template-arm check rather than the unaccounted check, because by then the arms
# exist. Case 1 is the form where they do not. Both matter: #679 shipped with
# NEITHER, and whichever gets written first, the other guard catches the rest.
#
# WHAT THIS SUITE IS NOT. Five of these cases — 11, 12, 13, 14, 15 — exist because
# a reviewer found a hole this suite had not thought of, on a gate whose whole
# premise is that an ungated gate is no gate. Every one was the same species the
# gate exists to catch: a one-way containment check, an unexamined `next`, a
# method surface measured at 86 and recorded at 63, a duplicate check written for
# one input and not its twin. The suite passing means these fifteen things are
# checked. It has never meant the list is complete, and the list's own history is
# the argument against reading it that way.
#
# Run directly (`ruby scripts/test-check-grouped-client-coverage.rb`) or via
# `make test-grouped-client-coverage`.

require "json"
require "yaml"
require "tmpdir"
require "open3"

# Per-case lines go to stdout and the failure report to stderr. Unsynced, stdout
# block-buffers when redirected to a file and the report lands ahead of the cases
# it summarizes — which is exactly when someone is reading the log.
$stdout.sync = true

ROOT = File.expand_path("..", __dir__)
CHECKER = File.expand_path(
  ENV.fetch("GROUPED_COVERAGE_CHECKER", "scripts/check-grouped-client-coverage"), ROOT
)
REAL_INVENTORY = File.join(ROOT, "go/grouped-client-inventory.yml")
REAL_TEMPLATE  = File.join(ROOT, "go/templates/client.tmpl")
REAL_OPENAPI   = File.join(ROOT, "openapi.json")

def read_utf8(path) = File.read(path, encoding: "UTF-8")

REAL_OPS = JSON.parse(read_utf8(REAL_OPENAPI)).fetch("paths").flat_map { |_p, item|
  item.filter_map { |_v, op| op["operationId"] if op.is_a?(Hash) && op["operationId"] }
}.freeze

# A spec carrying exactly the given operationIds. The checker only ever reads
# operationIds out of openapi.json, so a synthetic spec exercises it identically
# while staying small enough to build per case.
def synthetic_spec(ops)
  { "openapi" => "3.0.3", "info" => { "title" => "t", "version" => "0" },
    "paths" => ops.each_with_object({}) { |op, h| h["/synthetic/#{op}"] = { "get" => { "operationId" => op } } } }
end

# Run the checker with any subset of its inputs overridden. Anything not passed
# falls through to the real tracked file.
def run_checker(inventory: nil, template: nil, openapi: nil, generated: nil)
  env = {}
  env["GROUPED_CLIENT_INVENTORY"] = inventory if inventory
  env["GROUPED_CLIENT_TEMPLATE"]  = template  if template
  env["GROUPED_CLIENT_OPENAPI"]   = openapi   if openapi
  # The generated cross-check is not what any case here is about, and pointing it
  # at a nonexistent path makes the checker skip it — so a synthetic inventory
  # entry is not ALSO reported as missing from client.gen.go, which would let a
  # case pass on the wrong message.
  env["GROUPED_CLIENT_GENERATED"] = generated || "/nonexistent/client.gen.go"
  out, status = Open3.capture2e(env, "ruby", CHECKER)
  # Under LC_ALL=C the captured output comes back tagged US-ASCII, and every
  # expected fragment below contains UTF-8 punctuation — so `out.include?(fragment)`
  # raises Encoding::CompatibilityError before it can compare anything, and every
  # negative case dies for a reason unrelated to what it tests. The bytes are UTF-8
  # either way; only the tag is wrong.
  [out.dup.force_encoding("UTF-8"), status]
end

# Build a case's temp inputs, run, and clean up. `inventory` and `spec_ops` are
# deep copies, so a mutation cannot leak into a later case.
def with_inputs(spec_ops: nil, template: nil)
  Dir.mktmpdir("grouped-client-coverage-test") do |dir|
    inventory = Marshal.load(Marshal.dump(YAML.safe_load(read_utf8(REAL_INVENTORY))))
    yield inventory

    inv_path = File.join(dir, "inventory.yml")
    File.write(inv_path, YAML.dump(inventory))

    spec_path = nil
    if spec_ops
      spec_path = File.join(dir, "openapi.json")
      File.write(spec_path, JSON.generate(synthetic_spec(spec_ops)))
    end

    tmpl_path = nil
    if template
      tmpl_path = File.join(dir, "client.tmpl")
      File.write(tmpl_path, template)
    end

    run_checker(inventory: inv_path, openapi: spec_path, template: tmpl_path)
  end
end

def grouped_entry(inventory, opid)
  inventory.fetch("grouped").each do |service, entries|
    entry = (entries || []).find { |e| e["operation"] == opid }
    return [service, entry] if entry
  end
  raise "no `grouped:` entry for #{opid} in #{REAL_INVENTORY}"
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

puts "==> grouped-client coverage self-test (checker: #{CHECKER.sub("#{ROOT}/", '')})"

# --- Positive control ----------------------------------------------------------
#
# First, and load-bearing: if the real inventory does not pass, every negative
# case below fails for a reason that has nothing to do with the mutation.

out, status = run_checker(generated: File.join(ROOT, "go/pkg/generated/client.gen.go"))
expect_pass(failures, "positive control: the real inventory passes", out, status)

# --- 1. A new operation, in neither bucket -- THE #679 MISS --------------------
#
# The whole reason this gate exists. An operation is added to the spec and to no
# grouped service. Under a plain "inventory of what is exposed", the inventory and
# the template would agree that it is absent and the gate would pass.

out, status = with_inputs(spec_ops: REAL_OPS + ["ResurrectProject"]) { |_inv| }
expect_fail(failures, "1. new operation accounted for nowhere (#679)", out, status,
            "new operation `ResurrectProject` is unaccounted for")

# --- 2. The literal #679 replay -----------------------------------------------
#
# Case 1 with the real names: an operation that IS grouped today, removed from the
# inventory to stand in for the state before anyone recorded it. It must be named.

out, status = with_inputs do |inv|
  service, entry = grouped_entry(inv, "ArchiveProject")
  inv["grouped"][service].delete(entry)
end
expect_fail(failures, "2. ArchiveProject dropped from the inventory", out, status,
            "go/templates/client.tmpl exposes `ArchiveProject` on `ProjectsService` " \
            "but it is not under `grouped:`")

# --- 3. Accounted for twice ----------------------------------------------------

out, status = with_inputs { |inv| inv["not_grouped"] << "ArchiveProject" }
expect_fail(failures, "3. operation in both `grouped:` and `not_grouped:`", out, status,
            "`ArchiveProject` is in BOTH `grouped:`")

# --- 4. Stale inventory entry --------------------------------------------------
#
# An operation removed from the spec — #504 removed GetEverythingBoosts this way —
# must not linger in the accounting, or the totals stop meaning anything.

out, status = with_inputs { |inv| inv["not_grouped"] << "GetEverythingBoosts" }
expect_fail(failures, "4. inventory entry absent from openapi.json", out, status,
            "`GetEverythingBoosts` is in the inventory but not in openapi.json — stale")

# --- 5. Recorded on the wrong service ------------------------------------------
#
# The inventory says one service, client.tmpl emits another. Silent otherwise:
# both files parse, both list the operation, and they disagree about where it is.

out, status = with_inputs do |inv|
  service, entry = grouped_entry(inv, "ArchiveProject")
  inv["grouped"][service].delete(entry)
  inv["grouped"]["TodosService"] << entry
end
expect_fail(failures, "5. operation recorded on the wrong service", out, status,
            "`ArchiveProject` is recorded under `grouped: TodosService:` but client.tmpl " \
            "emits it on `ProjectsService`")

# --- 6. Inventory claims a grouping the template does not have -----------------
#
# The converse of case 2, and the one that would let someone "fix" a failure by
# editing only the inventory. Moving an operation out of `not_grouped:` asserts it
# is on the grouped surface; if no arm emits it, the assertion is false.

out, status = with_inputs do |inv|
  inv["not_grouped"].delete("GetProject") if inv["not_grouped"].include?("GetProject")
  target = inv["not_grouped"].first
  inv["not_grouped"].delete(target)
  inv["grouped"]["ProjectsService"] << { "operation" => target, "methods" => ["Get"] }
end
expect_fail(failures, "6. `grouped:` entry with no template arm", out, status,
            "go/templates/client.tmpl has no `$opid` arm for it")

# --- 7. Arm emits a method the inventory does not record -----------------------

out, status = with_inputs do |inv|
  _service, entry = grouped_entry(inv, "ArchiveProject")
  entry["methods"] = []
end
expect_fail(failures, "7. arm emits an unrecorded method", out, status,
            "is not in its inventory `methods:` — method names are recorded, not derived")

# --- 8. Inventory records a method no arm emits --------------------------------
#
# The renames (MoveCard -> Move, UpdateCard -> UpdateVerbatim) mean method names
# are data rather than something the gate can recompute, so a wrong name is
# otherwise unfalsifiable.

out, status = with_inputs do |inv|
  _service, entry = grouped_entry(inv, "ArchiveProject")
  entry["methods"] = entry["methods"] + ["Unarchive"]
end
expect_fail(failures, "8. inventory records a phantom method", out, status,
            "records method `Unarchive` but client.tmpl's arm does not emit it")

# --- 9. Template extraction collapses ------------------------------------------
#
# If the `$opid` chain's syntax changes, the regex matches nothing and returns an
# empty arm set. Without a floor that reads as "no drift" and the gate goes quiet
# — the failure mode a coverage gate can least afford.

out, status = with_inputs(template: "// a client template with no $opid chain at all\n") { |_inv| }
expect_fail(failures, "9. template extraction returns nothing", out, status,
            "the chain's syntax probably changed and extraction is silently failing")

# --- 11. A template arm for an operation the spec no longer has ----------------
#
# The gap between the other two checks. Remove an operation from openapi.json and
# from the inventory but leave its `$opid` arm: the stale-inventory check walks
# `accounted`, which no longer contains it, and the arm-not-in-inventory check
# used to `next` past anything absent from the spec on the (wrong) assumption that
# the stale check had it covered. Nothing looked at it, so a dead arm could sit in
# client.tmpl indefinitely with the gate green.

out, status = with_inputs(spec_ops: REAL_OPS - ["ArchiveProject"]) do |inv|
  service, entry = grouped_entry(inv, "ArchiveProject")
  inv["grouped"][service].delete(entry)
end
expect_fail(failures, "11. dead template arm for an operation dropped from the spec", out, status,
            "still has a `$opid` arm for `ArchiveProject`")

# --- 12. Typed `{{range .Bodies}}` variant dropped from the inventory ----------
#
# A body-bearing arm emits both `CreateWithBody` (literal) and `Create{{.Suffix}}`
# (dynamic). Recording only the WithBody anchor covered 63 of the 86 grouped
# methods and left the 23 typed variants — the ones people actually call — outside
# the accounting entirely.

out, status = with_inputs do |inv|
  _service, entry = grouped_entry(inv, "CreateTodo")
  entry["methods"] = entry["methods"].reject { |m| m == "Create" }
end
expect_fail(failures, "12. typed body variant missing from the inventory", out, status,
            "records no `Create…` method beyond the WithBody form")

# --- 13. The `{{range .Bodies}}` block deleted from the template ---------------
#
# The other direction, and the regression this pair exists for: regeneration drops
# the public `Todos().Create` while `CreateWithBody` survives. Before the typed
# variants were recorded, template, inventory and generated output all still
# agreed and the gate passed having lost a public method.

mutated = read_utf8(REAL_TEMPLATE).sub(
  /\{\{range \.Bodies\}\}\{\{if \.IsSupportedByClient\}\}\nfunc \(s \*TodosService\) Create\{\{\.Suffix\}\}.*?\n\{\{end\}\}\{\{end\}\}\n/m, ""
)
raise "template mutation for case 13 did not apply" if mutated == read_utf8(REAL_TEMPLATE)
out, status = with_inputs(template: mutated) { |_inv| }
expect_fail(failures, "13. typed body variant deleted from the template", out, status,
            "`CreateTodo` records method `Create` but client.tmpl's arm does not emit it")

# --- 14. A generated method nobody recorded ------------------------------------
#
# The reverse direction, and the one that makes the method accounting total. An
# operation gaining a second supported request content type makes
# `{{range .Bodies}}` emit an extra suffixed method; the dynamic-prefix check is
# already satisfied by the variant recorded earlier, so without this the new
# method sits unrecorded — and an unrecorded method is one that can later vanish
# with nothing noticing, which is #679 at method granularity.
#
# This is the only case that overrides GROUPED_CLIENT_GENERATED, so it builds a
# temp copy of the real client.gen.go with one extra method spliced in.

out, status = Dir.mktmpdir("grouped-client-coverage-generated") do |dir|
  real = File.join(ROOT, "go/pkg/generated/client.gen.go")
  src = read_utf8(real)
  anchor = "func (s *TodosService) Create(ctx context.Context"
  idx = src.index(anchor) or raise "anchor for case 14 not found in #{real}"
  extra = "func (s *TodosService) CreateFormdata(ctx context.Context) (*http.Response, error) { return nil, nil }\n\n"
  gen = File.join(dir, "client.gen.go")
  File.write(gen, src[0...idx] + extra + src[idx..])
  run_checker(generated: gen)
end
expect_fail(failures, "14. generated method absent from the inventory", out, status,
            "the generated client exposes `TodosService.CreateFormdata` but no `grouped:` entry records it")

# --- 15. The same operation armed twice in the template ------------------------
#
# The symmetric twin of case 10, which covers the inventory. Go evaluates the
# FIRST matching branch of the chain, so a copy-pasted arm is unreachable dead
# template — and parsing it into a hash keeps the LAST one, so the gate would
# otherwise compare the inventory against a branch that never runs, while an
# identical duplicate stayed invisible.

tmpl = read_utf8(REAL_TEMPLATE)
dup_marker = "{{else if eq $opid \"ArchiveProject\"}}"
raise "arm marker for case 15 not found" unless tmpl.include?(dup_marker)
dup_arm = "#{dup_marker}\nfunc (s *ProjectsService) Archive(ctx context.Context) (*http.Response, error) {\n\treturn nil, nil\n}\n"
out, status = with_inputs(template: tmpl.sub(dup_marker, dup_arm + dup_marker)) { |_inv| }
expect_fail(failures, "15. duplicate `$opid` arm in the template", out, status,
            "more than one `$opid` arm for `ArchiveProject`")

# --- 10. The same operation grouped twice --------------------------------------

out, status = with_inputs do |inv|
  _service, entry = grouped_entry(inv, "ArchiveProject")
  # A DEEP copy, not the same object: YAML.dump emits an alias for any repeated
  # reference, safe_load rejects aliases, and the case would then prove only that
  # the checker rejects aliases. `entry.dup` is not enough — the shared `methods`
  # array aliases on its own.
  inv["grouped"]["TodosService"] << Marshal.load(Marshal.dump(entry))
end
expect_fail(failures, "10. duplicate `grouped:` entry", out, status,
            "`ArchiveProject` is listed twice under `grouped:`")

# --- Report --------------------------------------------------------------------

puts
if failures.empty?
  puts "==> grouped-client coverage self-test: all cases passed"
  exit 0
else
  warn "==> grouped-client coverage self-test: #{failures.length} case(s) failed"
  warn ""
  failures.each { |f| warn "  #{f}\n\n" }
  exit 1
end
