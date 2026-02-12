import Foundation
@testable import Basecamp

/// Creates a test client with a mock transport.
///
/// The client is configured with:
/// - A static test token ("test-token")
/// - User-Agent: "test-suite"
/// - Retry disabled (for deterministic tests)
/// - A localhost base URL (to skip HTTPS validation)
func makeTestClient(
    transport: MockTransport,
    enableRetry: Bool = false,
    enableCache: Bool = false,
    hooks: (any BasecampHooks)? = nil
) -> BasecampClient {
    BasecampClient(
        tokenProvider: StaticTokenProvider("test-token"),
        userAgent: "test-suite",
        config: BasecampConfig(
            baseURL: "https://3.basecampapi.com",
            enableRetry: enableRetry,
            enableCache: enableCache
        ),
        hooks: hooks,
        transport: transport
    )
}

/// Creates a test AccountClient with a mock transport.
func makeTestAccountClient(
    transport: MockTransport,
    accountId: String = "999999999",
    enableRetry: Bool = false,
    hooks: (any BasecampHooks)? = nil
) -> AccountClient {
    let client = makeTestClient(transport: transport, enableRetry: enableRetry, hooks: hooks)
    return client.forAccount(accountId)
}

/// Creates an HTTPURLResponse with the given parameters.
func makeHTTPResponse(
    url: String = "https://3.basecampapi.com/999999999/projects.json",
    statusCode: Int = 200,
    headers: [String: String] = [:]
) -> HTTPURLResponse {
    HTTPURLResponse(
        url: URL(string: url)!,
        statusCode: statusCode,
        httpVersion: "HTTP/1.1",
        headerFields: headers
    )!
}

/// Encodes a value to JSON data using the SDK's encoder.
func jsonData<T: Encodable>(_ value: T) throws -> Data {
    try BaseService.encoder.encode(value)
}
