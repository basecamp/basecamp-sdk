import { TodosService as GeneratedTodosService } from "../generated/services/todos.js";
import type { Todo } from "../generated/services/todos.js";

/**
 * Request parameters for update. Every field is optional: an omitted field
 * is left untouched on the todo, guaranteed. An explicitly-passed empty
 * array is a set (clears the list).
 */
export interface UpdateTodoRequest {
  /** Text content */
  content?: string;
  /** Rich text description (HTML) */
  description?: string;
  /** Person IDs to assign to */
  assigneeIds?: number[];
  /** Person IDs to notify on completion */
  completionSubscriberIds?: number[];
  /** Whether to send notifications to relevant people */
  notify?: boolean;
  /** Due date (YYYY-MM-DD) */
  dueOn?: string;
  /** Start date (YYYY-MM-DD) */
  startsOn?: string;
}

/**
 * A todo's full writable state, handed to the `edit` callback. The whole
 * object is PUT back to the server, so clearing a field means setting it
 * empty (`""` for strings and dates, `[]` for ID lists) — there is no
 * third state.
 */
export interface TodoFields {
  /** Text content (required; the server rejects an empty one). */
  content: string;
  /** Rich text description (HTML). Set `""` to clear. */
  description: string;
  /** Complete list of assigned person IDs. Set `[]` to clear. */
  assigneeIds: number[];
  /** Complete list of person IDs notified on completion. Set `[]` to clear. */
  completionSubscriberIds: number[];
  /** Due date (YYYY-MM-DD). Set `""` to clear. */
  dueOn: string;
  /** Start date (YYYY-MM-DD). Set `""` to clear. */
  startsOn: string;
  /**
   * Send directive, not todo state: never populated from the current todo
   * and sent only when true, asking the server to notify assignees about
   * this write.
   */
  notify: boolean;
}

/**
 * TodosService with merge-safe `update` and read-modify-write `edit` on
 * top of the generated surface (`get`, `replace`, ...).
 *
 * Both compose the public `get` and `replace` methods, so hooks observe
 * the two wire operations (`GetTodo` then `ReplaceTodo`), not a synthetic
 * composite.
 */
export class TodosService extends GeneratedTodosService {
  /**
   * Sets the given fields on a todo and preserves everything else: GETs
   * the current todo, overlays the explicitly-set request fields, and PUTs
   * the full representation back. An omitted (`undefined`) field is
   * untouched, guaranteed; an explicitly-passed empty array clears.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so
   * a concurrent write between the GET and PUT is overwritten — last write
   * wins for the whole representation. The window is one round-trip. Use
   * `replace` to overwrite deliberately.
   *
   * @param todoId - The todo ID
   * @param req - Fields to set; omitted fields are preserved
   * @returns The updated Todo
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * // Retitle without touching description, dates, or assignees.
   * await client.todos.update(123, { content: "New title" });
   * ```
   */
  async update(todoId: number, req: UpdateTodoRequest): Promise<Todo> {
    const fields = await this.currentFields(todoId);
    if (req.content !== undefined) fields.content = req.content;
    if (req.description !== undefined) fields.description = req.description;
    if (req.assigneeIds !== undefined) fields.assigneeIds = req.assigneeIds;
    if (req.completionSubscriberIds !== undefined) {
      fields.completionSubscriberIds = req.completionSubscriberIds;
    }
    if (req.dueOn !== undefined) fields.dueOn = req.dueOn;
    if (req.startsOn !== undefined) fields.startsOn = req.startsOn;
    if (req.notify !== undefined) fields.notify = req.notify;
    return this.putFields(todoId, fields);
  }

  /**
   * Applies a read-modify-write callback to a todo: GETs the current todo,
   * hands the callback the full writable representation, and PUTs the
   * whole thing back. Clearing a field means setting it empty (`""` or
   * `[]`) — an untouched field keeps its current value. If the callback
   * throws (or rejects), the edit aborts and nothing is written.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so
   * a concurrent write between the GET and PUT is overwritten — last write
   * wins for the whole representation. The window is one round-trip. Use
   * `replace` to overwrite deliberately.
   *
   * @param todoId - The todo ID
   * @param fn - Callback that mutates the todo's writable fields in place
   * @returns The updated Todo
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.todos.edit(123, (t) => {
   *   t.content = `🚨 ${t.content}`;
   *   t.dueOn = ""; // clearing = setting empty on a full object
   * });
   * ```
   */
  async edit(todoId: number, fn: (t: TodoFields) => void | Promise<void>): Promise<Todo> {
    const fields = await this.currentFields(todoId);
    await fn(fields);
    return this.putFields(todoId, fields);
  }

  /** Fetches the todo and derives its full writable state. */
  private async currentFields(todoId: number): Promise<TodoFields> {
    const current = await this.get(todoId);
    return {
      content: current.content ?? "",
      description: current.description ?? "",
      assigneeIds: (current.assignees ?? []).map((p) => p.id),
      completionSubscriberIds: (current.completion_subscribers ?? []).map((p) => p.id),
      dueOn: current.due_on ?? "",
      startsOn: current.starts_on ?? "",
      notify: false,
    };
  }

  /**
   * PUTs the full writable state via `replace`: content, description, and
   * both ID lists are always sent (empties included, so clears survive);
   * dates are sent only when non-empty (the server clears an omitted date,
   * and `""` is a format error); notify is sent only when true.
   */
  private putFields(todoId: number, f: TodoFields): Promise<Todo> {
    return this.replace(todoId, {
      content: f.content,
      description: f.description,
      assigneeIds: f.assigneeIds,
      completionSubscriberIds: f.completionSubscriberIds,
      dueOn: f.dueOn === "" ? undefined : f.dueOn,
      startsOn: f.startsOn === "" ? undefined : f.startsOn,
      notify: f.notify ? true : undefined,
    });
  }
}
