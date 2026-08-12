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
#   3. IT READS THE COMMAND, NOT THE TEXT — and that is a correction, not a
#      boast. Three review rounds each produced one spelling that walked past a
#      literal-text scan: `cd ./kotlin` (own group of one), `cd "kotlin"`
#      (quotes as pathname characters), `$(GRADLE_WRAPPER)` (the wrapper itself
#      behind a make variable, so the word "gradlew" never appeared). Each was a
#      SILENT PASS in a control whose whole job is to catch the target nobody
#      has written yet, and a fourth spelling would have been a fourth matcher
#      arm. So the question moved to the layer make already answers: recipes are
#      expanded against make's own variable database, quotes are stripped in the
#      one place a directory becomes a grouping key, and paths are canonicalized
#      to a repo-relative cleanpath. One funnel, not one arm per spelling.
#
#      WHAT THAT STILL DOES NOT REACH, precisely:
#        - a value only a RUNNING shell knows (`cd $$RUNTIME_DIR`, `$(shell …)`).
#          Hard error, not a silent pass — the fail-loud side.
#        - a `define`/`endef` multi-line variable holding the Gradle call. That
#          one WOULD be missed silently: the reference stays unexpanded, so no
#          `gradlew` token appears and the target is never classified. Nothing in
#          this repo does it, and closing it means teaching this gate a second
#          slice of make's grammar — recorded here rather than guessed at, so the
#          next reader knows it is a hole and not an oversight.
#
#      A target may build in MORE THAN ONE project directory, and belongs to a
#      group for each. Recording only its first invocation left the second in no
#      group at all, so a target correctly ordered against the Kotlin chain could
#      still overlap the mapper targets with its second command. That has to be
#      TOTAL at every nesting level — every recipe line, every invocation on a
#      line, every invocation in a delegated script — and the counts must
#      balance: an occurrence the pattern cannot place makes the whole line
#      unplaceable rather than quietly dropping one directory. Fixing it per
#      recipe but not per line was itself one review round of partial.
#
#   4. TEXTUAL, AND IT ERRS TOWARD REPORTING. Nothing is executed, so a
#      `./gradlew` inside a shell heredoc is taken at face value. That direction
#      is deliberate: a false positive costs one unnecessary order-only edge and
#      says so out loud; a false negative costs the guarantee silently. An
#      invocation it cannot place is a hard error for the same reason.
#
# WHY THIS KEEPS FINDING THINGS, AND THE END-STATE IT IS NOT. Six review rounds
# produced twelve defects in this gate, every one of them a SILENT PASS: an
# undiscovered target, then a spelling, then two spellings, then a variable-name
# charset, a symlink alias, a target with two directories, two invocations on one
# line, a continued line, and a target that both called Gradle and delegated.
# That tail is a property of the instrument — static analysis of a shell-embedded
# DSL — and not of any one selector, which is why the later rounds were answered
# at the funnel (expand from make's database, canonicalize in one place, model a
# target as a SET of directories, fold continuations and union direct with
# delegated everywhere) rather than with a matcher arm apiece.
#
# The recurring shape is worth naming for whoever extends this: nearly every
# defect was an operation applied in ONE of the two scanning paths and not the
# other, or applied at one nesting level and not the one below. Recipes and
# delegated scripts are now folded, expanded, scanned and canonicalized the same
# way, and direct and delegated are unioned rather than treated as alternatives.
# If you add a step, add it to both, or the next round finds it.
#
# ONE DEFECT HAS A DIFFERENT SHAPE, AND IT ARRIVED LAST. Every finding above is
# a silent pass — the gate says yes when it should say no. Target-specific
# variables were the opposite: make -p prints them inside a target's block as
# comments, so expansion never saw them, `$(PROJECT)` stayed unresolved, and the
# fail-loud `$`-guard refused to run. The gate said NO when it should have said
# YES. That matters for how the fail-loud default is read: erring toward
# reporting is the right direction for a guard, but it is not free, and a guard
# that cannot be run is not a safe default either. Case 25 pins it.
#
# SCORE THE CONTROL, NOT THE INCREMENT. Every one of those twelve fixes cleared
# "small and obviously correct" on its own, and that is precisely how a control
# nobody sized gets built. The honest tally: the rules this gate applies have
# grown at every round; the CALL SITES it covers have grown exactly once, when
# delegation brought in kt-check-generated-drift. Everything since has been the
# same five targets, parsed harder.
#
# The count is now SEVEN rounds and fifteen defects, and one of them — the
# delegate scan returning on the first script — was the FIFTH appearance of a
# single defect: an operation made total at one nesting level and left partial at
# the next. Lines, then per-line invocations, then scripts, then direct-plus-
# delegated, then the delegate loop itself. That is not five findings; it is one
# finding found five times, which is the clearest possible evidence about the
# instrument rather than about any rule.
#
# So: NO MORE SELECTORS. A further parsing variation is not a task, it is the
# signal to switch instruments, and the two candidates are written down here so
# nobody has to derive them under review pressure.
#
#   A. BOUND WHERE GRADLE CAN BE REACHED AT ALL. Route every Makefile Gradle
#      invocation through one sanctioned variable or wrapper target. The gate
#      then inspects a single place instead of recognizing every spelling of a
#      recipe, and reduces to "no recipe mentions gradlew except through it" plus
#      the ordering check — a syntactic invariant that holds for call sites not
#      written yet, which is the thing a matcher can never promise. This is the
#      structural fix. Recipe text has unbounded shapes; a regex will not
#      enumerate them. Cost: a Makefile refactor, and it still would not cover
#      the one Gradle call that happens outside make entirely
#      (scripts/check-kotlin-generated-drift.sh) — that one needs its own rule.
#
#   B. ASSERT THE SCHEDULE INSTEAD OF PARSING THE TEXT. Put a shim on gradlew
#      that records (project_dir, start, end) per invocation during a real
#      `make -j16 check-targets`, then assert no two intervals for the same
#      project directory overlap. That observes the mechanism itself, is immune
#      to recipe syntax, and covers delegated scripts and non-make callers for
#      free. Note this is NOT the run-to-failure approach already rejected
#      above: two concurrent builds both exit 0, so watching for a failure
#      measures luck — but RECORDING THE SCHEDULE measures the thing the gate
#      actually claims. Cost: it needs a real parallel build, which may be too
#      slow for CI, and it only sees the call sites that run.
#
# A and B are complementary, not alternatives: A holds a shape across call sites
# that do not exist yet, B observes behaviour at the ones that execute. Neither
# is built here, because this PR's subject is the missing order-only edges, not a
# Gradle-discovery framework.
#
# WHAT THIS GATE CAN AND CANNOT SEE, so a green run is not over-read:
#
#   CAN   a `cd <dir> && ./gradlew` in a recipe, after make-variable expansion,
#         continuation folding and quote stripping; the same one level deep in a
#         shell script a recipe invokes; several invocations per line, per
#         recipe and per script; the same directory under any spelling or
#         symlink.
#   CANNOT a Gradle call reached by a shape it does not parse: a `define`/`endef`
#         variable, a Ruby or Python helper that shells out, a second level of
#         script delegation, a directory only a running shell knows (that last
#         one is a hard error, not a silent pass — the others are silent).
#
# A PASS THEREFORE MEANS "no collision in the shapes I can parse", NOT "no
# collision exists". The pass message says so out loud, because a guard that
# overstates its reach is worse than one that states a narrow reach accurately:
# the first stops people looking.
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
  variables = {}
  target_variables = Hash.new { |h, k| h[k] = {} }

  in_variables = false
  in_files = false
  current = nil
  not_a_target = false

  dump.each_line do |raw|
    line = raw.chomp

    # The "# Variables" section holds make's own values for every variable it
    # knows. Reading it is what lets the recipe scan below see the COMMAND make
    # would run rather than the text someone typed.
    if line.start_with?("# Variables")
      in_variables = true
      next
    end

    # The "# Files" section holds the target database. Everything before it is
    # variables and implicit rules; everything after is hash-table statistics.
    if line.start_with?("# Files")
      in_variables = false
      in_files = true
      next
    end

    if in_variables
      # `NAME = value`, `NAME := value`, `NAME ?= value`. Multi-line `define`
      # blocks are skipped: nothing in this repo puts a Gradle call in one, and
      # half-reading one would be worse than not reading it.
      if (var = /\A([^\s:#=]+) *[:?+]?= ?(.*)\z/.match(line))
        variables[var[1]] = var[2]
      end
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
      # Target-specific variables (`probe: PROJECT = kotlin`) never reach the
      # rule parser below: make -p prints them INSIDE the target's block as
      # comments, which is also why they were invisible to recipe expansion —
      # `$(PROJECT)` stayed unresolved and tripped the fail-loud `$`-guard, so
      # the symptom was the gate refusing to run rather than a silent pass.
      # `# Load=77/1024=8%` and friends carry no spaces around `=` and so do not
      # match.
      if current && (tvar = /\A#\s*([A-Za-z_][^\s=]*) *[:+?]?= (.*)\z/.match(line))
        target_variables[current][tvar[1]] = tvar[2]
      end
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

  [graph, recipes, variables, target_variables]
end

# Substitute make's own variable values into a recipe line, so what gets matched
# is the command make would run. Bounded passes because a value may itself hold
# a reference; an unknown name is left alone, which lands it in the fail-loud
# `$`-guard rather than being quietly dropped.
#
# This exists because the alternative is a matcher that keeps growing: a review
# round found `cd kotlin && $(GRADLE_WRAPPER) help` invisible to a literal-text
# scan, and the round before found `cd ./kotlin` in its own group. Both are the
# same defect — reading the text instead of the command — and widening the
# pattern for each spelling is how a control nobody sized gets built. Expanding
# once, from make's own database, is the layer the question actually lives at.
def expand_variables(text, variables, passes: 5)
  passes.times do
    # `[^\s(){}:#]+` rather than `\w+`: GNU make allows punctuation in variable
    # names (`GRADLE-WRAPPER`), and a name this did not recognize stayed
    # unexpanded — the same silent miss the expansion was added to remove.
    # `$(shell …)` and `$(call …)` hold spaces, so they never match and are left
    # to the fail-loud `$`-guard.
    expanded = text.gsub(/\$[({]([^\s(){}:#]+)[)}]/) { variables.fetch(Regexp.last_match(1), Regexp.last_match(0)) }
    return expanded if expanded == text

    text = expanded
  end
  text
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

# Fold `foo && \` + `bar` into one logical command. make -p prints a recipe's
# continued lines separately, and the conventional wrapped form
# `cd spec/smithy-bare-arrays && \` / `./gradlew help` then read as a bare
# repo-root invocation with the directory thrown away. The delegated-script scan
# already did this folding; the recipe scan did not, which is the whole of that
# defect — the same operation applied in one place and not the other.
def join_continuations(lines)
  folded = []
  lines.each do |line|
    if !folded.empty? && folded.last.end_with?("\\")
      folded[-1] = "#{folded.last.chomp('\\')} #{line.strip}"
    else
      folded << line
    end
  end
  folded
end

# Scan shell text for a Gradle invocation. Returns the project directory as
# written, :unplaceable, or nil. Continuations are joined and whole-line
# comments dropped first, so a wrapped invocation reads as one command and a
# commented-out one is not mistaken for a live call.
def scan_shell_for_gradle(text)
  code = text.lines.reject { |l| l.match?(/\A\s*#/) }.join.gsub(/\\\n\s*/, " ")
  return nil unless code.include?("./gradlew")

  dirs = code.scan(%r{cd\s+["']?([^"'\s]+)["']?\s*&&[^\n]*?\./gradlew}).flatten
  return :unplaceable if dirs.size != code.scan("./gradlew").size

  dirs
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

# EVERY delegated script, not the first. Returning on the first one repeated, at
# the outermost nesting level, the same partiality the line and per-script scans
# already had to be fixed for: a target delegating to two scripts kept only the
# first script's projects, so it could be correctly chained for one and race
# another target in the other.
def delegated_gradle_dir(lines, repo_root)
  dirs = []
  vias = []
  unplaceable_via = nil

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

      if found == :unplaceable
        unplaceable_via ||= rel
        next
      end

      resolved = found.map { |dir| resolve_script_dir(dir, repo_root) }
      if resolved.include?(:unplaceable)
        unplaceable_via ||= rel
        next
      end

      dirs.concat(resolved)
      vias << rel
    end
  end

  return [:unplaceable, unplaceable_via] if unplaceable_via
  return nil if dirs.empty?

  [dirs, vias.uniq.join(", ")]
end

# The grouping key is a PHYSICAL directory, so it has to survive being spelled
# differently. `cd ./kotlin` and `cd kotlin` are the same Gradle project and must
# land in the same group; grouping on the raw text put them in two groups of one,
# each trivially "serialized", and a new target spelled the other way would have
# walked through the gate. `kotlin/`, `kotlin/.` and `spec/../kotlin` are the same
# hazard. Normalize to a repo-relative cleanpath before anything is compared.
# expand_path normalizes `.` and `..` but not symlinks, and the claim this gate
# makes is about a PHYSICAL directory: two paths that are the same inode share
# one <project>/.gradle whatever they are called. realpath only works on paths
# that exist, so a directory that does not (a fixture, a target for a tree not
# checked out) falls back to the lexical form rather than failing.
def physical(path)
  File.realpath(path)
rescue SystemCallError
  path
end

def canonical_dir(raw, repo_root)
  # `cd "kotlin"` is `cd kotlin`. Dequoting lives here, in the one place a
  # directory becomes a grouping key, rather than as another arm of the matcher.
  unquoted = raw.sub(/\A(["'])(.*)\1\z/) { Regexp.last_match(2) }
  root = physical(File.expand_path(repo_root))
  absolute = physical(File.expand_path(unquoted, root))
  return "." if absolute == root

  absolute.start_with?("#{root}/") ? absolute.delete_prefix("#{root}/") : absolute
end

def gradle_targets(recipes, makefile, variables = {}, target_variables = {}, repo_root = REPO_ROOT)
  found = {}
  unrecognized = []

  recipes.each do |target, raw_lines|
    # Target-specific variables shadow global ones, as make itself resolves them.
    scope = variables.merge(target_variables[target] || {})
    lines = join_continuations(raw_lines.map { |line| expand_variables(line, scope) })
    # EVERY line, not the first match: one recipe may build in two projects, and
    # recording only its first invocation left the second in no group at all —
    # so a target ordered after the Kotlin chain could still overlap the mapper
    # targets with its second command. A target belongs to as many groups as it
    # has directories.
    direct = lines.select { |line| line.include?("./gradlew") }

    direct.each do |line|
      # EVERY invocation on the line, and the count must balance. Taking the
      # first match made the per-recipe fix above total per TARGET but not per
      # LINE, so `cd kotlin && ./gradlew help; cd spec/… && ./gradlew help`
      # was filed under kotlin/ alone. Requiring dirs.size to equal the number
      # of `./gradlew` occurrences is what makes it total rather than one
        # nesting level less partial: an invocation this cannot place is an
      # unplaceable LINE, not a silently dropped directory.
      dirs = line.scan(%r{cd\s+(\S+)\s*&&\s*\./gradlew}).flatten
      occurrences = line.scan("./gradlew").size

      if dirs.size != occurrences
        if dirs.empty? && occurrences == 1 && line.match?(%r{\A\s*\./gradlew})
          (found[target] ||= Set.new) << "."
        else
          unrecognized << [target, line]
        end
        next
      end

      if dirs.any? { |dir| dir.include?("$") }
        # Not resolvable from the database text, and guessing would put this
        # target in a group of its own — the same silent pass the spelling bug
        # caused.
        unrecognized << [target, line]
        next
      end

      dirs.each { |dir| (found[target] ||= Set.new) << canonical_dir(dir, repo_root) }
    end

    # NOT `next` when a direct call was found: one target may do both, and
    # skipping the delegate scan then dropped the script's project entirely.
    # Direct and delegated are two ways to reach Gradle, not two kinds of
    # target, so both are always scanned and the results union.
    dir, via = delegated_gradle_dir(lines, repo_root)
    next if dir.nil?

    if dir == :unplaceable
      unrecognized << [target, "delegates to #{via}, which invokes Gradle in a directory this gate cannot resolve"]
    else
      # Canonicalized through the same funnel as a direct recipe: a delegated
      # `cd "$ROOT_DIR/./kotlin"` has to group with a direct `cd kotlin`.
      dir.each { |d| (found[target] ||= Set.new) << canonical_dir(d, repo_root) }
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

graph, recipes, variables, target_variables = read_make_database(MAKEFILE)
die "goal #{GOAL} is not defined in #{MAKEFILE}" unless graph.key?(GOAL)

gradle = gradle_targets(recipes, MAKEFILE, variables, target_variables)
die "found no Gradle-backed targets in #{MAKEFILE} — the recipe shape this " \
    "gate matches (`cd <dir> && ./gradlew`) must have changed" if gradle.empty?

in_goal = reachable_from(graph, GOAL)
covered = gradle.select { |target, _| in_goal.include?(target) }

# dir => targets that build in it. A target with two directories appears twice,
# and must be ordered against the others in BOTH.
by_directory = Hash.new { |h, k| h[k] = [] }
covered.each { |target, dirs| dirs.each { |dir| by_directory[dir] << target } }

# Precompute each covered target's ancestry once; the pairwise test below is
# "does one of the pair reach the other".
closure = covered.keys.to_h { |target| [target, reachable_from(graph, target)] }

violations = []
chains = []

by_directory.sort.each do |dir, entries|
  targets = entries.uniq.sort
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
  puts "OK: no two Gradle invocations THIS GATE CAN SEE share a project directory"
  puts "    concurrently, in the graph reachable from #{GOAL}."
  puts "    Scope: parsed recipe text (variables expanded, continuations folded) plus"
  puts "    one level into shell scripts a recipe runs. A pass means \"no collision in"
  puts "    the shapes I can parse\", NOT \"no collision exists\" — a define'd variable,"
  puts "    a Ruby/Python helper or a second level of delegation is invisible here."
  puts "    See the header of scripts/check-gradle-serialization.rb."
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
warn ""
warn "And note the converse of this report: these are the collisions in the shapes"
warn "this gate can parse. It cannot see a Gradle call reached through a define'd"
warn "variable, a Ruby/Python helper, or a second level of script delegation, so"
warn "fixing what is listed here does not by itself prove there are no others."
exit 1
