#!/usr/bin/env tsx
/**
 * Generates PATH_TO_OPERATION mapping from OpenAPI spec.
 *
 * Usage: npx tsx scripts/generate-path-mapping.ts
 *
 * IMPORTANT: This reads from openapi-stripped.json (same source as extract-metadata.ts)
 * to ensure operation IDs match those in metadata.json. The stripped spec has the
 * {accountId} prefix removed, so we add it back when generating the mapping.
 */

import { readFileSync, writeFileSync, existsSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));

// Use openapi-stripped.json - same source as extract-metadata.ts
// This ensures operation IDs match those in metadata.json
const OPENAPI_STRIPPED = resolve(__dirname, "../src/generated/openapi-stripped.json");
const OUTPUT_PATH = resolve(__dirname, "../src/generated/path-mapping.ts");

interface OpenAPISpec {
  paths: Record<string, Record<string, { operationId?: string }>>;
}

interface PathEntry {
  method: string;
  path: string;
  operationId: string;
}

/**
 * Resolves the OpenAPI spec path.
 */
function resolveOpenAPIPath(): string {
  if (existsSync(OPENAPI_STRIPPED)) {
    return OPENAPI_STRIPPED;
  }
  console.error("Error: openapi-stripped.json not found.");
  console.error("  Expected: src/generated/openapi-stripped.json");
  console.error("\nRun the earlier generate steps first (strip-account-id.ts).");
  process.exit(1);
}

/**
 * Parses OpenAPI spec and extracts all method+path+operationId combinations.
 * Adds {accountId} prefix to paths since openapi-stripped.json has it removed.
 */
function parseOpenAPI(specPath: string): PathEntry[] {
  const spec: OpenAPISpec = JSON.parse(readFileSync(specPath, "utf-8"));
  const entries: PathEntry[] = [];

  for (const [path, methods] of Object.entries(spec.paths)) {
    for (const [method, details] of Object.entries(methods)) {
      if (method === "parameters") continue; // Skip shared parameters
      if (!details.operationId) continue;

      // Add {accountId} prefix back - it was stripped by strip-account-id.ts
      const fullPath = `/{accountId}${path}`;

      entries.push({
        method: method.toUpperCase(),
        path: fullPath,
        operationId: details.operationId,
      });
    }
  }

  // Sort by path then method for consistent output
  entries.sort((a, b) => {
    const pathCmp = a.path.localeCompare(b.path);
    if (pathCmp !== 0) return pathCmp;
    return a.method.localeCompare(b.method);
  });

  return entries;
}

/**
 * Groups entries by path prefix for organized output with comments.
 */
function groupByPrefix(entries: PathEntry[]): Map<string, PathEntry[]> {
  const groups = new Map<string, PathEntry[]>();

  for (const entry of entries) {
    // Extract meaningful prefix from path
    const prefix = getPathPrefix(entry.path);
    if (!groups.has(prefix)) {
      groups.set(prefix, []);
    }
    groups.get(prefix)!.push(entry);
  }

  return groups;
}

/**
 * Extracts a meaningful prefix from a path for grouping.
 */
function getPathPrefix(path: string): string {
  // Common patterns to group by
  const patterns: [RegExp, string][] =
    [
      [/\/buckets\/\{projectId\}\/card_tables/, "Card Tables"],
      [/\/buckets\/\{projectId\}\/todolists/, "Todolists"],
      [/\/buckets\/\{projectId\}\/todolist_groups/, "Todolist Groups"],
      [/\/buckets\/\{projectId\}\/todosets/, "Todosets"],
      [/\/buckets\/\{projectId\}\/todos/, "Todos"],
      [/\/buckets\/\{projectId\}\/message_boards/, "Message Boards"],
      [/\/buckets\/\{projectId\}\/messages/, "Messages"],
      [/\/buckets\/\{projectId\}\/comments/, "Comments"],
      [/\/buckets\/\{projectId\}\/categories/, "Message Types"],
      [/\/buckets\/\{projectId\}\/chats/, "Campfires"],
      [/\/buckets\/\{projectId\}\/chatbots/, "Chatbots"],
      [/\/buckets\/\{projectId\}\/vaults/, "Vaults"],
      [/\/buckets\/\{projectId\}\/documents/, "Documents"],
      [/\/buckets\/\{projectId\}\/uploads/, "Uploads"],
      [/\/buckets\/\{projectId\}\/schedules/, "Schedules"],
      [/\/buckets\/\{projectId\}\/schedule_entries/, "Schedule Entries"],
      [/\/buckets\/\{projectId\}\/questionnaires/, "Questionnaires"],
      [/\/buckets\/\{projectId\}\/questions/, "Questions"],
      [/\/buckets\/\{projectId\}\/question_answers/, "Question Answers"],
      [/\/buckets\/\{projectId\}\/recordings/, "Recordings"],
      [/\/buckets\/\{projectId\}\/subscriptions/, "Subscriptions"],
      [/\/buckets\/\{projectId\}\/webhooks/, "Webhooks"],
      [/\/buckets\/\{projectId\}\/client/, "Client Features"],
      [/\/buckets\/\{projectId\}\/inbox/, "Inbox"],
      [/\/buckets\/\{projectId\}\/forwards/, "Forwards"],
      [/\/attachments/, "Attachments"],
      [/\/projects/, "Projects"],
      [/\/people/, "People"],
      [/\/templates/, "Templates"],
      [/\/my/, "My Profile"],
      [/\/events/, "Events"],
      [/\/search/, "Search"],
    ];

  for (const [pattern, name] of patterns) {
    if (pattern.test(path)) {
      return name;
    }
  }

  return "Other";
}

/**
 * Generates the TypeScript source code.
 */
function generateCode(entries: PathEntry[]): string {
  const groups = groupByPrefix(entries);
  const lines: string[] = [];

  lines.push("/**");
  lines.push(" * Maps HTTP method + path to OpenAPI operationId.");
  lines.push(" *");
  lines.push(" * @generated from OpenAPI spec - do not edit directly");
  lines.push(" * Run `npm run generate` to regenerate.");
  lines.push(" */");
  lines.push("");
  lines.push("export const PATH_TO_OPERATION: Record<string, string> = {");

  let first = true;
  for (const [group, groupEntries] of groups) {
    if (!first) {
      lines.push("");
    }
    first = false;

    lines.push(`  // ${group}`);
    for (const entry of groupEntries) {
      lines.push(`  "${entry.method}:${entry.path}": "${entry.operationId}",`);
    }
  }

  lines.push("};");
  lines.push("");

  return lines.join("\n");
}

/**
 * Main entry point.
 */
function main() {
  const openapiPath = resolveOpenAPIPath();
  console.log(`Reading OpenAPI spec from: ${openapiPath}`);

  const entries = parseOpenAPI(openapiPath);
  console.log(`Found ${entries.length} operations`);

  const code = generateCode(entries);
  writeFileSync(OUTPUT_PATH, code);
  console.log(`Generated: ${OUTPUT_PATH}`);
}

main();
