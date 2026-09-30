"""Unit tests for the TypedDict generator's schema→type mapping.

Focuses on the `types.FlexInt` pixel-dimension handling: the BC3 API serializes
these float-spelled (1024.0), and Python's raw `response.json()` preserves the
float, so the generated type must admit both int and float (and None when the
schema is nullable). Covers both the nullable RichTextAttachment form and the
non-nullable Upload form.
"""

from __future__ import annotations

import importlib.util
from pathlib import Path

_spec = importlib.util.spec_from_file_location(
    "generate_types",
    Path(__file__).parent.parent / "scripts" / "generate_types.py",
)
assert _spec and _spec.loader
generate_types = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(generate_types)
schema_to_type = generate_types.schema_to_type


def test_flexint_nullable_widens_to_optional_int_or_float():
    # RichTextAttachment.width: nullable FlexInt (float-spelled; null for non-images).
    schema = {"type": "integer", "nullable": True, "x-go-type": "types.FlexInt"}
    assert schema_to_type(schema, {}, optional=True) == "NotRequired[Optional[int | float]]"


def test_flexint_non_nullable_widens_to_int_or_float():
    # Upload.width: FlexInt, not nullable.
    schema = {"type": "integer", "x-go-type": "types.FlexInt"}
    assert schema_to_type(schema, {}, optional=True) == "NotRequired[int | float]"
    assert schema_to_type(schema, {}, optional=False) == "int | float"


def test_plain_integer_stays_int():
    assert schema_to_type({"type": "integer"}, {}, optional=False) == "int"


def test_flexible_int64_is_not_treated_as_flexint():
    # Person id's marker must NOT trip the FlexInt widening (substring guard:
    # "FlexibleInt64" does not contain "FlexInt").
    schema = {"type": "integer", "format": "int64", "x-go-type": "types.FlexibleInt64"}
    assert schema_to_type(schema, {}, optional=False) == "int"


def test_nullable_string_is_optional():
    schema = {"type": "string", "nullable": True}
    assert schema_to_type(schema, {}, optional=True) == "NotRequired[Optional[str]]"


def test_card_step_is_a_deprecated_alias_of_subtask():
    # CardStep is the deprecated former name of Subtask: the generated module
    # binds the old name to the very same TypedDict, so annotations that spell
    # CardStep keep meaning exactly what they meant.
    from basecamp.generated import types

    assert types.CardStep is types.Subtask


def test_deprecated_ref_only_component_emits_an_alias(tmp_path):
    # A component that is nothing but a deprecated $ref to a generated TypedDict
    # becomes a module-level alias after every class, with a source comment.
    import json
    import sys

    spec = {
        "components": {
            "schemas": {
                "New": {"type": "object", "properties": {"id": {"type": "integer"}}, "required": ["id"]},
                "Old": {
                    "$ref": "#/components/schemas/New",
                    "deprecated": True,
                    "description": "Deprecated: renamed to New.",
                    "x-deprecated-reason": "renamed to New.",
                },
                "NotAnAlias": {"$ref": "#/components/schemas/New"},
            }
        }
    }
    openapi = tmp_path / "openapi.json"
    openapi.write_text(json.dumps(spec))
    out = tmp_path / "types.py"
    argv = sys.argv
    sys.argv = ["generate_types.py", "--openapi", str(openapi), "--output", str(out)]
    try:
        generate_types.main()
    finally:
        sys.argv = argv
    text = out.read_text()
    assert "# Deprecated: renamed to New.\nOld = New" in text
    assert text.index("class New(TypedDict):") < text.index("Old = New")
    assert "NotAnAlias" not in text
