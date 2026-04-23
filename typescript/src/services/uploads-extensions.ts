import type { BasecampHooks } from "../hooks.js";
import type { DownloadResult } from "../download.js";
import { Errors } from "../errors.js";
import { UploadsService as GeneratedUploadsService } from "../generated/services/uploads.js";
import type { RawClient } from "./base.js";

/** Function signature for the client-level `downloadURL` primitive. */
type DownloadURLFn = (rawURL: string) => Promise<DownloadResult>;

/**
 * UploadsService with a hand-written `download(uploadId)` convenience.
 *
 * The subclass injects the client-level `downloadURL` function so the
 * convenience method can delegate to it — the authenticated-hop + 302-follow
 * flow stays in one place (`createDownloadURL` in `download.ts`).
 */
export class UploadsService extends GeneratedUploadsService {
  private readonly downloadURLFn?: DownloadURLFn;

  // Preserve BaseService's full positional signature (authenticatedFetch, baseUrl),
  // then append downloadURLFn — otherwise existing `new UploadsService(client, hooks,
  // fetchPage, maxPages, authenticatedFetch, baseUrl)` callers would silently miswire
  // authenticatedFetch into the 5th slot. All params optional for direct-construction
  // back-compat; `download()` throws a clear usage error if the primitive wasn't
  // injected (i.e. the service was constructed outside createBasecampClient).
  constructor(
    client: RawClient,
    hooks?: BasecampHooks,
    fetchPage?: (url: string) => Promise<Response>,
    maxPages?: number,
    authenticatedFetch?: (url: string, init: RequestInit) => Promise<Response>,
    baseUrl?: string,
    downloadURLFn?: DownloadURLFn,
  ) {
    super(client, hooks, fetchPage, maxPages, authenticatedFetch, baseUrl);
    this.downloadURLFn = downloadURLFn;
  }

  /**
   * Downloads an upload's file content in one call.
   *
   * Fetches the upload metadata to retrieve `download_url`, then delegates to
   * the client-level `downloadURL` primitive (authenticated-hop + 302-follow).
   *
   * @param uploadId - The upload's numeric id.
   * @returns A `DownloadResult` whose `filename` prefers `upload.filename`
   *   from metadata, falling back to the URL-derived filename.
   * @throws {BasecampError} `usage` if the upload has no `download_url`,
   *   or if the service was constructed without the injected download
   *   primitive (obtain the service through `createBasecampClient().uploads`).
   */
  async download(uploadId: number): Promise<DownloadResult> {
    if (!this.downloadURLFn) {
      throw Errors.usage(
        "uploads.download requires the download primitive — obtain UploadsService via createBasecampClient(...).uploads rather than instantiating it directly",
      );
    }
    const upload = await this.get(uploadId);
    const url = upload.download_url;
    if (!url) {
      throw Errors.usage(`upload ${uploadId} has no download_url`);
    }
    const result = await this.downloadURLFn(url);
    return upload.filename ? { ...result, filename: upload.filename } : result;
  }
}
