"""Documents service with merge-safe ``update`` and read-modify-write ``edit``.

``PUT /{accountId}/documents/{documentId}`` is a full replace: BC3's
``DocumentsController#update`` builds a brand-new ``Document`` from only the
permitted params and swaps the recordable wholesale, so a sparse PUT that
omits ``content`` erases it. Omitting ``title`` erases that too — the document
then reads back as ``"Untitled"``, because ``Document#title`` falls back when
blank. Neither attribute is presence-validated, so **neither omission is a
422**; both are a ``200`` that quietly clears. What BC3 does require is the
wrapping ``document`` object, so a body naming neither field is a ``400``.

Both composites compose the public ``get`` and ``replace`` methods, so hooks
observe the two wire operations (``get`` then ``replace``), not a synthetic
composite.

Neither is atomic: there is no conditional-update signal on this endpoint,
so a concurrent write between the GET and PUT is overwritten — last write
wins for the whole representation. The window is one round-trip. Use
``replace`` to overwrite deliberately.
"""

from __future__ import annotations

from typing import Any

from basecamp.generated.services.documents import (
    AsyncDocumentsService as _GeneratedAsyncDocumentsService,
)
from basecamp.generated.services.documents import DocumentsService as _GeneratedDocumentsService
from basecamp.services._merge_safe import (
    require_mapping,
    required_writable_string,
    writable_string,
)

_ESCAPE = "replace()"


def _fields_from_document(document: dict[str, Any]) -> dict[str, Any]:
    """Derive a document's full writable state from a GET response.

    Every value here is resent in the full-replace PUT, so every value is
    validated before it is read. A plain ``or ""`` would coerce each falsey
    non-string (``False``, ``0``, ``[]``, ``{}``) to ``""`` — erasing the field
    on a call that never mentioned it — and pass ``42``/``True`` straight
    through to be written verbatim. Python has no typed decoder between the GET
    and this read (``get`` returns ``dict[str, Any]``), so the check is
    explicit work here rather than something the layer below already did. See
    :mod:`basecamp.services._merge_safe` and #576.

    The two writable fields read differently because the spec models them
    differently: ``title`` is ``@required``, so absent or null is malformed;
    ``content`` is optional, so absent or null is a genuinely empty body.
    """
    body = require_mapping(document, record="Document", operation="GetDocument", escape=_ESCAPE)
    return {
        "title": required_writable_string(body, "title", record="Document", escape=_ESCAPE),
        "content": writable_string(body, "content", record="Document", escape=_ESCAPE),
    }


def _replace_kwargs(fields: dict[str, Any]) -> dict[str, Any]:
    """Serialize full writable state for the replace transport.

    Both fields are always sent, empties included: on a full-replace endpoint
    ``""`` is how a clear is expressed — never JSON null (SPEC section 18 body
    compaction), and never by omission, which would leave the field to the
    server's own clear-by-default and read as an accident rather than an
    intent.
    """
    return {"title": fields["title"], "content": fields["content"]}


class _DocumentEditBase:
    """Shared writable state for :class:`DocumentEdit` / :class:`AsyncDocumentEdit`.

    Inside the ``with`` block the edit object exposes the document's full
    writable state: ``title`` and ``content``. Clearing a field means setting
    it empty (``""``) — an untouched field keeps its current value.
    """

    title: str
    content: str

    def __init__(self, document_id: int) -> None:
        self._document_id = document_id
        self._result: dict[str, Any] | None = None
        self._completed = False

    def _load(self, document: dict[str, Any]) -> None:
        for key, value in _fields_from_document(document).items():
            setattr(self, key, value)

    def _fields(self) -> dict[str, Any]:
        return {"title": self.title, "content": self.content}

    @property
    def result(self) -> dict[str, Any]:
        """The updated document, available after the ``with`` block exits cleanly."""
        if not self._completed:
            raise RuntimeError("edit has not completed")
        assert self._result is not None
        return self._result


class DocumentEdit(_DocumentEditBase):
    """Read-modify-write context manager returned by :meth:`DocumentsService.edit`.

    Entering the block GETs the current document; exiting cleanly PUTs the
    whole representation back. If the block raises, the edit aborts and
    nothing is written.
    """

    def __init__(self, service: DocumentsService, document_id: int) -> None:
        super().__init__(document_id)
        self._service = service

    def __enter__(self) -> DocumentEdit:
        self._load(self._service.get(document_id=self._document_id))
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = self._service.replace(document_id=self._document_id, **_replace_kwargs(self._fields()))
            self._completed = True


class AsyncDocumentEdit(_DocumentEditBase):
    """Async twin of :class:`DocumentEdit`, for ``async with``."""

    def __init__(self, service: AsyncDocumentsService, document_id: int) -> None:
        super().__init__(document_id)
        self._service = service

    async def __aenter__(self) -> AsyncDocumentEdit:
        self._load(await self._service.get(document_id=self._document_id))
        return self

    async def __aexit__(self, exc_type: object, exc: object, tb: object) -> None:
        if exc_type is None:
            self._result = await self._service.replace(document_id=self._document_id, **_replace_kwargs(self._fields()))
            self._completed = True


def _overlay(fields: dict[str, Any], **updates: Any) -> dict[str, Any]:
    for key, value in updates.items():
        if value is not None:
            fields[key] = value
    return fields


class DocumentsService(_GeneratedDocumentsService):
    """Documents service with merge-safe ``update`` and ``edit`` on top of the
    generated surface (``get``, ``replace``, ...)."""

    def update(
        self,
        *,
        document_id: int,
        title: str | None = None,
        content: str | None = None,
    ) -> dict[str, Any]:
        """Set the given fields on a document and preserve everything else.

        GETs the current document, overlays the explicitly-passed keyword
        arguments, and PUTs the full representation back. An omitted
        (``None``) field is untouched, guaranteed; an explicitly-passed
        ``""`` clears.

        Not atomic: a concurrent write between the GET and PUT is
        overwritten (last write wins for the whole representation; the
        window is one round-trip). Use :meth:`replace` to overwrite
        deliberately.
        """
        fields = _overlay(
            _fields_from_document(self.get(document_id=document_id)),
            title=title,
            content=content,
        )
        return self.replace(document_id=document_id, **_replace_kwargs(fields))

    def edit(self, *, document_id: int) -> DocumentEdit:
        """Open a read-modify-write edit of a document, as a context manager.

        Entering the ``with`` block GETs the current document and exposes its
        full writable state; exiting cleanly PUTs the whole representation
        back. Clearing a field means setting it empty (``""``). If the block
        raises, nothing is written. The updated document is available as
        ``.result`` after the block::

            with client.documents.edit(document_id=123) as d:
                d.title = f"🚨 {d.title}"
                d.content = ""  # clearing = setting empty on a full object
            updated = d.result

        Not atomic: a concurrent write between the GET and PUT is
        overwritten (last write wins for the whole representation; the
        window is one round-trip).
        """
        return DocumentEdit(self, document_id)


class AsyncDocumentsService(_GeneratedAsyncDocumentsService):
    """Async documents service with merge-safe ``update`` and ``edit``."""

    async def update(
        self,
        *,
        document_id: int,
        title: str | None = None,
        content: str | None = None,
    ) -> dict[str, Any]:
        """Async twin of :meth:`DocumentsService.update`."""
        fields = _overlay(
            _fields_from_document(await self.get(document_id=document_id)),
            title=title,
            content=content,
        )
        return await self.replace(document_id=document_id, **_replace_kwargs(fields))

    def edit(self, *, document_id: int) -> AsyncDocumentEdit:
        """Async twin of :meth:`DocumentsService.edit`, for ``async with``::

        async with client.documents.edit(document_id=123) as d:
            d.title = f"🚨 {d.title}"
        updated = d.result
        """
        return AsyncDocumentEdit(self, document_id)
