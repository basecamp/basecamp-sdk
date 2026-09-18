#!/usr/bin/env node
/**
 * Extracts x-basecamp-* extensions from OpenAPI spec into a runtime-accessible metadata module.
 * The generated ESM module keeps operation metadata in the static import graph for bundlers and serverless file tracing.
 *
 * Usage: npx tsx extract-metadata.ts ../openapi.json > src/generated/metadata.ts
 */

import * as fs from "fs";
import * as path from "path";
import { operationsOf } from "./path-items.js";

interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  backoff: "exponential" | "linear" | "constant";
  retryOn: number[];
}

interface PaginationConfig {
  style: "link" | "cursor";
  totalCountHeader?: string;
  maxPageSize?: number;
  key?: string;
}

interface IdempotentConfig {
  keySupported?: boolean;
  keyHeader?: string;
  natural?: boolean;
}

interface OperationMetadata {
  retry?: RetryConfig;
  pagination?: PaginationConfig;
  idempotent?: IdempotentConfig;
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

  // Verb-agnostic by construction: metadata is keyed by operationId, so this
  // walk takes no emission bound and extracts an operation on any verb.
  for (const [pathKey, pathItem] of Object.entries(openapi.paths || {})) {
    for (const [_method, operation] of operationsOf(pathKey, pathItem)) {
      const operationId = operation.operationId as string;

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

const json = JSON.stringify(metadata, null, 2);
console.log(`// Generated from OpenAPI x-basecamp-* extensions. Do not edit by hand.

export interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  backoff: "exponential" | "linear" | "constant";
  retryOn: number[];
}

export interface PaginationConfig {
  style: "link" | "cursor";
  totalCountHeader?: string;
  maxPageSize?: number;
  key?: string;
}

export interface IdempotentConfig {
  keySupported?: boolean;
  keyHeader?: string;
  natural?: boolean;
}

export interface OperationMetadata {
  retry?: RetryConfig;
  pagination?: PaginationConfig;
  idempotent?: IdempotentConfig;
}

export interface MetadataOutput {
  $schema: string;
  version: string;
  generated: string;
  operations: Record<string, OperationMetadata>;
}

const metadata: MetadataOutput = ${json};

export default metadata;
`);
