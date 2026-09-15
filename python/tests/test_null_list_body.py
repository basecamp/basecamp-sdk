"""A null list body is an empty listing; every other wrong-typed body fails the read.

Go hands each list response to ``json.Unmarshal`` against a typed destination, and
``null`` is a no-op there at any depth: it leaves the destination at its zero value
and returns no error. A null body, a null value under the item key, and an absent
item key are therefore *no rows* in the reference — not a failure. Everything else
of the wrong type fails the decode of the whole response.

Measured against the real reference, not asserted from memory. A linked Go oracle
calling ``encoding/json`` through the generated parsers
(``ParseGetProgressReportResponse`` for the array shape,
``ParseGetPersonProgressResponse`` for the envelope shape) reports:

    body                      array op                       envelope op
    ----                      --------                       -----------
    null                      len=0, no error                events len=0, no error
    0                         cannot unmarshal number        cannot unmarshal number
    false                     cannot unmarshal bool          cannot unmarshal bool
    "s"                       cannot unmarshal string        cannot unmarshal string
    {}                        cannot unmarshal object        events len=0, no error
    []                        len=0, no error                cannot unmarshal array
    [null]                    len=1                          cannot unmarshal array
    {"events": null}          cannot unmarshal object        events len=0, no error
    {"events": 0}             cannot unmarshal object        cannot unmarshal number
    {"events": "abc"}         cannot unmarshal object        cannot unmarshal string
    {"events": [null]}        cannot unmarshal object        events len=1

Both halves are pinned below, because only pinning the refusals would be satisfied
by an implementation that refused everything, and only pinning ``null`` would be
satisfied by ``items or []`` — which reads ``0`` and ``false`` as an empty listing
too, swapping a loud crash for a silent wrong answer. ``"abc"`` and ``{}`` earn
their own rows on the array sites for the same reason: Python's ``extend`` and
``list()`` accept both and fabricate items out of them (three characters, or the
envelope's keys) rather than raising.

All four list-reading sites are covered, sync and async: the three paginators
(``_paginate``, ``_paginate_key``, ``_paginate_wrapped``) and the unpaginated
full-array read (``_request_list``). They are eight separate decode sites, and a
site left unguarded would keep raising a bare builtin ``TypeError`` or
``AttributeError`` out of the SDK while every other one stayed green.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp._pagination import decode_envelope, decode_list
from basecamp.errors import ApiError

_ACCOUNT_URL = "https://3.basecampapi.com/12345"

# Wrong-typed bodies that must fail an ARRAY-shaped read. Go: "cannot unmarshal
# <kind> into Go value of type []T". ``{}`` and ``"abc"`` are the two Python would
# silently accept — ``extend`` walks a dict's keys and a string's characters.
_NOT_AN_ARRAY = [
    pytest.param(0, id="number"),
    pytest.param(False, id="boolean"),
    pytest.param("abc", id="string"),
    pytest.param({}, id="object"),
    pytest.param({"id": 1}, id="non-empty-object"),
]

# Wrong-typed bodies that must fail an ENVELOPE-shaped read. Go unmarshals these
# into a struct, so an array fails too.
_NOT_AN_OBJECT = [
    pytest.param(0, id="number"),
    pytest.param(False, id="boolean"),
    pytest.param("abc", id="string"),
    pytest.param([], id="array"),
    pytest.param([{"id": 1}], id="non-empty-array"),
]

# Wrong-typed values under the item key of an envelope. Go: "cannot unmarshal
# <kind> into Go struct field ....events of type []T".
_NOT_AN_ARRAY_UNDER_KEY = [
    pytest.param(0, id="number"),
    pytest.param(False, id="boolean"),
    pytest.param("abc", id="string"),
    pytest.param({"id": 1}, id="object"),
]


def _json(body, headers: dict[str, str] | None = None) -> httpx.Response:
    """Serialize the body explicitly.

    ``httpx.Response(200, json=None)`` sends an EMPTY body, not the four bytes
    ``null`` — which would exercise the decoder's refusal path instead of the
    null-body path these rows exist to pin.
    """
    return httpx.Response(
        200,
        content=json.dumps(body),
        headers={"Content-Type": "application/json", **(headers or {})},
    )


def _link_to(url: str) -> dict[str, str]:
    return {"Link": f'<{url}>; rel="next"'}


def _account() -> object:
    return Client(access_token="test-token").for_account("12345")


class TestDecodeHelpers:
    """The rule itself, named directly, one level below the eight call sites."""

    def test_null_decodes_to_no_rows(self):
        assert decode_list(None, where="list response") == []

    def test_an_array_decodes_to_itself(self):
        assert decode_list([{"id": 1}], where="list response") == [{"id": 1}]

    @pytest.mark.parametrize("value", _NOT_AN_ARRAY)
    def test_every_other_shape_fails_the_decode(self, value):
        with pytest.raises(ApiError, match="expected a JSON array"):
            decode_list(value, where="list response")

    def test_the_refusal_names_the_shape_it_got(self):
        """``0`` and ``False`` are the pair ``value or []`` would read as an empty
        listing; the message has to be able to tell them apart from ``null``."""
        with pytest.raises(ApiError, match="got number"):
            decode_list(0, where="list response")
        with pytest.raises(ApiError, match="got boolean"):
            decode_list(False, where="list response")

    def test_null_envelope_decodes_to_the_zero_struct(self):
        assert decode_envelope(None, where="list response") == {}

    @pytest.mark.parametrize("value", _NOT_AN_OBJECT)
    def test_every_other_envelope_shape_fails_the_decode(self, value):
        with pytest.raises(ApiError, match="expected a JSON object"):
            decode_envelope(value, where="list response")


class TestPaginateNullBody:
    """``_paginate`` — bare-array pages. Live through every plain list operation."""

    @respx.mock
    def test_null_body_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/projects.json").mock(return_value=_json(None))

        result = _account().projects.list()

        assert list(result) == []
        assert result.meta.truncated is False

    @respx.mock
    def test_empty_array_body_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/projects.json").mock(return_value=_json([]))

        assert list(_account().projects.list()) == []

    @respx.mock
    def test_a_null_element_is_kept_as_an_item(self):
        """Go's ``[null]`` decodes to one zero-valued element, not to no rows.
        The null-is-empty rule is about the list itself, not its members."""
        respx.get(f"{_ACCOUNT_URL}/projects.json").mock(return_value=_json([None]))

        assert list(_account().projects.list()) == [None]

    @pytest.mark.parametrize("body", _NOT_AN_ARRAY)
    @respx.mock
    def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(f"{_ACCOUNT_URL}/projects.json").mock(return_value=_json(body))

        with pytest.raises(ApiError) as excinfo:
            _account().projects.list()

        assert "expected a JSON array" in str(excinfo.value)
        assert "page 1" in str(excinfo.value)

    @respx.mock
    def test_null_on_a_later_page_keeps_the_earlier_pages(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json(None))
        respx.get(url).mock(return_value=_json([{"id": 1}], _link_to(f"{url}?page=2")))

        assert list(_account().projects.list()) == [{"id": 1}]

    @respx.mock
    def test_wrong_typed_later_page_fails_the_read(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json("abc"))
        respx.get(url).mock(return_value=_json([{"id": 1}], _link_to(f"{url}?page=2")))

        with pytest.raises(ApiError) as excinfo:
            _account().projects.list()

        assert "page 2" in str(excinfo.value)
        assert "expected a JSON array" in str(excinfo.value)


class TestAsyncPaginateNullBody:
    @pytest.mark.asyncio
    @respx.mock
    async def test_null_body_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/projects.json").mock(return_value=_json(None))

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").projects.list()
        await client.close()

        assert list(result) == []

    @pytest.mark.asyncio
    @pytest.mark.parametrize("body", _NOT_AN_ARRAY)
    @respx.mock
    async def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(f"{_ACCOUNT_URL}/projects.json").mock(return_value=_json(body))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").projects.list()
        await client.close()

        assert "expected a JSON array" in str(excinfo.value)


class TestPaginateKeyNullBody:
    """``_paginate_key`` — envelope pages, items only. No generated public caller
    today, so it is driven on the private helper, as test_paginate_truncated.py and
    test_max_items_public.py do for the same family."""

    @respx.mock
    def test_null_body_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json(None))

        assert list(_account().projects._paginate_key("/x.json", "events")) == []

    @respx.mock
    def test_absent_key_is_an_empty_listing(self):
        """Go's zero struct leaves the field nil when the key never appears."""
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json({"person": {"id": 9}}))

        assert list(_account().projects._paginate_key("/x.json", "events")) == []

    @respx.mock
    def test_null_under_the_key_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json({"events": None}))

        assert list(_account().projects._paginate_key("/x.json", "events")) == []

    @respx.mock
    def test_items_under_the_key_are_collected(self):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json({"events": [{"id": 1}]}))

        assert list(_account().projects._paginate_key("/x.json", "events")) == [{"id": 1}]

    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json(body))

        with pytest.raises(ApiError) as excinfo:
            _account().projects._paginate_key("/x.json", "events")

        assert "expected a JSON object" in str(excinfo.value)

    @pytest.mark.parametrize("value", _NOT_AN_ARRAY_UNDER_KEY)
    @respx.mock
    def test_wrong_typed_value_under_the_key_fails_the_read(self, value):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json({"events": value}))

        with pytest.raises(ApiError) as excinfo:
            _account().projects._paginate_key("/x.json", "events")

        assert "expected a JSON array at 'events'" in str(excinfo.value)


class TestAsyncPaginateKeyNullBody:
    @pytest.mark.asyncio
    @respx.mock
    async def test_null_body_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json(None))

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").projects._paginate_key("/x.json", "events")
        await client.close()

        assert list(result) == []

    @pytest.mark.asyncio
    @pytest.mark.parametrize("value", _NOT_AN_ARRAY_UNDER_KEY)
    @respx.mock
    async def test_wrong_typed_value_under_the_key_fails_the_read(self, value):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json({"events": value}))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").projects._paginate_key("/x.json", "events")
        await client.close()

        assert "expected a JSON array at 'events'" in str(excinfo.value)

    @pytest.mark.asyncio
    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    async def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json(body))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").projects._paginate_key("/x.json", "events")
        await client.close()

        assert "expected a JSON object" in str(excinfo.value)


_PROGRESS_URL = f"{_ACCOUNT_URL}/reports/users/progress/1.json"


class TestPaginateWrappedNullBody:
    """``_paginate_wrapped`` — envelope preserved alongside the items. Live through
    ``reports.person_progress``."""

    @respx.mock
    def test_null_body_is_an_empty_listing_under_the_key(self):
        respx.get(_PROGRESS_URL).mock(return_value=_json(None))

        result = _account().reports.person_progress(person_id=1)

        assert list(result["events"]) == []

    @respx.mock
    def test_absent_key_keeps_the_rest_of_the_envelope(self):
        respx.get(_PROGRESS_URL).mock(return_value=_json({"person": {"id": 9}}))

        result = _account().reports.person_progress(person_id=1)

        assert list(result["events"]) == []
        assert result["person"] == {"id": 9}

    @respx.mock
    def test_null_under_the_key_keeps_the_rest_of_the_envelope(self):
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": None, "person": {"id": 9}}))

        result = _account().reports.person_progress(person_id=1)

        assert list(result["events"]) == []
        assert result["person"] == {"id": 9}

    @respx.mock
    def test_a_null_element_is_kept_as_an_item(self):
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": [None]}))

        assert list(_account().reports.person_progress(person_id=1)["events"]) == [None]

    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(_PROGRESS_URL).mock(return_value=_json(body))

        with pytest.raises(ApiError) as excinfo:
            _account().reports.person_progress(person_id=1)

        assert "expected a JSON object" in str(excinfo.value)

    @pytest.mark.parametrize("value", _NOT_AN_ARRAY_UNDER_KEY)
    @respx.mock
    def test_wrong_typed_value_under_the_key_fails_the_read(self, value):
        """A string here is the specific defect: ``list("abc")`` fabricates three
        single-character items out of a body that the reference refuses outright."""
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": value}))

        with pytest.raises(ApiError) as excinfo:
            _account().reports.person_progress(person_id=1)

        assert "expected a JSON array at 'events'" in str(excinfo.value)

    @respx.mock
    def test_null_on_a_later_page_keeps_the_earlier_pages(self):
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json(None))
        respx.get(_PROGRESS_URL).mock(
            return_value=_json({"events": [{"id": 1}], "person": {"id": 9}}, _link_to(f"{_PROGRESS_URL}?page=2"))
        )

        result = _account().reports.person_progress(person_id=1)

        assert list(result["events"]) == [{"id": 1}]
        assert result["person"] == {"id": 9}

    @respx.mock
    def test_wrong_typed_later_page_fails_the_read(self):
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json({"events": "abc"}))
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{_PROGRESS_URL}?page=2")))

        with pytest.raises(ApiError) as excinfo:
            _account().reports.person_progress(person_id=1)

        assert "page 2" in str(excinfo.value)
        assert "expected a JSON array at 'events'" in str(excinfo.value)


class TestAsyncPaginateWrappedNullBody:
    @pytest.mark.asyncio
    @respx.mock
    async def test_null_body_is_an_empty_listing_under_the_key(self):
        respx.get(_PROGRESS_URL).mock(return_value=_json(None))

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").reports.person_progress(person_id=1)
        await client.close()

        assert list(result["events"]) == []

    @pytest.mark.asyncio
    @pytest.mark.parametrize("value", _NOT_AN_ARRAY_UNDER_KEY)
    @respx.mock
    async def test_wrong_typed_value_under_the_key_fails_the_read(self, value):
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": value}))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").reports.person_progress(person_id=1)
        await client.close()

        assert "expected a JSON array at 'events'" in str(excinfo.value)

    @pytest.mark.asyncio
    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    async def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(_PROGRESS_URL).mock(return_value=_json(body))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").reports.person_progress(person_id=1)
        await client.close()

        assert "expected a JSON object" in str(excinfo.value)

    @pytest.mark.asyncio
    @respx.mock
    async def test_wrong_typed_later_page_fails_the_read(self):
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json({"events": 0}))
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{_PROGRESS_URL}?page=2")))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").reports.person_progress(person_id=1)
        await client.close()

        assert "page 2" in str(excinfo.value)
        assert "expected a JSON array at 'events'" in str(excinfo.value)


class TestRequestListNullBody:
    """``_request_list`` — the unpaginated full-array read. Not a paginator, but the
    same array-shaped decode, and the same two builtin escapes: ``len(None)`` and
    ``ListResult("abc")``."""

    @respx.mock
    def test_null_body_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/stacks.json").mock(return_value=_json(None))

        result = _account().folders.list_folders()

        assert list(result) == []
        assert result.meta.total_count == 0

    @pytest.mark.parametrize("body", _NOT_AN_ARRAY)
    @respx.mock
    def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(f"{_ACCOUNT_URL}/stacks.json").mock(return_value=_json(body))

        with pytest.raises(ApiError) as excinfo:
            _account().folders.list_folders()

        assert "expected a JSON array" in str(excinfo.value)


class TestAsyncRequestListNullBody:
    @pytest.mark.asyncio
    @respx.mock
    async def test_null_body_is_an_empty_listing(self):
        respx.get(f"{_ACCOUNT_URL}/stacks.json").mock(return_value=_json(None))

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").folders.list_folders()
        await client.close()

        assert list(result) == []
        assert result.meta.total_count == 0

    @pytest.mark.asyncio
    @pytest.mark.parametrize("body", _NOT_AN_ARRAY)
    @respx.mock
    async def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(f"{_ACCOUNT_URL}/stacks.json").mock(return_value=_json(body))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").folders.list_folders()
        await client.close()

        assert "expected a JSON array" in str(excinfo.value)
