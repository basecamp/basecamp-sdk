import Foundation
import XCTest
@testable import Basecamp

/// `GetUpcomingSchedule` renders BC3's reduced calendar partials
/// (`app/views/api/schedules/calendar/_entry.json.jbuilder` and
/// `_assignable.json.jbuilder`), not the per-resource ones. Until #635 the spec
/// declared the shared `ScheduleEntry` and a half-modelled `Assignable` instead,
/// and Swift is the tier where that was not merely untidy: `ReportsService`
/// returns the typed content and `BaseService.request` decodes it with a plain
/// `JSONDecoder`, so a response containing a schedule entry, a recurring
/// occurrence or an assignable threw `DecodingError.keyNotFound`.
///
/// The failure was never "every live response" — an empty window decodes fine,
/// and a call with no window 400s before any body exists. It was **any populated
/// window**: one entry, one occurrence or one assignable was enough.
///
/// These tests drive the real service through the real decoder against the
/// shared `spec/fixtures/schedules/upcoming.json` body, which is validated
/// against the generated schema by `make check-fixture-coverage`. Reading it
/// from disk rather than restating it inline is deliberate: an invented body is
/// how the mismatch survived six SDKs in the first place.
final class UpcomingScheduleDecodeTests: XCTestCase {
    /// The shared fixture, read relative to this source file so it cannot drift
    /// from the copy the other five SDKs and the conformance runners use.
    private func sharedFixture() throws -> Data {
        let repoRoot = URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent()  // BasecampTests
            .deletingLastPathComponent()  // Tests
            .deletingLastPathComponent()  // swift
            .deletingLastPathComponent()  // <repo root>
        return try Data(
            contentsOf: repoRoot
                .appendingPathComponent("spec/fixtures/schedules/upcoming.json")
        )
    }

    private func upcoming(body: Data) async throws -> GetUpcomingScheduleResponseContent {
        let transport = MockTransport(
            statusCode: 200,
            data: body,
            headers: ["Content-Type": "application/json"]
        )
        let account = makeTestAccountClient(transport: transport)
        return try await account.reports.upcoming(
            windowStartsOn: "2026-06-01",
            windowEndsOn: "2026-06-30"
        )
    }

    /// The regression proof. Against the pre-#635 contract this throws
    /// `DecodingError.keyNotFound` on `bucket.type` — the nested omission a
    /// strict decoder reaches before any of the six top-level members the
    /// calendar partial drops.
    func testPopulatedWindowDecodes() async throws {
        let result = try await upcoming(body: try sharedFixture())

        XCTAssertEqual(result.scheduleEntries.count, 1)
        XCTAssertEqual(result.recurringScheduleEntryOccurrences.count, 1)
        XCTAssertEqual(result.assignables.count, 2)

        let entry = result.scheduleEntries[0]
        XCTAssertEqual(entry.summary, "Team Meeting")
        XCTAssertEqual(entry.type, "ScheduleEntry")
        // `recurring` is emitted only by the calendar partial, and it separates
        // the two envelope arrays: BC3 selects schedule_entries with
        // recurrence_schedule IS NULL and the occurrences with it NOT NULL.
        XCTAssertFalse(entry.recurring)
        XCTAssertEqual(entry.startsAt, "2026-06-05T06:00:00.000Z")
        // id + name, no `type`: UpcomingScheduleBucket has no such member, so
        // this line would not compile against TodoBucket's shape.
        XCTAssertEqual(entry.bucket.id, 2085958499)
        XCTAssertEqual(entry.bucket.name, "The Leto Laptop")
        XCTAssertEqual(entry.participants.count, 2)
        XCTAssertEqual(entry.creator.name, "Victor Cooper")
        XCTAssertEqual(entry.commentsCount, 2)

        let occurrence = result.recurringScheduleEntryOccurrences[0]
        XCTAssertTrue(occurrence.recurring)
        XCTAssertTrue(occurrence.allDay)
        // An all-day entry reads back as a bare date, not a timestamp.
        XCTAssertEqual(occurrence.startsAt, "2026-06-08")
        XCTAssertEqual(occurrence.endsAt, "2026-06-08")
        XCTAssertTrue(occurrence.participants.isEmpty)

        // BC3 emits the item text as `content`. The retired model declared
        // `title`, so the one field a caller actually wants was permanently nil.
        let todo = result.assignables[0]
        XCTAssertEqual(todo.content, "Ship the hardware")
        XCTAssertEqual(todo.type, "todo")
        XCTAssertEqual(todo.parent.title, "Launch: Hardware")
        XCTAssertEqual(todo.startsOn, "2026-06-01")
        XCTAssertEqual(todo.dueOn, "2026-06-10")
        XCTAssertTrue(todo.completed)
        XCTAssertFalse(todo.repeating)
        XCTAssertEqual(todo.completion?.creator.name, "Steve Marsh")

        let card = result.assignables[1]
        XCTAssertEqual(card.type, "card")
        // Kanban::Card and Step both define starts_on as a literal nil to
        // duck-type Todo, and the partial reads it unconditionally.
        XCTAssertNil(card.startsOn)
        XCTAssertNil(card.dueOn)
        // The one conditional key in either partial: absent, not null.
        XCTAssertNil(card.completion)
        // Non-to-dos get a `_path` helper, which emits no host.
        XCTAssertEqual(card.completionUrl, "/999/buckets/2085958499/steps/1069479526/completions.json")
        XCTAssertTrue(card.assignees.isEmpty)
    }

    /// The half that always worked, pinned so the failure claim stays precise:
    /// an empty window is three empty arrays and decodes on any contract.
    func testEmptyWindowDecodes() async throws {
        let body = Data(
            """
            {"schedule_entries": [], "recurring_schedule_entry_occurrences": [], "assignables": []}
            """.utf8
        )
        let result = try await upcoming(body: body)

        XCTAssertTrue(result.scheduleEntries.isEmpty)
        XCTAssertTrue(result.recurringScheduleEntryOccurrences.isEmpty)
        XCTAssertTrue(result.assignables.isEmpty)
    }

    /// All three arrays are `@required`: BC3's index template writes every key
    /// unconditionally, so a body missing one is a contract violation rather
    /// than a shape to tolerate.
    func testRejectsMissingEnvelopeArray() async {
        let body = Data(
            """
            {"schedule_entries": [], "recurring_schedule_entry_occurrences": []}
            """.utf8
        )

        do {
            _ = try await upcoming(body: body)
            XCTFail("expected a decode failure for an envelope missing `assignables`")
        } catch let error as BasecampError {
            // Since #604 the decoder's refusal wears the SPEC §6
            // malformed-2xx-body shape; the `DecodingError`'s own description
            // rides in the message, and since #750 the exception itself rides in
            // `decodeFailure`.
            let message = assertStatuslessDecodeFailure(error)
            XCTAssertTrue(message.contains("keyNotFound"), message)
            XCTAssertTrue(message.contains("assignables"), message)
        } catch {
            XCTFail("expected BasecampError, got a raw \(type(of: error)): \(error)")
        }
    }

    /// Both window bounds are `@required`, so they are plain parameters rather
    /// than an options struct and always reach the query string. BC3 answers a
    /// bodiless 400 without them, which is why the SDK can no longer express the
    /// call that produced it.
    func testSendsBothWindowBounds() async throws {
        let transport = MockTransport(
            statusCode: 200,
            data: Data(
                """
                {"schedule_entries": [], "recurring_schedule_entry_occurrences": [], "assignables": []}
                """.utf8
            ),
            headers: ["Content-Type": "application/json"]
        )
        let account = makeTestAccountClient(transport: transport)
        _ = try await account.reports.upcoming(windowStartsOn: "2026-06-01", windowEndsOn: "2026-06-30")

        let url = try XCTUnwrap(transport.lastRequest?.request.url?.absoluteString)
        XCTAssertTrue(url.contains("window_starts_on=2026-06-01"), url)
        XCTAssertTrue(url.contains("window_ends_on=2026-06-30"), url)
    }
}
