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

  * Locale-following reads (#774) — every fixture, snapshot and schema read
    in the replay path is pinned to ``encoding="utf-8"``, because
    ``Path.read_text()`` with no ``encoding=`` decodes in the process
    locale's. ``Utf8ReplayPathTest`` and ``SchemaWalkerUtf8Test`` below run
    the production path over WELL-FORMED non-ASCII UTF-8, which is what
    makes those pins load-bearing under the ``LC_ALL=C PYTHONUTF8=0`` leg
    CI runs this suite in. The malformed-bytes case above cannot do that
    job: those bytes decode under neither US-ASCII nor UTF-8, so it passes
    with or without a pin. The two are complementary — do not fold either
    into the other.

Run: ``uv run python -m unittest test_replay_runner -v``
"""

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from replay_runner import DECODERS, ReplayRunner, _decode, _resolve_body_text, _safe_name
from schema_walker import SchemaWalker

LIVE_FIXTURE = Path(__file__).parent.parent.parent / "tests" / "live-my-surface.json"

# Literal non-ASCII for each file the replay path opens: the fixture's case
# name, the schema's description, and the wire body's values and keys. Mixed
# 2- and 3-byte sequences, since a US-ASCII decode must fail on the first byte
# either way and a narrower set would be a weaker fixture for no gain.
UTF8_CASE_NAME = "GetProject — café ☕"
UTF8_SCHEMA_DESCRIPTION = "Projet — café"
UTF8_PROJECT_NAME = "Café — Ünicode ☕"
UTF8_EXTRA_FIELD = "résumé"


def _live_operations() -> set[str]:
    # Pinned like every other fixture read: the file carries non-ASCII, and
    # read_text() follows the process locale unless told otherwise.
    tests = json.loads(LIVE_FIXTURE.read_text(encoding="utf-8"))
    ops = {t["operation"] for t in tests if t.get("mode") == "live"}
    # Fail closed: an empty set would make the assertions below vacuous.
    assert ops, f"{LIVE_FIXTURE} declared no live operations — the reader is broken"
    return ops


class DecoderCoverageTest(unittest.TestCase):
    """DECODERS must cover exactly the live fixture's operations.

    ``ReplayRunner.coverage_gate`` asserts the same thing, but it only runs
    during a live canary — and the scheduled canary skips whenever its secrets
    are unconfigured. That is how DECODERS sat twenty operations behind the
    fixture with CI fully green (#553). Asserting it here means the drift fails
    an ordinary run. ``scripts/check-replay-decoder-parity`` makes the same
    comparison statically across all five dispatch tables.
    """

    def test_every_live_operation_has_a_decoder(self) -> None:
        missing = sorted(_live_operations() - DECODERS.keys())
        self.assertEqual([], missing, "live operations with no entry in DECODERS")

    def test_no_decoder_for_an_unknown_operation(self) -> None:
        extra = sorted(DECODERS.keys() - _live_operations())
        self.assertEqual([], extra, "DECODERS entries that are not live fixture operations")

    def test_every_decoder_runs_the_sdk_normalize_boundary(self) -> None:
        # The Python SDK has no per-op typed deserializer, so every entry must
        # be the shared parse+normalize callable. A stub or a lambda that
        # swallowed errors would satisfy coverage while decoding nothing.
        for op, decoder in DECODERS.items():
            with self.subTest(op=op):
                self.assertIs(decoder, _decode)
                with self.assertRaises(json.JSONDecodeError):
                    decoder("")


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


def _write_utf8_json(path: Path, payload: object) -> None:
    """Write ``payload`` as JSON with LITERAL non-ASCII bytes on disk.

    Both arguments are load-bearing. ``json.dumps`` defaults to
    ``ensure_ascii=True``, which escapes every non-ASCII character back to
    ``\\uXXXX`` and would leave a pure-ASCII file that any decoder accepts —
    that is precisely why the pre-existing constructor fixture proved nothing.
    ``write_text`` with no ``encoding=`` encodes in the process locale's, which
    under this suite's CI env is US-ASCII, so the write itself would raise.
    """
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")


def _build_utf8_replay_tree(tmpdir: Path) -> tuple[Path, Path, Path]:
    """Lay out a valid single-operation replay tree carrying real UTF-8.

    Returns ``(replay_dir, fixture_path, openapi_path)``. Every file the
    production path opens — fixture, openapi document, wire snapshot — holds
    non-ASCII, so each pinned read is exercised for real rather than over an
    ASCII stand-in.
    """
    fixture_path = tmpdir / "live.json"
    _write_utf8_json(fixture_path, [
        {"name": UTF8_CASE_NAME, "mode": "live", "operation": "GetProject"},
        # A non-live entry, to keep the constructor's mode filter honest.
        {"name": "mock — ignoré", "mode": "mock", "operation": "ListProjects"},
    ])

    openapi_path = tmpdir / "openapi.json"
    _write_utf8_json(openapi_path, {
        "paths": {
            "/projects/{id}.json": {
                "get": {
                    "operationId": "GetProject",
                    "responses": {
                        "200": {
                            "content": {
                                "application/json": {
                                    "schema": {
                                        "type": "object",
                                        "description": UTF8_SCHEMA_DESCRIPTION,
                                        "required": ["id", "name"],
                                        "properties": {
                                            "id": {"type": "integer"},
                                            "name": {"type": "string"},
                                        },
                                    }
                                }
                            }
                        }
                    },
                }
            }
        }
    })

    body = {"id": 1, "name": UTF8_PROJECT_NAME, UTF8_EXTRA_FIELD: "présent"}
    replay_dir = tmpdir / "replay"
    wire_dir = replay_dir / "bc4" / "wire"
    wire_dir.mkdir(parents=True)
    _write_utf8_json(wire_dir / f"{_safe_name(UTF8_CASE_NAME)}.json", {
        "operation": "GetProject",
        "pages_count": 1,
        "pages": [{
            "status": 200,
            "headers": {},
            "body": body,
            "bodyText": json.dumps(body, ensure_ascii=False),
            "url": "https://example.test/projects/1.json",
        }],
    })

    return replay_dir, fixture_path, openapi_path


class Utf8ReplayPathTest(unittest.TestCase):
    """Run the production replay path over well-formed non-ASCII UTF-8.

    Every read this exercises is pinned to ``encoding="utf-8"``; drop any one
    of them and this class reds under ``LC_ALL=C PYTHONUTF8=0``, which is the
    env .github/workflows/test.yml runs the suite in. Before these tests
    existed the pins were assertions nothing could falsify: the only
    ``ReplayRunner`` construction wrote its fixture with default ASCII
    escaping, the coverage gate was only ever handed malformed bytes, and
    ``_read_snapshot`` was reached by nothing.
    """

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.replay_dir, self.fixture_path, self.openapi_path = \
            _build_utf8_replay_tree(Path(self._tmp.name))

    def _runner(self) -> ReplayRunner:
        return ReplayRunner(self.replay_dir, "bc4", self.fixture_path, self.openapi_path)

    def test_constructor_reads_a_non_ascii_fixture(self) -> None:
        # Covers ReplayRunner.__init__'s fixture read. Asserting the name
        # round-trips (not just that construction succeeded) keeps this from
        # passing on a mojibake decode.
        self.assertEqual([UTF8_CASE_NAME], [t["name"] for t in self._runner()._fixture])

    def test_coverage_gate_accepts_a_well_formed_non_ascii_snapshot(self) -> None:
        # Covers coverage_gate's snapshot read. Unpinned, the read raises
        # UnicodeDecodeError, the gate's own handler turns it into a "not
        # valid UTF-8" message, and this assertion fails — the gate reports a
        # corrupt snapshot that is in fact perfectly good.
        self.assertEqual([], self._runner().coverage_gate())

    def test_run_decodes_a_non_ascii_snapshot(self) -> None:
        # Covers _read_snapshot, reachable only through run().
        self.assertEqual(0, self._runner().run())

        result = json.loads(
            (self.replay_dir / "bc4" / "decode" / "python"
             / f"{_safe_name(UTF8_CASE_NAME)}.json").read_text(encoding="utf-8")
        )
        page = result["pages"][0]
        self.assertTrue(page["decoded"], page["decode_error"])
        self.assertEqual([], page["missing_required"])
        # The snapshot's non-ASCII content reached the schema walker intact —
        # a decoded=True on its own would also hold for an empty body.
        self.assertEqual([UTF8_EXTRA_FIELD], page["extras_seen"])


class SchemaWalkerUtf8Test(unittest.TestCase):
    def test_walker_reads_a_non_ascii_openapi_document(self) -> None:
        # Covers SchemaWalker.__init__ directly. The real openapi.json carries
        # non-ASCII descriptions; the only construction in this suite before
        # now passed the ASCII-only document "{}".
        with tempfile.TemporaryDirectory() as tmp:
            _, _, openapi_path = _build_utf8_replay_tree(Path(tmp))
            schema = SchemaWalker(openapi_path).find_response_schema("GetProject")

            self.assertIsNotNone(schema)
            self.assertEqual(UTF8_SCHEMA_DESCRIPTION, schema["description"])


if __name__ == "__main__":
    unittest.main()
