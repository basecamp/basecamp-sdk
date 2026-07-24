import XCTest
@testable import Basecamp

/// Tests for the generated WormholesService: card-table wormhole CRUD plus the
/// wormholes[] decode on a card table.
final class WormholesServiceTests: XCTestCase {

    private func wormholeJSON(id: Int, linked: Bool = true) -> [String: Any] {
        var json: [String: Any] = [
            "id": id,
            "status": "active",
            "visible_to_clients": false,
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
            "title": "Design → Marketing backlog",
            "inherits_status": true,
            "type": "Kanban::Wormhole",
            "url": "https://3.basecampapi.com/1/buckets/1/card_tables/wormholes/\(id).json",
            "app_url": "https://3.basecamp.com/1/buckets/1/card_tables/wormholes/\(id)",
            "parent": ["id": 10, "title": "Development Board", "type": "Kanban::Board", "app_url": "a", "url": "u"] as [String: Any],
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "linked": linked,
        ]
        // color and destination_url are required-but-nullable: an unlinked wormhole
        // carries an explicit null for both, exercising the nullable decode path.
        if linked {
            json["color"] = "#f5d76e"
            json["destination_url"] = "https://3.basecampapi.com/1/buckets/2/card_tables/columns/500.json"
        } else {
            json["color"] = NSNull()
            json["destination_url"] = NSNull()
        }
        return json
    }

    func testCreateEncodesBodyAndDecodes() async throws {
        let responseData = try JSONSerialization.data(withJSONObject: wormholeJSON(id: 99))
        let transport = MockTransport(statusCode: 201, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let req = CreateWormholeRequest(destinationRecordingId: 500)
        let wormhole = try await account.wormholes.create(bucketId: 1, cardTableId: 42, req: req)

        XCTAssertEqual(wormhole.id, 99)
        XCTAssertTrue(wormhole.linked)
        XCTAssertNotNil(wormhole.destinationUrl)

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/buckets/1/card_tables/42/wormholes.json"), "Got \(sentURL)")
        let sentBody = transport.lastRequest!.request.httpBody!
        let sentJSON = try JSONSerialization.jsonObject(with: sentBody) as! [String: Any]
        XCTAssertEqual(sentJSON["destination_recording_id"] as? Int, 500)
    }

    func testCreateValidationErrorAtLimit() async throws {
        let transport = MockTransport(statusCode: 422, data: Data(#"{"error":"Limit reached"}"#.utf8))
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.wormholes.create(bucketId: 1, cardTableId: 42, req: CreateWormholeRequest(destinationRecordingId: 500))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .validation = error else { return XCTFail("Expected .validation, got \(error)") }
        }
    }

    func testCreateNotFoundDestination() async throws {
        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.wormholes.create(bucketId: 1, cardTableId: 42, req: CreateWormholeRequest(destinationRecordingId: 999))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
        }
    }

    func testUpdateSendsPUTAndDecodes() async throws {
        let responseData = try JSONSerialization.data(withJSONObject: wormholeJSON(id: 400))
        let transport = MockTransport(statusCode: 200, data: responseData)
        let account = makeTestAccountClient(transport: transport)

        let wormhole = try await account.wormholes.update(bucketId: 1, wormholeId: 400, req: UpdateWormholeRequest(destinationRecordingId: 501))

        XCTAssertEqual(wormhole.id, 400)
        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/buckets/1/card_tables/wormholes/400"), "Got \(sentURL)")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "PUT")
    }

    func testUpdateNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.wormholes.update(bucketId: 1, wormholeId: 999, req: UpdateWormholeRequest(destinationRecordingId: 1))
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
        }
    }

    func testDeleteSendsDELETE() async throws {
        let transport = MockTransport(statusCode: 204, data: Data())
        let account = makeTestAccountClient(transport: transport)

        try await account.wormholes.delete(bucketId: 1, wormholeId: 400)

        let sentURL = transport.lastRequest!.request.url!.absoluteString
        XCTAssertTrue(sentURL.hasSuffix("/buckets/1/card_tables/wormholes/400"), "Got \(sentURL)")
        XCTAssertEqual(transport.lastRequest!.request.httpMethod, "DELETE")
    }

    func testDeleteForbidden() async throws {
        let transport = MockTransport(statusCode: 403, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.wormholes.delete(bucketId: 1, wormholeId: 400)
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .forbidden = error else { return XCTFail("Expected .forbidden, got \(error)") }
        }
    }

    func testDeleteNotFound() async throws {
        let transport = MockTransport(statusCode: 404, data: Data())
        let account = makeTestAccountClient(transport: transport)

        do {
            try await account.wormholes.delete(bucketId: 1, wormholeId: 999)
            XCTFail("Expected error")
        } catch let error as BasecampError {
            guard case .notFound = error else { return XCTFail("Expected .notFound, got \(error)") }
        }
    }

    func testCardTableDecodesLinkedAndUnlinkedWormholes() async throws {
        let json: [String: Any] = [
            "id": 1069479345,
            "status": "active",
            "visible_to_clients": false,
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
            "title": "Development Board",
            "inherits_status": true,
            "type": "Kanban::Board",
            "url": "u",
            "app_url": "a",
            "bucket": ["id": 1, "name": "Project", "type": "Project"] as [String: Any],
            "creator": ["id": 1, "name": "Test User"] as [String: Any],
            "wormholes": [wormholeJSON(id: 400, linked: true), wormholeJSON(id: 401, linked: false)],
        ]
        let data = try JSONSerialization.data(withJSONObject: json)
        let transport = MockTransport(statusCode: 200, data: data)
        let account = makeTestAccountClient(transport: transport)

        let cardTable = try await account.cardTables.get(cardTableId: 1069479345)

        XCTAssertEqual(cardTable.wormholes?.count, 2)
        XCTAssertEqual(cardTable.wormholes?[0].linked, true)
        XCTAssertNotNil(cardTable.wormholes?[0].destinationUrl)
        XCTAssertNotNil(cardTable.wormholes?[0].color)
        XCTAssertEqual(cardTable.wormholes?[1].linked, false)
        XCTAssertNil(cardTable.wormholes?[1].destinationUrl)
        XCTAssertNil(cardTable.wormholes?[1].color)
    }
}
