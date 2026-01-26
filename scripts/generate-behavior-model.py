#!/usr/bin/env python3
"""
Generate behavior-model.json from Smithy spec files.

Extracts operation semantics (readonly, idempotent, pagination, retry policies)
and redaction rules (sensitive fields) from the Smithy specification.
"""

import json
import re
import sys
from pathlib import Path
from typing import Any


def parse_smithy_file(filepath: Path) -> str:
    """Read a Smithy file and return its contents."""
    return filepath.read_text()


def extract_operations_with_traits(content: str) -> dict[str, dict[str, Any]]:
    """
    Extract operations and their standard Smithy traits from the main spec.

    Looks for patterns like:
        @readonly
        @http(method: "GET", uri: "/projects.json")
        operation ListProjects { ... }
    """
    operations = {}

    # Pattern to match operation definitions with preceding traits
    # We look for trait annotations followed by operation declarations
    operation_pattern = re.compile(
        r'((?:@[a-zA-Z]+(?:\([^)]*\))?\s*\n)*)'  # Capture traits
        r'operation\s+(\w+)\s*\{',  # Capture operation name
        re.MULTILINE
    )

    for match in operation_pattern.finditer(content):
        traits_block = match.group(1)
        op_name = match.group(2)

        op_info = {}

        # Check for @readonly trait
        if '@readonly' in traits_block:
            op_info['readonly'] = True

        # Check for @idempotent trait
        if '@idempotent' in traits_block:
            op_info['idempotent'] = True

        operations[op_name] = op_info

    return operations


def extract_overlay_traits(content: str) -> dict[str, dict[str, Any]]:
    """
    Extract custom traits from overlay files.

    Looks for patterns like:
        apply ListProjects @pagination({ style: "link" })
        apply GetProject @retry({ max: 5, base_delay_seconds: 1, backoff: "exp+jitter" })
    """
    traits = {}

    # Pattern for apply statements with trait objects
    apply_pattern = re.compile(
        r'apply\s+(\w+)\s+@(\w+)\(\{([^}]+)\}\)',
        re.MULTILINE
    )

    for match in apply_pattern.finditer(content):
        op_name = match.group(1)
        trait_name = match.group(2)
        trait_body = match.group(3)

        if op_name not in traits:
            traits[op_name] = {}

        # Parse the trait body (simple key-value pairs)
        trait_values = {}
        # Match patterns like: style: "link" or max: 5 or supported: false
        kv_pattern = re.compile(r'(\w+):\s*(?:"([^"]+)"|(\d+)|(\w+))')
        for kv_match in kv_pattern.finditer(trait_body):
            key = kv_match.group(1)
            # Get the value from whichever group matched
            value = kv_match.group(2) or kv_match.group(3) or kv_match.group(4)
            # Convert numeric strings to int, booleans to bool
            if value and value.isdigit():
                value = int(value)
            elif value == 'true':
                value = True
            elif value == 'false':
                value = False
            trait_values[key] = value

        traits[op_name][trait_name] = trait_values

    return traits


def extract_sensitive_types(content: str) -> set[str]:
    """
    Extract types marked with @sensitive trait.

    Looks for patterns like:
        @sensitive
        string PersonName
    """
    sensitive_types = set()

    # Pattern for @sensitive followed by type definition
    sensitive_pattern = re.compile(
        r'@sensitive\s*\n\s*(?:string|integer|long|blob|timestamp)\s+(\w+)',
        re.MULTILINE
    )

    for match in sensitive_pattern.finditer(content):
        sensitive_types.add(match.group(1))

    return sensitive_types


def extract_structures_with_sensitive_fields(
    content: str,
    sensitive_types: set[str]
) -> dict[str, list[str]]:
    """
    Find structures that contain sensitive fields.

    Returns a dict mapping structure names to lists of JSON paths for redaction.
    """
    redaction_rules = {}

    # Pattern to extract structure definitions and their members
    struct_pattern = re.compile(
        r'structure\s+(\w+)\s*\{([^}]+)\}',
        re.MULTILINE | re.DOTALL
    )

    member_pattern = re.compile(
        r'(\w+):\s*(\w+)',
        re.MULTILINE
    )

    for struct_match in struct_pattern.finditer(content):
        struct_name = struct_match.group(1)
        struct_body = struct_match.group(2)

        sensitive_fields = []
        for member_match in member_pattern.finditer(struct_body):
            field_name = member_match.group(1)
            field_type = member_match.group(2)

            if field_type in sensitive_types:
                sensitive_fields.append(f'$.{field_name}')

        if sensitive_fields:
            redaction_rules[struct_name] = sensitive_fields

    return redaction_rules


def detect_pagination_from_docs(content: str) -> dict[str, dict[str, Any]]:
    """
    Detect pagination from documentation comments.

    Looks for patterns like:
        /// **Pagination**: Uses Link header (RFC5988). Follow the `next` rel URL
    """
    pagination_ops = {}

    # Pattern to find pagination documentation followed by operation
    doc_pattern = re.compile(
        r'\*\*Pagination\*\*:\s*Uses\s+Link\s+header.*?\n'
        r'.*?operation\s+(\w+)\s*\{',
        re.MULTILINE | re.DOTALL
    )

    for match in doc_pattern.finditer(content):
        op_name = match.group(1)
        pagination_ops[op_name] = {'style': 'link'}

    return pagination_ops


def build_behavior_model(spec_dir: Path) -> dict[str, Any]:
    """
    Build the complete behavior model from all Smithy files.
    """
    # Read main spec
    main_spec = parse_smithy_file(spec_dir / 'basecamp.smithy')

    # Extract operations with standard traits
    operations = extract_operations_with_traits(main_spec)

    # Detect pagination from documentation (fallback)
    doc_pagination = detect_pagination_from_docs(main_spec)

    # Extract sensitive types
    sensitive_types = extract_sensitive_types(main_spec)

    # Extract structures with sensitive fields for redaction
    redaction_rules = extract_structures_with_sensitive_fields(main_spec, sensitive_types)

    # Read overlay files for custom traits
    overlays_dir = spec_dir / 'overlays'
    if overlays_dir.exists():
        for overlay_file in overlays_dir.glob('*.smithy'):
            if overlay_file.name in ('examples.smithy', 'tags.smithy'):
                continue  # Skip non-behavior overlays

            overlay_content = parse_smithy_file(overlay_file)
            overlay_traits = extract_overlay_traits(overlay_content)

            # Merge overlay traits into operations
            for op_name, traits in overlay_traits.items():
                if op_name not in operations:
                    operations[op_name] = {}

                if 'pagination' in traits:
                    operations[op_name]['pagination'] = traits['pagination']
                if 'retry' in traits:
                    operations[op_name]['retry'] = traits['retry']
                if 'idempotency' in traits:
                    operations[op_name]['idempotency'] = traits['idempotency']

    # Apply documentation-based pagination where not already set by overlays
    for op_name, pagination_info in doc_pagination.items():
        if op_name in operations and 'pagination' not in operations[op_name]:
            operations[op_name]['pagination'] = pagination_info

    # Build final model
    behavior_model = {
        '$schema': 'https://basecamp.com/schemas/behavior-model.json',
        'version': '1.0.0',
        'generated': True,
        'operations': {},
        'redaction': redaction_rules,
        'sensitiveTypes': sorted(list(sensitive_types))
    }

    # Process operations into final format
    for op_name, op_traits in sorted(operations.items()):
        op_entry = {}

        if op_traits.get('readonly'):
            op_entry['readonly'] = True

        if op_traits.get('idempotent'):
            op_entry['idempotent'] = True
        elif 'idempotency' in op_traits:
            op_entry['idempotent'] = op_traits['idempotency'].get('supported', False)

        if 'pagination' in op_traits:
            op_entry['pagination'] = op_traits['pagination']

        if 'retry' in op_traits:
            op_entry['retry'] = op_traits['retry']
        else:
            # Default retry policy based on operation type
            if op_traits.get('readonly'):
                op_entry['retry'] = {'max': 3, 'base_delay_seconds': 1, 'backoff': 'exp+jitter'}
            else:
                op_entry['retry'] = {'max': 0}

        behavior_model['operations'][op_name] = op_entry

    return behavior_model


def main():
    # Determine spec directory
    if len(sys.argv) > 1:
        spec_dir = Path(sys.argv[1])
    else:
        # Default to spec directory relative to script
        script_dir = Path(__file__).parent
        spec_dir = script_dir.parent / 'spec'

    if not spec_dir.exists():
        print(f'Error: Spec directory not found: {spec_dir}', file=sys.stderr)
        sys.exit(1)

    # Output file path
    if len(sys.argv) > 2:
        output_path = Path(sys.argv[2])
    else:
        output_path = spec_dir.parent / 'behavior-model.json'

    # Build and write behavior model
    behavior_model = build_behavior_model(spec_dir)

    with open(output_path, 'w') as f:
        json.dump(behavior_model, f, indent=2)
        f.write('\n')

    # Summary output
    op_count = len(behavior_model['operations'])
    readonly_count = sum(1 for o in behavior_model['operations'].values() if o.get('readonly'))
    paginated_count = sum(1 for o in behavior_model['operations'].values() if 'pagination' in o)
    redaction_count = len(behavior_model['redaction'])

    print(f'Generated {output_path}')
    print(f'  Operations: {op_count} ({readonly_count} readonly, {paginated_count} paginated)')
    print(f'  Redaction rules: {redaction_count} structures')
    print(f'  Sensitive types: {len(behavior_model["sensitiveTypes"])}')


if __name__ == '__main__':
    main()
