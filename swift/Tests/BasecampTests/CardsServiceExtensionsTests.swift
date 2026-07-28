import XCTest
@testable import Basecamp

/// Swift mirror of `conformance/tests/cards_write.json`.
///
/// Swift is not part of `make conformance`, so these tests carry the same two
/// assertions the five executable runners make: the composite `update` does
/// GET-then-PUT and resends the fetched `due_on`, and `updateVerbatim` sends a
/// single PUT with `due_on` absent.
///
/// Without the second case, later generator drift could silently turn both
/// public methods into composite behaviour and nothing would notice.
private final class CardRequestLog: @unchecked Sendable {
    private let lock = NSLock()
    private var _methods: [String] = []
    private var _putBody: [String: Any]?

    var methods: [String] { lock.withLock { _methods } }
    var putBody: [String: Any]? { lock.withLock { _putBody } }

    func record(_ request: URLRequest) {
        lock.withLock {
            _methods.append(request.httpMethod ?? "?")
            if request.httpMethod == "PUT", let data = request.httpBody ?? request.cardBodyStreamData() {
                _putBody = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            }
        }
    }
}

extension URLRequest {
    fileprivate func cardBodyStreamData() -> Data? {
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

/// A card carrying a due date — the thing a sparse verbatim PUT would erase.
private func cardJSON(id: Int = 1069479350) -> [String: Any] {
    [
        "id": id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "Ship it",
        "inherits_status": true,
        "type": "Kanban::Card",
        "url": "https://3.basecampapi.com/999/buckets/1/card_tables/cards/\(id).json",
        "app_url": "https://3.basecamp.com/999/buckets/1/card_tables/cards/\(id)",
        "due_on": "2024-02-01",
        "comments_count": 0,
        "position": 1,
        "description_attachments": [] as [Any],
        "bucket": ["id": 1, "name": "The Leto Laptop", "type": "Project"],
        "parent": [
            "id": 2,
            "title": "In Progress",
            "type": "Kanban::Column",
            "url": "https://3.basecampapi.com/999/buckets/1/card_tables/columns/2.json",
            "app_url": "https://3.basecamp.com/999/buckets/1/card_tables/columns/2",
        ],
        "creator": [
            "id": 3,
            "name": "Victor Cooper",
            "email_address": "victor@honchodesign.com",
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
        ],
    ]
}

final class CardsServiceExtensionsTests: XCTestCase {
    private func makeCardsClient(log: CardRequestLog) throws -> AccountClient {
        let data = try JSONSerialization.data(withJSONObject: cardJSON())
        let transport = MockTransport { request in
            log.record(request)
            return (
                data,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        return makeTestAccountClient(transport: transport)
    }

    func testUpdatePreservesDueOnWhenUnaddressed() async throws {
        let log = CardRequestLog()
        let account = try makeCardsClient(log: log)

        _ = try await account.cards.update(cardId: 1069479350, title: "Renamed card")

        XCTAssertEqual(log.methods, ["GET", "PUT"], "the composite must read before writing")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["title"] as? String, "Renamed card")
        XCTAssertEqual(
            body["due_on"] as? String, "2024-02-01",
            "the fetched due date must be resent — BC3 clears an omitted due_on"
        )
        // Never echoed back: BC3 filters assignee ids through reachable_people.
        XCTAssertNil(body["assignee_ids"])
        XCTAssertNil(body["content"])
    }

    func testUpdateVerbatimSendsOnePutWithNoGet() async throws {
        let log = CardRequestLog()
        let account = try makeCardsClient(log: log)

        _ = try await account.cards.updateVerbatim(
            cardId: 1069479350,
            req: UpdateCardRequest(title: "Renamed card")
        )

        XCTAssertEqual(log.methods, ["PUT"], "verbatim must not read before writing")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["title"] as? String, "Renamed card")
        XCTAssertNil(body["due_on"], "an unset due_on must stay off the wire on the raw path")
    }

    func testUpdateClearSkipsTheFetchAndOmitsDueOn() async throws {
        let log = CardRequestLog()
        let account = try makeCardsClient(log: log)

        _ = try await account.cards.update(cardId: 1069479350, dueOn: .clear)

        XCTAssertEqual(log.methods, ["PUT"], "an explicit clear needs no read")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertNil(
            body["due_on"],
            "clearing is encoded by omitting due_on — never by sending null (SPEC section 18)"
        )
    }

    func testUpdateExplicitDateSkipsTheFetch() async throws {
        let log = CardRequestLog()
        let account = try makeCardsClient(log: log)

        _ = try await account.cards.update(cardId: 1069479350, dueOn: .on("2026-09-01"))

        XCTAssertEqual(log.methods, ["PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["due_on"] as? String, "2026-09-01")
    }
}
