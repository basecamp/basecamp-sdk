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

The same URI addresses a to-do list or a group inside one — BC3 has no group
model, so a group is a ``Todolist`` whose parent is a ``Todolist`` — and since
#544 the spec models that as the single flat ``Todolist`` shape rather than a
``TodolistOrGroup`` union of three. Everything below is therefore
variant-agnostic and reads the body flat: no envelope, no arms, and no
branching on ``type``, which reads ``"Todolist"`` for both.
"""

from __future__ import annotations

from typing import Any

from basecamp._security import truncate as _truncate_message
from basecamp.errors import ApiError, UsageError
from basecamp.generated.services.todolists import AsyncTodolistsService as _GeneratedAsyncTodolistsService
from basecamp.generated.services.todolists import TodolistsService as _GeneratedTodolistsService


def _describe(value: object) -> str:
    """Render a value for an error message without ever throwing.

    The guard's own error path must not fail while explaining a failure: ``repr``
    is arbitrary user code and can raise. The type name is always available; the
    rendering is a bonus, capped per SPEC section 9 and dropped if it fails.
    """
    kind = type(value).__name__
    try:
        return f"{kind} {_truncate_message(repr(value))}"
    except Exception:
        return kind


def _require_mapping(body: object) -> dict[str, Any]:
    """The response must be a JSON object before any field is read.

    One level up from the malformed-*field* guards: a successful GET can return a
    scalar, a list, or null, and ``body.get("name")`` raises AttributeError on
    every one of those, so the body is checked before the fields.

    Since #544 this is the first of TWO levels rather than of three — body then
    scalar, with no union arm in between — which is a level fewer to guard, not
    a reason to stop guarding. The flat shape says what the API returns; it does
    not hand Python a decoder, and there is none under this read: the generated
    methods return ``dict[str, Any]`` verbatim. Making that structurally safe is
    tracked in #578.
    """
    if not isinstance(body, dict):
        raise ApiError(
            _truncate_message(f"GetTodolistOrGroup returned {_describe(body)} where a todolist object was expected"),
            hint=(
                "The merge-safe update/edit read this record's fields before rewriting them, "
                "so a non-object body cannot be used. Use replace() to write the record "
                "deliberately."
            ),
        )
    return body


def _fields_from_todolist(todolist: dict[str, Any]) -> dict[str, Any]:
    """Derive a todolist's full writable state from a GET response.

    BC3 answers this route with the recordable's FLAT JSON, and since #544 the
    spec agrees: ``Todolist``, ``TodolistGroup`` and the ``TodolistOrGroup``
    union were three declarations of one wire body and are now one structure.
    So the read is object -> scalar, with no arm to unwrap between them. The
    unwrap this used to do — and the conditional that kept a nested ``group``
    key from hijacking a flat body — went out together with the union that
    made either necessary; a ``group`` key in the body is now ordinary data
    that no code path looks at.

    Nothing here sniffs the variant, and nothing may. The same URI addresses a
    to-do list or a group inside one, BC3 renders both through
    ``todolists/_todolist.json.jbuilder``, and both report ``"type":
    "Todolist"``. They differ only structurally — ``groups_url`` when the
    parent is a todoset, ``group_position_url`` when it is a todolist — and
    they carry the same writable pair either way, so a group's description is
    preserved exactly as a list's is.
    """
    body = _require_mapping(todolist)
    return {
        "name": _writable_string(body, "name", non_empty=True),
        "description": _writable_string(body, "description"),
    }


def _writable_string(body: dict[str, Any], key: str, *, non_empty: bool = False) -> str:
    """Read a writable string field, refusing to coerce a malformed one.

    **Classification is by origin, not by value.** The same empty string is a
    caller error when the caller passed it and malformed response data when it
    came off the wire, so each provenance is checked where it is unambiguous:
    this read step owns the response, and :func:`_replace_kwargs` owns the
    caller. That is why an empty ``name`` here raises :class:`ApiError` while an
    empty ``name`` the caller supplied raises :class:`UsageError` — same value,
    different origin, different fault.

    **Presence and non-emptiness are two different claims, and only one of them
    is per-field.** Since #544 ``name`` and ``description`` are both
    ``@required`` and never null on this shape — ``format_api_content`` funnels
    a blank rich text through ``call_pipeline``, which returns ``""`` rather
    than nil — so for *both* an absent key and an explicit ``None`` are
    malformed and are refused here, before any PUT. Reading either as ``""``
    would put that ``""`` in the full-replace body and erase the record's real
    value on a call that never mentioned the field.

    ``non_empty`` is the *other* claim and holds for ``name`` alone: BC3
    presence-validates the attribute, so no real todolist carries an empty one
    and ``""`` off the wire is malformed too. ``description`` has no such
    validation — a description-less list carries ``""``, which is the ordinary
    case — so an empty description is a real value, preserved and resent
    verbatim.

    A wrong type is malformed either way and must NOT be coerced: a plain
    ``or ""`` turns every falsey non-string (``False``, ``0``, ``[]``, ``{}``)
    into ``""``, and this endpoint is full-replace, so that value would be
    written straight back over the real one — erasing the field these composites
    exist to preserve, on a call that never mentioned it.

    Python has no typed decoder between the GET and this read, unlike the Go,
    Swift and Kotlin composites where a wrong-typed field fails at decode. That
    makes the check explicit work here rather than something the layer below
    already did, and #544 did not change it: flattening the declared shape
    changes what the API returns, not what Python validates — ``get`` still
    hands back the parsed JSON as ``dict[str, Any]``. The same shape is live in
    the shipped Todos and Cards composites; that is tracked separately in #576,
    and giving Python a decoder at all in #578.
    """
    if key not in body:
        raise ApiError(
            f"Todolist field {key!r} is missing from the response",
            hint=(
                f"{key} is required on every todolist, so a body without one is a malformed "
                "response, not an empty value to preserve. The merge-safe update/edit PUT the "
                "full writable state back, so reading it as empty would erase the real value."
            ),
        )
    value = body[key]
    if value is None:
        raise ApiError(
            f"Todolist field {key!r} is null in the response",
            hint=(
                f"{key} is required and never null, so a null one is a malformed response, not "
                "an empty value to preserve. The merge-safe update/edit PUT the full writable "
                "state back, so reading it as empty would erase the real value."
            ),
        )
    if not isinstance(value, str):
        raise ApiError(
            _truncate_message(f"Todolist field {key!r} is not a string: {_describe(value)}"),
            hint=(
                "The merge-safe update/edit resend this field verbatim, so a coerced or "
                "empty value would overwrite the current one. Use replace() to write the "
                "record deliberately."
            ),
        )
    if non_empty and not value:
        raise ApiError(
            f"Todolist field {key!r} is empty in the response",
            hint=(
                f"{key} is presence-validated server-side, so an empty one is a malformed "
                "response. The caller did not ask to clear it."
            ),
        )
    return value


def _replace_kwargs(fields: dict[str, Any]) -> dict[str, Any]:
    """Serialize full writable state for the replace transport.

    Both keys always go out. ``description`` rides along even when empty:
    ``""`` is how a clear is expressed, and the generated layer's ``_compact``
    strips ``None``, so a ``None`` would leave the key off the wire entirely
    (SPEC §18 body compaction — never JSON null). ``name`` is refused when
    empty: BC3 presence-validates it, so a full write cannot clear it and an
    empty one is a 422 waiting to happen.
    """
    name = _caller_string(fields, "name")
    description = _caller_string(fields, "description")
    if not name:
        raise UsageError("name must be a non-empty string; BC3 presence-validates it, so a full write cannot clear it")
    return {"name": name, "description": description}


def _caller_string(fields: dict[str, Any], key: str) -> str:
    """Validate a caller-supplied writable value, the mirror of the read step.

    The read step (:func:`_writable_string`) owns *response* provenance; this
    owns *caller* provenance, and the two are the same rule seen from opposite
    ends. ``edit`` hands the caller a mutable view of the full writable state,
    and Python enforces nothing about what comes back — a closure that assigns
    ``42`` or ``[]`` would otherwise walk straight into the full-replace PUT and
    write it. That is caller misuse, hence :class:`UsageError`, where the same
    wrong type arriving from the server is an :class:`ApiError`.
    """
    value = fields[key]
    if not isinstance(value, str):
        raise UsageError(
            _truncate_message(f"todolist {key} must be a string, got {_describe(value)}"),
            hint=(
                "The full writable state is PUT back verbatim, so a non-string would be "
                "written to the record. Assign a string; use '' to clear."
            ),
        )
    return value


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
        if not self._completed or self._result is None:
            raise RuntimeError("edit has not completed")
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
