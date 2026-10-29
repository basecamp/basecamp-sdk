from __future__ import annotations

import asyncio

import httpx
import pytest
import respx

from basecamp import (
    AsyncClient,
    FilingFailedError,
    FilingMayHaveStartedError,
    FilingRefusedError,
    FilingUnsettledError,
    filing_container_for,
    filing_container_takes_folders,
)
from basecamp.client import Client
from basecamp.errors import BasecampError, NotFoundError

BASE = "https://3.basecampapi.com/12345"


def filing(status, destination=False):
    body = {"id": 4, "status": status, "type": "Copy", "source_recording_id": 456, "url": f"{BASE}/filings/4.json"}
    if destination:
        body["destination_recording"] = {"id": 900, "type": "Message", "url": "u", "app_url": "a"}
    return body


def make_account():
    client = Client(access_token="test-token")
    return client, client.for_account("12345")


class TestCopyAndWait:
    @respx.mock
    def test_polls_until_completed(self):
        start = respx.post(f"{BASE}/recordings/456/copies.json").mock(
            return_value=httpx.Response(201, json=filing("pending"))
        )
        respx.get(f"{BASE}/filings/4").mock(
            side_effect=[
                httpx.Response(200, json=filing("processing")),
                httpx.Response(200, json=filing("completed", True)),
            ]
        )
        client, account = make_account()
        result = account.recordings.copy_and_wait(recording_id=456, filing_destination_id=789)
        client.close()
        assert result["destination_recording"]["id"] == 900
        assert start.call_count == 1

    @respx.mock
    def test_failed(self):
        respx.post(f"{BASE}/recordings/456/moves.json").mock(return_value=httpx.Response(201, json=filing("pending")))
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(200, json=filing("failed")))
        client, account = make_account()
        with pytest.raises(FilingFailedError) as raised:
            account.recordings.move_and_wait(recording_id=456, filing_destination_id=789)
        client.close()
        assert raised.value.filing["id"] == 4

    @respx.mock
    def test_wait_ends_while_processing(self):
        start = respx.post(f"{BASE}/recordings/456/copies.json").mock(
            return_value=httpx.Response(201, json=filing("pending"))
        )
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(200, json=filing("processing")))
        client, account = make_account()
        with pytest.raises(FilingUnsettledError) as raised:
            account.recordings.copy_and_wait(recording_id=456, filing_destination_id=789, timeout=0.6)
        client.close()
        assert raised.value.filing["status"] == "processing"
        assert start.call_count == 1

    @respx.mock
    def test_poll_failure_is_unsettled(self):
        respx.post(f"{BASE}/recordings/456/copies.json").mock(return_value=httpx.Response(201, json=filing("pending")))
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(404, json={"error": "Not Found"}))
        client, account = make_account()
        with pytest.raises(FilingUnsettledError) as raised:
            account.recordings.copy_and_wait(recording_id=456, filing_destination_id=789)
        client.close()
        assert isinstance(raised.value.__cause__, NotFoundError)

    @respx.mock
    def test_malformed_poll_keeps_the_filing(self):
        respx.post(f"{BASE}/recordings/456/copies.json").mock(return_value=httpx.Response(201, json=filing("pending")))
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(200, json=[]))
        client, account = make_account()
        with pytest.raises(FilingUnsettledError) as raised:
            account.recordings.copy_and_wait(recording_id=456, filing_destination_id=789)
        client.close()
        assert raised.value.filing["id"] == 4

    @respx.mock
    def test_async_malformed_poll_keeps_the_filing(self):
        respx.post(f"{BASE}/recordings/456/copies.json").mock(return_value=httpx.Response(201, json=filing("pending")))
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(200, json=[]))

        async def run():
            async with AsyncClient(access_token="test-token") as client:
                await client.for_account("12345").recordings.copy_and_wait(recording_id=456, filing_destination_id=789)

        with pytest.raises(FilingUnsettledError) as raised:
            asyncio.run(run())
        assert raised.value.filing["id"] == 4

    @respx.mock
    @pytest.mark.parametrize("status", [403, 404, 422])
    def test_start_refused(self, status):
        start = respx.post(f"{BASE}/recordings/456/moves.json").mock(return_value=httpx.Response(status, json={}))
        client, account = make_account()
        with pytest.raises(FilingRefusedError) as raised:
            account.recordings.move_and_wait(recording_id=456, filing_destination_id=789)
        client.close()
        assert raised.value.__cause__.http_status == status
        assert start.call_count == 1

    @respx.mock
    @pytest.mark.parametrize("status", [500, 503])
    def test_start_never_retried(self, status):
        start = respx.post(f"{BASE}/recordings/456/copies.json").mock(return_value=httpx.Response(status, json={}))
        client, account = make_account()
        with pytest.raises(FilingMayHaveStartedError) as raised:
            account.recordings.copy_and_wait(recording_id=456, filing_destination_id=789)
        client.close()
        assert not isinstance(raised.value, BasecampError)
        assert start.call_count == 1

    @respx.mock
    def test_async_polls_until_completed(self):
        respx.post(f"{BASE}/recordings/456/copies.json").mock(return_value=httpx.Response(201, json=filing("pending")))
        respx.get(f"{BASE}/filings/4").mock(return_value=httpx.Response(200, json=filing("completed", True)))

        async def run():
            async with AsyncClient(access_token="test-token") as client:
                return await client.for_account("12345").recordings.copy_and_wait(
                    recording_id=456, filing_destination_id=789
                )

        assert asyncio.run(run())["destination_recording"]["id"] == 900


@pytest.mark.parametrize(
    ("recording", "parent", "want"),
    [
        ("Kanban::Card", "Kanban::Triage", "kanban_board"),
        ("Kanban::Column", "Kanban::Board", "kanban_board"),
        ("Message", "Message::Board", "message_board"),
        ("Document", "Vault", "vault"),
        ("Upload", "Vault", "vault"),
        ("CloudFile", "Vault", "vault"),
        ("GoogleDocument", "Vault", "vault"),
        ("Vault", "Vault", "vault"),
        ("Schedule::Entry", "Schedule", "schedule"),
        ("Question", "Questionnaire", "questionnaire"),
        ("Inbox::Forward", "Inbox", "inbox"),
        ("Todolist", "Todoset", "todoset"),
        ("Todolist", "Todolist", "todolist"),
        ("Todo", "Todolist", "todolist"),
        ("Vault", "", None),
        ("Message::Board", "", None),
        ("Kanban::Triage", "Kanban::Board", None),
        ("Comment", "Message", None),
        ("Question::Answer", "Question", None),
    ],
)
def test_filing_container_for(recording, parent, want):
    got = filing_container_for(recording, parent)
    assert got == want
    if got:
        assert filing_container_takes_folders(got) == (got == "vault")
