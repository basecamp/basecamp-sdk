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
