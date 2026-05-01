#!/usr/bin/env bash
# Validate spec/api-gaps/*.md frontmatter + required body sections.
# Also validates spec/api-gaps/allowlist.yml against allowlist-schema.json.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

if ! command -v ruby >/dev/null 2>&1; then
  echo "ERROR: ruby is required for validate-api-gaps" >&2
  exit 2
fi

exec ruby "$SCRIPT_DIR/validate-api-gaps.rb" "$PROJECT_ROOT/spec/api-gaps"
