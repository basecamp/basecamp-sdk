"""Tests for generated my_notifications service routes."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import Client


def _bubble_ups() -> list[dict]:
    return [
        {
            "id": 2,
            "created_at": "2026-07-21T00:01:43.009Z",
            "updated_at": "2026-07-21T00:01:43.031Z",
            "section": "bubbles",
            "unread_count": 0,
            "read_at": "2026-07-21T00:01:43.031Z",
            "title": "We won Leto!",
            "type": "Message",
            "bucket_name": "The Leto Laptop",
        },
        {
            "id": 3,
            "created_at": "2026-07-21T00:02:00.000Z",
            "updated_at": "2026-07-21T00:02:00.000Z",
            "section": "bubbles",
            "unread_count": 1,
            "title": "Scheduled follow-up",
            "type": "Todo",
            "bubble_up_at": "2026-08-01T00:00:00Z",
        },
    ]


class TestSyncMyNotifications:
    @respx.mock
    def test_get_bubble_ups_returns_parsed_readings(self):
        route = respx.get("https://3.basecampapi.com/12345/my/readings/bubble_ups.json").mock(
            return_value=httpx.Response(200, json=_bubble_ups())
        )

        account = Client(access_token="test-token").for_account("12345")
        result = account.my_notifications.get_bubble_ups()

        assert route.called
        assert len(result) == 2
        assert result[0]["id"] == 2
        assert result[0]["title"] == "We won Leto!"
        assert result[0]["type"] == "Message"
        assert result[1]["bubble_up_at"] == "2026-08-01T00:00:00Z"

    @respx.mock
    def test_get_bubble_ups_propagates_not_found(self):
        respx.get("https://3.basecampapi.com/12345/my/readings/bubble_ups.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        from basecamp.errors import NotFoundError

        account = Client(access_token="test-token").for_account("12345")
        with pytest.raises(NotFoundError):
            account.my_notifications.get_bubble_ups()
