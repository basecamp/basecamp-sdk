"""Cards service with a presence-aware ``update``.

BC3 permits a card's JSON update params exactly as submitted (bc3#12521):
``kanban/cards_controller`` builds them from ``card_params`` alone, so a key the
body never mentions is never written. An omitted ``due_on`` therefore leaves the
card's due date **unchanged**, and only an explicit ``""`` or null clears it. The
``{ due_on: nil }.merge(card_params)`` default that used to make every sparse
PUT destructive survives for HTML and turbo_stream web forms alone, which this
SDK never speaks.

A sparse PUT is consequently the correct thing to write, so ``update`` sends
exactly the fields the caller addressed in a single request. The
read-modify-write that used to guard ``due_on`` is gone with the behaviour it
guarded: it now costs a round-trip to preserve a value nothing threatens, and
its GET-then-PUT window was a race a single request does not have.

A clear is spelled ``""`` rather than a JSON null — null would violate body
compaction (SPEC section 18), and Rails casts a blank date to nil regardless.

``update_verbatim`` remains as the unadorned name for the same single PUT.
"""

from __future__ import annotations

from typing import Any

from basecamp.generated.services.cards import AsyncCardsService as _GeneratedAsyncCardsService
from basecamp.generated.services.cards import CardsService as _GeneratedCardsService

_UPDATE_DOC = """Update a card without disturbing fields you did not mention.

``due_on`` is tri-state:

* ``None`` (unaddressed) — the key is omitted and the due date is left alone
* ``""`` — the due date is cleared, sent as an explicit empty string
* a date — the due date is set

Nothing is resent on your behalf: the request body carries only the arguments
you passed. Echoing a field back would be actively wrong for assignees — BC3
filters incoming IDs through ``reachable_people``, so replaying an id belonging
to someone who has since lost board access would silently unassign them.
"""


class CardsService(_GeneratedCardsService):
    """Sync cards service with a presence-aware ``update``."""

    def update(
        self,
        *,
        card_id: int,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        return self.update_verbatim(
            card_id=card_id,
            title=title,
            content=content,
            due_on=due_on,
            assignee_ids=assignee_ids,
        )

    update.__doc__ = _UPDATE_DOC


class AsyncCardsService(_GeneratedAsyncCardsService):
    """Async cards service with a presence-aware ``update``."""

    async def update(
        self,
        *,
        card_id: int,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        return await self.update_verbatim(
            card_id=card_id,
            title=title,
            content=content,
            due_on=due_on,
            assignee_ids=assignee_ids,
        )

    update.__doc__ = _UPDATE_DOC
