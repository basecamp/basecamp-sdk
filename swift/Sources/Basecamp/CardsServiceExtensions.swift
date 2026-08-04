import Foundation

/// A due-date-aware `update` on top of the generated `CardsService` surface
/// (`get`, `updateVerbatim`, `move`, ...).
///
/// BC3's JSON card update is presence-aware (basecamp/bc3#12521):
/// `kanban/cards_controller.rb` builds the update params from `card_params`
/// directly, so an **omitted** `due_on` leaves the stored date **unchanged**,
/// and only an explicit `""` (Rails blank-casts it to nil on the date
/// attribute) or an explicit null clears it.
///
/// That makes a sparse PUT safe, so `update` sends exactly what the caller
/// addressed, in a single request. Its remaining job is the one distinction a
/// bare `String?` cannot express: "leave the due date alone" (omit the key)
/// versus "clear the due date" (send `""`). `updateVerbatim` remains the raw
/// request-struct surface.
extension CardsService {
    /// How a caller addresses a card's due date.
    public enum DueDate: Sendable, Equatable {
        /// Leave the due date alone. `due_on` is omitted, and the server
        /// leaves whatever it holds untouched.
        case preserve
        /// Clear the due date. Sent as `"due_on": ""`.
        case clear
        /// Set the due date (YYYY-MM-DD).
        case on(String)
    }

    /// Updates a card without disturbing fields the caller did not mention.
    ///
    /// One PUT, no preceding read: the server already treats an omitted field
    /// as "unchanged", so there is nothing to fetch and resend and no
    /// read-modify-write race to lose.
    ///
    /// Assignees are never resent on the caller's behalf: BC3 filters incoming
    /// IDs through `reachable_people`, so echoing back an id belonging to
    /// someone who has since lost board access would silently unassign them.
    public func update(
        cardId: Int,
        title: String? = nil,
        content: String? = nil,
        dueOn: DueDate = .preserve,
        assigneeIds: [Int]? = nil
    ) async throws -> Card {
        // The two cases that matter are distinguished by presence, not value.
        //
        // `.preserve` resolves to nil so the key never reaches the wire, which
        // is what "unchanged" is spelled as.
        //
        // `.clear` resolves to the empty string, which must actually arrive as
        // `"due_on": ""`. Omitting the key would silently no-op against a
        // presence-aware server, and sending an explicit null would violate the
        // body-compaction rule in SPEC section 18.
        //
        // `UpdateCardRequest.dueOn` is `String?` and Swift's synthesized
        // `Codable` encodes it with `encodeIfPresent`, which drops a nil but
        // keeps an empty string — so this nil/"" split is exactly the
        // absent/present split on the wire.
        let resolvedDueOn: String?
        switch dueOn {
        case .preserve:
            resolvedDueOn = nil
        case .clear:
            resolvedDueOn = ""
        case .on(let date):
            resolvedDueOn = date
        }

        return try await updateVerbatim(
            cardId: cardId,
            req: UpdateCardRequest(
                assigneeIds: assigneeIds,
                content: content,
                dueOn: resolvedDueOn,
                title: title
            )
        )
    }
}
