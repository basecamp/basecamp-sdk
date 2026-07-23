#!/usr/bin/env python3
"""Generates TypedDict classes from OpenAPI response schemas.

Usage: python scripts/generate_types.py [--openapi ../openapi.json] [--output src/basecamp/generated/types.py]
"""
from __future__ import annotations

import json
import keyword
import sys
from pathlib import Path

# Python keywords that can't be used as field names in TypedDicts
PYTHON_KEYWORDS = set(keyword.kwlist)


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
    generated_count = 0
    for name in sorted(schemas):
        schema = schemas[name]
        if schema.get("type") != "object" or not schema.get("properties"):
            continue

        required_fields = set(schema.get("required", []))
        props = schema["properties"]

        lines.append("")
        lines.append(f"class {name}(TypedDict):")

        for prop_name in sorted(props):
            prop = props[prop_name]
            is_optional = prop_name not in required_fields
            py_type = schema_to_type(prop, schemas, optional=is_optional)
            # Escape Python keywords by appending underscore
            field_name = f"{prop_name}_" if prop_name in PYTHON_KEYWORDS else prop_name
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
