"""Tests for the cards update surface (sync + async).

BC3 permits a card's JSON update params exactly as submitted (bc3#12521), so an
omitted ``due_on`` leaves the stored due date unchanged and only an explicit
``""`` or null clears it.

Both halves of that contract are pinned here. A clear must be spelled ``""`` on
the wire, because an omission now means "leave it alone" and would silently
no-op. And an unaddressed ``due_on`` must be a plain single PUT that omits the
key — no read-modify-write, which the old ``{ due_on: nil }.merge(card_params)``
behaviour required and this one does not.
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
    def test_leaves_due_on_off_the_wire_when_unaddressed(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        _sync_cards().update(card_id=42, title="Renamed")

        assert not get_route.called, "an unaddressed due_on is left alone by the server; nothing to read"
        assert put_route.call_count == 1, "one request, not a read-modify-write pair"
        body = _put_body(put_route)
        assert "due_on" not in body, "a key the body never mentions is never written"
        assert body["title"] == "Renamed"
        # Never echoed back: BC3 filters ids through reachable_people.
        assert "assignee_ids" not in body
        assert "content" not in body

    @respx.mock
    def test_explicit_clear_sends_an_empty_due_on(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        _sync_cards().update(card_id=42, due_on="")

        assert not get_route.called
        body = _put_body(put_route)
        # Clearing is an explicit empty string — never an omission, which BC3
        # reads as "unchanged", and never a literal null (SPEC section 18).
        assert "due_on" in body, "an omitted due_on no-ops against a presence-aware BC3"
        assert body["due_on"] == ""

    @respx.mock
    def test_explicit_date_is_sent(self):
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
        assert body["due_on"] == "", "an explicitly-empty due date clears it and must be sent"


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
    async def test_leaves_due_on_off_the_wire_when_unaddressed(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        await _async_cards().update(card_id=42, title="Renamed")

        assert not get_route.called
        assert put_route.call_count == 1
        body = _put_body(put_route)
        assert "due_on" not in body
        assert "assignee_ids" not in body

    @pytest.mark.asyncio
    @respx.mock
    async def test_explicit_clear_sends_an_empty_due_on(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        await _async_cards().update(card_id=42, due_on="")

        assert not get_route.called
        body = _put_body(put_route)
        assert "due_on" in body, "an omitted due_on no-ops against a presence-aware BC3"
        assert body["due_on"] == ""

    @pytest.mark.asyncio
    @respx.mock
    async def test_verbatim_sends_one_put_with_no_read(self):
        get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))
        put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(return_value=httpx.Response(200, json=_card()))

        await _async_cards().update_verbatim(card_id=42, title="Renamed")

        assert not get_route.called
        assert put_route.call_count == 1


# --- The due date the caller asked for is the one the card ends up with ------
#
# Everything above inspects the request body. These model the server that reads
# it and assert on the STORED value, so a wire spelling that parses fine but
# no-ops still fails.


class _PresenceAwareCards:
    """BC3: ``card_update_params`` is the submitted ``card_params``.

    A key the JSON body never mentions is never written, so an omitted
    ``due_on`` leaves the stored due date alone. An explicit ``""`` or null
    clears it — Rails casts a blank date to nil (bc3#12521).
    """

    def __init__(self, due_on: str | None = "2024-02-01") -> None:
        self.due_on = due_on

    def get(self, request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json=_card(due_on=self.due_on))

    def put(self, request: httpx.Request) -> httpx.Response:
        body = json.loads(request.content)
        if "due_on" in body:
            self.due_on = body["due_on"] or None
        return httpx.Response(200, json=_card(due_on=self.due_on))


def _serve(server: _PresenceAwareCards):
    get_route = respx.get(f"{BASE}/card_tables/cards/42").mock(side_effect=server.get)
    put_route = respx.put(f"{BASE}/card_tables/cards/42").mock(side_effect=server.put)
    return get_route, put_route


class TestAgainstAPresenceAwareServer:
    @respx.mock
    def test_explicit_clear_actually_clears_the_stored_due_date(self):
        server = _PresenceAwareCards()
        _serve(server)

        _sync_cards().update(card_id=42, due_on="")

        assert server.due_on is None, "the clear must land; an omitted due_on silently no-ops here"

    @respx.mock
    def test_an_unaddressed_update_keeps_the_stored_due_date_without_resending_it(self):
        server = _PresenceAwareCards()
        get_route, put_route = _serve(server)

        _sync_cards().update(card_id=42, title="Renamed")

        assert "due_on" not in _put_body(put_route), "the server preserves it; the SDK must not restate it"
        assert not get_route.called, "there is nothing to read back"
        assert server.due_on == "2024-02-01"

    @respx.mock
    def test_an_explicit_date_actually_lands(self):
        server = _PresenceAwareCards()
        _serve(server)

        _sync_cards().update(card_id=42, due_on="2026-09-01")

        assert server.due_on == "2026-09-01"

    @respx.mock
    def test_clearing_an_already_empty_due_date_is_a_no_op(self):
        server = _PresenceAwareCards(due_on=None)
        _serve(server)

        _sync_cards().update(card_id=42, due_on="")

        assert server.due_on is None

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_explicit_clear_actually_clears_the_stored_due_date(self):
        server = _PresenceAwareCards()
        _serve(server)

        await _async_cards().update(card_id=42, due_on="")

        assert server.due_on is None

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_unaddressed_update_keeps_the_stored_due_date(self):
        server = _PresenceAwareCards()
        get_route, put_route = _serve(server)

        await _async_cards().update(card_id=42, title="Renamed")

        assert "due_on" not in _put_body(put_route)
        assert not get_route.called
        assert server.due_on == "2024-02-01"
