"""Tests for the cards merge-safe update surface (sync + async).

BC3 builds a card's update params as ``{ due_on: nil }.merge(card_params)``
(``kanban/cards_controller.rb``), so any update whose body omits ``due_on``
erases the card's due date. ``update`` reads first and resends it;
``update_verbatim`` is the raw single PUT.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client

BASE = "https://3.basecampapi.com/12345"


def _card(card_id: int = 42, **overrides) -> dict:
    card = {
        "id": card_id,
        "status": "active",
        "visible_to_clients": False,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-01-01T00:00:00Z",
        "title": "Ship it",
        "inherits_status": True,
        "type": "Kanban::Card",
        "url": f"{BASE}/card_tables/cards/{card_id}",
        "app_url": f"https://3.basecamp.com/12345/card_tables/cards/{card_id}",
        "due_on": "2024-02-01",
    }
    card.update(overrides)
    return card


def _put_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


def _sync_cards():
    return Client(access_token="test-token").for_account("12345").cards


def _async_cards():
    return AsyncClient(access_token="test-token").for_account("12345").cards


class TestSyncUpdate:
    @respx.mock
    def test_preserves_due_on_when_unaddressed(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        _sync_cards().update(card_id=42, title="Renamed")

        assert get_route.called, "the composite must read before writing"
        body = _put_body(put_route)
        assert body["due_on"] == "2024-02-01"
        assert body["title"] == "Renamed"
        # Never echoed back: BC3 filters ids through reachable_people.
        assert "assignee_ids" not in body
        assert "content" not in body

    @respx.mock
    def test_explicit_clear_omits_due_on_and_skips_the_read(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        _sync_cards().update(card_id=42, due_on="")

        assert not get_route.called, "an explicit clear needs no read"
        # Clearing is omission, never a literal null (SPEC section 18).
        assert "due_on" not in _put_body(put_route)

    @respx.mock
    def test_explicit_date_skips_the_read(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        _sync_cards().update(card_id=42, due_on="2026-09-01")

        assert not get_route.called
        assert _put_body(put_route)["due_on"] == "2026-09-01"

    @respx.mock
    def test_explicit_empty_content_and_assignees_are_sent(self):
        respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        _sync_cards().update(card_id=42, content="", assignee_ids=[], due_on="")

        body = _put_body(put_route)
        assert body["content"] == "", "an explicitly-empty content clears the body and must be sent"
        assert body["assignee_ids"] == [], "an empty list unassigns everyone and must be sent"


class TestSyncUpdateVerbatim:
    @respx.mock
    def test_sends_one_put_with_no_read(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        _sync_cards().update_verbatim(card_id=42, title="Renamed")

        assert not get_route.called, "verbatim must not read before writing"
        assert put_route.call_count == 1
        assert "due_on" not in _put_body(put_route)


class TestAsyncUpdate:
    @pytest.mark.asyncio
    @respx.mock
    async def test_preserves_due_on_when_unaddressed(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        await _async_cards().update(card_id=42, title="Renamed")

        assert get_route.called
        body = _put_body(put_route)
        assert body["due_on"] == "2024-02-01"
        assert "assignee_ids" not in body

    @pytest.mark.asyncio
    @respx.mock
    async def test_explicit_clear_omits_due_on_and_skips_the_read(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        await _async_cards().update(card_id=42, due_on="")

        assert not get_route.called
        assert "due_on" not in _put_body(put_route)

    @pytest.mark.asyncio
    @respx.mock
    async def test_verbatim_sends_one_put_with_no_read(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        await _async_cards().update_verbatim(card_id=42, title="Renamed")

        assert not get_route.called
        assert put_route.call_count == 1
