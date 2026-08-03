/**
 * Response guards shared by the merge-safe composites.
 *
 * A merge-safe `update`/`edit` GETs a record, reads each writable field, and
 * PUTs the **full** representation back. The endpoint is full-replace, so every
 * value read here is written — including one the caller never mentioned. If the
 * read step coerces or forwards a malformed value instead of refusing it, that
 * value lands on the record.
 *
 * Two failure modes, the same defect wearing different clothes:
 *
 * - **erasure** — a falsey value dropped or coalesced away, wiping the field;
 * - **corruption** — a non-string forwarded verbatim, writing a number,
 *   boolean, array or object where a string belongs.
 *
 * `?? ""` catches only `null` and `undefined`, so it rules out erasure while
 * leaving corruption wide open — every one of `false`, `0`, `[]`, `{}`, `42`,
 * `true`, `["x"]` and `{a:1}` rides through unchanged. Testing only for erasure
 * is what let this class survive five review passes, so both are refused here.
 *
 * **The rule: a composite is safe exactly when a decoder *rejects* a
 * wrong-typed field at runtime — not when a type merely claims one.** Go
 * (`json.Unmarshal`), Swift (`Codable`) and Kotlin (kotlinx.serialization)
 * genuinely refuse. TypeScript's `schema.d.ts` is erased at build time, so the
 * type on a GET result is a compile-time claim nothing validates; structurally
 * this sits with Python and Ruby, not with Go and Swift (#576).
 *
 * Todolists carries its own copy of these guards (#574). #544 flattened the
 * shape those guards read — dropping the envelope-arm rung, not the guards —
 * but did not unify them here. A generated validating layer (#578) is the
 * intended end state for all of them.
 */
import { Errors, truncateErrorMessage, type BasecampError } from "../errors.js";

const resendHint = (escape: string): string =>
  "The merge-safe update/edit resend this field verbatim, so a coerced or empty value " +
  `would overwrite the current one. Use ${escape} to write the record deliberately.`;

/**
 * Renders a value for an error message without ever throwing.
 *
 * The guard's own error path must not fail while explaining a failure.
 * `JSON.stringify` raises `TypeError` on a circular structure, and a value can
 * carry a `toJSON` that throws — either would replace a clean `api_error` with
 * an incidental `TypeError` and lose the diagnosis. The type name is always
 * available; the rendering is a bonus, capped per SPEC §9 and dropped if it
 * fails.
 */
export function describeValue(value: unknown): string {
  const kind = value === null ? "null" : Array.isArray(value) ? "array" : typeof value;
  try {
    const rendered = JSON.stringify(value);
    return rendered === undefined ? kind : `${kind} ${truncateErrorMessage(rendered)}`;
  } catch {
    return kind;
  }
}

/**
 * Builds the malformed-response error, with the message capped per SPEC §9.
 *
 * `api_error`, not `usage`: the value arrived in a successful API response, so
 * nothing the caller passed is at fault. Statusless — the transport succeeded,
 * there is no HTTP status to attribute — and non-retryable, because
 * re-requesting cannot repair a malformed body.
 */
export function malformedResponse(message: string, hint: string): BasecampError {
  return Errors.apiError(truncateErrorMessage(message), undefined, { hint, retryable: false });
}

/**
 * The response must be a JSON object before any field is read.
 *
 * One level up from the malformed-*field* guards: a successful GET can return a
 * scalar, an array, or null. Reading a property off `null` throws a raw
 * `TypeError`, and off an array or a string it silently yields `undefined`,
 * which the field guards would then read as "genuinely empty" and write back —
 * so the envelope needs checking before the fields.
 */
export function requireRecord(
  body: unknown,
  opts: { record: string; operation: string; escape: string }
): Record<string, unknown> {
  if (typeof body !== "object" || body === null || Array.isArray(body)) {
    throw malformedResponse(
      `${opts.operation} returned ${describeValue(body)} where a ${opts.record.toLowerCase()} object was expected`,
      "The merge-safe update/edit read this record's fields before rewriting them, so a " +
        `non-object body cannot be used. Use ${opts.escape} to write the record deliberately.`
    );
  }
  return body as Record<string, unknown>;
}

/**
 * Reads a writable string field, refusing to pass a malformed one through.
 *
 * An absent key or an explicit `null` is genuinely empty — there is nothing to
 * preserve and `""` is what the server already holds. An actual string passes
 * verbatim. Anything else is a malformed response and is refused **before** the
 * PUT, naming the offending field.
 */
export function writableString(
  body: Record<string, unknown>,
  key: string,
  opts: { record: string; escape: string }
): string {
  const value = body[key];
  if (value === undefined || value === null) return "";
  if (typeof value !== "string") {
    throw malformedResponse(
      `${opts.record} field "${key}" is not a string: ${describeValue(value)}`,
      resendHint(opts.escape)
    );
  }
  return value;
}

/**
 * Reads a list of person records and projects it to their integer IDs.
 *
 * The analogue of {@link writableString} for the ID-list fields. The `.map()`
 * it replaces (`(body[key] ?? []).map((p) => p.id)`) has three ways to go wrong
 * on malformed data: a non-array has no `.map` (a raw `TypeError`), a
 * non-object element yields `undefined`, and a non-integer `id` rides through
 * verbatim into the full-replace PUT — the same corruption as a wrong-typed
 * string, one level down.
 *
 * `Number.isInteger` is the test rather than `typeof === "number"`: `1.5` and
 * `NaN` are numbers and neither is a person ID. Booleans fail it outright,
 * which is what we want — JavaScript would happily coerce `true` to `1`
 * downstream.
 */
export function writableIdList(
  body: Record<string, unknown>,
  key: string,
  opts: { record: string; escape: string }
): number[] {
  const value = body[key];
  if (value === undefined || value === null) return [];
  if (!Array.isArray(value)) {
    throw malformedResponse(
      `${opts.record} field "${key}" is not an array: ${describeValue(value)}`,
      resendHint(opts.escape)
    );
  }
  return value.map((element: unknown, index: number) => {
    if (typeof element !== "object" || element === null || Array.isArray(element)) {
      throw malformedResponse(
        `${opts.record} field "${key}"[${index}] is not an object: ${describeValue(element)}`,
        resendHint(opts.escape)
      );
    }
    const id = (element as Record<string, unknown>)["id"];
    if (id === undefined || id === null) {
      throw malformedResponse(
        `${opts.record} field "${key}"[${index}] has no "id"`,
        resendHint(opts.escape)
      );
    }
    if (typeof id !== "number" || !Number.isInteger(id)) {
      throw malformedResponse(
        `${opts.record} field "${key}"[${index}].id is not an integer: ${describeValue(id)}`,
        resendHint(opts.escape)
      );
    }
    return id;
  });
}
