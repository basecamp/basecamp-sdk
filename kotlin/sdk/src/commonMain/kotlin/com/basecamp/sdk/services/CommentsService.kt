package com.basecamp.sdk.services

import com.basecamp.sdk.AccountClient
import com.basecamp.sdk.BasecampException
import com.basecamp.sdk.generated.models.Comment
import com.basecamp.sdk.generated.models.Person
import com.basecamp.sdk.generated.people
import com.basecamp.sdk.generated.services.CreateCommentBody
import com.basecamp.sdk.withMentions

/**
 * `CommentsService` with the mention-expanding writes on top of the generated
 * surface (`get`, `list`, `create`, `update`).
 *
 * Both are SPEC.md §18 composites over generated wire methods — one people read
 * per distinct id, then the generated create — and they mint no operation
 * identity of their own: hooks see `GetPerson` and `CreateComment` under their
 * normal names (rule 3).
 */
class CommentsService(private val account: AccountClient) :
    com.basecamp.sdk.generated.services.CommentsService(account) {

    /**
     * Returns [content] mentioning each of [personIds], for posting as a comment
     * — or, since the markup is the same, as a rich-text Campfire line.
     *
     * Every requested id is read through `people.get` for its
     * `attachable_sgid` — one read per distinct id, ALWAYS: an sgid already in
     * the content is unsigned and cannot prove the person is mentioned, so it
     * never stands in for the read — and the mentions are placed as
     * [withMentions] places them, which adds nothing for a person whose exact
     * `attachable_sgid` the content already carries. A person read that fails —
     * an id that is not a person in this account, a 403 — fails the expansion;
     * nothing is posted on a partial mention list.
     *
     * That read's own exception propagates unwrapped, which is a deliberate
     * divergence from the Go original: Go annotates it with the person id
     * (`resolving mention for person N`) and keeps the underlying error
     * reachable through `errors.As`, a pairing Kotlin has no equivalent for —
     * wrapping here would replace `BasecampException.NotFound` with something a
     * caller can no longer match on. The type is kept and the id is lost; with
     * several ids in flight, a caller wanting to know WHICH read failed has to
     * expand them one at a time.
     *
     * The rendered mentions round-trip:
     * [com.basecamp.sdk.mentionedPersonIds] on the returned content reports
     * every id passed here, and `RecordingsService.summarize` reports them on the
     * comment once posted.
     */
    suspend fun expandMentions(content: String, personIds: List<Long>): String {
        if (personIds.isEmpty()) return content
        val people = mutableListOf<Person>()
        val seen = mutableSetOf<Long>()
        for (id in personIds) {
            if (id <= 0) throw BasecampException.Usage("invalid mention person id $id")
            if (!seen.add(id)) continue
            people.add(account.people.get(id))
        }
        return withMentions(content, people)
    }

    /**
     * Creates a comment on [recordingId] whose content mentions [personIds]:
     * [expandMentions], then the generated `create`. The mention reads happen
     * before the write, so a failed lookup posts nothing.
     */
    suspend fun createWithMentions(recordingId: Long, content: String, personIds: List<Long>): Comment {
        if (content.isEmpty()) throw BasecampException.Usage("comment content is required")
        return create(recordingId, CreateCommentBody(content = expandMentions(content, personIds)))
    }
}
