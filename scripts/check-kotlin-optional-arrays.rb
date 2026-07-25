#!/usr/bin/env ruby
# frozen_string_literal: true

# D-invariant guard for the Kotlin generator's optional-array contract.
#
# Every OPTIONAL generated array must be `List<T>? = null` so an absent array
# stays distinct from a present-but-empty one (SPEC.md Optional Fields rule),
# aligning Kotlin with the other five SDKs. Every REQUIRED array stays a plain
# non-null `List<T>`. No generated array may default to the `= emptyList()`
# sentinel that this fix removed.
#
# Concretely, this scans every generated Kotlin declaration of a `List<...>`
# property and enforces:
#   * a property with a default MUST be nullable with a `= null` default
#     (so `= emptyList()`, or any other non-null default, fails);
#   * a nullable array that carries a default MUST default to exactly `null`.
# A required array (`List<T>` with no default) and a nullable request param
# (`List<T>?` with no default) both pass.
#
# Pins the D fix against regression. Wired into `make check`.

PROJECT_ROOT = File.expand_path("..", __dir__)
GENERATED_DIR = File.join(
  PROJECT_ROOT,
  "kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/generated"
)

errors = []

files = Dir.glob(File.join(GENERATED_DIR, "**", "*.kt")).sort
if files.empty?
  warn "ERROR: no generated Kotlin files found under #{GENERATED_DIR}"
  exit 2
end

files.each do |path|
  rel = path.sub("#{PROJECT_ROOT}/", "")
  File.foreach(path).with_index(1) do |line, lineno|
    # Match an array-typed property declaration: capture everything after the
    # first `: List<`. Generated properties are one per line.
    next unless line =~ /:\s*(List<.*)$/

    decl = Regexp.last_match(1).strip.sub(/[,)]\s*$/, "").strip

    type_part, default_part =
      if decl.include?("=")
        left, right = decl.split("=", 2)
        [left.strip, right.strip]
      else
        [decl, nil]
      end

    nullable = type_part.end_with?("?")

    next if default_part.nil? # required array, or nullable param w/o default — OK

    if default_part == "emptyList()"
      errors << "#{rel}:#{lineno}: optional array uses the forbidden " \
                "`= emptyList()` sentinel — must be `List<T>? = null`"
    elsif !nullable
      errors << "#{rel}:#{lineno}: non-null array `#{type_part}` carries a " \
                "default (`= #{default_part}`) — an optional array must be " \
                "nullable (`List<T>? = null`)"
    elsif default_part != "null"
      errors << "#{rel}:#{lineno}: nullable array defaults to " \
                "`#{default_part}` — must default to `null`"
    end
  end
end

if errors.empty?
  puts "==> Kotlin optional-array invariant clean — #{files.size} generated files scanned"
  exit 0
else
  warn "Kotlin optional-array invariant failed:"
  errors.each { |e| warn "  - #{e}" }
  exit 1
end
