#!/usr/bin/env bash
#
# upgrade-openapi-3.2.sh - Upgrade OpenAPI 3.1.0 to 3.2.0
#
# Smithy generates OpenAPI 3.1.0 (max supported version).
# This script upgrades to 3.2.0 for latest spec compliance.
#
# Changes applied:
# - openapi version field: "3.1.0" -> "3.2.0"
# - Add top-level tags array with kind:"resource" for each tag used in operations
# - Preserves all x-basecamp-* extensions
#
# Usage:
#   ./scripts/upgrade-openapi-3.2.sh [input.json] [output.json]
#
# Defaults:
#   input:  openapi.json
#   output: openapi.json (in-place)
#
# Requirements: jq

set -euo pipefail

# Default paths
INPUT_FILE="${1:-openapi.json}"
OUTPUT_FILE="${2:-$INPUT_FILE}"

# Check for jq
if ! command -v jq &> /dev/null; then
    echo "Error: jq is required but not installed." >&2
    echo "Install with: brew install jq" >&2
    exit 1
fi

# Check input file exists
if [[ ! -f "$INPUT_FILE" ]]; then
    echo "Error: Input file not found: $INPUT_FILE" >&2
    exit 1
fi

# Get current version
CURRENT_VERSION=$(jq -r '.openapi' "$INPUT_FILE")

# Validate it's a 3.x version
if [[ ! "$CURRENT_VERSION" =~ ^3\. ]]; then
    echo "Error: Expected OpenAPI 3.x, got: $CURRENT_VERSION" >&2
    exit 1
fi

echo "Upgrading OpenAPI from $CURRENT_VERSION to 3.2.0..."

# Perform the upgrade:
# 1. Update openapi version to 3.2.0
# 2. Extract unique tags from operations and create top-level tags array with kind:"resource"
# 3. Preserve all other content including x-basecamp-* extensions
jq '
  # Extract all unique tags from operations
  ([.paths[][].tags[]?] | unique) as $unique_tags |

  # Create tags array with kind:"resource" for each tag
  ($unique_tags | map({name: ., "x-kind": "resource"})) as $tags_array |

  # Apply transformations
  .openapi = "3.2.0" |
  .tags = $tags_array
' "$INPUT_FILE" > "${OUTPUT_FILE}.tmp"

# Atomic move to output
mv "${OUTPUT_FILE}.tmp" "$OUTPUT_FILE"

echo "Successfully upgraded: $OUTPUT_FILE"

# Report stats
EXTENSION_COUNT=$(grep -c "x-basecamp" "$OUTPUT_FILE" 2>/dev/null || echo "0")
TAG_COUNT=$(jq '.tags | length' "$OUTPUT_FILE")
echo "x-basecamp-* extensions preserved: $EXTENSION_COUNT"
echo "Tags with x-kind:resource added: $TAG_COUNT"
