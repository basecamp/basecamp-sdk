#!/usr/bin/env python3
"""Every `errorRaised` fixture must have a control sibling that pins its body.

Declaring `errorRaised` switches OFF the #555 stop-on-mismatch policy in the
decoder-backed runners: Swift's `DecodingError` branch and Kotlin's
`MissingFieldException`/`SerializationException` branches normally fail loudly
on a mock body that no longer decodes into the generated model, but when the
fixture declares `errorRaised` they treat the refusal as the behaviour under
test and pass.

That is correct — a decoder rejecting a deliberately malformed field is exactly
how Go, Kotlin and Swift satisfy the #576 kill cases — but on its own it is
also a way for a kill case to keep passing for the wrong reason. If the model
later gains a required field, or any unrelated field in one of these large mock
bodies drifts, the decode fails for a reason that has nothing to do with the
field under test. `errorRaised`, `requestCount: 1`, `requestMethod` and
`requestPath` all still hold, and the case silently stops proving anything.

The protection is structural rather than declared: each kill body is some
passing case's body with exactly one field perturbed, and that passing sibling
does NOT declare `errorRaised`, so it keeps the full #555 policy. Model drift
therefore still fails loudly — in the sibling. This gate is what keeps that
true, because nothing else stops the two bodies from drifting apart:

    for every case declaring errorRaised, a case in the same file that does NOT
    declare it, exercising the SAME operation, must have a first mock response
    body with the identical key set, differing in exactly one field.

That one differing field is the field under test.

Three deliberate restrictions, all load-bearing — a looser version of this gate
can be satisfied by a body that is never decoded:

* **The first response only, on both sides.** These cases assert
  `requestCount: 1`, so response 0 is the GET whose decoder rejection is under
  test; the second queued response is a decoy, there so a runner cannot pass by
  exhausting the queue instead of refusing the field. It is never consumed.
  Matching a kill body against *any* queued response of *any* control would let
  an unconsumed decoy satisfy the gate while the body that actually gets
  decoded drifts away from its control — which is the exact hole this gate
  exists to close, one level up.
* **The same operation.** A body that decodes into a different model says
  nothing about whether this one still decodes.
* **A status whose body is actually decoded, on both sides.** Body equality is
  blind to the response *outcome*, and several outcomes hand the body to
  nobody: a transport failure, an HTTP error, and — less obviously — a 204,
  which the SDKs short-circuit before any parse. See `DECODED_STATUSES`.

Enforcing the claim rather than asserting it in a comment, per the lesson from
#576 itself: a guarantee nobody checks is a guarantee that quietly stops
holding.

Run: `python3 conformance/check_kill_case_controls.py [FIXTURE_DIR]` — defaulting
to `conformance/tests` — wired into `make conformance-fixtures-check`, which then
runs `conformance/test_check_kill_case_controls.py` to prove the gate can still
say no.
"""
from __future__ import annotations

import glob
import json
import os
import sys

ASSERTION = "errorRaised"

# The statuses whose response body actually reaches a generated model's decoder
# in every SDK the conformance runners drive. Two independent constraints meet
# here, and the intersection is {200, 201}:
#
# * **Go's success arm is exactly {200, 201, 204}** (`go/pkg/basecamp/client.go`,
#   `case http.StatusOK, http.StatusCreated, http.StatusNoContent`). 202, 203,
#   205 and 206 fall through to the error default, so a body sent under them is
#   never decoded there at all.
# * **204 is short-circuited before any parse**, everywhere it is recognised:
#   TypeScript returns `undefined` (`typescript/src/services/base.ts`), Kotlin
#   returns `Unit` without calling `parse` (`kotlin/.../BaseService.kt`), and Go
#   rewrites the body to JSON `null` (`client.go`). A 204 carries no body by
#   definition, and 205 makes the same promise in HTTP terms.
#
# So a kill case whose first response is a 204 is silently vacuous: the malformed
# field is never decoded, the composite fails because the record came back absent,
# and `errorRaised` + `requestCount: 1` + `requestMethod` + `requestPath` are all
# still satisfied. Meanwhile its control, still a 200, stays green — so body
# equality alone certifies the pair as valid while the kill case tests nothing.
#
# Written as an allowlist rather than a 204 exclusion, deliberately. This gate's
# entire job is to prove a body is decoded, and it cannot prove that for a status
# nobody has reasoned about; closed-by-default means the next such status is
# rejected with a message pointing here instead of quietly joining 204.
DECODED_STATUSES = frozenset({200, 201})

# Statuses that forbid a response body outright, called out separately only so
# the failure explains itself rather than reading as an arbitrary allowlist miss.
NO_BODY_STATUSES = frozenset({204, 205})


def consumed_response(case: dict) -> dict | None:
    """The first mock response — the one the case under test decodes."""
    responses = case.get("mockResponses", [])
    return responses[0] if responses else None


def consumed_body(case: dict) -> dict | None:
    """The first mock response body — the one the case under test decodes."""
    response = consumed_response(case)
    if response is None:
        return None
    body = response.get("body")
    return body if isinstance(body, dict) else None


def not_decoded(response: dict) -> str | None:
    """Why this response's body never reaches a decoder — or None if it does.

    A kill case's premise is that the malformed value arrived in a response the
    SDK ACCEPTED AND DECODED. That is what makes it the SDK's problem rather
    than the server's, and it is why #576 classifies the refusal as `api_error`
    with no status rather than as a transport or HTTP failure.

    Three ways a body goes undecoded, and all three leave the case green:

    * `networkError` — the call dies in transport.
    * a non-2xx status — the call fails on the HTTP error first.
    * a 2xx the SDKs do not decode a body from, of which 204 is the live one:
      it is short-circuited before any parse, so the composite fails merely
      because the record came back absent.

    In every case `errorRaised` still holds — along with `requestCount`,
    `requestMethod` and `requestPath` — while the malformed field was never
    decoded at all, and the case goes green having tested nothing. Body
    equality cannot see any of it, so it is checked here.
    """
    if response.get("networkError"):
        return "declares networkError, so the call fails in transport and the body is never decoded"
    status = response.get("status")
    if not isinstance(status, int):
        return f"has a non-integer status {status!r}"
    if not 200 <= status < 300:
        return f"has status {status}, so the call fails on the HTTP error before the body is decoded"
    if status in NO_BODY_STATUSES:
        return (
            f"has status {status}, which carries no body by definition and is "
            f"short-circuited before any parse — TypeScript returns undefined, Kotlin "
            f"returns Unit without parsing, Go rewrites the body to JSON null — so the "
            f"body is never decoded"
        )
    if status not in DECODED_STATUSES:
        return (
            f"has status {status}, which is not one of the statuses whose body every SDK "
            f"decodes ({', '.join(str(s) for s in sorted(DECODED_STATUSES))}); Go's "
            f"success arm alone excludes it, so the body is never decoded there"
        )
    return None


def declares_error_raised(case: dict) -> bool:
    return any(a.get("type") == ASSERTION for a in case.get("assertions", []))


def check_file(path: str) -> list[str]:
    with open(path) as f:
        cases = json.load(f)

    failures: list[str] = []
    name = os.path.basename(path)
    controls = [c for c in cases if not declares_error_raised(c)]

    for kill in (c for c in cases if declares_error_raised(c)):
        kill_response = consumed_response(kill)
        kill_body = consumed_body(kill)
        if kill_response is None or kill_body is None:
            failures.append(
                f"{name}: {kill['name']!r} declares {ASSERTION} but its first mock "
                f"response has no object-shaped body to control against"
            )
            continue

        # The body must actually reach a decoder for its shape to matter.
        reason = not_decoded(kill_response)
        if reason is not None:
            failures.append(
                f"{name}: {kill['name']!r} declares {ASSERTION}, but its first mock response "
                f"{reason}.\n"
                f"      {ASSERTION} would then be satisfied by that failure — with requestCount, "
                f"requestMethod and requestPath still green — and the malformed field would "
                f"never be decoded, so the case would test nothing.\n"
                f"      A kill case must deliver its malformed value in a response the SDKs "
                f"DECODE: status "
                f"{' or '.join(str(s) for s in sorted(DECODED_STATUSES))}."
            )
            continue

        operation = kill.get("operation")
        matched = None
        for control in controls:
            if control.get("operation") != operation:
                continue
            control_response = consumed_response(control)
            # The control earns its keep only by being DECODED: that is what
            # makes it fail loudly (#555) when the model drifts, which is the
            # entire protection the kill case borrows from it. A control that
            # answers 500, a 204, or networkError never reaches its decoder, so
            # it can sit green on its own HTTP/transport assertions while the
            # drift it was supposed to catch goes unnoticed in both bodies. Same
            # check as the kill side, for the same reason.
            if control_response is None or not_decoded(control_response) is not None:
                continue
            control_body = consumed_body(control)
            if control_body is None or set(control_body) != set(kill_body):
                continue
            differing = sorted(k for k in kill_body if kill_body[k] != control_body[k])
            if len(differing) == 1:
                matched = (control["name"], differing[0])
                break

        if matched:
            print(
                f"  ok  {kill['name'][:64]!r}\n"
                f"      field {matched[1]!r} controlled by {matched[0][:56]!r}"
            )
        else:
            failures.append(
                f"{name}: {kill['name']!r} declares {ASSERTION} but no non-{ASSERTION} "
                f"case for operation {operation!r} in this file has a DECODED (status "
                f"{' or '.join(str(s) for s in sorted(DECODED_STATUSES))}) FIRST mock "
                f"response whose body has the same key set differing in exactly one field.\n"
                f"      Without one, a decode failure caused by unrelated model drift would "
                f"satisfy this case and it would stop testing the field it names.\n"
                f"      Add (or repair) the control case so the two decoded bodies differ "
                f"only in the field under test."
            )

    return failures


def main(argv: list[str] | None = None) -> int:
    # The optional argument exists for the self-test, which has to be able to
    # make this gate FAIL — a gate only ever pointed at the valid repo proves
    # it can say yes and nothing else (the #576 lesson, applied to the gate).
    args = sys.argv[1:] if argv is None else argv
    if len(args) > 1:
        print(f"usage: {os.path.basename(__file__)} [FIXTURE_DIR]", file=sys.stderr)
        return 2
    root = args[0] if args else os.path.join(os.path.dirname(os.path.abspath(__file__)), "tests")
    paths = sorted(glob.glob(os.path.join(root, "*.json")))
    if not paths:
        print(f"no fixtures found in {root}", file=sys.stderr)
        return 1

    failures: list[str] = []
    for path in paths:
        failures.extend(check_file(path))

    if failures:
        print("\nFAIL: errorRaised fixtures without a control sibling:\n", file=sys.stderr)
        for f in failures:
            print(f"  - {f}", file=sys.stderr)
        return 1

    print("\nAll errorRaised fixtures have a body-pinning control sibling.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
