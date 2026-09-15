package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.campfires
import com.basecamp.sdk.generated.cardColumns
import com.basecamp.sdk.generated.cardSteps
import com.basecamp.sdk.generated.cardTables
import com.basecamp.sdk.generated.cards
import com.basecamp.sdk.generated.checkins
import com.basecamp.sdk.generated.clientApprovals
import com.basecamp.sdk.generated.clientCorrespondences
import com.basecamp.sdk.generated.cloudFiles
import com.basecamp.sdk.generated.comments
import com.basecamp.sdk.generated.documents
import com.basecamp.sdk.generated.forwards
import com.basecamp.sdk.generated.googleDocuments
import com.basecamp.sdk.generated.messageBoards
import com.basecamp.sdk.generated.messages
import com.basecamp.sdk.generated.models.CampfireLine
import com.basecamp.sdk.generated.models.Person
import com.basecamp.sdk.generated.models.RecordingParent
import com.basecamp.sdk.generated.models.TodoBucket
import com.basecamp.sdk.generated.schedules
import com.basecamp.sdk.generated.todolists
import com.basecamp.sdk.generated.todos
import com.basecamp.sdk.generated.todosets
import com.basecamp.sdk.generated.uploads
import com.basecamp.sdk.generated.vaults
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.longOrNull

/**
 * `RecordingsService` with [summarize] on top of the generated surface (`list`,
 * `spotlight`, ...).
 *
 * [summarize] is a compact projection of one recording, resolved from the
 * pointer an account event feed row or a webhook carries — bucket id, recording
 * id, and the event type or recording type — through the typed read that type
 * names. It exists for consumers that must decide something about a recording
 * without paying for its full payload: an agent connector's admission step, an
 * MCP tool answering "what is this?".
 *
 * The SDK has no untyped recording read (BC3 has no such route), so the type is
 * the routing key: `comment.created` reads a comment, `card.created` reads a
 * card, and so on — one typed read per type. Chat lines are the exception,
 * because their read needs the Campfire id and the pointer does not carry it;
 * [summarize] discovers the Campfire first (see [resolveChatLine]).
 *
 * This is hand-written composition over the generated services (AGENTS.md;
 * SPEC.md §18, Appendix F). It makes no wire request of its own, and it mints no
 * operation identity: hooks see the constituent reads under their own names
 * (SPEC.md §18 rule 3).
 */
class RecordingsService(private val account: AccountClient) :
    com.basecamp.sdk.generated.services.RecordingsService(account) {

    /**
     * Resolves a recording pointer into a [RecordingSummary] through the typed
     * read its type names. See [RecordingRef] for the routing inputs and the
     * class comment above for the design.
     *
     * Throws: [BasecampException.Usage] for a pointer with no ids;
     * [BasecampException.RecordingSummaryFailure] with
     * [BasecampException.RECORDING_NO_TYPE] or
     * [BasecampException.RECORDING_UNKNOWN_TYPE] for a routing failure, before
     * any request; the read's own [BasecampException] otherwise — a 404 is
     * [BasecampException.NotFound], as from the typed read itself; for chat
     * lines, [BasecampException.RECORDING_UNRESOLVED] when every visible
     * Campfire answered 404, which is distinct from a read that failed (any
     * non-404 from a candidate is raised as that error, and the loop stops
     * there) and from [BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE]
     * (candidates were left unsearched);
     * [BasecampException.RECORDING_BUCKET_MISMATCH] when the read returned a
     * recording from another bucket.
     */
    suspend fun summarize(ref: RecordingRef): RecordingSummary =
        summarize(ref, account.parent.campfireIndex)

    /**
     * [summarize] against a given discovery index.
     *
     * The clock seam: the caches are TTL- and floor-governed, and the only way to
     * reach the refresh-on-miss branches deterministically is to hand in an index
     * built on a test clock. Production callers use the overload above, which
     * passes the client's own index — the one shared across every account client
     * the client hands out.
     */
    internal suspend fun summarize(ref: RecordingRef, index: CampfireIndex): RecordingSummary {
        if (ref.bucketId <= 0 || ref.recordingId <= 0) {
            throw BasecampException.Usage("bucket id and recording id are required")
        }
        val kind = routeRecording(ref)
        val summary = readSummary(ref, kind, index)
        val bucketId = summary.bucket?.id
        if (bucketId != null && bucketId != 0L && bucketId != ref.bucketId) {
            throw BasecampException.RecordingSummaryFailure(
                BasecampException.RECORDING_BUCKET_MISMATCH,
                "recording is not in the requested bucket: recording ${ref.recordingId} " +
                    "is in bucket $bucketId, not ${ref.bucketId}",
                bucketId = ref.bucketId,
                recordingId = ref.recordingId,
            )
        }
        return summary
    }

    /** Performs the one typed read a kind names and projects it. */
    @Suppress("CyclomaticComplexMethod", "LongMethod")
    private suspend fun readSummary(ref: RecordingRef, kind: SummaryKind, index: CampfireIndex): RecordingSummary {
        val id = ref.recordingId
        return when (kind) {
            SummaryKind.COMMENT -> account.comments.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket, it.creator, null, it.content, it.updatedAt,
                )
            }

            SummaryKind.MESSAGE -> account.messages.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.subject), it.appUrl,
                    it.parent, it.bucket, it.creator, null, it.content, it.updatedAt,
                )
            }

            // A to-do's content is its plain title; the rich text — where
            // mentions live — is the description.
            SummaryKind.TODO -> account.todos.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.content), it.appUrl,
                    it.parent.asRecordingParent(), it.bucket, it.creator, it.assignees,
                    it.description.orEmpty(), it.updatedAt,
                )
            }

            SummaryKind.CARD -> account.cards.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket, it.creator, it.assignees,
                    firstNonEmpty(it.content, it.description), it.updatedAt,
                )
            }

            SummaryKind.CHAT_LINE -> {
                val (line, campfireId) = resolveChatLine(index, ref.bucketId, id)
                val content = line.content.orEmpty()
                summaryOf(
                    line.id, line.status, line.type, line.title, line.appUrl,
                    line.parent, line.bucket, line.creator, null, content, line.updatedAt,
                    campfireId = campfireId,
                    // A plain-text, code or upload line's content is text BC3
                    // never read as markup, so a literal "<bc-attachment>" in it
                    // mentions nobody.
                    mentions = if (chatLineIsRichText(line.type)) {
                        com.basecamp.sdk.mentionedPersonIds(content)
                    } else {
                        emptyList()
                    },
                )
            }

            SummaryKind.DOCUMENT -> account.documents.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket, it.creator, null, it.content.orEmpty(), it.updatedAt,
                )
            }

            SummaryKind.UPLOAD -> account.uploads.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.filename), it.appUrl,
                    it.parent, it.bucket, it.creator, null, it.description.orEmpty(), it.updatedAt,
                )
            }

            SummaryKind.SCHEDULE_ENTRY -> account.schedules.getEntry(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.summary), it.appUrl,
                    it.parent, it.bucket, it.creator, null, it.description.orEmpty(), it.updatedAt,
                )
            }

            SummaryKind.QUESTION -> account.checkins.getQuestion(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket.asTodoBucket(), it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.QUESTION_ANSWER -> account.checkins.getAnswer(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket.asTodoBucket(), it.creator, null, it.content, it.updatedAt,
                )
            }

            SummaryKind.TODOLIST -> account.todolists.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.name), it.appUrl,
                    it.parent.asRecordingParent(), it.bucket, it.creator, null, it.description, it.updatedAt,
                )
            }

            SummaryKind.VAULT -> account.vaults.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket, it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.FORWARD -> account.forwards.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.subject), it.appUrl,
                    it.parent, it.bucket, it.creator, null, it.content.orEmpty(), it.updatedAt,
                )
            }

            SummaryKind.CLIENT_APPROVAL -> account.clientApprovals.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.subject), it.appUrl,
                    it.parent, it.bucket.asTodoBucket(), it.creator, null, it.content.orEmpty(), it.updatedAt,
                )
            }

            SummaryKind.CLIENT_CORRESPONDENCE -> account.clientCorrespondences.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.subject), it.appUrl,
                    it.parent, it.bucket.asTodoBucket(), it.creator, null, it.content.orEmpty(), it.updatedAt,
                )
            }

            // The two cloud-storage reads are modeled as raw documents in the
            // spec, so the generated method hands back the decoded JSON rather
            // than a named type. The projection reads the same keys the other
            // content recordings expose.
            SummaryKind.GOOGLE_DOCUMENT -> untypedRecordingSummary(account.googleDocuments.googleDocument(id))

            SummaryKind.CLOUD_FILE -> untypedRecordingSummary(account.cloudFiles.cloudFile(id))

            SummaryKind.CARD_STEP -> account.cardSteps.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket, it.creator, it.assignees, "", it.updatedAt,
                )
            }

            SummaryKind.QUESTIONNAIRE -> account.checkins.getQuestionnaire(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.name), it.appUrl,
                    null, it.bucket.asTodoBucket(), it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.SCHEDULE -> account.schedules.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    null, it.bucket, it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.TODOSET -> account.todosets.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, firstNonEmpty(it.title, it.name), it.appUrl,
                    null, it.bucket, it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.MESSAGE_BOARD -> account.messageBoards.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    null, it.bucket, it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.CARD_TABLE -> account.cardTables.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    null, it.bucket, it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.CARD_COLUMN -> account.cardColumns.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    it.parent, it.bucket, it.creator, null, it.description.orEmpty(), it.updatedAt,
                )
            }

            SummaryKind.INBOX -> account.forwards.getInbox(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    null, it.bucket, it.creator, null, "", it.updatedAt,
                )
            }

            SummaryKind.CAMPFIRE -> account.campfires.get(id).let {
                summaryOf(
                    it.id, it.status, it.type, it.title, it.appUrl,
                    null, it.bucket, it.creator, null, "", it.updatedAt,
                )
            }
        }
    }

    /** Projects a recording the spec models as a raw document. */
    private fun untypedRecordingSummary(element: JsonElement): RecordingSummary {
        val obj = element as? JsonObject
            ?: throw BasecampException.Api("recording body is not a JSON object", httpStatus = null)
        fun string(key: String): String = (obj[key] as? JsonPrimitive)?.contentOrNull.orEmpty()
        val id = (obj["id"] as? JsonPrimitive)?.longOrNull
            ?: throw BasecampException.Api("recording body carries no id", httpStatus = null)
        return summaryOf(
            id,
            string("status"),
            string("type"),
            string("title"),
            string("app_url"),
            obj["parent"]?.let { json.decodeFromJsonElement(RecordingParent.serializer(), it) },
            obj["bucket"]?.let { json.decodeFromJsonElement(TodoBucket.serializer(), it) },
            obj["creator"]?.let { json.decodeFromJsonElement(Person.serializer(), it) },
            null,
            string("description"),
            string("updated_at"),
        )
    }

    // --- Campfire discovery for chat lines ------------------------------------
    //
    // The loop tries the line under each candidate until one answers, within one
    // total budget of MAX_CAMPFIRE_CANDIDATES per call.
    //
    // Two failure shapes are kept apart on purpose. A candidate that answers
    // anything but 404 — 401, 403, 5xx, a network error, a cancelled job — stops
    // the loop and is raised as that error: the read failed, and trying the next
    // Campfire would only hide it. A 404 means "not here", so the loop moves on.
    // Only when every candidate said "not here" is the line unresolved
    // (RECORDING_UNRESOLVED) — and before concluding that, the cached sources are
    // refreshed (subject to a floor) so a Campfire created after the cache filled
    // is tried too. Discovery that could not be completed — a listing cut off at
    // its cap, a bucket with more candidates than the budget — is
    // CAMPFIRE_DISCOVERY_INCOMPLETE, never "unresolved": nothing unsearched is
    // ever reported absent.
    //
    // What HTTP cannot tell apart: BC3 answers 404 both for a line that is not in
    // a Campfire and for a Campfire the caller may no longer see. "Unresolved"
    // therefore means "under no Campfire the caller can currently see", and the
    // error reports the cached candidates that the refreshed sources no longer
    // list (staleCampfireIds) so a consumer can see when visibility, not
    // existence, is what changed.

    /** One `summarize` call's discovery state. */
    private inner class ChatLineSearch(val lineId: Long) {
        /** Candidates that answered 404, in order. */
        val tried = mutableListOf<Long>()

        /** Candidates still allowed. */
        var budget = MAX_CAMPFIRE_CANDIDATES

        /** A candidate was left untried for want of budget. */
        var skipped = false

        /**
         * Reads the line under each candidate not yet tried. Returns the line and
         * its Campfire on a hit, or null on a miss, recording the candidates in
         * [tried]. Any answer but 404 is raised as is.
         */
        suspend fun tryAll(candidates: List<Long>): Pair<CampfireLine, Long>? {
            for (campfireId in candidates) {
                if (campfireId in tried) continue
                if (budget <= 0) {
                    skipped = true
                    return null
                }
                budget--
                val line = try {
                    account.campfires.getLine(campfireId, lineId)
                } catch (e: BasecampException.NotFound) {
                    tried.add(campfireId)
                    continue
                }
                return line to campfireId
            }
            return null
        }
    }

    /**
     * Finds the Campfire a line lives in and reads it. See the discovery comment
     * above for the contract.
     */
    private suspend fun resolveChatLine(
        index: CampfireIndex,
        bucketId: Long,
        lineId: Long,
    ): Pair<CampfireLine, Long> {
        val search = ChatLineSearch(lineId)

        // Pass 1: what the sources already hold — the dock (read if it must be),
        // then the listing only if it is cached. A listing fetch is the
        // expensive, slow request, and it is not made until the dock — including
        // its refresh — has had its say, so a listing that is down, over its cap,
        // or stalled never stands between a project's line and the one project
        // read that finds it.
        var dock = index.dockCampfires(account, bucketId, refresh = false)
        search.tryAll(dock.ids)?.let { return it }
        val listed = index.cachedListedCampfires(account.accountId, bucketId)
        val listCached = listed != null
        if (listed != null) {
            search.tryAll(listed.ids)?.let { return it }
        }

        // Pass 2: re-read the dock if it was served from cache (the floor may
        // decline), then fetch or refresh the listing. Whatever comes back is the
        // current snapshot of that source, whoever loaded it — another caller may
        // have populated or refreshed it in the meantime — so it always replaces
        // the pass-1 one; "refreshed" is whether a source the conclusion had
        // consulted is now newer than when it was consulted.
        //
        // Not when the budget is already spent: a re-read could return no
        // candidate this call may try, so it would cost a request that cannot
        // help — and a failure on it would replace the deterministic
        // "incomplete" verdict with a transient error a consumer retries forever.
        var refreshed = false
        if (search.skipped) throw tooManyCandidates(bucketId, lineId)
        if (dock.cached) {
            val again = index.dockCampfires(account, bucketId, refresh = true)
            if (again.fetched > dock.fetched || !again.cached) refreshed = true
            dock = again
            search.tryAll(dock.ids)?.let { return it }
        }
        if (search.skipped) throw tooManyCandidates(bucketId, lineId)
        val againListed = try {
            index.listedCampfires(account, bucketId, refresh = listCached)
        } catch (e: CampfireListingOverflow) {
            throw BasecampException.RecordingSummaryFailure(
                BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE,
                "campfire discovery incomplete: line $lineId in bucket $bucketId: ${e.message}",
                "narrow the search by reading the line through campfires.getLine with a known Campfire id",
                bucketId = bucketId,
                recordingId = lineId,
            )
        }
        if (listed != null && (againListed.fetched > listed.fetched || !againListed.cached)) refreshed = true
        search.tryAll(againListed.ids)?.let { return it }
        if (search.skipped) throw tooManyCandidates(bucketId, lineId)

        val stale = if (refreshed) {
            search.tried.filter { it !in dock.ids && it !in againListed.ids }
        } else {
            emptyList()
        }
        throw BasecampException.RecordingSummaryFailure(
            BasecampException.RECORDING_UNRESOLVED,
            "chat line found under no visible campfire: line $lineId in bucket $bucketId " +
                "(tried ${search.tried.size} campfires)",
            "the line may have been deleted, or its Campfire may no longer be visible to this credential",
            bucketId = bucketId,
            recordingId = lineId,
            campfireIds = search.tried.toList(),
            refreshed = refreshed,
            staleCampfireIds = stale,
        )
    }

    private fun tooManyCandidates(bucketId: Long, lineId: Long) = BasecampException.RecordingSummaryFailure(
        BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE,
        "campfire discovery incomplete: line $lineId in bucket $bucketId: " +
            "more than $MAX_CAMPFIRE_CANDIDATES visible campfires in the bucket",
        "read the line through campfires.getLine with a known Campfire id",
        bucketId = bucketId,
        recordingId = lineId,
    )
}
