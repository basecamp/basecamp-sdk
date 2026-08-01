"""Todolists service with merge-safe ``update`` and read-modify-write ``edit``.

``PUT /{accountId}/todolists/{id}`` is a full replace: BC3's
``TodolistsController#update`` rebuilds the recordable from the permitted
params alone, so a PUT that omits ``description`` ERASES it. A sparse PUT —
the natural thing to write — is therefore destructive on the raw endpoint,
which remains available as ``replace``. The writable set is exactly
``{name, description}``, and ``name`` is presence-validated server-side, so
omitting it is a 422 rather than a preserve.

Both ``update`` and ``edit`` compose the public ``get`` and ``replace``
methods, so hooks observe the two wire operations (``get`` then ``replace``),
not a synthetic composite.

Neither is atomic: there is no conditional-update signal on this endpoint, so
a concurrent write between the GET and PUT is overwritten — last write wins
for the whole representation. The window is one round-trip. Use ``replace``
to overwrite deliberately.
"""

from __future__ import annotations

from typing import Any

from basecamp.errors import UsageError
from basecamp.generated.services.todolists import AsyncTodolistsService as _GeneratedAsyncTodolistsService
from basecamp.generated.services.todolists import TodolistsService as _GeneratedTodolistsService


def _fields_from_todolist(todolist: dict[str, Any]) -> dict[str, Any]:
    """Derive a todolist's full writable state from a GET response.

    BC3 answers this route with the recordable's FLAT JSON; the
    ``{"todolist": ...}`` / ``{"group": ...}`` envelope in the Smithy spec is
    a modelling convention, not the wire shape. Unwrapping one anyway costs a
    dict lookup and stops a hypothetical enveloped body from reading every
    field as empty — which here would mean writing an empty name. The flat
    shape always wins: only a body carrying neither writable key is treated as
    an envelope, so a nested ``group`` object could never hijack the read.

    Nothing here sniffs the variant. The same URI addresses a todolist or a
    todolist group, BC3 renders both through the same template, and both carry
    the same writable pair — so a group is preserved exactly as a list is.
    """
    body = todolist
    if "name" not in todolist and "description" not in todolist:
        for key in ("todolist", "group"):
            nested = todolist.get(key)
            if isinstance(nested, dict):
                body = nested
                break
    return {
        "name": body.get("name") or "",
        "description": body.get("description") or "",
    }


def _replace_kwargs(fields: dict[str, Any]) -> dict[str, Any]:
    """Serialize full writable state for the replace transport.

    Both keys always go out. ``description`` rides along even when empty:
    ``""`` is how a clear is expressed, and the generated layer's ``_compact``
    strips ``None``, so a ``None`` would leave the key off the wire entirely
    (SPEC §18 body compaction — never JSON null). ``name`` is refused when
    empty: BC3 presence-validates it, so a full write cannot clear it and an
    empty one is a 422 waiting to happen.
    """
    if not fields["name"]:
        raise UsageError("name must be a non-empty string; BC3 presence-validates it, so a full write cannot clear it")
    return {"name": fields["name"], "description": fields["description"] or ""}


class _TodolistEditBase:
    """Shared writable state for :class:`TodolistEdit` / :class:`AsyncTodolistEdit`.

    Inside the ``with`` block the edit object exposes the todolist's full
    writable state: ``name`` and ``description``. Clearing the description
    means setting it empty (``""``) — an untouched field keeps its current
    value. The name cannot be cleared; emptying it raises
    :class:`~basecamp.errors.UsageError` on exit and writes nothing.
    """

    name: str
    description: str

    def __init__(self, id: int) -> None:
        self._id = id
        self._result: dict[str, Any] | None = None
        self._completed = False

    def _load(self, todolist: dict[str, Any]) -> None:
        for key, value in _fields_from_todolist(todolist).items():
            setattr(self, key, value)

    def _fields(self) -> dict[str, Any]:
        return {"name": self.name, "description": self.description}

    @property
    def result(self) -> dict[str, Any]:
        """The updated todolist, available after the ``with`` block exits cleanly."""
        if not self._completed:
            raise RuntimeError("edit has not completed")
        assert self._result is not None
        return self._result


class TodolistEdit(_TodolistEditBase):
    """Read-modify-write context manager returned by :meth:`TodolistsService.edit`.

    Entering the block GETs the current todolist; exiting cleanly PUTs the
    whole representation back. If the block raises, the edit aborts and
    nothing is written.
    """

    def __init__(self, service: TodolistsService, id: int) -> None:
        super().__init__(id)
        self._service = service

    def __enter__(self) -> TodolistEdit:
        self._load(self._service.get(id=self._id))
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = self._service.replace(id=self._id, **_replace_kwargs(self._fields()))
            self._completed = True


class AsyncTodolistEdit(_TodolistEditBase):
    """Async twin of :class:`TodolistEdit`, for ``async with``."""

    def __init__(self, service: AsyncTodolistsService, id: int) -> None:
        super().__init__(id)
        self._service = service

    async def __aenter__(self) -> AsyncTodolistEdit:
        self._load(await self._service.get(id=self._id))
        return self

    async def __aexit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = await self._service.replace(id=self._id, **_replace_kwargs(self._fields()))
            self._completed = True


def _overlay(fields: dict[str, Any], **updates: Any) -> dict[str, Any]:
    for key, value in updates.items():
        if value is not None:
            fields[key] = value
    return fields


class TodolistsService(_GeneratedTodolistsService):
    """Todolists service with merge-safe ``update`` and ``edit`` on top of the
    generated surface (``get``, ``replace``, ...)."""

    def update(
        self,
        *,
        id: int,
        name: str | None = None,
        description: str | None = None,
    ) -> dict[str, Any]:
        """Set the given fields on a todolist and preserve everything else.

        GETs the current todolist, overlays the explicitly-passed keyword
        arguments, and PUTs the full representation back. An omitted
        (``None``) field is untouched, guaranteed; an explicitly-passed ``""``
        clears the description.

        Not atomic: a concurrent write between the GET and PUT is overwritten
        (last write wins for the whole representation; the window is one
        round-trip). Use :meth:`replace` to overwrite deliberately, or
        :meth:`edit` to read the current values before deciding.
        """
        fields = _overlay(
            _fields_from_todolist(self.get(id=id)),
            name=name,
            description=description,
        )
        return self.replace(id=id, **_replace_kwargs(fields))

    def edit(self, *, id: int) -> TodolistEdit:
        """Open a read-modify-write edit of a todolist, as a context manager.

        Entering the ``with`` block GETs the current todolist and exposes its
        full writable state; exiting cleanly PUTs the whole representation
        back. Clearing the description means setting it empty (``""``). If the
        block raises, nothing is written. The updated todolist is available as
        ``.result`` after the block::

            with client.todolists.edit(id=123) as tl:
                tl.name = f"🚨 {tl.name}"
                tl.description = ""  # clearing = setting empty on a full object
            updated = tl.result

        Not atomic: a concurrent write between the GET and PUT is overwritten
        (last write wins for the whole representation; the window is one
        round-trip).
        """
        return TodolistEdit(self, id)


class AsyncTodolistsService(_GeneratedAsyncTodolistsService):
    """Async todolists service with merge-safe ``update`` and ``edit``."""

    async def update(
        self,
        *,
        id: int,
        name: str | None = None,
        description: str | None = None,
    ) -> dict[str, Any]:
        """Async twin of :meth:`TodolistsService.update`."""
        fields = _overlay(
            _fields_from_todolist(await self.get(id=id)),
            name=name,
            description=description,
        )
        return await self.replace(id=id, **_replace_kwargs(fields))

    def edit(self, *, id: int) -> AsyncTodolistEdit:
        """Async twin of :meth:`TodolistsService.edit`, for ``async with``::

        async with client.todolists.edit(id=123) as tl:
            tl.name = f"🚨 {tl.name}"
        updated = tl.result
        """
        return AsyncTodolistEdit(self, id)
