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

All four list-reading methods are covered, sync and async — the three paginators
(``_paginate``, ``_paginate_key``, ``_paginate_wrapped``) and the unpaginated
full-array read (``_request_list``) — and the method is not the unit that
matters. ``_paginate_key`` and ``_paginate_wrapped`` each decode on the first
page and again on every later page, from separate arms, so the rows go per ARM:
a guard on the first page alone leaves the original crash reachable behind any
``Link: rel="next"`` while the suite stays green. Every later-page arm therefore
carries its own rows, both directions, in both flavours.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp._decoding import decoded_array, decoded_object
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
        assert decoded_array(None, "the body") == []

    def test_an_array_decodes_to_itself(self):
        assert decoded_array([{"id": 1}], "the body") == [{"id": 1}]

    @pytest.mark.parametrize("value", _NOT_AN_ARRAY)
    def test_every_other_shape_fails_the_decode(self, value):
        with pytest.raises(ApiError, match="was not an array"):
            decoded_array(value, "the body")

    def test_the_refusal_names_the_shape_it_got(self):
        """``0`` and ``False`` are the pair ``value or []`` would read as an empty
        listing; the refusal has to tell them apart from ``null``, which is the
        one shape that legitimately reads as empty."""
        with pytest.raises(ApiError, match="was not an array: int"):
            decoded_array(0, "the body")
        with pytest.raises(ApiError, match="was not an array: bool"):
            decoded_array(False, "the body")

    def test_null_envelope_decodes_to_the_zero_struct(self):
        assert decoded_object(None, "the body") == {}

    @pytest.mark.parametrize("value", _NOT_AN_OBJECT)
    def test_every_other_envelope_shape_fails_the_decode(self, value):
        with pytest.raises(ApiError, match="was not an object"):
            decoded_object(value, "the body")


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

        assert "was not an array" in str(excinfo.value)
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
        assert "was not an array" in str(excinfo.value)


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

        assert "was not an array" in str(excinfo.value)

    @pytest.mark.asyncio
    @respx.mock
    async def test_null_on_a_later_page_keeps_the_earlier_pages(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json(None))
        respx.get(url).mock(return_value=_json([{"id": 1}], _link_to(f"{url}?page=2")))

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").projects.list()
        await client.close()

        assert list(result) == [{"id": 1}]

    @pytest.mark.asyncio
    @respx.mock
    async def test_wrong_typed_later_page_fails_the_read(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json("abc"))
        respx.get(url).mock(return_value=_json([{"id": 1}], _link_to(f"{url}?page=2")))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").projects.list()
        await client.close()

        assert "page 2" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)


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

        assert "was not an object" in str(excinfo.value)

    @pytest.mark.parametrize("value", _NOT_AN_ARRAY_UNDER_KEY)
    @respx.mock
    def test_wrong_typed_value_under_the_key_fails_the_read(self, value):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json({"events": value}))

        with pytest.raises(ApiError) as excinfo:
            _account().projects._paginate_key("/x.json", "events")

        assert "the 'events' list in the response" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)

    @respx.mock
    def test_null_on_a_later_page_keeps_the_earlier_pages(self):
        """The later-page decode is a separate arm from the first-page one, and a
        guard on only the first page leaves the original crash reachable behind
        any ``Link: rel="next"``."""
        url = f"{_ACCOUNT_URL}/x.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json(None))
        respx.get(url).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{url}?page=2")))

        assert list(_account().projects._paginate_key("/x.json", "events")) == [{"id": 1}]

    @respx.mock
    def test_wrong_typed_later_page_body_fails_the_read(self):
        url = f"{_ACCOUNT_URL}/x.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json([]))
        respx.get(url).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{url}?page=2")))

        with pytest.raises(ApiError) as excinfo:
            _account().projects._paginate_key("/x.json", "events")

        assert "page 2" in str(excinfo.value)
        assert "was not an object" in str(excinfo.value)

    @respx.mock
    def test_wrong_typed_later_page_value_under_the_key_fails_the_read(self):
        url = f"{_ACCOUNT_URL}/x.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json({"events": 0}))
        respx.get(url).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{url}?page=2")))

        with pytest.raises(ApiError) as excinfo:
            _account().projects._paginate_key("/x.json", "events")

        assert "page 2" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)


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

        assert "the 'events' list in the response" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)

    @pytest.mark.asyncio
    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    async def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(f"{_ACCOUNT_URL}/x.json").mock(return_value=_json(body))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").projects._paginate_key("/x.json", "events")
        await client.close()

        assert "was not an object" in str(excinfo.value)

    @pytest.mark.asyncio
    @respx.mock
    async def test_null_on_a_later_page_keeps_the_earlier_pages(self):
        url = f"{_ACCOUNT_URL}/x.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json(None))
        respx.get(url).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{url}?page=2")))

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").projects._paginate_key("/x.json", "events")
        await client.close()

        assert list(result) == [{"id": 1}]

    @pytest.mark.asyncio
    @respx.mock
    async def test_wrong_typed_later_page_body_fails_the_read(self):
        url = f"{_ACCOUNT_URL}/x.json"
        respx.get(url, params={"page": "2"}).mock(return_value=_json("abc"))
        respx.get(url).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{url}?page=2")))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").projects._paginate_key("/x.json", "events")
        await client.close()

        assert "page 2" in str(excinfo.value)
        assert "was not an object" in str(excinfo.value)


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

        assert "was not an object" in str(excinfo.value)

    @pytest.mark.parametrize("value", _NOT_AN_ARRAY_UNDER_KEY)
    @respx.mock
    def test_wrong_typed_value_under_the_key_fails_the_read(self, value):
        """A string here is the specific defect: ``list("abc")`` fabricates three
        single-character items out of a body that the reference refuses outright."""
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": value}))

        with pytest.raises(ApiError) as excinfo:
            _account().reports.person_progress(person_id=1)

        assert "the 'events' list in the response" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)

    @respx.mock
    def test_null_on_a_later_page_keeps_the_earlier_pages(self):
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json(None))
        respx.get(_PROGRESS_URL).mock(
            return_value=_json({"events": [{"id": 1}], "person": {"id": 9}}, _link_to(f"{_PROGRESS_URL}?page=2"))
        )

        result = _account().reports.person_progress(person_id=1)

        assert list(result["events"]) == [{"id": 1}]
        assert result["person"] == {"id": 9}

    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    def test_wrong_typed_later_page_body_fails_the_read(self, body):
        """The later page's ENVELOPE decode is its own arm: the wrong-typed-value
        rows below go through ``decoded_array`` at the key and never reach it."""
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json(body))
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{_PROGRESS_URL}?page=2")))

        with pytest.raises(ApiError) as excinfo:
            _account().reports.person_progress(person_id=1)

        assert "page 2" in str(excinfo.value)
        assert "was not an object" in str(excinfo.value)

    @respx.mock
    def test_wrong_typed_later_page_fails_the_read(self):
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json({"events": "abc"}))
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{_PROGRESS_URL}?page=2")))

        with pytest.raises(ApiError) as excinfo:
            _account().reports.person_progress(person_id=1)

        assert "page 2" in str(excinfo.value)
        assert "the 'events' list in the response" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)


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

        assert "the 'events' list in the response" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)

    @pytest.mark.asyncio
    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    async def test_wrong_typed_body_fails_the_read(self, body):
        respx.get(_PROGRESS_URL).mock(return_value=_json(body))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").reports.person_progress(person_id=1)
        await client.close()

        assert "was not an object" in str(excinfo.value)

    @pytest.mark.asyncio
    @respx.mock
    async def test_null_on_a_later_page_keeps_the_earlier_pages(self):
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json(None))
        respx.get(_PROGRESS_URL).mock(
            return_value=_json({"events": [{"id": 1}], "person": {"id": 9}}, _link_to(f"{_PROGRESS_URL}?page=2"))
        )

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").reports.person_progress(person_id=1)
        await client.close()

        assert list(result["events"]) == [{"id": 1}]
        assert result["person"] == {"id": 9}

    @pytest.mark.asyncio
    @pytest.mark.parametrize("body", _NOT_AN_OBJECT)
    @respx.mock
    async def test_wrong_typed_later_page_body_fails_the_read(self, body):
        respx.get(_PROGRESS_URL, params={"page": "2"}).mock(return_value=_json(body))
        respx.get(_PROGRESS_URL).mock(return_value=_json({"events": [{"id": 1}]}, _link_to(f"{_PROGRESS_URL}?page=2")))

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").reports.person_progress(person_id=1)
        await client.close()

        assert "page 2" in str(excinfo.value)
        assert "was not an object" in str(excinfo.value)

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
        assert "the 'events' list in the response" in str(excinfo.value)
        assert "was not an array" in str(excinfo.value)


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

        assert "was not an array" in str(excinfo.value)

    @respx.mock
    def test_an_undecodable_body_is_an_api_error_with_the_decoder_behind_it(self):
        """The two malformed-body classes at this site have to answer in the same
        taxonomy. Guarding the wrong-typed body while an undecodable one still
        escaped as a raw ``JSONDecodeError`` would leave the very builtin escape
        this change exists to close, at the site it edits — and the paginators
        have classified both as ``ApiError`` with a populated cause since #750."""
        respx.get(f"{_ACCOUNT_URL}/stacks.json").mock(
            return_value=httpx.Response(200, content='[{"id": 1', headers={"Content-Type": "application/json"})
        )

        with pytest.raises(ApiError) as excinfo:
            _account().folders.list_folders()

        assert "Failed to parse list response" in str(excinfo.value)
        assert isinstance(excinfo.value.cause, json.JSONDecodeError)


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

        assert "was not an array" in str(excinfo.value)

    @pytest.mark.asyncio
    @respx.mock
    async def test_an_undecodable_body_is_an_api_error_with_the_decoder_behind_it(self):
        respx.get(f"{_ACCOUNT_URL}/stacks.json").mock(
            return_value=httpx.Response(200, content='[{"id": 1', headers={"Content-Type": "application/json"})
        )

        client = AsyncClient(access_token="test-token")
        with pytest.raises(ApiError) as excinfo:
            await client.for_account("12345").folders.list_folders()
        await client.close()

        assert "Failed to parse list response" in str(excinfo.value)
        assert isinstance(excinfo.value.cause, json.JSONDecodeError)
