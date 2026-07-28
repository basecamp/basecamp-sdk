import Foundation

/// A merge-safe `update` on top of the generated `CardsService` surface
/// (`get`, `updateVerbatim`, `move`, ...).
///
/// BC3 builds the card's update params as `{ due_on: nil }.merge(card_params)`
/// (`kanban/cards_controller.rb`), so **any** update whose body omits `due_on`
/// erases the card's due date. A sparse PUT — the natural thing to write — is
/// therefore destructive on the raw endpoint, which stays available as
/// `updateVerbatim`.
///
/// `update` composes the public `get` and `updateVerbatim` methods, so hooks
/// observe the two wire operations rather than a synthetic composite.
extension CardsService {
    /// How a caller addresses a card's due date.
    ///
    /// The raw endpoint cannot express "leave it alone" — an absent `due_on`
    /// already means "clear" there — so this is the distinction that makes
    /// `update` safe.
    public enum DueDate: Sendable, Equatable {
        /// Fetch the card and resend whatever due date it currently has.
        case preserve
        /// Clear the due date.
        case clear
        /// Set the due date (YYYY-MM-DD).
        case on(String)
    }

    /// Updates a card without disturbing fields the caller did not mention.
    ///
    /// The extra GET is only paid for when `dueOn` is `.preserve` — the one
    /// case where the API would otherwise destroy something.
    ///
    /// Assignees are never resent on the caller's behalf: BC3 filters incoming
    /// IDs through `reachable_people`, so echoing back an id belonging to
    /// someone who has since lost board access would silently unassign them.
    ///
    /// Not atomic: a concurrent due-date change landing between the GET and the
    /// PUT is overwritten with the value this call read. The window is one
    /// round-trip.
    public func update(
        cardId: Int,
        title: String? = nil,
        content: String? = nil,
        dueOn: DueDate = .preserve,
        assigneeIds: [Int]? = nil
    ) async throws -> Card {
        // Clearing is encoded by OMITTING due_on — `encodeIfPresent` drops a
        // nil, and BC3 nils an omitted due date. Sending an explicit null would
        // violate body compaction (SPEC §18), and sending "" risks a
        // date-format error.
        let resolvedDueOn: String?
        switch dueOn {
        case .preserve:
            let current = try await get(cardId: cardId)
            resolvedDueOn = current.dueOn?.isEmpty == false ? current.dueOn : nil
        case .clear:
            resolvedDueOn = nil
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
