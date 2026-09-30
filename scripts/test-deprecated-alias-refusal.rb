#!/usr/bin/env ruby
# frozen_string_literal: true

# Self-test: a deprecated alias a generator cannot place fails the run, BY NAME.
#
# WHAT THIS PINS. A rename kept for compatibility (CardStep -> Subtask, #955)
# leaves the old component in openapi.json as nothing but a deprecated `$ref`,
# and every generator emits it as an alias of the new name. The committed spec
# carries exactly one such alias, and every generator can place it, so the drift
# checks only ever see the passing case. Before this, the generators each did
# something different with the shapes below — Go refused some, Ruby and Python
# skipped them in silence, and Ruby emitted an alias of a class it never
# defined, a NameError the moment types.rb loaded.
#
# WHAT IT DRIVES. The scripted generators on the stdlib toolchains the Spec Gates
# job already has: Go's post-pass (scripts/emit-go-deprecated-aliases.sh, jq),
# Ruby's generate-types.rb and Python's generate_types.py. The compiled four
# (TypeScript, Kotlin, Swift, Rust) get the same refusals as rows of
# scripts/test-compiled-generator-refusal, in their own language jobs.
#
# Each refusal asserts on the message (the alias, its target and the problem),
# not merely on the exit status, and that nothing was written. Each acceptance
# asserts on the artifact: the real CardStep alias is still emitted, a
# `deprecated` that is not the JSON boolean true makes no alias, and a reason
# carrying `*/`, quotes and line breaks stays inside its comment.
#
# Stdlib only. Wired into `make check` and the spec-gates CI job.

require "json"
require "open3"
require "tmpdir"

ROOT = File.expand_path("..", __dir__)
SPEC = JSON.parse(File.read(File.join(ROOT, "openapi.json"), encoding: "UTF-8"))
# The committed Go client WITHOUT the alias block the post-pass added, i.e. what
# oapi-codegen hands it, so the acceptance cases prove the pass emits the alias
# rather than finding it already there.
GO_CLIENT = File.read(File.join(ROOT, "go/pkg/generated/client.gen.go"))
  .sub(%r{\n// CardStep is the deprecated former name of Subtask\.\n//\n// Deprecated: [^\n]*\ntype CardStep = Subtask\n}, "")
abort "FAIL: the committed Go client no longer carries the CardStep alias block" unless
  GO_CLIENT.length < File.size(File.join(ROOT, "go/pkg/generated/client.gen.go"))

FAILURES = []
PASSES = []

def check(name)
  ok, detail = yield
  (ok ? PASSES : FAILURES) << (ok ? name : "#{name}\n    #{detail.to_s.gsub("\n", "\n    ")}")
end

# The committed spec plus ONE extra component.
def spec_with(name, component)
  doc = JSON.parse(JSON.generate(SPEC))
  doc["components"]["schemas"][name] = component
  doc
end

def alias_of(target, deprecated: true, reason: nil)
  component = { "$ref" => "#/components/schemas/#{target}", "deprecated" => deprecated }
  component["x-deprecated-reason"] = reason if reason
  component
end

# Runs one generator against `doc`; answers [status, output, artifact-or-nil].
def generate(language, doc)
  Dir.mktmpdir("deprecated-alias") do |dir|
    spec_path = File.join(dir, "openapi.json")
    File.write(spec_path, JSON.generate(doc))
    case language
    when :go
      file = File.join(dir, "client.gen.go")
      File.write(file, GO_CLIENT)
      output, status = Open3.capture2e(File.join(ROOT, "scripts/emit-go-deprecated-aliases.sh"), spec_path, file)
      artifact = File.read(file)
      # A refusal must leave the file exactly as it found it.
      artifact = nil if !status.success? && artifact == GO_CLIENT
    when :ruby
      artifact, errors, status = Open3.capture3("ruby", File.join(ROOT, "ruby/scripts/generate-types.rb"), spec_path)
      output = errors
      artifact = nil if artifact.empty?
    when :python
      out = File.join(dir, "types.py")
      output, status = Open3.capture2e("python3", File.join(ROOT, "python/scripts/generate_types.py"),
                                       "--openapi", spec_path, "--output", out)
      artifact = File.exist?(out) ? File.read(out) : nil
    end
    [ status.exitstatus, output, artifact ]
  end
end

# The alias as each generator spells it.
def emitted?(language, artifact, name, target)
  case language
  when :go then artifact.include?("\ntype #{name} = #{target}\n")
  when :ruby then artifact.include?("\n    #{name} = #{target}\n")
  when :python then artifact.include?("\n#{name} = #{target}\n")
  end
end

# A name each language already holds, which a deprecated alias may not take.
COLLISIONS = { go: "CreateCardStepJSONRequestBody", ruby: "TypeHelpers", python: "TypedDict" }.freeze

REFUSALS = [
  [ "an alias of an enum", "AliasOfEnum", "FirstWeekDay", "target is not an object model" ],
  [ "an alias of a missing schema", "AliasOfNothing", "NoSuchSchema", "target schema does not exist" ],
  [ "an alias of an alias, rejected rather than resolved", "AliasOfAlias", "CardStep",
    "target is itself a deprecated alias" ]
].freeze

%i[go ruby python].each do |language|
  cases = REFUSALS + [
    [ "an alias named like an existing type", COLLISIONS[language], "Subtask", "collides with an existing" ]
  ]
  # Ruby skips *RequestContent classes (SKIP_PATTERNS). The alias used to name
  # the class anyway; Go and Python emit that model, so there it is valid.
  if language == :ruby
    cases << [ "an alias of a class SKIP_PATTERNS excludes", "AliasOfRequest", "CreateCardStepRequestContent",
               "target class was not emitted" ]
  end

  cases.each do |description, name, target, problem|
    check("[#{language}] refuses #{description}") do
      status, output, artifact = generate(language, spec_with(name, alias_of(target)))
      named = output.include?("deprecated alias #{name} -> #{target}: ") && output.include?(problem)
      [ status != 0 && named && artifact.nil?,
        "exit #{status}, #{artifact ? 'wrote output' : 'wrote nothing'}; expected " \
        "'deprecated alias #{name} -> #{target}: ...#{problem}...', got: #{output}" ]
    end
  end

  check("[#{language}] still emits the real CardStep alias") do
    status, output, artifact = generate(language, SPEC)
    [ status.zero? && artifact && emitted?(language, artifact, "CardStep", "Subtask"), "exit #{status}: #{output}" ]
  end

  # Strict: only the JSON boolean true marks a deprecated alias.
  [ 1, "true", "false" ].each do |deprecated|
    check("[#{language}] deprecated: #{deprecated.inspect} makes no deprecated alias") do
      status, output, artifact = generate(language, spec_with("NotAnAlias", alias_of("Subtask", deprecated: deprecated)))
      [ status.zero? && artifact && !emitted?(language, artifact, "NotAnAlias", "Subtask") &&
        emitted?(language, artifact, "CardStep", "Subtask"), "exit #{status}: #{output}" ]
    end
  end

  check("[#{language}] a reason with */, quotes and line breaks stays inside its comment") do
    reason = "ends */ here \"q\"\nsecond\r\nthird"
    status, output, artifact = generate(language, spec_with("AliasReason", alias_of("Subtask", reason: reason)))
    next [ false, "exit #{status}: #{output}" ] unless status.zero? && artifact && emitted?(language, artifact, "AliasReason", "Subtask")

    # Every line from the alias's comment up to its declaration is a comment.
    lines = artifact.lines.map(&:chomp)
    at = lines.index { |l| l.strip.start_with?("AliasReason =", "type AliasReason =") }
    comment = lines[0...at].reverse.take_while { |l| l.strip.start_with?("#", "//") }.reverse
    [ comment.join("\n").include?("ends */ here") && comment.join("\n").include?("third"),
      "the reason escaped its comment:\n#{lines[(at - 6)..at].join("\n")}" ]
  end
end

PASSES.each { |name| puts "ok   #{name}" }
FAILURES.each { |failure| puts "FAIL #{failure}" }
puts "#{PASSES.size} passed, #{FAILURES.size} failed"
exit(FAILURES.empty? ? 0 : 1)
