import Foundation

/// Result of downloading file content from a URL.
public struct DownloadResult: Sendable {
    /// Raw file content.
    public let body: Data
    /// MIME type of the file.
    public let contentType: String
    /// Size in bytes, or -1 if unknown.
    public let contentLength: Int64
    /// Filename extracted from the URL.
    public let filename: String

    public init(body: Data, contentType: String, contentLength: Int64, filename: String) {
        self.body = body
        self.contentType = contentType
        self.contentLength = contentLength
        self.filename = filename
    }
}

/// Extracts a filename from the last path segment of a URL.
/// Falls back to "download" if the URL is unparseable or has no path segments.
public func filenameFromURL(_ rawURL: String) -> String {
    guard !rawURL.isEmpty, let url = URL(string: rawURL) else {
        return "download"
    }
    // A trailing slash means directory-style URL — no filename to extract.
    // url.path strips trailing slashes, so check the raw URL string instead.
    let pathPart = rawURL.components(separatedBy: "?").first ?? rawURL
    guard !pathPart.hasSuffix("/") else {
        return "download"
    }
    let segments = url.pathComponents.filter { $0 != "/" }
    guard let last = segments.last, !last.isEmpty, last != ".", last != "/" else {
        return "download"
    }
    return last.removingPercentEncoding ?? last
}

extension AccountClient {
    /// Downloads file content from any API-routable download URL.
    ///
    /// Handles the full download flow: URL rewriting to the configured API host,
    /// authenticated first hop (which typically 302s to a signed download URL),
    /// and unauthenticated second hop to fetch the actual file content.
    ///
    /// - Parameter rawURL: Absolute download URL (e.g., from bc-attachment elements).
    /// - Returns: A ``DownloadResult`` with body, content type, content length, and filename.
    /// - Throws: ``BasecampError/usage(message:hint:)`` if rawURL is empty or not absolute.
    public func downloadURL(_ rawURL: String) async throws -> DownloadResult {
        // Validation
        guard !rawURL.isEmpty else {
            throw BasecampError.usage(message: "download URL is required", hint: nil)
        }
        guard let parsed = URL(string: rawURL),
              let scheme = parsed.scheme,
              scheme == "http" || scheme == "https" else {
            throw BasecampError.usage(message: "download URL must be an absolute URL", hint: nil)
        }

        // Operation hooks
        let op = OperationInfo(
            service: "Account",
            operation: "DownloadURL",
            resourceType: "download",
            isMutation: false
        )
        let startTime = CFAbsoluteTimeGetCurrent()
        safeInvokeHooks { $0.onOperationStart(op) }

        do {
            // URL rewriting: replace origin with config.baseURL, preserve path+query+fragment
            guard var rewrittenComponents = URLComponents(string: client.config.baseURL) else {
                throw BasecampError.usage(message: "Invalid base URL", hint: nil)
            }
            rewrittenComponents.path = parsed.path
            rewrittenComponents.query = parsed.query
            rewrittenComponents.fragment = parsed.fragment

            guard let rewrittenURL = rewrittenComponents.string else {
                throw BasecampError.usage(message: "Failed to construct rewritten URL", hint: nil)
            }

            // Hop 1: Authenticated API request (capture redirect)
            let (data, httpResponse) = try await httpClient.performDownloadRequest(url: rewrittenURL)

            let statusCode = httpResponse.statusCode

            switch statusCode {
            case 301, 302, 303, 307, 308:
                // Redirect — extract Location, proceed to hop 2
                guard let location = httpResponse.value(forHTTPHeaderField: "Location"),
                      !location.isEmpty else {
                    throw BasecampError.api(
                        message: "redirect \(statusCode) with no Location header",
                        httpStatus: statusCode, hint: nil, requestId: nil
                    )
                }

                // Resolve relative Location against the rewritten API URL
                let resolvedLocation = resolveURL(base: rewrittenURL, target: location)

                // Hop 2: fetch from signed URL (no auth, no hooks)
                let (signedData, signedResponse) = try await httpClient.fetchSignedDownload(url: resolvedLocation)

                guard signedResponse.statusCode >= 200 && signedResponse.statusCode < 300 else {
                    throw BasecampError.api(
                        message: "download failed with status \(signedResponse.statusCode)",
                        httpStatus: signedResponse.statusCode, hint: nil, requestId: nil
                    )
                }

                let result = DownloadResult(
                    body: signedData,
                    contentType: signedResponse.value(forHTTPHeaderField: "Content-Type") ?? "",
                    contentLength: parseContentLength(signedResponse.value(forHTTPHeaderField: "Content-Length")),
                    filename: filenameFromURL(rawURL)
                )
                let durationMs = millisSince(startTime)
                safeInvokeHooks { $0.onOperationEnd(op, result: OperationResult(durationMs: durationMs)) }
                return result

            case 200..<300:
                // Direct download — no second hop
                let result = DownloadResult(
                    body: data,
                    contentType: httpResponse.value(forHTTPHeaderField: "Content-Type") ?? "",
                    contentLength: parseContentLength(httpResponse.value(forHTTPHeaderField: "Content-Length")),
                    filename: filenameFromURL(rawURL)
                )
                let durationMs = millisSince(startTime)
                safeInvokeHooks { $0.onOperationEnd(op, result: OperationResult(durationMs: durationMs)) }
                return result

            default:
                // Error response
                throw BasecampError.fromHTTPResponse(
                    status: statusCode, data: data,
                    headers: httpResponse.allHeaderFields as? [String: String] ?? [:],
                    requestId: httpResponse.value(forHTTPHeaderField: "X-Request-Id")
                )
            }
        } catch {
            let durationMs = millisSince(startTime)
            safeInvokeHooks { $0.onOperationEnd(op, result: OperationResult(durationMs: durationMs, error: error)) }
            throw error
        }
    }

    // MARK: - Private Helpers

    private func safeInvokeHooks(_ invoke: (any BasecampHooks) -> Void) {
        invoke(hooks)
    }

    private func millisSince(_ startTime: CFAbsoluteTime) -> Int {
        Int((CFAbsoluteTimeGetCurrent() - startTime) * 1000)
    }
}

/// Parse Content-Length header defensively, returning -1 for missing/invalid values.
private func parseContentLength(_ value: String?) -> Int64 {
    guard let value, !value.isEmpty else { return -1 }
    guard let parsed = Int64(value), parsed >= 0 else { return -1 }
    return parsed
}
