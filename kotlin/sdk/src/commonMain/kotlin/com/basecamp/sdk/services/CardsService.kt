package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.generated.models.Card
import com.basecamp.sdk.generated.services.UpdateCardBody

/**
 * CardsService with a named-argument [update] on top of the generated surface
 * ([updateVerbatim], `get`, `move`, ...).
 *
 * BC3 (basecamp/bc3#12521) builds a card update from the JSON params exactly as
 * they arrive, so the body reads as a patch: a field the caller leaves out is
 * left alone, and only a field that is present changes. [update] takes the
 * caller at their word and sends one PUT carrying what they addressed and
 * nothing else — no field is read back and echoed.
 *
 * Clearing the due date is therefore something to *say*, not something to leave
 * unsaid: `dueOn = ""` puts `"due_on": ""` on the wire, which BC3 casts to nil.
 * Omitting the key changes nothing.
 */
class CardsService(client: AccountClient) :
    com.basecamp.sdk.generated.services.CardsService(client) {

    /**
     * Updates a card, touching only the fields the caller names.
     *
     * [dueOn] is tri-state:
     *
     * - `null` (omitted) — `due_on` stays off the wire and the card's due date
     *   is left as it is
     * - `""` — sent as `"due_on": ""`, which clears the due date
     * - a date — the due date is set
     *
     * [assigneeIds] reads the same way, and leaving it null is the only safe
     * default: BC3 filters incoming ids through `reachable_people`, so sending
     * back a list read from the card would silently unassign anyone who has
     * since lost board access.
     */
    suspend fun update(
        cardId: Long,
        title: String? = null,
        content: String? = null,
        dueOn: String? = null,
        assigneeIds: List<Long>? = null,
    ): Card =
        // dueOn goes to the wire verbatim, `""` included: the generated body
        // builder drops only nulls (`?.let`), so the blank survives as
        // `"due_on": ""` — the explicit clear. A literal null is not an
        // alternative; body compaction (SPEC §18) would drop it back to an
        // omission, which now means "leave it alone".
        updateVerbatim(
            cardId,
            UpdateCardBody(
                title = title,
                content = content,
                dueOn = dueOn,
                assigneeIds = assigneeIds,
            ),
        )
}
