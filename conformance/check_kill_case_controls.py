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

    for every case declaring errorRaised, some case in the same file that does
    NOT declare it must have a mock response body with the identical key set,
    differing in exactly one field.

That one differing field is the field under test. Enforcing the claim rather
than asserting it in a comment, per the lesson from #576 itself: a guarantee
nobody checks is a guarantee that quietly stops holding.

Run: `python3 conformance/check_kill_case_controls.py` (wired into
`make conformance-fixtures-check`).
"""
from __future__ import annotations

import glob
import json
import os
import sys

ASSERTION = "errorRaised"


def bodies(case: dict) -> list[dict]:
    """Every object-shaped mock response body in a case."""
    return [
        r["body"]
        for r in case.get("mockResponses", [])
        if isinstance(r.get("body"), dict)
    ]


def declares_error_raised(case: dict) -> bool:
    return any(a.get("type") == ASSERTION for a in case.get("assertions", []))


def check_file(path: str) -> list[str]:
    with open(path) as f:
        cases = json.load(f)

    failures: list[str] = []
    controls = [c for c in cases if not declares_error_raised(c)]

    for kill in (c for c in cases if declares_error_raised(c)):
        kill_bodies = bodies(kill)
        if not kill_bodies:
            failures.append(
                f"{os.path.basename(path)}: {kill['name']!r} declares {ASSERTION} "
                f"but has no object-shaped mock response body to control against"
            )
            continue

        matched = None
        for kb in kill_bodies:
            for control in controls:
                for cb in bodies(control):
                    if set(cb) != set(kb):
                        continue
                    differing = sorted(k for k in kb if kb[k] != cb[k])
                    if len(differing) == 1:
                        matched = (control["name"], differing[0])
                        break
                if matched:
                    break
            if matched:
                break

        if matched:
            print(
                f"  ok  {kill['name'][:64]!r}\n"
                f"      field {matched[1]!r} controlled by {matched[0][:56]!r}"
            )
        else:
            failures.append(
                f"{os.path.basename(path)}: {kill['name']!r} declares {ASSERTION} but no "
                f"non-{ASSERTION} case in this file has a mock body with the same key set "
                f"differing in exactly one field.\n"
                f"      Without one, a decode failure caused by unrelated model drift would "
                f"satisfy this case and it would stop testing the field it names.\n"
                f"      Add (or repair) the control case so the two bodies differ only in "
                f"the field under test."
            )

    return failures


def main() -> int:
    root = os.path.dirname(os.path.abspath(__file__))
    paths = sorted(glob.glob(os.path.join(root, "tests", "*.json")))
    if not paths:
        print("no fixtures found", file=sys.stderr)
        return 1

    failures: list[str] = []
    for path in paths:
        found = check_file(path)
        failures.extend(found)

    if failures:
        print("\nFAIL: errorRaised fixtures without a control sibling:\n", file=sys.stderr)
        for f in failures:
            print(f"  - {f}", file=sys.stderr)
        return 1

    print("\nAll errorRaised fixtures have a body-pinning control sibling.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
