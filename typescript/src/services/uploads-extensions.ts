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
  private readonly downloadURLFn: DownloadURLFn;

  constructor(
    client: RawClient,
    hooks: BasecampHooks | undefined,
    fetchPage: ((url: string) => Promise<Response>) | undefined,
    maxPages: number | undefined,
    downloadURLFn: DownloadURLFn,
  ) {
    super(client, hooks, fetchPage, maxPages);
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
   * @throws {BasecampError} `usage` if the upload has no `download_url`.
   */
  async download(uploadId: number): Promise<DownloadResult> {
    const upload = await this.get(uploadId);
    const url = upload.download_url;
    if (!url) {
      throw Errors.usage(`upload ${uploadId} has no download_url`);
    }
    const result = await this.downloadURLFn(url);
    return upload.filename ? { ...result, filename: upload.filename } : result;
  }
}
