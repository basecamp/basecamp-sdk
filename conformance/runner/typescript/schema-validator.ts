/**
 * OpenAPI response-body schema validation for the live canary.
 *
 * The validator operates on raw JSON (per §5b/5f of the plan) — never on
 * SDK-decoded structures, since language-specific decoders silently drop
 * unknown fields and we need the canary to surface them.
 *
 * Rules per §5b:
 *   - additionalProperties permissive (forward-compat must not break the canary)
 *   - required strict
 *   - type/format/nullable per OpenAPI
 *   - $ref resolved against openapi.json
 *   - failure reports include path, expected schema, actual value
 *   - extras collected per-run as "fields seen but not modeled"
 */

import Ajv, { type ValidateFunction, type ErrorObject } from "ajv";
import addFormats from "ajv-formats";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const OPENAPI_PATH = path.resolve(__dirname, "../../../openapi.json");

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
    // Required strict (default), additionalProperties permissive (default false
    // for openapi-generated schemas means "no extras allowed"; we explicitly
    // override at validation time to permit forward-compat additions).
  });
  addFormats(ajv);

  const raw = fs.readFileSync(OPENAPI_PATH, "utf-8");
  openapi = JSON.parse(raw) as OpenAPIDocument;

  // Register the openapi document so $refs resolve.
  ajv.addSchema(
    {
      $id: "openapi",
      ...openapi,
    } as object,
    "openapi.json",
  );

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
 * Strip `additionalProperties: false` from a schema tree so that extra fields
 * on the wire (forward-compat additions from BC5) do not fail validation.
 * Required-field checks still apply.
 */
function permitExtras(schema: unknown): unknown {
  if (!schema || typeof schema !== "object") return schema;
  if (Array.isArray(schema)) return schema.map(permitExtras);
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(schema as Record<string, unknown>)) {
    if (key === "additionalProperties" && value === false) continue;
    out[key] = permitExtras(value);
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
      const permissive = permitExtras(schema);
      validator = ajv.compile(permissive as object);
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
  const extras = collectExtras(operationId, body, doc);
  return { ok, errors, extras };
}

function formatError(err: ErrorObject): string {
  const path = err.instancePath || "(root)";
  const expected = err.schemaPath ? ` (schema ${err.schemaPath})` : "";
  return `${path}: ${err.message}${expected}`;
}

/**
 * Collect dotted-path field names that appear on the wire body but are not
 * declared in the operation's response schema. Top-level scan only — nested
 * extras are reported when their parent property's schema permits them.
 */
function collectExtras(
  operationId: string,
  body: unknown,
  doc: OpenAPIDocument,
): string[] {
  const schema = findResponseSchema(doc, operationId);
  if (!schema || typeof body !== "object" || body === null) return [];
  const props = readObjectProperties(schema, doc);
  if (!props) return [];

  const declared = new Set(Object.keys(props));
  const extras: string[] = [];
  for (const key of Object.keys(body as Record<string, unknown>)) {
    if (!declared.has(key)) extras.push(key);
  }
  return extras;
}

function readObjectProperties(
  schema: unknown,
  doc: OpenAPIDocument,
): Record<string, unknown> | null {
  if (!schema || typeof schema !== "object") return null;
  const s = schema as Record<string, unknown>;
  if (typeof s["$ref"] === "string") {
    const refName = (s["$ref"] as string).replace("#/components/schemas/", "");
    return readObjectProperties(doc.components?.schemas?.[refName], doc);
  }
  if (s.type === "object" && s.properties && typeof s.properties === "object") {
    return s.properties as Record<string, unknown>;
  }
  return null;
}
