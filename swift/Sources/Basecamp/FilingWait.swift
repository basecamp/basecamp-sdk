import Foundation

/// The kind of container a move or copy files a recording into (SPEC.md
/// Appendix F, Filing Waits). Every kind but ``todolist`` is a tool on a
/// project's dock, and its raw value is the tool's dock name: the destination
/// project's container is its enabled dock item of that name. A to-do list is
/// not on the dock; list the destination project's to-do lists under its
/// todoset.
public enum FilingContainer: String, Sendable, CaseIterable {
    case cardTable = "kanban_board"
    case messageBoard = "message_board"
    case vault
    case schedule
    case questionnaire
    case inbox
    case todoset
    case todolist

    /// Whether a destination of this kind can take `parentId`, a folder inside it.
    public var takesFolders: Bool { self == .vault }

    private static let byType: [String: FilingContainer] = [
        "Kanban::Card": .cardTable,
        "Kanban::Column": .cardTable,
        "Message": .messageBoard,
        "Document": .vault,
        "Upload": .vault,
        "CloudFile": .vault,
        "GoogleDocument": .vault,
        "Vault": .vault,
        "Schedule::Entry": .schedule,
        "Question": .questionnaire,
        "Inbox::Forward": .inbox,
        "Todolist": .todoset,
        "Todo": .todolist,
    ]

    /// The kind of container a move or copy files a recording into, from its
    /// recordable type and its parent's type as the recording's own read
    /// reports them (`""` when it has no parent). A card lands in Triage, or in
    /// Done if it is completed. Nil for everything else: a recording that moves
    /// only with its parent, such as a comment or the columns every card table
    /// has, and a dock tool, such as a docked Vault, which is filed by
    /// `destinationProjectId`.
    public static func of(recordingType: String, parentType: String) -> FilingContainer? {
        if recordingType == "Todolist" && parentType == "Todolist" { return .todolist }
        if recordingType == "Vault" && parentType != "Vault" { return nil }
        return byType[recordingType]
    }
}

/// How far a move or copy got when it did not complete. Only ``refused(_:)``
/// is safe to try again. It is not a ``BasecampError``: a retryable error
/// inside ``mayHaveStarted(_:)`` must not read as permission to send the start
/// again.
public enum FilingError: Error, Sendable, LocalizedError {
    /// Basecamp refused to start it (a 4xx), or the SDK refused before sending
    /// it: nothing was filed.
    case refused(BasecampError)
    /// The start failed without Basecamp refusing it, such as a 5xx or a
    /// dropped connection. It may have been filed, so look before trying again.
    case mayHaveStarted(any Error & Sendable)
    /// Basecamp started it and reported it failed.
    case failed(Filing)
    /// Basecamp started it and had not finished when the wait ended. The
    /// filing is its last known state; poll `getFiling(filingId:)` with its id
    /// rather than starting it again. `cause` is a `getFiling` that failed, or
    /// nil when the timeout ended the wait.
    case unsettled(Filing, cause: (any Error & Sendable)?)

    /// The fixture vocabulary's name for this outcome.
    public var kind: String {
        switch self {
        case .refused: "filing_refused"
        case .mayHaveStarted: "filing_may_have_started"
        case .failed: "filing_failed"
        case .unsettled: "filing_unsettled"
        }
    }

    public var message: String {
        switch self {
        case .refused(let cause): "filing refused: \(cause.localizedDescription)"
        case .mayHaveStarted(let cause): "filing may have started: \(cause)"
        case .failed(let filing): "filing \(filing.id) failed"
        case .unsettled(let filing, _): "filing \(filing.id) is still \(filing.status)"
        }
    }

    public var errorDescription: String? { message }
}

extension RecordingsService {
    /// Starts a move once, as ``move(recordingId:req:)`` does, and waits for
    /// Basecamp to finish it, polling ``getFiling(filingId:)`` with backoff.
    /// Returns the completed filing, whose `destinationRecording` is the moved
    /// recording; otherwise throws a ``FilingError`` saying how far it got.
    /// `timeout` is in seconds and defaults to a minute.
    public func moveAndWait(
        recordingId: Int, req: MoveRecordingRequest, timeout: TimeInterval? = nil
    ) async throws -> Filing {
        try await fileAndWait(timeout: timeout, start: { try await self.move(recordingId: recordingId, req: req) })
    }

    /// Starts a copy once, as ``copy(recordingId:req:)`` does, and waits for
    /// Basecamp to finish it. Only after ``FilingError/refused(_:)`` is copying
    /// again safe; anything else may already have made a copy.
    public func copyAndWait(
        recordingId: Int, req: CopyRecordingRequest, timeout: TimeInterval? = nil
    ) async throws -> Filing {
        try await fileAndWait(timeout: timeout, start: { try await self.copy(recordingId: recordingId, req: req) })
    }

    private func fileAndWait(
        timeout: TimeInterval?, start: () async throws -> Filing
    ) async throws -> Filing {
        var filing: Filing
        do {
            filing = try await start()
        } catch let error as BasecampError {
            let status = error.httpStatusCode ?? 0
            let refused: Bool
            switch error {
            case .usage, .rateLimit: refused = true
            default: refused = (400..<500).contains(status)
            }
            throw refused ? FilingError.refused(error) : FilingError.mayHaveStarted(error)
        } catch {
            throw FilingError.mayHaveStarted(error)
        }

        // Nanoseconds on the monotonic uptime clock: ContinuousClock and
        // Duration need macOS 13, and this package supports macOS 12.
        let waitSeconds = (timeout ?? 0) > 0 ? timeout! : 60
        let deadline = DispatchTime.now().uptimeNanoseconds + UInt64(waitSeconds * 1_000_000_000)
        var delay: UInt64 = 250_000_000
        while filing.status != "completed" || filing.destinationRecording == nil {
            if filing.status == "failed" { throw FilingError.failed(filing) }
            let now = DispatchTime.now().uptimeNanoseconds
            let remaining = deadline > now ? deadline - now : 0
            if remaining <= delay {
                if remaining > 0 { try await Task.sleep(nanoseconds: remaining) }
                throw FilingError.unsettled(filing, cause: nil)
            }
            try await Task.sleep(nanoseconds: delay)
            do {
                filing = try await getFiling(filingId: filing.id)
            } catch {
                throw FilingError.unsettled(filing, cause: error)
            }
            delay = min(delay * 2, 2_000_000_000)
        }
        return filing
    }
}
