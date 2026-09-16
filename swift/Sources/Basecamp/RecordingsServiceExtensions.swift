import Foundation

/// A compact projection of one recording, resolved from the pointer an account
/// event feed row or a webhook carries — bucket id, recording id, and the event
/// type or recording type — through the typed read that type names.
///
/// It exists for consumers that must decide something about a recording without
/// paying for its full payload: an agent connector's admission step, an MCP tool
/// answering "what is this?".
///
/// The SDK has no untyped recording read (BC3 has no such route), so the type is
/// the routing key: `comment.created` reads a comment, `card.created` reads a
/// card, and so on — one typed read per type. Chat lines are the exception,
/// because their read needs the Campfire id and the pointer does not carry it;
/// ``RecordingsService/summarize(_:)`` discovers the Campfire first (see
/// ``CampfireIndex``).
///
/// This is hand-written composition over the generated services (SPEC §18,
/// Appendix F "Recording Summaries and Mention Helpers"), in the Swift
/// same-module extension §18 rule 5 designates. It makes no wire request of its
/// own, and it mints no operation identity: hooks see the constituent reads
/// under their own names (§18 rule 3).

// MARK: - Inputs

/// Points at one recording the way an event feed row does.
public struct RecordingRef: Sendable, Equatable {
    /// The project the recording lives in. Required: it scopes the Campfire
    /// discovery for chat lines, and the read is checked against it so a pointer
    /// from one project can never resolve to a recording in another.
    public var bucketId: Int

    /// The recording's id.
    public var recordingId: Int

    /// The account event feed type that named the recording —
    /// `comment.created`, `card.assignment_changed`, `chat.line.created`. The
    /// segment before the action names the recording type. Used when
    /// ``recordingType`` is empty.
    public var eventType: String?

    /// The recording's own type as BC3 spells it — `Comment`, `Kanban::Card`,
    /// `Chat::Lines::Text`. When set it takes precedence over ``eventType``,
    /// being the more exact of the two.
    public var recordingType: String?

    public init(
        bucketId: Int, recordingId: Int, eventType: String? = nil, recordingType: String? = nil
    ) {
        self.bucketId = bucketId
        self.recordingId = recordingId
        self.eventType = eventType
        self.recordingType = recordingType
    }

    /// The routing key a failure names: the recording type when there is one,
    /// the event type otherwise.
    ///
    /// Trimmed the way Go's `strings.TrimSpace` trims — newlines, carriage
    /// returns and form feeds included, not just the spaces
    /// `CharacterSet.whitespaces` covers. A type read off a line-terminated feed
    /// field arrives as `"Comment\n"`, and a recording type of `"\n"` beside a
    /// usable event type has to read as ABSENT: otherwise a pointer Go resolves
    /// to a summary becomes an unknown-type refusal here.
    ///
    /// Against ``goWhitespace`` and not `CharacterSet.whitespacesAndNewlines`,
    /// which is that set plus U+200B ZERO WIDTH SPACE. The difference is one
    /// scalar and it runs the permissive way: a recording type of `"\u{200B}"`
    /// is a type Go cannot route and this trimmed to nothing, fell through to
    /// the event type, and issued a read.
    var routingKey: String {
        let type = recordingType?.trimmingCharacters(in: goWhitespace) ?? ""
        if !type.isEmpty { return type }
        return eventType?.trimmingCharacters(in: goWhitespace) ?? ""
    }
}

// MARK: - Output

/// The recording this one hangs off — the commented recording for a comment, the
/// Campfire for a chat line, the column for a card.
///
/// Its own type rather than one of the generated parent shapes: the reads route
/// through several of those (`RecordingParent`, `TodoParent`) and a projection
/// that is one recording's view must not change shape with the read that
/// produced it.
public struct RecordingSummaryParent: Codable, Sendable, Equatable {
    public let id: Int
    public let title: String
    public let type: String
    public let url: String
    public let appUrl: String

    public init(id: Int, title: String, type: String, url: String, appUrl: String) {
        self.id = id
        self.title = title
        self.type = type
        self.url = url
        self.appUrl = appUrl
    }
}

/// The project a summarized recording lives in, for the same reason
/// ``RecordingSummaryParent`` is its own type.
public struct RecordingSummaryBucket: Codable, Sendable, Equatable {
    public let id: Int
    public let name: String
    public let type: String

    public init(id: Int, name: String, type: String) {
        self.id = id
        self.name = name
        self.type = type
    }
}

/// The projection ``RecordingsService/summarize(_:)`` returns. Fields a type
/// does not have are absent: a comment has no ``assignees``, a vault no
/// ``content``.
public struct RecordingSummary: Codable, Sendable {
    public let id: Int
    public let status: String
    /// The recording type as BC3 spells it (`Comment`, `Kanban::Card`).
    public let type: String
    public let title: String
    public let appUrl: String
    public let parent: RecordingSummaryParent?
    public let bucket: RecordingSummaryBucket?
    public let creator: Person?
    /// Set for the assignable types (to-dos, cards, card steps).
    public let assignees: [Person]?
    /// The people ``content`` mentions, per ``Mentions/personIds(in:)``. Never
    /// nil, so a JSON consumer reads `[]` rather than a missing key.
    public let mentionedPersonIds: [Int]
    /// The recording's rich text, in full: the comment body, the message body, a
    /// to-do's description, a card's content, the chat line.
    public let content: String
    /// ISO 8601, exactly as the read returned it (SPEC §10 "Date/Time Fields":
    /// this SDK keeps wire strings).
    public let updatedAt: String
    /// The Campfire a chat line was found under — the reply destination for a
    /// chat trigger. Absent for every other type.
    public let campfireId: Int?

    public init(
        id: Int, status: String, type: String, title: String, appUrl: String,
        parent: RecordingSummaryParent? = nil, bucket: RecordingSummaryBucket? = nil,
        creator: Person? = nil, assignees: [Person]? = nil, mentionedPersonIds: [Int] = [],
        content: String = "", updatedAt: String = "", campfireId: Int? = nil
    ) {
        self.id = id
        self.status = status
        self.type = type
        self.title = title
        self.appUrl = appUrl
        self.parent = parent
        self.bucket = bucket
        self.creator = creator
        self.assignees = assignees
        self.mentionedPersonIds = mentionedPersonIds
        self.content = content
        self.updatedAt = updatedAt
        self.campfireId = campfireId
    }
}

// MARK: - Errors

/// A chat line found under no visible Campfire.
public struct UnresolvedRecording: Sendable, Equatable {
    public let bucketId: Int
    public let recordingId: Int
    /// The candidates tried, in order; empty when the bucket has no visible
    /// Campfire at all.
    public let campfireIds: [Int]
    /// Whether the cached discovery sources were re-read before concluding.
    /// False when every source had been read within the last
    /// ``RecordingsService/campfireIndexMinRefresh``, so a Campfire created in
    /// that window was not seen: the conclusion stands on data up to that old,
    /// and a retry after the floor sees the current sources.
    public let refreshed: Bool
    /// Candidates from the cache that the refreshed sources no longer list —
    /// Campfires the caller could see when the cache filled and cannot now. Set
    /// only when ``refreshed``.
    public let staleCampfireIds: [Int]

    public init(
        bucketId: Int, recordingId: Int, campfireIds: [Int], refreshed: Bool,
        staleCampfireIds: [Int] = []
    ) {
        self.bucketId = bucketId
        self.recordingId = recordingId
        self.campfireIds = campfireIds
        self.refreshed = refreshed
        self.staleCampfireIds = staleCampfireIds
    }
}

/// Discovery that could not be carried to a conclusion.
public struct IncompleteCampfireDiscovery: Sendable, Equatable {
    public let bucketId: Int
    public let recordingId: Int
    public let reason: String

    public init(bucketId: Int, recordingId: Int, reason: String) {
        self.bucketId = bucketId
        self.recordingId = recordingId
        self.reason = reason
    }
}

/// What ``RecordingsService/summarize(_:)`` throws that is *not* a read failure.
///
/// These are the composite's own identities, matched by `case`, never by an HTTP
/// status — a caller has to be able to tell "under no Campfire you can see"
/// from "the read failed" and from "candidates were left unsearched", and no
/// ``BasecampError`` case carries that distinction. A failing constituent read
/// still throws its own ``BasecampError``, unchanged.
public enum RecordingSummaryError: Error, Sendable, LocalizedError {
    /// The event type names no recording type — `boost.created`, whose recording
    /// is the boost's target and whose type the feed row does not carry. A
    /// consumer resolves those from its own record of what it posted, not
    /// through `summarize`.
    case noRecordingType(RecordingRef)

    /// Neither the event type nor the recording type names a type in the routing
    /// table. This is a design decision, not a gap: see
    /// ``RecordingsService/summarizableRecordingTypes``.
    case unknownRecordingType(RecordingRef)

    /// The recording the read returned lives in a different bucket from the one
    /// the pointer named. The second value is the bucket the read returned.
    case bucketMismatch(RecordingRef, Int)

    /// A chat line was found under none of the Campfires the caller can
    /// currently see in its bucket. Distinct from a failed read (any non-404
    /// answer is thrown as itself) and from
    /// ``campfireDiscoveryIncomplete(_:)``: every candidate answered 404. It is
    /// not distinct from lost visibility — BC3 answers 404 for a Campfire the
    /// caller may not see, too — so a consumer marks the record blocked and
    /// retries on its own schedule; see
    /// ``UnresolvedRecording/staleCampfireIds``.
    case recordingUnresolved(UnresolvedRecording)

    /// Discovery could not be carried to a conclusion — the Campfire listing
    /// overflowed ``RecordingsService/maxCampfireListing``, or the bucket has
    /// more visible Campfires than ``RecordingsService/maxCampfireCandidates``.
    /// Candidates were left unsearched, so nothing can be reported absent.
    case campfireDiscoveryIncomplete(IncompleteCampfireDiscovery)

    public var message: String {
        switch self {
        case .noRecordingType(let ref):
            "event type names no recording type: \"\(ref.routingKey)\""
        case .unknownRecordingType(let ref):
            "no typed read for recording type: \"\(ref.routingKey)\""
        case .bucketMismatch(let ref, let actual):
            "recording is not in the requested bucket: recording \(ref.recordingId) is in bucket \(actual), not \(ref.bucketId)"
        case .recordingUnresolved(let unresolved):
            "chat line found under no visible campfire: line \(unresolved.recordingId) in bucket \(unresolved.bucketId) (tried \(unresolved.campfireIds.count) campfires)"
        case .campfireDiscoveryIncomplete(let incomplete):
            "campfire discovery incomplete: line \(incomplete.recordingId) in bucket \(incomplete.bucketId): \(incomplete.reason)"
        }
    }

    public var errorDescription: String? { message }

    /// The canonical SPEC §6 code this verdict is CLASSIFIED under.
    ///
    /// The verdict itself stays out of ``BasecampError``: the identity is this
    /// enum, matched by `case`, and §6's taxonomy describes HTTP answers, which
    /// none of these are. But a CLI still has to choose an exit status for one,
    /// and until this existed every consumer chose its own — which is how
    /// `campfire_discovery_incomplete` came to exit 1 from the Kotlin SDK and 7
    /// from Python's for the same condition. Card 40 settled that value as
    /// `usage` and this is where the Swift side of it is written down.
    ///
    /// <https://app.basecamp.com/2914079/buckets/48699913/card_tables/cards/10308122086>
    ///
    /// Every verdict has a row, and each row is one the seven ports agree on —
    /// which is what makes it a shared contract rather than one more port's
    /// taste. ``bucketMismatch(_:_:)`` was the last to be settled, on card 41,
    /// after Rust classified it `not_found` where Python, Ruby, Kotlin and
    /// TypeScript said `usage`.
    ///
    /// <https://app.basecamp.com/2914079/buckets/48699913/card_tables/cards/10308966794>
    public var canonicalCode: String { classification.code }

    /// The CLI exit status ``canonicalCode`` decides. Read from the same table,
    /// never derived from the code a second time: a `default` arm over the code
    /// string would answer the wrong thing for a code the second switch had not
    /// been taught.
    public var exitCode: Int { classification.exit }

    /// The one table both accessors read. Total over the enum, so a verdict
    /// added later must be classified here or the compiler refuses the build.
    private var classification: (code: String, exit: Int) {
        switch self {
        // Refused from the caller's own arguments, before any request.
        case .noRecordingType, .unknownRecordingType: return ("usage", 1)
        // Not `not_found`: the read FOUND the recording, in another bucket, and
        // returned it, so nothing is absent. What failed is the caller's
        // pointer, which named a bucket the recording is not in (card 41).
        case .bucketMismatch: return ("usage", 1)
        // Every visible candidate answered 404: the line is not there.
        case .recordingUnresolved: return ("not_found", 2)
        // `usage` is one of only THREE coarse codes no HTTP response can
        // produce: the status mapping yields auth_required, forbidden,
        // not_found, rate_limit, validation, limit_exceeded and api_error, and
        // network and ambiguous are equally unreachable from a status. `usage`
        // is the one of those three that also describes a call the SDK declined
        // to complete, which is why it and not the other two. Never
        // `not_found`: nothing left unsearched may be reported absent.
        case .campfireDiscoveryIncomplete: return ("usage", 1)
        }
    }
}

// MARK: - Routing

/// Which typed read serves a ``RecordingRef``.
enum RecordingSummaryKind: Sendable {
    case comment, message, todo, card, chatLine, document, upload, scheduleEntry, question
    case questionAnswer, todolist, vault, forward, clientApproval, clientCorrespondence
    case googleDocument, cloudFile, cardStep, questionnaire, schedule, todoset, messageBoard
    case cardTable, cardColumn, inbox, campfire
}

extension RecordingsService {
    /// How long a cached discovery source — a bucket's project dock, the
    /// account's Campfire listing — is reused before it is read again.
    public static let campfireIndexTTL: TimeInterval = 10 * 60

    /// Bounds the refresh-on-miss: a line found under no candidate re-reads the
    /// cached sources, but not more often than this per source, so a run of
    /// unresolvable lines cannot turn into a listing per line.
    /// ``UnresolvedRecording/refreshed`` says whether the floor applied.
    ///
    /// It also makes the budget half of the pass-2 dock gate untestable from
    /// outside. Two calls in one test are milliseconds apart, so the floor
    /// declines the re-read whatever the budget says, and reverting
    /// `budgetRemaining > 0` leaves the suite green. Reaching that half needs a
    /// cached dock older than this floor, which needs a clock seam
    /// ``BasecampClient`` does not expose — a deliberate gap, recorded here
    /// rather than papered over with a test that passes for the other reason.
    static let campfireIndexMinRefresh: TimeInterval = 30

    /// Bounds how many Campfires one ``summarize(_:)`` call tries, across both
    /// sources and the refresh. A project has one Campfire and a handful of
    /// pings; a bucket past this bound is not a shape BC3 produces, and the call
    /// reports incomplete discovery rather than calling the rest absent.
    public static let maxCampfireCandidates = 50

    /// Caps the account-wide Campfire listing the fallback source reads. A
    /// listing that overflows it is not cached and the call reports incomplete
    /// discovery: the dock covers every project, so the listing only ever serves
    /// the leftover, and an account with more Campfires than this should not pay
    /// a full walk per TTL for it.
    public static let maxCampfireListing = 1000

    /// The prefix every chat-line subtype shares (`Chat::Lines::Text`,
    /// `::RichText`, `::Code`, `::Upload`, `::Integration`), all of which read
    /// through the same route.
    public static let chatLineTypePrefix = "Chat::Lines::"

    /// Maps the subject of an account event feed type — everything before its
    /// final `.` — to a read. This is the feed's catalog (bc3
    /// `Event::EventType`) minus `boost`, which names no recording type and is
    /// refused explicitly rather than left to fall through as unknown.
    static let summarizableEventSubjects: [String: RecordingSummaryKind] = [
        "comment": .comment,
        "message": .message,
        "todo": .todo,
        "card": .card,
        "chat.line": .chatLine,
    ]

    /// Maps BC3's recording type strings to a read. It is the routing contract,
    /// and it is a DELIBERATE set, not an exhaustive one: the recording types the
    /// account event feed's trigger matrix names (comment, message, to-do, card,
    /// chat line), plus the content and tool recordings a consumer reasoning
    /// about those is likely to hold an id for. Chat lines are matched by prefix;
    /// everything else exactly.
    ///
    /// A type outside this set is ``RecordingSummaryError/unknownRecordingType(_:)``
    /// by design, whether or not the SDK has an id-only read for it —
    /// `Timesheet::Entry` and `Gauge::Needle` do, and are not routed;
    /// `Client::Reply` and `Forward::Reply` cannot be, since their reads need a
    /// parent id the pointer does not carry. Widening the set is a product
    /// decision, not a gap: add the type here, its projection in
    /// `readSummary(_:_:)`, a routing row in the native test, and a case in
    /// `conformance/tests/recording_summary.json`, the fixture every port
    /// implements.
    static let summarizableTypes: [String: RecordingSummaryKind] = [
        "Comment": .comment,
        "Message": .message,
        "Todo": .todo,
        "Kanban::Card": .card,
        "Document": .document,
        "Upload": .upload,
        "Schedule::Entry": .scheduleEntry,
        "Question": .question,
        "Question::Answer": .questionAnswer,
        "Todolist": .todolist,
        "Vault": .vault,
        "Inbox::Forward": .forward,
        "Client::Approval": .clientApproval,
        "Client::Correspondence": .clientCorrespondence,
        "GoogleDocument": .googleDocument,
        "CloudFile": .cloudFile,
        "Kanban::Step": .cardStep,
        "Questionnaire": .questionnaire,
        "Schedule": .schedule,
        "Todoset": .todoset,
        "Message::Board": .messageBoard,
        "Kanban::Board": .cardTable,
        "Kanban::Column": .cardColumn,
        "Inbox": .inbox,
        "Chat::Transcript": .campfire,
    ]

    /// The recording types ``summarize(_:)`` routes by
    /// ``RecordingRef/recordingType``, sorted, with the `Chat::Lines` subtypes
    /// represented by their shared prefix (`Chat::Lines::*`). The set is
    /// deliberate rather than exhaustive — see ``summarizableTypes`` — and any
    /// other type is ``RecordingSummaryError/unknownRecordingType(_:)`` by
    /// design.
    public static var summarizableRecordingTypes: [String] {
        (summarizableTypes.keys.map { $0 } + ["\(chatLineTypePrefix)*"]).sorted()
    }

    /// The account event feed subjects ``summarize(_:)`` routes by
    /// ``RecordingRef/eventType`` — an event type is `<subject>.<action>`, and
    /// any action on a listed subject routes to that subject's read — sorted.
    /// `boost` is absent on purpose:
    /// ``RecordingSummaryError/noRecordingType(_:)``.
    public static var summarizableEventTypes: [String] {
        summarizableEventSubjects.keys.map { "\($0).*" }.sorted()
    }

    /// Picks the read for a ref. ``RecordingRef/recordingType`` wins when set.
    ///
    /// Every comparison here is over UTF-8 BYTES, because Go's are. `hasPrefix`
    /// and `lastIndex(of:)` walk grapheme clusters, so
    /// `"Chat::Lines::\u{0301}RichText"` hid the prefix from one and
    /// `"comment.\u{0301}created"` hid the dot from the other — a pointer Go
    /// routes, refused here — while the trim above went the other way.
    static func route(_ ref: RecordingRef) throws -> RecordingSummaryKind {
        let type = ref.recordingType?.trimmingCharacters(in: goWhitespace) ?? ""
        if !type.isEmpty {
            if Array(type.utf8).starts(with: Array(chatLineTypePrefix.utf8)) { return .chatLine }
            guard let kind = byteExactLookup(summarizableTypes, type) else {
                throw RecordingSummaryError.unknownRecordingType(ref)
            }
            return kind
        }

        let eventType = ref.eventType?.trimmingCharacters(in: goWhitespace) ?? ""
        guard !eventType.isEmpty else { throw RecordingSummaryError.unknownRecordingType(ref) }
        // A feed type is "<subject>.<action>"; the subject names the recording
        // type. A string with no action is not a feed type and is not routed.
        let bytes = Array(eventType.utf8)
        guard let dot = bytes.lastIndex(of: UInt8(ascii: ".")), dot != 0, dot != bytes.count - 1
        else {
            throw RecordingSummaryError.unknownRecordingType(ref)
        }
        let subject = String(decoding: bytes[..<dot], as: UTF8.self)
        if bytes[..<dot].elementsEqual("boost".utf8) {
            throw RecordingSummaryError.noRecordingType(ref)
        }
        guard let kind = byteExactLookup(summarizableEventSubjects, subject) else {
            throw RecordingSummaryError.unknownRecordingType(ref)
        }
        return kind
    }

    /// A dictionary lookup that compares BYTES, as Go's map does.
    ///
    /// Swift `String` keys — and `String` equality — compare by canonical
    /// equivalence, and exactly three scalars decompose to pure ASCII: U+037E
    /// GREEK QUESTION MARK is `;`, U+1FEF GREEK VARIA is a backtick, and U+212A
    /// KELVIN SIGN is `K`. One of them lands here: `"\u{212A}anban::Card"` is a
    /// recording type Go refuses before any request, and it matched
    /// `"Kanban::Card"` and issued a read. The byte comparison is O(n) over
    /// tables of twenty-five and five rows, which is the wrong thing to optimise
    /// against being wrong.
    ///
    /// Applied to BOTH tables, though only one has a key an alias can reach:
    /// none of `comment`, `message`, `todo`, `card` or `chat.line` contains a
    /// `K`, a `;` or a backtick. That argument is correct and it is exactly the
    /// shape of argument that has been wrong twice on this branch — an
    /// equivalence proved over one operation and inherited by another — so the
    /// second table gets the same comparison rather than the same reasoning. A
    /// mutation that reverts it cannot be killed by a test, and that is the
    /// reason to apply it rather than a reason not to.
    private static func byteExactLookup<V>(_ table: [String: V], _ key: String) -> V? {
        guard table[key] != nil else { return nil }
        return table.first(where: { $0.key.utf8.elementsEqual(key.utf8) })?.value
    }
}

// MARK: - Summarize

extension RecordingsService {
    /// Resolves a recording pointer into a ``RecordingSummary`` through the typed
    /// read its type names. See ``RecordingRef`` for the routing inputs and the
    /// file comment for the design.
    ///
    /// - Throws: ``RecordingSummaryError/noRecordingType(_:)`` or
    ///   ``RecordingSummaryError/unknownRecordingType(_:)`` before any request;
    ///   the read's own ``BasecampError`` otherwise — a 404 is `.notFound`, as
    ///   from the typed read itself; for chat lines,
    ///   ``RecordingSummaryError/recordingUnresolved(_:)`` when every visible
    ///   Campfire answered 404, which is distinct from a read that failed (any
    ///   non-404 from a candidate is thrown as that error, and the loop stops
    ///   there) and from
    ///   ``RecordingSummaryError/campfireDiscoveryIncomplete(_:)`` (candidates
    ///   were left unsearched); ``RecordingSummaryError/bucketMismatch(_:_:)``
    ///   when the read returned a recording from another bucket.
    public func summarize(_ ref: RecordingRef) async throws -> RecordingSummary {
        guard ref.bucketId > 0, ref.recordingId > 0 else {
            throw BasecampError.usage(
                message: "bucket id and recording id are required", hint: nil)
        }
        let summary = try await readSummary(ref, Self.route(ref))
        if let bucket = summary.bucket, bucket.id != 0, bucket.id != ref.bucketId {
            throw RecordingSummaryError.bucketMismatch(ref, bucket.id)
        }
        return summary
    }

    /// Performs the one typed read a kind names and projects it.
    ///
    /// Every arm is one generated wire method under its own hook identity — no
    /// path is built here, and no verb is chosen (§18 rules 1 and 3).
    private func readSummary(_ ref: RecordingRef, _ kind: RecordingSummaryKind) async throws
        -> RecordingSummary
    {
        let account = accountClient
        let id = ref.recordingId

        switch kind {
        case .comment:
            let comment = try await account.comments.get(commentId: id)
            return project(
                id: comment.id, status: comment.status, type: comment.type, title: comment.title,
                appUrl: comment.appUrl, parent: RecordingSummaryParent(comment.parent), bucket: RecordingSummaryBucket(comment.bucket),
                creator: comment.creator, content: comment.content, updatedAt: comment.updatedAt)

        case .message:
            let message = try await account.messages.get(messageId: id)
            return project(
                id: message.id, status: message.status, type: message.type,
                title: firstNonEmpty(message.title, message.subject), appUrl: message.appUrl,
                parent: RecordingSummaryParent(message.parent), bucket: RecordingSummaryBucket(message.bucket),
                creator: message.creator, content: message.content, updatedAt: message.updatedAt)

        case .todo:
            // A to-do's content is its plain title; the rich text — where
            // mentions live — is the description.
            let todo = try await account.todos.get(todoId: id)
            return project(
                id: todo.id, status: todo.status, type: todo.type,
                title: firstNonEmpty(todo.title, todo.content), appUrl: todo.appUrl,
                parent: RecordingSummaryParent(todo.parent), bucket: RecordingSummaryBucket(todo.bucket), creator: todo.creator,
                assignees: todo.assignees, content: todo.description ?? "",
                updatedAt: todo.updatedAt)

        case .card:
            let card = try await account.cards.get(cardId: id)
            return project(
                id: card.id, status: card.status, type: card.type, title: card.title,
                appUrl: card.appUrl, parent: RecordingSummaryParent(card.parent), bucket: RecordingSummaryBucket(card.bucket),
                creator: card.creator, assignees: card.assignees,
                content: firstNonEmpty(card.content, card.description), updatedAt: card.updatedAt)

        case .chatLine:
            let (line, campfireId) = try await resolveChatLine(
                bucketId: ref.bucketId, lineId: id)
            // A plain-text or code line's content is text BC3 never read as
            // markup, so a literal "<bc-attachment>" in it mentions nobody.
            let content = line.content ?? ""
            return project(
                id: line.id, status: line.status, type: line.type, title: line.title,
                appUrl: line.appUrl, parent: RecordingSummaryParent(line.parent), bucket: RecordingSummaryBucket(line.bucket),
                creator: line.creator,
                mentionedPersonIds: Self.chatLineCarriesRichText(line.type)
                    ? Mentions.personIds(in: content) : [],
                content: content, updatedAt: line.updatedAt, campfireId: campfireId)

        case .document:
            let document = try await account.documents.get(documentId: id)
            return project(
                id: document.id, status: document.status, type: document.type,
                title: document.title, appUrl: document.appUrl, parent: RecordingSummaryParent(document.parent),
                bucket: RecordingSummaryBucket(document.bucket), creator: document.creator,
                content: document.content ?? "", updatedAt: document.updatedAt)

        case .upload:
            let upload = try await account.uploads.get(uploadId: id)
            return project(
                id: upload.id, status: upload.status, type: upload.type,
                title: firstNonEmpty(upload.title, upload.filename), appUrl: upload.appUrl,
                parent: RecordingSummaryParent(upload.parent), bucket: RecordingSummaryBucket(upload.bucket),
                creator: upload.creator, content: upload.description ?? "",
                updatedAt: upload.updatedAt)

        case .scheduleEntry:
            let entry = try await account.schedules.getEntry(entryId: id)
            return project(
                id: entry.id, status: entry.status, type: entry.type,
                title: firstNonEmpty(entry.title, entry.summary), appUrl: entry.appUrl,
                parent: RecordingSummaryParent(entry.parent), bucket: RecordingSummaryBucket(entry.bucket), creator: entry.creator,
                content: entry.description ?? "", updatedAt: entry.updatedAt)

        case .question:
            let question = try await account.checkins.getQuestion(questionId: id)
            return project(
                id: question.id, status: question.status, type: question.type,
                title: question.title, appUrl: question.appUrl, parent: RecordingSummaryParent(question.parent),
                bucket: RecordingSummaryBucket(question.bucket), creator: question.creator, content: "",
                updatedAt: question.updatedAt)

        case .questionAnswer:
            let answer = try await account.checkins.getAnswer(answerId: id)
            return project(
                id: answer.id, status: answer.status, type: answer.type, title: answer.title,
                appUrl: answer.appUrl, parent: RecordingSummaryParent(answer.parent), bucket: RecordingSummaryBucket(answer.bucket),
                creator: answer.creator, content: answer.content, updatedAt: answer.updatedAt)

        case .todolist:
            let todolist = try await account.todolists.get(id: id)
            return project(
                id: todolist.id, status: todolist.status, type: todolist.type,
                title: firstNonEmpty(todolist.title, todolist.name), appUrl: todolist.appUrl,
                parent: RecordingSummaryParent(todolist.parent), bucket: RecordingSummaryBucket(todolist.bucket),
                creator: todolist.creator, content: todolist.description,
                updatedAt: todolist.updatedAt)

        case .vault:
            let vault = try await account.vaults.get(vaultId: id)
            return project(
                id: vault.id, status: vault.status, type: vault.type, title: vault.title,
                appUrl: vault.appUrl, parent: vault.parent.flatMap { RecordingSummaryParent($0) }, bucket: RecordingSummaryBucket(vault.bucket),
                creator: vault.creator, content: "", updatedAt: vault.updatedAt)

        case .forward:
            let forward = try await account.forwards.get(forwardId: id)
            return project(
                id: forward.id, status: forward.status, type: forward.type,
                title: firstNonEmpty(forward.title, forward.subject), appUrl: forward.appUrl,
                parent: RecordingSummaryParent(forward.parent), bucket: RecordingSummaryBucket(forward.bucket),
                creator: forward.creator, content: forward.content ?? "",
                updatedAt: forward.updatedAt)

        case .clientApproval:
            let approval = try await account.clientApprovals.get(approvalId: id)
            return project(
                id: approval.id, status: approval.status, type: approval.type,
                title: firstNonEmpty(approval.title, approval.subject), appUrl: approval.appUrl,
                parent: RecordingSummaryParent(approval.parent), bucket: RecordingSummaryBucket(approval.bucket),
                creator: approval.creator, content: approval.content ?? "",
                updatedAt: approval.updatedAt)

        case .clientCorrespondence:
            let correspondence = try await account.clientCorrespondences.get(correspondenceId: id)
            return project(
                id: correspondence.id, status: correspondence.status, type: correspondence.type,
                title: firstNonEmpty(correspondence.title, correspondence.subject),
                appUrl: correspondence.appUrl, parent: RecordingSummaryParent(correspondence.parent),
                bucket: RecordingSummaryBucket(correspondence.bucket), creator: correspondence.creator,
                content: correspondence.content ?? "", updatedAt: correspondence.updatedAt)

        case .googleDocument:
            let document = try await account.googleDocuments.googleDocument(googleDocumentId: id)
            return project(
                id: document.id, status: document.status, type: document.type,
                title: document.title, appUrl: document.appUrl, parent: RecordingSummaryParent(document.parent),
                bucket: RecordingSummaryBucket(document.bucket), creator: document.creator,
                content: document.description ?? "", updatedAt: document.updatedAt)

        case .cloudFile:
            let file = try await account.cloudFiles.cloudFile(cloudFileId: id)
            return project(
                id: file.id, status: file.status, type: file.type, title: file.title,
                appUrl: file.appUrl, parent: RecordingSummaryParent(file.parent), bucket: RecordingSummaryBucket(file.bucket),
                creator: file.creator, content: file.description ?? "", updatedAt: file.updatedAt)

        case .cardStep:
            let step = try await account.cardSteps.get(stepId: id)
            return project(
                id: step.id, status: step.status, type: step.type, title: step.title,
                appUrl: step.appUrl, parent: RecordingSummaryParent(step.parent), bucket: RecordingSummaryBucket(step.bucket),
                creator: step.creator, assignees: step.assignees, content: "",
                updatedAt: step.updatedAt)

        case .questionnaire:
            let questionnaire = try await account.checkins.getQuestionnaire(questionnaireId: id)
            return project(
                id: questionnaire.id, status: questionnaire.status, type: questionnaire.type,
                title: firstNonEmpty(questionnaire.title, questionnaire.name),
                appUrl: questionnaire.appUrl, parent: nil, bucket: RecordingSummaryBucket(questionnaire.bucket),
                creator: questionnaire.creator, content: "", updatedAt: questionnaire.updatedAt)

        case .schedule:
            let schedule = try await account.schedules.get(scheduleId: id)
            return project(
                id: schedule.id, status: schedule.status, type: schedule.type,
                title: schedule.title, appUrl: schedule.appUrl, parent: nil,
                bucket: RecordingSummaryBucket(schedule.bucket), creator: schedule.creator, content: "",
                updatedAt: schedule.updatedAt)

        case .todoset:
            let todoset = try await account.todosets.get(todosetId: id)
            return project(
                id: todoset.id, status: todoset.status, type: todoset.type,
                title: firstNonEmpty(todoset.title, todoset.name), appUrl: todoset.appUrl,
                parent: nil, bucket: RecordingSummaryBucket(todoset.bucket), creator: todoset.creator, content: "",
                updatedAt: todoset.updatedAt)

        case .messageBoard:
            let board = try await account.messageBoards.get(boardId: id)
            return project(
                id: board.id, status: board.status, type: board.type, title: board.title,
                appUrl: board.appUrl, parent: nil, bucket: RecordingSummaryBucket(board.bucket),
                creator: board.creator, content: "", updatedAt: board.updatedAt)

        case .cardTable:
            let table = try await account.cardTables.get(cardTableId: id)
            return project(
                id: table.id, status: table.status, type: table.type, title: table.title,
                appUrl: table.appUrl, parent: nil, bucket: RecordingSummaryBucket(table.bucket),
                creator: table.creator, content: "", updatedAt: table.updatedAt)

        case .cardColumn:
            let column = try await account.cardColumns.get(columnId: id)
            return project(
                id: column.id, status: column.status, type: column.type, title: column.title,
                appUrl: column.appUrl, parent: RecordingSummaryParent(column.parent), bucket: RecordingSummaryBucket(column.bucket),
                creator: column.creator, content: column.description ?? "",
                updatedAt: column.updatedAt)

        case .inbox:
            let inbox = try await account.forwards.getInbox(inboxId: id)
            return project(
                id: inbox.id, status: inbox.status, type: inbox.type, title: inbox.title,
                appUrl: inbox.appUrl, parent: nil, bucket: RecordingSummaryBucket(inbox.bucket),
                creator: inbox.creator, content: "", updatedAt: inbox.updatedAt)

        case .campfire:
            let campfire = try await account.campfires.get(campfireId: id)
            return project(
                id: campfire.id, status: campfire.status, type: campfire.type,
                title: campfire.title, appUrl: campfire.appUrl, parent: nil,
                bucket: RecordingSummaryBucket(campfire.bucket), creator: campfire.creator, content: "",
                updatedAt: campfire.updatedAt)
        }
    }

    /// Whether a chat line subtype carries rich text — the two that declare
    /// `rich_text_attribute :content` in BC3, and so the only two whose content
    /// can hold a mention. A `Text` line's content is HTML-escaped on the way out
    /// (`content_helper.rb`, `format_chat_line_with`), a `Code` line's is served
    /// verbatim — a snippet that happens to contain a `bc-attachment` tag — and
    /// an `Upload` line has no content.
    static func chatLineCarriesRichText(_ lineType: String) -> Bool {
        lineType == "Chat::Lines::RichText" || lineType == "Chat::Lines::Integration"
    }
}

// MARK: - Chat-line discovery

extension RecordingsService {
    /// Finds the Campfire a line lives in and reads it.
    ///
    /// Two failure shapes are kept apart on purpose. A candidate that answers
    /// anything but 404 — 401, 403, 5xx, a network error — stops the loop and is
    /// thrown as that error: the read failed, and trying the next Campfire would
    /// only hide it. A 404 means "not here", so the loop moves on. Only when
    /// every candidate said "not here" is the line unresolved — and before
    /// concluding that, the cached sources are refreshed (subject to the floor)
    /// so a Campfire created after the cache filled is tried too. Discovery that
    /// could not be completed — a listing cut off at its cap, a bucket with more
    /// candidates than the budget — is incomplete, never "unresolved": nothing
    /// unsearched is ever reported absent.
    ///
    /// What HTTP cannot tell apart: BC3 answers 404 both for a line that is not
    /// in a Campfire and for a Campfire the caller may no longer see.
    /// "Unresolved" therefore means "under no Campfire the caller can currently
    /// see", and the error reports the cached candidates that the refreshed
    /// sources no longer list, so a consumer can see when visibility, not
    /// existence, is what changed.
    private func resolveChatLine(bucketId: Int, lineId: Int) async throws -> (CampfireLine, Int) {
        let account = accountClient
        let index = account.client.campfireIndex
        let search = ChatLineSearch(
            service: self, lineId: lineId, budget: Self.maxCampfireCandidates)

        func incomplete(_ reason: String) -> RecordingSummaryError {
            .campfireDiscoveryIncomplete(
                IncompleteCampfireDiscovery(
                    bucketId: bucketId, recordingId: lineId, reason: reason))
        }
        let tooManyCandidates =
            "more than \(Self.maxCampfireCandidates) visible campfires in the bucket"

        // Pass 1: what the sources already hold — the dock (read if it must be),
        // then the listing only if it is cached. A listing fetch is the
        // expensive, slow request, and it is not made until the dock — including
        // its refresh — has had its say, so a listing that is down, over its cap,
        // or stalled never stands between a project's line and the one project
        // read that finds it.
        var dock = try await index.dockCampfires(
            account: account, bucketId: bucketId, refresh: false)
        if let found = try await search.read(dock.ids) { return found }

        let cachedListing = await index.cachedListedCampfires(
            accountId: account.accountId, bucketId: bucketId)
        let listingWasCached = cachedListing != nil
        if let cachedListing, let found = try await search.read(cachedListing.ids) {
            return found
        }

        // Pass 2: re-read the dock if it was served from cache (the floor may
        // decline), then fetch or refresh the listing. Whatever comes back is the
        // current snapshot of that source, whoever loaded it — another caller may
        // have populated or refreshed it in the meantime — so it always replaces
        // the pass-1 one; "refreshed" is whether a source the conclusion had
        // consulted is now newer than when it was consulted.
        //
        // Not when the budget is already spent: a re-read could return no
        // candidate this call may try, so it would cost a request that cannot
        // help — and a failure on it would replace the deterministic "incomplete"
        // verdict with a transient error a consumer retries forever.
        // No budget left means no re-read: a source already consulted cannot
        // hand this call a candidate it may try, so its refresh is skipped and
        // the conclusion stands on what was seen (`refreshed: false`). A source
        // never consulted is different — candidates may exist there unsearched —
        // so running out of budget before it makes the verdict incomplete.
        //
        // The budget is what this keys on, not `skipped`. `skipped` is set only
        // when a candidate is OBSERVED and cannot be tried, so a dock holding
        // exactly `maxCampfireCandidates` entries that all answer 404 leaves the
        // budget at zero with `skipped` still false — and the old shape then
        // fetched the account listing, a request that cannot help, where a
        // failure would replace a settled "incomplete" with a transient error a
        // consumer retries forever.
        var refreshed = false
        var listed = cachedListing.map(\.ids) ?? []

        if await search.budgetRemaining > 0, dock.cached {
            let again = try await index.dockCampfires(
                account: account, bucketId: bucketId, refresh: true)
            if again.fetchedAt > dock.fetchedAt || !again.cached { refreshed = true }
            dock = again
            if let found = try await search.read(dock.ids) { return found }
        }

        if await search.budgetRemaining <= 0 {
            if !listingWasCached {
                throw incomplete(
                    "the candidate budget of \(Self.maxCampfireCandidates) was spent before the account listing was consulted"
                )
            }
        } else {
            let againListed: CampfireIndex.SourceRead
            do {
                againListed = try await index.listedCampfires(
                    account: account, bucketId: bucketId, refresh: listingWasCached)
            } catch let overflow as CampfireListingOverflow {
                throw incomplete(overflow.description)
            }
            if let previous = cachedListing,
                againListed.fetchedAt > previous.fetchedAt || !againListed.cached
            {
                refreshed = true
            }
            listed = againListed.ids
            if let found = try await search.read(againListed.ids) { return found }
        }

        try Task.checkCancellation()
        if await search.skipped { throw incomplete(tooManyCandidates) }

        let tried = await search.tried
        let stale =
            refreshed
            ? tried.filter { !dock.ids.contains($0) && !listed.contains($0) }
            : []
        throw RecordingSummaryError.recordingUnresolved(
            UnresolvedRecording(
                bucketId: bucketId, recordingId: lineId, campfireIds: tried, refreshed: refreshed,
                staleCampfireIds: stale))
    }
}

/// One `summarize` call's discovery state: which candidates have answered 404,
/// how many more this call may try, and whether any were left untried for want
/// of budget.
private actor ChatLineSearch {
    private let service: RecordingsService
    private let lineId: Int
    /// Candidates that answered 404, in order.
    private(set) var tried: [Int] = []
    private var budget: Int
    /// A candidate was left untried for want of budget. Set only when one is
    /// OBSERVED and cannot be tried — which is why the pass-2 gating reads
    /// ``budgetRemaining`` instead: a source whose every candidate was tried
    /// leaves the budget spent and this false.
    private(set) var skipped = false

    /// Candidates this call may still try.
    var budgetRemaining: Int { budget }

    init(service: RecordingsService, lineId: Int, budget: Int) {
        self.service = service
        self.lineId = lineId
        self.budget = budget
    }

    /// Reads the line under each candidate not yet tried. Returns the line and
    /// its Campfire on a hit, nil on a miss (recording the candidates in
    /// ``tried``). Any answer but 404 is thrown as it stands.
    func read(_ candidates: [Int]) async throws -> (CampfireLine, Int)? {
        for campfireId in candidates {
            if tried.contains(campfireId) { continue }
            if budget <= 0 {
                skipped = true
                return nil
            }
            try Task.checkCancellation()
            budget -= 1
            do {
                let line = try await service.accountClient.campfires.getLine(
                    campfireId: campfireId, lineId: lineId)
                return (line, campfireId)
            } catch let error as BasecampError {
                guard case .notFound = error else { throw error }
                tried.append(campfireId)
            }
        }
        return nil
    }
}

// MARK: - Projection helpers

extension RecordingsService {
    /// Builds the projection, reading the mentions out of `content` unless the
    /// caller already decided them (the chat-line arm, whose plain-text subtypes
    /// mention nobody whatever their content looks like).
    private func project(
        id: Int, status: String, type: String, title: String, appUrl: String,
        parent: RecordingSummaryParent?, bucket: RecordingSummaryBucket?, creator: Person?,
        assignees: [Person]? = nil, mentionedPersonIds: [Int]? = nil, content: String,
        updatedAt: String, campfireId: Int? = nil
    ) -> RecordingSummary {
        RecordingSummary(
            id: id, status: status, type: type, title: title, appUrl: appUrl, parent: parent,
            bucket: bucket, creator: creator,
            // An empty assignee list is absent, not present-and-empty: Go's
            // `omitempty` drops it, and the projection must not depend on which
            // SDK rendered it.
            assignees: (assignees?.isEmpty ?? true) ? nil : assignees,
            mentionedPersonIds: mentionedPersonIds ?? Mentions.personIds(in: content),
            content: content, updatedAt: updatedAt, campfireId: campfireId)
    }
}

/// The first value that is neither nil nor empty, or "".
private func firstNonEmpty(_ values: String?...) -> String {
    for value in values {
        if let value, !value.isEmpty { return value }
    }
    return ""
}

// An all-zero nested identity is ABSENT, not an identity whose every field
// happens to be empty. Go's model conversions build `*Parent`/`*Bucket` only
// when the payload carried an id or a name, so the projection omits the key
// rather than emitting `{"id":0,"title":"","type":"",…}`, and a consumer reading
// the projection from any SDK has to see the same thing.
extension RecordingSummaryParent {
    init?(_ parent: RecordingParent) {
        guard parent.id != 0 || !parent.title.isEmpty else { return nil }
        self.init(
            id: parent.id, title: parent.title, type: parent.type, url: parent.url,
            appUrl: parent.appUrl)
    }

    init?(_ parent: TodoParent) {
        guard parent.id != 0 || !parent.title.isEmpty else { return nil }
        self.init(
            id: parent.id, title: parent.title, type: parent.type, url: parent.url,
            appUrl: parent.appUrl)
    }
}

extension RecordingSummaryBucket {
    init?(_ bucket: TodoBucket) {
        guard bucket.id != 0 || !bucket.name.isEmpty else { return nil }
        self.init(id: bucket.id, name: bucket.name, type: bucket.type)
    }

    init?(_ bucket: RecordingBucket) {
        guard bucket.id != 0 || !bucket.name.isEmpty else { return nil }
        self.init(id: bucket.id, name: bucket.name, type: bucket.type)
    }
}
