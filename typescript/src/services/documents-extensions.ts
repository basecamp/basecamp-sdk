import { DocumentsService as GeneratedDocumentsService } from "../generated/services/documents.js";
import type { Document } from "../generated/services/documents.js";
import { requireRecord, requiredWritableString, writableString } from "./merge-safe.js";

/** The deliberate-overwrite escape hatch named in this composite's error hints. */
const ESCAPE = "replace()";

/**
 * Request parameters for update. Both fields are optional: an omitted field
 * is left untouched on the document, guaranteed. An explicitly-passed empty
 * string is a set (clears the field).
 */
export interface UpdateDocumentRequest {
  /** Plain-text title. Omit to leave unchanged; `""` clears it. */
  title?: string;
  /** Rich text body (HTML). Omit to leave unchanged; `""` clears it. */
  content?: string;
}

/**
 * A document's full writable state, handed to the `edit` callback. The whole
 * object is PUT back to the server, so clearing a field means setting it empty
 * (`""`) — there is no third state. The writable set is exactly
 * `{title, content}`.
 */
export interface DocumentFields {
  /** Plain-text title. Set `""` to clear — the document then reads back as "Untitled". */
  title: string;
  /** Rich text body (HTML). Set `""` to clear. */
  content: string;
}

/**
 * DocumentsService with merge-safe `update` and read-modify-write `edit` on
 * top of the generated surface (`get`, `replace`, ...).
 *
 * `PUT /{accountId}/documents/{documentId}` is a full replace: BC3's
 * `DocumentsController#update` builds a brand-new `Document` from only the
 * permitted params and swaps the recordable wholesale, so a sparse PUT that
 * omits `content` erases it. Omitting `title` erases that too — the document
 * then reads back as `"Untitled"`, because `Document#title` falls back when
 * blank. Neither field is presence-validated, so **neither omission is a 422**;
 * both are a `200` that quietly clears. What BC3 does require is the wrapping
 * `document` object, so a body naming neither field is a `400`.
 *
 * Both methods compose the public `get` and `replace` methods, so hooks
 * observe the two wire operations (`GetDocument` then `ReplaceDocument`), not
 * a synthetic composite.
 *
 * Publishing a draft (`status: "active"`) is not part of this surface: the
 * spec models only `title` and `content`, and BC3 rejects a status-only update
 * for the same reason it rejects an empty body.
 */
export class DocumentsService extends GeneratedDocumentsService {
  /**
   * Sets the given fields on a document and preserves everything else: GETs
   * the current document, overlays the explicitly-set request fields, and PUTs
   * the full representation back. An omitted (`undefined`) field is untouched,
   * guaranteed; an explicitly-passed `""` clears.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so a
   * concurrent write between the GET and PUT is overwritten — last write wins
   * for the whole representation. The window is one round-trip. Use `replace`
   * to overwrite deliberately.
   *
   * @param documentId - The document ID
   * @param req - Fields to set; omitted fields are preserved
   * @returns The updated Document
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * // Retitle without erasing the body.
   * await client.documents.update(123, { title: "Q3 Plan" });
   * ```
   */
  async update(documentId: number, req: UpdateDocumentRequest): Promise<Document> {
    const fields = await this.currentFields(documentId);
    if (req.title !== undefined) fields.title = req.title;
    if (req.content !== undefined) fields.content = req.content;
    return this.putFields(documentId, fields);
  }

  /**
   * Applies a read-modify-write callback to a document: GETs the current
   * document, hands the callback its full writable representation, and PUTs
   * the whole thing back. Clearing a field means setting it empty (`""`) — an
   * untouched field keeps its current value. If the callback throws (or
   * rejects), the edit aborts and nothing is written.
   *
   * Not atomic: there is no conditional-update signal on this endpoint, so a
   * concurrent write between the GET and PUT is overwritten — last write wins
   * for the whole representation. The window is one round-trip. Use `replace`
   * to overwrite deliberately.
   *
   * @param documentId - The document ID
   * @param fn - Callback that mutates the document's writable fields in place
   * @returns The updated Document
   * @throws {BasecampError} If the request fails
   *
   * @example
   * ```ts
   * await client.documents.edit(123, (d) => {
   *   d.title = `🚨 ${d.title}`;
   *   d.content = ""; // clearing = setting empty on a full object
   * });
   * ```
   */
  async edit(
    documentId: number,
    fn: (d: DocumentFields) => void | Promise<void>
  ): Promise<Document> {
    const fields = await this.currentFields(documentId);
    await fn(fields);
    return this.putFields(documentId, fields);
  }

  /**
   * Fetches the document and derives its full writable state.
   *
   * Every value here is resent in the full-replace PUT, so every value is
   * validated before it is read. `?? ""` coalesces only `null` and
   * `undefined`, leaving corruption wide open: `false`, `0`, `[]`, `{}`, `42`,
   * `true` and friends would all be forwarded **verbatim** and written to the
   * document on a call that never mentioned the field. Nothing below this
   * rejects them — `schema.d.ts` is erased at build time, so `Document` is a
   * compile-time claim about runtime data. See `merge-safe.ts` and #576.
   *
   * The two writable fields read differently because the spec models them
   * differently: `title` is `@required`, so absent or null is malformed;
   * `content` is optional, so absent or null is a genuinely empty body.
   */
  private async currentFields(documentId: number): Promise<DocumentFields> {
    const current = requireRecord(await this.get(documentId), {
      record: "Document",
      operation: "GetDocument",
      escape: ESCAPE,
    });
    const opts = { record: "Document", escape: ESCAPE };
    return {
      title: requiredWritableString(current, "title", opts),
      content: writableString(current, "content", opts),
    };
  }

  /**
   * PUTs the full writable state via `replace`. Both fields are always sent,
   * empties included: on a full-replace endpoint `""` is how a clear is
   * expressed — never JSON null (SPEC §18 body compaction), and never by
   * omission, which would leave the field to the server's own clear-by-default
   * and read as an accident rather than an intent.
   */
  private putFields(documentId: number, f: DocumentFields): Promise<Document> {
    return this.replace(documentId, { title: f.title, content: f.content });
  }
}
