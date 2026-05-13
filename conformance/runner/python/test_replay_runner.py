"""Regression test for the empty-bodyText decode-masking bug.

Pre-fix, an empty-but-present ``bodyText`` ("" for HTTP 204 or a genuinely
empty 200 body) was treated as falsy and silently replaced with a
re-serialized ``body: {}`` → ``"{}"``, which decoded successfully. That hid
a real decode failure (the production SDK calls ``json.loads`` on the raw
bytes and would error on ``""``). Post-fix, ``_resolve_body_text`` returns
``""`` directly so the decoder errors and the page reports ``decode_error``.

Run: ``uv run python -m unittest test_replay_runner -v``
"""

from __future__ import annotations

import json
import unittest

from replay_runner import _decode, _resolve_body_text


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


if __name__ == "__main__":
    unittest.main()
