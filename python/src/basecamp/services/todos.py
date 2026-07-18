"""Todos service with merge-safe ``update`` and read-modify-write ``edit``.

Both compose the public ``get`` and ``replace`` methods, so hooks observe
the two wire operations (``get`` then ``replace``), not a synthetic
composite.

Neither is atomic: there is no conditional-update signal on this endpoint,
so a concurrent write between the GET and PUT is overwritten — last write
wins for the whole representation. The window is one round-trip. Use
``replace`` to overwrite deliberately.
"""

from __future__ import annotations

from typing import Any

from basecamp.errors import UsageError
from basecamp.generated.services.todos import AsyncTodosService as _GeneratedAsyncTodosService
from basecamp.generated.services.todos import TodosService as _GeneratedTodosService


def _fields_from_todo(todo: dict[str, Any]) -> dict[str, Any]:
    """Derive a todo's full writable state from a GET response."""
    return {
        "content": todo.get("content") or "",
        "description": todo.get("description") or "",
        "assignee_ids": [p["id"] for p in todo.get("assignees") or []],
        "completion_subscriber_ids": [p["id"] for p in todo.get("completion_subscribers") or []],
        "due_on": todo.get("due_on") or "",
        "starts_on": todo.get("starts_on") or "",
        # Send directive, not todo state: never populated from the current
        # todo; sent only when True.
        "notify": False,
    }


def _replace_kwargs(fields: dict[str, Any]) -> dict[str, Any]:
    """Serialize full writable state for the replace transport.

    Content, description, and both ID lists are always sent (empties
    included, so clears survive the PUT); dates are sent only when
    non-empty (the server clears an omitted date, and ``""`` is a format
    error); notify is sent only when True.
    """
    for key in ("assignee_ids", "completion_subscriber_ids"):
        if fields[key] is None:
            raise UsageError(f"{key} must be a list of person IDs; use [] to clear — a full write has no None state")
    return {
        "content": fields["content"],
        "description": fields["description"],
        "assignee_ids": list(fields["assignee_ids"]),
        "completion_subscriber_ids": list(fields["completion_subscriber_ids"]),
        "due_on": fields["due_on"] or None,
        "starts_on": fields["starts_on"] or None,
        "notify": True if fields["notify"] else None,
    }


class _TodoEditBase:
    """Shared writable state for :class:`TodoEdit` / :class:`AsyncTodoEdit`.

    Inside the ``with`` block the edit object exposes the todo's full
    writable state: ``content``, ``description``, ``assignee_ids``,
    ``completion_subscriber_ids``, ``due_on``, ``starts_on``, and
    ``notify``. Clearing a field means setting it empty (``""`` for
    strings and dates, ``[]`` for ID lists) — an untouched field keeps its
    current value.
    """

    content: str
    description: str
    assignee_ids: list[int]
    completion_subscriber_ids: list[int]
    due_on: str
    starts_on: str
    notify: bool

    def __init__(self, todo_id: int) -> None:
        self._todo_id = todo_id
        self._result: dict[str, Any] | None = None
        self._completed = False

    def _load(self, todo: dict[str, Any]) -> None:
        for key, value in _fields_from_todo(todo).items():
            setattr(self, key, value)

    def _fields(self) -> dict[str, Any]:
        return {
            "content": self.content,
            "description": self.description,
            "assignee_ids": self.assignee_ids,
            "completion_subscriber_ids": self.completion_subscriber_ids,
            "due_on": self.due_on,
            "starts_on": self.starts_on,
            "notify": self.notify,
        }

    @property
    def result(self) -> dict[str, Any]:
        """The updated todo, available after the ``with`` block exits cleanly."""
        if not self._completed:
            raise RuntimeError("edit has not completed")
        assert self._result is not None
        return self._result


class TodoEdit(_TodoEditBase):
    """Read-modify-write context manager returned by :meth:`TodosService.edit`.

    Entering the block GETs the current todo; exiting cleanly PUTs the
    whole representation back. If the block raises, the edit aborts and
    nothing is written.
    """

    def __init__(self, service: TodosService, todo_id: int) -> None:
        super().__init__(todo_id)
        self._service = service

    def __enter__(self) -> TodoEdit:
        self._load(self._service.get(todo_id=self._todo_id))
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = self._service.replace(todo_id=self._todo_id, **_replace_kwargs(self._fields()))
            self._completed = True


class AsyncTodoEdit(_TodoEditBase):
    """Async twin of :class:`TodoEdit`, for ``async with``."""

    def __init__(self, service: AsyncTodosService, todo_id: int) -> None:
        super().__init__(todo_id)
        self._service = service

    async def __aenter__(self) -> AsyncTodoEdit:
        self._load(await self._service.get(todo_id=self._todo_id))
        return self

    async def __aexit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = await self._service.replace(todo_id=self._todo_id, **_replace_kwargs(self._fields()))
            self._completed = True


def _overlay(fields: dict[str, Any], **updates: Any) -> dict[str, Any]:
    for key, value in updates.items():
        if value is not None:
            fields[key] = value
    return fields


class TodosService(_GeneratedTodosService):
    """Todos service with merge-safe ``update`` and ``edit`` on top of the
    generated surface (``get``, ``replace``, ...)."""

    def update(
        self,
        *,
        todo_id: int,
        content: str | None = None,
        description: str | None = None,
        assignee_ids: list | None = None,
        completion_subscriber_ids: list | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Set the given fields on a todo and preserve everything else.

        GETs the current todo, overlays the explicitly-passed keyword
        arguments, and PUTs the full representation back. An omitted
        (``None``) field is untouched, guaranteed; an explicitly-passed
        empty list clears.

        Not atomic: a concurrent write between the GET and PUT is
        overwritten (last write wins for the whole representation; the
        window is one round-trip). Use :meth:`replace` to overwrite
        deliberately, or :meth:`edit` to clear fields.
        """
        fields = _overlay(
            _fields_from_todo(self.get(todo_id=todo_id)),
            content=content,
            description=description,
            assignee_ids=assignee_ids,
            completion_subscriber_ids=completion_subscriber_ids,
            notify=notify,
            due_on=due_on,
            starts_on=starts_on,
        )
        return self.replace(todo_id=todo_id, **_replace_kwargs(fields))

    def edit(self, *, todo_id: int) -> TodoEdit:
        """Open a read-modify-write edit of a todo, as a context manager.

        Entering the ``with`` block GETs the current todo and exposes its
        full writable state; exiting cleanly PUTs the whole representation
        back. Clearing a field means setting it empty (``""`` / ``[]``).
        If the block raises, nothing is written. The updated todo is
        available as ``.result`` after the block::

            with client.todos.edit(todo_id=123) as t:
                t.content = f"🚨 {t.content}"
                t.due_on = ""  # clearing = setting empty on a full object
            updated = t.result

        Not atomic: a concurrent write between the GET and PUT is
        overwritten (last write wins for the whole representation; the
        window is one round-trip).
        """
        return TodoEdit(self, todo_id)


class AsyncTodosService(_GeneratedAsyncTodosService):
    """Async todos service with merge-safe ``update`` and ``edit``."""

    async def update(
        self,
        *,
        todo_id: int,
        content: str | None = None,
        description: str | None = None,
        assignee_ids: list | None = None,
        completion_subscriber_ids: list | None = None,
        notify: bool | None = None,
        due_on: str | None = None,
        starts_on: str | None = None,
    ) -> dict[str, Any]:
        """Async twin of :meth:`TodosService.update`."""
        fields = _overlay(
            _fields_from_todo(await self.get(todo_id=todo_id)),
            content=content,
            description=description,
            assignee_ids=assignee_ids,
            completion_subscriber_ids=completion_subscriber_ids,
            notify=notify,
            due_on=due_on,
            starts_on=starts_on,
        )
        return await self.replace(todo_id=todo_id, **_replace_kwargs(fields))

    def edit(self, *, todo_id: int) -> AsyncTodoEdit:
        """Async twin of :meth:`TodosService.edit`, for ``async with``::

        async with client.todos.edit(todo_id=123) as t:
            t.content = f"🚨 {t.content}"
        updated = t.result
        """
        return AsyncTodoEdit(self, todo_id)
