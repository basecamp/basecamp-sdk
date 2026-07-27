#!/usr/bin/env python3
"""Generates TypedDict classes from OpenAPI response schemas.

Usage: python scripts/generate_types.py [--openapi ../openapi.json] [--output src/basecamp/generated/types.py]
"""
from __future__ import annotations

import json
import keyword
import re
import sys
from pathlib import Path

# Make the shared generator helper importable whether this file is run as a
# script (its dir is already sys.path[0]) or loaded via importlib in tests.
sys.path.insert(0, str(Path(__file__).parent))
from gen_common import escape_py_string  # noqa: E402

# Python keywords that can't be used as field names in TypedDicts
PYTHON_KEYWORDS = set(keyword.kwlist)


def quote_schema_refs(py_type: str, schema_names: set[str]) -> str:
    """Quote whole-word schema-name identifiers in a type annotation as forward
    references, leaving qualifiers (NotRequired, Optional, list) and builtins
    (str, int, bool, Any, None) bare. Used only for functional-syntax TypedDicts,
    where the value expressions are evaluated eagerly: the qualifier must stay
    live so required/optional keys resolve correctly, while a schema defined later
    in the file must be deferred to avoid NameError.
    """

    def repl(m: re.Match) -> str:
        tok = m.group(0)
        return f'"{tok}"' if tok in schema_names else tok

    return re.sub(r"[A-Za-z_][A-Za-z0-9_]*", repl, py_type)


def schema_to_type(schema: dict, schemas: dict, *, optional: bool = False) -> str:
    # OpenAPI 3.1 nullable union: `type: ["string", "null"]`. Resolve the
    # non-null base type (which may itself be a FlexInt dimension) and union it
    # with None so present-but-null values are typed.
    schema_type = schema.get("type")
    if isinstance(schema_type, list) and "null" in schema_type:
        non_null = [t for t in schema_type if t != "null"]
        base_schema = {**schema, "type": non_null[0] if non_null else None}
        base = schema_to_type(base_schema, schemas)
        t = f"{base} | None"
        return f"NotRequired[{t}]" if optional else t

    # types.FlexInt dimensions (rich-text attachment / upload width & height)
    # arrive float-spelled (1024.0) and Python's raw response.json() preserves
    # the float — there is no int-coercion layer — so the honest static type is
    # int | float. ("FlexibleInt64" for Person id does not contain the substring
    # "FlexInt", so the two markers never collide.)
    if "FlexInt" in str(schema.get("x-go-type", "")):
        t = "int | float"
    elif "$ref" in schema:
        ref_name = schema["$ref"].rsplit("/", 1)[-1]
        ref_schema = schemas.get(ref_name, {})
        # Enum schemas (string with enum values) map to str, not a TypedDict
        if ref_schema.get("enum"):
            t = "str"
        else:
            t = ref_name
    elif schema.get("type") == "array":
        items = schema.get("items", {})
        inner = schema_to_type(items, schemas)
        t = f"list[{inner}]"
    elif schema.get("type") == "integer":
        t = "int"
    elif schema.get("type") == "number":
        t = "float"
    elif schema.get("type") == "boolean":
        t = "bool"
    elif schema.get("type") == "string":
        t = "str"
    elif schema.get("type") == "object":
        t = "dict[str, Any]"
    else:
        t = "Any"

    # Nullable schemas carry an explicit null value on the wire (e.g. a
    # rich-text attachment's width/height for non-image blobs). Preserve that
    # in the static type as Optional so a present None is captured, not just an
    # absent key.
    if schema.get("nullable") is True:
        t = f"Optional[{t}]"

    if optional:
        return f"NotRequired[{t}]"
    return t


def main() -> None:
    import argparse
    parser = argparse.ArgumentParser()
    parser.add_argument("--openapi", default=str(Path(__file__).parent.parent.parent / "openapi.json"))
    parser.add_argument("--output", default=str(Path(__file__).parent.parent / "src" / "basecamp" / "generated" / "types.py"))
    args = parser.parse_args()

    with open(args.openapi, encoding="utf-8") as f:
        spec = json.load(f)

    schemas = spec.get("components", {}).get("schemas", {})

    lines: list[str] = [
        "# @generated from OpenAPI spec — do not edit manually",
        "",
        "from __future__ import annotations",
        "",
        "from typing import Any, NotRequired, Optional, TypedDict",
    ]

    # Emit type aliases for map schemas (object with additionalProperties, no properties)
    for sname in sorted(schemas):
        schema = schemas[sname]
        if schema.get("type") == "object" and not schema.get("properties") and schema.get("additionalProperties"):
            val_type = schema_to_type(schema["additionalProperties"], schemas)
            lines.append(f"\n{sname} = dict[str, {val_type}]")

    lines.append("")

    # Sort schemas alphabetically for deterministic output
    schema_names = set(schemas)
    generated_count = 0
    for name in sorted(schemas):
        schema = schemas[name]
        if schema.get("type") != "object" or not schema.get("properties"):
            continue

        required_fields = set(schema.get("required", []))
        props = schema["properties"]

        # A JSON key that is a Python keyword (e.g. `from` on Inbox::Forward) or
        # otherwise not a valid identifier cannot be a class-based TypedDict field
        # without mangling the key. Appending `_` (from_) would make the public
        # typing lie about the real wire key. When any key needs that, emit the
        # whole TypedDict with the functional TypedDict("Name", {...}) syntax,
        # which preserves the real JSON keys verbatim.
        needs_functional = any(
            (p in PYTHON_KEYWORDS) or (not p.isidentifier()) for p in props
        )

        lines.append("")
        if needs_functional:
            if schema.get("deprecated"):
                reason = escape_py_string(schema.get("x-deprecated-reason") or "deprecated")
                lines.append(f"# Deprecated: {reason}")
            lines.append(f'{name} = TypedDict("{name}", {{')
            for prop_name in sorted(props):
                prop = props[prop_name]
                is_optional = prop_name not in required_fields
                py_type = schema_to_type(prop, schemas, optional=is_optional)
                if prop.get("deprecated"):
                    reason = escape_py_string(prop.get("x-deprecated-reason") or "deprecated")
                    lines.append(f"    # deprecated (source-only): {reason}")
                # Forward-reference only the schema-name identifiers (types defined
                # later in the file), leaving NotRequired/list/Optional and builtins
                # bare so the functional TypedDict still SEES the NotRequired
                # qualifier at construction time — otherwise every field lands in
                # __required_keys__ and none in __optional_keys__, breaking runtime
                # introspection. E.g. NotRequired[TodoBucket] -> NotRequired["TodoBucket"].
                lines.append(f'    "{prop_name}": {quote_schema_refs(py_type, schema_names)},')
                if "FlexibleInt64" in str(prop.get("x-go-type", "")):
                    lines.append('    "system_label": NotRequired[str],')
            lines.append("})")
            generated_count += 1
            continue

        lines.append(f"class {name}(TypedDict):")
        # Documentation-only deprecation (see #406): a real class docstring for a
        # wholly deprecated TypedDict. TypedDicts have no runtime docstring hook
        # for individual fields, so per-field deprecation is a source-only comment
        # below.
        if schema.get("deprecated"):
            reason = escape_py_string(schema.get("x-deprecated-reason") or "deprecated")
            lines.append(f'    """Deprecated: {reason}"""')

        for prop_name in sorted(props):
            prop = props[prop_name]
            is_optional = prop_name not in required_fields
            py_type = schema_to_type(prop, schemas, optional=is_optional)
            field_name = prop_name
            # TypedDict fields carry no directive; label deprecation honestly as a
            # source-only comment (see #406).
            if prop.get("deprecated"):
                # escape_py_string collapses control chars to escapes, so a
                # multi-line reason stays a single `#` comment line.
                reason = escape_py_string(prop.get("x-deprecated-reason") or "deprecated")
                lines.append(f"    # deprecated (source-only): {reason}")
            lines.append(f"    {field_name}: {py_type}")
            # Add system_label field after id for flexible integer fields
            # (system actors like LocalPerson have non-numeric labels as id)
            if "FlexibleInt64" in str(prop.get("x-go-type", "")):
                lines.append("    system_label: NotRequired[str]")

        generated_count += 1

    if generated_count == 0:
        lines.append("")
        lines.append("# No object schemas found in spec")

    lines.append("")

    output = "\n".join(lines)

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(output, encoding="utf-8")
    print(f"Generated {output_path} ({generated_count} TypedDict classes)")


if __name__ == "__main__":
    main()
