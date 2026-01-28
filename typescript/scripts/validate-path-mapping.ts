#!/usr/bin/env tsx
/**
 * Validates that PATH_TO_OPERATION in client.ts stays in sync with OpenAPI spec.
 *
 * Checks for:
 * - missing: OpenAPI method+path not in mapping
 * - extra: mapping entries not in OpenAPI
 * - wrongOperationId: mapping value doesn't match OpenAPI operationId
 *
 * Exit codes:
 * - 0: all checks pass
 * - 1: validation errors found
 */

import { readFileSync, existsSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));

// Paths relative to script location (try spec/build first, fall back to repo root)
const OPENAPI_SPEC_BUILD = resolve(__dirname, "../../spec/build/smithy/openapi/openapi/Basecamp.openapi.json");
const OPENAPI_REPO_ROOT = resolve(__dirname, "../../openapi.json");
const CLIENT_PATH = resolve(__dirname, "../src/client.ts");

/**
 * Resolves the OpenAPI spec path, preferring spec/build but falling back to repo root.
 */
function resolveOpenAPIPath(): string {
  if (existsSync(OPENAPI_SPEC_BUILD)) {
    return OPENAPI_SPEC_BUILD;
  }
  if (existsSync(OPENAPI_REPO_ROOT)) {
    console.log("Note: Using openapi.json from repo root (spec/build not found)\n");
    return OPENAPI_REPO_ROOT;
  }
  console.error("Error: OpenAPI spec not found.");
  console.error("  Tried: spec/build/smithy/openapi/openapi/Basecamp.openapi.json");
  console.error("  Tried: openapi.json (repo root)");
  console.error("\nRun 'make smithy-build' to generate the spec, or ensure openapi.json exists.");
  process.exit(1);
}

interface OpenAPISpec {
  paths: Record<string, Record<string, { operationId?: string }>>;
}

interface PathEntry {
  method: string;
  path: string;
  canonicalKey: string;
  operationId?: string;
}

/**
 * Canonicalizes a path by replacing {placeholder} with {} so placeholder
 * name differences don't cause false positives.
 */
function canonicalizePath(path: string): string {
  return path.replace(/\{[^}]+\}/g, "{}");
}

/**
 * Creates a canonical key from method and path for comparison.
 */
function makeCanonicalKey(method: string, path: string): string {
  return `${method.toUpperCase()}:${canonicalizePath(path)}`;
}

/**
 * Parses OpenAPI spec and extracts all method+path combinations.
 */
function parseOpenAPI(specPath: string): Map<string, PathEntry> {
  const spec: OpenAPISpec = JSON.parse(readFileSync(specPath, "utf-8"));
  const entries = new Map<string, PathEntry>();

  for (const [path, methods] of Object.entries(spec.paths)) {
    for (const [method, details] of Object.entries(methods)) {
      if (method === "parameters") continue; // Skip shared parameters
      const canonicalKey = makeCanonicalKey(method, path);
      entries.set(canonicalKey, {
        method: method.toUpperCase(),
        path,
        canonicalKey,
        operationId: details.operationId,
      });
    }
  }

  return entries;
}

/**
 * Parses PATH_TO_OPERATION from client.ts using regex.
 */
function parsePathMapping(clientPath: string): Map<string, PathEntry> {
  const content = readFileSync(clientPath, "utf-8");
  const entries = new Map<string, PathEntry>();

  // Match entries like: "GET:/buckets/{projectId}/todos.json": "ListTodos",
  const regex = /"([A-Z]+):([^"]+)":\s*"([^"]+)"/g;
  let match;

  while ((match = regex.exec(content)) !== null) {
    const [, method, path, operationId] = match;
    const canonicalKey = makeCanonicalKey(method, path);
    entries.set(canonicalKey, {
      method,
      path,
      canonicalKey,
      operationId,
    });
  }

  return entries;
}

/**
 * Main validation logic.
 */
function validate(): boolean {
  console.log("Validating PATH_TO_OPERATION against OpenAPI spec...\n");

  const openapiPath = resolveOpenAPIPath();
  const openapi = parseOpenAPI(openapiPath);
  const mapping = parsePathMapping(CLIENT_PATH);

  const missing: PathEntry[] = [];
  const extra: PathEntry[] = [];
  const wrongOperationId: Array<{
    entry: PathEntry;
    expected: string;
    actual: string;
  }> = [];

  // Check for missing entries (in OpenAPI but not in mapping)
  for (const [canonicalKey, entry] of openapi) {
    if (!mapping.has(canonicalKey)) {
      missing.push(entry);
    }
  }

  // Check for extra entries and wrong operation IDs
  for (const [canonicalKey, mappingEntry] of mapping) {
    const openapiEntry = openapi.get(canonicalKey);
    if (!openapiEntry) {
      extra.push(mappingEntry);
    } else if (openapiEntry.operationId && mappingEntry.operationId !== openapiEntry.operationId) {
      wrongOperationId.push({
        entry: mappingEntry,
        expected: openapiEntry.operationId,
        actual: mappingEntry.operationId!,
      });
    }
  }

  // Report results
  let hasErrors = false;

  if (missing.length > 0) {
    hasErrors = true;
    console.log(`❌ MISSING (${missing.length} entries in OpenAPI but not in PATH_TO_OPERATION):`);
    for (const entry of missing.sort((a, b) => a.path.localeCompare(b.path))) {
      console.log(`   ${entry.method}:${entry.path} → ${entry.operationId || "(no operationId)"}`);
    }
    console.log();
  }

  if (extra.length > 0) {
    hasErrors = true;
    console.log(`❌ EXTRA (${extra.length} entries in PATH_TO_OPERATION but not in OpenAPI):`);
    for (const entry of extra.sort((a, b) => a.path.localeCompare(b.path))) {
      console.log(`   ${entry.method}:${entry.path} → ${entry.operationId}`);
    }
    console.log();
  }

  if (wrongOperationId.length > 0) {
    hasErrors = true;
    console.log(`❌ WRONG OPERATION ID (${wrongOperationId.length} mismatches):`);
    for (const { entry, expected, actual } of wrongOperationId) {
      console.log(`   ${entry.method}:${entry.path}`);
      console.log(`      expected: ${expected}`);
      console.log(`      actual:   ${actual}`);
    }
    console.log();
  }

  if (!hasErrors) {
    console.log(`✅ PATH_TO_OPERATION is in sync with OpenAPI spec`);
    console.log(`   ${mapping.size} entries validated`);
  }

  return !hasErrors;
}

// Run validation
const success = validate();
process.exit(success ? 0 : 1);
