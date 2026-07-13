import XCTest
@testable import Basecamp

/// Verifies the same-origin credential guard: the bearer token is attached only
/// to the configured origin (localhost carve-out for dev/test). A foreign-origin
/// absolute URL must error before any network send, so the mock transport
/// records no request (no Authorization egress). Drives `BaseService`'s
/// `buildURL` chokepoint via a tiny in-test subclass.
final class SameOriginGuardTests: XCTestCase {
    final class ProbeService: BaseService {
        func get(_ path: String) async throws -> [String] {
            try await request(
                OperationInfo(service: "Probe", operation: "Get", resourceType: "probe", isMutation: false),
                method: "GET", path: path)
        }
    }

    func testForeignOriginRejectedNoEgress() async throws {
        let transport = MockTransport(statusCode: 200, data: Data("[]".utf8))
        let svc = ProbeService(accountClient: makeTestAccountClient(transport: transport))
        do { _ = try await svc.get("https://evil.example/steal.json"); XCTFail("expected rejection") }
        catch let error as BasecampError { guard case .usage = error else { return XCTFail("got \(error)") } }
        catch { XCTFail("expected BasecampError.usage, got \(error)") }
        XCTAssertTrue(transport.requests.isEmpty)
    }

    func testSameOriginAbsoluteCarriesToken() async throws {
        let transport = MockTransport(statusCode: 200, data: Data("[]".utf8))
        let svc = ProbeService(accountClient: makeTestAccountClient(transport: transport))
        _ = try await svc.get("https://3.basecampapi.com/999999999/projects.json")
        XCTAssertEqual(transport.requests.count, 1)
        XCTAssertEqual(transport.lastRequest?.request.value(forHTTPHeaderField: "Authorization"), "Bearer test-token")
    }

    func testLocalhostAbsoluteAllowed() async throws {
        let transport = MockTransport(statusCode: 200, data: Data("[]".utf8))
        let svc = ProbeService(accountClient: makeTestAccountClient(transport: transport))
        _ = try await svc.get("https://localhost:8080/x.json")
        XCTAssertEqual(transport.requests.count, 1)
    }

    func testLocalhostAllowsPlainHTTP() async throws {
        // Localhost may use plain HTTP for local development.
        let transport = MockTransport(statusCode: 200, data: Data("[]".utf8))
        let svc = ProbeService(accountClient: makeTestAccountClient(transport: transport))
        _ = try await svc.get("http://localhost:8080/x.json")
        XCTAssertEqual(transport.requests.count, 1)
    }

    func testForeignOriginPlainHTTPRejectedNoEgress() async throws {
        // A non-localhost http:// target must still be rejected (HTTPS required),
        // with no token egress.
        let transport = MockTransport(statusCode: 200, data: Data("[]".utf8))
        let svc = ProbeService(accountClient: makeTestAccountClient(transport: transport))
        do { _ = try await svc.get("http://evil.example/steal.json"); XCTFail("expected rejection") }
        catch let error as BasecampError { guard case .usage = error else { return XCTFail("got \(error)") } }
        catch { XCTFail("expected BasecampError.usage, got \(error)") }
        XCTAssertTrue(transport.requests.isEmpty)
    }
}
