#!/usr/bin/env npx tsx
/**
 * Post-processes OpenAPI spec to remove {accountId} from paths.
 *
 * The Basecamp API includes accountId in URL paths, but the SDK's baseUrl
 * already includes the account ID, so we strip it from paths to:
 * 1. Simplify the generated types
 * 2. Match how the services actually call the API
 *
 * Usage: npx tsx scripts/strip-account-id.ts <input.json> <output.json>
 */

import * as fs from "fs";
import * as path from "path";

interface OpenAPISpec {
  openapi: string;
  info: Record<string, unknown>;
  paths: Record<string, PathItem>;
  components: Record<string, unknown>;
  [key: string]: unknown;
}

interface PathItem {
  [method: string]: Operation | undefined;
}

interface Operation {
  parameters?: Parameter[];
  [key: string]: unknown;
}

interface Parameter {
  name: string;
  in: string;
  [key: string]: unknown;
}

function stripAccountId(spec: OpenAPISpec): OpenAPISpec {
  const newPaths: Record<string, PathItem> = {};

  for (const [pathKey, pathItem] of Object.entries(spec.paths)) {
    // Strip /{accountId} prefix from path
    const newPathKey = pathKey.replace(/^\/{accountId}/, "");

    // Process each method in the path
    const newPathItem: PathItem = {};
    for (const [method, operation] of Object.entries(pathItem)) {
      if (!operation || typeof operation !== "object") {
        newPathItem[method] = operation;
        continue;
      }

      // Remove accountId from parameters array
      const newOperation = { ...operation };
      if (Array.isArray(newOperation.parameters)) {
        newOperation.parameters = newOperation.parameters.filter(
          (p: Parameter) => !(p.name === "accountId" && p.in === "path")
        );
      }

      newPathItem[method] = newOperation;
    }

    newPaths[newPathKey] = newPathItem;
  }

  return {
    ...spec,
    paths: newPaths,
  };
}

function main() {
  const args = process.argv.slice(2);

  if (args.length < 2) {
    console.error("Usage: npx tsx scripts/strip-account-id.ts <input.json> <output.json>");
    process.exit(1);
  }

  const inputPath = path.resolve(args[0]);
  const outputPath = path.resolve(args[1]);

  if (!fs.existsSync(inputPath)) {
    console.error(`Error: Input file not found: ${inputPath}`);
    process.exit(1);
  }

  const spec: OpenAPISpec = JSON.parse(fs.readFileSync(inputPath, "utf-8"));
  const stripped = stripAccountId(spec);

  fs.writeFileSync(outputPath, JSON.stringify(stripped, null, 2));
  console.log(`Stripped {accountId} from ${Object.keys(spec.paths).length} paths`);
  console.log(`Output written to ${outputPath}`);
}

main();
