package com.basecamp.sdk.services

import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.models.Person
import com.basecamp.sdk.generated.models.RecordingBucket
import com.basecamp.sdk.generated.models.RecordingParent
import com.basecamp.sdk.generated.models.TodoBucket
import com.basecamp.sdk.generated.models.TodoParent
import com.basecamp.sdk.mentionedPersonIds
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Points at one recording the way an event feed row does.
 *
 * @property bucketId The project the recording lives in. Required: it scopes the
 *   Campfire discovery for chat lines, and the read is checked against it so a
 *   pointer from one project can never resolve to a recording in another.
 * @property recordingId The recording's id.
 * @property eventType The account event feed type that named the recording —
 *   `comment.created`, `card.assignment_changed`, `chat.line.created`. The
 *   segment before the action names the recording type. Used when
 *   [recordingType] is empty.
 * @property recordingType The recording's own type as BC3 spells it —
 *   `Comment`, `Kanban::Card`, `Chat::Lines::Text`. When set it takes precedence
 *   over [eventType], being the more exact of the two.
 */
data class RecordingRef(
    val bucketId: Long,
    val recordingId: Long,
    val eventType: String = "",
    val recordingType: String = "",
)

/**
 * The compact projection `RecordingsService.summarize` returns.
 *
 * A field a type does not have is either MISSING from the JSON or present and
 * empty, and which one is not a property of nullability — it is whether the
 * field has a DEFAULT. `Json` here leaves `encodeDefaults` off, so a property
 * equal to its default is omitted; `explicitNulls` is on, so a nullable
 * property with no default would serialize as `null` rather than vanish.
 *
 *  - [parent], [bucket], [creator], [assignees] and [campfireId] declare
 *    `= null` and so are omitted. [assignees] needs one more step to get there,
 *    in [summaryOf]: an empty list is not the default, so it is narrowed to
 *    null before it can be left out.
 *  - Every other property has no default and is always written, including when
 *    it is empty. A vault reads `"content": ""`, and a type whose read carries
 *    no title reads `"title": ""` — [firstNonEmpty] returns the empty string
 *    rather than null.
 *
 * The reference says the same thing about its own struct ("fields a type does
 * not have are zero") and reaches the missing-key half with `omitempty`.
 *
 * @property type The recording type as BC3 spells it (`Comment`, `Kanban::Card`).
 * @property parent The recording this one hangs off — the commented recording
 *   for a comment, the Campfire for a chat line, the column for a card.
 * @property assignees Set for the assignable types (to-dos, cards, card steps).
 * @property mentionedPersonIds The people [content] mentions, per
 *   [com.basecamp.sdk.mentionedPersonIds]. Always present, so a JSON consumer
 *   reads `[]` rather than a missing key.
 * @property content The recording's rich text, in full: the comment body, the
 *   message body, a to-do's description, a card's content, the chat line.
 * @property campfireId The Campfire a chat line was found under — the reply
 *   destination for a chat trigger. Absent for every other type.
 */
@Serializable
data class RecordingSummary(
    val id: Long,
    val status: String,
    val type: String,
    val title: String,
    @SerialName("app_url") val appUrl: String,
    val parent: RecordingParent? = null,
    val bucket: TodoBucket? = null,
    val creator: Person? = null,
    val assignees: List<Person>? = null,
    @SerialName("mentioned_person_ids") val mentionedPersonIds: List<Long>,
    val content: String,
    @SerialName("updated_at") val updatedAt: String,
    @SerialName("campfire_id") val campfireId: Long? = null,
)

/** The routing key: which typed read serves a [RecordingRef]. */
internal enum class SummaryKind {
    COMMENT,
    MESSAGE,
    TODO,
    CARD,
    CHAT_LINE,
    DOCUMENT,
    UPLOAD,
    SCHEDULE_ENTRY,
    QUESTION,
    QUESTION_ANSWER,
    TODOLIST,
    VAULT,
    FORWARD,
    CLIENT_APPROVAL,
    CLIENT_CORRESPONDENCE,
    GOOGLE_DOCUMENT,
    CLOUD_FILE,
    CARD_STEP,
    QUESTIONNAIRE,
    SCHEDULE,
    TODOSET,
    MESSAGE_BOARD,
    CARD_TABLE,
    CARD_COLUMN,
    INBOX,
    CAMPFIRE,
}

/**
 * Maps the subject of an account event feed type — everything before its final
 * `.` — to a read. This is the feed's catalog (bc3 `Event::EventType`) minus
 * `boost`, which names no recording type and is refused explicitly rather than
 * left to fall through as unknown.
 */
private val EVENT_SUBJECTS: Map<String, SummaryKind> = mapOf(
    "comment" to SummaryKind.COMMENT,
    "message" to SummaryKind.MESSAGE,
    "todo" to SummaryKind.TODO,
    "card" to SummaryKind.CARD,
    "chat.line" to SummaryKind.CHAT_LINE,
)

/**
 * Maps BC3's recording type strings to a read. It is the routing contract, and
 * it is a DELIBERATE set, not an exhaustive one: the recording types the account
 * event feed's trigger matrix names (comment, message, to-do, card, chat line),
 * plus the content and tool recordings a consumer reasoning about those is
 * likely to hold an id for. Chat lines are matched by prefix
 * (`Chat::Lines::Text`, `::RichText`, `::Code`, `::Upload`, `::Integration` all
 * read through the same route); everything else exactly.
 *
 * A type outside this set is `unknown_recording_type` by design, whether or not
 * the SDK has an id-only read for it — `Timesheet::Entry` and `Gauge::Needle`
 * do, and are not routed; `Client::Reply` and `Forward::Reply` cannot be, since
 * their reads need a parent id the pointer does not carry. Widening the set is a
 * product decision, not a gap: add the type here, its projection in
 * `RecordingsService.readSummary`, a routing row in the native test, and a case
 * in `conformance/tests/recording_summary.json`, the fixture every port
 * implements.
 *
 * [summarizableRecordingTypes] exposes this map's keys, so the documented
 * contract cannot drift from the routed one.
 */
private val RECORDING_TYPES: Map<String, SummaryKind> = mapOf(
    "Comment" to SummaryKind.COMMENT,
    "Message" to SummaryKind.MESSAGE,
    "Todo" to SummaryKind.TODO,
    "Kanban::Card" to SummaryKind.CARD,
    "Document" to SummaryKind.DOCUMENT,
    "Upload" to SummaryKind.UPLOAD,
    "Schedule::Entry" to SummaryKind.SCHEDULE_ENTRY,
    "Question" to SummaryKind.QUESTION,
    "Question::Answer" to SummaryKind.QUESTION_ANSWER,
    "Todolist" to SummaryKind.TODOLIST,
    "Vault" to SummaryKind.VAULT,
    "Inbox::Forward" to SummaryKind.FORWARD,
    "Client::Approval" to SummaryKind.CLIENT_APPROVAL,
    "Client::Correspondence" to SummaryKind.CLIENT_CORRESPONDENCE,
    "GoogleDocument" to SummaryKind.GOOGLE_DOCUMENT,
    "CloudFile" to SummaryKind.CLOUD_FILE,
    "Kanban::Step" to SummaryKind.CARD_STEP,
    "Questionnaire" to SummaryKind.QUESTIONNAIRE,
    "Schedule" to SummaryKind.SCHEDULE,
    "Todoset" to SummaryKind.TODOSET,
    "Message::Board" to SummaryKind.MESSAGE_BOARD,
    "Kanban::Board" to SummaryKind.CARD_TABLE,
    "Kanban::Column" to SummaryKind.CARD_COLUMN,
    "Inbox" to SummaryKind.INBOX,
    "Chat::Transcript" to SummaryKind.CAMPFIRE,
)

internal const val CHAT_LINE_TYPE_PREFIX = "Chat::Lines::"

/**
 * The recording types `RecordingsService.summarize` routes by
 * [RecordingRef.recordingType], sorted, with the `Chat::Lines` subtypes
 * represented by their shared prefix (`Chat::Lines::*`). The set is deliberate
 * rather than exhaustive — see [RECORDING_TYPES] — and any other type is
 * `unknown_recording_type` by design.
 */
fun summarizableRecordingTypes(): List<String> =
    (RECORDING_TYPES.keys + "$CHAT_LINE_TYPE_PREFIX*").sorted()

/**
 * The account event feed subjects `RecordingsService.summarize` routes by
 * [RecordingRef.eventType] — an event type is `<subject>.<action>`, and any
 * action on a listed subject routes to that subject's read — sorted. `boost` is
 * absent on purpose: `no_recording_type`.
 */
fun summarizableEventTypes(): List<String> = EVENT_SUBJECTS.keys.map { "$it.*" }.sorted()

/**
 * Whether a chat line subtype carries rich text — the two that declare
 * `rich_text_attribute :content` in BC3, and so the only two whose content can
 * hold a mention. A `Chat::Lines::Text` line's content is HTML-escaped on the way
 * out (`content_helper.rb`, `format_chat_line_with`), a `Chat::Lines::Code`
 * line's is served verbatim — a snippet that happens to contain a
 * `bc-attachment` tag — and an upload line has no content.
 */
internal fun chatLineIsRichText(lineType: String): Boolean =
    lineType == "Chat::Lines::RichText" || lineType == "Chat::Lines::Integration"

/** Picks the read for a ref. [RecordingRef.recordingType] wins when set. */
internal fun routeRecording(ref: RecordingRef): SummaryKind {
    // The routing key is trimmed; the MESSAGE renders what the caller wrote, so
    // a pointer whose type is " Widget " reads back as " Widget " and the report
    // is of the value that was actually handed over.
    val reported = ref.recordingType.ifEmpty { ref.eventType }
    val recordingType = ref.recordingType.trim()
    if (recordingType.isNotEmpty()) {
        if (recordingType.startsWith(CHAT_LINE_TYPE_PREFIX)) return SummaryKind.CHAT_LINE
        return RECORDING_TYPES[recordingType] ?: throw unknownRecordingType(reported)
    }
    val eventType = ref.eventType.trim()
    if (eventType.isEmpty()) throw unknownRecordingType(reported)
    // A feed type is "<subject>.<action>"; the subject names the recording type.
    // A string with no action is not a feed type and is not routed.
    val i = eventType.lastIndexOf('.')
    if (i <= 0 || i == eventType.length - 1) throw unknownRecordingType(reported)
    val subject = eventType.substring(0, i)
    if (subject == "boost") {
        throw BasecampException.RecordingSummaryFailure(
            BasecampException.RECORDING_NO_TYPE,
            "event type names no recording type: \"$reported\"",
            "the row points at the boost's target, whose type it does not carry; " +
                "resolve it from your own record of what you posted",
        )
    }
    return EVENT_SUBJECTS[subject] ?: throw unknownRecordingType(reported)
}

private fun unknownRecordingType(key: String) = BasecampException.RecordingSummaryFailure(
    BasecampException.RECORDING_UNKNOWN_TYPE,
    "no typed read for recording type: \"$key\"",
    "summarizableRecordingTypes() and summarizableEventTypes() list what is routed",
)

/** The first non-empty of [values], or `""`. */
internal fun firstNonEmpty(vararg values: String?): String = values.firstOrNull { !it.isNullOrEmpty() } ?: ""

/**
 * Projects one read onto the summary shape, reading the mentions out of
 * [content].
 */
@Suppress("LongParameterList")
internal fun summaryOf(
    id: Long,
    status: String,
    type: String,
    title: String,
    appUrl: String,
    parent: RecordingParent?,
    bucket: TodoBucket?,
    creator: Person?,
    assignees: List<Person>?,
    content: String,
    updatedAt: String,
    campfireId: Long? = null,
    mentions: List<Long> = mentionedPersonIds(content),
): RecordingSummary = RecordingSummary(
    id = id,
    status = status,
    type = type,
    title = title,
    appUrl = appUrl,
    parent = parent,
    bucket = bucket,
    creator = creator,
    // Empty and absent are one thing here, as Go's `omitempty` makes them: a
    // type with no assignees emits no key rather than an empty array, so the
    // projection reads the same in every SDK.
    assignees = assignees?.takeIf { it.isNotEmpty() },
    mentionedPersonIds = mentions,
    content = content,
    updatedAt = updatedAt,
    campfireId = campfireId,
)

/**
 * The bucket projections are the same three fields under two generated names —
 * BC3 serves one shape and the spec models it twice — so the summary carries one
 * of them and the other converts.
 */
internal fun RecordingBucket.asTodoBucket(): TodoBucket = TodoBucket(id = id, name = name, type = type)

/** The parent projections likewise differ only in which generated name they carry. */
internal fun TodoParent.asRecordingParent(): RecordingParent =
    RecordingParent(id = id, title = title, type = type, url = url, appUrl = appUrl)
