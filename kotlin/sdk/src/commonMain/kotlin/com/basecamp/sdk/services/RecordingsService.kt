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
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonElement

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
                readBucketId = bucketId,
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
            SummaryKind.GOOGLE_DOCUMENT ->
                untypedRecordingSummary("GetGoogleDocument", account.googleDocuments.googleDocument(id))

            SummaryKind.CLOUD_FILE -> untypedRecordingSummary("GetCloudFile", account.cloudFiles.cloudFile(id))

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

    /**
     * Projects a recording the spec models as a raw document.
     *
     * The whole projection runs inside [decodeOrApiError], the same seam every
     * generated read decodes through. The nested identities are the reason: their
     * serializers declare required members, so a payload missing one raises a
     * `SerializationException` — and unwrapped that would escape `summarize`
     * through an error contract that promises only [BasecampException], while the
     * typed reads turn the identical failure into SPEC §6's statusless
     * `api_error`. Go cannot reach this at all: unmarshalling into its pointer
     * structs leaves zero values rather than failing, so the divergence is one
     * this decode has to close rather than inherit.
     */
    private fun untypedRecordingSummary(operation: String, element: JsonElement): RecordingSummary =
        decodeOrApiError(operation) { projectUntypedRecording(element) }

    private fun projectUntypedRecording(element: JsonElement): RecordingSummary {
        // Decoded through a declared shape rather than read key by key. Reading
        // by key coerced a type mismatch instead of refusing it — a quoted
        // `"id": "7"` projected 0 and a numeric `"status": 5` projected "" —
        // which is the opposite of what routing this through the shared decode
        // seam was for: the typed reads REFUSE those bodies, and so does Go's
        // unmarshal. A declared shape refuses them here for the same reason,
        // and gets the JSON-null-is-absent handling for free, since a nullable
        // member accepts an explicit null.
        val body = json.decodeFromJsonElement(UntypedRecording.serializer(), element)
        return summaryOf(
            body.id,
            body.status,
            body.type,
            body.title,
            body.appUrl,
            body.parent,
            body.bucket,
            body.creator,
            null,
            body.description.orEmpty(),
            body.updatedAt,
        )
    }

    /**
     * The shape the two cloud-storage reads are projected from. Every member is
     * optional with a default, because the reference's unmarshal into a struct
     * leaves a zero value for an absent key rather than failing — but each is
     * TYPED, so a present key of the wrong type is refused on both sides.
     */
    @Serializable
    private data class UntypedRecording(
        val id: Long = 0,
        val status: String = "",
        val type: String = "",
        val title: String = "",
        @SerialName("app_url") val appUrl: String = "",
        @SerialName("updated_at") val updatedAt: String = "",
        val description: String? = null,
        val parent: RecordingParent? = null,
        val bucket: TodoBucket? = null,
        val creator: Person? = null,
    )

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
                // A cancelled caller must not be handed a verdict: without this,
                // a run of already-cached candidates could reach "unresolved"
                // without ever suspending. Go checks ctx.Err() here.
                currentCoroutineContext().ensureActive()
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
        // The budget decides both halves, and the two halves are NOT the same
        // rule. A source ALREADY CONSULTED is not re-read once the budget is
        // spent: it cannot hand this call a candidate it may try, so the refresh
        // is skipped and the conclusion stands on what was seen. A source NEVER
        // CONSULTED is different — candidates may exist there unsearched — so a
        // budget spent before reaching it makes the verdict incomplete, with its
        // own reason.
        //
        // Gating on `skipped` instead is subtly wrong, and it is the shape this
        // code had: `skipped` is set only when a candidate is OBSERVED and cannot
        // be tried, so a dock holding exactly the budget's worth of candidates
        // that all answer 404 leaves the budget at zero with `skipped` still
        // false. The listing would then be fetched — a request that cannot help —
        // and a failure on it replaces a deterministic `incomplete` with a
        // transient error a consumer retries forever.
        var refreshed = false
        var listedIds = listed?.ids.orEmpty()
        if (search.budget > 0 && dock.cached) {
            val again = index.dockCampfires(account, bucketId, refresh = true)
            if (again.fetched > dock.fetched || !again.cached) refreshed = true
            dock = again
            search.tryAll(dock.ids)?.let { return it }
        }
        if (search.budget <= 0) {
            if (!listCached) throw budgetSpentBeforeListing(bucketId, lineId)
        } else {
            val again = try {
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
            if (listed != null && (again.fetched > listed.fetched || !again.cached)) refreshed = true
            listedIds = again.ids
            search.tryAll(again.ids)?.let { return it }
        }
        currentCoroutineContext().ensureActive()
        if (search.skipped) throw tooManyCandidates(bucketId, lineId)

        val stale = if (refreshed) {
            search.tried.filter { it !in dock.ids && it !in listedIds }
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

    private fun budgetSpentBeforeListing(bucketId: Long, lineId: Long) = BasecampException.RecordingSummaryFailure(
        BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE,
        "campfire discovery incomplete: line $lineId in bucket $bucketId: " +
            "the candidate budget of $MAX_CAMPFIRE_CANDIDATES was spent before the account listing was consulted",
        "read the line through campfires.getLine with a known Campfire id",
        bucketId = bucketId,
        recordingId = lineId,
    )

    private fun tooManyCandidates(bucketId: Long, lineId: Long) = BasecampException.RecordingSummaryFailure(
        BasecampException.CAMPFIRE_DISCOVERY_INCOMPLETE,
        "campfire discovery incomplete: line $lineId in bucket $bucketId: " +
            "more than $MAX_CAMPFIRE_CANDIDATES visible campfires in the bucket",
        "read the line through campfires.getLine with a known Campfire id",
        bucketId = bucketId,
        recordingId = lineId,
    )
}
