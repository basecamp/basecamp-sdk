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
            guard case .api(let message, let httpStatus, _, _, _) = error else {
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
            guard case .api(let message, let httpStatus, _, _, _) = error else {
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
            guard case .api(_, let httpStatus, _, _, _) = error else {
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
            guard case .api(let message, let httpStatus, _, _, _) = error else {
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

    // MARK: - The wrapped-pagination envelope, member by member (#728)

    /// `GetPersonProgress` is the SDK's only wrapped-pagination operation, and
    /// its envelope used to be half-decoded outside the primitive: `person` by
    /// GENERATED code running after `requestPaginatedWrapped` returned, so a
    /// missing or wrong-typed one threw a raw `DecodingError`. One case was
    /// worse than raw: an absent `events` alongside a valid `person`, where
    /// `decodeWrappedItems` answered with an EMPTY LIST and the wrapper decode
    /// was then happy with the rest, so the whole operation SUCCEEDED on a
    /// response the SDK had not understood — where Kotlin threw.
    ///
    /// That is the only silent success, and the tests below are careful about
    /// it. A non-object body reached the same `?? [:]` fallback, but the wrapper
    /// decode rejected it a line later, so that case failed — just unmapped.
    ///
    /// Absence is malformed, not empty, on BC3's authority:
    /// `app/views/api/users/timelines/show.json.jbuilder` is two unconditional
    /// `json.` lines — `person` and `events` — with no `if` between them, so
    /// every member below is always on the wire.

    /// The control: both members present, so the envelope decodes.
    func testAWellFormedWrapperStillDecodes() async throws {
        let account = makeTestAccountClient(
            transport: transport(
                body: #"{"person":{"id":45678,"name":"Victor Cooper"},"events":[]}"#))

        let result = try await account.reports.personProgress(personId: 7)

        XCTAssertEqual(result.person.id, 45678)
        XCTAssertEqual(result.events.count, 0)
    }

    /// **The silent one.** With `person` valid and only `events` absent, nothing
    /// downstream objected, so this body used to come back as a successful read
    /// of zero events. The `person` in this fixture is deliberately well-formed:
    /// break it and the wrapper decode fails instead, and the case stops being
    /// about the items key at all.
    func testAnAbsentItemsKeyIsAStatuslessApiError() async throws {
        let message = try await personProgressFailureMessage(
            #"{"person":{"id":45678,"name":"Victor Cooper"}}"#)

        assertNamesTheMember(message, "events", saying: "is absent")
    }

    /// An absent `person` used to throw a raw `DecodingError.keyNotFound`.
    func testAnAbsentWrapperMemberIsAStatuslessApiError() async throws {
        let message = try await personProgressFailureMessage(#"{"events":[]}"#)

        XCTAssertTrue(
            message.contains("person"), "the decoder's own account must survive: \(message)")
    }

    /// A wrong-typed `person` used to throw a raw `DecodingError.typeMismatch`.
    func testAWrongTypedWrapperMemberIsAStatuslessApiError() async throws {
        _ = try await personProgressFailureMessage(#"{"events":[],"person":42}"#)
    }

    /// The one case here that was **already** an `api_error`: `{}` is valid
    /// input to `JSONSerialization.data(withJSONObject:)`, so the old path
    /// serialized it and the `[T]` decoder then rejected it. What the guard adds
    /// is a message that names the member — and cover for the values `{}` does
    /// not stand in for. A string or a number at `events` is not a valid
    /// top-level JSON object, and `data(withJSONObject:)` answers that with an
    /// `NSInvalidArgumentException`, which is not a Swift error and cannot be
    /// caught. Refusing the shape first is what keeps that unreachable.
    func testANonArrayItemsKeyIsAStatuslessApiError() async throws {
        let message = try await personProgressFailureMessage(
            #"{"person":{"id":45678,"name":"Victor Cooper"},"events":{}}"#)

        assertNamesTheMember(message, "events", saying: "not an array")
    }

    /// The case `{}` above does **not** stand in for, and the one the type guard
    /// genuinely exists for. A scalar at `events` is not a valid top-level JSON
    /// object, so the old path reached
    /// `JSONSerialization.data(withJSONObject:)` with a value it answers by
    /// raising `NSInvalidArgumentException` — an Objective-C exception, which is
    /// not a Swift error and which no `catch` in this SDK can see.
    ///
    /// So this test does not assert an error type so much as assert that the
    /// process is still alive to report one: delete the guard and it does not
    /// fail, it takes the test runner down with it. That is the whole reason it
    /// is a separate case from the dictionary above.
    func testAScalarItemsKeyIsRefusedRatherThanCrashing() async throws {
        let message = try await personProgressFailureMessage(
            #"{"person":{"id":45678,"name":"Victor Cooper"},"events":42}"#)

        assertNamesTheMember(message, "events", saying: "not an array")
    }

    /// A top-level array — valid JSON, wrong shape. `as? [String: Any] ?? [:]`
    /// swallowed it far enough to yield an empty items list, but the wrapper
    /// decode then rejected the same body, so this case *did* fail before —
    /// with a raw `DecodingError` about `person`, naming the wrong member. What
    /// changes is that it is now mapped, and refused for what is actually wrong
    /// with it.
    func testANonObjectWrappedBodyIsAStatuslessApiError() async throws {
        let message = try await personProgressFailureMessage(#"[{"id":1}]"#)

        XCTAssertTrue(
            message.contains("not a JSON object"), "expected the shape refusal, got: \(message)")
    }

    /// The ordering `finishWrapped` exists to hold: the wrapper decode runs
    /// BEFORE `onOperationEnd`, so a malformed wrapper is reported once, as a
    /// failure.
    ///
    /// Without this, moving the hook call ahead of `decoding(_:_:)` — the exact
    /// regression the helper is written to prevent, and what the generated code
    /// did before this change — would pass every other test in this file,
    /// because they only read the thrown error. Both halves are one test on
    /// purpose: the claim is that the hook reports what actually happened, so
    /// the well-formed body has to be shown reporting a SUCCESS through the same
    /// path, or "always attaches an error" would satisfy the malformed half.
    func testAMalformedWrapperIsReportedToHooksOnceAsAFailure() async throws {
        let badSpy = EndSpy()
        let bad = makeTestAccountClient(
            transport: transport(body: #"{"events":[]}"#), hooks: badSpy)
        do {
            _ = try await bad.reports.personProgress(personId: 7)
            XCTFail("expected a malformed wrapper to fail")
        } catch is BasecampError {}

        XCTAssertEqual(
            badSpy.ends.count, 1, "a malformed wrapper must end the operation exactly once")
        guard let recorded = badSpy.ends.first, let reported = recorded else {
            return XCTFail("the end event must carry the mapped error, not report a success")
        }
        XCTAssertTrue(reported is BasecampError, "got a raw \(type(of: reported))")

        let goodSpy = EndSpy()
        let good = makeTestAccountClient(
            transport: transport(
                body: #"{"person":{"id":45678,"name":"Victor Cooper"},"events":[]}"#),
            hooks: goodSpy)
        _ = try await good.reports.personProgress(personId: 7)

        XCTAssertEqual(goodSpy.ends.count, 1)
        XCTAssertNil(
            goodSpy.ends.first ?? nil, "a well-formed wrapper must still report a success")
    }

    /// Asserts the message names the member and says what was wrong with it,
    /// **without quoting the quotes around it**. The guards in `BaseService`
    /// write `'events'`, but the message reaches here through
    /// `String(describing: DecodingError)`, and that rendering is a toolchain
    /// detail rather than a contract. Swift 6.4 has a `CustomStringConvertible`
    /// for `DecodingError` and prints the `debugDescription` verbatim; the CI
    /// toolchain falls back to reflecting the enum, which escapes the nested
    /// string's apostrophes to `\'`. An assertion on `'events'` therefore passed
    /// here and failed there while both SDKs behaved identically — a test
    /// coupled to the renderer, not to the behaviour. The member name and the
    /// complaint survive either rendering.
    private func assertNamesTheMember(
        _ message: String, _ member: String, saying complaint: String,
        file: StaticString = #filePath, line: UInt = #line
    ) {
        XCTAssertTrue(
            message.contains(member), "the member must be named, got: \(message)", file: file,
            line: line)
        XCTAssertTrue(
            message.contains(complaint), "expected \"\(complaint)\", got: \(message)", file: file,
            line: line)
    }

    /// Drives `GetPersonProgress` against `body` and asserts the SPEC §6
    /// statusless shape, returning the message so each case can pin what the
    /// decoder said about its own member.
    private func personProgressFailureMessage(
        _ body: String, file: StaticString = #filePath, line: UInt = #line
    ) async throws -> String {
        let account = makeTestAccountClient(transport: transport(body: body))

        do {
            _ = try await account.reports.personProgress(personId: 7)
            XCTFail("expected a malformed wrapper to fail", file: file, line: line)
            return ""
        } catch let error as BasecampError {
            guard case .api(let message, let httpStatus, _, _, let decodeFailure) = error else {
                XCTFail("expected .api, got \(error)", file: file, line: line)
                return ""
            }
            XCTAssertNil(
                httpStatus, "the transport succeeded, so no status describes this", file: file,
                line: line)
            XCTAssertFalse(
                error.isRetryable, "re-requesting cannot repair a malformed body", file: file,
                line: line)
            XCTAssertNotNil(
                decodeFailure, "the structural marker separates this from any other .api",
                file: file, line: line)
            XCTAssertTrue(
                message.contains("GetPersonProgress returned a body that does not decode"), message,
                file: file, line: line)
            return message
        }
    }

    // MARK: - The marker: which statusless api_error is this?

    /// The two statusless `.api`s the SDK produces, told apart structurally
    /// (#750).
    ///
    /// This is one test and not two on purpose: the question is not "does a
    /// decode failure carry the marker" but "does the marker SEPARATE the two
    /// shapes", and the pagination same-origin refusal is the other one. Both
    /// carry a nil `httpStatus` and both are `.api`, so before the marker the
    /// only thing distinguishing them was a phrase inside the message — read
    /// back through a `@_spi(Conformance)` substring test whose contract was
    /// therefore the wording of a sentence.
    func testTheMarkerSeparatesADecodeFailureFromThePaginationRefusal() async throws {
        // Shape 1: a body the model refuses.
        let decodeAccount = makeTestAccountClient(transport: transport(body: "{}"))
        var decodeError: BasecampError?
        do {
            _ = try await decodeAccount.projects.get(projectId: 1)
            XCTFail("expected a malformed body to fail")
        } catch let error as BasecampError {
            decodeError = error
        }

        // Shape 2: a Link header pointing off-origin. A deliberate guard, not a
        // bad body — and `security.json` asserts on this refusal, so mislabelling
        // it as a fixture-repair job would lose the assertion.
        let page1 = Data("[]".utf8)
        let refusalTransport = MockTransport { request in
            (
                page1,
                makeHTTPResponse(
                    url: request.url!.absoluteString, statusCode: 200,
                    headers: [
                        "Content-Type": "application/json",
                        "Link": "<https://evil.example.com/steal?page=2>; rel=\"next\"",
                    ])
            )
        }
        let refusalAccount = makeTestAccountClient(transport: refusalTransport)
        var refusalError: BasecampError?
        do {
            _ = try await refusalAccount.projects.list()
            XCTFail("expected the off-origin Link header to be refused")
        } catch let error as BasecampError {
            refusalError = error
        }

        // Both are statusless `.api`, which is what makes the message the only
        // thing that used to separate them.
        XCTAssertNil(decodeError?.httpStatusCode)
        XCTAssertNil(refusalError?.httpStatusCode)

        // The marker separates them, and no substring is consulted to do it.
        XCTAssertTrue(
            decodeError?.decodeFailure is DecodingError,
            "the decoder's own refusal must be the marker's value, got: "
                + String(describing: decodeError?.decodeFailure))
        XCTAssertNil(
            refusalError?.decodeFailure,
            "a deliberate guard is not a malformed body")
    }

    /// A body that is not JSON at all is refused by `JSONSerialization` on the
    /// wrapped-list path, which reports a `CocoaError` rather than a
    /// `DecodingError`. The marker carries whichever one refused, so a caller
    /// telling those apart matches the concrete type instead of a message.
    func testTheMarkerCarriesACocoaErrorForABodyThatIsNotJSON() async throws {
        let account = makeTestAccountClient(transport: transport(body: "not json at all"))

        do {
            _ = try await account.reports.personProgress(personId: 7)
            XCTFail("expected a body that is not JSON to fail")
        } catch let error as BasecampError {
            XCTAssertNotNil(error.decodeFailure, "this is still a malformed body")
            XCTAssertFalse(
                error.decodeFailure is DecodingError,
                "JSONSerialization refused it, so the marker is not a DecodingError")
        }
    }

    /// `decodeFailure` is nil for every error shape that is not `.api`, the way
    /// `fieldErrors` is nil for everything that is not `.validation`. The
    /// property is what callers are steered to read, so its behaviour off the
    /// case it belongs to is part of the contract.
    func testTheMarkerIsNilForEveryOtherErrorShape() {
        XCTAssertNil(BasecampError.usage(message: "bad argument", hint: nil).decodeFailure)
        XCTAssertNil(
            BasecampError.network(message: "socket dropped", cause: nil).decodeFailure)
        XCTAssertNil(
            BasecampError.validation(
                message: "invalid", httpStatus: 422, hint: nil, requestId: nil,
                fieldErrors: ["title": ["is required"]]
            ).decodeFailure)
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

/// Records only what `testAMalformedWrapperIsReportedToHooksOnceAsAFailure`
/// reads: one entry per `onOperationEnd`, carrying its error or `nil`. Its own
/// spy rather than `HooksTests`', which is private to that file.
private final class EndSpy: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _ends: [(any Error)?] = []

    var ends: [(any Error)?] { lock.withLock { _ends } }

    func onOperationEnd(_ info: OperationInfo, result: OperationResult) {
        lock.withLock { _ends.append(result.error) }
    }
}
