"""The decode failure behind a malformed-body refusal stays reachable (#750).

A page body that is not JSON is Python's "the server sent something I could not
decode" refusal, and ``_base``/``_async_base`` classify it as a statusless
``api_error`` with ``raise ... from e``. What #750 adds is the NAME the other
five SDKs use for that slot, so a caller does not have to know to reach for
``__cause__`` to get what Go hands over through ``errors.As`` and Kotlin and
Swift through ``decodeFailure``.

Both paginators are exercised, sync and async: they are four separate ``except``
arms, and an arm that dropped its ``from e`` would leave the slot empty while
every other one stayed green.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ApiError

_ACCOUNT_URL = "https://3.basecampapi.com/12345"
_TRUNCATED_BODY = '[{"id": 1'


def _link_to(url: str) -> dict[str, str]:
    return {"Link": f'<{url}>; rel="next"'}


def _mount_bad_second_page(url: str) -> None:
    respx.get(url, params={"page": "2"}).mock(
        return_value=httpx.Response(200, content=_TRUNCATED_BODY, headers={"Content-Type": "application/json"})
    )
    respx.get(url).mock(
        return_value=httpx.Response(200, json=[{"id": 1, "title": "Item 1"}], headers=_link_to(f"{url}?page=2"))
    )


class TestPaginatedDecodeFailure:
    @respx.mock
    def test_bare_array_page_keeps_the_decoder_error(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        _mount_bad_second_page(url)

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(ApiError) as excinfo:
            account.projects._paginate("/projects.json")

        assert "page 2" in str(excinfo.value)
        assert isinstance(excinfo.value.cause, json.JSONDecodeError)

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_bare_array_page_keeps_the_decoder_error(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        _mount_bad_second_page(url)

        client = AsyncClient(access_token="test-token")
        account = client.for_account("12345")
        with pytest.raises(ApiError) as excinfo:
            await account.projects._paginate("/projects.json")
        await client.close()

        assert "page 2" in str(excinfo.value)
        assert isinstance(excinfo.value.cause, json.JSONDecodeError)


class TestCauseSlot:
    def test_cause_is_none_for_a_refusal_with_nothing_behind_it(self):
        """The malformed-*field* guards refuse a decodable body on their own
        terms, so there is no underlying failure and the slot is empty. That is
        the honest answer, and the same one Swift's ``decodeFailure`` gives for
        the pagination same-origin refusal."""
        assert ApiError("Basecamp returned a document with no title").cause is None
