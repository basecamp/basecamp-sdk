# frozen_string_literal: true

require "test_helper"

# #537 was an EXACT-STRING match on x-go-type: a pointer-spelled `*time.Time`
# skipped Time coercion and the field decoded as a raw String. The fix is
# `timestamp_go_type?` normalizing the leading star.
#
# The generated-types tests assert the current output, which is built from the
# checked-in spec where those fields are spelled bare — so they would still pass
# with the star-stripping removed. This exercises the predicate directly, which
# is the only thing that actually pins the fix.
class GenerateTypesSpellingTest < Minitest::Test
  # Requires only the shared module, not the whole generator: `load`ing the
  # script defined every one of its helpers (SKIP_PATTERNS, header,
  # generate_helpers, ...) on the test process.
  require_relative "../../scripts/go_type_spellings"

  def timestamp_go_type?(spelling) = GoTypeSpellings.timestamp_go_type?(spelling)

  def test_pointer_spelled_timestamp_is_recognized
    assert timestamp_go_type?("*time.Time"), "*time.Time must coerce (the #537 regression)"
  end

  def test_bare_timestamp_is_recognized
    assert timestamp_go_type?("time.Time")
  end

  def test_non_timestamp_go_types_are_not_coerced
    assert_not timestamp_go_type?("types.FlexibleTime"), "FlexibleTime also accepts date-only; coercing it would be a behavior change"
    assert_not timestamp_go_type?("*types.FlexibleTime")
    assert_not timestamp_go_type?("types.Date")
    assert_not timestamp_go_type?("string")
    assert_not timestamp_go_type?(nil)
  end
end
