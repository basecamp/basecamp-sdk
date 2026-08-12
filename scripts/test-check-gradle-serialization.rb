#!/usr/bin/env ruby
# frozen_string_literal: true

# Negative-case self-test for scripts/check-gradle-serialization.rb.
#
# The gate's live run in `make check` only ever sees a Makefile whose Gradle
# targets ARE chained, so nothing there shows it reacts when one is not. Each
# case below writes a MUTATED COPY of the real Makefile to a tmp file — one
# mutation each — drives the checker at it through GRADLE_SERIALIZATION_MAKEFILE,
# and asserts the expected verdict. The tracked Makefile is never written to.
#
# Cases 1-3 delete one order-only edge each: this is the literal "would it go red
# if the edges were removed" proof, run three times, once per edge. Case 4 is the
# one that matters more over time — it adds a SIXTH Gradle-backed target to
# check-targets with no edge at all, the shape a future change takes.
#
# Case 5 is the positive control (unmutated copy passes), and case 6 pins the
# gate's other end: a target chained but in a DIFFERENT project directory is not
# a collision and must not be reported.
#
# Run: ruby scripts/test-check-gradle-serialization.rb

require "English"
require "fileutils"
require "tmpdir"

CHECKER = ENV.fetch("GRADLE_SERIALIZATION_CHECKER", "scripts/check-gradle-serialization.rb")
MAKEFILE = File.read("Makefile", encoding: "UTF-8")

failures = []

def run_checker(makefile_path, root: nil)
  env = { "GRADLE_SERIALIZATION_MAKEFILE" => makefile_path }
  env["GRADLE_SERIALIZATION_ROOT"] = root if root

  # external_encoding pinned: the checker echoes Makefile recipe lines, which
  # carry em-dashes, and under LC_ALL=C an unpinned pipe read is US-ASCII (#669).
  out = IO.popen(
    env,
    ["ruby", CHECKER],
    err: [:child, :out], external_encoding: Encoding::UTF_8, &:read
  )
  [$CHILD_STATUS.exitstatus, out]
end

def check(name, mutated, expect_pass:, expect_fragment: nil, root: nil)
  Dir.mktmpdir("gradle-serialization") do |dir|
    path = File.join(dir, "Makefile.mutated")
    File.write(path, mutated, encoding: "UTF-8")
    status, out = run_checker(path, root: root)

    if expect_pass && status != 0
      return "#{name}: expected the gate to PASS, got exit #{status}\n#{out}"
    end
    if !expect_pass && status.zero?
      return "#{name}: expected the gate to FAIL, it passed\n#{out}"
    end
    if expect_fragment && !out.include?(expect_fragment)
      return "#{name}: expected output to mention #{expect_fragment.inspect}\n#{out}"
    end

    nil
  end
end

def must_substitute(source, from, to)
  raise "self-test is stale: #{from.inspect} no longer appears in the Makefile" \
    unless source.include?(from)

  source.sub(from, to)
end

# --- Cases 1-3: delete one order-only edge each ------------------------------

edges = {
  "1: smithy-mapper-test edge removed" =>
    ["smithy-mapper-test: | smithy-mapper", "smithy-mapper-test:"],
  "2: kt-test edge removed" =>
    ["kt-test: | conformance-runner-tests-kotlin", "kt-test:"],
  "3: conformance-kotlin edge removed" =>
    ["conformance-kotlin: | kt-test", "conformance-kotlin:"],
}

edges.each do |name, (from, to)|
  failures << check(
    name,
    must_substitute(MAKEFILE, from, to),
    expect_pass: false,
    expect_fragment: "can schedule two Gradle builds in one project directory",
  )
end

# --- Case 4: a new, unchained Gradle target joins check-targets --------------

with_new_target = must_substitute(
  MAKEFILE,
  "check-targets: ",
  "check-targets: probe-new-gradle-target ",
)
with_new_target += <<~MAKE

  .PHONY: probe-new-gradle-target
  probe-new-gradle-target:
  \tcd kotlin && ./gradlew :basecamp-sdk:jvmJar
MAKE

failures << check(
  "4: an unchained sixth Gradle target",
  with_new_target,
  expect_pass: false,
  expect_fragment: "probe-new-gradle-target",
)

# --- Case 5: positive control ------------------------------------------------

failures << check("5: unmutated Makefile", MAKEFILE, expect_pass: true)

# --- Case 6: a Gradle target in its own project directory is not a collision --

other_directory = must_substitute(
  MAKEFILE,
  "check-targets: ",
  "check-targets: probe-other-directory ",
)
other_directory += <<~MAKE

  .PHONY: probe-other-directory
  probe-other-directory:
  \tcd spec/smithy-bare-arrays && ./gradlew jar
MAKE

failures << check(
  "6a: a second target in an EXISTING directory still collides",
  other_directory,
  expect_pass: false,
  expect_fragment: "probe-other-directory",
)

lone_directory = must_substitute(
  MAKEFILE,
  "check-targets: ",
  "check-targets: probe-lone-directory ",
)
lone_directory += <<~MAKE

  .PHONY: probe-lone-directory
  probe-lone-directory:
  \tcd tools/probe-project && ./gradlew nothing
MAKE

failures << check(
  "6b: the only target in its directory is not a collision",
  lone_directory,
  expect_pass: true,
)

# --- Case 7: a Gradle recipe the gate cannot place must be an ERROR ----------
#
# The guard against the gate's own blind spot: `cd kotlin; ./gradlew` (semicolon,
# not &&) is a real Gradle invocation in kotlin/, and a checker that shrugged and
# filed it under the repo root would certify it collision-free.

unplaceable = must_substitute(
  MAKEFILE,
  "check-targets: ",
  "check-targets: probe-unplaceable ",
)
unplaceable += <<~MAKE

  .PHONY: probe-unplaceable
  probe-unplaceable:
  \tcd kotlin; ./gradlew :basecamp-sdk:jvmJar
MAKE

failures << check(
  "7: a Gradle recipe in an unrecognized shape",
  unplaceable,
  expect_pass: false,
  expect_fragment: "cannot tell which project directory",
)

# --- Cases 8-9: the DELEGATED target, which is a real one, not a fixture ------
#
# kt-check-generated-drift's recipe is `@./scripts/check-kotlin-generated-drift.sh`
# and the Gradle call is inside that script, wrapped across a backslash
# continuation and cd'd via a shell variable. It is the counterexample the gate's
# first cut could not see: added to check-targets it was the exact "sixth target"
# case, and the gate passed. Case 8 is that scenario; case 9 pins that the remedy
# is an EDGE, not a ban on the target.

delegated_unchained = must_substitute(
  MAKEFILE,
  "check-targets: ",
  "check-targets: kt-check-generated-drift ",
)

failures << check(
  "8: a Gradle target that delegates through a shell script",
  delegated_unchained,
  expect_pass: false,
  expect_fragment: "kt-check-generated-drift",
)

failures << check(
  "9: the same target, chained",
  delegated_unchained + "\nkt-check-generated-drift: | conformance-kotlin\n",
  expect_pass: true,
)

# --- Cases 10-11: the delegation boundaries, against fixture trees ------------
#
# These use GRADLE_SERIALIZATION_ROOT so the fixtures never enter the real
# scripts/. Case 10 pins the fail-loud direction. Case 11 pins the gate's DECLARED
# HOLE as measured behaviour rather than a claim in a comment: a Ruby helper that
# shells out to Gradle is NOT seen. It is asserted here so that the day someone
# writes one, this test is where they find out what the gate does not do — and so
# that "shell scripts only" cannot quietly become "all scripts" in the prose while
# the code says otherwise.

def with_fixture_root(script_name, script_body)
  Dir.mktmpdir("gradle-serialization-root") do |root|
    FileUtils.mkdir_p(File.join(root, "scripts"))
    FileUtils.mkdir_p(File.join(root, "kotlin"))
    FileUtils.touch(File.join(root, "kotlin", "gradlew"))
    File.write(File.join(root, "scripts", script_name), script_body, encoding: "UTF-8")
    yield root
  end
end

unplaceable_delegate = must_substitute(
  MAKEFILE,
  "check-targets: ",
  "check-targets: probe-delegate ",
) + <<~MAKE

  .PHONY: probe-delegate
  probe-delegate:
  \t@./scripts/probe-delegate.sh
MAKE

with_fixture_root("probe-delegate.sh", <<~SH) do |root|
  #!/usr/bin/env bash
  # The directory is not resolvable from the text: two variables, no literal.
  (cd "$SOME_DIR/$SUBPROJECT" && ./gradlew build)
SH
  failures << check(
    "10: a delegated Gradle call in a directory the gate cannot resolve",
    unplaceable_delegate,
    expect_pass: false,
    expect_fragment: "cannot tell which project directory",
    root: root,
  )
end

with_fixture_root("probe-delegate.rb", <<~RB) do |root|
  #!/usr/bin/env ruby
  # Declared hole: a Ruby helper that shells out to Gradle is not followed.
  system("cd kotlin && ./gradlew build")
RB
  failures << check(
    "11: a Ruby delegate is NOT followed (declared limitation, asserted)",
    must_substitute(MAKEFILE, "check-targets: ", "check-targets: probe-ruby-delegate ") + <<~MAKE,

      .PHONY: probe-ruby-delegate
      probe-ruby-delegate:
      \t@ruby ./scripts/probe-delegate.rb
    MAKE
    expect_pass: true,
    root: root,
  )
end

failures.compact!

if failures.empty?
  puts "check-gradle-serialization self-test: all cases passed"
  exit 0
end

warn "check-gradle-serialization self-test FAILED"
failures.each { |f| warn "\n#{f}" }
exit 1
