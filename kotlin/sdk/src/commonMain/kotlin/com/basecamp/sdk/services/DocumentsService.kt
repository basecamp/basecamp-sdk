package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.models.Document
import com.basecamp.sdk.generated.services.ReplaceDocumentBody
import com.basecamp.sdk.generated.services.UpdateDocumentBody
import kotlinx.serialization.SerializationException

/**
 * A document's full writable state, the receiver of the
 * [DocumentsService.edit] block. The whole object is PUT back to the server,
 * so clearing a field means setting it empty (`""`) — there is no third
 * state. The writable set is exactly `{title, content}`.
 */
class DocumentFields internal constructor(
    /** Plain-text title. Set `""` to clear — the document then reads back as "Untitled". */
    var title: String,
    /** Rich text body (HTML). Set `""` to clear. */
    var content: String,
)

/**
 * Documents service with merge-safe [update] and read-modify-write [edit] on
 * top of the generated surface (`get`, `replace`, ...).
 *
 * The endpoint is full-replace: BC3's `DocumentsController#update` builds a
 * brand-new `Document` from only the permitted params and swaps the recordable
 * wholesale, so a sparse PUT that omits `content` erases it. Omitting `title`
 * erases that too — the document then reads back as `"Untitled"`, because
 * `Document#title` falls back when blank. Neither attribute is
 * presence-validated, so **neither omission is a 422**; both are a 200 that
 * quietly clears. What BC3 does require is the wrapping `document` object, so
 * a body naming neither field is a 400. The raw, destructive-by-design path
 * stays reachable as `replace`.
 *
 * Both composites call the public `get` and `replace` methods, so hooks
 * observe the two wire operations (`GetDocument` then `ReplaceDocument`), not
 * a synthetic composite.
 *
 * Neither is atomic: there is no conditional-update signal on this endpoint,
 * so a concurrent write between the GET and PUT is overwritten — last write
 * wins for the whole representation. The window is one round-trip. Use
 * `replace` to overwrite deliberately.
 */
class DocumentsService(client: AccountClient) :
    com.basecamp.sdk.generated.services.DocumentsService(client) {

    /**
     * Sets the given fields on a document and preserves everything else: GETs
     * the current document, overlays the explicitly-set (non-null) body
     * fields, and PUTs the full representation back. A null field is
     * untouched, guaranteed; an explicitly-passed `""` clears.
     *
     * Not atomic — see the class docs for the GET→PUT race.
     */
    suspend fun update(documentId: Long, body: UpdateDocumentBody): Document {
        val fields = fieldsFromDocument(fetchDocument(documentId))
        body.title?.let { fields.title = it }
        body.content?.let { fields.content = it }
        return putFields(documentId, fields)
    }

    /**
     * Applies a read-modify-write block to a document: GETs the current
     * document, runs the block with the full writable state
     * ([DocumentFields]) as receiver, and PUTs the whole thing back. Clearing
     * a field means setting it empty (`""`) — an untouched field keeps its
     * current value. If the block throws, the edit aborts and nothing is
     * written.
     *
     * ```kotlin
     * account.documents.edit(documentId) {
     *     title = "🚨 $title"
     *     content = "" // clearing = setting empty on a full object
     * }
     * ```
     *
     * Not atomic — see the class docs for the GET→PUT race.
     */
    suspend fun edit(documentId: Long, block: DocumentFields.() -> Unit): Document {
        val fields = fieldsFromDocument(fetchDocument(documentId))
        fields.block()
        return putFields(documentId, fields)
    }

    /**
     * GETs the document the composites read their writable state from,
     * normalizing a decode failure into the SPEC §6 shape.
     *
     * kotlinx.serialization is the typed guard the dynamic SDKs write by hand,
     * and it rejects a structurally wrong-typed field before this composite
     * ever sees it. `BaseService` now renders that as the SPEC §6
     * malformed-2xx-body shape for every operation (#604) — a statusless,
     * non-retryable [BasecampException.Api] carrying the [SerializationException]
     * as its `cause`. What the base layer cannot know is this composite's own
     * account of the failure: which record failed to decode, and the escape
     * hatch for writing it deliberately. That is what is restated here.
     *
     * The restatement is keyed off [BasecampException.Api.decodeFailure], the
     * internal slot the base layer's decoder wrapper alone fills, so any other
     * [BasecampException.Api] passes through untouched. Reading `cause is
     * SerializationException` instead would catch more than this GET's decode:
     * an auth strategy that classifies its own JSON failure that way has its
     * exception propagated untouched by `BasecampHttpClient`, and would arrive
     * here relabelled as a malformed document — for a request that was never
     * sent (#730).
     *
     * (Bare JSON scalars are refused as well as structural mismatches. They
     * were not until #598 removed the client-wide `isLenient`, which rendered a
     * number or boolean as a String before this composite could ever see it.)
     */
    private suspend fun fetchDocument(documentId: Long): Document =
        try {
            get(documentId)
        } catch (e: BasecampException.Api) {
            val decodeFailure = e.decodeFailure ?: throw e
            throw BasecampException.Api(
                message = "GetDocument returned a body that does not decode as a document: " +
                    "${decodeFailure.message}",
                hint = "The merge-safe update/edit resend this record's fields verbatim, so a " +
                    "malformed response cannot be written back safely. Use replace to write the " +
                    "record deliberately.",
                retryable = false,
                cause = decodeFailure,
            )
        }

    /**
     * Reads the writable state out of a fetched document.
     *
     * No hand-written type guard here, unlike the Todolists composite: `get`
     * returns a decoded [Document], so kotlinx.serialization has already
     * rejected a wrong-typed field before this runs — a bare scalar as well as
     * a structural mismatch, since #598 removed the client-wide `isLenient`
     * that used to render a number or boolean as a String. `content` is
     * nullable on the model — absent or JSON null is genuinely empty, and `""`
     * is what the server already holds.
     *
     * `title` is the exception, and it needs a hand-written check the decoder
     * cannot supply. The field is non-nullable on the model, so an absent or
     * null title is already refused — but `""` decodes fine, and BC3 can never
     * render it blank (`Document#title` is `super.presence || "Untitled"`).
     * A blank title on a 2xx read is therefore a malformed response, and
     * carrying it into the full-replace PUT would blank the real title on a
     * call that only touched `content`.
     */
    private fun fieldsFromDocument(document: Document): DocumentFields {
        if (document.title.isBlank()) {
            throw BasecampException.Api(
                message = "GetDocument returned a document with a blank \"title\", " +
                    "but the API never renders it blank",
                hint = "The merge-safe update/edit resend this field verbatim, so a blank value " +
                    "would blank the current one. Use replace to write the record deliberately.",
                retryable = false,
            )
        }
        return DocumentFields(title = document.title, content = document.content ?: "")
    }

    /**
     * PUTs the full writable state via `replace`. Both fields are always sent,
     * empties included: on a full-replace endpoint `""` is how a clear is
     * expressed — never JSON null (SPEC §18 body compaction), and never by
     * omission, which would leave the field to the server's own
     * clear-by-default and read as an accident rather than an intent.
     */
    private suspend fun putFields(documentId: Long, fields: DocumentFields): Document =
        replace(documentId, ReplaceDocumentBody(title = fields.title, content = fields.content))
}
