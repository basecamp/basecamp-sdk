#!/usr/bin/env python3
"""Verify the event-feed schema's load-bearing allOf pins via derived mutants.

Each probe file under the probes directory declares a `control` scenario and one
`mutation` (a path into the control and the value to plant there). The gate:

  1. validates the control against the schema — it MUST be accepted;
  2. applies the mutation in-process and validates the mutant — it MUST be rejected.

Because the mutant is derived from a control that just parsed and validated, the
accepted/rejected delta is exactly the mutation: there is no committed invalid
file to go stale, drift extra deltas, or fail as malformed JSON. The mutant's
rejection must additionally prove itself as an instance-validation failure via
the validator's structured JSON output — a tool, schema-parse, or reference
error on that invocation fails the gate instead of counting as the pin firing.
A mutation whose path is absent from the control, or whose value already equals
the control's, fails the gate as a vacuous probe.
"""

import copy
import json
import subprocess
import sys
import tempfile
from pathlib import Path


def fail(message):
    print(f"ERROR: {message}", file=sys.stderr)
    sys.exit(1)


def validate(schema, instance_path, checker_version):
    return subprocess.run(
        [
            "uvx",
            "--from",
            f"check-jsonschema=={checker_version}",
            "check-jsonschema",
            "--output-format",
            "json",
            "--schemafile",
            str(schema),
            str(instance_path),
        ],
        capture_output=True,
        text=True,
    )


def instance_validation_errors(result):
    """The mutant's rejection must be a genuine instance-validation failure.

    check-jsonschema exits nonzero for tool, schema-parse, and reference errors
    too, and those must fail the gate rather than count as the pin firing. Under
    --output-format json an instance rejection is the one outcome that emits
    parseable JSON with status "fail", at least one validation error, and no
    parse errors; everything else (non-JSON error text, parse_errors, empty
    errors) is an unexpected validator outcome.
    """
    if result.returncode == 0:
        return None
    try:
        report = json.loads(result.stdout)
    except json.JSONDecodeError:
        return None
    if report.get("status") != "fail" or report.get("parse_errors") or not report.get("errors"):
        return None
    return report["errors"]


def main():
    if len(sys.argv) != 4:
        fail(f"usage: {sys.argv[0]} <schema.json> <probes-dir> <check-jsonschema-version>")
    schema, probes_dir, checker_version = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]

    probes = sorted(probes_dir.glob("*.json"))
    if not probes:
        fail(f"no pin probes found in {probes_dir} — the gate would be vacuously green")

    for probe_path in probes:
        try:
            probe = json.loads(probe_path.read_text())
        except json.JSONDecodeError as e:
            fail(f"{probe_path.name}: not valid JSON ({e})")
        try:
            control, mutation = probe["control"], probe["mutation"]
            path, value = mutation["path"], mutation["value"]
        except (KeyError, TypeError):
            fail(f"{probe_path.name}: a probe must carry 'control' and 'mutation' {{path, value}}")

        node = control
        for key in path[:-1]:
            if not isinstance(node, dict) or key not in node:
                fail(f"{probe_path.name}: mutation path {path} does not exist in the control")
            node = node[key]
        leaf = path[-1]
        if not isinstance(node, dict) or leaf not in node:
            fail(f"{probe_path.name}: mutation path {path} does not exist in the control")
        if node[leaf] == value:
            fail(f"{probe_path.name}: mutation value equals the control's — the probe is vacuous")

        mutant = copy.deepcopy(control)
        target = mutant
        for key in path[:-1]:
            target = target[key]
        target[leaf] = value

        with tempfile.TemporaryDirectory() as tmp:
            control_path = Path(tmp) / f"{probe_path.stem}.control.json"
            mutant_path = Path(tmp) / f"{probe_path.stem}.mutant.json"
            control_path.write_text(json.dumps(control))
            mutant_path.write_text(json.dumps(mutant))

            result = validate(schema, control_path, checker_version)
            if result.returncode != 0:
                fail(
                    f"{probe_path.name}: the CONTROL failed validation, so the pin under test "
                    f"cannot be isolated:\n{result.stdout}{result.stderr}"
                )
            result = validate(schema, mutant_path, checker_version)
            if result.returncode == 0:
                fail(
                    f"{probe_path.name}: the MUTANT validated — the pin this probe exercises "
                    f"is missing or has been widened (mutation {path} = {json.dumps(value)})"
                )
            errors = instance_validation_errors(result)
            if errors is None:
                fail(
                    f"{probe_path.name}: the mutant run failed for a reason other than instance "
                    f"validation (tool, schema, or parse error) — the pin is unverified:\n"
                    f"{result.stdout}{result.stderr}"
                )

        print(f"pin verified: {probe_path.name} (control accepted, mutant rejected)")


if __name__ == "__main__":
    main()
