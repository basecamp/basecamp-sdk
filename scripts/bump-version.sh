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
  sed "$expr" "$file" > "$tmp" && cat "$tmp" > "$file" && rm "$tmp"
}

echo "Bumping version to: $VERSION"

# 0. MIGRATING.md — promote "# Unreleased" to "# v$VERSION" (or verify the
# no-notes/idempotent states). Runs FIRST so its refusals — a backward bump,
# a rollback — abort before any version file has been rewritten.
"$(dirname "$0")/promote-migrating.sh" "$VERSION"

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

# 9. Python pyproject.toml
sedi "s/^version = \".*\"/version = \"$VERSION\"/" python/pyproject.toml

# 10. Python _version.py
sedi "s/^VERSION = \".*\"/VERSION = \"$VERSION\"/" python/src/basecamp/_version.py

# 11. Rust workspace Cargo.toml — the ONE Rust version constant (the crate
# inherits it with `version.workspace = true`; code reads env!("CARGO_PKG_VERSION")).
# The edit is bounded to the [workspace.package] table: a bare `^version = `
# sed also matches `[package]` and `[dependencies]` lines when the file is
# reordered, and cargo has no built-in setter without cargo-edit.
awk -v want="version = \"$VERSION\"" '
  /^\[/ { intable = ($0 == "[workspace.package]") }
  intable && /^version = "/ { $0 = want }
  { print }
' rust/Cargo.toml > rust/Cargo.toml.tmp && cat rust/Cargo.toml.tmp > rust/Cargo.toml && rm rust/Cargo.toml.tmp

# Sync TypeScript lockfile
echo "Syncing TypeScript lockfile..."
(cd typescript && npm install --package-lock-only --ignore-scripts)

# Sync Ruby lockfile
echo "Syncing Ruby lockfile..."
(cd ruby && bundle install --quiet)

# Sync Python lockfile
echo "Syncing Python lockfile..."
(cd python && uv lock --quiet)

# Sync conformance TypeScript runner lockfile (records the SDK's version
# via its file:../../../typescript link)
echo "Syncing conformance TypeScript runner lockfile..."
(cd conformance/runner/typescript && npm install --package-lock-only --ignore-scripts --silent)

# Sync conformance Ruby and Python runner lockfiles (they record the SDK's
# version via their path deps on ../../../ruby and ../../../python). Both are
# tracked and installed frozen (#670): left stale, the conformance targets'
# installs fail fast instead of rewriting them mid-check (#671). This is the
# sanctioned rewriter, so it stays unfrozen.
echo "Syncing conformance Ruby runner lockfile..."
(cd conformance/runner/ruby && bundle install --quiet)

echo "Syncing conformance Python runner lockfile..."
(cd conformance/runner/python && uv lock --quiet)

# Sync the Rust lockfiles. Both record the SDK's version (the workspace's own
# members in rust/Cargo.lock; the path dep on ../../../rust/basecamp-sdk in the
# conformance runner's). Both are tracked and every CI/make consumer passes
# --locked, so a stale one fails the next build instead of being rewritten
# silently. `-w` limits the update to workspace members; --offline because
# nothing else moves.
if ! command -v cargo >/dev/null 2>&1; then
  echo "ERROR: cargo is required to refresh the Rust lockfiles" >&2
  exit 1
fi
echo "Syncing Rust lockfile..."
(cd rust && cargo update -q -w --offline)

echo "Syncing conformance Rust runner lockfile..."
(cd conformance/runner/rust && cargo update -q -w --offline)

echo "Done. Bumped 11 version files and synced 8 lockfiles to $VERSION."
