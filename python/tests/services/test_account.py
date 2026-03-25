"""Tests for AccountService — verifies Account.logo and system actor normalization."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client


def _make_account(account_id: str = "12345"):
    c = Client(access_token="test-token")
    return c.for_account(account_id)


class TestGetAccount:
    @respx.mock
    def test_logo_deserialized_as_dict(self):
        respx.get("https://3.basecampapi.com/12345/account.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "id": 3,
                    "name": "37signals",
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                    "logo": {"url": "https://3.basecampapi.com/2914079/account/logo?v=1650492527"},
                },
            )
        )

        account = _make_account().account.get_account()

        assert isinstance(account["logo"], dict)
        assert account["logo"]["url"] == "https://3.basecampapi.com/2914079/account/logo?v=1650492527"

    @respx.mock
    def test_absent_logo_is_none(self):
        respx.get("https://3.basecampapi.com/12345/account.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "id": 3,
                    "name": "37signals",
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z",
                },
            )
        )

        account = _make_account().account.get_account()

        assert account.get("logo") is None


class TestUpdateAccountLogo:
    @respx.mock
    def test_sends_multipart_with_logo_field(self):
        route = respx.put("https://3.basecampapi.com/12345/account/logo.json").mock(return_value=httpx.Response(204))

        _make_account().account.update_account_logo(
            content=b"fake-png-data",
            filename="logo.png",
            content_type="image/png",
        )

        assert route.called
        request = route.calls[0].request
        assert b"logo" in request.content
        assert b"fake-png-data" in request.content
        assert request.headers["content-type"].startswith("multipart/form-data")

    @respx.mock
    def test_sanitizes_crlf_in_filename_and_content_type(self):
        route = respx.put("https://3.basecampapi.com/12345/account/logo.json").mock(return_value=httpx.Response(204))

        _make_account().account.update_account_logo(
            content=b"data",
            filename="evil\r\nContent-Type: text/html\r\n\r\nname.png",
            content_type="image/png\r\nX-Injected: true",
        )

        assert route.called
        raw = route.calls[0].request.content
        # CRLF stripped — the injected text is concatenated, not on a separate header line
        # "evil\r\nContent-Type: text/html\r\n\r\nname.png" → "evilContent-Type: text/htmlname.png"
        assert b"evilContent-Type" in raw  # concatenated, not a separate header
        # Content-Type value should not have CRLF creating a new header
        assert b"image/pngX-Injected" in raw  # concatenated, not injected


class TestAsyncUpdateAccountLogo:
    @pytest.mark.asyncio
    @respx.mock
    async def test_sends_multipart_with_logo_field(self):
        route = respx.put("https://3.basecampapi.com/12345/account/logo.json").mock(return_value=httpx.Response(204))

        c = AsyncClient(access_token="test-token")
        acct = c.for_account("12345")
        await acct.account.update_account_logo(
            content=b"fake-png-data",
            filename="logo.png",
            content_type="image/png",
        )

        assert route.called
        request = route.calls[0].request
        assert b"fake-png-data" in request.content
        assert request.headers["content-type"].startswith("multipart/form-data")
