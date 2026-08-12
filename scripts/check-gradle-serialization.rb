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
# COVERAGE, HONESTLY. Four boundaries, and none of them is "every Gradle
# target":
#
#   1. ONE GOAL. Only targets reachable from check-targets (by default), because
#      that is the graph `make check` executes. kt-check-generated-drift is
#      DISCOVERED (see 2) but not COVERED: `make check` cannot schedule it, so
#      it collides with nothing there, while a hand-written
#      `make -j kt-check kt-check-generated-drift` still can. Chaining it would
#      charge CI, which invokes it as a standalone goal, for work it already
#      does another way. Pass a different goal to check a different graph.
#
#   2. ONE LEVEL OF DELEGATION, SHELL SCRIPTS ONLY. A recipe's own `./gradlew`
#      is found directly. A recipe that delegates — `@./scripts/foo.sh`, the
#      shape kt-check-generated-drift uses — is followed exactly one level, and
#      only into a file whose shebang names a shell. A script that delegates to
#      another script is not followed, and NEITHER IS A RUBY OR PYTHON HELPER
#      THAT SHELLS OUT TO GRADLE. There is none today; if one appears, this gate
#      will not see it, and that is a hole in this gate rather than a fact about
#      the repo. The shebang test is not squeamishness about Ruby: this file and
#      its self-test both contain the literal text `./gradlew` (in a regex, in
#      comments, in fixture heredocs) and are themselves reachable from
#      check-targets, so a scanner that read every delegated script would
#      classify the gate as a Gradle build in kotlin/ and fail on the real
#      Makefile.
#
#   3. THE KEY IS A PHYSICAL DIRECTORY, NOT A SPELLING. `cd ./kotlin` and
#      `cd kotlin` are canonicalized to one group; an unresolvable spelling — a
#      make variable, a shell variable this gate will not guess at — is a hard
#      error, not a group of one. Grouping on raw text was a real bypass, caught
#      in review: two spellings became two trivially-serialized groups.
#
#   4. TEXTUAL, AND IT ERRS TOWARD REPORTING. The delegation scan reads source,
#      it does not execute anything, so a `./gradlew` inside a shell heredoc
#      would be taken at face value. That direction is deliberate: a false
#      positive costs one unnecessary order-only edge and says so out loud; a
#      false negative costs the guarantee silently. An invocation it cannot
#      place is a hard error for the same reason.
#
# Env overrides (used by scripts/test-check-gradle-serialization.rb):
#   GRADLE_SERIALIZATION_MAKEFILE  makefile to read (default: Makefile)
#   GRADLE_SERIALIZATION_GOAL      goal to walk from (default: check-targets)
#   GRADLE_SERIALIZATION_ROOT      directory a recipe's `scripts/...` paths
#                                  resolve against (default: cwd). Test-only, so
#                                  the delegation branches can be exercised
#                                  against fixture trees instead of by adding
#                                  fixture scripts to the real scripts/.

require "English"
require "set"

MAKEFILE = ENV.fetch("GRADLE_SERIALIZATION_MAKEFILE", "Makefile")
GOAL = ENV.fetch("GRADLE_SERIALIZATION_GOAL", "check-targets")
REPO_ROOT = ENV.fetch("GRADLE_SERIALIZATION_ROOT", Dir.pwd)

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
#
# DELEGATION (boundary 2 in the header). A recipe that runs a shell script under
# scripts/ is followed one level into that script, because the live
# counterexample is exactly that shape: kt-check-generated-drift's recipe is
# `@./scripts/check-kotlin-generated-drift.sh`, and the Gradle invocation is at
# that script's line 71, spelled across a backslash continuation as
# `(cd "$ROOT_DIR/kotlin" && \` / `./gradlew ...`. Without this, the target is
# invisible: added to check-targets it is the exact "sixth target" case this
# gate exists to reject, and the gate would have passed.

# Scan shell text for a Gradle invocation. Returns the project directory as
# written, :unplaceable, or nil. Continuations are joined and whole-line
# comments dropped first, so a wrapped invocation reads as one command and a
# commented-out one is not mistaken for a live call.
def scan_shell_for_gradle(text)
  code = text.lines.reject { |l| l.match?(/\A\s*#/) }.join.gsub(/\\\n\s*/, " ")
  return nil unless code.include?("./gradlew")

  matched = code.match(%r{cd\s+["']?([^"'\s]+)["']?\s*&&[^\n]*?\./gradlew})
  return matched[1] if matched

  :unplaceable
end

# `$ROOT_DIR/kotlin` -> `kotlin`. Exactly one leading shell variable is stripped
# — the repo-root handle every script under scripts/ computes for itself — and
# the result must actually be a Gradle project, which is what keeps the strip
# from being a guess. Anything still holding a `$` is unplaceable.
def resolve_script_dir(raw, repo_root)
  dir = raw.sub(/\A\$\{?\w+\}?\/+/, "")
  return :unplaceable if dir.include?("$")
  return :unplaceable unless File.exist?(File.join(repo_root, dir, "gradlew"))

  dir
end

def delegated_gradle_dir(lines, repo_root)
  lines.each do |line|
    # Lookbehind rather than a leading-character class: the live case is
    # `@./scripts/check-kotlin-generated-drift.sh`, and make's recipe prefixes
    # (@, -, +) sit flush against the path.
    line.scan(%r{(?<![\w/.\-])\.?/?(scripts/[\w.\-]+)}) do |(rel)|
      path = File.join(repo_root, rel)
      next unless File.file?(path)

      # UTF-8 pinned for the same reason as the pipe read above: these scripts
      # carry em-dashes, and LC_ALL=C would make an unpinned read US-ASCII.
      source = File.read(path, encoding: "UTF-8")
      shebang = source.lines.first.to_s
      next unless shebang.start_with?("#!") && shebang.match?(/\b(?:ba|da|k|z)?sh\b/)

      found = scan_shell_for_gradle(source)
      next if found.nil?
      return [:unplaceable, rel] if found == :unplaceable

      resolved = resolve_script_dir(found, repo_root)
      return [resolved == :unplaceable ? :unplaceable : resolved, rel]
    end
  end
  nil
end

# The grouping key is a PHYSICAL directory, so it has to survive being spelled
# differently. `cd ./kotlin` and `cd kotlin` are the same Gradle project and must
# land in the same group; grouping on the raw text put them in two groups of one,
# each trivially "serialized", and a new target spelled the other way would have
# walked through the gate. `kotlin/`, `kotlin/.` and `spec/../kotlin` are the same
# hazard. Normalize to a repo-relative cleanpath before anything is compared.
def canonical_dir(raw, repo_root)
  root = File.expand_path(repo_root)
  absolute = File.expand_path(raw, root)
  return "." if absolute == root

  absolute.start_with?("#{root}/") ? absolute.delete_prefix("#{root}/") : absolute
end

def gradle_targets(recipes, makefile, repo_root = REPO_ROOT)
  found = {}
  unrecognized = []

  recipes.each do |target, lines|
    direct = lines.find { |line| line.include?("./gradlew") }

    if direct
      matched = direct.match(%r{cd\s+(\S+)\s*&&\s*\./gradlew})
      if matched && matched[1].include?("$")
        # A make variable is not resolvable from the database text, and guessing
        # would put this target in a group of its own — the same silent pass the
        # spelling bug caused.
        unrecognized << [target, direct]
      elsif matched
        found[target] = canonical_dir(matched[1], repo_root)
      elsif direct.match?(%r{\A\s*\./gradlew})
        found[target] = "."
      else
        unrecognized << [target, direct]
      end
      next
    end

    dir, via = delegated_gradle_dir(lines, repo_root)
    next if dir.nil?

    if dir == :unplaceable
      unrecognized << [target, "delegates to #{via}, which invokes Gradle in a directory this gate cannot resolve"]
    else
      # Canonicalized through the same funnel as a direct recipe: a delegated
      # `cd "$ROOT_DIR/./kotlin"` has to group with a direct `cd kotlin`.
      found[target] = canonical_dir(dir, repo_root)
    end
  end

  unless unrecognized.empty?
    warn "ERROR: cannot tell which project directory these Gradle recipes run in:"
    unrecognized.each { |target, line| warn "  #{target}: #{line}" }
    warn ""
    warn "This gate reads `cd <dir> && ./gradlew ...` in a recipe, and follows a " \
         "recipe ONE level into a shell script under scripts/ (not into a second " \
         "script, and not into a Ruby or Python helper). Spell it that way, or " \
         "teach scripts/check-gradle-serialization.rb the new shape — do not " \
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
