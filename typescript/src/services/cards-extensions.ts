import { CardsService as GeneratedCardsService } from "../generated/services/cards.js";
import type { Card } from "../generated/services/cards.js";

/**
 * Request parameters for `update`.
 *
 * Every field is presence-bearing: omitting it leaves that part of the card
 * alone. `dueOn` additionally accepts `null`, which is how you ask for the due
 * date to be *cleared* — a distinction the raw generated shape cannot express,
 * because its `dueOn` is a plain optional string and absence now means "leave
 * it alone".
 */
export interface UpdateCardRequest {
  /** Card title. Omit to leave unchanged. */
  title?: string;
  /** Card body (HTML). Omit to leave unchanged; `""` clears it. */
  content?: string;
  /**
   * Due date (YYYY-MM-DD).
   *
   * - omitted → the current due date is left unchanged
   * - `null` → the due date is cleared, sent on the wire as `"due_on": ""`
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
 * CardsService with a presence-aware `update` on top of the generated surface
 * (`get`, `updateVerbatim`, `move`, ...).
 *
 * BC3's `kanban/cards_controller.rb` builds a JSON card update's params
 * straight from `card_params` (basecamp/bc3#12521), so on the JSON
 * representation an omitted key means **leave unchanged** — `due_on` included.
 * Clearing the due date is asked for explicitly: the server accepts JSON `null`
 * or `""`, and this SDK sends `""`, because a null would violate body
 * compaction (SPEC §18). Only the HTML/turbo_stream web forms still default an
 * omitted `due_on` to nil (`card_params.with_defaults(due_on: nil)`), which is
 * how the JSON path could be made presence-aware without disturbing the web
 * contract.
 *
 * `update` therefore sends exactly what the caller addressed, in a single PUT.
 * It differs from `updateVerbatim` only in accepting `dueOn: null` and encoding
 * it as the wire spelling of a clear.
 */
export class CardsService extends GeneratedCardsService {
  /**
   * Updates a card without disturbing fields you did not mention.
   *
   * One request. A field you omit is omitted from the body and the server
   * leaves it alone; `dueOn: null` clears the due date.
   *
   * Earlier releases read the card back first and resent its due date, because
   * a JSON update that omitted `due_on` used to erase it. bc3#12521 removed
   * that behaviour, so the extra round-trip — and the race between the read and
   * the write — is gone.
   *
   * @param cardId - The card ID
   * @param req - Fields to set; omitted fields are left unchanged
   * @returns The updated Card
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * // Retitle without touching the due date.
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

    // `due_on` stays off the wire unless the caller addressed it, which the
    // server reads as "leave the due date alone". A date sets it; `null` asks
    // for a clear and is sent as `""` — the server casts the blank to nil (a
    // BC3 server test pins it), while `{"due_on": null}` would violate body
    // compaction (SPEC §18) and omission would now be silently ignored.
    if (req.dueOn !== undefined) body.dueOn = req.dueOn ?? "";

    return this.updateVerbatim(cardId, body);
  }
}
