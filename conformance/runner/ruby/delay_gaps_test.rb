# frozen_string_literal: true

# Bounds and every-gap contract for the delayBetweenRequests assertion.
#
# Each case names a behavior that regressed on #563/#568: an assertion that
# looked like it covered a timing gap and did not.
#
# Run: `bundle exec ruby delay_gaps_test.rb`

require "minitest/autorun"
require_relative "runner"

class DelayGapsTest < Minitest::Test
  def test_omitted_index_catches_a_later_failing_gap
    # Three runners measured gap 0 and stopped, so a second backoff that never
    # happened passed unnoticed.
    failure = DelayGaps.check([ 1000, 5 ], 500, nil)

    assert_includes failure, "at gap 1"
  end

  def test_omitted_index_passes_when_every_gap_clears_the_minimum
    assert_nil DelayGaps.check([ 1000, 2000, 800 ], 500, nil)
  end

  def test_omitted_index_fails_when_there_are_no_gaps_at_all
    # An "every gap" rule with no gaps left must not wave the run through: a
    # fully dropped retry lands exactly here.
    assert_equal "Expected a delay between requests, but only 1 request(s) were made",
      DelayGaps.check([], 500, nil)
  end

  def test_named_gap_fails_when_the_run_never_produced_it
    assert_equal "Expected a delay at gap 1, but only 2 request(s) were made",
      DelayGaps.check([ 1000 ], 500, 1)
  end

  def test_named_gap_fails_on_a_single_request_run
    assert_equal "Expected a delay at gap 0, but only 1 request(s) were made",
      DelayGaps.check([], 500, 0)
  end

  def test_negative_gap_index_is_rejected
    # Rejected categorically, not wrapped to the end the way headerPresent's
    # index is: there is no sensible "last gap" when the point is to name one.
    assert_equal "delayBetweenRequests gap index must be non-negative, got -1",
      DelayGaps.check([ 1000, 2000 ], 500, -1)
  end

  def test_enormous_gap_index_fails_rather_than_raising
    assert_includes DelayGaps.check([ 1000, 2000 ], 500, 2**63 - 1), "Expected a delay at gap"
  end

  def test_named_gap_passes_when_it_clears_the_minimum
    assert_nil DelayGaps.check([ 5, 2000 ], 500, 1)
  end

  def test_named_gap_fails_when_it_is_below_the_minimum
    assert_includes DelayGaps.check([ 2000, 5 ], 500, 1), "at gap 1"
  end

  def test_zero_minimum_still_asserts_that_the_gap_exists
    # A `min` of zero (or an absent one, which the caller defaults to zero)
    # still requires the gap to EXIST. Gating the call on the value's presence
    # would degrade the assertion to nothing.
    [ 0, nil ].each do |missing|
      assert_equal "Expected a delay between requests, but only 1 request(s) were made",
        DelayGaps.check([], missing, nil)
      assert_equal "Expected a delay at gap 0, but only 1 request(s) were made",
        DelayGaps.check([], missing, 0)
      assert_nil DelayGaps.check([ 5 ], missing, nil)
    end
  end
end
