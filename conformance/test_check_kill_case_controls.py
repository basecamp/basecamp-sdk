#!/usr/bin/env python3
"""Negative-case self-test for conformance/check_kill_case_controls.py.

The gate's own `make conformance-fixtures-check` run only ever exercises the
VALID fixture set, which proves it can say yes and nothing else. Every rejection
this gate claims to make was, until this file existed, correct by inspection
alone — and the gate exists precisely because "correct by inspection" is what
let #576 through five review passes.

So each case here crafts one input the gate claims to reject and asserts that it
does: non-zero exit plus the expected message fragment, driven through the REAL
entry point via its FIXTURE_DIR argument. The real fixture set is run too, as a
positive control, so a gate that had regressed into rejecting everything would
fail here rather than look thorough.

The pairs are handcrafted rather than lifted from conformance/tests, so this file
stays readable and does not rot the day a fixture body gains a field.

Run directly (`python3 conformance/test_check_kill_case_controls.py`) or via
`make conformance-fixtures-check`, which runs it after the live check.
Stdlib only: the Makefile invokes it with a bare `python3`.
"""
from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
GATE = os.path.join(HERE, "check_kill_case_controls.py")
REAL_FIXTURES = os.path.join(HERE, "tests")

failures: list[str] = []


# --- the crafted pair ----------------------------------------------------------
#
# One control and one kill, differing in exactly `description` — the smallest
# input the gate accepts. Every case below breaks exactly one thing about it, so
# a failure names the property under test rather than an incidental defect.

GOOD_DESCRIPTION = "<p>From the store</p>"
MALFORMED_DESCRIPTION = ["x"]


def body(description: object = GOOD_DESCRIPTION, **overrides: object) -> dict:
    b: dict = {
        "id": 456,
        "title": "Buy milk",
        "description": description,
        "due_on": "2024-03-01",
    }
    b.update(overrides)
    return b


def ok_response(b: dict, status: int = 200) -> dict:
    return {"status": status, "headers": {"Content-Type": "application/json"}, "body": b}


def control_case(**overrides: object) -> dict:
    case: dict = {
        "name": "update-merge: content-only update preserves every unset field",
        "operation": "UpdateTodo",
        "method": "PUT",
        "path": "/todos/{todoId}",
        # Two queued responses on both sides, matching the real fixtures: the
        # GET under test and a decoy the case must never reach.
        "mockResponses": [ok_response(body()), ok_response(body())],
        "assertions": [{"type": "requestCount", "expected": 2}],
    }
    case.update(overrides)
    return case


def kill_case(**overrides: object) -> dict:
    case: dict = {
        "name": "update-kill: an array description is refused before the full-replace PUT",
        "operation": "UpdateTodo",
        "method": "PUT",
        "path": "/todos/{todoId}",
        "mockResponses": [
            ok_response(body(description=MALFORMED_DESCRIPTION)),
            ok_response(body(description=MALFORMED_DESCRIPTION)),
        ],
        "assertions": [{"type": "errorRaised"}, {"type": "requestCount", "expected": 1}],
    }
    case.update(overrides)
    return case


def pair(kill: dict | None = None, control: dict | None = None) -> list[dict]:
    return [control if control is not None else control_case(),
            kill if kill is not None else kill_case()]


# --- harness -------------------------------------------------------------------


def run(cases: list[dict] | None, *, args: list[str] | None = None,
        empty_dir: bool = False) -> tuple[str, int]:
    """Run the gate over a throwaway fixture directory. Returns (output, exit)."""
    with tempfile.TemporaryDirectory(prefix="kill-case-controls-test") as tmp:
        if not empty_dir:
            with open(os.path.join(tmp, "todos_write.json"), "w") as f:
                json.dump(cases, f, indent=2)
        argv = args if args is not None else [tmp]
        proc = subprocess.run(
            [sys.executable, GATE, *argv],
            capture_output=True, text=True,
        )
        return proc.stdout + proc.stderr, proc.returncode


def expect_pass(label: str, out: str, code: int) -> None:
    if code != 0:
        failures.append(f"{label}: expected PASS, gate exited {code}:\n{out}")


def expect_fail(label: str, out: str, code: int, fragment: str) -> None:
    if code == 0:
        failures.append(f"{label}: expected FAILURE, gate exited 0:\n{out}")
    elif fragment not in out:
        failures.append(f"{label}: failed as expected but message missing {fragment!r}:\n{out}")


# --- positive controls ---------------------------------------------------------

out, code = run(None, args=[REAL_FIXTURES])
expect_pass("the real fixture set passes", out, code)

out, code = run(pair())
expect_pass("a crafted valid pair passes", out, code)

# The gate must also work with no argument at all, since that is how the Makefile
# calls it — an argv change that broke the default would otherwise go unnoticed.
out, code = run(None, args=[])
expect_pass("the default fixture directory is conformance/tests", out, code)

# ...and the control really is what carries the pair: delete it and the same kill
# case must be rejected. Without this, every "valid pair" case above could be
# passing for a reason unrelated to the control.
out, code = run([kill_case()])
expect_fail("a kill case with no control at all", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")


# --- the kill response must reach a decoder ------------------------------------

# THE #204 CASE. Only the kill's first response changes, 200 -> 204; the control
# stays 200 and the one-field body comparison still holds. Before this was
# checked, the gate printed "ok" and exited 0 for exactly this input, certifying
# a case that cannot kill anything: TypeScript returns undefined for a 204,
# Kotlin returns Unit without parsing, Go rewrites the body to JSON null — so the
# malformed description is never decoded, the composite fails because the record
# came back absent, and errorRaised + requestCount: 1 are satisfied by that.
out, code = run(pair(kill=kill_case(mockResponses=[
    ok_response(body(description=MALFORMED_DESCRIPTION), status=204),
    ok_response(body(description=MALFORMED_DESCRIPTION)),
])))
expect_fail("kill case answers 204", out, code,
            "has status 204, which carries no body by definition")

# 205 forbids a body for the same reason and must not need its own discovery.
out, code = run(pair(kill=kill_case(mockResponses=[
    ok_response(body(description=MALFORMED_DESCRIPTION), status=205),
    ok_response(body(description=MALFORMED_DESCRIPTION)),
])))
expect_fail("kill case answers 205", out, code,
            "has status 205, which carries no body by definition")

# The allowlist is what makes this closed-by-default: 202 is a 2xx nobody
# excluded by name, and Go's success arm does not cover it, so its body is never
# decoded there. A 204-only exclusion would wave this through.
out, code = run(pair(kill=kill_case(mockResponses=[
    ok_response(body(description=MALFORMED_DESCRIPTION), status=202),
    ok_response(body(description=MALFORMED_DESCRIPTION)),
])))
expect_fail("kill case answers an undecoded 2xx", out, code,
            "has status 202, which is not one of the statuses whose body every SDK decodes")

# 201 IS decoded, so the allowlist must not be a 200-only rule wearing a larger
# name — otherwise the next fixture to use a create response fails spuriously.
out, code = run(pair(
    control=control_case(mockResponses=[ok_response(body(), status=201), ok_response(body())]),
    kill=kill_case(mockResponses=[
        ok_response(body(description=MALFORMED_DESCRIPTION), status=201),
        ok_response(body(description=MALFORMED_DESCRIPTION)),
    ]),
))
expect_pass("201 is a decoded status", out, code)

out, code = run(pair(kill=kill_case(mockResponses=[
    ok_response(body(description=MALFORMED_DESCRIPTION), status=500),
    ok_response(body(description=MALFORMED_DESCRIPTION)),
])))
expect_fail("kill case answers an HTTP error", out, code,
            "has status 500, so the call fails on the HTTP error before the body is decoded")

# `body` alongside `networkError` is schema-VALID (conformance/schema.json only
# makes status and networkError mutually exclusive), so flipping a status to
# networkError while retaining the body sails through the schema pass. This gate
# is the only thing that catches it, which is why the case keeps its body.
out, code = run(pair(kill=kill_case(mockResponses=[
    {"networkError": True, "body": body(description=MALFORMED_DESCRIPTION)},
    ok_response(body(description=MALFORMED_DESCRIPTION)),
])))
expect_fail("kill case dies in transport", out, code,
            "declares networkError, so the call fails in transport")

# Dropping the body too is caught earlier, by the object-shape branch.
out, code = run(pair(kill=kill_case(mockResponses=[
    {"networkError": True},
    ok_response(body(description=MALFORMED_DESCRIPTION)),
])))
expect_fail("kill case dies in transport with no body", out, code,
            "has no object-shaped body to control against")

out, code = run(pair(kill=kill_case(mockResponses=[
    {"status": "200", "body": body(description=MALFORMED_DESCRIPTION)},
    ok_response(body(description=MALFORMED_DESCRIPTION)),
])))
expect_fail("kill case has a stringly-typed status", out, code,
            "has a non-integer status '200'")

out, code = run(pair(kill=kill_case(mockResponses=[])))
expect_fail("kill case queues no response", out, code,
            "has no object-shaped body to control against")

out, code = run(pair(kill=kill_case(mockResponses=[{"status": 200, "body": "not an object"}])))
expect_fail("kill case body is not an object", out, code,
            "has no object-shaped body to control against")


# --- the control response must reach a decoder too -----------------------------
#
# The symmetric half. A control that is never decoded cannot fail loudly (#555)
# when the model drifts, which is the entire protection the kill case borrows
# from it.

out, code = run(pair(control=control_case(mockResponses=[
    ok_response(body(), status=204),
    ok_response(body()),
])))
expect_fail("control answers 204", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")

out, code = run(pair(control=control_case(mockResponses=[
    ok_response(body(), status=500),
    ok_response(body()),
])))
expect_fail("control answers an HTTP error", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")

out, code = run(pair(control=control_case(mockResponses=[
    {"networkError": True, "body": body()},
    ok_response(body()),
])))
expect_fail("control dies in transport", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")


# --- the control must be the right control -------------------------------------

out, code = run(pair(control=control_case(operation="UpdateCard")))
expect_fail("control exercises a different operation", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")

out, code = run(pair(control=control_case(mockResponses=[
    ok_response(body(title="Buy oat milk")),
    ok_response(body()),
])))
expect_fail("control differs in two fields", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")

out, code = run(pair(control=control_case(mockResponses=[
    ok_response({"id": 456, "title": "Buy milk", "description": GOOD_DESCRIPTION}),
    ok_response(body()),
])))
expect_fail("control body has a different key set", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")

# A control whose bodies are IDENTICAL to the kill's pins nothing: there is no
# field under test, so the gate cannot name what the kill case kills.
out, code = run(pair(control=control_case(mockResponses=[
    ok_response(body(description=MALFORMED_DESCRIPTION)),
    ok_response(body()),
])))
expect_fail("control body is identical to the kill body", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")


# --- only the FIRST response counts, on both sides -----------------------------
#
# The second queued response is a decoy that is never consumed. Matching against
# it would let the body that actually gets decoded drift away from its control —
# the hole this gate exists to close, one level up.

out, code = run(pair(control=control_case(mockResponses=[
    ok_response(body(title="Buy oat milk")),   # first: differs in TWO fields
    ok_response(body()),                       # second: would match, but is a decoy
])))
expect_fail("a control's decoy response cannot satisfy the gate", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")

out, code = run(pair(kill=kill_case(mockResponses=[
    ok_response(body(description=MALFORMED_DESCRIPTION, title="Buy oat milk")),  # decoded: 2 fields off
    ok_response(body(description=MALFORMED_DESCRIPTION)),                        # decoy: would match
])))
expect_fail("a kill's decoy response cannot satisfy the gate", out, code,
            "no non-errorRaised case for operation 'UpdateTodo'")


# --- entry point ---------------------------------------------------------------

out, code = run(None, empty_dir=True)
expect_fail("an empty fixture directory is not a pass", out, code, "no fixtures found")

out, code = run(None, args=["a", "b"])
expect_fail("more than one argument is a usage error", out, code, "usage:")
if code != 2:
    failures.append(f"usage error must exit 2, got {code}:\n{out}")


# --- report --------------------------------------------------------------------

if failures:
    print(f"check_kill_case_controls self-test: {len(failures)} case(s) failed.", file=sys.stderr)
    for f in failures:
        print(f"\n{f}", file=sys.stderr)
    sys.exit(1)

print("check_kill_case_controls self-test: all cases passed.")
