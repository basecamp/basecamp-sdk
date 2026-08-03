# frozen_string_literal: true

# Bounds contract for the requestCount assertion (#573).
#
# Until this commit the five non-Swift runners evaluated requestCount as a LOWER
# bound whenever any mock response carried `Link: rel="next"`. Every committed
# fixture passes under both rules, so nothing in the suite could tell them
# apart — the same shape as the #563 delayBetweenRequests regression these
# support modules exist to pin. The over-fetch case below is the one that
# distinguishes them, and it is the case that matters: pagination.json's
# maxPages and maxItems fixtures each queue three pages and assert two requests,
# so a lower bound green-passes an SDK that ignored the cap.
#
# Run: `bundle exec ruby request_count_test.rb`

require "minitest/autorun"
require_relative "runner"

class RequestCountTest < Minitest::Test
  def test_exact_count_passes
    assert_nil RequestCount.check(2, 2)
  end

  def test_under_fetch_fails
    refute_nil RequestCount.check(1, 2)
  end

  def test_over_fetch_fails
    # The regression. Under the old lower bound this returned nil — an SDK that
    # walked all three queued pages instead of stopping at the maxPages cap
    # reported a clean pass.
    refute_nil RequestCount.check(3, 2)
  end

  def test_failure_message_names_both_counts
    assert_equal "Expected 2 requests, got 3", RequestCount.check(3, 2)
  end

  def test_zero_requests_is_not_a_free_pass
    # A test whose operation never reached the wire records zero requests. That
    # must fail an assertion expecting one, not read as "no data, no opinion".
    refute_nil RequestCount.check(0, 1)
  end

  def test_zero_expected_requires_zero_actual
    assert_nil RequestCount.check(0, 0)
    refute_nil RequestCount.check(1, 0)
  end

  # Applicability contract (#573). The `link-header` fixture's requestCount is
  # inapplicable to an auto-paginating SDK; its statusCode and noError
  # assertions are not. Suppressing the CASE instead of the ASSERTION left the
  # fixture executed by nothing at all — it stays in pagination.json and passes
  # conformance-fixtures-check and check-fixture-coverage either way, so
  # nothing else would have reported it.

  def test_request_count_does_not_apply_to_link_header_fixtures
    refute RequestCount.applies?(["pagination", "link-header"])
  end

  def test_request_count_applies_to_every_other_fixture
    [nil, [], ["pagination"], ["retry", "idempotent"]].each do |tags|
      assert RequestCount.applies?(tags), "requestCount must be asserted for #{tags.inspect}"
    end
  end

  # The suppression is one assertion wide, not one case wide.
  def test_link_header_suppression_is_scoped_to_the_count_assertion
    tags = ["pagination", "link-header"]
    assertions = [
      { "type" => "requestCount", "expected" => 1 },
      { "type" => "statusCode", "expected" => 200 },
      { "type" => "noError" }
    ]
    live = assertions.reject do |a|
      a["type"] == "requestCount" && !RequestCount.applies?(tags)
    end
    assert_equal ["statusCode", "noError"], live.map { |a| a["type"] }
  end
end
