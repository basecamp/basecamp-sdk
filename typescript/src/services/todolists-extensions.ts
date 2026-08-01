import { TodolistsService as GeneratedTodolistsService } from "../generated/services/todolists.js";
import type { Todolist } from "../generated/services/todolists.js";
import type { components } from "../generated/schema.js";
import { Errors } from "../errors.js";

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
  // A `group` envelope carries the same writable surface this composite reads
  // (`name`); anything else is the flat recordable the API really sends.
  const flat: unknown = "group" in response ? response.group : response;
  return flat as Todolist;
}

/**
 * Reads a writable string field off a fetched todolist, refusing to pass a
 * malformed one through.
 *
 * An absent field or an explicit `null`/`undefined` is genuinely empty — there
 * is nothing to preserve, and `""` is what the server already holds. Anything
 * else that is not a string is a malformed response and must not be forwarded:
 * `?? ""` only coalesces null and undefined, so a number, boolean, array or
 * object would ride through **verbatim** into the full-replace PUT and
 * overwrite the real value with a corrupted one.
 *
 * The type annotation on the GET result is a compile-time claim about runtime
 * data that nothing validates — `schema.d.ts` is erased at build time, so
 * TypeScript has no decoder that rejects a wrong-typed field the way Go's
 * `json.Unmarshal`, Swift's `Codable`, or kotlinx.serialization do. That puts
 * this composite in the same position as the Python and Ruby ones: the check
 * has to be explicit here. The shipped Todos and Cards analogues are #576.
 */
function writableString(todolist: Todolist, key: "name" | "description"): string {
  const value: unknown = todolist[key];
  if (value === undefined || value === null) return "";
  if (typeof value !== "string") {
    throw Errors.usage(
      `todolist ${key} is not a string: ${JSON.stringify(value)}`,
      "The merge-safe update/edit resend this field verbatim, so a malformed value would " +
        "overwrite the current one. Use replace() to write the record deliberately."
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
      name: writableString(current, "name"),
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
    return unwrapTodolist(
      await this.replace(id, {
        name: f.name,
        description: f.description,
      })
    );
  }
}
