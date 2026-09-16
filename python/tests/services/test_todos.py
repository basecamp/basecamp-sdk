"""Tests for the todos merge-safe update / edit / replace surface (sync + async)."""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client, Config
from basecamp.errors import ApiError, UsageError, ValidationError
from basecamp.hooks import BasecampHooks, OperationInfo

BASE = "https://3.basecampapi.com/12345"

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"


def load_fixture(rel: str) -> dict:
    return json.loads((_FIXTURES / rel).read_text(encoding="utf-8"))


def _todo(todo_id: int = 42, **overrides) -> dict:
    # Source the full validated fixture for shape (every required Todo field is
    # present), then keep the test-critical override values that the assertions
    # verify flow through to the PUT body.
    return {
        **load_fixture("todos/get.json"),
        "id": todo_id,
        "content": "Buy milk",
        "description": "<p>From the store</p>",
        "due_on": "2024-03-01",
        "starts_on": "2024-02-01",
        "assignees": [{"id": 100, "name": "Jane Doe"}],
        "completion_subscribers": [{"id": 555, "name": "Sub Scriber"}],
        "completed": False,
        **overrides,
    }


def _put_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


class _RecordingHooks(BasecampHooks):
    def __init__(self) -> None:
        self.operations: list[str] = []

    def on_operation_start(self, info: OperationInfo) -> None:
        self.operations.append(f"{info.service}.{info.operation}")


def _sync_todos(hooks: BasecampHooks | None = None):
    return Client(access_token="test-token", hooks=hooks).for_account("12345").todos


def _async_todos(hooks: BasecampHooks | None = None):
    return AsyncClient(access_token="test-token", hooks=hooks).for_account("12345").todos


class TestCreateTodosetTodo:
    """Loose to-dos: create directly under the project's to-do set (#12359)."""

    @respx.mock
    def test_creates_directly_under_the_todoset(self):
        route = respx.post(f"{BASE}/buckets/2/todosets/9/todos.json").mock(
            return_value=httpx.Response(201, json=_todo(1000, content="Loose task"))
        )

        todo = _sync_todos().create_todoset_todo(bucket_id=2, todoset_id=9, content="Loose task", assignee_ids=[1, 2])

        assert route.called
        body = json.loads(route.calls[-1].request.content)
        assert body["content"] == "Loose task"
        assert body["assignee_ids"] == [1, 2]
        assert todo["id"] == 1000

    @respx.mock
    def test_validation_error_surfaces(self):
        respx.post(f"{BASE}/buckets/2/todosets/9/todos.json").mock(
            return_value=httpx.Response(422, json={"error": "Content can't be blank"})
        )

        with pytest.raises(ValidationError):
            _sync_todos().create_todoset_todo(bucket_id=2, todoset_id=9, content="x")


class TestSyncUpdate:
    @respx.mock
    def test_merges_unset_fields(self):
        get_route = respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        result = _sync_todos().update(todo_id=42, content="Updated task")

        assert result["id"] == 42
        assert get_route.called
        body = _put_body(put_route)
        assert body["content"] == "Updated task"
        assert body["description"] == "<p>From the store</p>"
        assert body["due_on"] == "2024-03-01"
        assert body["starts_on"] == "2024-02-01"
        assert body["assignee_ids"] == [100]
        assert body["completion_subscriber_ids"] == [555]
        assert "notify" not in body

    @respx.mock
    def test_explicit_empty_list_clears(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        _sync_todos().update(todo_id=42, assignee_ids=[])

        body = _put_body(put_route)
        assert body["assignee_ids"] == []
        assert body["completion_subscriber_ids"] == [555]
        assert body["content"] == "Buy milk"

    @respx.mock
    def test_notify_only_when_true(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        _sync_todos().update(todo_id=42, content="ping", notify=True)

        assert _put_body(put_route)["notify"] is True

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        hooks = _RecordingHooks()
        _sync_todos(hooks).update(todo_id=42, content="observed")

        assert hooks.operations == ["todos.get", "todos.replace"]


class TestSyncEdit:
    @respx.mock
    def test_edit_puts_full_state_back(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with _sync_todos().edit(todo_id=42) as t:
            assert t.content == "Buy milk"
            t.content = f"🚨 {t.content}"

        assert t.result["id"] == 42
        body = _put_body(put_route)
        assert body["content"] == "🚨 Buy milk"
        assert body["description"] == "<p>From the store</p>"
        assert body["assignee_ids"] == [100]

    @respx.mock
    def test_clear_date_is_omitted_from_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with _sync_todos().edit(todo_id=42) as t:
            assert t.due_on == "2024-03-01"
            t.due_on = ""

        body = _put_body(put_route)
        assert "due_on" not in body
        assert body["content"] == "Buy milk"

    @respx.mock
    def test_clear_description_and_ids_present_and_empty(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with _sync_todos().edit(todo_id=42) as t:
            t.description = ""
            t.assignee_ids = []
            t.completion_subscriber_ids = []

        body = _put_body(put_route)
        assert body["description"] == ""
        assert body["assignee_ids"] == []
        assert body["completion_subscriber_ids"] == []

    @respx.mock
    def test_exception_aborts_without_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with pytest.raises(RuntimeError, match="abort"), _sync_todos().edit(todo_id=42) as t:
            t.content = "never written"
            raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    def test_none_id_list_raises_usage_error_without_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with pytest.raises(UsageError, match=r"use \[\] to clear"), _sync_todos().edit(todo_id=42) as t:
            t.assignee_ids = None

        assert not put_route.called

    @respx.mock
    def test_result_raises_before_completion(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        edit = _sync_todos().edit(todo_id=42)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result

    @respx.mock
    def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        hooks = _RecordingHooks()
        with _sync_todos(hooks).edit(todo_id=42) as t:
            t.content = "observed"

        assert hooks.operations == ["todos.get", "todos.replace"]


class TestSyncReplace:
    @respx.mock
    def test_sparse_replace_issues_no_get_and_omits_unset(self):
        get_route = respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        result = _sync_todos().replace(todo_id=42, content="the whole new todo")

        assert result["id"] == 42
        assert not get_route.called
        body = _put_body(put_route)
        assert body["content"] == "the whole new todo"
        for field in ("description", "assignee_ids", "completion_subscriber_ids", "notify", "due_on", "starts_on"):
            assert field not in body


class TestAsyncUpdate:
    @respx.mock
    @pytest.mark.asyncio
    async def test_merges_unset_fields(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        result = await _async_todos().update(todo_id=42, content="Updated task")

        assert result["id"] == 42
        body = _put_body(put_route)
        assert body["content"] == "Updated task"
        assert body["description"] == "<p>From the store</p>"
        assert body["assignee_ids"] == [100]
        assert body["completion_subscriber_ids"] == [555]

    @respx.mock
    @pytest.mark.asyncio
    async def test_hooks_observe_get_then_replace(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        hooks = _RecordingHooks()
        await _async_todos(hooks).update(todo_id=42, content="observed")

        assert hooks.operations == ["todos.get", "todos.replace"]


class TestAsyncEdit:
    @respx.mock
    @pytest.mark.asyncio
    async def test_edit_puts_full_state_back(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        async with _async_todos().edit(todo_id=42) as t:
            t.content = f"🚨 {t.content}"

        assert t.result["id"] == 42
        assert _put_body(put_route)["content"] == "🚨 Buy milk"

    @respx.mock
    @pytest.mark.asyncio
    async def test_exception_aborts_without_put(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with pytest.raises(RuntimeError, match="abort"):
            async with _async_todos().edit(todo_id=42) as t:
                t.content = "never written"
                raise RuntimeError("abort")

        assert not put_route.called

    @respx.mock
    @pytest.mark.asyncio
    async def test_result_raises_before_completion(self):
        respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        edit = _async_todos().edit(todo_id=42)
        with pytest.raises(RuntimeError, match="edit has not completed"):
            _ = edit.result


class TestAsyncReplace:
    @respx.mock
    @pytest.mark.asyncio
    async def test_sparse_replace_issues_no_get(self):
        get_route = respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        await _async_todos().replace(todo_id=42, content="verbatim")

        assert not get_route.called
        body = _put_body(put_route)
        assert body["content"] == "verbatim"
        assert "description" not in body


class TestDescriptionAttachments:
    @respx.mock
    def test_get_preserves_dimension_float_and_none(self):
        """A Todo's rich-text description is paired with a description_attachments
        array. Pixel dimensions arrive float-spelled (1024.0) for images and null
        for non-image blobs. The service returns the parsed response dict, so
        httpx/json preserves both the float and None verbatim — Python performs
        no int coercion. The generated TypedDict types these honestly as
        ``NotRequired[Optional[int | float]]`` (the schema is nullable and the
        FlexInt dimension may arrive as a float), so both the float value and
        None below are within the declared type. See SPEC.md §10 Type Fidelity.
        """
        todo = _todo(
            77,
            description_attachments=[
                {
                    "id": 1069480000,
                    "sgid": "BAh-img",
                    "filename": "leto-schematic.png",
                    "content_type": "image/png",
                    "byte_size": 284111,
                    "download_url": f"{BASE}/buckets/1/blobs/img/download/leto-schematic.png",
                    "width": 1024.0,
                    "height": 768,
                    "previewable": True,
                    "preview_url": f"{BASE}/buckets/1/blobs/img/previews/leto-schematic.png",
                    "thumbnail_url": f"{BASE}/buckets/1/blobs/img/thumbnails/leto-schematic.png",
                },
                {
                    "id": 1069480001,
                    "sgid": "BAh-pdf",
                    "filename": "leto-spec.pdf",
                    "content_type": "application/pdf",
                    "byte_size": 1048576,
                    "download_url": f"{BASE}/buckets/1/blobs/pdf/download/leto-spec.pdf",
                    "width": None,
                    "height": None,
                    "previewable": False,
                    "preview_url": f"{BASE}/buckets/1/blobs/pdf/previews/leto-spec.pdf",
                    "thumbnail_url": f"{BASE}/buckets/1/blobs/pdf/thumbnails/leto-spec.pdf",
                },
            ],
        )
        respx.get(f"{BASE}/todos/77").mock(return_value=httpx.Response(200, json=todo))

        result = _sync_todos().get(todo_id=77)
        attachments = result["description_attachments"]
        assert len(attachments) == 2

        # Float-spelled 1024.0 is preserved verbatim, as a Python float — the
        # runtime performs no integer coercion (unlike Go's FlexInt).
        assert attachments[0]["width"] == 1024
        assert isinstance(attachments[0]["width"], float)
        assert attachments[0]["height"] == 768
        # None is preserved verbatim despite the TypedDict's non-optional width.
        assert attachments[1]["width"] is None
        assert attachments[1]["height"] is None


# Instant retries: CompleteTodo's retry config would otherwise back off ~1s.
_FAST = Config(base_delay=0.0, max_jitter=0.0)


class TestCompleteRetriesIdempotentPost:
    """CompleteTodo is a POST flagged idempotent in metadata, so it must be
    retried on a transient 503. Driving the generated ``todos.complete`` service
    (not the raw HTTP client) exercises the metadata idempotency gate
    (``_is_retryable_operation``), so these fail if CompleteTodo's ``idempotent``
    flag is flipped off. Sync and async use separate retry loops (_http.py vs
    _async_http.py), so both are pinned. Regression guard for #439 / #417.
    """

    @respx.mock
    def test_sync_retries_on_503_then_succeeds(self):
        route = respx.post(f"{BASE}/todos/42/completion.json")
        route.side_effect = [httpx.Response(503), httpx.Response(204)]

        client = Client(access_token="test-token", config=_FAST)
        client.for_account("12345").todos.complete(todo_id=42)

        assert route.call_count == 2  # initial 503 + 1 retry that succeeds

    @respx.mock
    @pytest.mark.asyncio
    async def test_async_retries_on_503_then_succeeds(self):
        route = respx.post(f"{BASE}/todos/42/completion.json")
        route.side_effect = [httpx.Response(503), httpx.Response(204)]

        client = AsyncClient(access_token="test-token", config=_FAST)
        await client.for_account("12345").todos.complete(todo_id=42)

        assert route.call_count == 2  # initial 503 + 1 retry that succeeds


# --- #576: a malformed GET field must never reach the full-replace PUT -------
#
# `update`/`edit` GET the todo, read each writable field, and PUT the FULL
# representation back. Every value read is therefore written, including one the
# caller never mentioned. The old `todo.get(key) or ""` coerced each falsey
# non-string to `""` (erasure) and passed `42`/`True` through verbatim
# (corruption). Python has no typed decoder between the GET and the read — the
# generated `get` returns `dict[str, Any]` — so the refusal is explicit.
#
# The assertion that matters is the ORDERING: `put_route.called` must be False.
# A guard that fires after the PUT has already lost the field.

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

_WRITABLE_STRINGS = ["content", "description", "due_on", "starts_on"]

_ID_LISTS = ["assignees", "completion_subscribers"]

# The response key each id list is read from, and the request key it is written
# back to.
_ID_LIST_BODY_KEYS = {"assignees": "assignee_ids", "completion_subscribers": "completion_subscriber_ids"}


def _malformed_routes(todo: dict):
    get_route = respx.get(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=todo))
    put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))
    return get_route, put_route


class TestMalformedResponseFields:
    @respx.mock
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    @pytest.mark.parametrize("value", _MALFORMED)
    def test_update_refuses_a_non_string_before_writing(self, field, value):
        get_route, put_route = _malformed_routes(_todo(**{field: value}))

        with pytest.raises(ApiError) as excinfo:
            _sync_todos().update(todo_id=42, content="New title")

        assert f"Todo field {field!r} is not a string" in str(excinfo.value)
        # api_error, not usage: the value arrived in a successful response.
        assert excinfo.value.code == "api_error"
        assert get_route.called
        assert not put_route.called, "the guard must fire BEFORE the full-replace PUT"

    @respx.mock
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    def test_edit_refuses_a_non_string_before_writing(self, field):
        get_route, put_route = _malformed_routes(_todo(**{field: 42}))

        with pytest.raises(ApiError), _sync_todos().edit(todo_id=42) as t:
            t.content = "New title"

        assert get_route.called
        assert not put_route.called

    @respx.mock
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    @pytest.mark.asyncio
    async def test_async_update_refuses_a_non_string_before_writing(self, field):
        get_route, put_route = _malformed_routes(_todo(**{field: ["x"]}))

        with pytest.raises(ApiError):
            await _async_todos().update(todo_id=42, content="New title")

        assert get_route.called
        assert not put_route.called

    @respx.mock
    @pytest.mark.parametrize("absent", [True, False], ids=["absent", "null"])
    @pytest.mark.parametrize("field", _WRITABLE_STRINGS)
    def test_absent_and_null_stay_genuinely_empty(self, field, absent):
        # The other half of the rule: absent and null are not malformed, they
        # are empty. Guarding types must not turn a legitimately blank field
        # into an error. The call overlays an ID list rather than a string so
        # that no writable string under test is overwritten by the caller.
        todo = _todo()
        if absent:
            todo.pop(field, None)
        else:
            todo[field] = None
        _, put_route = _malformed_routes(todo)

        _sync_todos().update(todo_id=42, assignee_ids=[100])

        body = _put_body(put_route)
        if field in ("due_on", "starts_on"):
            # Dates ride only when non-empty: "" is a format error and an
            # omitted date is how the server clears one.
            assert field not in body
        else:
            assert body[field] == ""

    @respx.mock
    @pytest.mark.parametrize("value", _MALFORMED)
    @pytest.mark.parametrize("field", _ID_LISTS)
    def test_update_refuses_a_non_array_id_list_before_writing(self, field, value):
        if isinstance(value, list):
            pytest.skip("a list is checked element-wise, not by the array guard")
        get_route, put_route = _malformed_routes(_todo(**{field: value}))

        with pytest.raises(ApiError) as excinfo:
            _sync_todos().update(todo_id=42, content="New title")

        assert f"Todo field {field!r} is not an array" in str(excinfo.value)
        assert get_route.called
        assert not put_route.called

    @respx.mock
    @pytest.mark.parametrize("field", _ID_LISTS)
    def test_update_refuses_a_non_object_element_before_writing(self, field):
        get_route, put_route = _malformed_routes(_todo(**{field: ["nope"]}))

        with pytest.raises(ApiError) as excinfo:
            _sync_todos().update(todo_id=42, content="New title")

        assert f"Todo field {field!r}[0] is not an object" in str(excinfo.value)
        assert get_route.called
        assert not put_route.called

    @respx.mock
    @pytest.mark.parametrize("bad_id", [10.5, 10.0, None, True, [], {}, 2**63, -(2**63) - 1])
    @pytest.mark.parametrize("field", _ID_LISTS)
    def test_update_refuses_a_non_person_id_before_writing(self, field, bad_id):
        # The ID lists are resent in full, so a float, bool, null or
        # out-of-int64 id would be written as the complete assignee set. `True`
        # matters specifically: bool subclasses int in Python, so a naive
        # isinstance check passes it. `10.0` matters for the same reason one
        # level over: it is integral, and the reference still refuses it, because
        # `json.Number("10.0").Int64()` is a syntax error there.
        #
        # A STRING is not in this list and must not be: see
        # test_update_reads_a_string_person_id_the_way_the_reference_does.
        get_route, put_route = _malformed_routes(_todo(**{field: [{"id": bad_id, "name": "Jane"}]}))

        with pytest.raises(ApiError) as excinfo:
            _sync_todos().update(todo_id=42, content="New title")

        # Both lists are `Person` sites on GetTodo, so the generated read now
        # refuses the id at typed decode, as Go's `FlexibleInt64` does, before
        # the merge-safe guard ever sees it.
        assert f"GetTodo: person id at {field}.[] is not an int64" in str(excinfo.value)
        assert get_route.called
        assert not put_route.called

    # Measured through the reference's own todos Update composite: a Person's id
    # is the one field the generated model decodes flexibly (types.FlexibleInt64
    # via Person.Id), and `fieldsFromTodo` appends whatever that produced with no
    # filter at all. These ids reach this guard exactly as BC3 sent them whenever
    # the pre-decode normalizer did not find the person first — and which people
    # it finds is the walk's rule, not this guard's: 3 of 7 `assignees` people
    # in spec/fixtures omit the `personable_type` marker it keys on.
    @respx.mock
    @pytest.mark.parametrize(
        ("raw_id", "expected"),
        [
            ("1049715914", 1049715914),  # the live case: a bare numeric string
            ("007", 7),  # ParseInt takes leading zeros at base 10
            ("-5", -5),
            ("+5", 5),  # ParseInt takes a leading plus; a `^-?\\d+$` regex does not
            ("9223372036854775807", 2**63 - 1),
            ("-9223372036854775808", -(2**63)),
            ("basecamp", 0),  # the LocalPerson sentinel, read as the system actor
            ("", 0),
            (" 12", 0),  # Go trims nothing; int(" 12") would say 12
            ("0x10", 0),  # no base detection
            ("12.0", 0),
            ("1_0", 0),  # PEP 515 underscores are not Go digits
            ("\uff17", 0),  # fullwidth seven: Go tests the ASCII byte range
            ("18446744073709551615x", 0),  # syntax refusal wins, uint64 max intact
        ],
    )
    @pytest.mark.parametrize("field", _ID_LISTS)
    def test_update_reads_a_string_person_id_the_way_the_reference_does(self, field, raw_id, expected):
        get_route, put_route = _malformed_routes(_todo(**{field: [{"id": raw_id, "name": "Jane"}]}))

        _sync_todos().update(todo_id=42, content="New title")

        assert get_route.called
        assert _put_body(put_route)[_ID_LIST_BODY_KEYS[field]] == [expected]

    @respx.mock
    @pytest.mark.parametrize("raw_id", ["9223372036854775808", "18446744073709551616x"])
    @pytest.mark.parametrize("field", _ID_LISTS)
    def test_update_refuses_a_string_person_id_that_overflows_int64(self, field, raw_id):
        # The RANGE refusal and the SYNTAX refusal go opposite ways, and which
        # one fires depends on where the disqualifying byte sits.
        # "18446744073709551615x" is the system actor (0, above);
        # "18446744073709551616x" is one digit larger, overflows inside the scan
        # before the 'x' is ever reached, and fails the read.
        get_route, put_route = _malformed_routes(_todo(**{field: [{"id": raw_id, "name": "Jane"}]}))

        with pytest.raises(ApiError) as excinfo:
            _sync_todos().update(todo_id=42, content="New title")

        # Refused at the generated read's typed decode (see above).
        assert f"GetTodo: person id at {field}.[] is not an int64" in str(excinfo.value)
        assert get_route.called
        assert not put_route.called

    @respx.mock
    @pytest.mark.parametrize(
        ("people", "expected"),
        [
            ([{"name": "Jane"}], [0]),  # id absent is the zero value, not an error
            ([None], [0]),  # a null element decodes to the reference's zero Person
            ([{"id": 7}, {"id": "basecamp"}], [7, 0]),  # one real person, one system actor
        ],
    )
    @pytest.mark.parametrize("field", _ID_LISTS)
    def test_update_writes_back_the_person_shapes_the_reference_writes_back(self, field, people, expected):
        # An ABSENT id is 0 with no error while a NULL id fails the read: the
        # reference's flexible decoder is only called for a value that is there,
        # and its number path turns a JSON null into ParseInt(""), which fails.
        get_route, put_route = _malformed_routes(_todo(**{field: people}))

        _sync_todos().update(todo_id=42, content="New title")

        assert get_route.called
        assert _put_body(put_route)[_ID_LIST_BODY_KEYS[field]] == expected

    @respx.mock
    @pytest.mark.parametrize("raw", [b"[]", b'"todo"', b"42", b"null", b"true"])
    def test_update_refuses_a_non_object_response_before_writing(self, raw):
        # One level up from the field guards: a successful GET can return a
        # scalar, a list or null, and `body.get(key)` would raise a raw
        # AttributeError instead of the documented statusless api_error.
        get_route = respx.get(f"{BASE}/todos/42").mock(
            return_value=httpx.Response(200, content=raw, headers={"Content-Type": "application/json"})
        )
        put_route = respx.put(f"{BASE}/todos/42").mock(return_value=httpx.Response(200, json=_todo()))

        with pytest.raises(ApiError) as excinfo:
            _sync_todos().update(todo_id=42, content="New title")

        assert "GetTodo returned" in str(excinfo.value)
        assert get_route.called
        assert not put_route.called
