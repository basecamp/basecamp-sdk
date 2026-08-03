"""Both directions of the ``errorRaised`` assertion contract.

Run: `uv run pytest test_error_raised.py`

Only the passing direction ever runs against a committed fixture: every case
declaring errorRaised is one the SDK does refuse. A handler that accepted
everything would therefore look green in all six runners at once, which is
exactly how #563 shipped a vacuous delayBetweenRequests check.
"""
from __future__ import annotations

from runner import error_raised_failure

# Asserted verbatim here and in the five sibling runners. A fixture debugged in
# one language should not read differently in another.
MESSAGE = "Expected the call to fail, but it succeeded"


def test_a_failed_dispatch_satisfies_the_assertion():
    assert error_raised_failure(True) is None


def test_a_successful_dispatch_fails_the_assertion():
    # The branch under test. It is unreachable from conformance/tests/, so
    # without this case the handler could accept everything undetected.
    assert error_raised_failure(False) == MESSAGE
