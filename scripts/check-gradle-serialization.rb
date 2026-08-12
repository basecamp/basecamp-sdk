#!/usr/bin/env ruby
# frozen_string_literal: true

# check-gradle-serialization — assert `make -j` can never schedule two Gradle
# invocations in the same project directory (#674).
#
# THE HAZARD. Two concurrent Gradle builds sharing one project directory share
# <project>/.gradle (file-hash and task-history caches — the FilePageCache) and
# write the same task output directories. Gradle's cross-process locks serialize
# the caches; nothing serializes the outputs. `check-targets` lists five
# Gradle-backed targets as INDEPENDENT prerequisites, in two project
# directories:
#
#   spec/smithy-bare-arrays   smithy-mapper (via smithy-check), smithy-mapper-test
#   kotlin                    kt-test (via kt-check), conformance-runner-tests-kotlin,
#                             conformance-kotlin (both via conformance)
#
# The fix is order-only prerequisites in the Makefile that chain each group into
# a total order. This gate asserts those edges are there — and, more usefully,
# that a SIXTH Gradle target added to check-targets tomorrow cannot land
# unchained.
#
# WHY A GRAPH CHECK AND NOT A STRESS TEST. Running `make -j16 check` repeatedly
# and watching for a failure measures luck, not the fix: two concurrent Gradle
# builds in one directory usually survive (measured — see the PR that added
# this), which is exactly why the collision sat unnoticed. What is decidable is
# the schedule. If X is a transitive prerequisite of Y — order-only or not —
# make cannot start Y's recipe until X's has finished. So this reads make's own
# parsed database (`make -p`, not a hand-rolled Makefile parser) and asserts
# that within each project directory the reachable Gradle targets are totally
# ordered by that relation. That is the mechanism, and it goes red the moment an
# edge is deleted.
#
# COVERAGE, HONESTLY. This checks the targets reachable from ONE goal
# (check-targets by default), because that is the graph `make check` executes.
# Gradle-backed targets outside it — kt-check-generated-drift is the live
# example — are NOT covered: `make check` cannot schedule them, but a
# hand-written `make -j kt-check kt-check-generated-drift` still can. Chaining
# those would charge CI, which invokes them as standalone goals, for work it
# already does a different way. Pass a different goal to check a different
# graph.
#
# Env overrides (used by scripts/test-check-gradle-serialization.rb):
#   GRADLE_SERIALIZATION_MAKEFILE  makefile to read (default: Makefile)
#   GRADLE_SERIALIZATION_GOAL      goal to walk from (default: check-targets)

require "English"
require "set"

MAKEFILE = ENV.fetch("GRADLE_SERIALIZATION_MAKEFILE", "Makefile")
GOAL = ENV.fetch("GRADLE_SERIALIZATION_GOAL", "check-targets")

# A goal make cannot resolve: make prints its whole parsed database, then exits
# non-zero without running or recursing into a single recipe. Asking for a real
# goal under -n would recurse into every `$(MAKE)` sub-make, which make runs even
# in dry-run mode.
PROBE_GOAL = "__gradle_serialization_probe__"

def die(message)
  warn "ERROR: #{message}"
  exit 1
end

# Make's database, parsed. Returns [graph, recipes]:
#   graph[target]   => Set of prerequisites (order-only and normal alike; both
#                      constrain the schedule identically, which is all we ask)
#   recipes[target] => Array of recipe lines
def read_make_database(makefile)
  # MAKEFLAGS is cleared because this runs from inside `make check`: inheriting
  # the parent's flags would hand the child a jobserver it cannot join (and, with
  # a stale -j, a warning on every run). Nothing here needs them — the database
  # is a parse of the makefile, not of the invocation.
  #
  # external_encoding is pinned for the same reason the Gradle builds beside this
  # gate pin theirs (#669): under LC_ALL=C Ruby reads a pipe as US-ASCII, and the
  # Makefile this dumps is full of em-dashed comments, so an unpinned read raises
  # ArgumentError on the first one. Caught by `LC_ALL=C make`, not by `make`.
  dump = IO.popen(
    { "MAKEFLAGS" => "", "MAKELEVEL" => "0" },
    ["make", "-p", "-n", "--no-print-directory", "-f", makefile, PROBE_GOAL],
    err: File::NULL, external_encoding: Encoding::UTF_8, &:read
  )
  die "could not read make's database from #{makefile}" if dump.nil? || dump.empty?

  graph = {}
  recipes = Hash.new { |h, k| h[k] = [] }

  in_files = false
  current = nil
  not_a_target = false

  dump.each_line do |raw|
    line = raw.chomp

    # The "# Files" section holds the target database. Everything before it is
    # variables and implicit rules; everything after is hash-table statistics.
    if line.start_with?("# Files")
      in_files = true
      next
    end
    next unless in_files
    break if line.start_with?("# files hash-table stats", "# VPATH Search Paths")

    if line.empty?
      current = nil
      not_a_target = false
      next
    end

    if line.start_with?("\t")
      recipes[current] << line.sub(/\A\t/, "") if current
      next
    end

    if line.start_with?("#")
      not_a_target = true if line.include?("Not a target")
      next
    end

    # `target: normal-prereqs | order-only-prereqs`
    match = /\A([^:=#]+):(?!=)(.*)\z/.match(line)
    next if match.nil?

    if not_a_target
      current = nil
      next
    end

    target = match[1].strip
    prereqs = match[2].tr("|", " ").split(/\s+/).reject(&:empty?)
    current = target
    (graph[target] ||= Set.new).merge(prereqs)
  end

  [graph, recipes]
end

# Every target whose recipe shells out to a Gradle wrapper, mapped to the
# project directory it runs in — `cd <dir> && ./gradlew ...`, the only shape
# this repo uses. A bare `./gradlew` would mean the repo root.
#
# An unrecognized shape is a hard error rather than a default, because the
# default would be a SILENT PASS: a recipe spelled `cd kotlin; ./gradlew` would
# be filed under the repo root, share a group with nothing, and be certified
# collision-free while colliding with everything in kotlin/. The gate's own
# blind spot is the one failure it must not have.
def gradle_targets(recipes, makefile)
  found = {}
  unrecognized = []

  recipes.each do |target, lines|
    lines.each do |line|
      next unless line.include?("./gradlew")

      matched = line.match(%r{cd\s+(\S+)\s*&&\s*\./gradlew})
      if matched
        found[target] = matched[1]
      elsif line.match?(%r{\A\s*\./gradlew})
        found[target] = "."
      else
        unrecognized << [target, line]
      end
      break
    end
  end

  unless unrecognized.empty?
    warn "ERROR: cannot tell which project directory these Gradle recipes run in:"
    unrecognized.each { |target, line| warn "  #{target}: #{line}" }
    warn ""
    warn "This gate reads `cd <dir> && ./gradlew ...`. Spell the recipe that way, " \
         "or teach scripts/check-gradle-serialization.rb the new shape — do not " \
         "leave it guessing, since a guess here passes silently. (#{makefile})"
    exit 1
  end

  found
end

def reachable_from(graph, root)
  seen = Set.new
  stack = [root]
  until stack.empty?
    node = stack.pop
    next unless seen.add?(node)

    (graph[node] || []).each { |prereq| stack.push(prereq) }
  end
  seen
end

graph, recipes = read_make_database(MAKEFILE)
die "goal #{GOAL} is not defined in #{MAKEFILE}" unless graph.key?(GOAL)

gradle = gradle_targets(recipes, MAKEFILE)
die "found no Gradle-backed targets in #{MAKEFILE} — the recipe shape this " \
    "gate matches (`cd <dir> && ./gradlew`) must have changed" if gradle.empty?

in_goal = reachable_from(graph, GOAL)
covered = gradle.select { |target, _| in_goal.include?(target) }

# Precompute each covered target's ancestry once; the pairwise test below is
# "does one of the pair reach the other".
closure = covered.keys.to_h { |target| [target, reachable_from(graph, target)] }

violations = []
chains = []

covered.group_by { |_, dir| dir }.sort.each do |dir, entries|
  targets = entries.map(&:first).sort
  next if targets.size < 2

  targets.combination(2).each do |a, b|
    ordered = closure[a].include?(b) || closure[b].include?(a)
    next if ordered

    violations << [dir, a, b]
  end

  # Report the group in EXECUTION order when it is totally ordered: the target
  # depending on the fewest others in its directory runs first.
  ranked = targets.sort_by { |t| (closure[t] & targets).size }
  chains << [dir, ranked]
end

if violations.empty?
  chains.each do |dir, ranked|
    puts "#{dir}: #{ranked.join(' -> ')}"
  end
  puts "Gradle targets reachable from #{GOAL} are serialized per project directory"
  exit 0
end

warn "ERROR: `make -j #{GOAL}` can schedule two Gradle builds in one project directory."
warn ""
violations.each do |dir, a, b|
  warn "  #{dir}: #{a} and #{b} are independent — neither is a prerequisite of the other"
end
warn ""
warn "Add an order-only prerequisite (`target: | other-target`) chaining them."
warn "See the anchor comment on smithy-mapper-test in the Makefile (#674)."
exit 1
