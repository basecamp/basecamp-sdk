#!/usr/bin/env bash
#
# Fix List*ResponseContent schemas to match BC3 API response format.
#
# Problem: Smithy generates wrapped responses like {"projects": [...]}
# Reality: BC3 API returns raw arrays like [...]
#
# This script transforms all List*ResponseContent schemas from:
#   {"type": "object", "properties": {"items": {"type": "array", ...}}}
# To:
#   {"type": "array", "items": ...}
#
# Usage: ./fix-list-response-schemas.sh [input.json] [output.json]
#        ./fix-list-response-schemas.sh    # defaults to openapi.json in-place

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

INPUT_FILE="${1:-$PROJECT_ROOT/openapi.json}"
OUTPUT_FILE="${2:-$INPUT_FILE}"

if [[ ! -f "$INPUT_FILE" ]]; then
    echo "Error: Input file not found: $INPUT_FILE" >&2
    exit 1
fi

# Use Python for complex JSON transformation
python3 << EOF
import json
import re

with open('$INPUT_FILE', 'r') as f:
    spec = json.load(f)

schemas = spec.get('components', {}).get('schemas', {})
fixed = 0

for name, schema in list(schemas.items()):
    # Only process List*ResponseContent schemas
    if not (name.startswith('List') and name.endswith('ResponseContent')):
        continue

    # Check if it's the wrapped object pattern
    if schema.get('type') != 'object':
        continue

    props = schema.get('properties', {})
    if len(props) != 1:
        continue

    # Get the single property (e.g., "projects", "todos", etc.)
    prop_name = list(props.keys())[0]
    prop_value = props[prop_name]

    # Check if it's an array type
    if prop_value.get('type') != 'array':
        continue

    # Convert to direct array
    schemas[name] = {
        'type': 'array',
        'items': prop_value['items']
    }
    fixed += 1

with open('$OUTPUT_FILE', 'w') as f:
    json.dump(spec, f, indent=2)
    f.write('\n')

print(f"Fixed {fixed} List*ResponseContent schemas to use raw arrays")
EOF
