// Hand-written merge-safe update / edit surface for DocumentsService.
import Foundation

/// Request parameters for the merge-safe ``DocumentsService/update(documentId:req:)``.
/// Every field is optional: a `nil` field is left untouched on the document,
/// guaranteed. An explicitly-passed empty string is a set (clears the field).
public struct UpdateDocumentRequest: Codable, Sendable {
    public var content: String?
    public var title: String?

    public init(
        content: String? = nil,
        title: String? = nil
    ) {
        self.content = content
        self.title = title
    }
}

/// A document's full writable state, handed to the
/// ``DocumentsService/edit(documentId:_:)`` closure. The whole value is PUT
/// back to the server, so clearing a field means setting it empty (`""`) —
/// there is no third state. BC3's writable set on this endpoint is exactly
/// `{title, content}`; everything else on a document is server-owned or has its
/// own endpoint (position, status, subscriptions, client visibility).
public struct DocumentFields: Sendable {
    /// Plain-text title. Set `""` to clear — the document then reads back as
    /// "Untitled", because `Document#title` falls back when blank.
    public var title: String
    /// Rich text body (HTML). Set `""` to clear.
    public var content: String

    /// `title` needs a hand-written check the decoder cannot supply. The field
    /// is non-optional on the model, so an absent or null title is already
    /// refused — but `""` decodes fine, and BC3 can never render it blank
    /// (`Document#title` is `super.presence || "Untitled"`). A blank title on a
    /// 2xx read is therefore a malformed response, and carrying it into the
    /// full-replace PUT would blank the real title on a call that only touched
    /// `content`.
    init(from document: Document) throws {
        guard !document.title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw BasecampError.api(
                message: "GetDocument returned a document with a blank \"title\", "
                    + "but the API never renders it blank",
                httpStatus: nil,
                hint: "The merge-safe update/edit resend this field verbatim, so a blank value "
                    + "would blank the current one. Use replace(documentId:req:) to write the "
                    + "record deliberately.",
                requestId: nil,
                decodeFailure: nil
            )
        }
        title = document.title
        content = document.content ?? ""
    }
}

// Merge-safe `update` and read-modify-write `edit`, composed from the public
// `get` and `replace` methods — hooks observe the two wire operations
// (`GetDocument` then `ReplaceDocument`), not a synthetic composite.
//
// `PUT /{accountId}/documents/{documentId}` is a full replace: BC3's
// `DocumentsController#update` builds a brand-new `Document` from only the
// permitted params and swaps the recordable wholesale, so a PUT that omits
// `content` ERASES it, and one that omits `title` erases that too. Neither
// attribute is presence-validated, so neither omission is a 422 — both are a
// 200 that quietly clears. That makes the natural sparse write destructive on
// the raw endpoint, which stays available as `replace`.
//
// Neither composite is atomic: there is no conditional-update signal on this
// endpoint, so a concurrent write between the GET and PUT is overwritten — last
// write wins for the whole representation. The window is one round-trip. Use
// `replace` to overwrite deliberately.
extension DocumentsService {
    /// Sets the given fields on a document and preserves everything else: GETs
    /// the current document, overlays the explicitly-set (non-`nil`) request
    /// fields, and PUTs the full representation back. A `nil` field is
    /// untouched, guaranteed; an explicitly-passed `""` clears.
    ///
    /// Not atomic — see the extension docs for the GET→PUT race.
    public func update(documentId: Int, req: UpdateDocumentRequest) async throws -> Document {
        var fields = try DocumentFields(from: try await fetchDocument(documentId: documentId))
        if let title = req.title { fields.title = title }
        if let content = req.content { fields.content = content }
        return try await putFields(documentId: documentId, fields: fields)
    }

    /// Applies a read-modify-write closure to a document: GETs the current
    /// document, hands the closure the full writable representation
    /// (``DocumentFields``), and PUTs the whole thing back. Clearing a field
    /// means setting it empty (`""`) — an untouched field keeps its current
    /// value. If the closure throws, the edit aborts and nothing is written.
    ///
    /// ```swift
    /// try await account.documents.edit(documentId: 123) {
    ///     $0.title = "🚨 " + $0.title
    ///     $0.content = "" // clearing = setting empty on a full object
    /// }
    /// ```
    ///
    /// Not atomic — see the extension docs for the GET→PUT race.
    public func edit(
        documentId: Int, _ mutate: (inout DocumentFields) throws -> Void
    ) async throws -> Document {
        var fields = try DocumentFields(from: try await fetchDocument(documentId: documentId))
        try mutate(&fields)
        return try await putFields(documentId: documentId, fields: fields)
    }

    /// PUTs the full writable state via `replace`. Both fields are ALWAYS sent,
    /// empties included: `""` is how a clear is expressed on a full-replace
    /// endpoint, and an explicit JSON null is never sent (SPEC §18 body
    /// compaction). Neither field is presence-validated server-side, so there
    /// is nothing to reject here — an empty title is a legitimate clear that
    /// reads back as "Untitled".
    private func putFields(documentId: Int, fields: DocumentFields) async throws -> Document {
        try await replace(
            documentId: documentId,
            req: ReplaceDocumentRequest(content: fields.content, title: fields.title)
        )
    }

    /// GETs the document the composites read their writable state from.
    private func fetchDocument(documentId: Int) async throws -> Document {
        // Swift's decoder is the typed guard the dynamic SDKs have to write by
        // hand, and it rejects a wrong-typed field before this composite ever
        // sees it. `BaseService.decoding(_:_:)` now renders that as the SPEC §6
        // malformed-2xx-body shape for every operation (#604) — statusless,
        // non-retryable `api_error` — so what is left to add here is the part
        // the base layer cannot know: the composite's escape hatch.
        // `BasecampError.decodeFailure` is what recognizes that one failure —
        // statuslessness alone would also match the pagination same-origin
        // guard — so every other error passes through untouched. The restatement
        // carries the marker forward: it is the same malformed body with a
        // better hint, and dropping it would tell the conformance runner (and
        // any caller) this was not a decode failure (#750).
        do {
            return try await get(documentId: documentId)
        } catch let error as BasecampError {
            guard let decodeFailure = error.decodeFailure else { throw error }
            throw BasecampError.api(
                message: error.message,
                httpStatus: nil,
                hint: "The merge-safe update/edit resend this record's fields verbatim, so a "
                    + "malformed response cannot be written back safely. Use "
                    + "replace(documentId:req:) to write the record deliberately.",
                requestId: nil,
                decodeFailure: decodeFailure
            )
        }
    }
}
