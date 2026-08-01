package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.DownloadResult
import com.basecamp.sdk.downloadURL

/**
 * Uploads service with a [download] convenience on top of the generated
 * surface (`get`, `update`, ...).
 *
 * [download] composes the public `get` method and the client-level
 * [downloadURL] primitive, so hooks observe the two wire operations
 * (`GetUpload`, then `DownloadURL`'s authenticated-hop + 302-follow flow)
 * rather than a synthetic composite — the same delegation shape as the
 * TypeScript SDK's `uploads.download`.
 */
class UploadsService(private val client: AccountClient) :
    com.basecamp.sdk.generated.services.UploadsService(client) {

    /**
     * Downloads an upload's file content in one call.
     *
     * Fetches the upload metadata to read `download_url`, then delegates to
     * [downloadURL] (authenticated first hop, unauthenticated signed second
     * hop). The result's filename prefers the upload's `filename` from
     * metadata, falling back to the URL-derived name.
     *
     * @param uploadId The upload's numeric id.
     * @throws BasecampException.Usage if the upload has no `download_url`.
     */
    suspend fun download(uploadId: Long): DownloadResult {
        val upload = get(uploadId)
        val url = upload.downloadUrl
        if (url.isNullOrEmpty()) {
            throw BasecampException.Usage("upload $uploadId has no download_url")
        }
        val result = client.downloadURL(url)
        return upload.filename?.takeIf { it.isNotEmpty() }
            ?.let { result.copy(filename = it) }
            ?: result
    }
}
