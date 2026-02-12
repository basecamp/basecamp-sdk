import XCTest
@testable import Basecamp

final class AccountClientTests: XCTestCase {

    func testServiceCacheReturnsSameInstance() {
        let transport = MockTransport(statusCode: 200)
        let account = makeTestAccountClient(transport: transport)

        // Use the public service cache API
        let service1: String = account.service("test") { "instance-\(UUID())" }
        let service2: String = account.service("test") { "should-not-be-called" }

        XCTAssertEqual(service1, service2, "Service cache should return the same instance")
    }

    func testServiceCacheDifferentKeys() {
        let transport = MockTransport(statusCode: 200)
        let account = makeTestAccountClient(transport: transport)

        let a: String = account.service("a") { "service-a" }
        let b: String = account.service("b") { "service-b" }

        XCTAssertEqual(a, "service-a")
        XCTAssertEqual(b, "service-b")
    }

    func testBaseURLIncludesAccountId() {
        let transport = MockTransport(statusCode: 200)
        let account = makeTestAccountClient(transport: transport, accountId: "42")

        XCTAssertEqual(account.baseURL, "https://3.basecampapi.com/42")
    }
}
