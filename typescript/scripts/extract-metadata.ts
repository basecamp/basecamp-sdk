#!/usr/bin/env node
/**
 * Extracts x-basecamp-* extensions from OpenAPI spec into a runtime-accessible metadata file.
 * This allows the TypeScript SDK to read operation metadata at runtime for retry, pagination, etc.
 *
 * Usage: npx tsx extract-metadata.ts ../openapi.json > src/generated/metadata.json
 */

import * as fs from "fs";
import * as path from "path";

interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  backoff: "exponential" | "linear" | "constant";
  retryOn: number[];
}

interface PaginationConfig {
  style: "link" | "cursor" | "page";
  pageParam?: string;
  totalCountHeader?: string;
  maxPageSize?: number;
}

interface IdempotentConfig {
  keySupported?: boolean;
  keyHeader?: string;
  natural?: boolean;
}

interface SensitiveField {
  field: string;
  category: string;
  redact: boolean;
}

interface OperationMetadata {
  retry?: RetryConfig;
  pagination?: PaginationConfig;
  idempotent?: IdempotentConfig;
  sensitive?: SensitiveField[];
}

interface MetadataOutput {
  $schema: string;
  version: string;
  generated: string;
  operations: Record<string, OperationMetadata>;
}

function extractMetadata(openapiPath: string): MetadataOutput {
  const openapiContent = fs.readFileSync(openapiPath, "utf-8");
  const openapi = JSON.parse(openapiContent);

  const operations: Record<string, OperationMetadata> = {};

  // Iterate through all paths and operations
  for (const [_pathKey, pathItem] of Object.entries(openapi.paths || {})) {
    const pathObj = pathItem as Record<string, unknown>;

    for (const method of ["get", "post", "put", "patch", "delete"]) {
      const operation = pathObj[method] as Record<string, unknown> | undefined;
      if (!operation) continue;

      const operationId = operation.operationId as string;
      if (!operationId) continue;

      const metadata: OperationMetadata = {};

      // Extract x-basecamp-retry
      if (operation["x-basecamp-retry"]) {
        metadata.retry = operation["x-basecamp-retry"] as RetryConfig;
      }

      // Extract x-basecamp-pagination
      if (operation["x-basecamp-pagination"]) {
        metadata.pagination = operation["x-basecamp-pagination"] as PaginationConfig;
      }

      // Extract x-basecamp-idempotent
      if (operation["x-basecamp-idempotent"]) {
        metadata.idempotent = operation["x-basecamp-idempotent"] as IdempotentConfig;
      }

      // Only add if we found any metadata
      if (Object.keys(metadata).length > 0) {
        operations[operationId] = metadata;
      }
    }
  }

  return {
    $schema: "https://basecamp.com/schemas/sdk-metadata.json",
    version: "1.0.0",
    generated: new Date().toISOString(),
    operations,
  };
}

// Main
const openapiPath = process.argv[2] || "../openapi.json";
const resolvedPath = path.resolve(openapiPath);

if (!fs.existsSync(resolvedPath)) {
  console.error(`Error: OpenAPI file not found: ${resolvedPath}`);
  process.exit(1);
}

const metadata = extractMetadata(resolvedPath);
console.log(JSON.stringify(metadata, null, 2));
