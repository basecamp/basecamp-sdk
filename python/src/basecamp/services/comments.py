"""Comments service with the mention-expanding write composites.

``expand_mentions`` turns a list of person ids into the ``<bc-attachment>``
markup BC3 honours, and ``create_with_mentions`` posts a comment whose content
carries it. Both are hand-written composition over generated operations
(SPEC.md section 18): the people reads and the comment create are the generated
service methods, under their own hook identities, and nothing here builds a path
or picks a verb.
"""

from __future__ import annotations

import contextlib
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


def _checked_person_id(person_id: Any, seen: set[int]) -> int | None:
    """One requested id, checked; ``None`` when it is a repeat to skip.

    Checked WHERE Go checks it -- inside the resolve loop, not in a pass of its
    own beforehand. The difference is observable: with ids ``[5, -1]`` Go reads
    person 5 and only then refuses, and a fixture pinning that request count
    has to mean the same thing on every runner.

    The type is checked as well as the value, which Go gets from `int64`: a
    `bool` is an `int` in Python, so ``True`` would otherwise resolve as person
    1, and a string id would raise a bare `TypeError` from the comparison
    instead of the SDK's own usage error.
    """
    if not isinstance(person_id, int) or isinstance(person_id, bool) or person_id <= 0:
        raise UsageError(f"invalid mention person id {person_id!r}")
    if person_id in seen:
        return None
    seen.add(person_id)
    return person_id


#: Marks an exception this module has already annotated. Go builds a NEW error
#: per wrap, so it cannot double-prefix; rewriting `args` in place can, and the
#: standard mock idiom -- a ``side_effect`` holding one pre-built exception
#: instance -- re-raises the same object on every call.
_ANNOTATED = "_basecamp_mention_context"


def _annotate(error: BaseException, person_id: int) -> None:
    """Say which mention failed, without replacing the error that says why.

    Go wraps this with ``fmt.Errorf("resolving mention for person %d: %w", ...)``,
    so the prefix is part of the message — and the conformance runners assert on
    message text. Rewriting `args` keeps that text while leaving the error's
    class, canonical code, HTTP status and retry hints exactly as the read
    produced them; raising a new instance would throw all of that away.
    """
    if getattr(error, _ANNOTATED, False):
        return
    context = f"resolving mention for person {person_id}"
    if error.args and isinstance(error.args[0], str):
        error.args = (f"{context}: {error.args[0]}", *error.args[1:])
    else:
        # An exception carrying no message still has to say which mention it
        # was, and it has to say it in ``str(error)`` like the others.
        error.args = (context, *error.args)
    with contextlib.suppress(AttributeError):  # an exception with __slots__
        setattr(error, _ANNOTATED, True)


class CommentsService(_GeneratedCommentsService):
    """Sync comments service with the mention-expanding composites."""

    def expand_mentions(self, *, content: str, person_ids: Iterable[int] | None = None) -> str:
        seen: set[int] = set()
        people = []
        for requested in person_ids or ():
            person_id = _checked_person_id(requested, seen)
            if person_id is None:
                continue
            try:
                people.append(self._client.people.get(person_id=person_id))
            except BaseException as error:
                _annotate(error, person_id)
                raise
        if not people:
            return content
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
        seen: set[int] = set()
        people = []
        for requested in person_ids or ():
            person_id = _checked_person_id(requested, seen)
            if person_id is None:
                continue
            try:
                people.append(await self._client.people.get(person_id=person_id))
            except BaseException as error:
                _annotate(error, person_id)
                raise
        if not people:
            return content
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
