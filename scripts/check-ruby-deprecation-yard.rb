# frozen_string_literal: true

# Assert, via the YARD registry, that the generated Ruby deprecation docs attach
# to the right objects. Run from the ruby/ directory (see check-deprecation-parity).
#
# A text grep cannot prove this: the models declare all accessors in one grouped
# `attr_accessor`, so the `@!attribute [rw] clientside` directive is the only
# thing that scopes the @deprecated tag to `clientside` rather than the whole
# class. This parses the file and checks the actual registry objects.

require 'yard'

TYPES = File.expand_path('lib/basecamp/generated/types.rb', Dir.pwd)

YARD::Registry.clear
YARD.parse(TYPES)

failures = []

# The resolved reason for both clientside sites (see the shared resolver in
# scripts/enhance-openapi-go-types.sh). Asserting the tag *text* — not just its
# presence — makes the Ruby check enforce the same reason guarantee as the
# other SDK checks.
EXPECTED_REASON = 'This shape is deprecated since 2024-01: Use Client Visibility feature instead'

def check_deprecated_reason(obj, name, failures)
  if obj.nil?
    failures << "#{name} not found in registry"
    return
  end
  tag = obj.tag(:deprecated)
  if tag.nil?
    failures << "#{name} is missing a @deprecated tag"
  elsif tag.text.to_s.strip != EXPECTED_REASON
    failures << "#{name} @deprecated reason mismatch: #{tag.text.inspect}"
  end
end

check_deprecated_reason(YARD::Registry.at('Basecamp::Types::ClientSide'), 'Basecamp::Types::ClientSide', failures)
check_deprecated_reason(YARD::Registry.at('Basecamp::Types::Project#clientside'), 'Basecamp::Types::Project#clientside', failures)

# Control: a sibling attribute on the same grouped accessor must NOT be
# deprecated — proves the @!attribute directive scoped the tag correctly.
sibling = YARD::Registry.at('Basecamp::Types::Project#name')
if sibling && !sibling.tag(:deprecated).nil?
  failures << 'control violated: Basecamp::Types::Project#name is unexpectedly @deprecated'
end

if failures.empty?
  puts '  Ruby YARD registry: clientside/ClientSide deprecation attached; controls clean'
  exit 0
else
  failures.each { |f| warn "    #{f}" }
  exit 1
end
