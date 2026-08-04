"""Schedules service with merge-safe ``update_entry`` and read-modify-write ``edit_entry``.

``PUT /{accountId}/schedule_entries/{entryId}`` is a full replace: BC3's
``Schedules::EntriesController#update`` rebuilds the recordable from the
submitted params, so a writable field the body omits is cleared. Omitting
``summary`` leaves the entry reading back as ``"Untitled"``; omitting
``description`` clears it; omitting ``all_day`` turns an all-day event into a
midnight-to-midnight timed one. None of those is a 422 — each is a ``200`` that
quietly rewrites the entry.

**Except three.** BC3 seeds ``participant_ids``, ``url`` and ``highlighted``
from the existing recordable when the request does not address them, so for
those three an omission preserves rather than clears. That splits the writable
set in two, and the split is the whole point of this module:

* **full state** — ``summary``, ``starts_at``, ``ends_at``, ``description``,
  ``all_day``: always sent, empties included. On a full-replace endpoint ``""``
  is how a clear is expressed, never JSON null and never omission.
* **addressed-only** — ``participant_ids``, ``url``, ``highlighted``,
  ``notify``: sent only when the caller addressed them, and never seeded from
  the read-back. Resending what BC3 already preserves is redundant at best and
  wrong if the read raced a concurrent change. ``notify`` is addressed-only for
  a different reason: it is a directive, not state — sending it makes BC3
  recompute a drafted entry's subscriber list.

An explicitly empty value in the second class is an address, not an absence:
``participant_ids=[]`` clears participants, ``url=""`` clears the join link,
``highlighted=False`` removes the highlight. Each survives body compaction,
which strips only ``None``.

The response spells the join link ``join_url`` — the entry's ``url`` is its own
Basecamp API URL, written by the recordings partial that renders first. Echoing
the response's ``url`` into the request's ``url`` would therefore write the
entry's API URL into its join link, which is why the read-side seed reads
``join_url`` and the composite never seeds it at all.

Both composites compose the public ``get_entry`` and ``replace_entry`` methods,
so hooks observe the two wire operations (``get_entry`` then ``replace_entry``),
not a synthetic composite.

**Non-recurring entries only.** BC3's ``ensure_non_recurring_event`` 302-redirects
both ``show`` and ``update`` for a recurring entry, so this route serves
non-recurring entries. A recurring entry surfaces as a redirect the SDK does not
follow for the PUT, or as an unexpected body on the GET — which the read guards
below refuse before anything is written.

Neither composite is atomic: there is no conditional-update signal on this
endpoint, so a concurrent write between the GET and PUT is overwritten — last
write wins for the whole representation. The window is one round-trip. Use
``replace_entry`` to overwrite deliberately.
"""

from __future__ import annotations

from typing import Any

from basecamp.errors import UsageError
from basecamp.generated.services.schedules import (
    AsyncSchedulesService as _GeneratedAsyncSchedulesService,
)
from basecamp.generated.services.schedules import SchedulesService as _GeneratedSchedulesService
from basecamp.services._merge_safe import (
    require_mapping,
    required_writable_boolean,
    required_writable_string,
    writable_boolean,
    writable_id_list,
    writable_string,
)

_ESCAPE = "replace_entry()"
_RECORD = "ScheduleEntry"

#: Resent verbatim on every PUT, empties included — BC3 clears what the body omits.
_FULL_STATE = ("summary", "starts_at", "ends_at", "description", "all_day")

#: Sent only when the caller addressed them. BC3's ``preservedOnOmission`` carve-out
#: covers the first three; ``notify`` is a directive rather than entry state.
_CARVE_OUTS = frozenset({"participant_ids", "url", "highlighted", "notify"})


def _require_entry(entry: object) -> dict[str, Any]:
    return require_mapping(entry, record=_RECORD, operation="GetScheduleEntry", escape=_ESCAPE)


def _fields_from_entry(body: dict[str, Any]) -> dict[str, Any]:
    """Derive an entry's full writable state from a GET response.

    Every value here is resent in the full-replace PUT, so every value is
    validated before it is read. A plain ``or ""`` would coerce each falsey
    non-string to ``""`` — erasing the field on a call that never mentioned it —
    and pass ``42``/``True`` straight through to be written verbatim. Python has
    no typed decoder between the GET and this read (``get_entry`` returns
    ``dict[str, Any]``), so the check is explicit work here rather than
    something the layer below already did. See
    :mod:`basecamp.services._merge_safe` and #576.

    The five fields read differently because the spec models them differently:

    * ``summary`` is ``@required`` and ``Schedule::Entry#summary`` is
      ``super.presence || "Untitled"``, so absent, null or blank is a malformed
      response rather than an empty summary;
    * ``starts_at``/``ends_at`` are ``@required`` and ``NOT NULL`` — and their
      wire value is a bare date (``"2026-06-05"``) for an all-day entry and a
      timestamp otherwise, so they are round-tripped verbatim and never parsed;
    * ``all_day`` is ``@required``, ``NOT NULL DEFAULT false``, and the one
      field whose guard cannot be a truthiness test: ``False`` is the value it
      most needs to admit, and defaulting a missing one to ``False`` would
      convert an all-day event into a midnight-to-midnight timed one;
    * ``description`` is optional and nullable, so absent or null is a genuinely
      empty description.
    """
    return {
        "summary": required_writable_string(body, "summary", record=_RECORD, escape=_ESCAPE),
        "starts_at": required_writable_string(body, "starts_at", record=_RECORD, escape=_ESCAPE),
        "ends_at": required_writable_string(body, "ends_at", record=_RECORD, escape=_ESCAPE),
        "description": writable_string(body, "description", record=_RECORD, escape=_ESCAPE),
        "all_day": required_writable_boolean(body, "all_day", record=_RECORD, escape=_ESCAPE),
    }


def _carve_out_seeds(body: dict[str, Any]) -> dict[str, Any]:
    """Seed the carve-outs **for reading inside an edit block**, never for sending.

    :meth:`SchedulesService.edit_entry` exposes the current join link, highlight
    and participants so a block can inspect them before deciding; only an
    assignment puts one on the wire. The values are still guarded, because
    ``entry.url = entry.url`` is a legitimate write and would otherwise carry a
    malformed value into the PUT.

    ``url`` is read from the response's ``join_url``: the entry's own ``url`` is
    its Basecamp API URL. ``participant_ids`` is projected from the
    ``participants`` array. ``notify`` has no read-back at all — it is a send
    directive — so it starts ``False`` and only reaches the wire if the block
    assigns it.
    """
    return {
        "participant_ids": writable_id_list(body, "participants", record=_RECORD, escape=_ESCAPE),
        "url": writable_string(body, "join_url", record=_RECORD, escape=_ESCAPE),
        "highlighted": writable_boolean(body, "highlighted", record=_RECORD, escape=_ESCAPE),
        "notify": False,
    }


def _replace_kwargs(fields: dict[str, Any], addressed: dict[str, Any]) -> dict[str, Any]:
    """Serialize full writable state plus whatever the caller addressed.

    The five full-state fields are always named, empties included. The
    addressed-only keys are merged in exactly as passed, so an explicit ``""``,
    ``False`` or ``[]`` reaches the wire while an unaddressed key never appears
    — the transport's compaction strips ``None`` only, so absence has to be
    expressed by leaving the key out here.
    """
    return {
        "summary": fields["summary"],
        "starts_at": fields["starts_at"],
        "ends_at": fields["ends_at"],
        "description": fields["description"],
        "all_day": fields["all_day"],
        **addressed,
    }


def _overlay(fields: dict[str, Any], **updates: Any) -> dict[str, Any]:
    for key, value in updates.items():
        if value is not None:
            fields[key] = value
    return fields


def _addressed_carve_outs(**candidates: Any) -> dict[str, Any]:
    """Keep the carve-outs the caller actually passed.

    ``None`` is the only "not addressed" signal, so ``""``, ``False`` and ``[]``
    all survive — each of them is an instruction to clear, and handing a clear
    back to BC3's carve-out would preserve instead.
    """
    return {key: value for key, value in candidates.items() if value is not None}


class _ScheduleEntryEditBase:
    """Shared writable state for :class:`ScheduleEntryEdit` / :class:`AsyncScheduleEntryEdit`.

    Inside the ``with`` block the edit object exposes the entry's full writable
    state — ``summary``, ``starts_at``, ``ends_at``, ``description``,
    ``all_day`` — which is always written back, plus the four addressed-only
    fields ``participant_ids``, ``url``, ``highlighted`` and ``notify``, which
    are written back **only if the block assigns them**.

    That dirty set is keyed on setter invocation, not on value comparison. A
    snapshot diff would drop ``entry.url = entry.url``, and the intent behind
    that statement is not recoverable from the value: re-asserting the join link
    the read returned is a write, and a caller who wrote it deserves to have it
    sent. Clearing a field means setting it empty (``""``, ``[]``, ``False``);
    an untouched full-state field keeps its current value, and an untouched
    carve-out is left to BC3, which preserves it.
    """

    summary: str
    starts_at: str
    ends_at: str
    description: str
    all_day: bool
    participant_ids: list[int]
    url: str
    highlighted: bool
    notify: bool

    def __init__(self, entry_id: int) -> None:
        # First, and before any tracked attribute can be set: __setattr__ reads it.
        self._touched: set[str] = set()
        self._entry_id = entry_id
        self._result: dict[str, Any] | None = None
        self._completed = False

    def __setattr__(self, name: str, value: Any) -> None:
        """Record an assignment to a carve-out, then store it.

        This — not a comparison against the read-back — is what puts
        ``participant_ids``, ``url``, ``highlighted`` or ``notify`` on the wire.
        """
        if name in _CARVE_OUTS:
            self._touched.add(name)
        object.__setattr__(self, name, value)

    def _load(self, entry: dict[str, Any]) -> None:
        body = _require_entry(entry)
        for key, value in _fields_from_entry(body).items():
            setattr(self, key, value)
        # Seeded for reading only. Going through object.__setattr__ keeps the
        # seeds out of the dirty set, so a block that merely reads `url` does
        # not write it back.
        for key, value in _carve_out_seeds(body).items():
            object.__setattr__(self, key, value)

    def _fields(self) -> dict[str, Any]:
        return {key: getattr(self, key) for key in _FULL_STATE}

    def _addressed(self) -> dict[str, Any]:
        addressed = {key: getattr(self, key) for key in sorted(self._touched)}
        for key, value in addressed.items():
            if value is None:
                raise UsageError(
                    f"{key} was set to None; a full write has no None state — "
                    f'use "" or [] or False to clear it, or leave it unassigned to keep it'
                )
        return addressed

    @property
    def result(self) -> dict[str, Any]:
        """The updated entry, available after the ``with`` block exits cleanly."""
        if not self._completed:
            raise RuntimeError("edit has not completed")
        assert self._result is not None
        return self._result


class ScheduleEntryEdit(_ScheduleEntryEditBase):
    """Read-modify-write context manager returned by :meth:`SchedulesService.edit_entry`.

    Entering the block GETs the current entry; exiting cleanly PUTs the whole
    representation back. If the block raises, the edit aborts and nothing is
    written.
    """

    def __init__(self, service: SchedulesService, entry_id: int) -> None:
        super().__init__(entry_id)
        self._service = service

    def __enter__(self) -> ScheduleEntryEdit:
        self._load(self._service.get_entry(entry_id=self._entry_id))
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = self._service.replace_entry(
                entry_id=self._entry_id, **_replace_kwargs(self._fields(), self._addressed())
            )
            self._completed = True


class AsyncScheduleEntryEdit(_ScheduleEntryEditBase):
    """Async twin of :class:`ScheduleEntryEdit`, for ``async with``."""

    def __init__(self, service: AsyncSchedulesService, entry_id: int) -> None:
        super().__init__(entry_id)
        self._service = service

    async def __aenter__(self) -> AsyncScheduleEntryEdit:
        self._load(await self._service.get_entry(entry_id=self._entry_id))
        return self

    async def __aexit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = await self._service.replace_entry(
                entry_id=self._entry_id, **_replace_kwargs(self._fields(), self._addressed())
            )
            self._completed = True


class SchedulesService(_GeneratedSchedulesService):
    """Schedules service with merge-safe ``update_entry`` and ``edit_entry`` on top
    of the generated surface (``get_entry``, ``replace_entry``, ...)."""

    def update_entry(
        self,
        *,
        entry_id: int,
        summary: str | None = None,
        starts_at: str | None = None,
        ends_at: str | None = None,
        description: str | None = None,
        all_day: bool | None = None,
        participant_ids: list[int] | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        """Set the given fields on a schedule entry and preserve everything else.

        GETs the current entry, overlays the explicitly-passed keyword
        arguments, and PUTs the full representation back. An omitted (``None``)
        full-state field is untouched, guaranteed; an explicitly-passed ``""``
        or ``False`` sets it.

        ``participant_ids``, ``url``, ``highlighted`` and ``notify`` are sent
        only when passed and are never read back from the entry: BC3 preserves
        the first three when the request does not address them, and ``notify``
        is a send directive rather than entry state. Passing ``[]``, ``""`` or
        ``False`` for one of them is an address, and clears.

        ``url`` is the join link — the request spelling of the field the
        response returns as ``join_url``.

        Non-recurring entries only: BC3 302-redirects both hops for a recurring
        entry. Not atomic — a concurrent write between the GET and PUT is
        overwritten (last write wins for the whole representation; the window is
        one round-trip). Use :meth:`replace_entry` to overwrite deliberately.
        """
        fields = _overlay(
            _fields_from_entry(_require_entry(self.get_entry(entry_id=entry_id))),
            summary=summary,
            starts_at=starts_at,
            ends_at=ends_at,
            description=description,
            all_day=all_day,
        )
        addressed = _addressed_carve_outs(
            participant_ids=participant_ids, url=url, highlighted=highlighted, notify=notify
        )
        return self.replace_entry(entry_id=entry_id, **_replace_kwargs(fields, addressed))

    def edit_entry(self, *, entry_id: int) -> ScheduleEntryEdit:
        """Open a read-modify-write edit of a schedule entry, as a context manager.

        Entering the ``with`` block GETs the current entry and exposes its full
        writable state; exiting cleanly PUTs the whole representation back.
        Clearing a field means setting it empty (``""``, ``[]``, ``False``). If
        the block raises, nothing is written. The updated entry is available as
        ``.result`` after the block::

            with client.schedules.edit_entry(entry_id=123) as e:
                e.summary = f"🚨 {e.summary}"
                e.description = ""  # clearing = setting empty on a full object
                e.url = e.url  # re-asserting the join link IS a write

        ``participant_ids``, ``url``, ``highlighted`` and ``notify`` reach the
        wire only if the block assigns them — reading one does not. The last
        line above therefore sends the join link, even though its value did not
        change: the dirty set is keyed on the assignment, not on a comparison.

        Non-recurring entries only. Not atomic: a concurrent write between the
        GET and PUT is overwritten (last write wins for the whole
        representation; the window is one round-trip).
        """
        return ScheduleEntryEdit(self, entry_id)


class AsyncSchedulesService(_GeneratedAsyncSchedulesService):
    """Async schedules service with merge-safe ``update_entry`` and ``edit_entry``."""

    async def update_entry(
        self,
        *,
        entry_id: int,
        summary: str | None = None,
        starts_at: str | None = None,
        ends_at: str | None = None,
        description: str | None = None,
        all_day: bool | None = None,
        participant_ids: list[int] | None = None,
        url: str | None = None,
        highlighted: bool | None = None,
        notify: bool | None = None,
    ) -> dict[str, Any]:
        """Async twin of :meth:`SchedulesService.update_entry`."""
        fields = _overlay(
            _fields_from_entry(_require_entry(await self.get_entry(entry_id=entry_id))),
            summary=summary,
            starts_at=starts_at,
            ends_at=ends_at,
            description=description,
            all_day=all_day,
        )
        addressed = _addressed_carve_outs(
            participant_ids=participant_ids, url=url, highlighted=highlighted, notify=notify
        )
        return await self.replace_entry(entry_id=entry_id, **_replace_kwargs(fields, addressed))

    def edit_entry(self, *, entry_id: int) -> AsyncScheduleEntryEdit:
        """Async twin of :meth:`SchedulesService.edit_entry`, for ``async with``::

        async with client.schedules.edit_entry(entry_id=123) as e:
            e.summary = f"🚨 {e.summary}"
        updated = e.result
        """
        return AsyncScheduleEntryEdit(self, entry_id)
