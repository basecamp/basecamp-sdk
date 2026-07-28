import XCTest
@testable import Basecamp

/// Swift mirror of `conformance/tests/schedule_entries_write.json`.
///
/// Swift is not part of `make conformance`, so these carry the same two
/// assertions the five executable runners make.
///
/// The contract is BC3-side and recent. Until basecamp/bc3#12425,
/// `Schedules::EntriesController#update` called `replace_participants`
/// unconditionally, so an update omitting `participant_ids` removed every
/// participant and notified each one — including the shape in BC3's own
/// "Update a schedule entry" doc example. The controller now guards on the
/// request actually addressing participants, which makes the SDK's job narrow
/// but load-bearing: an unaddressed list must stay *off the wire entirely*.
/// Emitting `null`, or defaulting to `[]`, would clear it on the server.
private final class ScheduleRequestLog: @unchecked Sendable {
    private let lock = NSLock()
    private var _putBody: [String: Any]?

    var putBody: [String: Any]? { lock.withLock { _putBody } }

    func record(_ request: URLRequest) {
        guard request.httpMethod == "PUT" else { return }
        lock.withLock {
            if let data = request.httpBody ?? request.scheduleBodyStreamData() {
                _putBody = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            }
        }
    }
}

extension URLRequest {
    fileprivate func scheduleBodyStreamData() -> Data? {
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

private func scheduleEntryJSON(id: Int = 1069479523) -> [String: Any] {
    [
        "id": id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "Team Meeting",
        "inherits_status": true,
        "type": "Schedule::Entry",
        "url": "https://3.basecampapi.com/999/buckets/1/schedule_entries/\(id).json",
        "app_url": "https://3.basecamp.com/999/buckets/1/schedule_entries/\(id)",
        "summary": "Team Meeting",
        "all_day": false,
        "starts_at": "2026-06-05T06:00:00Z",
        "ends_at": "2026-06-05T08:30:00Z",
        "description_attachments": [] as [Any],
        "participants": [
            ["id": 1049715914, "name": "Victor Cooper"],
            ["id": 1049715915, "name": "Annie Bryan"],
        ],
        "parent": [
            "id": 1069479521,
            "title": "Schedule",
            "type": "Schedule",
            "url": "https://3.basecampapi.com/999/buckets/1/schedules/1069479521.json",
            "app_url": "https://3.basecamp.com/999/buckets/1/schedules/1069479521",
        ],
        "bucket": ["id": 1, "name": "The Leto Laptop", "type": "Project"],
        "creator": [
            "id": 1049715914,
            "name": "Victor Cooper",
            "email_address": "victor@honchodesign.com",
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
        ],
    ]
}

final class ScheduleEntryParticipantsTests: XCTestCase {
    private func makeClient(log: ScheduleRequestLog) throws -> AccountClient {
        let data = try JSONSerialization.data(withJSONObject: scheduleEntryJSON())
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

    func testOmittedParticipantIdsStayOffTheWire() async throws {
        let log = ScheduleRequestLog()
        let account = try makeClient(log: log)

        _ = try await account.schedules.updateEntry(
            entryId: 1069479523,
            req: UpdateScheduleEntryRequest(summary: "Team Meeting")
        )

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["summary"] as? String, "Team Meeting")
        XCTAssertNil(
            body["participant_ids"],
            "an unaddressed participant list must not reach the wire — BC3 preserves participants only because the key is absent"
        )
        XCTAssertFalse(body.keys.contains("participant_ids"))
    }

    func testExplicitEmptyParticipantIdsReachTheWire() async throws {
        let log = ScheduleRequestLog()
        let account = try makeClient(log: log)

        _ = try await account.schedules.updateEntry(
            entryId: 1069479523,
            req: UpdateScheduleEntryRequest(participantIds: [], summary: "Team Meeting")
        )

        let body = try XCTUnwrap(log.putBody)
        let ids = try XCTUnwrap(
            body["participant_ids"] as? [Any],
            "an explicitly empty list means 'remove everyone' and must be sent"
        )
        XCTAssertTrue(ids.isEmpty)
    }
}
