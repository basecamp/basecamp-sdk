#!/usr/bin/env ruby
# Validates spec/api-gaps/*.md frontmatter and required body sections,
# and spec/api-gaps/allowlist.yml against its schema.
# Uses stdlib only (yaml, json) — no gem dependencies.

require "date"
require "json"
require "yaml"

GAPS_DIR = ARGV.fetch(0) { "spec/api-gaps" }
SCHEMA_FILE = File.join(GAPS_DIR, "schema.json")
ALLOWLIST_FILE = File.join(GAPS_DIR, "allowlist.yml")
ALLOWLIST_SCHEMA_FILE = File.join(GAPS_DIR, "allowlist-schema.json")

REQUIRED_BODY_SECTIONS = [
  "## What's missing",
  "## Why it matters",
  "## Suggested API shape",
  "## Implementation notes for BC3",
  "## SDK absorption plan when this lands"
].freeze

errors = []

def add_error(errors, file, message)
  errors << "#{file}: #{message}"
end

# --- Frontmatter validators ----------------------------------------------------

unless File.file?(SCHEMA_FILE)
  warn "ERROR: schema not found at #{SCHEMA_FILE}"
  exit 2
end

schema = JSON.parse(File.read(SCHEMA_FILE))
required_fields = schema.dig("required") || []
known_fields = schema.dig("properties")&.keys || []
additional_strict = schema["additionalProperties"] == false
status_alts = schema.dig("properties", "status", "oneOf") || []
status_enum = status_alts.flat_map { |alt| alt["enum"] || [] }
status_pattern = status_alts.find { |alt| alt["pattern"] }&.dig("pattern")
date_pattern = schema.dig("properties", "detected", "pattern")

def parse_frontmatter(path)
  content = File.read(path)
  return [nil, content, "missing leading frontmatter delimiter"] unless content.start_with?("---\n")

  rest = content[4..]
  end_idx = rest.index("\n---\n")
  return [nil, content, "missing closing frontmatter delimiter"] unless end_idx

  yaml_text = rest[0...end_idx]
  body = rest[(end_idx + 5)..]
  begin
    [YAML.safe_load(yaml_text, permitted_classes: [Date, Time]), body, nil]
  rescue Psych::SyntaxError => e
    [nil, body, "YAML parse error: #{e.message}"]
  end
end

def validate_frontmatter(meta, file, required_fields, known_fields, additional_strict, status_enum, status_pattern, date_pattern, errors, expected_gap)
  unless meta.is_a?(Hash)
    add_error(errors, file, "frontmatter is not a mapping")
    return
  end

  required_fields.each do |field|
    unless meta.key?(field)
      add_error(errors, file, "missing required frontmatter field: #{field}")
    end
  end

  if additional_strict
    extra = meta.keys - known_fields
    extra.each do |k|
      add_error(errors, file, "unknown frontmatter field: #{k} (schema declares additionalProperties: false)")
    end
  end

  if (gap = meta["gap"]) && expected_gap && gap != expected_gap
    add_error(errors, file, "frontmatter gap=#{gap.inspect} does not match filename #{expected_gap.inspect}")
  end

  if (status = meta["status"])
    unless status.is_a?(String)
      add_error(errors, file, "status must be a string, got #{status.class}: #{status.inspect}")
      return
    end
    valid = status_enum.include?(status) || (status_pattern && status.match?(Regexp.new(status_pattern)))
    unless valid
      add_error(errors, file, "status #{status.inspect} not in enum and does not match pattern")
    end

    case status
    when "absorbed-in-sdk"
      smithy_refs = meta["smithy_refs"]
      if !smithy_refs.is_a?(Array) || smithy_refs.empty?
        add_error(errors, file, "status=absorbed-in-sdk requires non-empty smithy_refs array")
      end
    when "confirmed-not-api-resource"
      section = meta.dig("bc3_refs", "bc3_plan_section")
      unless section.is_a?(String) && !section.empty?
        add_error(errors, file, "status=confirmed-not-api-resource requires bc3_refs.bc3_plan_section")
      end
    when /\Aaddressed-in-bc3-pr-\d+\z/
      pr = meta["bc3_pr"]
      if pr.nil? || pr.to_s.empty?
        add_error(errors, file, "status=#{status} requires bc3_pr ref")
      end
    end
  end

  if (detected = meta["detected"]) && date_pattern && detected.to_s !~ Regexp.new(date_pattern)
    add_error(errors, file, "detected #{detected.inspect} does not match #{date_pattern}")
  end

  bc3_refs = meta["bc3_refs"]
  unless bc3_refs.is_a?(Hash)
    add_error(errors, file, "bc3_refs must be a mapping")
  end
end

def validate_body_sections(body, file, errors)
  REQUIRED_BODY_SECTIONS.each do |section|
    unless body.include?(section)
      add_error(errors, file, "missing required section heading: #{section}")
    end
  end
end

# Find all api-gap markdown files (excluding README and archive).
gap_files = Dir.glob(File.join(GAPS_DIR, "*.md")).reject do |path|
  basename = File.basename(path)
  basename == "README.md"
end

if gap_files.empty?
  warn "WARN: no api-gap entries found in #{GAPS_DIR}"
end

gap_files.each do |path|
  expected_gap = File.basename(path, ".md")
  meta, body, err = parse_frontmatter(path)
  if err
    add_error(errors, path, err)
    next
  end
  validate_frontmatter(meta, path, required_fields, known_fields, additional_strict, status_enum, status_pattern, date_pattern, errors, expected_gap)
  validate_body_sections(body || "", path, errors)
end

# --- Allowlist validator ------------------------------------------------------

if File.file?(ALLOWLIST_FILE)
  unless File.file?(ALLOWLIST_SCHEMA_FILE)
    add_error(errors, ALLOWLIST_FILE, "allowlist exists but allowlist-schema.json is missing at #{ALLOWLIST_SCHEMA_FILE}")
    # Skip allowlist content validation when the schema is missing.
    allowlist_schema = nil
  else
    allowlist_schema = JSON.parse(File.read(ALLOWLIST_SCHEMA_FILE))
  end
  allowed_keys = allowlist_schema && allowlist_schema["properties"]&.keys || []
  begin
    allowlist = YAML.safe_load(File.read(ALLOWLIST_FILE), permitted_classes: [Date, Time]) || {}
  rescue Psych::SyntaxError => e
    add_error(errors, ALLOWLIST_FILE, "YAML parse error: #{e.message}")
    allowlist = nil
  end

  if allowlist.is_a?(Hash) && allowlist_schema
    extra_keys = allowlist.keys - allowed_keys
    extra_keys.each do |k|
      add_error(errors, ALLOWLIST_FILE, "unknown top-level key: #{k}")
    end

    allowlist.each do |category, entries|
      next unless allowed_keys.include?(category)
      entry_schema = allowlist_schema.dig("properties", category, "items", "properties") || {}
      required_entry_fields = allowlist_schema.dig("properties", category, "items", "required") || []
      Array(entries).each_with_index do |entry, idx|
        unless entry.is_a?(Hash)
          add_error(errors, ALLOWLIST_FILE, "#{category}[#{idx}] must be a mapping")
          next
        end
        required_entry_fields.each do |field|
          unless entry.key?(field)
            add_error(errors, ALLOWLIST_FILE, "#{category}[#{idx}] missing required field: #{field}")
          end
        end
        if entry["classified"] && entry["classified"].to_s !~ /\A\d{4}-\d{2}-\d{2}\z/
          add_error(errors, ALLOWLIST_FILE, "#{category}[#{idx}] classified date format invalid: #{entry["classified"]}")
        end
        extra_fields = entry.keys - entry_schema.keys
        extra_fields.each do |k|
          add_error(errors, ALLOWLIST_FILE, "#{category}[#{idx}] unknown field: #{k}")
        end
      end
    end
  elsif allowlist
    add_error(errors, ALLOWLIST_FILE, "allowlist must be a mapping")
  end
end

# --- Cross-reference: absorbed_under_other_gap covers refer to existing entries

if File.file?(ALLOWLIST_FILE)
  begin
    allowlist = YAML.safe_load(File.read(ALLOWLIST_FILE), permitted_classes: [Date, Time]) || {}
    Array(allowlist["absorbed_under_other_brief"]).each_with_index do |entry, idx|
      next unless entry.is_a?(Hash)
      cover = entry["covers_under"]
      next if cover.nil? || cover.empty?
      gap_path = File.join(GAPS_DIR, "#{cover}.md")
      unless File.file?(gap_path)
        add_error(errors, ALLOWLIST_FILE, "absorbed_under_other_brief[#{idx}].covers_under references missing entry: #{cover}.md")
      end
    end
  rescue Psych::SyntaxError
    # Already reported.
  end
end

# --- Report ------------------------------------------------------------------

if errors.empty?
  puts "==> Validated #{gap_files.length} api-gap entr#{gap_files.length == 1 ? 'y' : 'ies'} and allowlist — clean"
  exit 0
else
  warn "==> API gap validation failed:"
  errors.each { |e| warn "  #{e}" }
  exit 1
end
