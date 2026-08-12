#!/usr/bin/env python3
"""Pin the conformance search bodies to the shared search fixture.

`conformance/tests/search.json` inlines its mock bodies from
`spec/fixtures/search/results.json`, because the two systems have no `$ref`
mechanism between them: the conformance runners read a self-contained fixture
file and never resolve external references. That makes the bodies a COPY, and
`make conformance-fixtures-check` validates their format, not their fidelity —
a hit edited on one side and not the other is silently divergent.

Divergence is not hypothetical for these particular bytes. The shared fixture is
what `make check-fixture-coverage` validates against the generated `SearchResult`
schema, so it is the copy that moves when the spec gains a member; the
conformance copy has no such pull and would quietly keep testing the old shape.

The check is deliberately narrow: each mock body must be an ordered sublist of
the shared fixture's hits, compared as parsed JSON (so formatting is free to
differ). A case may carry FEWER hits than the shared fixture — the
file-attachment case presents one branch alone on purpose — but every hit it
does carry must be byte-for-byte the same object, in the same relative order.
"""

from __future__ import annotations

import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
SHARED = ROOT / "spec/fixtures/search/results.json"
CONFORMANCE = ROOT / "conformance/tests/search.json"


def is_ordered_sublist(needle: list, haystack: list) -> bool:
    it = iter(haystack)
    return all(any(item == candidate for candidate in it) for item in needle)


def main() -> int:
    print("==> Checking conformance search bodies against the shared fixture...")
    shared = json.loads(SHARED.read_text())
    cases = json.loads(CONFORMANCE.read_text())

    failures: list[str] = []
    checked = 0

    for case in cases:
        for i, response in enumerate(case.get("mockResponses", [])):
            body = response.get("body")
            checked += 1
            # Every Search mock body is an array of hits, so a non-array here is
            # already wrong. Skipping it instead would let a body edited into an
            # object slip past while `checked` stayed nonzero from a sibling
            # case — the vacuity guard below would not fire, and this gate would
            # report success having inspected nothing that changed.
            if not isinstance(body, list):
                failures.append(
                    f"  {case['name']!r} mockResponses[{i}]: body is "
                    f"{type(body).__name__}, expected an array of search hits"
                )
            elif not is_ordered_sublist(body, shared):
                failures.append(
                    f"  {case['name']!r} mockResponses[{i}]: "
                    f"{len(body)} hit(s) not an ordered sublist of "
                    f"{SHARED.relative_to(ROOT)} ({len(shared)} hits)"
                )

    if not checked:
        print(
            f"FAIL: no mock response found in "
            f"{CONFORMANCE.relative_to(ROOT)} — the gate would pass vacuously",
            file=sys.stderr,
        )
        return 1

    if failures:
        print(
            f"FAIL: conformance search bodies have drifted from "
            f"{SHARED.relative_to(ROOT)}:",
            file=sys.stderr,
        )
        for failure in failures:
            print(failure, file=sys.stderr)
        print(
            "\nRe-copy the hits from the shared fixture. It is the copy "
            "`make check-fixture-coverage` validates against the generated "
            "schema, so it is the one that is right.",
            file=sys.stderr,
        )
        return 1

    print(f"  ok: {checked} mock body/bodies match {SHARED.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
