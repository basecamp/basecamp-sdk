"""Shared helpers for the Python code generators (generate_services.py,
generate_types.py)."""

from __future__ import annotations

import re


def escape_py_string(value: str) -> str:
    """Escape arbitrary text for safe interpolation into a Python string or
    docstring literal.

    Escapes backslashes, double-quotes (so the text can't close a triple-quoted
    docstring), and the control characters that would otherwise split the
    emitted source line or form an invalid escape (e.g. a lone ``\\u``). Any
    remaining C0 control or DEL is dropped — a literal NUL in particular makes
    the whole module uncompilable ("source code string cannot contain null
    bytes"). The result stays on one line and is syntactically valid for any
    input, so a deprecation reason sourced from a multi-line OpenAPI
    description can't break the generated module.
    """
    value = (
        value.replace("\\", "\\\\")
        .replace('"', '\\"')
        .replace("\n", "\\n")
        .replace("\r", "\\r")
        .replace("\t", "\\t")
    )
    return re.sub(r"[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]", "", value)
