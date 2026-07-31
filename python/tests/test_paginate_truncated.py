"""Regression tests for issue #513: truncated-flag fidelity in the base pagination loops.

Two defects pinned here (SPEC.md steps 4a/6g mandate the precise form
``truncated = collected > cap OR current response has a next Link``):

1. Boundary false positive: ``_paginate`` (sync + async) set ``truncated=True``
   unconditionally when ``max_items`` was reached, even when the cap landed
   exactly on the final item of a Link-less last page (nothing dropped, no
   more pages).

2. Page-cap false negative: ``_paginate_key`` and ``_paginate_wrapped``
   (sync + async) never computed ``truncated`` at ``max_pages`` exhaustion, so
   a live next Link on the final fetched page still reported
   ``truncated=False``.

No public list method exposes ``max_items`` today, so the boundary cases
exercise the private helpers directly on a concrete service.
"""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.config import Config

_ACCOUNT_URL = "https://3.basecampapi.com/12345"


def _items(*ids: int) -> list[dict]:
    return [{"id": i, "title": f"Item {i}"} for i in ids]


def _link_to(url: str) -> dict[str, str]:
    return {"Link": f'<{url}>; rel="next"'}


def _mount_two_pages(url: str, page1_json, page2_json, *, page2_link: bool = False):
    """Mount page 1 (with a next Link to ?page=2) and page 2 on the same path.

    The ``?page=2`` route must be registered first so it wins for the follow-up
    request; the bare route then catches the initial (query-less) request.
    """
    page2_headers = _link_to(f"{url}?page=3") if page2_link else {}
    page2 = respx.get(url, params={"page": "2"}).mock(
        return_value=httpx.Response(200, json=page2_json, headers=page2_headers)
    )
    page1 = respx.get(url).mock(return_value=httpx.Response(200, json=page1_json, headers=_link_to(f"{url}?page=2")))
    return page1, page2


class TestPaginateMaxItemsBoundary:
    @respx.mock
    def test_truncated_false_when_cap_lands_exactly_on_final_linkless_page(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        page1, page2 = _mount_two_pages(url, _items(1, 2), _items(3))

        account = Client(access_token="test-token").for_account("12345")
        result = account.projects._paginate("/projects.json", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is False

    @respx.mock
    def test_truncated_true_when_cap_drops_items_mid_page(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        page1, page2 = _mount_two_pages(url, _items(1, 2), _items(3, 4))

        account = Client(access_token="test-token").for_account("12345")
        result = account.projects._paginate("/projects.json", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is True

    @respx.mock
    def test_truncated_true_when_cap_met_but_next_link_present(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        page1 = respx.get(url).mock(
            return_value=httpx.Response(200, json=_items(1, 2, 3), headers=_link_to(f"{url}?page=2"))
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.projects._paginate("/projects.json", max_items=3)

        assert page1.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is True


class TestAsyncPaginateMaxItemsBoundary:
    @pytest.mark.asyncio
    @respx.mock
    async def test_truncated_false_when_cap_lands_exactly_on_final_linkless_page(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        page1, page2 = _mount_two_pages(url, _items(1, 2), _items(3))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.projects._paginate("/projects.json", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is False

    @pytest.mark.asyncio
    @respx.mock
    async def test_truncated_true_when_cap_drops_items_mid_page(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        page1, page2 = _mount_two_pages(url, _items(1, 2), _items(3, 4))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.projects._paginate("/projects.json", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_truncated_true_when_cap_met_but_next_link_present(self):
        url = f"{_ACCOUNT_URL}/projects.json"
        page1 = respx.get(url).mock(
            return_value=httpx.Response(200, json=_items(1, 2, 3), headers=_link_to(f"{url}?page=2"))
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.projects._paginate("/projects.json", max_items=3)

        assert page1.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is True


_PROGRESS_PATH = "/reports/users/progress/1.json"
_PROGRESS_URL = f"{_ACCOUNT_URL}{_PROGRESS_PATH}"


class TestPaginateKeyPageCapTruncation:
    @respx.mock
    def test_truncated_true_when_max_pages_exhausted_with_live_next_link(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3, 4)}, page2_link=True
        )

        config = Config(max_pages=2)
        account = Client(access_token="test-token", config=config).for_account("12345")
        result = account.reports._paginate_key(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 4
        assert result.meta.truncated is True

    @respx.mock
    def test_truncated_false_when_final_page_has_no_next_link(self):
        page1, page2 = _mount_two_pages(_PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3, 4)})

        config = Config(max_pages=2)
        account = Client(access_token="test-token", config=config).for_account("12345")
        result = account.reports._paginate_key(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 4
        assert result.meta.truncated is False


class TestAsyncPaginateKeyPageCapTruncation:
    @pytest.mark.asyncio
    @respx.mock
    async def test_truncated_true_when_max_pages_exhausted_with_live_next_link(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3, 4)}, page2_link=True
        )

        config = Config(max_pages=2)
        account = AsyncClient(access_token="test-token", config=config).for_account("12345")
        result = await account.reports._paginate_key(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 4
        assert result.meta.truncated is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_truncated_false_when_final_page_has_no_next_link(self):
        page1, page2 = _mount_two_pages(_PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3, 4)})

        config = Config(max_pages=2)
        account = AsyncClient(access_token="test-token", config=config).for_account("12345")
        result = await account.reports._paginate_key(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 4
        assert result.meta.truncated is False


class TestPaginateWrappedPageCapTruncation:
    @respx.mock
    def test_truncated_true_when_max_pages_exhausted_with_live_next_link(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2)},
            {"events": _items(3, 4)},
            page2_link=True,
        )

        config = Config(max_pages=2)
        account = Client(access_token="test-token", config=config).for_account("12345")
        result = account.reports._paginate_wrapped(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert result["person"]["id"] == 7
        events = result["events"]
        assert len(events) == 4
        assert events.meta.truncated is True

    @respx.mock
    def test_truncated_false_when_final_page_has_no_next_link(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2)},
            {"events": _items(3, 4)},
        )

        config = Config(max_pages=2)
        account = Client(access_token="test-token", config=config).for_account("12345")
        result = account.reports._paginate_wrapped(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        events = result["events"]
        assert len(events) == 4
        assert events.meta.truncated is False


class TestAsyncPaginateWrappedPageCapTruncation:
    @pytest.mark.asyncio
    @respx.mock
    async def test_truncated_true_when_max_pages_exhausted_with_live_next_link(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2)},
            {"events": _items(3, 4)},
            page2_link=True,
        )

        config = Config(max_pages=2)
        account = AsyncClient(access_token="test-token", config=config).for_account("12345")
        result = await account.reports._paginate_wrapped(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert result["person"]["id"] == 7
        events = result["events"]
        assert len(events) == 4
        assert events.meta.truncated is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_truncated_false_when_final_page_has_no_next_link(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2)},
            {"events": _items(3, 4)},
        )

        config = Config(max_pages=2)
        account = AsyncClient(access_token="test-token", config=config).for_account("12345")
        result = await account.reports._paginate_wrapped(_PROGRESS_PATH, "events")

        assert page1.call_count == 1
        assert page2.call_count == 1
        events = result["events"]
        assert len(events) == 4
        assert events.meta.truncated is False
