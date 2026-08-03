# frozen_string_literal: true

# Both directions of the errorRaised assertion contract.
#
# Only the passing direction ever runs against a committed fixture: every case
# declaring errorRaised is one the SDK does refuse. A handler that accepted
# everything would therefore look green in all six runners at once, which is
# exactly how #563 shipped a vacuous delayBetweenRequests check.
#
# Run: `bundle exec ruby error_raised_test.rb`

require "minitest/autorun"
require_relative "runner"

class ErrorRaisedTest < Minitest::Test
  # Asserted verbatim here and in the five sibling runners. A fixture debugged
  # in one language should not read differently in another.
  MESSAGE = "Expected the call to fail, but it succeeded"

  def test_a_failed_dispatch_satisfies_the_assertion
    assert_nil ErrorRaised.check(true)
  end

  def test_a_successful_dispatch_fails_the_assertion
    # The branch under test. It is unreachable from conformance/tests/, so
    # without this case the handler could accept everything undetected.
    assert_equal MESSAGE, ErrorRaised.check(false)
  end
end
