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

# DECODERS must cover exactly the live fixture's operations.
#
# ReplayRunner#coverage_gate asserts the same thing, but it only runs during a
# live canary — and the scheduled canary skips whenever its secrets are
# unconfigured. That is how DECODERS sat twenty operations behind the fixture
# with CI fully green (#553). Asserting it here means the drift fails an
# ordinary run; scripts/check-replay-decoder-parity makes the same comparison
# statically across all five dispatch tables.
class DecoderCoverageTest < Minitest::Test
  LIVE_FIXTURE = File.expand_path("../../tests/live-my-surface.json", __dir__)

  # UTF-8 regardless of process locale (LC_ALL=C would otherwise read as US-ASCII)
  def live_operations
    ops = JSON.parse(File.read(LIVE_FIXTURE, encoding: "UTF-8"))
      .select { |t| t["mode"] == "live" }
      .map { |t| t["operation"] }
      .uniq
    # Fail closed: an empty set would make the assertions below vacuous.
    refute_empty ops, "#{LIVE_FIXTURE} declared no live operations — the reader is broken"
    ops
  end

  def test_every_live_operation_has_a_decoder
    assert_equal [], (live_operations - ReplayRunner::DECODERS.keys).sort,
                 "live operations with no entry in DECODERS"
  end

  def test_no_decoder_for_an_unknown_operation
    assert_equal [], (ReplayRunner::DECODERS.keys - live_operations).sort,
                 "DECODERS entries that are not live fixture operations"
  end

  def test_every_decoder_runs_the_sdk_decode_boundary
    # The Ruby SDK has no typed deserializers, so every entry must be the
    # shared SDK_DECODE lambda. A stub that swallowed errors would satisfy
    # coverage while decoding nothing.
    ReplayRunner::DECODERS.each do |op, decoder|
      assert_same ReplayRunner::SDK_DECODE, decoder, "#{op} is not wired to SDK_DECODE"
    end
  end
end
