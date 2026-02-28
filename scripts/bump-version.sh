#!/usr/bin/env bash
# Bumps SDK version across all language implementations.
# Usage: scripts/bump-version.sh <version>
# Example: scripts/bump-version.sh 0.3.0
set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "Usage: $0 <version>" >&2
  echo "Example: $0 0.3.0" >&2
  exit 1
fi

# Validate semver format (strict)
if ! echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "ERROR: Version must be semver (e.g., 0.3.0)" >&2
  exit 1
fi

# Portable in-place sed: use temp file instead of -i flag
sedi() {
  local expr="$1" file="$2"
  local tmp
  tmp=$(mktemp)
  sed "$expr" "$file" > "$tmp" && mv "$tmp" "$file"
}

echo "Bumping version to: $VERSION"

# 1. Root package.json
sedi "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" package.json

# 2. Go
sedi "s/^const Version = \".*\"/const Version = \"$VERSION\"/" go/pkg/basecamp/version.go

# 3. TypeScript package.json
sedi "s/\"version\": \".*\"/\"version\": \"$VERSION\"/" typescript/package.json

# 4. TypeScript client.ts
sedi "s/^export const VERSION = \".*\"/export const VERSION = \"$VERSION\"/" typescript/src/client.ts

# 5. Ruby
sedi "s/^  VERSION = \".*\"/  VERSION = \"$VERSION\"/" ruby/lib/basecamp/version.rb

# 6. Kotlin build.gradle.kts
sedi "s/^version = \".*\"/version = \"$VERSION\"/" kotlin/sdk/build.gradle.kts

# 7. Kotlin BasecampConfig.kt
sedi "s/const val VERSION = \".*\"/const val VERSION = \"$VERSION\"/" \
  kotlin/sdk/src/commonMain/kotlin/com/basecamp/sdk/BasecampConfig.kt

# 8. Swift BasecampConfig.swift
sedi "s/public static let version = \".*\"/public static let version = \"$VERSION\"/" \
  swift/Sources/Basecamp/BasecampConfig.swift

# Sync TypeScript lockfile
echo "Syncing TypeScript lockfile..."
(cd typescript && npm install --package-lock-only --ignore-scripts 2>/dev/null)

echo "Done. Bumped 8 files to $VERSION."
