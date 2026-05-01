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
      const response = op.responses?.["200"] ?? op.responses?.["default"];
      const schema = response?.content?.["application/json"]?.schema;
      if (schema) return schema;
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

function formatError(err: ErrorObject): string {
  const where = err.instancePath || "(root)";
  const expected = err.schemaPath ? ` (schema ${err.schemaPath})` : "";
  return `${where}: ${err.message}${expected}`;
}

/**
 * Resolve a $ref one level. Returns the target schema, or the input
 * unchanged if it isn't a ref.
 */
function resolveRef(schema: unknown, doc: OpenAPIDocument): unknown {
  if (!schema || typeof schema !== "object" || Array.isArray(schema)) return schema;
  const s = schema as Record<string, unknown>;
  const ref = s["$ref"];
  if (typeof ref !== "string") return schema;
  // Accept both "#/components/schemas/X" and "openapi.json#/components/schemas/X".
  const m = ref.match(/^(?:openapi\.json)?#\/components\/schemas\/(.+)$/);
  if (!m) return schema;
  return doc.components?.schemas?.[m[1]] ?? schema;
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
