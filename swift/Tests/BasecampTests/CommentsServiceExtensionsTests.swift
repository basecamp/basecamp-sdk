import XCTest

@testable import Basecamp

/// The mention-expanding write surface, where the shared fixture stops.
///
/// `conformance/tests/recording_summary.json` pins the happy path — one person
/// read, then the POST, with the tag inside the content's first block. What it
/// cannot script is a second wire shape: a person read that fails, a repeated
/// id, or content that already carries a tag for the same person under a
/// different sgid. Those decide whether a half-resolved mention list can reach
/// the wire, and they live here.
final class CommentsServiceExtensionsTests: XCTestCase {
    private let accountId = "999999999"
    /// `{"_rails": {"data": "gid://bc3/Person/42", "pur": "attachable"}}`.
    private let sgid42 =
        "eyJfcmFpbHMiOnsiZGF0YSI6ImdpZDovL2JjMy9QZXJzb24vNDIiLCJwdXIiOiJhdHRhY2hhYmxlIn19"
    /// `{"gid": "gid://bc3/Person/43", "purpose": "attachable", "expires_at": null}`.
    private let sgid43 =
        "eyJnaWQiOiJnaWQ6Ly9iYzMvUGVyc29uLzQzIiwicHVycG9zZSI6ImF0dGFjaGFibGUiLCJleHBpcmVzX2F0IjpudWxsfQ"

    func testEachDistinctIdIsReadOnceAndTheMentionsAreWrittenFromTheSgids() async throws {
        let server = CommentServer(sgids: [42: sgid42, 43: sgid43])
        let account = makeTestAccountClient(transport: server.makeTransport())

        _ = try await account.comments.createWithMentions(
            recordingId: 7, content: "<div>On it.</div>", mentions: [42, 43, 42])

        XCTAssertEqual(
            server.paths,
            [
                "/\(accountId)/people/42", "/\(accountId)/people/43",
                "/\(accountId)/recordings/7/comments.json",
            ],
            "one read per DISTINCT id, in the order given, and the write last")

        let posted = try XCTUnwrap(server.postedContent)
        XCTAssertEqual(
            posted,
            "<div><bc-attachment sgid=\"\(sgid42)\"></bc-attachment> "
                + "<bc-attachment sgid=\"\(sgid43)\"></bc-attachment> On it.</div>")
        XCTAssertEqual(
            Mentions.personIds(in: posted), [42, 43], "the rendered mentions round-trip")
    }

    /// Nothing is posted on a partial mention list.
    func testAFailedPersonReadPostsNothing() async throws {
        let server = CommentServer(sgids: [42: sgid42])  // 43 is not a person here
        let account = makeTestAccountClient(transport: server.makeTransport())

        do {
            _ = try await account.comments.createWithMentions(
                recordingId: 7, content: "<div>On it.</div>", mentions: [42, 43])
            XCTFail("expected the failed person read to fail the expansion")
        } catch let error as BasecampError {
            guard case .notFound = error else {
                return XCTFail("the read's own error reaches the caller, got \(error)")
            }
        }

        XCTAssertFalse(
            server.paths.contains { $0.hasSuffix("/comments.json") },
            "the reads happen before the write, so a failed lookup posts nothing")
    }

    func testRefusesANonPositivePersonIdBeforeAnyRequest() async throws {
        let server = CommentServer(sgids: [:])
        let account = makeTestAccountClient(transport: server.makeTransport())

        do {
            _ = try await account.comments.expandMentions("<p>hi</p>", mentioning: [0])
            XCTFail("expected a usage error")
        } catch let error as BasecampError {
            guard case .usage(let message, _) = error else {
                return XCTFail("expected usage, got \(error)")
            }
            XCTAssertTrue(message.contains("invalid mention person id"), message)
        }
        XCTAssertEqual(server.paths, [])
    }

    func testMentioningNobodyMakesNoPersonRead() async throws {
        let server = CommentServer(sgids: [:])
        let account = makeTestAccountClient(transport: server.makeTransport())

        _ = try await account.comments.createWithMentions(
            recordingId: 7, content: "<div>On it.</div>", mentions: [])

        XCTAssertEqual(server.paths, ["/\(accountId)/recordings/7/comments.json"])
        XCTAssertEqual(server.postedContent, "<div>On it.</div>", "the content is left alone")
    }

    func testEmptyContentIsRefusedBeforeAnyRequest() async throws {
        let server = CommentServer(sgids: [42: sgid42])
        let account = makeTestAccountClient(transport: server.makeTransport())

        do {
            _ = try await account.comments.createWithMentions(
                recordingId: 7, content: "", mentions: [42])
            XCTFail("expected a usage error")
        } catch let error as BasecampError {
            guard case .usage = error else { return XCTFail("expected usage, got \(error)") }
        }
        XCTAssertEqual(server.paths, [], "not even the person read is worth making")
    }

    /// The trust boundary. An sgid already in the content is unsigned: it cannot
    /// prove the person is mentioned, so it never stands in for the read, and
    /// only an EXACT match suppresses the tag.
    func testAnUnsignedTagNamingThePersonNeitherSkipsTheReadNorSuppressesTheMention() async throws {
        let server = CommentServer(sgids: [42: sgid42])
        let account = makeTestAccountClient(transport: server.makeTransport())
        let staleTag = "<bc-attachment sgid=\"\(sgid42)--forged\"></bc-attachment>"

        _ = try await account.comments.createWithMentions(
            recordingId: 7, content: "<div>\(staleTag) again</div>", mentions: [42])

        XCTAssertTrue(
            server.paths.contains("/\(accountId)/people/42"),
            "the authoritative people read happens even though the content names the person")
        let posted = try XCTUnwrap(server.postedContent)
        XCTAssertEqual(
            Mentions.attachmentSgids(in: posted), [sgid42, "\(sgid42)--forged"],
            "the real mention is added; the forged one is not a duplicate of it")
    }

    func testAnExactlyMatchingTagIsNotDuplicated() async throws {
        let server = CommentServer(sgids: [42: sgid42])
        let account = makeTestAccountClient(transport: server.makeTransport())
        let existing = "<div><bc-attachment sgid=\"\(sgid42)\"></bc-attachment> again</div>"

        _ = try await account.comments.createWithMentions(
            recordingId: 7, content: existing, mentions: [42])

        XCTAssertEqual(server.postedContent, existing)
        XCTAssertTrue(
            server.paths.contains("/\(accountId)/people/42"),
            "the read still happens — it is what proves the sgid in the content is the real one")
    }
}

extension URLRequest {
    /// URLSession may hand a body over as a stream rather than as `httpBody`;
    /// the assertions below are about what was posted, so read either.
    fileprivate func commentBodyStreamData() -> Data? {
        guard let stream = httpBodyStream else { return nil }
        stream.open()
        defer { stream.close() }
        var data = Data()
        let bufferSize = 4096
        let buffer = UnsafeMutablePointer<UInt8>.allocate(capacity: bufferSize)
        defer { buffer.deallocate() }
        while stream.hasBytesAvailable {
            let read = stream.read(buffer, maxLength: bufferSize)
            if read <= 0 { break }
            data.append(buffer, count: read)
        }
        return data
    }
}

/// Answers the people read and the comment write, recording both.
private final class CommentServer: @unchecked Sendable {
    private let lock = NSLock()
    private var _paths: [String] = []
    private var _postedContent: String?
    private let sgids: [Int: String]

    var paths: [String] { lock.withLock { _paths } }
    var postedContent: String? { lock.withLock { _postedContent } }

    init(sgids: [Int: String]) { self.sgids = sgids }

    func makeTransport() -> MockTransport {
        MockTransport { [self] request in
            let path = request.url?.path ?? ""
            lock.withLock { _paths.append(path) }
            let segments = path.split(separator: "/").map(String.init)

            func reply(_ body: Any, status: Int = 200) throws -> (Data, URLResponse) {
                (
                    try JSONSerialization.data(withJSONObject: body),
                    makeHTTPResponse(
                        url: request.url!.absoluteString, statusCode: status,
                        headers: ["Content-Type": "application/json"])
                )
            }

            if segments.count == 3, segments[1] == "people" {
                guard let id = Int(segments[2]), let sgid = sgids[id] else {
                    return try reply(["error": "Record not found"], status: 404)
                }
                return try reply(
                    ["id": id, "name": "Person \(id)", "attachable_sgid": sgid] as [String: Any])
            }

            if request.httpMethod == "POST", path.hasSuffix("/comments.json") {
                let raw = request.httpBody ?? request.commentBodyStreamData()
                let body = raw.flatMap { try? JSONSerialization.jsonObject(with: $0) }
                    as? [String: Any]
                lock.withLock { _postedContent = body?["content"] as? String }
                return try reply(commentJSON(content: body?["content"] as? String ?? ""), status: 201)
            }

            return try reply(["error": "unexpected \(request.httpMethod ?? "?") \(path)"], status: 500)
        }
    }

    private func commentJSON(content: String) -> [String: Any] {
        [
            "id": 1_069_479_370, "status": "active", "visible_to_clients": false,
            "inherits_status": true, "title": "Re: We won Leto!", "type": "Comment",
            "content": content, "content_attachments": [] as [Any],
            "created_at": "2026-01-01T00:00:00.000Z", "updated_at": "2026-01-01T00:00:00.000Z",
            "url": "https://3.basecampapi.com/999999999/buckets/1/comments/1069479370.json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/comments/1069479370",
            "bucket": ["id": 1, "name": "The Leto Laptop", "type": "Project"],
            "creator": ["id": 3, "name": "Victor Cooper"],
            "parent": [
                "id": 7, "title": "We won Leto!", "type": "Message",
                "url": "https://3.basecampapi.com/999999999/buckets/1/messages/7.json",
                "app_url": "https://3.basecamp.com/999999999/buckets/1/messages/7",
            ],
        ]
    }
}
