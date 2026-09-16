"""Tests for the EventFeedService (account event feed, agent inbox, stream tickets)."""

from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import Client
from basecamp.errors import ApiError, AuthError, BasecampError, ForbiddenError, ValidationError

BASE = "https://3.basecampapi.com/12345"


def _feed_event(event_id: int, **overrides) -> dict:
    return {
        "id": event_id,
        "kind": "message_created",
        "action": "created",
        "created_at": "2026-07-14T06:10:00.159Z",
        "event_type": "message.created",
        "bucket_id": 2085958499,
        "creator_id": 1049715945,
        "performed_by_id": None,
        "recording_id": 1069479766,
        **overrides,
    }


def _event_feed():
    return Client(access_token="test-token").for_account("12345").event_feed


class TestPollEvents:
    @respx.mock
    def test_sends_entry_and_filters_and_decodes_the_envelope(self):
        route = respx.get(f"{BASE}/events.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "events": [
                        _feed_event(1071915468),
                        _feed_event(
                            1071915470,
                            kind="boost_created",
                            event_type="boost.created",
                            performed_by_id=1049715999,
                            details={
                                "boost_id": 501,
                                "boosted_event_id": 1071915468,
                                "boosted_event_type": "message.created",
                            },
                        ),
                    ],
                    "position": "posAAA",
                    "next": "https://3.basecampapi.com/12345/events.json?position=posAAA&types=message.created%2Cboost.created",
                },
            )
        )

        page = _event_feed().poll_events(
            since="0",
            types="message.created,boost.created",
            buckets="2085958499",
            exclude_performers="self",
            actor_types="agent,person",
        )

        query = route.calls.last.request.url.params
        assert query["since"] == "0"
        assert query["types"] == "message.created,boost.created"
        assert query["buckets"] == "2085958499"
        assert query["exclude_performers"] == "self"
        assert query["actor_types"] == "agent,person"
        assert "position" not in query

        assert page["position"] == "posAAA"
        assert "position=posAAA" in page["next"]
        assert len(page["events"]) == 2
        assert page["events"][0]["performed_by_id"] is None
        assert "details" not in page["events"][0]
        assert page["events"][1]["performed_by_id"] == 1049715999
        assert page["events"][1]["details"]["boost_id"] == 501

    @respx.mock
    def test_bare_call_enters_at_the_present(self):
        route = respx.get(f"{BASE}/events.json").mock(
            return_value=httpx.Response(200, json={"events": [], "position": "posNOW"})
        )

        page = _event_feed().poll_events()

        assert str(route.calls.last.request.url.query, "utf-8") == ""
        assert page["events"] == []
        assert page["position"] == "posNOW"
        assert "next" not in page

    @respx.mock
    def test_409_filter_mismatch_is_a_non_retryable_api_error(self):
        respx.get(f"{BASE}/events.json").mock(
            return_value=httpx.Response(
                409,
                json={
                    "error": "Positions are bound to the filter set they were minted for.",
                    "position_digest": "38b223c13c89dc89",
                    "filters_digest": "44136fa355b3678a",
                },
            )
        )

        with pytest.raises(ApiError) as excinfo:
            _event_feed().poll_events(position="posAAA")

        assert excinfo.value.http_status == 409
        assert excinfo.value.retryable is False
        assert "Positions are bound" in str(excinfo.value)

    @respx.mock
    def test_410_stale_position_is_a_non_retryable_api_error(self):
        respx.get(f"{BASE}/events.json").mock(
            return_value=httpx.Response(
                410,
                json={
                    "error": "That position predates this feed's epoch, so the history behind it can't be served.",
                    "epoch_after_id": 1071915000,
                    "resume": "https://3.basecampapi.com/12345/events.json?since=1071915000",
                },
            )
        )

        with pytest.raises(ApiError) as excinfo:
            _event_feed().poll_events(position="posOLD")

        assert excinfo.value.http_status == 410
        assert excinfo.value.retryable is False

    @respx.mock
    def test_400_malformed_position_is_validation(self):
        respx.get(f"{BASE}/events.json").mock(
            return_value=httpx.Response(
                400, json={"error": "Unrecognized position. Resume with since=<id> or since=now."}
            )
        )

        with pytest.raises(ValidationError) as excinfo:
            _event_feed().poll_events(position="garbage")

        assert excinfo.value.http_status == 400


class TestPollInbox:
    @respx.mock
    def test_decodes_the_envelope_and_sends_reasons(self):
        route = respx.get(f"{BASE}/inbox.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "items": [
                        {
                            "addressing_id": 991,
                            "reason": "mentioned",
                            "addressed_at": "2026-07-14T06:10:00.159Z",
                            "event": _feed_event(1071915468, kind="comment_created", event_type="comment.created"),
                        }
                    ],
                    "position": "inboxPos",
                },
            )
        )

        page = _event_feed().poll_inbox(since="0", reasons="mentioned,assigned")

        query = route.calls.last.request.url.params
        assert query["since"] == "0"
        assert query["reasons"] == "mentioned,assigned"
        assert page["position"] == "inboxPos"
        assert page["items"][0]["addressing_id"] == 991
        assert page["items"][0]["event"]["event_type"] == "comment.created"

    @respx.mock
    def test_403_is_forbidden(self):
        respx.get(f"{BASE}/inbox.json").mock(return_value=httpx.Response(403))

        with pytest.raises(ForbiddenError) as excinfo:
            _event_feed().poll_inbox()

        assert excinfo.value.http_status == 403


class TestCreateStreamTicket:
    @respx.mock
    def test_posts_without_a_body_and_decodes_the_mint(self):
        route = respx.post(f"{BASE}/events/stream_ticket.json").mock(
            return_value=httpx.Response(
                200,
                json={
                    "ticket": "fixture-ticket-not-a-credential",
                    "expires_in": 120,
                    "url": "wss://cable.example.invalid/12345?ticket=fixture-ticket-not-a-credential",
                },
            )
        )

        ticket = _event_feed().create_stream_ticket()

        assert route.calls.last.request.content in (b"", b"null", b"{}")
        assert ticket["ticket"] == "fixture-ticket-not-a-credential"
        assert ticket["expires_in"] == 120
        assert "?ticket=" in ticket["url"]

    @respx.mock
    def test_401_is_auth_required(self):
        respx.post(f"{BASE}/events/stream_ticket.json").mock(
            return_value=httpx.Response(401, json={"error": "Unauthorized"})
        )

        with pytest.raises(AuthError):
            _event_feed().create_stream_ticket()

    def test_errors_share_the_base_class(self):
        assert issubclass(ApiError, BasecampError)
