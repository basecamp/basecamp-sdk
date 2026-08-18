// Hand-written merge-safe update / edit surface for SchedulesService.
import Foundation

/// Request parameters for the merge-safe
/// ``SchedulesService/updateEntry(entryId:req:)``.
///
/// Every field is optional, and `nil` means "not addressed":
///
/// - A `nil` **full-state** field (`summary`, `startsAt`, `endsAt`,
///   `description`, `allDay`) is left untouched on the entry, guaranteed — the
///   composite resends the value it read back.
/// - A `nil` **carve-out** (`participantIds`, `url`, `highlighted`, `notify`)
///   stays off the wire entirely, so BC3 preserves what it is already holding.
///
/// An explicitly-passed empty value is an address, not an absence:
/// `participantIds: []` removes every participant, `url: ""` drops the join
/// link and `highlighted: false` stops highlighting. Swift's synthesized
/// `Codable` encodes optionals with `encodeIfPresent`, so `nil` is omitted
/// while `[]`, `""` and `false` are all non-`nil` and reach the body.
public struct UpdateScheduleEntryRequest: Codable, Sendable {
    /// All-day flag. Full state: resent from the read-back when `nil`.
    public var allDay: Bool?
    /// Rich text description (HTML). Full state: resent when `nil`; `""` clears.
    public var description: String?
    /// End of the entry, in the API's own spelling — a bare date
    /// (`"2026-06-05"`) for an all-day entry, a timestamp otherwise. Full
    /// state: resent verbatim when `nil`.
    public var endsAt: String?
    /// Highlight flag. Carve-out: omitted when `nil`, `false` clears.
    public var highlighted: Bool?
    /// Send directive, not entry state — asks BC3 to recompute a drafted
    /// entry's subscriber list. Carve-out: omitted when `nil`.
    public var notify: Bool?
    /// Complete list of participant person IDs. Carve-out: omitted when `nil`,
    /// `[]` removes everyone.
    public var participantIds: [Int]?
    /// Start of the entry, same spelling rule as ``endsAt``. Full state.
    public var startsAt: String?
    /// One-line summary. Full state: resent when `nil`.
    public var summary: String?
    /// Join link (video call and the like). Carve-out: omitted when `nil`,
    /// `""` drops the link. Note the response spells this `join_url`; the
    /// request spells it `url`.
    public var url: String?

    public init(
        allDay: Bool? = nil,
        description: String? = nil,
        endsAt: String? = nil,
        highlighted: Bool? = nil,
        notify: Bool? = nil,
        participantIds: [Int]? = nil,
        startsAt: String? = nil,
        summary: String? = nil,
        url: String? = nil
    ) {
        self.allDay = allDay
        self.description = description
        self.endsAt = endsAt
        self.highlighted = highlighted
        self.notify = notify
        self.participantIds = participantIds
        self.startsAt = startsAt
        self.summary = summary
        self.url = url
    }
}

/// A schedule entry's full writable state, handed to the
/// ``SchedulesService/editEntry(entryId:_:)`` closure.
///
/// The writable set splits in two, and the split is the whole point:
///
/// - **Full state** — `summary`, `startsAt`, `endsAt`, `description`,
///   `allDay` — is seeded from the read-back and **always** PUT, empties
///   included. On a full-replace endpoint `""` is how a clear is expressed;
///   there is no third state.
/// - **Carve-outs** — `participantIds`, `url`, `highlighted`, `notify` — are
///   seeded so the closure can *read* them, but reach the wire only when the
///   closure *assigns* one. BC3 preserves each of the first three when the
///   request does not address it (`PRESERVED_ON_OMISSION`), so resending is
///   redundant at best and wrong if the read raced a concurrent change.
///
/// Dirty tracking is by **setter invocation**, not by comparing against the
/// read-back: `fields.url = fields.url` is a write, and this type sends it.
/// Property observers do not run during initialization, so seeding in `init`
/// leaves every carve-out clean.
public struct ScheduleEntryFields: Sendable {
    /// One-line summary. Set `""`… don't: BC3 renders `"Untitled"` for a blank
    /// summary, so a blank read-back is refused before this value exists.
    public var summary: String
    /// Start of the entry, in the API's own spelling — a bare date
    /// (`"2026-06-05"`) when ``allDay`` is set, a timestamp otherwise. Carried
    /// verbatim: reformatting it would rewrite an all-day entry's bounds.
    public var startsAt: String
    /// End of the entry, same spelling rule as ``startsAt``.
    public var endsAt: String
    /// Rich text description (HTML). Set `""` to clear.
    public var description: String
    /// All-day flag. Flipping it means restating ``startsAt``/``endsAt`` in the
    /// matching spelling — a bare date for all-day, a timestamp otherwise.
    public var allDay: Bool

    /// Complete list of participant person IDs, seeded from the read-back's
    /// `participants`. Carve-out: assign to send (`[]` removes everyone),
    /// leave alone to let BC3 keep the current list.
    public var participantIds: [Int] { didSet { addressedParticipantIds = true } }
    /// Join link, seeded from the read-back's `join_url` — NOT its `url`,
    /// which is the entry's own Basecamp API URL. Carve-out: assign to send
    /// (`""` drops the link), leave alone to preserve.
    public var url: String { didSet { addressedURL = true } }
    /// Highlight flag, seeded from the read-back. Carve-out: assign to send
    /// (`false` stops highlighting), leave alone to preserve.
    public var highlighted: Bool { didSet { addressedHighlighted = true } }
    /// Send directive, not entry state: never seeded from the entry, and sent
    /// only when the closure assigns it. Asks BC3 to recompute a drafted
    /// entry's subscriber list.
    public var notify: Bool { didSet { addressedNotify = true } }

    /// Setter-invocation dirty bits. `private(set)` so only the observers above
    /// can raise one — a caller cannot fake an address, and the composite
    /// cannot mistake a seeded value for one.
    public private(set) var addressedParticipantIds = false
    public private(set) var addressedURL = false
    public private(set) var addressedHighlighted = false
    public private(set) var addressedNotify = false

    /// Seeds the writable state from a read-back entry.
    ///
    /// Swift's decoder is the typed guard the dynamic SDKs write by hand:
    /// `summary`, `startsAt`, `endsAt` and `allDay` are non-optional on
    /// ``ScheduleEntry``, so an absent, null or wrong-typed one is already a
    /// `DecodingError` before this initializer runs. What `Codable` cannot
    /// check is *blankness*: `""` decodes fine, and BC3 can never render any of
    /// the three strings blank — `Schedule::Entry#summary` is
    /// `super.presence || "Untitled"`, and `starts_at`/`ends_at` are `NOT NULL`
    /// columns every partial emits. A blank one on a 2xx read is therefore a
    /// malformed response, and carrying it into the full-replace PUT would
    /// destroy the real value on a call that only touched something else.
    init(from entry: ScheduleEntry) throws {
        summary = try Self.required(entry.summary, field: "summary")
        startsAt = try Self.required(entry.startsAt, field: "starts_at")
        endsAt = try Self.required(entry.endsAt, field: "ends_at")
        description = entry.description ?? ""
        allDay = entry.allDay

        // Seeded for reading only. Property observers do not fire during
        // initialization, so every dirty bit stays false.
        participantIds = (entry.participants ?? []).map { $0.id.value }
        url = entry.joinUrl ?? ""
        highlighted = entry.highlighted ?? false
        notify = false
    }

    private static func required(_ value: String, field: String) throws -> String {
        guard !value.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw BasecampError.api(
                message: "GetScheduleEntry returned an entry with a blank \"\(field)\", "
                    + "but the API never renders it blank",
                httpStatus: nil,
                hint: "The merge-safe updateEntry/editEntry resend this field verbatim, so a "
                    + "blank value would overwrite the current one. Use "
                    + "replaceEntry(entryId:req:) to write the record deliberately.",
                requestId: nil,
                decodeFailure: nil
            )
        }
        return value
    }
}

// Merge-safe `updateEntry` and read-modify-write `editEntry`, composed from the
// public `getEntry` and `replaceEntry` methods — hooks observe the two wire
// operations (`GetScheduleEntry` then `ReplaceScheduleEntry`), not a synthetic
// composite.
//
// `PUT /{accountId}/schedule_entries/{entryId}` is a full replace: BC3's
// `Schedules::EntriesController#update` rebuilds the recordable from the
// submitted params, so a PUT that omits `summary` or `description` clears it —
// a 200 that quietly destroys, not a 422. Three writable fields are exempt:
// `participant_ids`, `url` and `highlighted` are seeded from the existing
// recordable when the request does not address them, which is why the
// composites keep them off the wire rather than echoing the read-back.
//
// Neither composite is atomic: there is no conditional-update signal on this
// endpoint, so a concurrent write between the GET and PUT is overwritten — last
// write wins for the whole representation. The window is one round-trip. Use
// `replaceEntry` to overwrite deliberately.
//
// Recurring entries are out of reach here: `ensure_non_recurring_event`
// 302-redirects both `show` and `update` for a recurring entry, so this route
// serves non-recurring entries only. The SDK does not follow a redirect on a
// PUT, and the GET's redirect surfaces as an unexpected body rather than an
// entry. Use `getEntryOccurrence` to read a single occurrence.
extension SchedulesService {
    /// Sets the given fields on a schedule entry and preserves everything else:
    /// GETs the current entry, overlays the explicitly-set (non-`nil`) request
    /// fields, and PUTs the full representation back.
    ///
    /// A `nil` full-state field is untouched, guaranteed. A `nil` carve-out
    /// (`participantIds`, `url`, `highlighted`, `notify`) stays off the wire so
    /// BC3 keeps what it holds; an explicitly-passed `[]`, `""` or `false`
    /// clears it.
    ///
    /// The read-back's `join_url`, `highlighted` and `participants` are never
    /// echoed into the PUT — in particular the entry's own `url` (its Basecamp
    /// API URL) is not the join link and must never be written into one.
    ///
    /// Not atomic, and non-recurring entries only — see the extension docs.
    public func updateEntry(entryId: Int, req: UpdateScheduleEntryRequest) async throws
        -> ScheduleEntry
    {
        var fields = try ScheduleEntryFields(from: try await fetchEntry(entryId: entryId))
        if let summary = req.summary { fields.summary = summary }
        if let startsAt = req.startsAt { fields.startsAt = startsAt }
        if let endsAt = req.endsAt { fields.endsAt = endsAt }
        if let description = req.description { fields.description = description }
        if let allDay = req.allDay { fields.allDay = allDay }
        // Each assignment below trips the matching dirty bit through the
        // property observer, which is what puts the carve-out on the wire.
        if let participantIds = req.participantIds { fields.participantIds = participantIds }
        if let url = req.url { fields.url = url }
        if let highlighted = req.highlighted { fields.highlighted = highlighted }
        if let notify = req.notify { fields.notify = notify }
        return try await putFields(entryId: entryId, fields: fields)
    }

    /// Applies a read-modify-write closure to a schedule entry: GETs the
    /// current entry, hands the closure the full writable representation
    /// (``ScheduleEntryFields``), and PUTs the whole thing back. If the closure
    /// throws, the edit aborts and nothing is written.
    ///
    /// Full-state fields ride through untouched; clearing one means setting it
    /// empty (`""`). The carve-outs are seeded so the closure can read them,
    /// but only an **assignment** sends one — assigning exactly the value the
    /// GET returned still counts, because intent is not recoverable from the
    /// value.
    ///
    /// ```swift
    /// try await account.schedules.editEntry(entryId: 123) {
    ///     $0.summary = "🚨 " + $0.summary
    ///     $0.description = ""          // clearing = setting empty
    ///     $0.url = $0.url              // a write, even though nothing changed
    /// }
    /// ```
    ///
    /// Not atomic, and non-recurring entries only — see the extension docs.
    public func editEntry(
        entryId: Int, _ mutate: (inout ScheduleEntryFields) throws -> Void
    ) async throws -> ScheduleEntry {
        var fields = try ScheduleEntryFields(from: try await fetchEntry(entryId: entryId))
        try mutate(&fields)
        return try await putFields(entryId: entryId, fields: fields)
    }

    /// PUTs the full writable state via `replaceEntry`. The five full-state
    /// fields are ALWAYS sent, empties included: `""` is how a clear is
    /// expressed on a full-replace endpoint, and an explicit JSON null is never
    /// sent (SPEC §18 body compaction). Each carve-out is sent only when its
    /// dirty bit is set, and `nil` for the rest — which the synthesized encoder
    /// omits via `encodeIfPresent`, leaving BC3's preserve-on-omission to run.
    private func putFields(entryId: Int, fields: ScheduleEntryFields) async throws -> ScheduleEntry
    {
        try await replaceEntry(
            entryId: entryId,
            req: ReplaceScheduleEntryRequest(
                allDay: fields.allDay,
                description: fields.description,
                endsAt: fields.endsAt,
                highlighted: fields.addressedHighlighted ? fields.highlighted : nil,
                notify: fields.addressedNotify ? fields.notify : nil,
                participantIds: fields.addressedParticipantIds ? fields.participantIds : nil,
                startsAt: fields.startsAt,
                summary: fields.summary,
                url: fields.addressedURL ? fields.url : nil
            )
        )
    }

    /// GETs the entry the composites read their writable state from.
    private func fetchEntry(entryId: Int) async throws -> ScheduleEntry {
        // Swift's decoder is the typed guard the dynamic SDKs have to write by
        // hand, and it rejects an absent, null or wrong-typed required field
        // before this composite ever sees it. `BaseService.decoding(_:_:)` now
        // renders that as the SPEC §6 malformed-2xx-body shape for every
        // operation (#604) — statusless, non-retryable `api_error` — so what is
        // left to add here is the part the base layer cannot know: the
        // composite's escape hatch. `BasecampError.decodeFailure` is what
        // recognizes that one failure — statuslessness alone would also match
        // the pagination same-origin guard — so every other error passes
        // through untouched. The restatement carries the marker forward: it is
        // the same malformed body with a better hint, and dropping it would tell
        // the conformance runner (and any caller) this was not a decode failure
        // (#750).
        do {
            return try await getEntry(entryId: entryId)
        } catch let error as BasecampError {
            guard let decodeFailure = error.decodeFailure else { throw error }
            throw BasecampError.api(
                message: error.message,
                httpStatus: nil,
                hint: "The merge-safe updateEntry/editEntry resend this record's fields verbatim, "
                    + "so a malformed response cannot be written back safely. Use "
                    + "replaceEntry(entryId:req:) to write the record deliberately.",
                requestId: nil,
                decodeFailure: decodeFailure
            )
        }
    }
}
