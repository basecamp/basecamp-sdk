#!/usr/bin/env python3
"""Verify the event-feed schema's load-bearing allOf pins via derived mutants.

Each probe file under the probes directory declares a `control` scenario and one
`mutation` (a path into the control and the value to plant there). The gate:

  1. validates the control against the schema — it MUST be accepted;
  2. applies the mutation in-process and validates the mutant — it MUST be rejected.

Because the mutant is derived from a control that just parsed and validated, a
rejection can only be a schema rejection of the mutation's delta: there is no
committed invalid file to go stale, drift extra deltas, or fail as malformed JSON,
and the control-first ordering rules out schema-load failures masquerading as
rejections. A mutation whose path is absent from the control, or whose value
already equals the control's, fails the gate as a vacuous probe.
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
            "--schemafile",
            str(schema),
            str(instance_path),
        ],
        capture_output=True,
        text=True,
    )


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

        print(f"pin verified: {probe_path.name} (control accepted, mutant rejected)")


if __name__ == "__main__":
    main()
