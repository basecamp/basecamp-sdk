"""Fixture for the generated-service method docstrings (#663). A description
can, in principle, contain triple quotes (which would close the docstring
early), backslashes (which would form stray escape sequences), and embedded
newlines; escape_docstring_text must keep such text valid when the generator
interpolates it into a method's triple-quoted docstring.
"""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
from generate_services import build_params, extract_body_params, method_docstring  # noqa: E402

# Triple quotes (would close the docstring), a backslash + `u` (would be an
# invalid unicode escape), an embedded newline, a literal NUL (JSON `\\u0000`
# — "source code string cannot contain null bytes"), and a trailing
# double-quote (would fuse with the closing delimiter if it ever ended a
# docstring).
NASTY = 'has """ triple, back\\slash \\u not-unicode, a \x00 NUL, and\na newline, ends with quote "'


def _hostile_op() -> dict:
    return {
        "operation_id": "NastyOp",
        "description": NASTY + "\n\n**Pagination**: manual\n\nsecond paragraph",
        "path_params": [
            {"name": "projectId", "python_name": "project_id", "type": "int", "description": NASTY},
        ],
        "query_params": [
            {
                "name": "type",
                "python_name": "type",
                "type": "str",
                "required": False,
                "deprecated": True,
                "deprecation_reason": NASTY,
                "description": None,
            },
        ],
        "body_params": [
            {"name": "note", "python_name": "note", "type": "str", "required": True, "description": NASTY},
        ],
        "has_body": True,
        "has_binary_body": False,
        "has_multipart_body": False,
        "has_pagination": True,
        "pagination_key": None,
    }


def _emit(op: dict) -> tuple[str, str]:
    """Render the docstring at class-method depth, compile it, and return
    (emitted source, runtime docstring)."""
    params = build_params(op)
    doc = "\n".join(method_docstring(op, params))
    src = f"class C:\n    def f(self):\n{doc}\n        return 1\n"
    namespace: dict = {}
    exec(compile(src, op["operation_id"], "exec"), namespace)  # noqa: S102
    return doc, namespace["C"].f.__doc__


def test_hostile_description_stays_valid_and_composes_deprecation():
    doc, runtime = _emit(_hostile_op())
    # The hostile text survives escaping intact at runtime...
    assert '"""' in runtime
    # ...minus the source-breaking NUL, which is dropped, not smuggled through.
    assert "\x00" not in doc and "\x00" not in runtime
    # ...and every docstring section is present: summary, deprecation note
    # (composed, not clobbered), and Args covering path/body/query/max_items.
    assert "Deprecated parameters (prefer the replacement):" in runtime
    for arg in ("project_id:", "note:", "type:", "max_items:"):
        assert arg in runtime


def test_ref_body_property_reads_target_schema_description():
    schemas = {
        "Body": {
            "properties": {
                "calendar": {"$ref": "#/components/schemas/Attrs"},
                "labeled": {"$ref": "#/components/schemas/Attrs", "description": "Sibling wins."},
                "bare": {"$ref": "#/components/schemas/Undescribed"},
            },
        },
        "Attrs": {"type": "object", "description": "The writable payload."},
        "Undescribed": {"type": "object"},
    }
    by_name = {p["name"]: p for p in extract_body_params({"$ref": "#/components/schemas/Body"}, schemas)}
    # A bare $ref property has no description of its own — the referenced
    # schema's description documents it; a sibling description still wins; an
    # undescribed target leaves None for the humanized fallback.
    assert by_name["calendar"]["description"] == "The writable payload."
    assert by_name["labeled"]["description"] == "Sibling wins."
    assert by_name["bare"]["description"] is None


def test_raw_description_would_break_without_escaping():
    # Guard: the fixture input really is hostile — the unescaped form does not
    # compile, so the escaping above is doing real work.
    src = f'x = """{NASTY}"""\n'
    try:
        compile(src, "<raw>", "exec")
    except (SyntaxError, ValueError):
        return
    raise AssertionError("expected the raw description to break compilation")


def test_degenerate_operation_shapes_compile():
    base = {
        "path_params": [],
        "query_params": [],
        "body_params": [],
        "has_body": False,
        "has_binary_body": False,
        "has_multipart_body": False,
        "has_pagination": False,
        "pagination_key": None,
    }
    no_params = {**base, "operation_id": "X", "description": ""}
    pagination_only = {
        **base,
        "operation_id": "P",
        "description": "**Pagination**: Uses Link header.",
        "has_pagination": True,
        "pagination_key": "events",
    }
    doc, runtime = _emit(no_params)
    assert doc.strip() == '"""X operation."""'
    # A no-param op whose description ends in a double-quote must not emit
    # `"""text""""`: the summary-period rule treats a trailing quote as
    # non-terminal punctuation, so every one-line docstring ends in `.!?`,
    # never adjacent to the closing delimiter.
    trailing_quote = {**base, "operation_id": "T", "description": 'Group by "bucket" or "date"'}
    doc, runtime = _emit(trailing_quote)
    assert '""""' not in doc
    assert doc.strip().endswith('"."""')
    assert runtime == 'Group by "bucket" or "date".'
    _, runtime = _emit(pagination_only)
    # The manual Link-following boilerplate is dropped (the method paginates
    # itself); max_items documents the collected-pages behavior instead.
    assert "Link header" not in runtime
    assert "max_items:" in runtime
