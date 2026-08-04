import XCTest
@testable import Basecamp

/// Swift mirror of `conformance/tests/cards_write.json`.
///
/// Swift is not part of `make conformance`, so these tests carry the same
/// assertions the five executable runners make. BC3's JSON card update is
/// presence-aware (basecamp/bc3#12521): an omitted `due_on` is left unchanged,
/// an explicit `""` or null clears it. So the contract under test is a
/// presence contract — `.preserve` keeps the key off the wire, `.clear` puts
/// `"due_on": ""` on it, `.on` puts the date on it — and every case is a
/// single PUT with no preceding read.
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

/// A card carrying a due date.
private func cardJSON(id: Int = 1069479350, dueOn: String? = "2024-02-01") -> [String: Any] {
    var json: [String: Any] = [
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
    if let dueOn {
        json["due_on"] = dueOn
    }
    return json
}

/// A transport that models BC3's presence-aware JSON card update
/// (basecamp/bc3#12521): `card_update_params` is plain `card_params`, so an
/// OMITTED `due_on` leaves the stored value UNCHANGED, while an explicit `""`
/// (Rails blank-casts it to nil on the date attribute) or an explicit null
/// clears it.
///
/// This is the behavioural half of the contract. Asserting only on the encoded
/// request body pins the bytes but not their effect; driving them through a
/// server that actually applies BC3's presence rule is what proves an explicit
/// clear still clears and an unaddressed update still leaves the date alone.
private final class PresenceAwareCardServer: @unchecked Sendable {
    private let lock = NSLock()
    private var _storedDueOn: String?
    private var _methods: [String] = []

    /// The due date the modelled server currently holds. `nil` means cleared.
    var storedDueOn: String? { lock.withLock { _storedDueOn } }
    /// Every HTTP method the server was asked for, in order.
    var methods: [String] { lock.withLock { _methods } }

    init(storedDueOn: String? = "2024-02-01") {
        self._storedDueOn = storedDueOn
    }

    func makeTransport() -> MockTransport {
        MockTransport { [self] request in
            let body = request.httpBody ?? request.cardBodyStreamData()

            lock.withLock {
                _methods.append(request.httpMethod ?? "?")

                if request.httpMethod == "PUT" {
                    let parsed = body.flatMap { try? JSONSerialization.jsonObject(with: $0) }
                        as? [String: Any]
                    // Presence, not truthiness: only a due_on the caller
                    // actually sent can change the stored value.
                    if let parsed, parsed.keys.contains("due_on") {
                        let value = parsed["due_on"]
                        if value is NSNull {
                            _storedDueOn = nil
                        } else if let string = value as? String {
                            _storedDueOn = string.isEmpty ? nil : string
                        }
                    }
                }
            }

            let payload = try JSONSerialization.data(
                withJSONObject: cardJSON(dueOn: self.storedDueOn)
            )
            return (
                payload,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
    }
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

    func testUpdateOmitsDueOnWhenUnaddressed() async throws {
        let log = CardRequestLog()
        let account = try makeCardsClient(log: log)

        _ = try await account.cards.update(cardId: 1069479350, title: "Renamed card")

        XCTAssertEqual(
            log.methods, ["PUT"],
            "an omitted due_on already means unchanged, so there is nothing to read first"
        )
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["title"] as? String, "Renamed card")
        XCTAssertNil(
            body["due_on"],
            "an unaddressed due date must stay off the wire — presence is what changes it"
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
        XCTAssertNil(
            body["due_on"],
            "a due_on the caller never set must stay off the wire on the raw path"
        )
    }

    func testUpdateVerbatimSendsAnExplicitEmptyDueOn() async throws {
        let log = CardRequestLog()
        let account = try makeCardsClient(log: log)

        _ = try await account.cards.updateVerbatim(
            cardId: 1069479350,
            req: UpdateCardRequest(dueOn: "")
        )

        let body = try XCTUnwrap(log.putBody)
        XCTAssertTrue(
            body.keys.contains("due_on"),
            "encodeIfPresent drops a nil but keeps an empty string — \"\" must survive to the wire"
        )
        XCTAssertEqual(body["due_on"] as? String, "")
    }

    func testUpdateClearSendsAnEmptyDueOn() async throws {
        let log = CardRequestLog()
        let account = try makeCardsClient(log: log)

        _ = try await account.cards.update(cardId: 1069479350, dueOn: .clear)

        XCTAssertEqual(log.methods, ["PUT"], "an explicit clear is one request")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertTrue(
            body.keys.contains("due_on"),
            "BC3 leaves an OMITTED due_on unchanged, so a clear must send the key"
        )
        XCTAssertEqual(
            body["due_on"] as? String, "",
            #"clearing is encoded as "due_on": "" — never by omission, and never as null (SPEC section 18)"#
        )
    }

    /// The behavioural proof, not the byte-level one: run an explicit clear
    /// against a server that applies BC3's presence rule and check the date is
    /// actually gone afterwards. Encoding the clear by omission satisfies a
    /// wire-shape assertion written against the old contract but silently
    /// no-ops here.
    func testExplicitClearActuallyClearsAgainstPresenceAwareBC3() async throws {
        let server = PresenceAwareCardServer(storedDueOn: "2024-02-01")
        let account = makeTestAccountClient(transport: server.makeTransport())

        let card = try await account.cards.update(cardId: 1069479350, dueOn: .clear)

        XCTAssertNil(
            server.storedDueOn,
            "the explicit clear must actually clear the stored due date, not no-op"
        )
        XCTAssertNil(card.dueOn, "the card the server echoes back must have no due date")
        XCTAssertEqual(server.methods, ["PUT"])
    }

    /// The same server, from the other side: an update that does not address
    /// the due date must leave it intact and must not send the key at all.
    /// This is what makes the test above a real discriminator rather than a
    /// server that clears on every PUT.
    func testUnaddressedUpdateLeavesTheDueDateAloneOnPresenceAwareBC3() async throws {
        let server = PresenceAwareCardServer(storedDueOn: "2024-02-01")
        let account = makeTestAccountClient(transport: server.makeTransport())

        let card = try await account.cards.update(cardId: 1069479350, title: "Renamed card")

        XCTAssertEqual(
            server.storedDueOn, "2024-02-01",
            "omitting due_on must leave the stored date untouched"
        )
        XCTAssertEqual(card.dueOn, "2024-02-01")
        XCTAssertEqual(
            server.methods, ["PUT"],
            "no preservation read is needed once the server treats absence as unchanged"
        )
    }

    /// Setting a date is also presence-driven, and also one request.
    func testExplicitDateSetsItOnPresenceAwareBC3() async throws {
        let server = PresenceAwareCardServer(storedDueOn: "2024-02-01")
        let account = makeTestAccountClient(transport: server.makeTransport())

        let card = try await account.cards.update(cardId: 1069479350, dueOn: .on("2026-09-01"))

        XCTAssertEqual(server.storedDueOn, "2026-09-01")
        XCTAssertEqual(card.dueOn, "2026-09-01")
        XCTAssertEqual(server.methods, ["PUT"])
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
