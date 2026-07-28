package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.generated.models.Card
import com.basecamp.sdk.generated.services.UpdateCardBody

/**
 * CardsService with a merge-safe [update] on top of the generated surface
 * ([updateVerbatim], `get`, `move`, ...).
 *
 * BC3 builds the card's update params as `{ due_on: nil }.merge(card_params)`
 * (`kanban/cards_controller.rb`), so **any** update whose body omits `due_on`
 * erases the card's due date. A sparse PUT — the natural thing to write — is
 * therefore destructive on the raw endpoint, which stays available as
 * [updateVerbatim].
 *
 * [update] composes the public `get` and [updateVerbatim] methods, so hooks
 * observe the two wire operations rather than a synthetic composite.
 */
class CardsService(client: AccountClient) :
    com.basecamp.sdk.generated.services.CardsService(client) {

    /**
     * Updates a card without disturbing fields the caller did not mention.
     *
     * [dueOn] is tri-state, which is what makes this safe:
     *
     * - `null` (omitted) — the current due date is fetched and resent
     * - `""` — the due date is cleared
     * - a date — the due date is set
     *
     * The extra GET is only paid for in the `null` case, the one where the API
     * would otherwise destroy something.
     *
     * Assignees are never resent on the caller's behalf: BC3 filters incoming
     * IDs through `reachable_people`, so echoing back an id belonging to
     * someone who has since lost board access would silently unassign them.
     *
     * Not atomic: a concurrent due-date change landing between the GET and the
     * PUT is overwritten with the value this call read. The window is one
     * round-trip.
     */
    suspend fun update(
        cardId: Long,
        title: String? = null,
        content: String? = null,
        dueOn: String? = null,
        assigneeIds: List<Long>? = null,
    ): Card {
        // Clearing is encoded by OMITTING due_on — the generated body builder
        // drops nulls via `?.let`, and BC3 nils an omitted due date. Sending an
        // explicit null would violate body compaction (SPEC §18), and sending
        // "" risks a date-format error.
        val resolvedDueOn = when {
            dueOn == null -> get(cardId).dueOn?.takeIf { it.isNotEmpty() }
            dueOn.isEmpty() -> null
            else -> dueOn
        }
        return updateVerbatim(
            cardId,
            UpdateCardBody(
                title = title,
                content = content,
                dueOn = resolvedDueOn,
                assigneeIds = assigneeIds,
            ),
        )
    }
}
