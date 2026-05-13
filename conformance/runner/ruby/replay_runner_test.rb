# frozen_string_literal: true

# Regression test for the empty-bodyText decode-masking bug.
#
# Pre-fix, SDK_DECODE had `return nil if body_text.nil? || body_text.empty?`,
# which silently green-passed an empty body — diverging from the production
# Ruby SDK (Basecamp::Http#json calls JSON.parse(@body) without an empty-body
# guard, so an empty body raises JSON::ParserError). Post-fix, an empty
# bodyText flows into JSON.parse and surfaces as a decode_error.
#
# Run: `bundle exec ruby replay_runner_test.rb`

require "json"
require "minitest/autorun"
require_relative "replay-runner"

class SdkDecodeTest < Minitest::Test
  def test_empty_body_raises_parser_error
    assert_raises(JSON::ParserError) { ReplayRunner::SDK_DECODE.call("") }
  end

  def test_well_formed_body_decodes_cleanly
    result = ReplayRunner::SDK_DECODE.call(%({"a":1}))
    assert_equal({ "a" => 1 }, result)
  end

  def test_malformed_body_raises_parser_error
    assert_raises(JSON::ParserError) { ReplayRunner::SDK_DECODE.call("not json") }
  end
end
