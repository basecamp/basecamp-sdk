#!/usr/bin/env node
/**
 * Emits, per operation, every place its 2xx response body holds a `Person`
 * whose id Go decodes as `types.FlexibleInt64`.
 *
 * Go converts a string person id at DECODE: `generated.Person.Id` is
 * `types.FlexibleInt64` (`go/pkg/types/flexible_int64.go:27-65`), and every
 * generated service method reads its body through `Parse<Op>Response` first.
 * TypeScript has no typed decoder, so `services/base.ts` replays that one
 * field's decode at exactly these sites and nowhere else.
 *
 * A site is selected by the `x-go-type` marker on the referenced schema's `id`,
 * NEVER by a property name: `creator`, `assignees`, `participants` and `person`
 * also name people whose id is a plain `int64` in Go
 * (`UpcomingSchedulePerson`, `MyAssignmentAssignee`, `OutOfOfficePerson`), where
 * a string is a decode error rather than a person to convert.
 *
 * Path syntax: dot-separated segments; `[]` is every array element, `{}` every
 * value of a map (`additionalProperties`); `$` is the body itself. A path
 * ending in `[]` is an array of people; any other path is one person.
 *
 * Usage: npx tsx scripts/extract-person-id-sites.ts src/generated/openapi-stripped.json > src/generated/person-id-sites.ts
 */

import * as fs from "fs";
import * as path from "path";

const FLEXIBLE_INT64 = "types.FlexibleInt64";

type Schema = Record<string, any>;

function refName(ref: string): string {
  return ref.split("/").pop()!;
}

function collectSites(spec: Schema): Record<string, string[]> {
  const schemas: Record<string, Schema> = spec.components?.schemas ?? {};
  const responses: Record<string, Schema> = spec.components?.responses ?? {};

  const isFlexiblePerson = (name: string) =>
    schemas[name]?.properties?.id?.["x-go-type"] === FLEXIBLE_INT64;

  function walk(schema: Schema | undefined, at: string[], stack: string[], out: Set<string>): void {
    if (!schema || typeof schema !== "object") return;
    if (typeof schema.$ref === "string") {
      const name = refName(schema.$ref);
      // A recursive schema cannot hold a person the first visit did not find.
      if (stack.includes(name)) return;
      if (isFlexiblePerson(name)) out.add(at.length === 0 ? "$" : at.join("."));
      walk(schemas[name], at, [...stack, name], out);
      return;
    }
    for (const key of ["allOf", "oneOf", "anyOf"]) {
      for (const sub of schema[key] ?? []) walk(sub, at, stack, out);
    }
    if (schema.type === "array" || schema.items) walk(schema.items, [...at, "[]"], stack, out);
    for (const [prop, sub] of Object.entries(schema.properties ?? {})) {
      walk(sub as Schema, [...at, prop], stack, out);
    }
    if (schema.additionalProperties && typeof schema.additionalProperties === "object") {
      walk(schema.additionalProperties, [...at, "{}"], stack, out);
    }
  }

  const sites: Record<string, string[]> = {};
  for (const pathItem of Object.values(spec.paths ?? {}) as Schema[]) {
    for (const method of ["get", "post", "put", "patch", "delete"]) {
      const operation = pathItem[method];
      if (!operation?.operationId) continue;
      const out = new Set<string>();
      for (const [code, raw] of Object.entries(operation.responses ?? {}) as [string, Schema][]) {
        if (!code.startsWith("2")) continue;
        const response = typeof raw.$ref === "string" ? responses[refName(raw.$ref)] : raw;
        for (const media of Object.values(response?.content ?? {}) as Schema[]) {
          walk(media.schema, [], [], out);
        }
      }
      if (out.size > 0) sites[operation.operationId] = [...out].sort();
    }
  }

  return Object.fromEntries(Object.entries(sites).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));
}

const specPath = path.resolve(process.argv[2] || "src/generated/openapi-stripped.json");
if (!fs.existsSync(specPath)) {
  console.error(`Error: OpenAPI file not found: ${specPath}`);
  process.exit(1);
}

const spec = JSON.parse(fs.readFileSync(specPath, "utf-8"));
const sites = collectSites(spec);
if (Object.keys(sites).length === 0) {
  // The marker is the only selector. Losing it (a stripping step, a spec
  // rename) would silently turn the decode off everywhere.
  console.error(`ERROR: no response reaches a schema whose id is ${FLEXIBLE_INT64} — has the x-go-type marker moved?`);
  process.exit(1);
}

console.log(`// Generated from OpenAPI by scripts/extract-person-id-sites.ts. Do not edit by hand.
//
// Operation id -> the paths in its response body where Go decodes a person id as
// types.FlexibleInt64. "[]" is every array element, "{}" every map value, "$" the
// body itself; a path ending in "[]" is an array of people.

export const PERSON_ID_SITES: Readonly<Record<string, readonly string[]>> = ${JSON.stringify(sites, null, 2)};
`);
