import XCTest

@testable import Basecamp

/// The half of the recording-summary contract the shared fixture cannot reach.
///
/// `conformance/tests/recording_summary.json` pins the routing matrix, the
/// projection, and the happy and unhappy paths of Campfire discovery, and those
/// are not restated here. What is restated here is everything that needs a wire
/// shape the fixture has no way to script: a bucket the read disagrees with, a
/// bucket with more visible Campfires than one call may try, a listing past its
/// cap, and the dock cache actually being reused between two calls.
final class RecordingSummaryTests: XCTestCase {
    private let accountId = "999999999"

    // MARK: - Routing, before any request

    func testRefusesAnEventTypeThatNamesNoRecordingType() async throws {
        let server = RecordingServer()
        let account = makeTestAccountClient(transport: server.makeTransport())

        await assertSummarizeFails(
            account,
            RecordingRef(bucketId: 1, recordingId: 2, eventType: "boost.created")
        ) { error in
            guard case .noRecordingType(let ref) = error else {
                return XCTFail("expected noRecordingType, got \(error)")
            }
            XCTAssertEqual(ref.routingKey, "boost.created")
            XCTAssertTrue(error.message.contains("names no recording type"), error.message)
        }
        XCTAssertEqual(server.paths, [], "the refusal happens before any request")
    }

    func testRefusesATypeOutsideTheRoutingTable() async throws {
        let server = RecordingServer()
        let account = makeTestAccountClient(transport: server.makeTransport())

        for ref in [
            // Readable in this SDK, deliberately not routed.
            RecordingRef(bucketId: 1, recordingId: 2, recordingType: "Timesheet::Entry"),
            // Not a feed type at all: a subject with no action.
            RecordingRef(bucketId: 1, recordingId: 2, eventType: "comment"),
            // An action with no subject.
            RecordingRef(bucketId: 1, recordingId: 2, eventType: ".created"),
            // Neither field carries anything.
            RecordingRef(bucketId: 1, recordingId: 2),
        ] {
            await assertSummarizeFails(account, ref) { error in
                guard case .unknownRecordingType = error else {
                    return XCTFail("expected unknownRecordingType for \(ref), got \(error)")
                }
            }
        }
        XCTAssertEqual(server.paths, [])
    }

    func testTheRecordingTypeWinsOverTheEventType() async throws {
        let server = RecordingServer()
        let account = makeTestAccountClient(transport: server.makeTransport())

        // A pointer whose event type would route to a comment and whose
        // recording type says message: the more exact of the two wins.
        _ = try await account.recordings.summarize(
            RecordingRef(
                bucketId: 1, recordingId: 7, eventType: "comment.created",
                recordingType: "Message"))

        XCTAssertEqual(server.paths, ["/\(accountId)/messages/7"])
    }

    /// Go trims with `strings.TrimSpace`, which takes newlines too. A type read
    /// off a line-terminated feed field must still route, and a whitespace-only
    /// recording type must read as ABSENT so the event type is consulted —
    /// otherwise a pointer Go resolves becomes a refusal here.
    func testRoutingTrimsTheWhitespaceGoTrims() async throws {
        let server = RecordingServer()
        let account = makeTestAccountClient(transport: server.makeTransport())

        _ = try await account.recordings.summarize(
            RecordingRef(bucketId: 1, recordingId: 7, recordingType: "Comment\n"))
        _ = try await account.recordings.summarize(
            RecordingRef(bucketId: 1, recordingId: 7, eventType: "comment.created\n"))
        _ = try await account.recordings.summarize(
            RecordingRef(
                bucketId: 1, recordingId: 7, eventType: "comment.created",
                recordingType: "\n \t"))

        XCTAssertEqual(
            server.paths, Array(repeating: "/\(accountId)/comments/7", count: 3))
    }

    /// The projection is one shape across every SDK, so a nested identity the
    /// payload did not carry is ABSENT rather than an object of empty fields.
    func testAnAllZeroParentAndAnEmptyAssigneeListAreAbsent() async throws {
        let server = RecordingServer(emptyParent: true)
        let account = makeTestAccountClient(transport: server.makeTransport())

        let summary = try await account.recordings.summarize(
            RecordingRef(bucketId: 1, recordingId: 7, recordingType: "Comment"))

        XCTAssertNil(summary.parent, "an id-less, title-less parent is not a parent")
        XCTAssertNotNil(summary.bucket, "the bucket this one does carry stays")
        XCTAssertNil(summary.assignees, "a comment has none, and none is absent, not []")

        let encoded = try XCTUnwrap(
            String(data: try BaseService.encoder.encode(summary), encoding: .utf8))
        XCTAssertFalse(encoded.contains("\"parent\""), encoded)
        XCTAssertFalse(encoded.contains("\"assignees\""), encoded)
    }

    func testRequiresBothIds() async {
        let server = RecordingServer()
        let account = makeTestAccountClient(transport: server.makeTransport())

        for ref in [
            RecordingRef(bucketId: 0, recordingId: 2, recordingType: "Comment"),
            RecordingRef(bucketId: 1, recordingId: 0, recordingType: "Comment"),
        ] {
            do {
                _ = try await account.recordings.summarize(ref)
                XCTFail("expected a usage error for \(ref)")
            } catch let error as BasecampError {
                guard case .usage = error else { return XCTFail("expected usage, got \(error)") }
            } catch {
                XCTFail("expected a BasecampError, got \(error)")
            }
        }
        XCTAssertEqual(server.paths, [])
    }

    func testTheDocumentedTypeListsMatchTheRoutingTable() {
        let types = RecordingsService.summarizableRecordingTypes
        XCTAssertEqual(types, types.sorted())
        XCTAssertTrue(types.contains("Chat::Lines::*"), "the subtypes share one prefix")
        XCTAssertTrue(types.contains("Kanban::Card"))
        XCTAssertFalse(types.contains("Timesheet::Entry"), "the set is deliberate, not exhaustive")
        XCTAssertEqual(
            types.count, RecordingsService.summarizableTypes.count + 1,
            "every routed type is listed, plus the chat-line prefix")

        XCTAssertEqual(
            RecordingsService.summarizableEventTypes,
            ["card.*", "chat.line.*", "comment.*", "message.*", "todo.*"],
            "boost is absent on purpose — it names no recording type")
    }

    // MARK: - Bucket check

    func testRefusesARecordingFromAnotherBucket() async throws {
        let server = RecordingServer(commentBucketId: 4242)
        let account = makeTestAccountClient(transport: server.makeTransport())

        await assertSummarizeFails(
            account, RecordingRef(bucketId: 1, recordingId: 7, recordingType: "Comment")
        ) { error in
            guard case .bucketMismatch(let ref, let actual) = error else {
                return XCTFail("expected bucketMismatch, got \(error)")
            }
            XCTAssertEqual(ref.bucketId, 1)
            XCTAssertEqual(actual, 4242)
        }
    }

    // MARK: - Chat-line content

    func testOnlyARichTextChatLineCanCarryAMention() async throws {
        // The same content under two subtypes. A Text line's content was
        // HTML-escaped on the way out, so a literal tag in it is text.
        let sgid = "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19"
        let markup = "<bc-attachment sgid=\"\(sgid)\"></bc-attachment>"

        for (type, expected) in [("Chat::Lines::Text", [Int]()), ("Chat::Lines::RichText", [42])] {
            let server = RecordingServer(
                dockCampfireIds: [10], lineType: type, lineContent: markup)
            let account = makeTestAccountClient(transport: server.makeTransport())

            let summary = try await account.recordings.summarize(
                RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created"))

            XCTAssertEqual(summary.mentionedPersonIds, expected, "for \(type)")
            XCTAssertEqual(summary.campfireId, 10, "the reply destination is reported")
        }
    }

    // MARK: - Campfire discovery

    func testReadsTheDockOnceForTwoLinesInTheSameBucket() async throws {
        let server = RecordingServer(dockCampfireIds: [10])
        let account = makeTestAccountClient(transport: server.makeTransport())

        for recordingId in [7, 8] {
            _ = try await account.recordings.summarize(
                RecordingRef(
                    bucketId: 1, recordingId: recordingId, eventType: "chat.line.created"))
        }

        XCTAssertEqual(
            server.paths.filter { $0.hasSuffix("/projects/1") }.count, 1,
            "the second line is answered from the cached dock")
        XCTAssertEqual(
            server.paths.filter { $0.hasSuffix("/chats.json") }.count, 0,
            "the listing is never fetched once the dock has answered")
    }

    func testTheListingIsNotFetchedUntilTheDockHasHadItsSay() async throws {
        let server = RecordingServer(dockCampfireIds: [10])
        let account = makeTestAccountClient(transport: server.makeTransport())

        _ = try await account.recordings.summarize(
            RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created"))

        XCTAssertEqual(
            server.paths,
            ["/\(accountId)/projects/1", "/\(accountId)/chats/10/lines/7"],
            "a project's line costs one project read and one line read")
    }

    func testACandidatesNon404IsReturnedAsThatReadsError() async throws {
        let server = RecordingServer(dockCampfireIds: [10, 11], lineStatusByCampfire: [10: 403])
        let account = makeTestAccountClient(transport: server.makeTransport())

        do {
            _ = try await account.recordings.summarize(
                RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created"))
            XCTFail("expected the candidate's 403")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("expected forbidden, got \(error)") }
        }

        XCTAssertFalse(
            server.paths.contains("/\(accountId)/chats/11/lines/7"),
            "the loop stops at the first non-404 rather than hiding it behind the next candidate")
    }

    func testEveryCandidateSaying404IsUnresolvedRatherThanAFailedRead() async throws {
        let server = RecordingServer(dockCampfireIds: [10, 11], lineStatusByCampfire: [:])
        server.lineFoundUnder = nil
        let account = makeTestAccountClient(transport: server.makeTransport())

        await assertSummarizeFails(
            account, RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created")
        ) { error in
            guard case .recordingUnresolved(let unresolved) = error else {
                return XCTFail("expected recordingUnresolved, got \(error)")
            }
            XCTAssertEqual(unresolved.campfireIds, [10, 11], "the candidates tried, in order")
            XCTAssertEqual(unresolved.bucketId, 1)
            XCTAssertEqual(unresolved.recordingId, 7)
            XCTAssertFalse(
                unresolved.refreshed,
                "both sources were loaded during this very call, so there was nothing to re-read")
            XCTAssertEqual(unresolved.staleCampfireIds, [], "stale is only meaningful after a refresh")
        }
    }

    /// Nothing left unsearched is ever reported absent.
    func testABucketWithMoreCandidatesThanTheBudgetIsIncompleteNotUnresolved() async throws {
        let campfireIds = Array(100..<(100 + RecordingsService.maxCampfireCandidates + 1))
        let server = RecordingServer(dockCampfireIds: campfireIds)
        server.lineFoundUnder = nil
        let account = makeTestAccountClient(transport: server.makeTransport())

        await assertSummarizeFails(
            account, RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created")
        ) { error in
            guard case .campfireDiscoveryIncomplete(let incomplete) = error else {
                return XCTFail("expected campfireDiscoveryIncomplete, got \(error)")
            }
            XCTAssertTrue(
                incomplete.reason.contains("\(RecordingsService.maxCampfireCandidates)"),
                incomplete.reason)
            XCTAssertTrue(
                incomplete.reason.contains("before the account listing was consulted"),
                "the budget ran out before a source that was never consulted: \(incomplete.reason)")
        }

        XCTAssertEqual(
            server.paths.filter { $0.contains("/lines/") }.count,
            RecordingsService.maxCampfireCandidates,
            "the budget bounds the reads one call makes")
    }

    /// The boundary the budget guard exists for, and the one a `skipped`-keyed
    /// guard misses: a dock holding EXACTLY the budget, every entry answering
    /// 404. Every candidate was observed and tried, so nothing was skipped and
    /// the budget is spent — and the account listing has not been consulted, so
    /// candidates may exist there unsearched.
    ///
    /// Reading `skipped` here would fall through and fetch the listing: a
    /// request that cannot help, whose failure would replace a settled
    /// "incomplete" with a transient error a consumer retries forever.
    func testASpentBudgetBeforeTheListingIsIncompleteRatherThanAFetchThatCannotHelp() async throws {
        let campfireIds = Array(100..<(100 + RecordingsService.maxCampfireCandidates))
        let server = RecordingServer(dockCampfireIds: campfireIds)
        server.lineFoundUnder = nil
        let account = makeTestAccountClient(transport: server.makeTransport())

        await assertSummarizeFails(
            account, RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created")
        ) { error in
            guard case .campfireDiscoveryIncomplete(let incomplete) = error else {
                return XCTFail("expected campfireDiscoveryIncomplete, got \(error)")
            }
            XCTAssertTrue(
                incomplete.reason.contains("before the account listing was consulted"),
                incomplete.reason)
        }

        XCTAssertFalse(
            server.paths.contains { $0.hasSuffix("/chats.json") },
            "the listing is never fetched: no candidate it returned could be tried")
        XCTAssertEqual(
            server.paths.filter { $0.contains("/lines/") }.count,
            RecordingsService.maxCampfireCandidates,
            "every candidate the budget allows was tried first")
    }

    /// The third property of the budget rule, and the one that produces a WRONG
    /// VERDICT rather than a wasted request: a spent budget where BOTH sources
    /// were consulted is `unresolved`, not `incomplete`. Everything was
    /// searched, so nothing is unsearched — and `unresolved` is settled while
    /// `incomplete` tells a consumer to look again for a recording that was
    /// thoroughly searched for and is not there.
    func testASpentBudgetWithBothSourcesConsultedIsUnresolvedNotIncomplete() async throws {
        // Exactly the budget, in the listing rather than the dock, so the second
        // call finds the listing already cached.
        let campfireIds = Array(100..<(100 + RecordingsService.maxCampfireCandidates))
        let server = RecordingServer(listedCampfireIds: campfireIds)
        server.lineFoundUnder = nil
        let account = makeTestAccountClient(transport: server.makeTransport())

        // First call populates both caches: an empty dock and the listing. It is
        // itself the case a budget check placed AFTER the listing fetch would
        // get wrong — the budget is spent by the time the listing's candidates
        // have all been tried, but both sources were consulted, so the verdict
        // is unresolved.
        await assertSummarizeFails(
            account, RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created")
        ) { error in
            guard case .recordingUnresolved(let unresolved) = error else {
                return XCTFail(
                    "a freshly fetched listing whose candidates exhaust the budget is still fully consulted, got \(error)"
                )
            }
            XCTAssertEqual(
                unresolved.campfireIds.count, RecordingsService.maxCampfireCandidates)
        }
        let afterFirst = server.paths.count

        // Second call: the dock is cached and empty, the listing is cached and
        // holds exactly the budget, every candidate answers 404.
        await assertSummarizeFails(
            account, RecordingRef(bucketId: 1, recordingId: 8, eventType: "chat.line.created")
        ) { error in
            guard case .recordingUnresolved(let unresolved) = error else {
                return XCTFail("expected recordingUnresolved, got \(error)")
            }
            XCTAssertEqual(
                unresolved.campfireIds.count, RecordingsService.maxCampfireCandidates,
                "every candidate was tried, so nothing was left unsearched")
        }

        let secondCall = Array(server.paths.dropFirst(afterFirst))
        XCTAssertFalse(
            secondCall.contains { $0.hasSuffix("/chats.json") },
            "the listing was already consulted, so a spent budget does not re-read it")
        XCTAssertFalse(
            secondCall.contains { $0.hasSuffix("/projects/1") },
            "nor the dock")
    }

    func testAListingPastItsCapIsIncompleteNotUnresolved() async throws {
        // The dock answers with nothing, so discovery falls through to the
        // listing — which advertises a further page the page cap will not let it
        // walk, which is how an over-cap listing presents.
        let server = RecordingServer(dockCampfireIds: [], listingAdvertisesMore: true)
        let client = BasecampClient(
            tokenProvider: StaticTokenProvider("test-token"),
            userAgent: "test-suite",
            config: BasecampConfig(
                baseURL: "https://3.basecampapi.com", enableRetry: false, maxPages: 1),
            transport: server.makeTransport())
        let account = client.forAccount(accountId)

        await assertSummarizeFails(
            account, RecordingRef(bucketId: 1, recordingId: 7, eventType: "chat.line.created")
        ) { error in
            guard case .campfireDiscoveryIncomplete(let incomplete) = error else {
                return XCTFail("expected campfireDiscoveryIncomplete, got \(error)")
            }
            XCTAssertTrue(incomplete.reason.contains("listing"), incomplete.reason)
        }
    }

    // MARK: - Helpers

    private func assertSummarizeFails(
        _ account: AccountClient,
        _ ref: RecordingRef,
        file: StaticString = #filePath,
        line: UInt = #line,
        _ check: (RecordingSummaryError) -> Void
    ) async {
        do {
            _ = try await account.recordings.summarize(ref)
            XCTFail("expected summarize to fail", file: file, line: line)
        } catch let error as RecordingSummaryError {
            check(error)
        } catch {
            XCTFail("expected a RecordingSummaryError, got \(error)", file: file, line: line)
        }
    }
}

// MARK: - A scriptable BC3

/// Answers the reads `summarize` makes, by path, so a test can shape the wire
/// rather than the SDK's internals.
private final class RecordingServer: @unchecked Sendable {
    private let lock = NSLock()
    private var _paths: [String] = []

    private let commentBucketId: Int
    private let emptyParent: Bool
    private let dockCampfireIds: [Int]
    private let listedCampfireIds: [Int]
    private let listingAdvertisesMore: Bool
    private let lineType: String
    private let lineContent: String
    private let lineStatusByCampfire: [Int: Int]
    /// The Campfire the line is under, or nil when it is under none of them.
    var lineFoundUnder: Int?

    var paths: [String] { lock.withLock { _paths } }

    init(
        commentBucketId: Int = 1,
        emptyParent: Bool = false,
        dockCampfireIds: [Int] = [],
        listedCampfireIds: [Int] = [],
        listingAdvertisesMore: Bool = false,
        lineType: String = "Chat::Lines::Text",
        lineContent: String = "Hello everyone!",
        lineStatusByCampfire: [Int: Int] = [:]
    ) {
        self.commentBucketId = commentBucketId
        self.emptyParent = emptyParent
        self.dockCampfireIds = dockCampfireIds
        self.listedCampfireIds = listedCampfireIds
        self.listingAdvertisesMore = listingAdvertisesMore
        self.lineType = lineType
        self.lineContent = lineContent
        self.lineStatusByCampfire = lineStatusByCampfire
        self.lineFoundUnder = dockCampfireIds.first ?? listedCampfireIds.first
    }

    func makeTransport() -> MockTransport {
        MockTransport { [self] request in
            let path = request.url?.path ?? ""
            lock.withLock { _paths.append(path) }
            let segments = path.split(separator: "/").map(String.init)

            func reply(_ body: Any, status: Int = 200, headers: [String: String] = [:]) throws -> (
                Data, URLResponse
            ) {
                var allHeaders = headers
                allHeaders["Content-Type"] = "application/json"
                return (
                    try JSONSerialization.data(withJSONObject: body),
                    makeHTTPResponse(
                        url: request.url!.absoluteString, statusCode: status, headers: allHeaders)
                )
            }
            func notFound() throws -> (Data, URLResponse) {
                try reply(["error": "Record not found"], status: 404)
            }

            // /{account}/projects/{id}
            if segments.count == 3, segments[1] == "projects" {
                guard !dockCampfireIds.isEmpty else { return try notFound() }
                return try reply(projectJSON(id: Int(segments[2]) ?? 0))
            }
            // /{account}/chats.json
            if segments.count == 2, segments[1] == "chats.json" {
                var headers: [String: String] = [:]
                if listingAdvertisesMore {
                    headers["Link"] =
                        "<\(request.url!.absoluteString)?page=2>; rel=\"next\""
                }
                return try reply(listedCampfireIds.map { campfireJSON(id: $0) }, headers: headers)
            }
            // /{account}/chats/{campfireId}/lines/{lineId}
            if segments.count == 5, segments[1] == "chats", segments[3] == "lines" {
                let campfireId = Int(segments[2]) ?? 0
                if let status = lineStatusByCampfire[campfireId] {
                    return try reply(["error": "no"], status: status)
                }
                guard campfireId == lineFoundUnder else { return try notFound() }
                return try reply(
                    lineJSON(id: Int(segments[4]) ?? 0, campfireId: campfireId))
            }
            // /{account}/comments/{id} and /{account}/messages/{id}
            if segments.count == 3, segments[1] == "comments" || segments[1] == "messages" {
                return try reply(
                    recordingJSON(
                        id: Int(segments[2]) ?? 0,
                        type: segments[1] == "comments" ? "Comment" : "Message",
                        bucketId: commentBucketId))
            }
            return try notFound()
        }
    }

    private var personJSON: [String: Any] {
        ["id": 1_049_715_914, "name": "Victor Cooper"]
    }

    private var bucketJSON: [String: Any] {
        ["id": 1, "name": "The Leto Laptop", "type": "Project"]
    }

    private func parentJSON(id: Int, type: String) -> [String: Any] {
        [
            "id": id, "title": "Parent", "type": type,
            "url": "https://3.basecampapi.com/999999999/buckets/1/x/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/x/\(id)",
        ]
    }

    private func projectJSON(id: Int) -> [String: Any] {
        [
            "id": id, "status": "active", "name": "The Leto Laptop",
            "created_at": "2026-01-01T00:00:00.000Z", "updated_at": "2026-01-01T00:00:00.000Z",
            "url": "https://3.basecampapi.com/999999999/projects/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/projects/\(id)",
            "dock": dockCampfireIds.map { campfireId in
                [
                    "id": campfireId, "title": "Campfire", "name": "chat", "enabled": true,
                    "position": 1,
                    "url": "https://3.basecampapi.com/999999999/buckets/\(id)/chats/\(campfireId).json",
                    "app_url": "https://3.basecamp.com/999999999/buckets/\(id)/chats/\(campfireId)",
                ] as [String: Any]
            } + [
                // A non-chat tool, so the filter is doing something.
                [
                    "id": 999, "title": "Message Board", "name": "message_board", "enabled": true,
                    "position": 2,
                    "url": "https://3.basecampapi.com/999999999/buckets/\(id)/message_boards/999.json",
                    "app_url": "https://3.basecamp.com/999999999/buckets/\(id)/message_boards/999",
                ] as [String: Any]
            ],
        ]
    }

    private func campfireJSON(id: Int) -> [String: Any] {
        [
            "id": id, "status": "active", "visible_to_clients": false, "inherits_status": true,
            "title": "Campfire", "type": "Chat::Transcript",
            "created_at": "2026-01-01T00:00:00.000Z", "updated_at": "2026-01-01T00:00:00.000Z",
            "url": "https://3.basecampapi.com/999999999/buckets/1/chats/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/chats/\(id)",
            "bucket": bucketJSON, "creator": personJSON,
        ]
    }

    private func lineJSON(id: Int, campfireId: Int) -> [String: Any] {
        [
            "id": id, "status": "active", "visible_to_clients": false, "inherits_status": true,
            "title": "Hello everyone!", "type": lineType, "content": lineContent,
            "created_at": "2026-01-01T00:00:00.000Z", "updated_at": "2026-01-01T00:00:00.000Z",
            "url": "https://3.basecampapi.com/999999999/buckets/1/chats/\(campfireId)/lines/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/chats/\(campfireId)#\(id)",
            "bucket": bucketJSON, "creator": personJSON,
            "parent": parentJSON(id: campfireId, type: "Chat::Transcript"),
        ]
    }

    private func recordingJSON(id: Int, type: String, bucketId: Int) -> [String: Any] {
        var json: [String: Any] = [
            "id": id, "status": "active", "visible_to_clients": false, "inherits_status": true,
            "title": "Re: We won Leto!", "type": type, "content": "<div>On it.</div>",
            "content_attachments": [] as [Any],
            "created_at": "2026-01-01T00:00:00.000Z", "updated_at": "2026-01-01T00:00:00.000Z",
            "url": "https://3.basecampapi.com/999999999/buckets/\(bucketId)/x/\(id).json",
            "app_url": "https://3.basecamp.com/999999999/buckets/\(bucketId)/x/\(id)",
            "bucket": ["id": bucketId, "name": "The Leto Laptop", "type": "Project"],
            "creator": personJSON,
            "parent": emptyParent
                ? ["id": 0, "title": "", "type": "", "url": "", "app_url": ""] as [String: Any]
                : parentJSON(id: 1, type: "Message::Board"),
        ]
        if type == "Message" { json["subject"] = "We won Leto!" }
        return json
    }
}
