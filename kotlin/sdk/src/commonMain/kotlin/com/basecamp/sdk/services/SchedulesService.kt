package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.models.ScheduleEntry
import com.basecamp.sdk.generated.services.ReplaceScheduleEntryBody
import kotlinx.serialization.SerializationException

/**
 * A schedule entry's writable state, the receiver of the
 * [SchedulesService.editEntry] block.
 *
 * The writable set does not have one uniform rule, so this class does not
 * either. It splits in two:
 *
 * - **Full state** — [summary], [startsAt], [endsAt], [description], [allDay].
 *   Plain properties, seeded from the read-back and resent whether or not the
 *   block touches them. Nothing has to be recorded: on a full-replace endpoint
 *   `""` is how a clear is expressed, so an untouched field is written back
 *   verbatim rather than left to the server's clear-by-default. Each is
 *   non-nullable, so the type system already refuses the one value that has no
 *   wire spelling.
 * - **Addressed-only** — [participantIds], [url], [highlighted], [notify].
 *   Each is *readable* (seeded from the read-back, so a block can inspect the
 *   current join link, highlight and participants before deciding), but each
 *   reaches the wire only when the block **assigns** it.
 *
 * Dirty tracking is by setter invocation, deliberately, and not by comparing
 * the block's result against the read-back. Assigning a carve-out the value the
 * GET just returned is an address like any other — `entry.url = entry.url` is a
 * write — and a diff would drop it, handing the field back to BC3's
 * preserve-on-omission so the server keeps whatever a concurrent writer left
 * there instead of the value the caller stated.
 *
 * Seeding does not mark anything dirty: a Kotlin property initializer assigns
 * the backing field directly rather than going through the custom setter.
 */
class ScheduleEntryFields internal constructor(
    /** Plain-text summary. Set `""` to clear — the entry then reads back as "Untitled". */
    var summary: String,
    /**
     * Start of the entry, round-tripped verbatim. BC3 renders a bare date
     * (`"2026-06-05"`) for an all-day entry and a full timestamp otherwise, so
     * this stays a `String`: parsing and re-rendering would rewrite an all-day
     * entry's bounds on a call that never mentioned them.
     */
    var startsAt: String,
    /** End of the entry, round-tripped verbatim. See [startsAt]. */
    var endsAt: String,
    /** Rich text description (HTML). Set `""` to clear. */
    var description: String,
    /**
     * Whether the entry spans whole days. Non-nullable: defaulting a missing
     * value to `false` would convert an all-day event into a
     * midnight-to-midnight timed one.
     */
    var allDay: Boolean,
    seedParticipantIds: List<Long>?,
    seedUrl: String?,
    seedHighlighted: Boolean?,
) {
    /** The carve-out keys the block assigned, in their wire spelling. */
    private val addressedKeys: MutableSet<String> = mutableSetOf()

    /**
     * The entry's participants, seeded from the read-back's `participants`.
     * Assign `emptyList()` to remove everyone; leave it alone and the key never
     * reaches the wire, so BC3 keeps the participants it already holds.
     */
    var participantIds: List<Long>? = seedParticipantIds
        set(value) {
            addressedKeys.add(PARTICIPANT_IDS)
            field = value
        }

    /**
     * The entry's join link, seeded from the read-back's **`join_url`** — never
     * from the response's `url`, which is the entry's own Basecamp API URL.
     * Assign `""` to drop the link; leave it alone and the key never reaches
     * the wire.
     */
    var url: String? = seedUrl
        set(value) {
            addressedKeys.add(URL)
            field = value
        }

    /**
     * Whether the entry is highlighted, seeded from the read-back. Assign
     * `false` to remove the highlight; leave it alone and the key never reaches
     * the wire.
     */
    var highlighted: Boolean? = seedHighlighted
        set(value) {
            addressedKeys.add(HIGHLIGHTED)
            field = value
        }

    /**
     * Whether to notify participants. A directive rather than state — sending
     * it makes BC3 recompute a drafted entry's subscriber list — so nothing in
     * the read-back seeds it and it starts null.
     */
    var notify: Boolean? = null
        set(value) {
            addressedKeys.add(NOTIFY)
            field = value
        }

    /** Whether the caller addressed this carve-out, by assignment. */
    internal fun isAddressed(key: String): Boolean = key in addressedKeys

    internal companion object {
        const val PARTICIPANT_IDS = "participant_ids"
        const val URL = "url"
        const val HIGHLIGHTED = "highlighted"
        const val NOTIFY = "notify"
    }
}

/**
 * Schedules service with merge-safe [updateEntry] and read-modify-write
 * [editEntry] on top of the generated surface (`getEntry`, `replaceEntry`,
 * `createEntry`, `listEntries`, ...).
 *
 * `PUT /schedule_entries/{entryId}` is a full replace: BC3's
 * `Schedules::EntriesController#update` rebuilds the recordable from the
 * submitted params, so a sparse PUT that omits `description` erases it, one
 * that omits `summary` leaves the entry reading back as `"Untitled"`
 * (`Schedule::Entry#summary` is `super.presence || "Untitled"`), and one that
 * omits `all_day` turns an all-day event into a midnight-to-midnight timed one.
 * None of those is a 422 — every one is a 200 that quietly clears. The raw,
 * destructive-by-design path stays reachable as `replaceEntry`.
 *
 * ## Two classes of writable field
 *
 * Three writable fields are exempt from the rebuild. BC3 seeds
 * `participant_ids`, `url` and `highlighted` from the existing recordable when
 * the request does not address them, so the SDK's job for those is the opposite
 * one: keep them **off** the wire unless the caller asked for them. Resending
 * them is redundant at best and wrong if the read raced a concurrent change —
 * and the response spells the join link `join_url`, because
 * `recordings/_recording` already wrote `url` for the recording's own API URL
 * before the entry partial renders, so echoing the response's `url` would store
 * the API URL as the join link. `notify` is addressed-only for a different
 * reason: it is a directive, not state, and the read-back carries nothing to
 * seed it from.
 *
 * An explicitly *empty* value in that second class is an address, not an
 * absence: `participantIds = emptyList()` clears participants, `url = ""`
 * clears the join link, `highlighted = false` removes the highlight.
 *
 * ## Non-recurring entries only
 *
 * `ensure_non_recurring_event` 302-redirects both `show` and `update` for a
 * recurring entry, and the SDK follows neither: the PUT surfaces as a redirect
 * and the GET as a body this composite refuses rather than reads. Recurrence
 * itself (`recurrence_schedule`, `recurs_until`, `time_zone_name`) is unmodeled
 * — BC3 forces all three to nil for a non-recurring entry.
 *
 * Both composites call the public `getEntry` and `replaceEntry`, so hooks
 * observe the two wire operations (`GetScheduleEntry` then
 * `ReplaceScheduleEntry`), not a synthetic composite.
 *
 * Neither is atomic: there is no conditional-update signal on this endpoint, so
 * a concurrent write between the GET and PUT is overwritten — last write wins
 * for the full-state fields. The window is one round-trip. Use `replaceEntry`
 * to overwrite deliberately.
 */
class SchedulesService(client: AccountClient) :
    com.basecamp.sdk.generated.services.SchedulesService(client) {

    /**
     * Sets the given fields on a schedule entry and preserves everything else:
     * GETs the current entry, overlays the explicitly-set (non-null) arguments,
     * and PUTs the full representation back.
     *
     * A null argument is not addressed. For the five full-state fields that
     * means the read-back value is resent; for the four carve-outs it means the
     * key never reaches the wire, leaving BC3 to seed it from the record it
     * already holds. An explicitly-passed `""`, `emptyList()` or `false` is an
     * address and is sent — in Kotlin a safe call short-circuits on null and
     * nothing else, so those three values pass the same null test a `"keep it"`
     * argument fails.
     *
     * Every argument is nullable for exactly that reason. A non-null default
     * (`allDay: Boolean = false`, `participantIds: List<Long> = emptyList()`)
     * would collapse "not addressed" into "addressed with the zero value" and
     * clear a field the caller never mentioned.
     *
     * Not atomic — see the class docs for the GET→PUT race and the
     * recurring-entry redirect.
     *
     * @param entryId The entry ID
     * @param summary New summary (null = keep current)
     * @param startsAt New start, a date or timestamp (null = keep current)
     * @param endsAt New end, a date or timestamp (null = keep current)
     * @param description New description (null = keep current, `""` clears)
     * @param allDay New all-day flag (null = keep current)
     * @param participantIds Replaces participants (null = leave to BC3, `emptyList()` clears)
     * @param url New join link (null = leave to BC3, `""` clears)
     * @param highlighted New highlight (null = leave to BC3, `false` removes)
     * @param notify Notify participants (null = do not address)
     */
    suspend fun updateEntry(
        entryId: Long,
        summary: String? = null,
        startsAt: String? = null,
        endsAt: String? = null,
        description: String? = null,
        allDay: Boolean? = null,
        participantIds: List<Long>? = null,
        url: String? = null,
        highlighted: Boolean? = null,
        notify: Boolean? = null,
    ): ScheduleEntry {
        val fields = fieldsFromEntry(fetchEntry(entryId))
        if (summary != null) fields.summary = summary
        if (startsAt != null) fields.startsAt = startsAt
        if (endsAt != null) fields.endsAt = endsAt
        if (description != null) fields.description = description
        if (allDay != null) fields.allDay = allDay
        // The carve-outs go through the same setters a block would invoke, so
        // "the caller addressed this" is one rule with one implementation.
        if (participantIds != null) fields.participantIds = participantIds
        if (url != null) fields.url = url
        if (highlighted != null) fields.highlighted = highlighted
        if (notify != null) fields.notify = notify
        return putEntryFields(entryId, fields)
    }

    /**
     * Applies a read-modify-write block to a schedule entry: GETs the current
     * entry, runs the block with its writable state ([ScheduleEntryFields]) as
     * receiver, and PUTs the whole thing back. If the block throws, the edit
     * aborts and nothing is written.
     *
     * The five full-state fields are resent whether or not the block touches
     * them, so clearing one means setting it empty (`""`). The four carve-outs
     * are seeded so the block can read them, but reach the wire only if the
     * block assigns them — even when it assigns the value the read returned.
     *
     * ```kotlin
     * account.schedules.editEntry(entryId) {
     *     summary = "🚨 $summary"
     *     description = "" // clearing = setting empty on a full object
     *     // participants, join link and highlight are untouched, so they stay
     *     // off the wire and BC3 keeps them.
     * }
     *
     * account.schedules.editEntry(entryId) {
     *     if (url?.startsWith("https://meet.example.com/") == true) url = ""
     * }
     * ```
     *
     * Not atomic — see the class docs for the GET→PUT race and the
     * recurring-entry redirect.
     */
    suspend fun editEntry(entryId: Long, block: ScheduleEntryFields.() -> Unit): ScheduleEntry {
        val fields = fieldsFromEntry(fetchEntry(entryId))
        fields.block()
        return putEntryFields(entryId, fields)
    }

    /**
     * GETs the entry the composites read their writable state from,
     * normalizing a decode failure into the SPEC §6 shape.
     *
     * kotlinx.serialization is the typed guard the dynamic SDKs write by hand,
     * and it is what enforces this response's requiredness: `summary`,
     * `starts_at`, `ends_at` and `all_day` are all `@required` and
     * non-nullable on [ScheduleEntry], so an absent, null or structurally
     * wrong-typed one is refused before this composite ever sees it. It reports
     * that as a raw [SerializationException] (a [kotlinx.serialization.MissingFieldException]
     * for the absent case), which is not the shape SPEC §6 defines for a
     * malformed 2xx body: callers catching [BasecampException] would miss it
     * entirely and it carries no hint. Wrap it, so a malformed response looks
     * the same in every SDK.
     *
     * (The client-wide `coerceInputValues`/`isLenient` scalar hole means a bare
     * JSON scalar is coerced rather than rejected. That is a cross-service gap
     * tracked out of #576, not something this composite can close.)
     */
    private suspend fun fetchEntry(entryId: Long): ScheduleEntry =
        try {
            getEntry(entryId)
        } catch (e: SerializationException) {
            throw BasecampException.Api(
                message = "GetScheduleEntry returned a body that does not decode as a schedule " +
                    "entry: ${e.message}",
                hint = MALFORMED_HINT,
                retryable = false,
                cause = e,
            )
        }

    /**
     * Reads the writable state out of a fetched entry.
     *
     * The decoder has already done most of the dynamic SDKs' hand-written work
     * (see [fetchEntry]), and Kotlin's types carry the rest: `allDay` is a
     * non-null `Boolean`, so it can never be read with a truthiness test that
     * loses `false`, and `startsAt`/`endsAt` are non-null `String`s taken
     * verbatim — never parsed into a date type and re-rendered, which would
     * rewrite an all-day entry's bare-date bounds. `description` is nullable:
     * the rich-text partial always sets the key but the value may be null, and
     * absent or null is genuinely empty.
     *
     * Blankness is the one thing the decoder cannot supply. `summary`,
     * `starts_at` and `ends_at` are `@required` and BC3 can never render them
     * blank — `Schedule::Entry#summary` falls back to `"Untitled"`, and
     * `starts_at`/`ends_at` are NOT NULL columns every partial emits — so a
     * blank one on a 2xx read is a malformed response, not an empty value, and
     * carrying it into the full-replace PUT would blank the real value on a
     * call that touched something else.
     *
     * The carve-outs are seeded for *reading* only; nothing here puts them on
     * the wire. `url` is seeded from `joinUrl`, never from `url` (see the class
     * docs). `highlighted` is taken verbatim: it is optional, absent from the
     * reduced calendar partial `GetUpcomingSchedule` renders, and — unlike
     * every other member — cannot reach the wire unless the caller assigns it,
     * so there is nothing to refuse a malformed value on behalf of.
     */
    private fun fieldsFromEntry(entry: ScheduleEntry): ScheduleEntryFields {
        requireRendered(entry.summary, "summary")
        requireRendered(entry.startsAt, "starts_at")
        requireRendered(entry.endsAt, "ends_at")
        return ScheduleEntryFields(
            summary = entry.summary,
            startsAt = entry.startsAt,
            endsAt = entry.endsAt,
            description = entry.description ?: "",
            allDay = entry.allDay,
            seedParticipantIds = entry.participants?.map { it.id },
            seedUrl = entry.joinUrl,
            seedHighlighted = entry.highlighted,
        )
    }

    private fun requireRendered(value: String, key: String) {
        if (value.isBlank()) {
            throw BasecampException.Api(
                message = "GetScheduleEntry returned a schedule entry with a blank \"$key\", " +
                    "but the API never renders it blank",
                hint = MALFORMED_HINT,
                retryable = false,
            )
        }
    }

    /**
     * PUTs the writable state via `replaceEntry`.
     *
     * The five full-state fields are always sent, empties included: on a
     * full-replace endpoint `""` is how a clear is expressed — never JSON null
     * (SPEC §18 body compaction), and never by omission, which would leave the
     * field to the server's own clear-by-default and read as an accident rather
     * than an intent.
     *
     * The four carve-outs are sent only when addressed. `replaceEntry` compacts
     * a null field out of the body, which is exactly the wire shape an
     * unaddressed carve-out needs, so an unaddressed one is passed as null and
     * leaves no key at all.
     *
     * That same compaction is why an *addressed* carve-out cannot be allowed to
     * be null: a null would be stripped and the stated address would silently
     * become an omission — the defect this composite exists to prevent.
     * `participantIds` and `url` have empty spellings and are normalized to
     * them; `highlighted` and `notify` are booleans with none, so a null
     * assigned to either is caller error and is refused rather than dropped.
     */
    private suspend fun putEntryFields(
        entryId: Long,
        fields: ScheduleEntryFields,
    ): ScheduleEntry = replaceEntry(
        entryId,
        ReplaceScheduleEntryBody(
            summary = fields.summary,
            startsAt = fields.startsAt,
            endsAt = fields.endsAt,
            description = fields.description,
            allDay = fields.allDay,
            participantIds = if (fields.isAddressed(ScheduleEntryFields.PARTICIPANT_IDS)) {
                fields.participantIds ?: emptyList()
            } else {
                null
            },
            url = if (fields.isAddressed(ScheduleEntryFields.URL)) fields.url ?: "" else null,
            highlighted = if (fields.isAddressed(ScheduleEntryFields.HIGHLIGHTED)) {
                fields.highlighted ?: throw nullBoolean(ScheduleEntryFields.HIGHLIGHTED)
            } else {
                null
            },
            notify = if (fields.isAddressed(ScheduleEntryFields.NOTIFY)) {
                fields.notify ?: throw nullBoolean(ScheduleEntryFields.NOTIFY)
            } else {
                null
            },
        ),
    )

    private fun nullBoolean(key: String) = BasecampException.Usage(
        message = "schedule entry $key must be true or false, not null",
        hint = "$key is a boolean with no empty value, and body compaction drops null — " +
            "sending it would omit the field rather than state it, letting the server decide. " +
            "Assign true or false, or leave the member alone.",
    )

    private companion object {
        const val MALFORMED_HINT =
            "The merge-safe updateEntry/editEntry resend this record's fields verbatim, so a " +
                "malformed response cannot be written back safely. Use replaceEntry to write " +
                "the record deliberately."
    }
}
