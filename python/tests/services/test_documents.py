"""Tests for the documents merge-safe update / edit / replace surface (sync + async).

``PUT /documents/{id}`` is a full replace: BC3 rebuilds the Document from the
permitted params and swaps the recordable wholesale. The writable set is exactly
``{title, content}`` and **both are optional** — omitting ``title`` is a 200 that
leaves the document reading back as "Untitled", omitting ``content`` is a 200
that clears it. Neither omission is a 422, so nothing on the wire tells you the
sparse PUT went wrong; only the next GET does.

That is what ``update`` and ``edit`` exist to prevent, and what these tests pin:
every PUT they issue names both fields, empties included, never JSON null.
"""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ApiError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"


def load_fixture(rel: str) -> dict:
    return json.loads((_FIXTURES / rel).read_text(encoding="utf-8"))


def _document(document_id: int = 5001, **overrides) -> dict:
    # Source the full validated fixture for shape (every required Document field
    # is present), then keep the test-critical override values that the
    # assertions verify flow through to the PUT body.
    return {
        **load_fixture("documents/get.json"),
        "id": document_id,
        "title": "Project Overview",
        "content": "<div>The plan so far.</div>",
        **overrides,
    }


def _put_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[str] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(f"{info.service}.{info.operation}")


def _sync_documents(hooks: BasecampHooks | None = None):
    return Client(access_token="test-token", hooks=hooks).for_account("12345").documents


def _async_documents(hooks: BasecampHooks | None = None):
    return AsyncClient(access_token="test-token", hooks=hooks).for_account("12345").documents


def _routes(document: dict | None = None):
    body = _document() if document is None else document
    get_route = respx.get(f"{BASE}/documents/5001").mock(return_value=httpx.Response(200, json=body))
    put_route = respx.put(f"{BASE}/documents/5001").mock(return_value=httpx.Response(200, json=_document()))
    return get_route, put_route


class TestSyncUpdate:
    @respx.mock
    def test_merges_unset_content(self):
        get_route, put_route = _routes()

        result = _sync_documents().update(document_id=5001, title="Q3 Plan")

        assert result["id"] == 5001
        assert get_route.called
        body = _put_body(put_route)
        assert body["title"] == "Q3 Plan"
        # The field the caller never mentioned rides back verbatim. A sparse PUT
        # here would have been a silent 200 that erased it.
        assert body["content"] == "<div>The plan so far.</div>"

    @respx.mock
    def test_merges_unset_title(self):
        _, put_route = _routes()

        _sync_documents().update(document_id=5001, content="<div>Rewritten.</div>")

        body = _put_body(put_route)
        assert body["content"] == "<div>Rewritten.</div>"
        # Omitting title on the wire is a 200 that leaves the document titled
        # "Untitled" — never a 422 — so preservation is the only defence.
        assert body["title"] == "Project Overview"

    @respx.mock
    def test_explicit_empty_string_clears_content(self):
        _, put_route = _routes()

        _sync_documents().update(document_id=5001, content="")

        body = _put_body(put_route)
        assert body["content"] == ""
        assert "content" in body, "a clear is an empty string, never an omission"
        assert body["title"] == "Project Overview"

    @respx.mock
    def test_explicit_empty_string_clears_title(self):
        _, put_route = _routes()

        _sync_documents().update(document_id=5001, title="")

        body = _put_body(put_route)
        assert body["title"] == ""
        assert body["content"] == "<div>The plan so far.</div>"

    @respx.mock
    def test_never_sends_json_null(self):
        _, put_route = _routes()

        _sync_documents().update(document_id=5001, title="Q3 Plan", content="")

        body = _put_body(put_route)
        assert set(body) == {"title", "content"}
        assert all(value is not None for value in body.values())

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        _routes()

        hooks = _RecordingHooks()
        _sync_documents(hooks).update(document_id=5001, title="observed")

        assert hooks.operations == ["documents.get", "documents.replace"]


class TestSyncEdit:
    @respx.mock
    def test_edit_puts_full_state_back(self):
        _, put_route = _routes()

        with _sync_documents().edit(document_id=5001) as d:
            assert d.title == "Project Overview"
            assert d.content == "<div>The plan so far.</div>"
            d.title = f"🚨 {d.title}"

        assert d.result["id"] == 5001
        body = _put_body(put_route)
        assert body["title"] == "🚨 Project Overview"
        assert body["content"] == "<div>The plan so far.</div>"

    @respx.mock
    def test_clear_content_present_and_empty(self):
        _, put_route = _routes()

        with _sync_documents().edit(document_id=5001) as d:
            d.content = ""

        body = _put_body(put_route)
        # Present and empty, not omitted: on a full-replace endpoint an omission
        # is the server's own clear-by-default and reads as an accident.
        assert "content" in body
        assert body["content"] == ""
        assert body["title"] == "Project Overview"

    @respx.mock
    def test_clear_title_present_and_empty(self):
        _, put_route = _routes()

        with _sync_documents().edit(document_id=5001) as d:
            d.title = ""

        body = _put_body(put_route)
        assert "title" in body
        assert body["title"] == ""
        assert body["content"] == "<div>The plan so far.</div>"

    @respx.mock
    def test_exception_aborts_without_put(self):
        _, put_route = _routes()

        with pytest.raises(RuntimeError, match="abort"), _sync_documents().edit(document_id=5001) as d:
            d.content = "never written"
            raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    def test_result_raises_before_completion(self):
        _routes()

        edit = _sync_documents().edit(document_id=5001)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        _routes()

        hooks = _RecordingHooks()
        with _sync_documents(hooks).edit(document_id=5001) as d:
            d.title = "observed"

        assert hooks.operations == ["documents.get", "documents.replace"]


class TestSyncReplace:
    @respx.mock
    def test_sparse_replace_issues_no_get_and_omits_unset(self):
        get_route, put_route = _routes()

        result = _sync_documents().replace(document_id=5001, title="the whole new document")

        assert result["id"] == 5001
        assert not get_route.called, "replace is the deliberate overwrite — it reads nothing first"
        body = _put_body(put_route)
        assert body["title"] == "the whole new document"
        # Sent verbatim: the unset field is omitted and the server clears it.
        assert "content" not in body


class TestAsyncUpdate:
    @respx.mock
    @pytest.mark.asyncio
    async def test_merges_unset_content(self):
        get_route, put_route = _routes()

        result = await _async_documents().update(document_id=5001, title="Q3 Plan")

        assert result["id"] == 5001
        assert get_route.called
        body = _put_body(put_route)
        assert body["title"] == "Q3 Plan"
        assert body["content"] == "<div>The plan so far.</div>"

    @respx.mock
    @pytest.mark.asyncio
    async def test_explicit_empty_string_clears_content(self):
        _, put_route = _routes()

        await _async_documents().update(document_id=5001, content="")

        body = _put_body(put_route)
        assert "content" in body
        assert body["content"] == ""
        assert body["title"] == "Project Overview"

    @respx.mock
    @pytest.mark.asyncio
    async def test_hooks_observe_get_then_replace(self):
        _routes()

        hooks = _RecordingHooks()
        await _async_documents(hooks).update(document_id=5001, title="observed")

        assert hooks.operations == ["documents.get", "documents.replace"]


class TestAsyncEdit:
    @respx.mock
    @pytest.mark.asyncio
    async def test_edit_puts_full_state_back(self):
        _, put_route = _routes()

        async with _async_documents().edit(document_id=5001) as d:
            assert d.content == "<div>The plan so far.</div>"
            d.title = f"🚨 {d.title}"

        assert d.result["id"] == 5001
        body = _put_body(put_route)
        assert body["title"] == "🚨 Project Overview"
        assert body["content"] == "<div>The plan so far.</div>"

    @respx.mock
    @pytest.mark.asyncio
    async def test_clear_content_present_and_empty(self):
        _, put_route = _routes()

        async with _async_documents().edit(document_id=5001) as d:
            d.content = ""

        body = _put_body(put_route)
        assert "content" in body
        assert body["content"] == ""
        assert body["title"] == "Project Overview"

    @respx.mock
    @pytest.mark.asyncio
    async def test_exception_aborts_without_put(self):
        _, put_route = _routes()

        with pytest.raises(RuntimeError, match="abort"):
            async with _async_documents().edit(document_id=5001) as d:
                d.content = "never written"
                raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    @pytest.mark.asyncio
    async def test_result_raises_before_completion(self):
        _routes()

        edit = _async_documents().edit(document_id=5001)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result

    @respx.mock
    @pytest.mark.asyncio
    async def test_hooks_observe_get_then_replace(self):
        _routes()

        hooks = _RecordingHooks()
        async with _async_documents(hooks).edit(document_id=5001) as d:
            d.title = "observed"

        assert hooks.operations == ["documents.get", "documents.replace"]


class TestAsyncReplace:
    @respx.mock
    @pytest.mark.asyncio
    async def test_sparse_replace_issues_no_get(self):
        get_route, put_route = _routes()

        await _async_documents().replace(document_id=5001, title="verbatim")

        assert not get_route.called
        body = _put_body(put_route)
        assert body["title"] == "verbatim"
        assert "content" not in body


# --- #576: a malformed GET field must never reach the full-replace PUT -------
#
# `update`/`edit` GET the document, read each writable field, and PUT the FULL
# representation back. Every value read is therefore written, including one the
# caller never mentioned. A plain `body.get(key) or ""` coerces each falsey
# non-string to `""` (erasure) and passes `42`/`True` through verbatim
# (corruption). Python has no typed decoder between the GET and the read — the
# generated `get` returns `dict[str, Any]` — so the refusal is explicit.
#
# The assertion that matters is the ORDERING: exactly one request may leave the
# client. A guard that fires after the PUT has already lost the field.

_MALFORMED = [
    pytest.param(False, id="false"),
    pytest.param(0, id="zero"),
    pytest.param([], id="empty-list"),
    pytest.param({}, id="empty-dict"),
    pytest.param(42, id="number"),
    pytest.param(True, id="true"),
    pytest.param(["x"], id="list"),
    pytest.param({"a": 1}, id="dict"),
]

_WRITABLE_STRINGS = ["title", "content"]


class TestMalformedResponseFields:
    @respx.mock
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    @pytest.mark.parametrize("value", _MALFORMED)
    def test_update_refuses_a_non_string_before_writing(self, field, value):
        get_route, put_route = _routes(_document(**{field: value}))

        with pytest.raises(ApiError) as excinfo:
            _sync_documents().update(document_id=5001, title="New title")

        assert f"Document field {field!r} is not a string" in str(excinfo.value)
        # api_error, not usage: the value arrived in a successful response.
        assert excinfo.value.code == "api_error"
        assert get_route.called
        assert not put_route.called, "the guard must fire BEFORE the full-replace PUT"
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    def test_edit_refuses_a_non_string_before_writing(self, field):
        get_route, put_route = _routes(_document(**{field: 42}))

        with pytest.raises(ApiError) as excinfo, _sync_documents().edit(document_id=5001) as d:
            d.title = "New title"

        assert f"Document field {field!r} is not a string" in str(excinfo.value)
        assert get_route.called
        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    @pytest.mark.asyncio
    async def test_async_update_refuses_a_non_string_before_writing(self, field):
        get_route, put_route = _routes(_document(**{field: ["x"]}))

        with pytest.raises(ApiError) as excinfo:
            await _async_documents().update(document_id=5001, title="New title")

        assert f"Document field {field!r} is not a string" in str(excinfo.value)
        assert get_route.called
        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    @pytest.mark.asyncio
    async def test_async_edit_refuses_a_non_string_before_writing(self, field):
        _, put_route = _routes(_document(**{field: 42}))

        with pytest.raises(ApiError):
            async with _async_documents().edit(document_id=5001) as d:
                d.title = "New title"

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("absent", [True, False], ids=["absent", "null"])
    def test_absent_and_null_content_stays_genuinely_empty(self, absent):
        # The other half of the rule: for an OPTIONAL field, absent and null
        # are not malformed, they are empty. Guarding types must not turn a
        # legitimately blank field into an error. The call sets the other
        # writable string so that ``content`` is never overwritten by the
        # caller.
        #
        # ``content`` only. ``title`` is ``@required`` in the spec and gets the
        # opposite treatment below.
        document = _document()
        if absent:
            document.pop("content", None)
        else:
            document["content"] = None
        _, put_route = _routes(document)

        _sync_documents().update(document_id=5001, title="set by the caller")

        body = _put_body(put_route)
        assert body["content"] == ""
        assert body["title"] == "set by the caller"

    # ``Document.title`` is ``@required`` in the spec, and BC3 can never render
    # it blank (``Document#title`` is ``super.presence or "Untitled"``). So an
    # absent or null title in a 2xx body is a MALFORMED RESPONSE, not an empty
    # title — and coalescing it to ``""`` would blank the real title on a call
    # that only touched ``content``. Same defect class as a forwarded
    # non-string, in the one shape ``or ""`` looks correct.
    @respx.mock
    # BC3 can never render a blank title, so "" is malformed too — and it is
    # the shape a missing/null check alone would let through.
    @pytest.mark.parametrize("mangle", ["absent", "null", "blank"])
    def test_update_refuses_an_absent_title_before_writing(self, mangle):
        document = _document()
        if mangle == "absent":
            document.pop("title", None)
        else:
            document["title"] = None if mangle == "null" else ""
        _, put_route = _routes(document)

        with pytest.raises(ApiError, match=r'field "title" is required'):
            _sync_documents().update(document_id=5001, content="<div>New body.</div>")

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("mangle", ["absent", "null", "blank"])
    def test_edit_refuses_an_absent_title_before_writing(self, mangle):
        document = _document()
        if mangle == "absent":
            document.pop("title", None)
        else:
            document["title"] = None if mangle == "null" else ""
        _, put_route = _routes(document)

        with (
            pytest.raises(ApiError, match=r'field "title" is required'),
            _sync_documents().edit(document_id=5001) as d,
        ):
            d.content = "<div>New body.</div>"

        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("raw", [b"[]", b'"document"', b"42", b"null", b"true"])
    def test_update_refuses_a_non_object_response_before_writing(self, raw):
        # One level up from the field guards: a successful GET can return a
        # scalar, a list or null, and `body.get(key)` would raise a raw
        # AttributeError instead of the documented statusless api_error.
        get_route = respx.get(f"{BASE}/documents/5001").mock(
            return_value=httpx.Response(200, content=raw, headers={"Content-Type": "application/json"})
        )
        put_route = respx.put(f"{BASE}/documents/5001").mock(return_value=httpx.Response(200, json=_document()))

        with pytest.raises(ApiError) as excinfo:
            _sync_documents().update(document_id=5001, title="New title")

        assert "GetDocument returned" in str(excinfo.value)
        assert excinfo.value.code == "api_error"
        assert get_route.called
        assert not put_route.called
        assert respx.calls.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("raw", [b"[]", b"null"])
    def test_edit_refuses_a_non_object_response_before_writing(self, raw):
        respx.get(f"{BASE}/documents/5001").mock(
            return_value=httpx.Response(200, content=raw, headers={"Content-Type": "application/json"})
        )
        put_route = respx.put(f"{BASE}/documents/5001").mock(return_value=httpx.Response(200, json=_document()))

        with pytest.raises(ApiError), _sync_documents().edit(document_id=5001) as d:
            d.title = "New title"

        assert not put_route.called
        assert respx.calls.call_count == 1
