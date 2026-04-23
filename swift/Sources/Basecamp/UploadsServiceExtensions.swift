import Foundation

extension UploadsService {
    /// Downloads an upload's file content in one call.
    ///
    /// Fetches the upload metadata to retrieve `download_url`, then delegates to
    /// ``AccountClient/downloadURL(_:)`` so the authenticated-hop + 302-follow
    /// flow lives in one place.
    ///
    /// - Parameter uploadId: The upload's numeric id.
    /// - Returns: A ``DownloadResult`` with body, content type, content length,
    ///   and filename. The filename prefers `upload.filename` from the metadata
    ///   response and falls back to the URL-derived filename.
    /// - Throws: ``BasecampError/usage(message:hint:)`` if the upload has no
    ///   `download_url`; other ``BasecampError`` cases for network/API errors.
    public func download(uploadId: Int) async throws -> DownloadResult {
        let upload = try await get(uploadId: uploadId)
        guard let url = upload.downloadUrl, !url.isEmpty else {
            throw BasecampError.usage(
                message: "upload \(uploadId) has no download_url",
                hint: nil
            )
        }
        let result = try await accountClient.downloadURL(url)
        guard let filename = upload.filename, !filename.isEmpty else {
            return result
        }
        return DownloadResult(
            body: result.body,
            contentType: result.contentType,
            contentLength: result.contentLength,
            filename: filename
        )
    }
}
