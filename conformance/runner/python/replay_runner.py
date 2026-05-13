#!/usr/bin/env python3
"""Wire-replay runner for the Python SDK.

Reads wire snapshots written by the TS canonical canary runner, decodes each
page through the Python SDK's parse + normalize pipeline, walks the raw JSON
for required-field and extras detection, and persists per-test decode-result
snapshots at ``<WIRE_REPLAY_DIR>/<BACKEND>/decode/python/<safe>.json``.

Mode-gate: this script is invoked only when ``WIRE_REPLAY_DIR`` is set
(see Makefile target ``conformance-python-replay``). The existing mock
runner (``runner.py``) handles the unset case.

Decode boundary
---------------
The Python SDK does not use typed dataclasses for response bodies — every
generated service method returns ``dict[str, Any]`` or a ``ListResult`` of
dicts. The only post-parse transformation the SDK applies is
``_normalize_person_ids`` (see ``basecamp.generated.services._base``),
which coerces system-actor Person ids ("basecamp", "campfire") to numeric
form. Calling that function directly on a parsed body is therefore the
faithful Python "decode" — it exercises the same code path the SDK runs
on every live response, so a regression in normalize (e.g. a new
personable shape) surfaces here as a decode_error rather than silently
passing.
"""

from __future__ import annotations

import json
import os
import re
import sys
from pathlib import Path
from typing import Any, Callable

# Driver import; the only SDK piece we exercise is the post-parse normalize
# function — see module docstring for why this is the right boundary.
from basecamp.generated.services._base import _normalize_person_ids

from schema_walker import SchemaWalker

SCHEMA_VERSION = 1


def _decode(body_text: str) -> None:
    """Parse a single page body and run the SDK's normalize pass.

    Raises on JSON parse failure; raises if normalize raises (it currently
    only mutates dict/list, but a future regression could throw).
    """
    parsed = json.loads(body_text)
    _normalize_person_ids(parsed)


# Map operation_id -> decoder. Same boundary for every op (see module
# docstring) — the SDK has no per-op typed deserializer to invoke. Keeping
# the dict explicit (rather than a default fallback) makes the
# coverage_gate diagnostic accurate: a typo or missing op surfaces
# loudly instead of being absorbed by a default decoder.
DECODERS: dict[str, Callable[[str], None]] = {
    "ListProjects": _decode,
    "GetProject": _decode,
    "GetMyAssignments": _decode,
    "GetMyCompletedAssignments": _decode,
    "GetMyDueAssignments": _decode,
    "GetMyNotifications": _decode,
    "GetMyProfile": _decode,
    "GetTodoset": _decode,
    "ListTodolists": _decode,
    "ListTodos": _decode,
}


def _safe_name(name: str) -> str:
    return re.sub(r"[^a-z0-9_-]+", "_", name, flags=re.IGNORECASE)


def _resolve_body_text(page: dict) -> str:
    """Return the bytes the decoder should see for a wire-snapshot page.

    Empty-but-present ``bodyText`` (HTTP 204 or an actually-empty 200 body)
    must flow through as ``""`` so the decoder errors — not be silently
    replaced with a re-serialized ``body`` field, which would mask a real
    decode failure.
    """
    raw = page.get("bodyText")
    return raw if raw is not None else json.dumps(page["body"])


class ReplayRunner:
    def __init__(self, replay_dir: Path, backend: str, fixture_path: Path, openapi_path: Path) -> None:
        self._replay_dir = replay_dir
        self._backend = backend
        self._walker = SchemaWalker(openapi_path)
        self._fixture: list[dict] = [
            t for t in json.loads(fixture_path.read_text())
            if t.get("mode") == "live"
        ]

    def coverage_gate(self) -> list[str]:
        msgs: list[str] = []
        fixture_ops = sorted({t["operation"] for t in self._fixture})

        # 1. Decoder coverage — every fixture operation must dispatch.
        missing = [op for op in fixture_ops if op not in DECODERS]
        if missing:
            msgs.append(
                f"Python replay runner missing decoders for: {', '.join(missing)}. "
                "Add to DECODERS in replay_runner.py."
            )

        # 2. Snapshot completeness — every fixture op needs a wire file.
        wire_dir = self._replay_dir / self._backend / "wire"
        for t in self._fixture:
            f = wire_dir / f"{_safe_name(t['name'])}.json"
            if not f.exists():
                msgs.append(
                    f"Snapshot missing for operation {t['operation']} "
                    f"(test {t['name']!r}); expected at {f}. "
                    "Re-run TS live capture or check skip status."
                )

        # 3. Snapshot recognition — every captured snapshot's operation
        #    must be in the shared fixture (catches TS-side dispatch drift).
        if wire_dir.exists():
            for f in sorted(wire_dir.glob("*.json")):
                snap = json.loads(f.read_text())
                op = snap.get("operation")
                if op is None:
                    msgs.append(
                        f"Snapshot {f.name} is missing the top-level 'operation' field. "
                        "Re-run the TS live canary; pre-PR3 snapshots are no longer supported."
                    )
                    continue
                if op not in fixture_ops:
                    msgs.append(
                        f"Unknown operation {op!r} in snapshot {f.name}; "
                        "TS dispatch table appears to have drifted from live-my-surface.json."
                    )
        return msgs

    def run(self) -> int:
        msgs = self.coverage_gate()
        if msgs:
            for m in msgs:
                print(m, file=sys.stderr)
            return 1

        out_dir = self._replay_dir / self._backend / "decode" / "python"
        out_dir.mkdir(parents=True, exist_ok=True)

        failures = 0
        for t in self._fixture:
            snapshot = self._read_snapshot(t["name"])
            result = self._decode_snapshot(snapshot)
            (out_dir / f"{_safe_name(t['name'])}.json").write_text(
                json.dumps(result, indent=2)
            )
            if any(not p["decoded"] or p["missing_required"] for p in result["pages"]):
                failures += 1

        return 0 if failures == 0 else 1

    def _read_snapshot(self, test_name: str) -> dict:
        path = self._replay_dir / self._backend / "wire" / f"{_safe_name(test_name)}.json"
        return json.loads(path.read_text())

    def _decode_snapshot(self, snapshot: dict) -> dict:
        operation = snapshot["operation"]
        decoder = DECODERS[operation]
        schema = self._walker.find_response_schema(operation)

        pages = []
        for page in snapshot["pages"]:
            body_text: str = _resolve_body_text(page)
            decoded = False
            decode_error: str | None = None
            missing_required: list[str] = []
            extras_seen: list[str] = []

            try:
                decoder(body_text)
                decoded = True
            except Exception as e:
                decode_error = f"{type(e).__name__}: {e}"

            if schema is not None:
                # Re-parse for the walker if body came across as a string
                # (non-JSON page body). On parse failure, leave body None
                # and skip the walker — the decode_error above already
                # records the failure.
                body: Any = page.get("body")
                if isinstance(body, str):
                    try:
                        body = json.loads(body_text)
                    except Exception:
                        body = None
                if body is not None:
                    missing_required = self._walker.missing_required(body, schema)
                    extras_seen = self._walker.extras_seen(body, schema)

            pages.append({
                "decoded": decoded,
                "decode_error": decode_error,
                "missing_required": missing_required,
                "extras_seen": extras_seen,
            })

        return {
            "schema_version": SCHEMA_VERSION,
            "operation": operation,
            "pages": pages,
        }


if __name__ == "__main__":
    replay_dir = os.environ.get("WIRE_REPLAY_DIR")
    backend = os.environ.get("BASECAMP_BACKEND")
    if not replay_dir:
        sys.exit("WIRE_REPLAY_DIR is required")
    if not backend:
        sys.exit("BASECAMP_BACKEND is required")

    fixture_path = Path(__file__).parent.parent.parent / "tests" / "live-my-surface.json"
    openapi_path = Path(__file__).parent.parent.parent.parent / "openapi.json"
    runner = ReplayRunner(Path(replay_dir), backend, fixture_path, openapi_path)
    sys.exit(runner.run())
