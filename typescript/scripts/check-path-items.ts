#!/usr/bin/env node
/**
 * Preflight: walk every path item in the source spec and refuse before the
 * pipeline writes anything.
 *
 * WHY THIS EXISTS. `npm run generate` is five steps, and the first of them
 * overwrites committed artifacts — `strip-account-id.ts` rewrites
 * `src/generated/openapi-stripped.json`, then openapi-typescript rewrites
 * `schema.d.ts` — while the walk that can REFUSE (`operationsOf`, via the two
 * extractors) does not run until steps four and five. A malformed
 * `operationId`, an `additionalOperations` map or an unreadable path-item field
 * therefore refused generation with `src/generated` already partially rewritten.
 *
 * That is the Kotlin data-loss bug in TypeScript. Kotlin DELETED before it could
 * refuse; this pipeline OVERWRITES before it can refuse, which is equally
 * destructive — the invariant is no destructive write of any kind, delete or
 * overwrite, until everything that can refuse has run.
 *
 * Kotlin's fix was to render the whole output in memory and write last. That is
 * not available here: the pipeline's steps are separate processes, and one of
 * them is a third-party binary (openapi-typescript) that writes its own output.
 * So the refusing walk is hoisted in front of the chain instead. It reads the
 * source spec, walks every path item through the same shared helper the
 * extractors use, and exits non-zero before step one runs.
 *
 * Usage: npx tsx scripts/check-path-items.ts ../openapi.json
 */

import * as fs from "fs";
import * as path from "path";
import { operationsOf } from "./path-items.js";

const specPath = path.resolve(process.argv[2] ?? "../openapi.json");

if (!fs.existsSync(specPath)) {
  console.error(`Error: spec not found: ${specPath}`);
  process.exit(1);
}

const spec = JSON.parse(fs.readFileSync(specPath, "utf-8"));
let operations = 0;

// Both places a 3.1 document holds path items, so the preflight cannot pass
// something the extractors would later refuse.
for (const collection of ["paths", "webhooks"] as const) {
  for (const [pathKey, pathItem] of Object.entries(spec[collection] ?? {})) {
    operations += operationsOf(`${collection === "paths" ? "" : "webhooks -> "}${pathKey}`, pathItem)
      .length;
  }
}

// Fail closed on a spec that yields nothing: a truncated file must not pass this
// preflight vacuously and let the pipeline rewrite the committed tree from it.
if (operations === 0) {
  console.error(`Error: ${specPath} declared no operations; refusing to regenerate from it.`);
  process.exit(1);
}

console.log(`path-items: ${operations} operations readable, nothing written yet`);
