"""Regression tests for the wire-replay runner.

Covers two known bugs:

  * Empty-bodyText decode masking — pre-fix, an empty-but-present
    ``bodyText`` ("" for HTTP 204 or a genuinely empty 200 body) was treated
    as falsy and silently replaced with a re-serialized ``body: {}`` →
    ``"{}"``, which decoded successfully. That hid a real decode failure
    (the production SDK calls ``json.loads`` on the raw bytes and would
    error on ``""``). Post-fix, ``_resolve_body_text`` returns ``""``
    directly so the decoder errors and the page reports ``decode_error``.

  * Malformed-UTF-8 coverage gate — pre-fix, the coverage-gate snapshot
    reader caught ``OSError`` and ``json.JSONDecodeError`` but not
    ``UnicodeDecodeError`` from ``Path.read_text()``. A snapshot containing
    invalid UTF-8 bytes crashed the gate instead of emitting a clear
    diagnostic. Post-fix, the gate appends a "not valid UTF-8" message and
    keeps going.

Run: ``uv run python -m unittest test_replay_runner -v``
"""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from replay_runner import ReplayRunner, _decode, _resolve_body_text


class ResolveBodyTextTest(unittest.TestCase):
    def test_empty_body_text_passes_through(self) -> None:
        page = {"status": 204, "headers": {}, "body": {}, "bodyText": "", "url": ""}
        self.assertEqual(_resolve_body_text(page), "")

    def test_missing_body_text_falls_back_to_serialized_body(self) -> None:
        page = {"status": 200, "headers": {}, "body": {"a": 1}, "url": ""}
        self.assertEqual(_resolve_body_text(page), json.dumps({"a": 1}))

    def test_non_empty_body_text_wins_over_body(self) -> None:
        page = {"status": 200, "headers": {}, "body": {"a": 1}, "bodyText": '{"b":2}', "url": ""}
        self.assertEqual(_resolve_body_text(page), '{"b":2}')

    def test_decoder_errors_on_empty_body_text(self) -> None:
        # Composes the regression: empty bodyText → "" → decoder raises.
        # Pre-fix this path would have green-passed because "" got replaced
        # by "{}" before reaching the decoder.
        with self.assertRaises(json.JSONDecodeError):
            _decode(_resolve_body_text({"body": {}, "bodyText": ""}))


class CoverageGateUtf8Test(unittest.TestCase):
    def test_malformed_utf8_snapshot_yields_gate_message(self) -> None:
        # Pre-fix: Path.read_text() raised UnicodeDecodeError, which is a
        # ValueError (not OSError) and so escaped the gate's exception
        # filters and crashed the runner. Post-fix: the gate appends a
        # clear "not valid UTF-8" diagnostic and continues.
        with tempfile.TemporaryDirectory() as tmp:
            tmpdir = Path(tmp)
            backend = "bc4"
            test_name = "GetProject"

            fixture_path = tmpdir / "live.json"
            fixture_path.write_text(json.dumps([
                {"name": test_name, "mode": "live", "operation": "GetProject"}
            ]))

            openapi_path = tmpdir / "openapi.json"
            openapi_path.write_text("{}")

            wire_dir = tmpdir / "replay" / backend / "wire"
            wire_dir.mkdir(parents=True)
            (wire_dir / f"{test_name}.json").write_bytes(b"\xff\xfe{\"operation\":\"GetProject\"}")

            runner = ReplayRunner(tmpdir / "replay", backend, fixture_path, openapi_path)
            msgs = runner.coverage_gate()

            self.assertTrue(
                any("not valid UTF-8" in m for m in msgs),
                f"expected 'not valid UTF-8' diagnostic in {msgs!r}",
            )


if __name__ == "__main__":
    unittest.main()
