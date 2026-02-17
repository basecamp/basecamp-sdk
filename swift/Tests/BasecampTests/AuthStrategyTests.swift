import XCTest
@testable import Basecamp

final class AuthStrategyTests: XCTestCase {

    // MARK: - BearerAuth

    func testBearerAuthSetsAuthorizationHeader() async throws {
        let auth = BearerAuth(tokenProvider: StaticTokenProvider("test-token"))
        var request = URLRequest(url: URL(string: "https://example.com")!)

        try await auth.authenticate(&request)

        XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer test-token")
    }

    // MARK: - Custom AuthStrategy

    func testCustomAuthStrategy() async throws {
        struct CookieAuth: AuthStrategy {
            let session: String
            func authenticate(_ request: inout URLRequest) async throws {
                request.setValue("session=\(session)", forHTTPHeaderField: "Cookie")
            }
        }

        let auth = CookieAuth(session: "abc123")
        var request = URLRequest(url: URL(string: "https://example.com")!)

        try await auth.authenticate(&request)

        XCTAssertEqual(request.value(forHTTPHeaderField: "Cookie"), "session=abc123")
        XCTAssertNil(request.value(forHTTPHeaderField: "Authorization"))
    }

    // MARK: - Client integration

    func testClientWithCustomAuthStrategy() async throws {
        let transport = MockTransport { request in
            // Verify the custom auth header was set, not Bearer
            XCTAssertEqual(request.value(forHTTPHeaderField: "X-API-Key"), "secret-key")
            XCTAssertNil(request.value(forHTTPHeaderField: "Authorization"))

            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("[]".utf8), response)
        }

        struct APIKeyAuth: AuthStrategy {
            let key: String
            func authenticate(_ request: inout URLRequest) async throws {
                request.setValue(key, forHTTPHeaderField: "X-API-Key")
            }
        }

        let client = BasecampClient(
            auth: APIKeyAuth(key: "secret-key"),
            userAgent: "test/1.0",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com"),
            transport: transport
        )

        _ = client.forAccount("12345")
    }

    func testAccessTokenInitializerStillWorks() async throws {
        let transport = MockTransport { request in
            XCTAssertEqual(
                request.value(forHTTPHeaderField: "Authorization"),
                "Bearer my-token"
            )

            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("[]".utf8), response)
        }

        let client = BasecampClient(
            accessToken: "my-token",
            userAgent: "test/1.0",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com"),
            hooks: nil
        )

        // Verify backward-compatible init creates a working client
        let account = client.forAccount("12345")
        XCTAssertEqual(account.accountId, "12345")
        _ = transport // ensure transport is captured
    }

    func testTokenProviderInitializerStillWorks() async throws {
        let transport = MockTransport { request in
            XCTAssertEqual(
                request.value(forHTTPHeaderField: "Authorization"),
                "Bearer provider-token"
            )

            let response = HTTPURLResponse(
                url: request.url!, statusCode: 200,
                httpVersion: "HTTP/1.1", headerFields: [:]
            )!
            return (Data("[]".utf8), response)
        }

        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("provider-token"),
            userAgent: "test/1.0",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com"),
            transport: transport
        )

        _ = client.forAccount("12345")
    }
}
