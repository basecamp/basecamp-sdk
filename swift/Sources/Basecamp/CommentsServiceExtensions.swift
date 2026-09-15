import Foundation

/// The mention-expanding write surface on top of the generated `CommentsService`
/// (`create`, `get`, `list`, `update`).
///
/// A sanctioned composite in SPEC §18's sense — multi-call orchestration over
/// generated operations, in the Swift same-module extension rule 5 designates.
/// Every request is a generated wire method under its own hook identity; no path
/// is built and no verb chosen here.
extension CommentsService {
    /// Returns content that mentions each of the given people, for posting as a
    /// comment — or, since the markup is the same, as a rich-text Campfire line.
    ///
    /// Every requested id is read through `people.get(personId:)` for its
    /// `attachable_sgid` — one read per distinct id, always: an sgid already in
    /// the content is unsigned and cannot prove the person is mentioned, so it
    /// never stands in for the read — and the mentions are placed as
    /// ``Mentions/adding(_:to:)`` places them, which adds nothing for a person
    /// whose exact `attachable_sgid` the content already carries.
    ///
    /// A person read that fails — an id that is not a person in this account, a
    /// 403 — fails the expansion; nothing is posted on a partial mention list.
    /// That read's ``BasecampError`` is rethrown unchanged rather than wrapped:
    /// its code is what a caller switches on, and no Swift wrapper preserves
    /// that through a `catch let error as BasecampError`.
    ///
    /// The rendered mentions round-trip: ``Mentions/personIds(in:)`` on the
    /// returned content reports every id passed here, and
    /// `RecordingsService.summarize(_:)` reports them on the comment once
    /// posted.
    public func expandMentions(_ content: String, mentioning personIds: [Int]) async throws
        -> String
    {
        guard !personIds.isEmpty else { return content }

        var people: [Person] = []
        people.reserveCapacity(personIds.count)
        var seen = Set<Int>()
        for personId in personIds {
            guard personId > 0 else {
                throw BasecampError.usage(
                    message: "invalid mention person id \(personId)", hint: nil)
            }
            guard seen.insert(personId).inserted else { continue }
            people.append(try await accountClient.people.get(personId: personId))
        }
        return try Mentions.adding(people, to: content)
    }

    /// Creates a comment on a recording whose content mentions the given people:
    /// ``expandMentions(_:mentioning:)``, then `create`. The mention reads happen
    /// before the write, so a failed lookup posts nothing.
    public func createWithMentions(recordingId: Int, content: String, mentions personIds: [Int])
        async throws -> Comment
    {
        guard !content.isEmpty else {
            throw BasecampError.usage(message: "comment content is required", hint: nil)
        }
        let expanded = try await expandMentions(content, mentioning: personIds)
        return try await create(recordingId: recordingId, req: CreateCommentRequest(content: expanded))
    }
}
