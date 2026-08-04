import XCTest

@testable import Basecamp

/// Thread-safe capture of requests seen by the mock transport.
private final class ScheduleEntryRequestLog: @unchecked Sendable {
    private let lock = NSLock()
    private var _methods: [String] = []
    private var _putBody: [String: Any]?

    var methods: [String] { lock.withLock { _methods } }
    var putBody: [String: Any]? { lock.withLock { _putBody } }

    func record(_ request: URLRequest) {
        lock.withLock {
            _methods.append(request.httpMethod ?? "?")
            if request.httpMethod == "PUT",
                let data = request.httpBody ?? request.scheduleEntryBodyStreamData()
            {
                _putBody = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
            }
        }
    }
}

extension URLRequest {
    /// URLSession moves httpBody into a stream in some paths; drain it if needed.
    fileprivate func scheduleEntryBodyStreamData() -> Data? {
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

private final class ScheduleEntryOperationRecorder: BasecampHooks, @unchecked Sendable {
    private let lock = NSLock()
    private var _operations: [String] = []

    var operations: [String] { lock.withLock { _operations } }

    func onOperationStart(_ info: OperationInfo) {
        lock.withLock { _operations.append(info.operation) }
    }
}

/// Full schedule-entry JSON on wire (snake_case) keys. Every carve-out the
/// composites must NOT echo is deliberately populated: a join link under
/// `join_url`, `highlighted: true`, and two participants. `url` is the entry's
/// own Basecamp API URL — the trap the `join_url` seeding exists to avoid.
private func fullScheduleEntryJSON(id: Int = 1069479523) -> [String: Any] {
    [
        "id": id,
        "status": "active",
        "visible_to_clients": false,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "Team Meeting",
        "inherits_status": true,
        "type": "Schedule::Entry",
        "url": "https://3.basecampapi.com/999999999/buckets/1/schedule_entries/\(id).json",
        "app_url": "https://3.basecamp.com/999999999/buckets/1/schedule_entries/\(id)",
        "summary": "Team Meeting",
        "description": "<div>Agenda in the doc.</div>",
        "description_attachments": [] as [Any],
        "all_day": false,
        "starts_at": "2026-06-05T06:00:00Z",
        "ends_at": "2026-06-05T08:30:00Z",
        "join_url": "https://meet.example.com/team",
        "highlighted": true,
        "participants": [
            ["id": 1049715914, "name": "Victor Cooper"],
            ["id": 1049715915, "name": "Annie Bryan"],
        ],
        "parent": [
            "id": 1069479521,
            "title": "Schedule",
            "type": "Schedule",
            "url": "https://3.basecampapi.com/999999999/buckets/1/schedules/1069479521.json",
            "app_url": "https://3.basecamp.com/999999999/buckets/1/schedules/1069479521",
        ] as [String: Any],
        "bucket": ["id": 1, "name": "The Leto Laptop", "type": "Project"] as [String: Any],
        "creator": [
            "id": 1049715914,
            "name": "Victor Cooper",
            "email_address": "victor@honchodesign.com",
            "created_at": "2026-01-01T00:00:00Z",
            "updated_at": "2026-01-01T00:00:00Z",
        ] as [String: Any],
    ]
}

/// The merge-safe `updateEntry` / read-modify-write `editEntry` composites and
/// the raw `replaceEntry` they are built on.
///
/// `PUT /schedule_entries/{id}` is a full replace: BC3's
/// `Schedules::EntriesController#update` rebuilds the recordable from the
/// submitted params, so a sparse PUT clears whatever it omits — a 200 that
/// quietly destroys, not a 422. Three writable fields are exempt
/// (`PRESERVED_ON_OMISSION` plus the participants guard from bc3#12425):
/// `participant_ids`, `url` and `highlighted` are seeded server-side from the
/// existing recordable when the request does not address them.
///
/// So the composites split the writable set in two. The full-state fields are
/// always resent, empties included. The carve-outs stay off the wire unless the
/// caller addressed them — and an explicit `[]`, `""` or `false` IS an address.
final class SchedulesServiceExtensionsTests: XCTestCase {

    private func makeSchedulesClient(
        log: ScheduleEntryRequestLog,
        body: [String: Any]? = nil,
        hooks: (any BasecampHooks)? = nil
    ) throws -> AccountClient {
        let entryData = try JSONSerialization.data(withJSONObject: body ?? fullScheduleEntryJSON())
        let transport = MockTransport { request in
            log.record(request)
            return (
                entryData,
                makeHTTPResponse(
                    url: request.url!.absoluteString,
                    statusCode: 200,
                    headers: ["Content-Type": "application/json"]
                )
            )
        }
        return makeTestAccountClient(transport: transport, hooks: hooks)
    }

    // MARK: - updateEntry (merge-safe)

    func testUpdateEntry_mergesUnsetFullStateFields() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        let entry = try await account.schedules.updateEntry(
            entryId: 1069479523,
            req: UpdateScheduleEntryRequest(summary: "Team Meeting & Kickoff"))

        XCTAssertEqual(entry.id, 1069479523)
        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["summary"] as? String, "Team Meeting & Kickoff")
        // The four the caller never named are written straight back rather than
        // left to the endpoint's clear-by-default.
        XCTAssertEqual(body["starts_at"] as? String, "2026-06-05T06:00:00Z")
        XCTAssertEqual(body["ends_at"] as? String, "2026-06-05T08:30:00Z")
        XCTAssertEqual(body["description"] as? String, "<div>Agenda in the doc.</div>")
        XCTAssertEqual(body["all_day"] as? Bool, false)
    }

    /// The read-back carries a join link, a highlight and two participants.
    /// None may be echoed: BC3 preserves all three when the request stays
    /// silent, so resending is redundant at best and wrong if the read raced a
    /// concurrent change.
    func testUpdateEntry_neverEchoesCarveOutsFromTheReadBack() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.updateEntry(
            entryId: 1069479523, req: UpdateScheduleEntryRequest(summary: "Team Sync"))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertFalse(body.keys.contains("participant_ids"), "participants must stay off the wire")
        XCTAssertFalse(
            body.keys.contains("url"),
            "the read-back's `url` is the entry's own API URL, never a join link")
        XCTAssertFalse(body.keys.contains("highlighted"), "the highlight must stay off the wire")
        XCTAssertFalse(body.keys.contains("notify"), "notify is a directive, never state")
    }

    /// An explicitly-passed empty value is an address, not an absence. A `??`,
    /// `||` or truthiness-based compaction that dropped these would hand the
    /// clear back to BC3's carve-out, which preserves instead.
    func testUpdateEntry_explicitEmptiesReachTheWire() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.updateEntry(
            entryId: 1069479523,
            req: UpdateScheduleEntryRequest(highlighted: false, participantIds: [], url: ""))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertNotNil(body["url"], "an explicit clear must be sent, not omitted")
        XCTAssertEqual(body["url"] as? String, "")
        XCTAssertEqual(body["highlighted"] as? Bool, false)
        let ids = try XCTUnwrap(body["participant_ids"] as? [Any])
        XCTAssertTrue(ids.isEmpty, "an empty list means 'remove everyone' and must be sent")
    }

    /// The request spells the join link `url`; the response spells it
    /// `join_url`. Addressing it writes the caller's value under the request
    /// spelling, and leaves the untouched participants alone — the three
    /// carve-outs are independent, not all-or-nothing.
    func testUpdateEntry_addressedJoinLinkUsesTheRequestSpelling() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.updateEntry(
            entryId: 1069479523,
            req: UpdateScheduleEntryRequest(
                highlighted: true, url: "https://meet.example.com/new-room"))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["url"] as? String, "https://meet.example.com/new-room")
        XCTAssertEqual(body["highlighted"] as? Bool, true)
        XCTAssertFalse(body.keys.contains("participant_ids"))
        // Full state still rides along.
        XCTAssertEqual(body["summary"] as? String, "Team Meeting")
        XCTAssertEqual(body["starts_at"] as? String, "2026-06-05T06:00:00Z")
    }

    /// An all-day entry's bounds are bare dates on the wire. Parsing and
    /// re-rendering them would either fail outright or silently rewrite the
    /// entry as a midnight-to-midnight timed one; they are carried verbatim.
    func testUpdateEntry_roundTripsAnAllDayBareDateVerbatim() async throws {
        var allDay = fullScheduleEntryJSON()
        allDay["all_day"] = true
        allDay["starts_at"] = "2026-06-01"
        allDay["ends_at"] = "2026-06-02"
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log, body: allDay)

        _ = try await account.schedules.updateEntry(
            entryId: 1069479523, req: UpdateScheduleEntryRequest(summary: "Offsite"))

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["starts_at"] as? String, "2026-06-01")
        XCTAssertEqual(body["ends_at"] as? String, "2026-06-02")
        XCTAssertEqual(body["all_day"] as? Bool, true)
    }

    func testUpdateEntry_hooksObserveGetThenReplace() async throws {
        let log = ScheduleEntryRequestLog()
        let recorder = ScheduleEntryOperationRecorder()
        let account = try makeSchedulesClient(log: log, hooks: recorder)

        _ = try await account.schedules.updateEntry(
            entryId: 1069479523, req: UpdateScheduleEntryRequest(summary: "observed"))

        // The composite is built from the public getEntry/replaceEntry, so
        // hooks see the two wire operations, not a synthetic composite.
        XCTAssertEqual(recorder.operations, ["GetScheduleEntry", "ReplaceScheduleEntry"])
    }

    // MARK: - updateEntry read-side guards

    /// `Schedule::Entry#summary` is `super.presence || "Untitled"`, so BC3 can
    /// never render it blank. `Codable` already refuses an absent or null
    /// summary because the model field is non-optional; `""` decodes fine and
    /// needs the hand-written check. The ordering is what matters: no PUT.
    func testUpdateEntry_refusesABlankSummary() async throws {
        var blank = fullScheduleEntryJSON()
        blank["summary"] = "   "
        try await assertMalformedReadAborts(body: blank)
    }

    /// `summary` is `@required` on the response, so an absent one is a
    /// `DecodingError` — normalized to the same statusless `.api` shape the
    /// blank check throws, per the Documents precedent.
    func testUpdateEntry_refusesAnAbsentSummary() async throws {
        var missing = fullScheduleEntryJSON()
        missing.removeValue(forKey: "summary")
        try await assertMalformedReadAborts(body: missing)
    }

    /// `all_day` is `NOT NULL DEFAULT false` and every partial emits it.
    /// Defaulting a missing one to `false` would convert an all-day event into
    /// a midnight-to-midnight timed one, so the decoder's refusal stands.
    func testUpdateEntry_refusesAMissingAllDay() async throws {
        var missing = fullScheduleEntryJSON()
        missing.removeValue(forKey: "all_day")
        try await assertMalformedReadAborts(body: missing)
    }

    func testUpdateEntry_refusesAMissingStartsAt() async throws {
        var missing = fullScheduleEntryJSON()
        missing.removeValue(forKey: "starts_at")
        try await assertMalformedReadAborts(body: missing)
    }

    /// `description` is optional and nullable — absent or null is genuinely
    /// empty — but a non-string is malformed, and resending it verbatim is not
    /// something the composite can do safely.
    func testUpdateEntry_refusesANonStringDescription() async throws {
        var wrongType = fullScheduleEntryJSON()
        wrongType["description"] = 42
        try await assertMalformedReadAborts(body: wrongType)
    }

    /// Every malformed read must abort BEFORE the PUT with the SDK's statusless,
    /// non-retryable `api_error`.
    private func assertMalformedReadAborts(
        body: [String: Any], file: StaticString = #filePath, line: UInt = #line
    ) async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log, body: body)

        do {
            _ = try await account.schedules.updateEntry(
                entryId: 1069479523, req: UpdateScheduleEntryRequest(summary: "never written"))
            XCTFail("expected the call to fail, but it succeeded", file: file, line: line)
        } catch let error as BasecampError {
            guard case .api(_, let httpStatus, let hint, _) = error else {
                return XCTFail("expected .api, got \(error)", file: file, line: line)
            }
            XCTAssertNil(httpStatus, "a malformed 2xx body carries no status", file: file, line: line)
            XCTAssertNotNil(hint, "expected a hint naming the escape hatch", file: file, line: line)
        }

        XCTAssertEqual(log.methods, ["GET"], "the guard must fire before the PUT", file: file, line: line)
    }

    // MARK: - editEntry (read-modify-write closure)

    /// The carve-outs are seeded so the closure can inspect them before
    /// deciding — and `url` is seeded from `join_url`, never from the entry's
    /// own `url`.
    func testEditEntry_seedsCarveOutsForReadingFromJoinUrl() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.editEntry(entryId: 1069479523) { fields in
            XCTAssertEqual(fields.summary, "Team Meeting")
            XCTAssertEqual(fields.startsAt, "2026-06-05T06:00:00Z")
            XCTAssertEqual(fields.description, "<div>Agenda in the doc.</div>")
            XCTAssertEqual(fields.allDay, false)
            XCTAssertEqual(fields.url, "https://meet.example.com/team")
            XCTAssertEqual(fields.highlighted, true)
            XCTAssertEqual(fields.participantIds, [1049715914, 1049715915])
            XCTAssertFalse(fields.notify)
            fields.summary = "Team Sync"
        }

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["summary"] as? String, "Team Sync")
    }

    /// The untouched half of the dirty-set contract: seeding a carve-out for
    /// reading must not put it on the wire.
    func testEditEntry_untouchedCarveOutsStayOffTheWire() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.editEntry(entryId: 1069479523) { fields in
            fields.summary = "Team Sync"
        }

        XCTAssertEqual(log.methods, ["GET", "PUT"])
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["summary"] as? String, "Team Sync")
        XCTAssertFalse(body.keys.contains("participant_ids"))
        XCTAssertFalse(body.keys.contains("url"))
        XCTAssertFalse(body.keys.contains("highlighted"))
        XCTAssertFalse(body.keys.contains("notify"))
    }

    /// The touched half, and the reason the contract is setter-invocation
    /// dirty tracking rather than a snapshot diff. The closure assigns exactly
    /// the join link and highlight the GET returned: a value-comparison
    /// implementation would conclude nothing changed and omit both, handing the
    /// write back to BC3's carve-out. Intent is not recoverable from the value.
    func testEditEntry_assigningTheReadBackValueStillSendsIt() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.editEntry(entryId: 1069479523) { fields in
            let seededURL = fields.url
            let seededHighlighted = fields.highlighted
            fields.url = seededURL
            fields.highlighted = seededHighlighted
        }

        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["url"] as? String, "https://meet.example.com/team")
        XCTAssertEqual(body["highlighted"] as? Bool, true)
        XCTAssertEqual(body["summary"] as? String, "Team Meeting")
        // Participants were never assigned, so they stay off the wire — the
        // carve-outs are tracked one by one, not as a group.
        XCTAssertFalse(body.keys.contains("participant_ids"))
    }

    func testEditEntry_explicitEmptiesOnCarveOutsReachTheWire() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.editEntry(entryId: 1069479523) { fields in
            fields.url = ""
            fields.highlighted = false
            fields.participantIds = []
        }

        let body = try XCTUnwrap(log.putBody)
        XCTAssertNotNil(body["url"], "a cleared join link must be sent present-and-empty")
        XCTAssertEqual(body["url"] as? String, "")
        XCTAssertEqual(body["highlighted"] as? Bool, false)
        let ids = try XCTUnwrap(body["participant_ids"] as? [Any])
        XCTAssertTrue(ids.isEmpty)
    }

    func testEditEntry_clearsDescriptionPresentAndEmpty() async throws {
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        _ = try await account.schedules.editEntry(entryId: 1069479523) { fields in
            fields.description = ""
        }

        let body = try XCTUnwrap(log.putBody)
        // Clearing on a full-replace endpoint is an explicit "": never JSON
        // null (SPEC §18 body compaction), and never by omission, which would
        // leave the clear to the server and read as an accident.
        XCTAssertNotNil(body["description"], "a cleared description must be present-and-empty")
        XCTAssertEqual(body["description"] as? String, "")
        XCTAssertEqual(body["summary"] as? String, "Team Meeting")
        XCTAssertEqual(body["starts_at"] as? String, "2026-06-05T06:00:00Z")
        XCTAssertEqual(body["ends_at"] as? String, "2026-06-05T08:30:00Z")
        XCTAssertEqual(body["all_day"] as? Bool, false)
    }

    func testEditEntry_closureErrorAbortsWithoutPut() async throws {
        struct Abort: Error {}
        let log = ScheduleEntryRequestLog()
        let account = try makeSchedulesClient(log: log)

        do {
            _ = try await account.schedules.editEntry(entryId: 1069479523) { fields in
                fields.summary = "never written"
                throw Abort()
            }
            XCTFail("expected the closure error to propagate")
        } catch is Abort {
            // expected
        }

        XCTAssertEqual(log.methods, ["GET"], "no PUT after a closure error")
    }

    // MARK: - replaceEntry (server-native verbatim PUT)

    /// SPEC §18 rule 6: the generated single-request method stays reachable
    /// under a name that says what it does. A full payload, and still exactly
    /// one PUT — no GET at index 0.
    func testReplaceEntry_sendsVerbatimWithNoGet() async throws {
        let log = ScheduleEntryRequestLog()
        let recorder = ScheduleEntryOperationRecorder()
        let account = try makeSchedulesClient(log: log, hooks: recorder)

        let entry = try await account.schedules.replaceEntry(
            entryId: 1069479523,
            req: ReplaceScheduleEntryRequest(
                endsAt: "2026-07-01T17:00:00Z",
                startsAt: "2026-07-01T09:00:00Z",
                summary: "Offsite"))

        XCTAssertEqual(entry.id, 1069479523)
        XCTAssertEqual(log.methods, ["PUT"], "replaceEntry must not GET")
        let body = try XCTUnwrap(log.putBody)
        XCTAssertEqual(body["summary"] as? String, "Offsite")
        // The raw path is presence-bearing by design: what the caller left out
        // stays out, and BC3's carve-out preserves it.
        XCTAssertFalse(body.keys.contains("participant_ids"))
        XCTAssertFalse(body.keys.contains("url"))
        XCTAssertFalse(body.keys.contains("highlighted"))
        XCTAssertEqual(recorder.operations, ["ReplaceScheduleEntry"])
    }
}
