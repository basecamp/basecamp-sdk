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
import { personIdNumber, scanPersonId } from "../person-id.js";

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
 * Reads a writable string the record is *required* to carry.
 *
 * {@link writableString} treats an absent key or an explicit `null` as
 * genuinely empty, which is right for an optional field — `""` is what the
 * server already holds. It is wrong for a required one. Where the spec marks a
 * response member `@required` and BC3 can never render it blank, an absent,
 * null or blank value in a 2xx body is a **malformed response**, not an empty
 * field. Coalescing it to `""` and sending that in the full-replace PUT would
 * blank the real value on a call that never mentioned it — #576's defect
 * exactly.
 *
 * Two records rely on this today and for the same reason: `Document#title` is
 * `super.presence || "Untitled"` and `Schedule::Entry#summary` is
 * `super.presence || "Untitled"`, so neither can come back blank from a healthy
 * server.
 *
 * The wrong-type branch is delegated to {@link writableString}, so a required
 * field and an optional one report a non-string identically.
 */
export function requiredWritableString(
  body: Record<string, unknown>,
  key: string,
  opts: { record: string; escape: string }
): string {
  const value = body[key];
  if (value === undefined || value === null || (typeof value === "string" && value.trim() === "")) {
    throw malformedResponse(
      `${opts.record} field "${key}" is required but the response carried ${describeValue(value)}`,
      `The merge-safe update/edit resend this field verbatim, so a missing or blank value would ` +
        `blank the current one. Use ${opts.escape} to write the record deliberately.`
    );
  }
  return writableString(body, key, opts);
}

/**
 * Reads a writable boolean the record is *required* to carry.
 *
 * The boolean analogue of {@link requiredWritableString}, and it cannot be
 * expressed with a truthiness test: the value this guard most needs to admit is
 * `false`, which every `||`/`if (!x)` idiom would treat as missing and replace
 * with a default. `ScheduleEntry.all_day` is `NOT NULL` with a `false` default
 * in BC3 and every partial emits it, so absent or null is a malformed
 * response — and defaulting it to `false` would silently convert an all-day
 * event into a midnight-to-midnight timed one on a call that only changed the
 * summary.
 *
 * `0`/`1` are refused rather than coerced, for the same reason
 * {@link writableString} refuses `42`: JSON has a boolean type and the server
 * uses it.
 */
export function requiredWritableBoolean(
  body: Record<string, unknown>,
  key: string,
  opts: { record: string; escape: string }
): boolean {
  const value = body[key];
  if (value === undefined || value === null) {
    throw malformedResponse(
      `${opts.record} field "${key}" is required but the response carried ${describeValue(value)}`,
      `The merge-safe update/edit resend this field verbatim, so a missing value would replace ` +
        `the current one with a default. Use ${opts.escape} to write the record deliberately.`
    );
  }
  if (typeof value !== "boolean") {
    throw malformedResponse(
      `${opts.record} field "${key}" is not a boolean: ${describeValue(value)}`,
      resendHint(opts.escape)
    );
  }
  return value;
}

/**
 * Reads an *optional* writable boolean, refusing to coerce a malformed one.
 *
 * {@link writableString}'s boolean sibling, standing in the same relation to
 * {@link requiredWritableBoolean} that `writableString` does to
 * {@link requiredWritableString}: an absent key or an explicit `null` is
 * genuinely "not set" and returns `false`, because that is what the server
 * already holds.
 *
 * `ScheduleEntry.highlighted` is the case it exists for. The entry partial
 * emits it unconditionally, but the reduced calendar partial behind
 * `GetUpcomingSchedule` does not, and both render through the same schema — so
 * the member is optional and absence is legitimate rather than malformed.
 *
 * What still cannot be tolerated is the *wrong type*: a `"yes"` or a `1` must be
 * refused, not coerced, because a caller who assigns the seeded value straight
 * back sends whatever it was seeded with. That branch is delegated to
 * {@link requiredWritableBoolean}, so an optional boolean and a required one
 * report a non-boolean identically.
 */
export function writableBoolean(
  body: Record<string, unknown>,
  key: string,
  opts: { record: string; escape: string }
): boolean {
  const value = body[key];
  if (value === undefined || value === null) return false;
  return requiredWritableBoolean(body, key, opts);
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
    // A null element is 0, not a malformed one. The reference decodes a JSON
    // null in this array into its zero `Person`, whose `Id` is 0, and
    // `fieldsFromTodo` appends that like any other id — measured through the
    // reference's own Update composite.
    if (element === null) return 0;
    if (typeof element !== "object" || Array.isArray(element)) {
      throw malformedResponse(
        `${opts.record} field "${key}"[${index}] is not an object: ${describeValue(element)}`,
        resendHint(opts.escape)
      );
    }
    const record = element as Record<string, unknown>;
    // An ABSENT id is the zero value with no error, while an explicit null
    // FAILS the read. The reference's flexible decoder is only called for a
    // value that is there; for a JSON null it IS called, its number path leaves
    // the buffer empty, and `ParseInt("")` fails. Two different answers, so the
    // two cases cannot be tested together with `id == null`.
    if (!("id" in record)) return 0;
    const id = record["id"];
    const refuse = (): never => {
      throw malformedResponse(
        `${opts.record} field "${key}"[${index}].id is not a person id: ${describeValue(id)}`,
        resendHint(opts.escape)
      );
    };
    // A NUMBER id. `Number.isInteger` is the test rather than `typeof`: `1.5`
    // and `NaN` are numbers and neither is a person id, and a boolean fails it
    // outright, which is what we want — JavaScript would coerce `true` to 1
    // downstream. `Number.isSafeInteger` is the second half and is the same
    // judgment `personIdNumber` makes for a string: past 2^53 `JSON.parse` has
    // already rounded, so the only honest answers are "unreadable" or a wrong
    // id, and the reference refuses this row anyway once it is past int64.
    if (typeof id === "number") return Number.isSafeInteger(id) ? id : refuse();
    // A STRING id. BC3 serializes person ids as strings in some responses, and
    // this reader must not lean on the pre-decode normalizer having converted
    // one: which keys that walk covers is the walk's rule, and it is being held
    // to the reference's own positional surfaces. The reference has no gap
    // here either way, because it covers this site with a DECODER — `Person.Id`
    // is `types.FlexibleInt64`, and `fieldsFromTodo` appends what that produced
    // without filtering. Same grammar as the walk, so the two cannot disagree
    // about a person depending on which of them saw it first.
    if (typeof id === "string") {
      const scan = scanPersonId(id);
      // A SYNTAX refusal is the system actor, not an error: that is what the
      // reference reads `"basecamp"` as, and the RANGE refusal beside it fails
      // the read. Which one a string earns is decided inside the scan, so they
      // cannot be told apart after the fact by looking at the string.
      if (scan.kind === "syntax") return 0;
      if (scan.kind === "range") return refuse();
      // Outside ±(2^53 - 1) the number is already rounded, so refusing the ID
      // is the local resolution `personIdNumber` documents — a residual
      // divergence from a reference that answers in `int64`, recorded rather
      // than papered over with a wrong id.
      return personIdNumber(scan.value) ?? refuse();
    }
    // Everything else is a decode failure at the reference, an explicit
    // `"id": null` included: the flexible decoder IS called for a JSON null,
    // its number path leaves the buffer empty, and `ParseInt("")` fails. An
    // absent key never reaches here, and is 0 — the two cannot be tested
    // together with `id == null`.
    return refuse();
  });
}
