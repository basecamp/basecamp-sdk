"""Shared helpers for the Python code generators (generate_services.py,
generate_types.py)."""

from __future__ import annotations


def escape_py_string(value: str) -> str:
    """Escape arbitrary text for safe interpolation into a Python string or
    docstring literal.

    Escapes backslashes, double-quotes (so the text can't close a triple-quoted
    docstring), and the control characters that would otherwise split the
    emitted source line or form an invalid escape (e.g. a lone ``\\u``). The
    result stays on one line and is syntactically valid for any input, so a
    deprecation reason sourced from a multi-line OpenAPI description can't break
    the generated module.
    """
    return (
        value.replace("\\", "\\\\")
        .replace('"', '\\"')
        .replace("\n", "\\n")
        .replace("\r", "\\r")
        .replace("\t", "\\t")
    )
