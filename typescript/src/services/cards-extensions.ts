import { CardsService as GeneratedCardsService } from "../generated/services/cards.js";
import type { Card } from "../generated/services/cards.js";
import { requireRecord, writableString } from "./merge-safe.js";

/** The deliberate-overwrite escape hatch named in this composite's error hints. */
const ESCAPE = "updateVerbatim()";

/**
 * Request parameters for `update`.
 *
 * Every field is presence-bearing: omitting it leaves that part of the card
 * alone. `dueOn` additionally accepts `null`, which is how you ask for the due
 * date to be *cleared* — a distinction the raw API cannot express, because
 * there an absent `due_on` already means "clear".
 */
export interface UpdateCardRequest {
  /** Card title. Omit to leave unchanged. */
  title?: string;
  /** Card body (HTML). Omit to leave unchanged; `""` clears it. */
  content?: string;
  /**
   * Due date (YYYY-MM-DD).
   *
   * - omitted → the current due date is preserved
   * - `null` → the due date is cleared
   * - a date → the due date is set
   */
  dueOn?: string | null;
  /**
   * Person IDs assigned to the card. Omit to leave the assignees alone; `[]`
   * clears them.
   *
   * Assignees are never resent on your behalf. BC3 filters incoming IDs
   * through `reachable_people`, so echoing back an ID belonging to someone who
   * has since lost board access would silently unassign them.
   */
  assigneeIds?: number[];
}

/**
 * CardsService with a merge-safe `update` on top of the generated surface
 * (`get`, `updateVerbatim`, `move`, ...).
 *
 * BC3 builds the card's update params as `{ due_on: nil }.merge(card_params)`
 * (`kanban/cards_controller.rb`), so **any** update whose body omits `due_on`
 * erases the card's due date. A sparse PUT — the natural thing to write — is
 * therefore destructive on the raw endpoint.
 *
 * `update` composes the public `get` and `updateVerbatim` methods, so hooks
 * observe the two real wire operations rather than a synthetic composite.
 */
export class CardsService extends GeneratedCardsService {
  /**
   * Updates a card without disturbing fields you did not mention.
   *
   * Fetches the card first and resends its existing due date when you left
   * `dueOn` unaddressed — so the extra GET is paid for only in the case where
   * the API would otherwise destroy something. Naming `dueOn` explicitly (a
   * date, or `null` to clear) skips the fetch entirely.
   *
   * Not atomic: a concurrent due-date change landing between the GET and the
   * PUT is overwritten with the value this call read. The window is one
   * round-trip. Use `updateVerbatim` to send a single request and manage
   * `due_on` yourself.
   *
   * @param cardId - The card ID
   * @param req - Fields to set; omitted fields are preserved
   * @returns The updated Card
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * // Retitle without losing the due date.
   * await client.cards.update(123, { title: "New title" });
   *
   * // Clear the due date.
   * await client.cards.update(123, { dueOn: null });
   * ```
   */
  async update(cardId: number, req: UpdateCardRequest): Promise<Card> {
    const body: {
      title?: string;
      content?: string;
      dueOn?: string;
      assigneeIds?: number[];
    } = {};

    if (req.title !== undefined) body.title = req.title;
    if (req.content !== undefined) body.content = req.content;
    if (req.assigneeIds !== undefined) body.assigneeIds = req.assigneeIds;

    if (req.dueOn === undefined) {
      // Unaddressed: preserve whatever is there now.
      //
      // The fetched date is validated before it is resent. `if (current.due_on)`
      // was the worst instance of #576 and failed in both directions at once:
      // a falsey non-string was DROPPED, and an omitted `due_on` is exactly how
      // BC3 erases a card's due date — the behaviour this composite exists to
      // prevent — while a truthy non-string (`42`, `true`, `["x"]`, `{a:1}`)
      // was forwarded verbatim and written to the card. Nothing below this
      // rejects either: `schema.d.ts` is erased at build time, so `Card` is a
      // compile-time claim about runtime data. See `merge-safe.ts`.
      const current = requireRecord(await this.get(cardId), {
        record: "Card",
        operation: "GetCard",
        escape: ESCAPE,
      });
      // The Card response carries wire-shaped keys; the request builder takes
      // camelCase. An absent or null date stays absent — omission is how the
      // API expresses "no due date", and `""` is not a date it accepts.
      const dueOn = writableString(current, "due_on", { record: "Card", escape: ESCAPE });
      if (dueOn !== "") body.dueOn = dueOn;
    } else if (req.dueOn !== null) {
      body.dueOn = req.dueOn;
    }
    // req.dueOn === null falls through with dueOn unset: an omitted due_on is
    // how the API clears the date, so omission IS the clear encoding. Sending
    // `{"due_on": null}` would violate body compaction (SPEC §18).

    return this.updateVerbatim(cardId, body);
  }
}
