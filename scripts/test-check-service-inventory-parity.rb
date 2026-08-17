#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-service-inventory-parity.
#
# The gate's own `make check` run only ever exercises the PASSING case, so
# nothing there proves it rejects anything. This repo treats an ungated gate as
# no gate, and a parity gate that cannot fail is worse than none: it converts
# "nobody compared them" into "the gate says they agree".
#
# Every case drives the real checker through its one env seam
# (SERVICE_INVENTORY_ROOT) against a synthetic repository tree in a tmpdir. The
# tracked tree is never written to.
#
# THE SYNTHETIC TREE IS ITSELF UNDER TEST. It is built by INVERTING each source's
# normalization — snake_case back to kebab, camelCase, PascalCase, `_service`
# suffixes, Go's three carve-outs — from one canonical list read out of the real
# Kotlin accessors. A builder that spelled anything differently from the real
# generators would make every negative case below fail for a reason unrelated to
# its mutation, so the synthetic positive control runs second and is as
# load-bearing as the real one. It is also the round-trip proof for the casing
# helpers: 53 names survive canonical -> per-SDK spelling -> canonical.
#
# SERVICE_INVENTORY_CHECKER names the checker under test (default
# scripts/check-service-inventory-parity), so a deliberately-mutated copy can be
# driven through the same suite to prove the suite is not vacuous:
#
#   cp scripts/check-service-inventory-parity /tmp/m       # mutate the COPY
#   SERVICE_INVENTORY_CHECKER=/tmp/m ruby scripts/test-check-service-inventory-parity.rb
#
# The copy resolves its sources from SERVICE_INVENTORY_ROOT, which every case
# sets, so it needs nothing beside it. Never mutate the tracked file.
#
# Every guard in the checker is pinned by at least one case, and the mapping
# below is MEASURED — mutate the guard in a copy, record which cases go red —
# rather than reasoned about. Removing a guard from the copy must turn exactly
# this red:
#
#   guard removed in the copy                     cases that must go red
#   -------------------------------------------   ----------------------
#   the parity comparison                         1, 2, 3, 9, 12
#   per-source duplicate check                    4
#   extraction floor                              5
#   source-exists check                           6
#   fold carve-out staleness check                7
#   spelling carve-out staleness check            8
#   spelling carve-out both-spellings check       10
#   reading Python from its BARREL                11, 12
#   APPLYING the fold carve-out                   both positive controls, 11
#   APPLYING the spelling carve-out               both positive controls, 11
#
# The last three rows are the measured result, not a tidy one.
#
# Applying a carve-out is what makes Go's 51 accessors line up with the canonical
# 53; delete either and the real tree stops passing. A rule that exists to prevent
# false positives can only be pinned by a case that is supposed to pass — the same
# shape as the `explained` filter in test-check-grouped-client-coverage.rb.
#
# Case 11 is the other pass-shaped case, and it shows up in those two rows for the
# same structural reason: a case asserting PASS goes red for ANY breakage that
# stops the checker passing at all, so it is not a specific pin for the carve-outs.
# It is a specific pin for the barrel reading, because reverting Python to a
# directory listing turns 11 and 12 red and nothing else.
#
# Cases 1, 2, 3, 9 and 12 share the parity comparison and are not redundant: they
# are five different shapes of disagreement (a service in three SDKs only, a
# service missing from one, a generated file with no accessor beside it, an SDK
# short a service its carve-outs do not cover, and a barrel short one while the
# directory beside it is complete), and the messages a reader gets differ. Case 3
# is the one that anchors SPEC §5 — its roster is derived from the two accessor
# files, and nothing else checks those against their own services directories.
#
# WHAT THIS SUITE IS NOT. The suite passing means these twelve things are checked.
# It has never meant the list is complete.
#
# Run directly (`ruby scripts/test-check-service-inventory-parity.rb`) or via
# `make test-check-service-inventory-parity`.

require "fileutils"
require "open3"
require "tmpdir"

# Per-case lines go to stdout and the failure report to stderr. Unsynced, stdout
# block-buffers when redirected to a file and the report lands ahead of the cases
# it summarizes — which is exactly when someone is reading the log.
$stdout.sync = true

ROOT = File.expand_path("..", __dir__)
CHECKER = File.expand_path(
  ENV.fetch("SERVICE_INVENTORY_CHECKER", "scripts/check-service-inventory-parity"), ROOT
)

TS_DIR     = "typescript/src/generated/services"
RB_DIR     = "ruby/lib/basecamp/generated/services"
PY_DIR     = "python/src/basecamp/generated/services"
KT_DIR     = "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/services"
SW_DIR     = "swift/Sources/Basecamp/Generated/Services"
PY_BARREL  = "python/src/basecamp/generated/services/__init__.py"
KT_ACCESS  = "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated/ServiceAccessors.kt"
SW_ACCESS  = "swift/Sources/Basecamp/Generated/AccountClient+Services.swift"
GO_CLIENT  = "go/pkg/basecamp/client.go"

def read_utf8(path) = File.read(path, encoding: "UTF-8")

def snake_case(name)
  name.gsub(/([a-z0-9])([A-Z])/, '\1_\2').gsub(/([A-Z]+)([A-Z][a-z])/, '\1_\2').downcase
end

def kebab(name) = name.tr("_", "-")
# The generator's one filename rename (generate_services.py's `service_filename`).
def py_module(name) = name == "webhooks" ? "webhooks_service" : name
def py_module_filename(name) = "#{py_module(name)}.py"
def pascal(name) = name.split("_").map(&:capitalize).join
def camel(name) = name.split("_").each_with_index.map { |p, i| i.zero? ? p : p.capitalize }.join

# The canonical roster, read from the real Kotlin accessors rather than typed out
# here — a literal would be a ninth hand-copy of the very table this gate exists
# because there are already five of.
CANONICAL = read_utf8(File.join(ROOT, KT_ACCESS))
  .scan(/^val AccountClient\.([A-Za-z][A-Za-z0-9_]*)\s*:/).flatten.map { |n| snake_case(n) }.sort.freeze

raise "canonical roster came back with #{CANONICAL.length} services; the real accessors changed shape" \
  if CANONICAL.length < 40

# --- Synthetic tree builder ----------------------------------------------------
#
# One writer per source, each inverting that source's normalization. `names` is
# per-source so a case can give one SDK a different roster from the rest.

def write_dir(root, rel, filenames)
  dir = File.join(root, rel)
  FileUtils.mkdir_p(dir)
  filenames.each { |f| File.write(File.join(dir, f), "// synthetic\n") }
end

def write_file(root, rel, body)
  path = File.join(root, rel)
  FileUtils.mkdir_p(File.dirname(path))
  File.write(path, body)
end

# Go's carve-outs, inverted: the two folded services have no accessor at all, and
# `timesheets` is spelled singular.
GO_FOLDED = %w[automation client_visibility].freeze

def go_accessor_names(names)
  names.reject { |n| GO_FOLDED.include?(n) }.map { |n| n == "timesheets" ? "timesheet" : n }
end

def build_root(dir, names)
  ts = names.fetch("typescript", CANONICAL)
  rb = names.fetch("ruby", CANONICAL)
  py = names.fetch("python", CANONICAL)
  kt = names.fetch("kotlin", CANONICAL)
  sw = names.fetch("swift", CANONICAL)
  kta = names.fetch("kotlin-accessors", CANONICAL)
  swa = names.fetch("swift-accessors", CANONICAL)
  go = names.fetch("go-accessors", go_accessor_names(CANONICAL))

  write_dir(dir, TS_DIR, ts.map { |n| "#{kebab(n)}.ts" } + ["index.ts"])
  write_dir(dir, RB_DIR, rb.map { |n| "#{n}_service.rb" } + ["base_service.rb"])
  # Python is read from its BARREL, so the directory is written from
  # `python_modules` (which a case can leave stale) while the barrel is written
  # from `py`. They are the same list unless a case separates them.
  py_modules = names.fetch("python-modules", py)
  write_dir(dir, PY_DIR, py_modules.map { |n| py_module_filename(n) } +
                         ["__init__.py", "_base.py", "_async_base.py"])
  write_file(dir, PY_BARREL, <<~PYTHON)
    # @generated from OpenAPI spec — do not edit manually
    #{py.map { |n| "from basecamp.generated.services.#{py_module(n)} import #{pascal(n)}Service, Async#{pascal(n)}Service" }.join("\n")}
  PYTHON
  write_dir(dir, KT_DIR, kt.map { |n| "#{kebab(n)}.kt" } + ["Types.kt"])
  write_dir(dir, SW_DIR, sw.map { |n| "#{pascal(n)}Service.swift" })

  write_file(dir, KT_ACCESS, <<~KOTLIN)
    package com.basecamp.sdk.generated
    #{kta.map { |n| "val AccountClient.#{camel(n)}: #{pascal(n)}Service\n    get() = service(\"#{pascal(n)}\") { #{pascal(n)}Service(this) }" }.join("\n")}
  KOTLIN

  write_file(dir, SW_ACCESS, <<~SWIFT)
    extension AccountClient {
    #{swa.map { |n| "    public var #{camel(n)}: #{pascal(n)}Service { service(\"#{camel(n)}\") { #{pascal(n)}Service(accountClient: self) } }" }.join("\n")}
    }
  SWIFT

  write_file(dir, GO_CLIENT, <<~GO)
    package basecamp
    #{go.map { |n| "func (ac *AccountClient) #{pascal(n)}() *#{pascal(n)}Service {\n\treturn nil\n}" }.join("\n")}
  GO
end

def run_checker(root)
  out, status = Open3.capture2e({ "SERVICE_INVENTORY_ROOT" => root }, "ruby", CHECKER)
  # Under LC_ALL=C the captured output comes back tagged US-ASCII, and every
  # expected fragment below contains UTF-8 punctuation — so `out.include?` raises
  # Encoding::CompatibilityError before it can compare anything, and every case
  # dies for a reason unrelated to what it tests. The bytes are UTF-8 either way;
  # only the tag is wrong.
  [out.dup.force_encoding("UTF-8"), status]
end

def with_root(names: {})
  Dir.mktmpdir("service-inventory-parity-test") do |dir|
    build_root(dir, names)
    yield dir if block_given?
    run_checker(dir)
  end
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

puts "==> service inventory parity self-test (checker: #{CHECKER.sub("#{ROOT}/", '')})"

# --- Positive controls ---------------------------------------------------------
#
# Both load-bearing. The first says the real tree agrees; the second says the
# synthetic builder spells all eight renderings the way the real generators do,
# without which no negative case below means anything.

out, status = run_checker(ROOT)
expect_pass(failures, "positive control: the real tree passes", out, status)

out, status = with_root
expect_pass(failures, "positive control: the synthetic tree passes (builder round-trips #{CANONICAL.length} names)",
            out, status)

# --- 1. THE #745 RESIDUE -------------------------------------------------------
#
# A service added to the TypeScript, Ruby and Python tables and omitted from both
# Kotlin and Swift. `make doc-constants-check` derives SPEC §5's roster from the
# Kotlin and Swift accessors alone, so those two agree with each other, certify
# the old roster and the old count, and see nothing. This is the case that gate
# structurally cannot catch and the reason this one exists.

extra = CANONICAL + ["fanfares"]
out, status = with_root(names: { "typescript" => extra, "ruby" => extra, "python" => extra })
expect_fail(failures, "1. service in the TS/Ruby/Python tables only (#745 residue)", out, status,
            "`fanfares` is emitted by typescript, ruby, python but NOT by " \
            "kotlin, swift, kotlin-accessors, swift-accessors, go-accessors")

# --- 2. One SDK short ----------------------------------------------------------

out, status = with_root(names: { "ruby" => CANONICAL - ["gauges"] })
expect_fail(failures, "2. one SDK missing a service the other seven emit", out, status,
            "but NOT by ruby")

# --- 3. A generated service with no accessor beside it -------------------------
#
# Kotlin emits the service file but no accessor. SPEC §5's roster reads the
# ACCESSORS, so the roster comes up short while every services directory agrees
# the service exists — and nothing but this checked the two against each other.

out, status = with_root(names: { "kotlin-accessors" => CANONICAL - ["wormholes"] })
expect_fail(failures, "3. Kotlin service file with no accessor (anchors SPEC section 5)", out, status,
            "but NOT by kotlin-accessors")

# --- 4. A name emitted twice ---------------------------------------------------
#
# Invisible to every set comparison downstream: Array#- drops all occurrences, so
# both diffs come back empty and the gate would pass while enumerating something
# the file does not say.

out, status = with_root do |dir|
  path = File.join(dir, SW_ACCESS)
  body = read_utf8(path)
  line = body.lines.find { |l| l.include?(" public var gauges:") }
  raise "no gauges accessor in the synthetic Swift file" if line.nil?
  File.write(path, body.sub(line, line + line))
end
expect_fail(failures, "4. duplicate accessor in one source", out, status,
            "yields gauges more than once")

# --- 5. Extraction collapse ----------------------------------------------------
#
# A glob or regex that stops matching returns nothing rather than an error, which
# reads as "all clear". The floor turns that into a failure instead.

out, status = with_root(names: { "typescript" => CANONICAL.first(3) })
expect_fail(failures, "5. extraction floor catches a collapsed source", out, status,
            "extraction is probably broken, not the SDK")

# --- 6. A rendering that is not there ------------------------------------------

out, status = with_root { |dir| FileUtils.rm(File.join(dir, GO_CLIENT)) }
expect_fail(failures, "6. missing input is a build problem, not a parity verdict", out, status,
            "go/pkg/basecamp/client.go not found")

# --- 7. A fold carve-out that closed -------------------------------------------
#
# Go grows an Automation() accessor. Without the staleness check this is INVISIBLE
# — the carve-out adds `automation` to Go's set either way, so the parity diff is
# empty and SPEC Appendix F keeps claiming a fold that no longer exists.

out, status = with_root(names: { "go-accessors" => go_accessor_names(CANONICAL) + ["automation"] })
expect_fail(failures, "7. Go closed the automation fold, carve-out now stale", out, status,
            "now exposes `automation`, but it is recorded as a fold carve-out")

# --- 8. A spelling carve-out that closed ---------------------------------------

out, status = with_root(names: { "go-accessors" => go_accessor_names(CANONICAL) - ["timesheet"] + ["timesheets"] })
expect_fail(failures, "8. Go renamed Timesheet to Timesheets, spelling carve-out now stale", out, status,
            "spelling carve-out (Go spells the accessor Timesheet(), singular) no longer matches")

# --- 9. Go short a service its carve-outs do not cover --------------------------
#
# The carve-outs are three named divergences, not a blanket exemption. A fourth
# service missing from Go has to fail like any other SDK's would.

out, status = with_root(names: { "go-accessors" => go_accessor_names(CANONICAL) - ["gauges"] })
expect_fail(failures, "9. Go missing an uncarved service still fails", out, status,
            "but NOT by go-accessors")

# --- 10. Both spellings at once ------------------------------------------------
#
# The rename would map one onto the other, and the duplicate it creates is
# reported by the check that runs before any set comparison — but only because
# the carve-out refuses instead of renaming blind.

out, status = with_root(names: { "go-accessors" => go_accessor_names(CANONICAL) + ["timesheets"] })
expect_fail(failures, "10. Go exposing both Timesheet and Timesheets", out, status,
            "Go exposes BOTH `timesheet` and `timesheets`")

# --- 11. A stale Python module on disk, absent from the barrel -----------------
#
# The one source read from a barrel rather than a directory, and the reason why.
# `python/scripts/generate_services.py` does not delete outputs the current
# mapping stopped producing — unlike generate-services.ts:1736,
# generate-services.rb:344 and Main.kt:65, which all do. So a mapping that DROPS
# a Python service leaves `fanfares.py` on disk, and a directory listing counts
# the corpse as still emitted.
#
# This case EXPECTS A PASS, which is the only shape that can pin the fix: the
# stale module must not make Python look like it emits a service the other seven
# do not. Run against the pre-fix checker (which enumerated the directory) the
# same input fails with "`fanfares` is emitted by python but NOT by ...".

out, status = with_root(names: { "python-modules" => CANONICAL + ["fanfares"] })
expect_pass(failures, "11. stale Python module on disk is not counted as emitted", out, status)

# --- 12. The Python barrel itself short ----------------------------------------
#
# Case 11 must not have made Python's rendering vacuous: a service genuinely
# absent from the barrel still has to fail. Note the directory here keeps ALL the
# modules, so only the barrel reading can catch it.

out, status = with_root(names: { "python" => CANONICAL - ["gauges"], "python-modules" => CANONICAL })
expect_fail(failures, "12. Python barrel omitting a service the other seven emit", out, status,
            "but NOT by python")

# --- Report --------------------------------------------------------------------

puts
if failures.empty?
  puts "==> service inventory parity self-test: all cases passed"
  exit 0
else
  warn "==> service inventory parity self-test: #{failures.length} case(s) failed"
  warn ""
  failures.each { |f| warn "  #{f}\n\n" }
  exit 1
end
