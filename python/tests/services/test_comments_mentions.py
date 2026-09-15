"""Tests for ``comments.expand_mentions`` / ``create_with_mentions`` (sync + async).

The account-bound half of the mention write side. Its one guarantee over the
pure helpers is the people read: every requested id is resolved through
``people.get`` for its ``attachable_sgid`` before anything is posted, so a
mention can never be written from an sgid the caller supplied.
"""

from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import ForbiddenError, NotFoundError, UsageError

ACCOUNT = "12345"
BASE = f"https://3.basecampapi.com/{ACCOUNT}"
RECORDING_ID = 1069479351

VICTOR_ID = 1049715915
VICTOR_SGID = (
    "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7"
    "AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--aeb392ebf54ffd820e45f27add22bae3a8c7da56"
)
ANNIE_ID = 1049715914
ANNIE_SGID = (
    "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE0P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7"
    "AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--anniesignature"
)


def _person_route(person_id: int, sgid: str):
    return respx.get(f"{BASE}/people/{person_id}").mock(
        return_value=httpx.Response(200, json={"id": person_id, "attachable_sgid": sgid, "name": "Someone"})
    )


def _create_route():
    return respx.post(f"{BASE}/recordings/{RECORDING_ID}/comments.json").mock(
        return_value=httpx.Response(201, json={"id": 9, "type": "Comment", "content": "posted"})
    )


def _posted_content(route) -> str:
    return json.loads(route.calls[-1].request.content)["content"]


def _comments():
    return Client(access_token="test-token").for_account(ACCOUNT).comments


def _async_comments():
    return AsyncClient(access_token="test-token").for_account(ACCOUNT).comments


class TestExpandMentions:
    @respx.mock
    def test_resolves_each_person_and_writes_the_tag_from_their_sgid(self):
        _person_route(VICTOR_ID, VICTOR_SGID)

        out = _comments().expand_mentions(content="<div>On it.</div>", person_ids=[VICTOR_ID])

        assert out == f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> On it.</div>'

    @respx.mock
    def test_reads_each_distinct_id_exactly_once(self):
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        annie = _person_route(ANNIE_ID, ANNIE_SGID)

        out = _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID, ANNIE_ID, VICTOR_ID])

        assert victor.call_count == 1 and annie.call_count == 1
        assert out.count("<bc-attachment ") == 2

    @respx.mock
    def test_reads_the_person_even_when_the_content_already_carries_their_sgid(self):
        # The sgid in caller-supplied content is unsigned and proves nothing, so
        # it never stands in for the authoritative read. What it does do is stop
        # the tag being written twice.
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        content = f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> hi</div>'

        out = _comments().expand_mentions(content=content, person_ids=[VICTOR_ID])

        assert victor.call_count == 1
        assert out == content

    @respx.mock
    def test_makes_no_request_when_nothing_is_mentioned(self):
        route = respx.route(host="3.basecampapi.com")

        assert _comments().expand_mentions(content="<div>x</div>") == "<div>x</div>"
        assert _comments().expand_mentions(content="<div>x</div>", person_ids=[]) == "<div>x</div>"
        assert not route.called

    @respx.mock
    def test_a_failed_person_read_fails_the_expansion(self):
        _person_route(VICTOR_ID, VICTOR_SGID)
        respx.get(f"{BASE}/people/{ANNIE_ID}").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID, ANNIE_ID])

        # The canonical code survives: the failure is annotated with which
        # person it was, not replaced by a different error.
        assert raised.value.code == "not_found"
        assert f"resolving mention for person {ANNIE_ID}" in str(raised.value)

    @respx.mock
    def test_checks_each_id_where_go_checks_it(self):
        # Go validates inside the resolve loop, so a bad id after a good one
        # still costs the good one's read. The request sequence is exactly what
        # a shared fixture pins, so it has to mean the same thing here.
        victor = _person_route(VICTOR_ID, VICTOR_SGID)

        with pytest.raises(UsageError, match="invalid mention person id"):
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID, -1])

        assert victor.call_count == 1

    @respx.mock
    def test_a_failed_read_says_which_mention_failed_in_the_message(self):
        # Go wraps this, so the prefix is part of the message — and the
        # conformance runners assert on message text. The error's class, code
        # and status survive intact.
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(return_value=httpx.Response(404, json={"error": "Not found"}))

        with pytest.raises(NotFoundError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert f"resolving mention for person {VICTOR_ID}" in str(raised.value)
        assert raised.value.code == "not_found"
        assert raised.value.http_status == 404

    @respx.mock
    def test_annotating_a_failure_twice_does_not_repeat_the_prefix(self):
        # Go builds a new error per wrap and cannot double-prefix. Rewriting
        # args in place can — and the standard mock idiom is a `side_effect`
        # holding one pre-built exception instance, re-raised on every call.
        failure = NotFoundError("Not found", http_status=404)
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(side_effect=failure)

        for _ in range(2):
            with pytest.raises(NotFoundError) as raised:
                _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert str(raised.value).count("resolving mention for person") == 1

    @respx.mock
    def test_a_failure_with_no_message_still_says_which_mention(self):
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(side_effect=ValueError())

        with pytest.raises(ValueError) as raised:
            _comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])

        assert str(raised.value) == f"resolving mention for person {VICTOR_ID}"

    @respx.mock
    def test_refuses_an_id_that_is_not_an_id(self):
        route = respx.route(host="3.basecampapi.com")
        for bad in (0, -1, "1049715915", None, True):
            with pytest.raises(UsageError, match="invalid mention person id"):
                _comments().expand_mentions(content="<div>x</div>", person_ids=[bad])
        assert not route.called


class TestCreateWithMentions:
    @respx.mock
    def test_reads_every_person_before_it_posts(self):
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        create = _create_route()

        _comments().create_with_mentions(recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID])

        assert victor.called and create.called
        assert _posted_content(create) == f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> On it.</div>'

    @respx.mock
    def test_posts_nothing_when_a_mention_cannot_be_resolved(self):
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(return_value=httpx.Response(403, json={"error": "Denied"}))
        create = _create_route()

        with pytest.raises(ForbiddenError):
            _comments().create_with_mentions(
                recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID]
            )

        assert not create.called, "nothing is posted on a partial mention list"

    @respx.mock
    def test_posts_a_plain_comment_when_nobody_is_mentioned(self):
        create = _create_route()

        _comments().create_with_mentions(recording_id=RECORDING_ID, content="<div>On it.</div>")

        assert _posted_content(create) == "<div>On it.</div>"

    @respx.mock
    def test_refuses_empty_content_before_any_request(self):
        route = respx.route(host="3.basecampapi.com")
        with pytest.raises(UsageError, match="content is required"):
            _comments().create_with_mentions(recording_id=RECORDING_ID, content="", mentions=[VICTOR_ID])
        assert not route.called

    @respx.mock
    def test_the_plain_create_is_still_reachable(self):
        # Rule 6 does not arise here -- the composite takes over no generated
        # method name -- and this is what says so.
        create = _create_route()
        _comments().create(recording_id=RECORDING_ID, content="<div>plain</div>")
        assert _posted_content(create) == "<div>plain</div>"


@pytest.mark.asyncio
class TestAsync:
    @respx.mock
    async def test_reads_every_person_before_it_posts(self):
        victor = _person_route(VICTOR_ID, VICTOR_SGID)
        create = _create_route()

        await _async_comments().create_with_mentions(
            recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID]
        )

        assert victor.called
        assert _posted_content(create) == f'<div><bc-attachment sgid="{VICTOR_SGID}"></bc-attachment> On it.</div>'

    @respx.mock
    async def test_posts_nothing_when_a_mention_cannot_be_resolved(self):
        respx.get(f"{BASE}/people/{VICTOR_ID}").mock(return_value=httpx.Response(403, json={"error": "Denied"}))
        create = _create_route()

        with pytest.raises(ForbiddenError):
            await _async_comments().create_with_mentions(
                recording_id=RECORDING_ID, content="<div>On it.</div>", mentions=[VICTOR_ID]
            )

        assert not create.called

    @respx.mock
    async def test_expands_without_a_write(self):
        _person_route(VICTOR_ID, VICTOR_SGID)
        out = await _async_comments().expand_mentions(content="<div>x</div>", person_ids=[VICTOR_ID])
        assert VICTOR_SGID in out
