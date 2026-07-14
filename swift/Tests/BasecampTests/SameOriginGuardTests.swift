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

    func testLocalhostCarveOutRequiresHttpScheme() {
        // The carve-out is limited to HTTP(S) so credential guards fail closed
        // on any other scheme.
        XCTAssertFalse(isLocalhost("ws://localhost:3000/x"))
        XCTAssertFalse(isLocalhost("ftp://127.0.0.1/x"))
        XCTAssertTrue(isLocalhost("HTTPS://localhost:3000/x"))
    }

    func testRedirectSanitizationStripsAuthorizationCrossOrigin() throws {
        // A cross-origin redirect must not carry the bearer token to the
        // foreign Location target; a same-origin redirect keeps it.
        let original = URL(string: "https://3.basecampapi.com/999/projects.json")!
        var foreign = URLRequest(url: URL(string: "https://evil.example/steal")!)
        foreign.setValue("Bearer test-token", forHTTPHeaderField: "Authorization")
        let stripped = sanitizedRedirectRequest(foreign, originalURL: original)
        XCTAssertNil(stripped.value(forHTTPHeaderField: "Authorization"))

        var sameOrigin = URLRequest(url: URL(string: "https://3.basecampapi.com/999/projects2.json")!)
        sameOrigin.setValue("Bearer test-token", forHTTPHeaderField: "Authorization")
        let kept = sanitizedRedirectRequest(sameOrigin, originalURL: original)
        XCTAssertEqual(kept.value(forHTTPHeaderField: "Authorization"), "Bearer test-token")
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

    // MARK: - Parser-differential regression

    /// URLs crafted so that two URL parsers may disagree about the host
    /// (backslash, userinfo, fragment, query, default-port tricks). Shared
    /// shape with the Kotlin and Python SDK test suites.
    private static let adversarialURLs = [
        "http://evil.example\\.localhost/x",
        "http://localhost@evil.example/x",
        "http://evil.example#foo.localhost",
        "http://evil.example?x=.localhost",
        "http://localhost:80@evil.example/x",
        "https://3.basecampapi.com:443@evil.example/x",
        "http://[::1]/x",
        "HTTPS://localhost/x",
        "https://3.basecampapi.com:443/x",
        "http://localhost.evil.example/x",
    ]

    private func isLoopbackHost(_ host: String) -> Bool {
        var h = host.lowercased()
        if h.hasPrefix("["), h.hasSuffix("]") { h = String(h.dropFirst().dropLast()) }
        return h == "localhost" || h == "127.0.0.1" || h == "::1" || h.hasSuffix(".localhost")
    }

    /// Hosts the token may legitimately reach: the configured base origin plus
    /// the localhost carve-out.
    private func tokenMayReach(_ host: String) -> Bool {
        host.lowercased() == "3.basecampapi.com" || isLoopbackHost(host)
    }

    /// A security guard must decide with the SAME parser the transport uses to
    /// dial (`URL(string:)`, see HTTPClient). Whenever the guard blesses a URL,
    /// the host the transport would actually dial must be the host the guard
    /// thought it blessed. Fails loudly if anyone reintroduces a second parser
    /// (e.g. URLComponents) into the guards.
    func testGuardDecidesWithTheTransportParser() {
        let base = "https://3.basecampapi.com"
        for url in Self.adversarialURLs {
            let dialed = URL(string: url)?.host?.lowercased()
            if isLocalhost(url) {
                XCTAssertNotNil(dialed, "isLocalhost blessed unparseable \(url)")
                XCTAssertTrue(
                    isLoopbackHost(dialed ?? ""),
                    "isLocalhost blessed \(url) but the transport dials \(dialed ?? "nil")")
            }
            if isSameOrigin(url, base) {
                XCTAssertEqual(
                    dialed, URL(string: base)?.host?.lowercased(),
                    "isSameOrigin blessed \(url) against \(base) but the transport dials \(dialed ?? "nil")")
            }
        }
    }

    /// End-to-end parser-differential regression: every adversarial URL, driven
    /// through the real token-attach path, must either be rejected by the guard
    /// or egress only to a host the token may reach — NEVER to a foreign host
    /// carrying Authorization.
    func testAdversarialURLsNeverEgressTokenToForeignHost() async {
        let transport = MockTransport(statusCode: 200, data: Data("[]".utf8))
        let svc = ProbeService(accountClient: makeTestAccountClient(transport: transport))
        for url in Self.adversarialURLs {
            // Rejection before egress is a passing outcome.
            do { _ = try await svc.get(url) } catch {}
        }
        for recorded in transport.requests {
            let host = recorded.request.url?.host?.lowercased() ?? ""
            let auth = recorded.request.value(forHTTPHeaderField: "Authorization")
            XCTAssertTrue(
                tokenMayReach(host) || auth == nil,
                "Bearer token egressed to foreign host \(host)")
        }
    }

    func testGuardsFailClosedOnRelativeInput() {
        // A scheme-less string is not an absolute URL: neither guard may bless it.
        XCTAssertFalse(isLocalhost("localhost"))
        XCTAssertFalse(isLocalhost("evil.example/x"))
        XCTAssertFalse(isSameOrigin("3.basecampapi.com", "https://3.basecampapi.com"))
        XCTAssertFalse(isSameOrigin("https://3.basecampapi.com", "3.basecampapi.com"))
    }
}
