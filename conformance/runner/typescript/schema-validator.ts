/**
 * OpenAPI response-body schema validation for the live canary.
 *
 * Operates on raw JSON (per §5b/5f of the plan) — never on SDK-decoded
 * structures, since language-specific decoders silently drop unknown
 * fields and we need the canary to surface them.
 *
 * Rules:
 *   - additionalProperties permissive (forward-compat must not break the canary)
 *   - required strict
 *   - type/format/nullable per OpenAPI
 *   - $ref rewritten from "#/..." to "openapi.json#/..." so refs in the
 *     compiled response-schema fragment resolve against the registered
 *     OpenAPI document, not the fragment root
 *   - extras collected per-run, walking arrays and nested objects, so
 *     item-level unknown fields on list responses are visible
 */

import Ajv, { type ValidateFunction, type ErrorObject } from "ajv";
import addFormats from "ajv-formats";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const OPENAPI_PATH = path.resolve(__dirname, "../../../openapi.json");
const OPENAPI_KEY = "openapi.json";

interface OpenAPIDocument {
  paths: Record<string, Record<string, OpenAPIOperation>>;
  components?: { schemas?: Record<string, unknown> };
}

interface OpenAPIOperation {
  operationId?: string;
  responses?: Record<string, OpenAPIResponse>;
}

interface OpenAPIResponse {
  content?: Record<string, { schema?: unknown }>;
}

let ajv: Ajv | null = null;
let openapi: OpenAPIDocument | null = null;
const validatorByOperation = new Map<string, ValidateFunction | null>();

function init(): { ajv: Ajv; doc: OpenAPIDocument } {
  if (ajv && openapi) return { ajv, doc: openapi };

  ajv = new Ajv({
    strict: false,
    allErrors: true,
  });
  addFormats(ajv);

  const raw = fs.readFileSync(OPENAPI_PATH, "utf-8");
  openapi = JSON.parse(raw) as OpenAPIDocument;

  // Register the OpenAPI document under a stable key so rewritten refs of
  // the form `openapi.json#/...` resolve against it.
  ajv.addSchema(openapi as object, OPENAPI_KEY);

  return { ajv, doc: openapi };
}

function findResponseSchema(doc: OpenAPIDocument, operationId: string): unknown | null {
  for (const pathItem of Object.values(doc.paths)) {
    for (const op of Object.values(pathItem)) {
      if (op.operationId !== operationId) continue;
      const responses = op.responses ?? {};
      // Prefer 200; fall through to any 2xx success (201, 202, 204, ...);
      // last resort is "default". Operations that return 201 (Create*)
      // shouldn't fall back to "" because their response body still has
      // a schema worth validating.
      const candidates = ["200", "201", "202", "203", "204"];
      for (const code of candidates) {
        if (!responses[code]) continue;
        const schema = responses[code].content?.["application/json"]?.schema;
        if (schema) return schema;
      }
      // Any 2xx key not in the explicit list above (e.g. 205-299).
      for (const [code, response] of Object.entries(responses)) {
        if (!/^2\d\d$/.test(code)) continue;
        const schema = response.content?.["application/json"]?.schema;
        if (schema) return schema;
      }
      // Last resort: "default".
      const defaultSchema =
        responses["default"]?.content?.["application/json"]?.schema;
      if (defaultSchema) return defaultSchema;
    }
  }
  return null;
}

/**
 * Walk the schema tree, doing two things in one pass:
 *   1. Rewrite "$ref": "#/..." to "$ref": "openapi.json#/..." so refs
 *      resolve against the registered OpenAPI doc, not the fragment root.
 *   2. Drop "additionalProperties: false" so forward-compat fields on the
 *      wire don't fail validation. Required-field checks still apply.
 */
function prepareForCompile(schema: unknown): unknown {
  if (!schema || typeof schema !== "object") return schema;
  if (Array.isArray(schema)) return schema.map(prepareForCompile);
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(schema as Record<string, unknown>)) {
    if (key === "additionalProperties" && value === false) continue;
    if (key === "$ref" && typeof value === "string" && value.startsWith("#/")) {
      out[key] = `${OPENAPI_KEY}${value}`;
      continue;
    }
    out[key] = prepareForCompile(value);
  }
  return out;
}

export interface ValidationResult {
  ok: boolean;
  /** Validation errors, formatted for human reading. */
  errors: string[];
  /** Field paths present on the wire that the schema did not declare. */
  extras: string[];
}

/**
 * Validate a single response body against the operation's response schema.
 *
 * For paginated operations, call once per page; the caller unions extras
 * across pages.
 */
export function validateResponse(operationId: string, body: unknown): ValidationResult {
  const { ajv, doc } = init();

  let validator = validatorByOperation.get(operationId);
  if (validator === undefined) {
    const schema = findResponseSchema(doc, operationId);
    if (!schema) {
      validatorByOperation.set(operationId, null);
      validator = null;
    } else {
      const prepared = prepareForCompile(schema);
      validator = ajv.compile(prepared as object);
      validatorByOperation.set(operationId, validator);
    }
  }

  if (!validator) {
    // Distinguish a bodyless success response (e.g. 204 No Content on a
    // delete/update) from a missing operation. The former is structurally
    // valid by design — no schema means no body to validate. The latter
    // is still a hard failure: the operation isn't covered by the spec.
    if (operationHasBodylessSuccessOnly(doc, operationId)) {
      return { ok: true, errors: [], extras: [] };
    }
    return {
      ok: false,
      errors: [`No response schema found for operation ${operationId}`],
      extras: [],
    };
  }

  const ok = validator(body) as boolean;
  const errors = (validator.errors ?? []).map(formatError);
  const schema = findResponseSchema(doc, operationId);
  const extras = schema ? collectExtras("", body, schema, doc) : [];
  return { ok, errors, extras };
}

/**
 * True when the operation declares at least one 2xx success response and
 * none of its 2xx responses carry an `application/json` schema — i.e. the
 * operation is intentionally bodyless (204 No Content, etc).
 */
function operationHasBodylessSuccessOnly(doc: OpenAPIDocument, operationId: string): boolean {
  for (const pathItem of Object.values(doc.paths)) {
    for (const op of Object.values(pathItem)) {
      if (op.operationId !== operationId) continue;
      const responses = op.responses ?? {};
      let hasSuccess = false;
      for (const [code, response] of Object.entries(responses)) {
        if (!/^2\d\d$/.test(code)) continue;
        hasSuccess = true;
        if (response.content?.["application/json"]?.schema) return false;
      }
      return hasSuccess;
    }
  }
  return false;
}

function formatError(err: ErrorObject): string {
  const where = err.instancePath || "(root)";
  const expected = err.schemaPath ? ` (schema ${err.schemaPath})` : "";
  return `${where}: ${err.message}${expected}`;
}

/**
 * Resolve $ref chains until we hit a non-ref schema (or a cycle).
 * One-level resolution misreports valid fields as extras when the schema
 * uses alias chains (e.g. Foo → Bar → Baz).
 */
function resolveRef(schema: unknown, doc: OpenAPIDocument): unknown {
  const seen = new Set<string>();
  let current: unknown = schema;
  while (current && typeof current === "object" && !Array.isArray(current)) {
    const ref = (current as Record<string, unknown>)["$ref"];
    if (typeof ref !== "string") return current;
    if (seen.has(ref)) return current;
    seen.add(ref);
    // Accept both "#/components/schemas/X" and "openapi.json#/components/schemas/X".
    const m = ref.match(/^(?:openapi\.json)?#\/components\/schemas\/(.+)$/);
    if (!m) return current;
    const next = doc.components?.schemas?.[m[1]];
    if (!next) return current;
    current = next;
  }
  return current;
}

/**
 * Walk body + schema together, emitting dotted-path field names that
 * appear on the wire but are not declared in the schema.
 *
 * Conventions:
 *   - Object extras emit at the property path (e.g. `unreads`).
 *   - Array items use [] as the path segment (e.g. `unreads[].some_field`).
 *   - Recurses into known properties so nested extras surface.
 *   - Bounded depth as a cycle guard; OpenAPI schemas are typically shallow.
 */
function collectExtras(
  prefix: string,
  body: unknown,
  schema: unknown,
  doc: OpenAPIDocument,
  depth = 0,
): string[] {
  if (depth > 12) return [];
  if (body === null || body === undefined) return [];
  const resolved = resolveRef(schema, doc);
  if (!resolved || typeof resolved !== "object") return [];
  const s = resolved as Record<string, unknown>;

  if (Array.isArray(body)) {
    if (s.type !== "array" || !s.items) return [];
    const seen = new Set<string>();
    const childPrefix = prefix ? `${prefix}[]` : "[]";
    for (const item of body) {
      for (const e of collectExtras(childPrefix, item, s.items, doc, depth + 1)) {
        seen.add(e);
      }
    }
    return [...seen];
  }

  if (typeof body !== "object") return [];

  // For non-object schemas (e.g., type: "string"), don't recurse.
  if (s.type !== undefined && s.type !== "object") return [];

  const props = (s.properties as Record<string, unknown> | undefined) ?? {};
  const extras: string[] = [];
  for (const [key, value] of Object.entries(body as Record<string, unknown>)) {
    const fieldPath = prefix ? `${prefix}.${key}` : key;
    if (!(key in props)) {
      extras.push(fieldPath);
    } else {
      for (const e of collectExtras(fieldPath, value, props[key], doc, depth + 1)) {
        extras.push(e);
      }
    }
  }
  return extras;
}
