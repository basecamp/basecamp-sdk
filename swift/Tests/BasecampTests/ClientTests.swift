import XCTest
@testable import Basecamp

final class ClientTests: XCTestCase {

    func testForAccountReturnsAccountClient() {
        let transport = MockTransport(statusCode: 200)
        let client = makeTestClient(transport: transport)

        let account = client.forAccount("12345")
        XCTAssertEqual(account.accountId, "12345")
    }

    func testForAccountBaseURL() {
        let transport = MockTransport(statusCode: 200)
        let client = makeTestClient(transport: transport)

        let account = client.forAccount("12345")
        XCTAssertEqual(account.baseURL, "https://3.basecampapi.com/12345")
    }

    func testBasecampConfigDefaults() {
        let config = BasecampConfig()
        XCTAssertEqual(config.baseURL, "https://3.basecampapi.com")
        XCTAssertTrue(config.enableRetry)
        XCTAssertFalse(config.enableCache)
        XCTAssertEqual(config.maxPages, 10_000)
        XCTAssertEqual(config.timeoutInterval, 30)
    }

    // The rejected side of this bound (`maxPages <= 0`) is enforced with
    // `precondition`, matching how the SDK rejects a non-HTTPS base URL and a
    // non-numeric account ID. A precondition traps the process, so XCTest cannot
    // observe it in-process — there is no death-test facility here, and neither
    // of the existing preconditions is covered either. What is asserted is the
    // accepted side: a positive cap survives construction and reaches the client
    // and its services unchanged, and an omitted one keeps the default.
    func testBasecampConfigAcceptsPositiveMaxPages() {
        XCTAssertEqual(BasecampConfig(maxPages: 1).maxPages, 1)
        XCTAssertEqual(BasecampConfig(maxPages: 25).maxPages, 25)
    }

    func testClientPropagatesMaxPagesToAccountClient() {
        let client = BasecampClient(
            accessToken: "test-token",
            userAgent: "test/1.0",
            config: BasecampConfig(maxPages: 25)
        )
        XCTAssertEqual(client.config.maxPages, 25)
        XCTAssertEqual(client.forAccount("12345").maxPages, 25)
    }

    func testOmittedMaxPagesKeepsTheDefault() {
        let client = BasecampClient(
            accessToken: "test-token",
            userAgent: "test/1.0"
        )
        XCTAssertEqual(client.config.maxPages, 10_000)
        XCTAssertEqual(client.forAccount("12345").maxPages, 10_000)
    }

    func testBasecampConfigStripsTrailingSlash() {
        let config = BasecampConfig(baseURL: "https://example.com/")
        XCTAssertEqual(config.baseURL, "https://example.com")
    }

    func testStaticTokenProvider() async throws {
        let provider = StaticTokenProvider("my-token")
        let token = try await provider.accessToken()
        XCTAssertEqual(token, "my-token")
    }
}
