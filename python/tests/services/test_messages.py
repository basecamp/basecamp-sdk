"""Tests for the messages.create request body (visible_to_clients tri-state).

visible_to_clients is tri-state: unset omits the key (``_compact`` drops None),
true/false are sent verbatim. An explicit false must reach the wire. The shared
generator carries this field on all six create ops; this messages coverage stands
in for the other five ops.
"""

from __future__ import annotations

import json

import httpx
import respx

from basecamp import Client

CREATE_URL = "https://3.basecampapi.com/12345/message_boards/456/messages.json"


def _sent_body(route) -> dict:
    return json.loads(route.calls[-1].request.content)


@respx.mock
def test_create_omits_visible_to_clients_when_unset():
    route = respx.post(CREATE_URL).mock(return_value=httpx.Response(201, json={"id": 99}))

    account = Client(access_token="test-token").for_account("12345")
    account.messages.create(board_id=456, subject="Test")

    assert "visible_to_clients" not in _sent_body(route)


@respx.mock
def test_create_sends_visible_to_clients_true():
    route = respx.post(CREATE_URL).mock(return_value=httpx.Response(201, json={"id": 99}))

    account = Client(access_token="test-token").for_account("12345")
    account.messages.create(board_id=456, subject="Test", visible_to_clients=True)

    assert _sent_body(route)["visible_to_clients"] is True


@respx.mock
def test_create_sends_visible_to_clients_false():
    route = respx.post(CREATE_URL).mock(return_value=httpx.Response(201, json={"id": 99}))

    account = Client(access_token="test-token").for_account("12345")
    account.messages.create(board_id=456, subject="Test", visible_to_clients=False)

    body = _sent_body(route)
    assert "visible_to_clients" in body
    assert body["visible_to_clients"] is False
