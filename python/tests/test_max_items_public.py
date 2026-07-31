"""Public ``max_items`` coverage across the three paginator families.

Every paginated list method now exposes a keyword-only ``max_items`` cap,
matching the ``maxItems`` pagination option in the other SDKs. These tests
drive the cap through the public generated surface where one exists:

- plain (bare-array pages): ``projects.list(max_items=...)``
- wrapped (envelope + item key): ``reports.person_progress(..., max_items=...)``

The key family (``_paginate_key``) has no generated public caller today, so
its cap semantics are pinned on the private helper directly, as
test_paginate_truncated.py does for its page-cap cases.

Semantics under test (SPEC.md steps 4a/6g): collection stops as soon as the
cap is met — no further pages are fetched — and ``truncated`` is True only
when items were actually dropped or a next Link was left unfetched. A cap
landing exactly on the final item of a Link-less last page is NOT truncated.
"""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client

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


_PROJECTS_URL = f"{_ACCOUNT_URL}/projects.json"
_PROGRESS_PATH = "/reports/users/progress/1.json"
_PROGRESS_URL = f"{_ACCOUNT_URL}{_PROGRESS_PATH}"


class TestPlainListMaxItems:
    @respx.mock
    def test_cap_stops_pagination_early_and_truncates(self):
        # Page 2 links onward to an unmounted page 3: reaching the cap there
        # must stop collection without following the link.
        page1, page2 = _mount_two_pages(_PROJECTS_URL, _items(1, 2), _items(3, 4), page2_link=True)

        account = Client(access_token="test-token").for_account("12345")
        result = account.projects.list(max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [p["id"] for p in result] == [1, 2, 3]
        assert result.meta.truncated is True

    @respx.mock
    def test_cap_on_exact_final_item_is_not_truncated(self):
        page1, page2 = _mount_two_pages(_PROJECTS_URL, _items(1, 2), _items(3))

        account = Client(access_token="test-token").for_account("12345")
        result = account.projects.list(max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is False


class TestAsyncPlainListMaxItems:
    @pytest.mark.asyncio
    @respx.mock
    async def test_cap_stops_pagination_early_and_truncates(self):
        page1, page2 = _mount_two_pages(_PROJECTS_URL, _items(1, 2), _items(3, 4), page2_link=True)

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.projects.list(max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [p["id"] for p in result] == [1, 2, 3]
        assert result.meta.truncated is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_cap_on_exact_final_item_is_not_truncated(self):
        page1, page2 = _mount_two_pages(_PROJECTS_URL, _items(1, 2), _items(3))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.projects.list(max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is False


class TestNonPositiveMaxItems:
    """Non-positive caps disable the cap, matching the other SDKs.

    TypeScript guards ``maxItems && maxItems > 0`` and Swift ``ln > 0``: zero
    and negative values mean "no cap", never a negative slice or an early stop.
    """

    @respx.mock
    def test_zero_collects_everything(self):
        page1, page2 = _mount_two_pages(_PROJECTS_URL, _items(1, 2), _items(3))

        account = Client(access_token="test-token").for_account("12345")
        result = account.projects.list(max_items=0)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [p["id"] for p in result] == [1, 2, 3]
        assert result.meta.truncated is False

    @respx.mock
    def test_negative_collects_everything(self):
        page1, page2 = _mount_two_pages(_PROJECTS_URL, _items(1, 2), _items(3))

        account = Client(access_token="test-token").for_account("12345")
        result = account.projects.list(max_items=-1)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [p["id"] for p in result] == [1, 2, 3]
        assert result.meta.truncated is False

    @pytest.mark.asyncio
    @respx.mock
    async def test_negative_collects_everything_async(self):
        page1, page2 = _mount_two_pages(_PROJECTS_URL, _items(1, 2), _items(3))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.projects.list(max_items=-1)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [p["id"] for p in result] == [1, 2, 3]
        assert result.meta.truncated is False


class TestKeyPaginateMaxItems:
    @respx.mock
    def test_cap_drops_items_and_stops_before_next_page(self):
        # Page 2 links onward to an unmounted page 3: the cap must break the
        # loop there, dropping the fourth item and never fetching page 3.
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3, 4)}, page2_link=True
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.reports._paginate_key(_PROGRESS_PATH, "events", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [e["id"] for e in result] == [1, 2, 3]
        assert result.meta.truncated is True

    @respx.mock
    def test_cap_on_exact_final_item_is_not_truncated(self):
        page1, page2 = _mount_two_pages(_PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3)})

        account = Client(access_token="test-token").for_account("12345")
        result = account.reports._paginate_key(_PROGRESS_PATH, "events", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is False


class TestAsyncKeyPaginateMaxItems:
    @pytest.mark.asyncio
    @respx.mock
    async def test_cap_drops_items_and_stops_before_next_page(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3, 4)}, page2_link=True
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.reports._paginate_key(_PROGRESS_PATH, "events", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [e["id"] for e in result] == [1, 2, 3]
        assert result.meta.truncated is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_cap_on_exact_final_item_is_not_truncated(self):
        page1, page2 = _mount_two_pages(_PROGRESS_URL, {"events": _items(1, 2)}, {"events": _items(3)})

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.reports._paginate_key(_PROGRESS_PATH, "events", max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert len(result) == 3
        assert result.meta.truncated is False


class TestWrappedMaxItems:
    @respx.mock
    def test_cap_met_on_first_page_stops_before_next_page(self):
        # The first page alone satisfies the cap; its next Link must be left
        # unfetched and the surplus item dropped.
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2, 3, 4)},
            {"events": _items(5)},
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.reports.person_progress(person_id=1, max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 0
        assert result["person"]["id"] == 7
        events = result["events"]
        assert [e["id"] for e in events] == [1, 2, 3]
        assert events.meta.truncated is True

    @respx.mock
    def test_cap_on_exact_final_item_is_not_truncated(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2)},
            {"events": _items(3)},
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.reports.person_progress(person_id=1, max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        events = result["events"]
        assert len(events) == 3
        assert events.meta.truncated is False


class TestAsyncWrappedMaxItems:
    @pytest.mark.asyncio
    @respx.mock
    async def test_cap_met_on_first_page_stops_before_next_page(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2, 3, 4)},
            {"events": _items(5)},
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.reports.person_progress(person_id=1, max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 0
        assert result["person"]["id"] == 7
        events = result["events"]
        assert [e["id"] for e in events] == [1, 2, 3]
        assert events.meta.truncated is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_cap_on_exact_final_item_is_not_truncated(self):
        page1, page2 = _mount_two_pages(
            _PROGRESS_URL,
            {"person": {"id": 7, "name": "Victor Cooper"}, "events": _items(1, 2)},
            {"events": _items(3)},
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.reports.person_progress(person_id=1, max_items=3)

        assert page1.call_count == 1
        assert page2.call_count == 1
        events = result["events"]
        assert len(events) == 3
        assert events.meta.truncated is False
