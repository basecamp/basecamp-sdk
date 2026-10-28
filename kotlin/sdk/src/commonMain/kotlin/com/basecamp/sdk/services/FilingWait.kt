package com.basecamp.sdk.services

import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.models.Filing
import kotlinx.coroutines.delay
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.TimeSource

/**
 * The kind of container a move or copy files a recording into (SPEC.md
 * Appendix F, Filing Waits). Every kind but [TODOLIST] is a tool on a project's
 * dock, and [dockName] is its name there: the destination project's container
 * is its enabled dock item of that name. A to-do list is not on the dock; list
 * the destination project's to-do lists under its todoset.
 */
enum class FilingContainer(val dockName: String) {
    CARD_TABLE("kanban_board"),
    MESSAGE_BOARD("message_board"),
    VAULT("vault"),
    SCHEDULE("schedule"),
    QUESTIONNAIRE("questionnaire"),
    INBOX("inbox"),
    TODOSET("todoset"),
    TODOLIST("todolist"),
    ;

    /** Whether a destination of this kind can take `parentId`, a folder inside it. */
    val takesFolders: Boolean get() = this == VAULT

    companion object {
        private val BY_TYPE = mapOf(
            "Kanban::Card" to CARD_TABLE,
            "Kanban::Column" to CARD_TABLE,
            "Message" to MESSAGE_BOARD,
            "Document" to VAULT,
            "Upload" to VAULT,
            "CloudFile" to VAULT,
            "GoogleDocument" to VAULT,
            "Vault" to VAULT,
            "Schedule::Entry" to SCHEDULE,
            "Question" to QUESTIONNAIRE,
            "Inbox::Forward" to INBOX,
            "Todolist" to TODOSET,
            "Todo" to TODOLIST,
        )

        /**
         * The kind of container a move or copy files a recording into, from its
         * recordable type and its parent's type as the recording's own read
         * reports them ("" when it has no parent). A card lands in Triage, or in
         * Done if it is completed. Null for everything else: a recording that
         * moves only with its parent, such as a comment or the columns every
         * card table has, and a dock tool, such as a docked Vault, which is
         * filed by `destinationProjectId`.
         */
        fun of(recordingType: String, parentType: String): FilingContainer? = when {
            recordingType == "Todolist" && parentType == "Todolist" -> TODOLIST
            recordingType == "Vault" && parentType != "Vault" -> null
            else -> BY_TYPE[recordingType]
        }
    }
}

private val DEFAULT_WAIT = 1.minutes
private val FIRST_POLL = 250.milliseconds
private val MAX_POLL = 2.seconds

private fun refusedBeforeFiling(e: BasecampException): Boolean {
    val status = e.httpStatus ?: 0
    return status in 400..499 ||
        e.code == BasecampException.CODE_USAGE ||
        e.code == BasecampException.CODE_RATE_LIMIT
}

/** Starts a filing once and waits for it; the body of moveAndWait and copyAndWait. */
internal suspend fun fileAndWait(
    timeout: Duration?,
    start: suspend () -> Filing,
    getFiling: suspend (Long) -> Filing,
): Filing {
    val filing = try {
        start()
    } catch (e: BasecampException) {
        val reason = if (refusedBeforeFiling(e)) {
            BasecampException.FILING_REFUSED
        } else {
            BasecampException.FILING_MAY_HAVE_STARTED
        }
        throw BasecampException.FilingFailure(reason, "$reason: ${e.message}", cause = e)
    }

    val deadline = TimeSource.Monotonic.markNow() + (timeout?.takeIf { it.isPositive() } ?: DEFAULT_WAIT)
    var last = filing
    var wait = FIRST_POLL
    while (last.status != "completed" || last.destinationRecording == null) {
        if (last.status == "failed") {
            throw BasecampException.FilingFailure(BasecampException.FILING_FAILED, "filing ${last.id} failed", last)
        }
        val remaining = -deadline.elapsedNow()
        if (remaining <= wait) {
            if (remaining.isPositive()) delay(remaining)
            throw BasecampException.FilingFailure(
                BasecampException.FILING_UNSETTLED, "filing ${last.id} is still ${last.status}", last,
            )
        }
        delay(wait)
        last = try {
            getFiling(last.id)
        } catch (e: BasecampException) {
            throw BasecampException.FilingFailure(
                BasecampException.FILING_UNSETTLED, "filing ${last.id} is still ${last.status}", last, e,
            )
        }
        wait = minOf(wait * 2, MAX_POLL)
    }
    return last
}
