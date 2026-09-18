"""Shared helpers for the Python code generators (generate_services.py,
generate_types.py)."""

from __future__ import annotations

import json
import os
import re
from collections.abc import Iterator, Sequence
from pathlib import Path


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


# --- Path Item walking (basecamp-sdk#925) -----------------------------------
#
# A Path Item Object is read by EXCLUSION. Its non-operation fields are a
# closed, spec-defined set and its extensions are ``x-`` prefixed, so every
# OTHER field is an operation. Enumerating the verbs instead is the defect this
# replaces: Smithy's ``@http`` trait takes the method as a free-form string it
# "will use literally and will perform no validation on", so a model author
# writing ``method: "HEAD"`` produced a valid model, a valid ``openapi.json``,
# and no method on any client — the verb was not in the list, so the operation
# was stepped over in silence.

NON_OPERATION_FIELDS = frozenset({"summary", "description", "servers", "parameters"})

# The self-test points this at a crafted declaration to prove the bound is sourced
# from the shared file rather than a private literal; production runs never set it.
GENERATED_VERBS_FILE = Path(
    os.environ.get(
        "BASECAMP_GENERATED_VERBS",
        Path(__file__).resolve().parent.parent.parent / "spec" / "generated-verbs.json",
    )
)


def generated_verbs() -> tuple[str, ...]:
    """The ordered HTTP methods the SDK generators emit.

    One declaration for all six SDKs; see ``spec/generated-verbs.json`` for why
    it is a policy rather than six per-language capabilities.
    """
    try:
        declaration = json.loads(GENERATED_VERBS_FILE.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise SystemExit(
            f"Error: cannot read the generated-verb declaration {GENERATED_VERBS_FILE}: {error}"
        ) from error
    verbs = declaration.get("verbs")
    if not isinstance(verbs, list) or not verbs or not all(isinstance(v, str) and v for v in verbs):
        raise SystemExit(
            f"Error: {GENERATED_VERBS_FILE} must declare a non-empty `verbs` array of strings."
        )
    return tuple(verbs)


def iter_operations(
    path: str, path_item: object, emittable: Sequence[str] | None = None
) -> Iterator[tuple[str, dict]]:
    """Yield ``(verb, operation)`` for every operation in one path item.

    ``emittable`` bounds what the caller can RENDER, and is checked after
    discovery: an operation on any other verb stops the run by name rather than
    being dropped. Pass ``None`` from a caller that is verb-agnostic (the
    metadata extractors key everything on operationId).

    Visit order follows ``generated_verbs()`` so emitted output stays
    byte-stable; a verb outside it sorts deterministically to the end by name,
    which is ordering, not membership.
    """
    order = generated_verbs()

    if not isinstance(path_item, dict):
        raise SystemExit(
            f"Error: openapi.json path {path} is a {type(path_item).__name__}, not a path item object."
        )

    # A `$ref` path item points at operations this walk cannot see without
    # resolving the reference. Skipping it is the same silent under-count the
    # exclusion walk exists to prevent, so refuse instead.
    if "$ref" in path_item:
        raise SystemExit(
            f"Error: openapi.json path {path} is a $ref to {path_item['$ref']!r}. Resolving a "
            "path-item reference is not implemented, and skipping it would hide every operation "
            "behind it from the SDK."
        )

    fields = [
        field
        for field in path_item
        if field not in NON_OPERATION_FIELDS and not field.startswith("x-")
    ]
    fields.sort(key=lambda f: (order.index(f) if f in order else len(order), f))

    for field in fields:
        operation = path_item[field]
        if not isinstance(operation, dict):
            raise SystemExit(
                f"Error: openapi.json path {path} field {field!r} is a "
                f"{type(operation).__name__}, which is neither a known non-operation field nor an "
                "operation object. If a later OpenAPI version added it, add it to "
                "NON_OPERATION_FIELDS with a reason."
            )
        if emittable is not None and field not in emittable:
            op_id = operation.get("operationId", "(no operationId)")
            raise SystemExit(
                f"Error: openapi.json declares {field.upper()} {path} ({op_id}), and this "
                f"generator emits only {'/'.join(v.upper() for v in emittable)}. Generating the "
                "rest of the SDK without it would drop the operation from every client in "
                "silence, which is the failure basecamp-sdk#925 closed. Give the runtime a "
                f"{field} helper and add {field!r} to spec/generated-verbs.json (read that file "
                "first — the other five SDKs need the same helper), or take the operation out of "
                "the Smithy model."
            )
        yield field, operation
