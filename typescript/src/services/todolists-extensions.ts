import { TodolistsService as GeneratedTodolistsService } from "../generated/services/todolists.js";
import type { Todolist } from "../generated/services/todolists.js";
import type { components } from "../generated/schema.js";
import { Errors, truncateErrorMessage } from "../errors.js";

/**
 * The response shape the generated `get` and `replace` are typed with — both
 * `GetTodolistOrGroupResponseContent` and `UpdateTodolistOrGroupResponseContent`
 * alias this union.
 */
type TodolistOrGroup = components["schemas"]["TodolistOrGroup"];

/**
 * Unwraps a `/{accountId}/todolists/{id}` response into the flat recordable.
 *
 * The generated signature is the Smithy union `{ todolist } | { group }`, but
 * that envelope is a modelling convention, not the wire shape: BC3 answers
 * this route with the recordable's own flat JSON. See AGENTS.md, "Smithy Spec
 * vs Actual API Responses"; the Go SDK carries the same note on
 * `TodolistsService.Get` (`go/pkg/basecamp/todolists.go`), where it decodes
 * the raw body instead of the modelled envelope. Both shapes are handled here
 * — an envelope, should one ever arrive, and the flat body that actually does.
 */
function unwrapTodolist(response: TodolistOrGroup): Todolist {
  if ("todolist" in response) return response.todolist;
  if ("group" in response) {
    // Refused, not converted. The `group` projection models no `description`
    // at all, so deriving a todolist from it would invent an empty one — and
    // on a full-replace endpoint that empty value is written back, erasing the
    // description. That is the same erasure Swift's unwrapper throws on; TS
    // must not silently do what Swift refuses.
    throw malformedResponse(
      "GetTodolistOrGroup returned a todolist group projection, which carries no description",
      "Writing it back would erase the description. Use replace() to overwrite deliberately."
    );
  }
  // The flat recordable the API really sends.
  return response as unknown as Todolist;
}

/**
 * Validates a caller-supplied writable value, the mirror of the read step.
 *
 * `writableString` owns *response* provenance; this owns *caller* provenance,
 * and the two are one rule seen from opposite ends. `edit` hands the caller a
 * mutable `TodolistFields` and the annotation is erased at build time, so a
 * closure assigning `42` or `[]` — trivially reachable from plain JS, or from
 * TypeScript via `as any` — would otherwise walk straight into the full-replace
 * PUT and write it. That is caller misuse, hence `usage`, where the same wrong
 * type arriving from the server is `api_error`.
 */
function callerString(fields: TodolistFields, key: "name" | "description"): string {
  const value: unknown = fields[key];
  if (typeof value !== "string") {
    throw Errors.usage(
      truncateErrorMessage(
        `todolist ${key} must be a string, got ${typeof value}: ${JSON.stringify(value)}`
      ),
      "The full writable state is PUT back verbatim, so a non-string would be written to the " +
        'record. Assign a string; use "" to clear.'
    );
  }
  return value;
}

/** Builds the malformed-response error, with the message capped per SPEC §9. */
function malformedResponse(message: string, hint: string) {
  // api_error, not usage: the value arrived in a successful API response, so
  // nothing the caller passed is at fault. Statusless — there is no HTTP status
  // to attribute, the transport succeeded — and non-retryable, because
  // re-requesting cannot repair a malformed body.
  return Errors.apiError(truncateErrorMessage(message), undefined, {
    hint,
    retryable: false,
  });
}

/**
 * Reads a writable string field off a fetched todolist, refusing to pass a
 * malformed one through.
 *
 * **Classification is by origin, not by value.** The same empty string is a
 * caller error when the caller passed it and malformed response data when it
 * came off the wire, so each provenance is checked where it is unambiguous:
 * this read step owns the response, and `putFields` owns the caller. That is
 * why an empty `name` here is an `api_error` while an empty `name` the caller
 * supplied is a `UsageError` — same value, different origin, different fault.
 *
 * `required` fields (those the schema marks non-nullable, as `name` is) must
 * arrive as a non-empty string: absent, `null`, and `""` are all malformed,
 * because BC3 presence-validates `name` so no real todolist has one. Optional
 * fields (`description`) treat absent and `null` as genuinely empty — there is
 * nothing to preserve and `""` is what the server already holds.
 *
 * Anything of the wrong type is malformed either way: `?? ""` only coalesces
 * null and undefined, so a number, boolean, array or object would ride through
 * **verbatim** into the full-replace PUT and overwrite the real value.
 *
 * The type annotation on the GET result is a compile-time claim about runtime
 * data that nothing validates — `schema.d.ts` is erased at build time, so
 * TypeScript has no decoder that rejects a wrong-typed field the way Go's
 * `json.Unmarshal`, Swift's `Codable`, or kotlinx.serialization do. That puts
 * this composite in the same position as the Python and Ruby ones: the check
 * has to be explicit here. The shipped Todos and Cards analogues are #576.
 */
function writableString(
  todolist: Todolist,
  key: "name" | "description",
  required = false
): string {
  const value: unknown = todolist[key];
  if (value === undefined || value === null) {
    if (required) {
      throw malformedResponse(
        `todolist ${key} is missing from the response`,
        `${key} is required and presence-validated server-side, so a todolist without one is a ` +
          "malformed response, not an empty value to preserve."
      );
    }
    return "";
  }
  if (typeof value !== "string") {
    throw malformedResponse(
      `todolist ${key} is not a string: ${JSON.stringify(value)}`,
      "The merge-safe update/edit resend this field verbatim, so a malformed value would " +
        "overwrite the current one. Use replace() to write the record deliberately."
    );
  }
  if (required && value === "") {
    throw malformedResponse(
      `todolist ${key} is empty in the response`,
      `${key} is presence-validated server-side, so an empty one is a malformed response. ` +
        "The caller did not ask to clear it."
    );
  }
  return value;
}

/**
 * Request parameters for update. Both fields are optional: an omitted field
 * is left untouched on the todolist, guaranteed. An explicitly-passed empty
 * string is a set (clears the description).
 */
export interface UpdateTodolistRequest {
  /** Display name. Omit to leave unchanged. */
  name?: string;
  /** Rich text description (HTML). Omit to leave unchanged; `""` clears it. */
  description?: string;
}

/**
 * A todolist's full writable state, handed to the `edit` callback. The whole
 * object is PUT back to the server, so clearing the description means setting
 * it empty (`""`) — there is no third state.
 */
export interface TodolistFields {
  /** Display name (required; the server rejects a missing one with a 422). */
  name: string;
  /** Rich text description (HTML). Set `""` to clear. */
  description: string;
}

/**
 * TodolistsService with merge-safe `update` and read-modify-write `edit` on
 * top of the generated surface (`get`, `replace`, ...).
 *
 * `PUT /{accountId}/todolists/{id}` is a full replace: BC3's
 * `TodolistsController#update` rebuilds the recordable from only the permitted
 * params, so a sparse PUT that omits `description` erases it, and one that
 * omits `name` is a 422 — the attribute is presence-validated server-side, so
 * omitting it is never a preserve. The writable set is exactly
 * `{name, description}`.
 *
 * Both methods compose the public `get` and `replace` methods, so hooks
 * observe the two wire operations (`GetTodolistOrGroup` then
 * `UpdateTodolistOrGroup`), not a synthetic composite.
 */
export class TodolistsService extends GeneratedTodolistsService {
  /**
   * Sets the given fields on a todolist and preserves everything else: GETs
   * the current todolist, overlays the explicitly-set request fields, and PUTs
   * the full representation back. An omitted (`undefined`) field is untouched,
   * guaranteed; an explicitly-passed `""` clears the description.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so a
   * concurrent write between the GET and PUT is overwritten — last write wins
   * for the whole representation. The window is one round-trip. Use `replace`
   * to overwrite deliberately.
   *
   * @param id - The todolist ID
   * @param req - Fields to set; omitted fields are preserved
   * @returns The updated Todolist
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * // Rename without erasing the description.
   * await client.todolists.update(123, { name: "Hardware" });
   * ```
   */
  async update(id: number, req: UpdateTodolistRequest): Promise<Todolist> {
    const fields = await this.currentFields(id);
    if (req.name !== undefined) fields.name = req.name;
    if (req.description !== undefined) fields.description = req.description;
    return this.putFields(id, fields);
  }

  /**
   * Applies a read-modify-write callback to a todolist: GETs the current
   * todolist, hands the callback its full writable representation, and PUTs
   * the whole thing back. Clearing the description means setting it empty
   * (`""`) — an untouched field keeps its current value. If the callback
   * throws (or rejects), the edit aborts and nothing is written.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so a
   * concurrent write between the GET and PUT is overwritten — last write wins
   * for the whole representation. The window is one round-trip. Use `replace`
   * to overwrite deliberately.
   *
   * @param id - The todolist ID
   * @param fn - Callback that mutates the todolist's writable fields in place
   * @returns The updated Todolist
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todolists.edit(123, (t) => {
   *   t.name = `🚨 ${t.name}`;
   *   t.description = ""; // clearing = setting empty on a full object
   * });
   * ```
   */
  async edit(id: number, fn: (t: TodolistFields) => void | Promise<void>): Promise<Todolist> {
    const fields = await this.currentFields(id);
    await fn(fields);
    return this.putFields(id, fields);
  }

  /** Fetches the todolist and derives its full writable state. */
  private async currentFields(id: number): Promise<TodolistFields> {
    const current = unwrapTodolist(await this.get(id));
    return {
      name: writableString(current, "name", true),
      description: writableString(current, "description"),
    };
  }

  /**
   * PUTs the full writable state via `replace`. Both fields are always sent:
   * description included when empty, because on a full-replace endpoint `""`
   * is how a clear is expressed — never JSON null (SPEC §18 body compaction),
   * and never by omission, which would leave the field to the server's own
   * clear-by-default and read as an accident rather than an intent.
   */
  private async putFields(id: number, f: TodolistFields): Promise<Todolist> {
    // The caller's side of the origin rule. `currentFields` already rejected an
    // empty name coming off the wire, so by here an empty one can only have been
    // supplied by the caller — genuine misuse, since BC3 presence-validates the
    // attribute and a full write has no state that clears it. Guarding in the
    // composite rather than leaning on the generated `replace()` also aligns
    // this path with the other five SDKs; `replace()` keeps its own
    // `Errors.validation` for direct callers, where the name really is theirs.
    const name = callerString(f, "name");
    const description = callerString(f, "description");
    if (name === "") {
      throw Errors.usage(
        "todolist name must not be empty",
        "BC3 presence-validates the name, so a full write cannot clear it. Pass a non-empty " +
          "name, or use replace() if you mean to write the record verbatim."
      );
    }
    return unwrapTodolist(await this.replace(id, { name, description }));
  }
}
