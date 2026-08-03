# frozen_string_literal: true

require "test_helper"

# SPEC §7 "Backoff Ceiling" (#577).
#
# Ruby Integers are unbounded, so +base_delay * (2**(attempt - 1))+ does not wrap
# the way the compiled SDKs do — it promotes. Coerced against a Float base delay
# the product becomes +Float::INFINITY+, and +sleep(Float::INFINITY)+ never
# returns: the retry loop stops being a retry loop. Before it gets that far the
# delays are already measured in geological time.
class BackoffCeilingTest < Minitest::Test
  include TestHelper

  # With the 1.0s default base, attempt 6 is the first whose unclamped term
  # (32s) exceeds the 30s ceiling, so every attempt from there on must sit at
  # the cap.
  SATURATED_ATTEMPTS = [ 6, 10, 64, 128, 1024, 10_000, 2**31 ].freeze

  def setup
    @config = Basecamp::Config.new
    @http = Basecamp::Http.new(config: @config, token_provider: test_token_provider)
  end

  # The bound is two-sided on purpose: once the exponential term passes the
  # ceiling the delay must SIT at the ceiling, so a formula that collapsed to
  # zero would fail here too rather than sneaking past a one-sided check.
  def test_calculate_delay_saturates_at_the_ceiling
    ceiling = Basecamp::Config::MAX_BACKOFF_DELAY
    violations = SATURATED_ATTEMPTS.filter_map do |attempt|
      delay = @http.send(:calculate_delay, attempt, nil)
      "attempt #{attempt} -> #{delay}s" unless delay.between?(ceiling, ceiling + @config.max_jitter)
    end

    assert_empty violations,
      "every backoff must land within #{ceiling}..#{ceiling + @config.max_jitter}s"
  end

  def test_delays_below_the_ceiling_are_unchanged
    { 1 => 1.0, 2 => 2.0, 3 => 4.0 }.each do |attempt, want|
      delay = @http.send(:calculate_delay, attempt, nil)

      assert_operator delay, :>=, want
      assert_operator delay, :<=, want + @config.max_jitter
    end
  end

  # SPEC §7 requirement 4: the ceiling governs the locally-computed formula, not
  # the server-directed Retry-After.
  def test_retry_after_is_exempt_from_the_ceiling
    assert_equal 120, @http.send(:calculate_delay, 1, 120)
  end

  def test_saturating_backoff_edges
    # SPEC §7 requirement 3: the ceiling binds base_delay itself.
    assert_equal Basecamp::Config::MAX_BACKOFF_DELAY, Basecamp::Config.saturating_backoff(600.0, 1)
    # A zero base delay stays at zero rather than saturating.
    assert_in_delta 0.0, Basecamp::Config.saturating_backoff(0.0, 10_000)
    # Below the ceiling the exponential term is exact.
    assert_in_delta 4.0, Basecamp::Config.saturating_backoff(1.0, 3)
  end
end
