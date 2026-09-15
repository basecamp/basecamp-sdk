"""Comments service with the mention-expanding write composites.

``expand_mentions`` turns a list of person ids into the ``<bc-attachment>``
markup BC3 honours, and ``create_with_mentions`` posts a comment whose content
carries it. Both are hand-written composition over generated operations
(SPEC.md section 18): the people reads and the comment create are the generated
service methods, under their own hook identities, and nothing here builds a path
or picks a verb.
"""

from __future__ import annotations

from collections.abc import Iterable
from typing import Any

from basecamp.errors import UsageError
from basecamp.generated.services.comments import AsyncCommentsService as _GeneratedAsyncCommentsService
from basecamp.generated.services.comments import CommentsService as _GeneratedCommentsService
from basecamp.mentions import with_mentions

_EXPAND_MENTIONS_DOC = """Content that mentions each of the given people, for posting as a comment.

Or, since the markup is the same, as a rich-text Campfire line. Every requested
id is read through ``people.get`` for its ``attachable_sgid`` -- one read per
distinct id, always: an sgid already in the content is unsigned and cannot prove
the person is mentioned, so it never stands in for the read -- and the mentions
are placed as :func:`basecamp.mentions.with_mentions` places them, which adds
nothing for a person whose exact ``attachable_sgid`` the content already
carries. A person read that fails -- an id that is not a person in this account,
a 403 -- fails the expansion; nothing is posted on a partial mention list.

The rendered mentions round-trip:
:func:`basecamp.mentions.mentioned_person_ids` on the returned content reports
every id passed here, and ``recordings.summarize`` reports them on the comment
once posted.
"""

_CREATE_WITH_MENTIONS_DOC = """Create a comment on a recording whose content mentions the given people.

``expand_mentions``, then ``create``. The mention reads happen before the write,
so a failed lookup posts nothing.
"""


def _distinct_person_ids(person_ids: Iterable[int]) -> list[int]:
    """The requested ids, in order, without repeats. Refuses one that is not an id."""
    seen: set[int] = set()
    distinct: list[int] = []
    for person_id in person_ids:
        if not isinstance(person_id, int) or isinstance(person_id, bool) or person_id <= 0:
            raise UsageError(f"invalid mention person id {person_id!r}")
        if person_id in seen:
            continue
        seen.add(person_id)
        distinct.append(person_id)
    return distinct


class CommentsService(_GeneratedCommentsService):
    """Sync comments service with the mention-expanding composites."""

    def expand_mentions(self, *, content: str, person_ids: Iterable[int] | None = None) -> str:
        ids = _distinct_person_ids(person_ids or ())
        if not ids:
            return content
        people = []
        for person_id in ids:
            try:
                people.append(self._client.people.get(person_id=person_id))
            except Exception as error:
                # Annotated rather than re-raised as something else: which
                # person failed is context, and replacing the error would throw
                # away the canonical code, the HTTP status and the retry hints a
                # caller classifies on.
                error.add_note(f"resolving mention for person {person_id}")
                raise
        return with_mentions(content, people)

    expand_mentions.__doc__ = _EXPAND_MENTIONS_DOC

    def create_with_mentions(
        self, *, recording_id: int, content: str, mentions: Iterable[int] | None = None
    ) -> dict[str, Any]:
        if not content:
            raise UsageError("comment content is required")
        expanded = self.expand_mentions(content=content, person_ids=mentions)
        return self.create(recording_id=recording_id, content=expanded)

    create_with_mentions.__doc__ = _CREATE_WITH_MENTIONS_DOC


class AsyncCommentsService(_GeneratedAsyncCommentsService):
    """Async comments service with the mention-expanding composites."""

    async def expand_mentions(self, *, content: str, person_ids: Iterable[int] | None = None) -> str:
        ids = _distinct_person_ids(person_ids or ())
        if not ids:
            return content
        people = []
        for person_id in ids:
            try:
                people.append(await self._client.people.get(person_id=person_id))
            except Exception as error:
                error.add_note(f"resolving mention for person {person_id}")
                raise
        return with_mentions(content, people)

    expand_mentions.__doc__ = _EXPAND_MENTIONS_DOC

    async def create_with_mentions(
        self, *, recording_id: int, content: str, mentions: Iterable[int] | None = None
    ) -> dict[str, Any]:
        if not content:
            raise UsageError("comment content is required")
        expanded = await self.expand_mentions(content=content, person_ids=mentions)
        return await self.create(recording_id=recording_id, content=expanded)

    create_with_mentions.__doc__ = _CREATE_WITH_MENTIONS_DOC
