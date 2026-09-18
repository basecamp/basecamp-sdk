/**
 * Walking an OpenAPI Path Item Object (basecamp-sdk#925).
 *
 * A path item is read by EXCLUSION. Its non-operation fields are a closed,
 * spec-defined set and its extensions are `x-` prefixed, so every OTHER field
 * is an operation. Enumerating the verbs instead is the defect this replaces:
 * Smithy's `@http` trait takes the method as a free-form string it "will use
 * literally and will perform no validation on", so a model author writing
 * `method: "HEAD"` produced a valid model, a valid openapi.json, and no method
 * on any client — the verb was not in the list, so the operation was stepped
 * over in silence.
 */

import * as fs from "fs";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const HERE = dirname(fileURLToPath(import.meta.url));

// The self-test points this at a crafted declaration to prove the bound is
// sourced from the shared file rather than a private literal; production runs
// never set it.
export const GENERATED_VERBS_FILE =
  process.env.BASECAMP_GENERATED_VERBS ?? resolve(HERE, "../../spec/generated-verbs.json");

const NON_OPERATION_FIELDS = new Set(["summary", "description", "servers", "parameters"]);

function die(message: string): never {
  console.error(message);
  process.exit(1);
}

/**
 * The ordered HTTP methods the SDK generators emit.
 *
 * Read VERBATIM. scripts/check-generated-verbs.rb is the only thing that rejects a malformed declaration, and it is a prerequisite of every *-generate target and a member of `make check`, so nothing gets here without passing it. This loader deliberately performs NO validation: six loaders that each validated disagreed five times in four review rounds, every one of them on invalid input, and each surviving predicate is another chance to disagree.
 */
export function generatedVerbs(): string[] {
  try {
    return JSON.parse(fs.readFileSync(GENERATED_VERBS_FILE, "utf-8")).verbs;
  } catch (error) {
    die(
      `Error: cannot read ${GENERATED_VERBS_FILE}: ${
        error instanceof Error ? error.message : String(error)
      }. Run 'make check-generated-verbs' — it is a prerequisite of every generate target.`
    );
  }
}

/**
 * Every operation in one path item, as [verb, operation] pairs.
 *
 * `emittable` bounds what the caller can RENDER, and is checked after
 * discovery: an operation on any other verb stops the run by name rather than
 * being dropped. Omit it from a caller that is verb-agnostic (the metadata
 * extractors key everything on operationId).
 *
 * Visit order follows `generatedVerbs()` so emitted output stays byte-stable;
 * a verb outside it sorts deterministically to the end by name, which is
 * ordering, not membership.
 */
export function operationsOf<T = Record<string, any>>(
  path: string,
  pathItem: unknown,
  emittable?: readonly string[]
): [string, T][] {
  const order = generatedVerbs();

  if (typeof pathItem !== "object" || pathItem === null || Array.isArray(pathItem)) {
    die(`Error: openapi.json path ${path} is not a path item object.`);
  }
  const item = pathItem as Record<string, unknown>;

  // A `$ref` path item points at operations this walk cannot see without
  // resolving the reference. Skipping it is the same silent under-count the
  // exclusion walk exists to prevent, so refuse instead.
  if ("$ref" in item) {
    die(
      `Error: openapi.json path ${path} is a $ref to ${JSON.stringify(item.$ref)}. Resolving a ` +
        `path-item reference is not implemented, and skipping it would hide every operation ` +
        `behind it from the SDK.`
    );
  }

  // OpenAPI 3.2's `additionalOperations` is a MAP of method to Operation, not an
  // operation. Read as one it carries no operationId, so a verb-agnostic caller
  // would drop every operation inside it without saying so. Refuse by name until
  // the walk learns the map shape.
  if ("additionalOperations" in item) {
    die(
      `Error: openapi.json path ${path} declares \`additionalOperations\`, which OpenAPI 3.2 ` +
        `defines as a map of method to Operation. This walk reads a path-item field as a single ` +
        `operation, so it would drop every operation inside it. Teach the walk the map shape, or ` +
        `take the field out of the spec.`
    );
  }

  const rank = (field: string) => {
    const at = order.indexOf(field);
    return at === -1 ? order.length : at;
  };
  const fields = Object.keys(item)
    .filter((field) => !NON_OPERATION_FIELDS.has(field) && !field.startsWith("x-"))
    .sort((a, b) => rank(a) - rank(b) || (a < b ? -1 : a > b ? 1 : 0));

  return fields.map((field): [string, T] => {
    const operation = item[field];
    if (typeof operation !== "object" || operation === null || Array.isArray(operation)) {
      die(
        `Error: openapi.json path ${path} field ${JSON.stringify(field)} is neither a known ` +
          `non-operation field nor an operation object. If a later OpenAPI version added it, add ` +
          `it to NON_OPERATION_FIELDS with a reason.`
      );
    }
    if (emittable && !emittable.includes(field)) {
      const opId = (operation as Record<string, unknown>).operationId ?? "(no operationId)";
      die(
        `Error: openapi.json declares ${field.toUpperCase()} ${path} (${opId}), and this ` +
          `generator emits only ${emittable.map((v) => v.toUpperCase()).join("/")}. Generating ` +
          `the rest of the SDK without it would drop the operation from every client in silence, ` +
          `which is the failure basecamp-sdk#925 closed. Give the runtime a ${field} helper and ` +
          `add ${JSON.stringify(field)} to spec/generated-verbs.json (read that file first — the ` +
          `other five SDKs need the same helper), or take the operation out of the Smithy model.`
      );
    }
    // An operation has to be IDENTIFIABLE. OpenAPI lets operationId be omitted,
    // and every walker here used to step over one that was — a silent drop of a
    // real operation, which is basecamp-sdk#925 wearing a different field.
    const operationId = (operation as Record<string, unknown>).operationId;
    if (typeof operationId !== "string" || operationId.length === 0) {
      die(
        `Error: openapi.json declares ${field.toUpperCase()} ${path} with no operationId. ` +
          `Everything downstream is keyed by it, and skipping the operation would drop it from ` +
          `the SDK in silence.`
      );
    }
    return [field, operation as T];
  });
}
