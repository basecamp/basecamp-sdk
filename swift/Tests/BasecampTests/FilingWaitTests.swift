import XCTest
@testable import Basecamp

/// The filing waits (SPEC Appendix F, Filing Waits): a move or copy started
/// once and polled until it settles.
final class FilingWaitTests: XCTestCase {
    private static func filing(_ status: String, destination: Bool = false) -> Data {
        let dest = destination
            ? #","destination_recording":{"id":900,"type":"Message","url":"u","app_url":"a"}"#
            : ""
        return Data(#"{"id":4,"status":"\#(status)","type":"Copy","source_recording_id":456,"url":"u"\#(dest)}"#.utf8)
    }

    /// Answers the start with `start` and each poll with the next of `polls`,
    /// repeating the last.
    private static func transport(start: (Int, Data), polls: [Data]) -> MockTransport {
        let counter = Counter()
        return MockTransport { request in
            let url = request.url!
            if request.httpMethod == "POST" {
                return (start.1, HTTPURLResponse(url: url, statusCode: start.0, httpVersion: "HTTP/1.1", headerFields: nil)!)
            }
            let n = counter.next()
            let body = polls[min(n, polls.count - 1)]
            return (body, HTTPURLResponse(url: url, statusCode: 200, httpVersion: "HTTP/1.1", headerFields: nil)!)
        }
    }

    private final class Counter: @unchecked Sendable {
        private let lock = NSLock()
        private var value = 0
        func next() -> Int { lock.withLock { defer { value += 1 }; return value } }
    }

    private func posts(_ transport: MockTransport) -> Int {
        transport.requests.filter { $0.request.httpMethod == "POST" }.count
    }

    func testCopyAndWaitPollsUntilCompleted() async throws {
        let transport = Self.transport(
            start: (201, Self.filing("pending")),
            polls: [Self.filing("processing"), Self.filing("completed", destination: true)])
        let account = makeTestAccountClient(transport: transport)

        let filing = try await account.recordings.copyAndWait(
            recordingId: 456, req: CopyRecordingRequest(filingDestinationId: 789))

        XCTAssertEqual(filing.destinationRecording?.id, 900)
        XCTAssertEqual(posts(transport), 1)
        XCTAssertEqual(transport.requests.count, 3)
    }

    func testMoveAndWaitReportsAFailedFiling() async throws {
        let transport = Self.transport(start: (201, Self.filing("pending")), polls: [Self.filing("failed")])
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.recordings.moveAndWait(
                recordingId: 456, req: MoveRecordingRequest(filingDestinationId: 789))
            XCTFail("Expected a failed filing")
        } catch FilingError.failed(let filing) {
            XCTAssertEqual(filing.id, 4)
        }
    }

    func testTheWaitEndsWhileProcessing() async throws {
        let transport = Self.transport(start: (201, Self.filing("pending")), polls: [Self.filing("processing")])
        let account = makeTestAccountClient(transport: transport)

        do {
            _ = try await account.recordings.copyAndWait(
                recordingId: 456, req: CopyRecordingRequest(filingDestinationId: 789), timeout: 0.6)
            XCTFail("Expected the wait to end")
        } catch FilingError.unsettled(let filing, let cause) {
            XCTAssertEqual(filing.status, "processing")
            XCTAssertNil(cause)
        }
        XCTAssertEqual(posts(transport), 1)
    }

    func testARefusedStartPollsNothing() async throws {
        for status in [403, 404, 422] {
            let transport = MockTransport(statusCode: status, data: Data("{}".utf8))
            let account = makeTestAccountClient(transport: transport)

            do {
                _ = try await account.recordings.moveAndWait(
                    recordingId: 456, req: MoveRecordingRequest(filingDestinationId: 789))
                XCTFail("Expected a refusal")
            } catch FilingError.refused(let cause) {
                XCTAssertEqual(cause.httpStatusCode, status)
            }
            XCTAssertEqual(transport.requests.count, 1)
        }
    }

    func testAStartThatMayHaveFiledIsSentOnce() async throws {
        for status in [500, 503] {
            let transport = MockTransport(statusCode: status, data: Data("{}".utf8))
            let account = makeTestAccountClient(transport: transport, enableRetry: true)

            do {
                _ = try await account.recordings.copyAndWait(
                    recordingId: 456, req: CopyRecordingRequest(filingDestinationId: 789))
                XCTFail("Expected a start that may have filed")
            } catch FilingError.mayHaveStarted {
            }
            XCTAssertEqual(transport.requests.count, 1)
        }
    }

    func testFilingContainers() {
        let rows: [(String, String, String?)] = [
            ("Kanban::Card", "Kanban::Triage", "kanban_board"),
            ("Kanban::Column", "Kanban::Board", "kanban_board"),
            ("Message", "Message::Board", "message_board"),
            ("Document", "Vault", "vault"),
            ("Upload", "Vault", "vault"),
            ("CloudFile", "Vault", "vault"),
            ("GoogleDocument", "Vault", "vault"),
            ("Vault", "Vault", "vault"),
            ("Schedule::Entry", "Schedule", "schedule"),
            ("Question", "Questionnaire", "questionnaire"),
            ("Inbox::Forward", "Inbox", "inbox"),
            ("Todolist", "Todoset", "todoset"),
            ("Todolist", "Todolist", "todolist"),
            ("Todo", "Todolist", "todolist"),
            ("Vault", "", nil),
            ("Message::Board", "", nil),
            ("Kanban::Triage", "Kanban::Board", nil),
            ("Comment", "Message", nil),
            ("Question::Answer", "Question", nil),
        ]
        for (recording, parent, want) in rows {
            let got = FilingContainer.of(recordingType: recording, parentType: parent)
            XCTAssertEqual(got?.rawValue, want, "\(recording) under \(parent)")
            if let got { XCTAssertEqual(got.takesFolders, want == "vault") }
        }
    }
}
