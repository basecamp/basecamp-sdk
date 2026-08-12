import Foundation
import XCTest

@testable import Basecamp

/// The response decoder is isolated from everything else the request primitives
/// do (#604).
///
/// Each primitive in `BaseService` runs encode → URL build → auth → transport →
/// status check → decode inside one `do` whose terminal `catch` maps nothing.
/// That made a malformed 2xx body indistinguishable, to a caller catching
/// `BasecampError`, from the auth strategy throwing or the socket dropping: all
/// three arrived raw. Only the decode call is wrapped now, so:
///
/// - a body that does not decode is a SPEC §6 statusless, non-retryable
///   `api_error` whose message carries the decoder's own account of the failure,
///   and
/// - an auth-phase throw and a *request-body* `encoder.encode` failure — which
///   runs inside the same `do` — still surface raw.
///
/// The negative tests are what makes the isolation checkable. Wrapping the block
/// rather than the expression, or widening the mapped type past `DecodingError`,
/// reintroduces the original conflation in a new shape and is what they fail on.
final class DecodeIsolationTests: XCTestCase {

    private func transport(status: Int = 200, body: String, headers: [String: String] = [:])
        -> MockTransport
    {
        MockTransport(
            statusCode: status, data: Data(body.utf8),
            headers: headers.merging(["Content-Type": "application/json"]) { a, _ in a })
    }

    // MARK: - Positive: a malformed 2xx body is a statusless api_error

    /// `request`: the single-object primitive. `{}` is a JSON object that then
    /// fails on `Project`'s required members — a decode failure and nothing
    /// else, because the transport returned 200.
    func testMalformedBodyOnASingleGetIsAStatuslessApiError() async throws {
        let account = makeTestAccountClient(transport: transport(body: "{}"))

        do {
            _ = try await account.projects.get(projectId: 1)
            XCTFail("expected a malformed body to fail")
        } catch let error as BasecampError {
            guard case .api(let message, let httpStatus, _, _) = error else {
                return XCTFail("expected .api for a malformed body, got \(error)")
            }
            XCTAssertNil(httpStatus, "the transport succeeded, so no status describes this")
            XCTAssertFalse(error.isRetryable, "re-requesting cannot repair a malformed body")
            XCTAssertTrue(
                message.contains("GetProject returned a body that does not decode"),
                "expected the operation named in the message, got: \(message)")
            XCTAssertTrue(
                message.contains("keyNotFound") || message.contains("No value associated"),
                "the decoder's own account of the failure must survive: \(message)")
            XCTAssertLessThanOrEqual(message.count, 500, "SPEC §9 caps the message")
        } catch {
            XCTFail("expected BasecampError, got a raw \(type(of: error)): \(error)")
        }
    }

    /// `requestPaginated`: first page.
    func testMalformedFirstPageIsAStatuslessApiError() async throws {
        let account = makeTestAccountClient(transport: transport(body: "[{}]"))

        do {
            _ = try await account.projects.list()
            XCTFail("expected a malformed page to fail")
        } catch let error as BasecampError {
            guard case .api(let message, let httpStatus, _, _) = error else {
                return XCTFail("expected .api, got \(error)")
            }
            XCTAssertNil(httpStatus)
            XCTAssertFalse(error.isRetryable)
            XCTAssertTrue(
                message.contains("ListProjects returned a body that does not decode"), message)
        } catch {
            XCTFail("expected BasecampError, got a raw \(type(of: error)): \(error)")
        }
    }

    /// `requestPaginated`: the follow loop. A page decoded outside the wrap
    /// would leak raw, so the loop site needs its own isolation — the first page
    /// here is well-formed.
    func testMalformedSecondPageIsAStatuslessApiError() async throws {
        let pages = PageCounter()
        let transport = MockTransport { request in
            let url = request.url!.absoluteString
            let page = pages.next()
            if page == 1 {
                return (
                    Data("[]".utf8),
                    makeHTTPResponse(
                        url: url, statusCode: 200,
                        headers: [
                            "Content-Type": "application/json",
                            "Link":
                                "<https://3.basecampapi.com/999999999/projects.json?page=2>; rel=\"next\"",
                        ])
                )
            }
            return (
                Data("[{}]".utf8),
                makeHTTPResponse(
                    url: url, statusCode: 200, headers: ["Content-Type": "application/json"])
            )
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.projects.list()
            XCTFail("expected the malformed second page to fail")
        } catch let error as BasecampError {
            guard case .api(_, let httpStatus, _, _) = error else {
                return XCTFail("expected .api, got \(error)")
            }
            XCTAssertEqual(pages.count, 2, "the second page must actually have been fetched")
            XCTAssertNil(httpStatus)
            XCTAssertFalse(error.isRetryable)
        } catch {
            XCTFail("expected BasecampError, got a raw \(type(of: error)): \(error)")
        }
    }

    /// `requestPaginatedWrapped`: a body that is not JSON at all never reaches
    /// the typed decoder — `JSONSerialization` refuses it first, and reports a
    /// `CocoaError` rather than a `DecodingError`. Same failure, same shape.
    func testMalformedWrappedBodyIsAStatuslessApiError() async throws {
        let account = makeTestAccountClient(transport: transport(body: "not json at all"))

        do {
            _ = try await account.reports.personProgress(personId: 7)
            XCTFail("expected a body that is not JSON to fail")
        } catch let error as BasecampError {
            guard case .api(let message, let httpStatus, _, _) = error else {
                return XCTFail("expected .api, got \(error)")
            }
            XCTAssertNil(httpStatus)
            XCTAssertFalse(error.isRetryable)
            XCTAssertTrue(
                message.contains("GetPersonProgress returned a body that does not decode"), message)
        } catch {
            XCTFail("expected BasecampError, got a raw \(type(of: error)): \(error)")
        }
    }

    // MARK: - Negative: everything else in the block keeps its own identity

    /// An auth-phase throw is a credential-provider fault, not a malformed
    /// response. It surfaces raw (the strategy's own error), which is only
    /// checkable *because* the decoder no longer shares its error shape.
    func testAnAuthStrategyFailureIsNotAnApiError() async throws {
        let transport = MockTransport(statusCode: 200, data: Data("{}".utf8))
        let client = BasecampClient(
            auth: ThrowingAuth(), userAgent: "test-suite",
            config: BasecampConfig(baseURL: "https://3.basecampapi.com", enableRetry: false),
            transport: transport)

        do {
            _ = try await client.forAccount("999999999").projects.get(projectId: 1)
            XCTFail("expected the auth strategy's throw to reach the caller")
        } catch let error as BasecampError {
            XCTFail("the auth strategy's own error must not be relabelled as \(error)")
        } catch let error as AuthVaultUnreachable {
            XCTAssertEqual(error.detail, "token vault unreachable")
            XCTAssertEqual(transport.requests.count, 0, "auth failed, so nothing was ever sent")
        }
    }

    /// The wrap-the-block mistake, pinned. `try Self.encoder.encode(body)` for
    /// the *request* runs inside the same `do` as the decode, so wrapping the
    /// block would report an encoding fault as a malformed *response*.
    func testARequestBodyEncodingFailureIsNotAnApiError() async throws {
        let transport = MockTransport(statusCode: 200, data: Data("{}".utf8))
        let account = makeTestAccountClient(transport: transport)
        let service = BaseService(accountClient: account)

        do {
            let _: Project = try await service.request(
                OperationInfo(
                    service: "Test", operation: "EncodeFailure", resourceType: "test",
                    isMutation: true),
                method: "POST", path: "/test.json", body: ExplodingBody())
            XCTFail("expected the encoding failure to reach the caller")
        } catch let error as BasecampError {
            XCTFail("an encode fault must not be reported as a malformed response: \(error)")
        } catch let error as EncodingError {
            guard case .invalidValue(_, let context) = error else {
                return XCTFail("expected .invalidValue, got \(error)")
            }
            XCTAssertEqual(context.debugDescription, "request body could not be encoded")
            XCTAssertEqual(transport.requests.count, 0, "encoding failed, so nothing was sent")
        }
    }

    /// A transport failure keeps its own classification (`network`, retryable),
    /// which a decoder mapping that reached past its expression would flatten
    /// into a statusless `api_error`.
    func testATransportFailureIsNotAnApiError() async throws {
        let transport = MockTransport { _ in
            throw URLError(.notConnectedToInternet)
        }
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.projects.get(projectId: 1)
            XCTFail("expected the transport failure to surface")
        } catch let error as BasecampError {
            guard case .network = error else {
                return XCTFail("expected .network for a transport fault, got \(error)")
            }
            XCTAssertTrue(
                error.isRetryable, "a network fault is retryable; a malformed body is not")
        }
    }
}

// MARK: - Support

/// Distinct from every SDK error type, so "the strategy's own error survived"
/// is checkable rather than inferred.
private struct AuthVaultUnreachable: Error {
    let detail: String
}

private struct ThrowingAuth: AuthStrategy {
    func authenticate(_ request: inout URLRequest) async throws {
        throw AuthVaultUnreachable(detail: "token vault unreachable")
    }
}

/// Stands in for a request body whose encoding fails. Nothing in the generated
/// surface can be made to fail from outside the SDK, and the point under test is
/// *where* the decoder's mapping reaches, not which value provoked it.
private struct ExplodingBody: Encodable, Sendable {
    func encode(to encoder: any Encoder) throws {
        throw EncodingError.invalidValue(
            self,
            EncodingError.Context(
                codingPath: [], debugDescription: "request body could not be encoded"))
    }
}

private final class PageCounter: @unchecked Sendable {
    private let lock = NSLock()
    private var value = 0

    var count: Int { lock.withLock { value } }

    func next() -> Int {
        lock.withLock {
            value += 1
            return value
        }
    }
}
