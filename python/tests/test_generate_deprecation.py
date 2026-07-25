"""Fixture for the deprecation-reason escaping shared by the Python generators
(#406). A deprecation reason can, in principle, be sourced from a multi-line
OpenAPI description containing quotes and backslashes; escape_py_string must keep
such text valid when interpolated into a docstring or a source comment.
"""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
from gen_common import escape_py_string  # noqa: E402

# Triple quotes (would close a docstring), a backslash + `u` (would be an invalid
# unicode escape), and an embedded newline (would split the source line).
NASTY = 'has """ triple, back\\slash \\u not-unicode, and\na newline'


def test_escaped_reason_stays_single_line_and_quote_safe():
    escaped = escape_py_string(NASTY)
    assert "\n" not in escaped
    assert "\r" not in escaped
    assert '"""' not in escaped


def test_escaped_reason_compiles_in_docstring_and_comment():
    reason = escape_py_string(NASTY)
    module_src = (
        "class Foo:\n"
        f'    """Deprecated: {reason}"""\n'
        f"    # deprecated (source-only): {reason}\n"
        "    x: int = 0\n"
        "\n"
        "def bar():\n"
        f'    """Deprecated parameters (prefer the replacement):\n\n    - type: {reason}\n    """\n'
        "    return 1\n"
    )
    # Both the class/def docstrings and the field comment must remain valid
    # Python for an arbitrary reason.
    compile(module_src, "<generated-deprecation-fixture>", "exec")


def test_raw_reason_would_break_without_escaping():
    # Guard: the fixture input really is hostile — the unescaped form does not
    # compile, so the escaping above is doing real work.
    module_src = f'x = """Deprecated: {NASTY}"""\n'
    try:
        compile(module_src, "<raw>", "exec")
    except (SyntaxError, ValueError):
        return
    raise AssertionError("expected the raw reason to break compilation")
