"""Tests for the PeopleService client-access operations (project client users and enablement)."""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client, Config
from basecamp.errors import ForbiddenError, RateLimitError, ValidationError

BASE = "https://3.basecampapi.com/12345"
CLIENT_USERS = f"{BASE}/projects/100/people/client_users.json"
ENABLEMENT = f"{BASE}/projects/100/client_enablement.json"


def _people():
    return Client(access_token="test-token").for_account("12345").people


def _person(person_id: int, name: str, email: str) -> dict:
    return {"id": person_id, "name": name, "email_address": email, "client": True}


class TestUpdateProjectClientAccess:
    @respx.mock
    def test_sends_the_change_set_and_returns_affected_clients(self):
        route = respx.put(CLIENT_USERS).mock(
            return_value=httpx.Response(
                200,
                json={
                    "granted": [_person(444, "annie@example.com", "annie@example.com")],
                    "revoked": [_person(333, "Former Client", "former@example.com")],
                },
            )
        )

        result = _people().update_project_client_access(
            project_id=100,
            revoke=[333],
            create=[{"email_address": "annie@example.com", "company_name": "Springfield Elementary"}],
        )

        assert json.loads(route.calls.last.request.content) == {
            "revoke": [333],
            "create": [{"email_address": "annie@example.com", "company_name": "Springfield Elementary"}],
        }
        assert [person["id"] for person in result["granted"]] == [444]
        assert result["granted"][0]["client"] is True
        assert [person["id"] for person in result["revoked"]] == [333]

    @respx.mock
    def test_403_until_clients_are_enabled(self):
        respx.put(CLIENT_USERS).mock(return_value=httpx.Response(403))

        with pytest.raises(ForbiddenError):
            _people().update_project_client_access(project_id=100, grant=[111])

    @respx.mock
    def test_422_rejects_the_whole_batch_for_an_invalid_row(self):
        respx.put(CLIENT_USERS).mock(
            return_value=httpx.Response(
                422,
                json={"errors": [{"email_address": "not-an-address", "messages": ["Email address must be valid"]}]},
            )
        )

        with pytest.raises(ValidationError) as excinfo:
            _people().update_project_client_access(
                project_id=100,
                create=[{"email_address": "annie@example.com"}, {"email_address": "not-an-address"}],
            )
        assert excinfo.value.http_status == 422
        assert str(excinfo.value) == "not-an-address: Email address must be valid"
        assert excinfo.value.field_errors == {"not-an-address": ["Email address must be valid"]}

    @respx.mock
    def test_429_seat_limit_surfaces_as_rate_limit(self):
        respx.put(CLIENT_USERS).mock(return_value=httpx.Response(429))

        with pytest.raises(RateLimitError) as excinfo:
            Client(access_token="test-token", config=Config(max_retries=0)).for_account(
                "12345"
            ).people.update_project_client_access(project_id=100, create=[{"email_address": "annie@example.com"}])
        assert excinfo.value.http_status == 429


class TestEnableProjectClients:
    @respx.mock
    def test_posts_the_enablement(self):
        route = respx.post(ENABLEMENT).mock(return_value=httpx.Response(200, json={"clients_enabled": True}))

        result = _people().enable_project_clients(project_id=100)

        assert route.called
        assert result == {"clients_enabled": True}

    @respx.mock
    def test_403_when_the_project_cannot_have_clients(self):
        respx.post(ENABLEMENT).mock(return_value=httpx.Response(403))

        with pytest.raises(ForbiddenError):
            _people().enable_project_clients(project_id=100)


class TestDisableProjectClients:
    @respx.mock
    def test_deletes_the_enablement(self):
        route = respx.delete(ENABLEMENT).mock(return_value=httpx.Response(200, json={"clients_enabled": False}))

        result = _people().disable_project_clients(project_id=100)

        assert route.called
        assert result == {"clients_enabled": False}

    @respx.mock
    def test_403_while_client_users_remain(self):
        respx.delete(ENABLEMENT).mock(return_value=httpx.Response(403))

        with pytest.raises(ForbiddenError):
            _people().disable_project_clients(project_id=100)


class TestAsyncProjectClients:
    @respx.mock
    @pytest.mark.asyncio
    async def test_update_project_client_access(self):
        route = respx.put(CLIENT_USERS).mock(
            return_value=httpx.Response(
                200, json={"granted": [_person(111, "Annie", "annie@example.com")], "revoked": []}
            )
        )

        client = AsyncClient(access_token="test-token")
        result = await client.for_account("12345").people.update_project_client_access(project_id=100, grant=[111])
        await client.close()

        assert json.loads(route.calls.last.request.content) == {"grant": [111]}
        assert [person["id"] for person in result["granted"]] == [111]

    @respx.mock
    @pytest.mark.asyncio
    async def test_enable_and_disable_project_clients(self):
        respx.post(ENABLEMENT).mock(return_value=httpx.Response(200, json={"clients_enabled": True}))
        respx.delete(ENABLEMENT).mock(return_value=httpx.Response(200, json={"clients_enabled": False}))

        client = AsyncClient(access_token="test-token")
        people = client.for_account("12345").people
        assert (await people.enable_project_clients(project_id=100)) == {"clients_enabled": True}
        assert (await people.disable_project_clients(project_id=100)) == {"clients_enabled": False}
        await client.close()


MY_AVATAR = f"{BASE}/my/avatar.json"


def _assert_single_avatar_part(request: httpx.Request, data: bytes, filename: str, content_type: str) -> None:
    assert request.method == "PUT"
    assert request.headers["content-type"].startswith("multipart/form-data; boundary=")
    body = request.content
    assert body.count(b"Content-Disposition: form-data;") == 1
    expected = (
        (
            f'Content-Disposition: form-data; name="avatar"; filename="{filename}"\r\n'
            f"Content-Type: {content_type}\r\n\r\n"
        ).encode()
        + data
        + b"\r\n"
    )
    assert expected in body


class TestMyAvatar:
    @respx.mock
    def test_update_my_avatar_sends_one_avatar_file_part(self):
        route = respx.put(MY_AVATAR).mock(return_value=httpx.Response(204))

        result = _people().update_my_avatar(
            content=b"\x89PNG avatar-bytes", filename="me.png", content_type="image/png"
        )

        assert result is None
        assert route.call_count == 1
        _assert_single_avatar_part(route.calls[0].request, b"\x89PNG avatar-bytes", "me.png", "image/png")

    @respx.mock
    def test_update_my_avatar_refused_image(self):
        respx.put(MY_AVATAR).mock(
            return_value=httpx.Response(422, json={"error": "Avatar is not an allowed image type"})
        )

        with pytest.raises(ValidationError) as excinfo:
            _people().update_my_avatar(content=b"not an image", filename="a.txt", content_type="text/plain")

        assert excinfo.value.http_status == 422

    @respx.mock
    def test_delete_my_avatar(self):
        route = respx.delete(MY_AVATAR).mock(return_value=httpx.Response(204))

        assert _people().delete_my_avatar() is None
        assert route.call_count == 1

    @respx.mock
    def test_delete_my_avatar_forbidden(self):
        respx.delete(MY_AVATAR).mock(return_value=httpx.Response(403, json={"error": "Access denied"}))

        with pytest.raises(ForbiddenError) as excinfo:
            _people().delete_my_avatar()

        assert excinfo.value.http_status == 403

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_update_my_avatar_sends_one_avatar_file_part(self):
        route = respx.put(MY_AVATAR).mock(return_value=httpx.Response(204))

        account = AsyncClient(access_token="test-token").for_account("12345")
        result = await account.people.update_my_avatar(
            content=b"gif-bytes", filename="me.gif", content_type="image/gif"
        )

        assert result is None
        _assert_single_avatar_part(route.calls[0].request, b"gif-bytes", "me.gif", "image/gif")
