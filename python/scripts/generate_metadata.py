#!/usr/bin/env python3
"""Generates metadata.json from openapi.json + behavior-model.json.

Extracts per-operation retry/idempotency config for the HTTP client.

Usage: python scripts/generate_metadata.py [--openapi ../openapi.json] [--behavior ../behavior-model.json] [--output src/basecamp/generated/metadata.json]
"""
from __future__ import annotations

import json
import sys
from pathlib import Path


def main() -> None:
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--openapi", default=str(Path(__file__).parent.parent.parent / "openapi.json"))
    parser.add_argument("--behavior", default=str(Path(__file__).parent.parent.parent / "behavior-model.json"))
    parser.add_argument("--output", default=str(Path(__file__).parent.parent / "src" / "basecamp" / "generated" / "metadata.json"))
    args = parser.parse_args()

    with open(args.openapi, encoding="utf-8") as f:
        spec = json.load(f)

    with open(args.behavior, encoding="utf-8") as f:
        behavior = json.load(f)

    operations = behavior.get("operations", {})
    metadata: dict[str, dict] = {}

    # Extract all operation IDs from the spec
    for path_item in spec.get("paths", {}).values():
        for method in ("get", "post", "put", "patch", "delete"):
            op = path_item.get(method)
            if not op:
                continue
            op_id = op.get("operationId")
            if not op_id:
                continue

            entry: dict = {}
            behavior_op = operations.get(op_id, {})

            if behavior_op.get("idempotent"):
                entry["idempotent"] = True

            retry = behavior_op.get("retry")
            if retry:
                entry["retry"] = {
                    "max": retry.get("max", 3),
                    "base_delay_ms": retry.get("base_delay_ms", 1000),
                    "backoff": retry.get("backoff", "exponential"),
                    "retry_on": retry.get("retry_on", [429, 503]),
                }

            if entry:
                metadata[op_id] = entry

    output = json.dumps(metadata, indent=2, sort_keys=True) + "\n"

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(output, encoding="utf-8")
    print(f"Generated {output_path} ({len(metadata)} operations)")


if __name__ == "__main__":
    main()
