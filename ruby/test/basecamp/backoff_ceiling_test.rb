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

  # Bases spanning the range Config accepts. 1e-30 is the case a fixed exponent
  # cap of 64 gets wrong; the denormal-adjacent ones prove the derived bound
  # does not simply move the plateau further out.
  TINY_BASE_DELAYS = [ 1e-3, 1e-9, 1e-30, 1e-100, 1e-300, 5e-324 ].freeze

  # A fixed exponent cap plus a trailing +min(..., MAX_BACKOFF_DELAY)+ bounds the
  # intermediate but not the outcome. With a cap of 64 and +base_delay = 1e-30+,
  # attempt 65 and every attempt after it returned ~1.84e-11s forever: a tight
  # retry loop against a server already answering 429/503, which is precisely
  # what SPEC §7 requirement 1 forbids.
  #
  # +validate!+ has no +base_delay+ check at all, so every one of these is a
  # configuration the constructor accepts — the bound has to come from the base.
  def test_saturating_backoff_reaches_the_ceiling_for_any_positive_base
    violations = TINY_BASE_DELAYS.flat_map do |base_delay|
      Basecamp::Config.new(base_delay: base_delay)

      [ 1_100, 5_000, 2**31 ].filter_map do |attempt|
        delay = Basecamp::Config.saturating_backoff(base_delay, attempt)
        "base_delay=#{base_delay} attempt=#{attempt} -> #{delay}s" unless delay == Basecamp::Config::MAX_BACKOFF_DELAY
      end
    end

    assert_empty violations, "the term must saturate at the ceiling, not plateau below it"
  end

  # It grows to the ceiling rather than jumping — SPEC §7's "and then stops".
  # Sampling every attempt from 1 to 1100 covers the whole exponent range any
  # positive Float can need, so a formula that stalled anywhere in the middle
  # (the fixed-cap plateau) is caught wherever it stalls.
  def test_saturating_backoff_is_monotonic_up_to_the_ceiling
    ceiling = Basecamp::Config::MAX_BACKOFF_DELAY

    TINY_BASE_DELAYS.each do |base_delay|
      previous = 0.0
      (1..1_100).each do |attempt|
        delay = Basecamp::Config.saturating_backoff(base_delay, attempt)

        assert_operator delay, :>=, previous, "base_delay=#{base_delay} went backwards at attempt #{attempt}"
        assert_operator delay, :<=, ceiling, "base_delay=#{base_delay} exceeded the ceiling at attempt #{attempt}"
        previous = delay
      end

      assert_equal ceiling, previous, "base_delay=#{base_delay} never reached the ceiling"
    end
  end

  # The last attempts before the ceiling are the specified term, not the ceiling.
  #
  # Monotonicity and eventual saturation both hold for a formula that saturates
  # EARLY, so neither catches this. +MAX_BACKOFF_DELAY / 1e-307+ coerces to
  # Float::INFINITY, and the fixed-1023 fallback that used to backstop it
  # returned 30.0 for attempt 1024 when the specified term is ~8.99s — a sleep
  # more than three times longer than the formula asks for, with the numeric
  # backstop rather than the ceiling deciding it.
  def test_saturating_backoff_tracks_the_term_for_a_denormal_adjacent_base
    base_delay = 1e-307

    # Exact: these are the products the exponential term is defined to produce.
    assert_equal 8.988465674311579, Basecamp::Config.saturating_backoff(base_delay, 1_024)
    assert_equal 17.976931348623157, Basecamp::Config.saturating_backoff(base_delay, 1_025)
    # And only then does it reach the ceiling.
    assert_equal Basecamp::Config::MAX_BACKOFF_DELAY, Basecamp::Config.saturating_backoff(base_delay, 1_026)
    assert_equal Basecamp::Config::MAX_BACKOFF_DELAY, Basecamp::Config.saturating_backoff(base_delay, 2**31)
  end
end
