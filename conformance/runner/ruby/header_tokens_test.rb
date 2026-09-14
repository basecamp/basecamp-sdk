# frozen_string_literal: true

# The `{{httpdate+Ns}}` header token (SPEC §19, conformance/schema.json).
#
# A static fixture has no clock, so the positive half of SPEC §6's HTTP-date
# branch was unpinnable until this token (#780). These cases pin the resolver's
# arithmetic against a frozen instant so the fixture's one-sided timing floor
# rests on a deterministic contract. Ruby is the runner that had to move its
# header merge into the serve block for the token to see the right `now`.
#
# Run: `bundle exec ruby header_tokens_test.rb`

require "minitest/autorun"
require_relative "runner"

class HeaderTokensTest < Minitest::Test
  # A quarter-second into 10:18:14 UTC, so floor and round-up differ.
  NOW = Time.at(1_623_233_894.25).utc

  def test_plain_values_pass_through
    [ "", "2", "Wed, 09 Jun 2021 10:18:14 GMT", "application/json", "{not a token}" ].each do |value|
      assert_equal value, HeaderTokens.resolve(value, NOW)
    end
  end

  def test_httpdate_resolves_to_the_whole_second_past_n
    assert_equal "Wed, 09 Jun 2021 10:18:17 GMT", HeaderTokens.resolve("{{httpdate+2s}}", NOW)
    assert_equal "Wed, 09 Jun 2021 10:18:15 GMT", HeaderTokens.resolve("{{httpdate+0s}}", NOW)
    assert_equal "Wed, 09 Jun 2021 10:18:25 GMT", HeaderTokens.resolve("{{httpdate+10s}}", NOW)
  end

  def test_unknown_tokens_are_errors_not_literals
    [ "{{httpdate}}", "{{httpdate+2}}", "{{httpdate-2s}}", "{{now}}", "{{}}", "{{httpdate+1000000000s}}" ].each do |value|
      error = assert_raises(ArgumentError) { HeaderTokens.resolve(value, NOW) }
      assert_includes error.message, value
    end
  end
end
