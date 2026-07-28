"""Cards service with a merge-safe ``update``.

BC3 builds the card's update params as ``{ due_on: nil }.merge(card_params)``
(``kanban/cards_controller.rb``), so **any** update whose body omits ``due_on``
erases the card's due date. A sparse PUT — the natural thing to write — is
therefore destructive on the raw endpoint, which remains available as
``update_verbatim``.

``update`` composes the public ``get`` and ``update_verbatim`` methods, so
hooks observe the two wire operations, not a synthetic composite.

Not atomic: a concurrent due-date change landing between the GET and the PUT is
overwritten with the value this call read. The window is one round-trip.
"""

from __future__ import annotations

from typing import Any

from basecamp.generated.services.cards import AsyncCardsService as _GeneratedAsyncCardsService
from basecamp.generated.services.cards import CardsService as _GeneratedCardsService

_UPDATE_DOC = """Update a card without disturbing fields you did not mention.

``due_on`` is tri-state, which is what makes this safe:

* ``None`` (omitted) — the current due date is fetched and resent
* ``""`` — the due date is cleared
* a date — the due date is set

The extra GET is only paid for in the ``None`` case, the one where the API
would otherwise destroy something.

Assignees are never resent on your behalf: BC3 filters incoming IDs through
``reachable_people``, so echoing back an id belonging to someone who has since
lost board access would silently unassign them.
"""


def _resolve_due_on(due_on: str | None, current: dict[str, Any] | None) -> str | None:
    """Map the caller's tri-state ``due_on`` onto a wire value.

    Clearing is encoded by OMITTING ``due_on`` — the generated service's
    ``_compact`` strips ``None``, and BC3 nils an omitted due date. Sending an
    explicit null would violate body compaction (SPEC §18), and sending ``""``
    risks a date-format error.
    """
    if due_on is None:
        return (current or {}).get("due_on") or None
    if due_on == "":
        return None
    return due_on


class CardsService(_GeneratedCardsService):
    """Sync cards service with a merge-safe ``update``."""

    def update(
        self,
        *,
        card_id: int,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        current = self.get(card_id=card_id) if due_on is None else None
        return self.update_verbatim(
            card_id=card_id,
            title=title,
            content=content,
            due_on=_resolve_due_on(due_on, current),
            assignee_ids=assignee_ids,
        )

    update.__doc__ = _UPDATE_DOC


class AsyncCardsService(_GeneratedAsyncCardsService):
    """Async cards service with a merge-safe ``update``."""

    async def update(
        self,
        *,
        card_id: int,
        title: str | None = None,
        content: str | None = None,
        due_on: str | None = None,
        assignee_ids: list[int] | None = None,
    ) -> dict[str, Any]:
        current = await self.get(card_id=card_id) if due_on is None else None
        return await self.update_verbatim(
            card_id=card_id,
            title=title,
            content=content,
            due_on=_resolve_due_on(due_on, current),
            assignee_ids=assignee_ids,
        )

    update.__doc__ = _UPDATE_DOC
