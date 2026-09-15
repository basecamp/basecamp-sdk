"""Tests for ``recordings.summarize`` (sync + async).

The conformance fixture ``conformance/tests/recording_summary.json`` pins the
routing matrix, the projection's shape and the request sequence for every routed
type. What it cannot reach lives here: the routing table's own integrity, the
error identities the discovery loop raises, the Campfire cache's lifetime and
refresh floor, the candidate bound, and the promise that hooks see the
constituent reads under their NATIVE names rather than a minted composite one
(SPEC section 18 rule 3).
"""

from __future__ import annotations

import inspect

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import (
    BucketMismatchError,
    CampfireDiscoveryIncompleteError,
    ForbiddenError,
    NoRecordingTypeError,
    RecordingRoutingError,
    RecordingUnresolvedError,
    UnknownRecordingTypeError,
    UsageError,
)
from basecamp.hooks import BasecampHooks
from basecamp.services import _campfire_index
from basecamp.services._campfire_index import MAX_CAMPFIRE_CANDIDATES, AsyncCampfireIndex, CampfireIndex
from basecamp.services.recordings import _READS, summarizable_event_types, summarizable_recording_types

ACCOUNT = "12345"
BASE = f"https://3.basecampapi.com/{ACCOUNT}"
BUCKET = 2085958499
LINE_ID = 1069479350


def _client(*, now=None, hooks=None) -> Client:
    client = Client(access_token="test-token", hooks=hooks)
    if now is not None:
        # The discovery cache is a lazily-filled slot on the client; seeding it
        # with a controlled clock is how a TTL and a refresh floor become
        # testable at all.
        client._campfire_index = CampfireIndex(now=now)
    return client


def _account(**kwargs):
    return _client(**kwargs).for_account(ACCOUNT)


def _project(*campfire_ids: int) -> dict:
    return {
        "id": BUCKET,
        "name": "The Leto Laptop",
        "dock": [{"id": cid, "name": "chat", "title": "Campfire", "enabled": True} for cid in campfire_ids]
        + [{"id": 999, "name": "message_board", "title": "Message Board", "enabled": True}],
    }


def _campfire(campfire_id: int, bucket_id: int = BUCKET) -> dict:
    return {"id": campfire_id, "type": "Chat::Transcript", "bucket": {"id": bucket_id, "name": "b", "type": "Project"}}


def _line(campfire_id: int, *, line_type: str = "Chat::Lines::Text", content: str = "Hello everyone!") -> dict:
    return {
        "id": LINE_ID,
        "status": "active",
        "type": line_type,
        "title": content,
        "app_url": f"https://3.basecamp.com/{ACCOUNT}/buckets/{BUCKET}/chats/{campfire_id}#__recording_{LINE_ID}",
        "content": content,
        "parent": {"id": campfire_id, "type": "Chat::Transcript"},
        "bucket": {"id": BUCKET, "name": "The Leto Laptop", "type": "Project"},
        "creator": {"id": 1049715914, "name": "Victor Cooper"},
        "updated_at": "2022-10-28T15:25:00.000Z",
    }


def _not_found() -> httpx.Response:
    return httpx.Response(404, json={"error": "Record not found"})


class TestRoutingTable:
    def test_every_routed_read_names_a_real_service_method(self):
        # The table is the routing contract, and it addresses the generated
        # services by name. A generated method renamed out from under it would
        # otherwise only surface as an AttributeError on the one type nobody
        # exercised that week.
        account = _account()
        for recording_type, read in _READS.items():
            service = getattr(account, read.service, None)
            assert service is not None, f"{recording_type}: no account.{read.service} accessor"
            method = getattr(service, read.method, None)
            assert callable(method), f"{recording_type}: {read.service} has no {read.method}"

    def test_every_routed_read_takes_its_declared_id_keyword(self):
        account = _account()
        for recording_type, read in _READS.items():
            signature = inspect.signature(getattr(getattr(account, read.service), read.method))
            assert read.param in signature.parameters, (
                f"{recording_type}: {read.service}.{read.method} takes no {read.param}"
            )

    def test_the_documented_type_list_is_the_routed_one(self):
        documented = summarizable_recording_types()
        assert "Chat::Lines::*" in documented
        assert "Comment" in documented and "Kanban::Card" in documented
        assert documented == sorted(documented)
        # A deliberate set, not an exhaustive one: these have id-only reads in
        # the SDK and are still not routed.
        assert "Timesheet::Entry" not in documented
        assert "Gauge::Needle" not in documented

    def test_the_documented_event_subjects_exclude_boost(self):
        assert summarizable_event_types() == [
            "card.*",
            "chat.line.*",
            "comment.*",
            "message.*",
            "todo.*",
        ]


class TestRoutingRefusals:
    @respx.mock
    def test_boost_is_refused_before_any_request(self):
        route = respx.route(host="3.basecampapi.com")
        with pytest.raises(NoRecordingTypeError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="boost.created")
        assert not route.called
        assert raised.value.code == "no_recording_type"
        assert isinstance(raised.value, RecordingRoutingError)

    @respx.mock
    @pytest.mark.parametrize(
        "pointer",
        [
            {"event_type": "widget.created"},
            {"event_type": "comment"},  # a subject with no action is not a feed type
            {"event_type": "comment."},
            {"event_type": ".created"},
            {"recording_type": "Timesheet::Entry"},
            {"recording_type": "Chat::Line"},  # near-miss on the prefix
            {},
        ],
    )
    def test_an_unroutable_pointer_is_refused_before_any_request(self, pointer):
        route = respx.route(host="3.basecampapi.com")
        with pytest.raises(UnknownRecordingTypeError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, **pointer)
        assert not route.called
        assert raised.value.code == "unknown_recording_type"

    @respx.mock
    def test_the_recording_type_wins_over_the_event_type(self):
        # The recording type is the more exact of the two, so a row carrying
        # both routes on it.
        route = respx.get(f"{BASE}/documents/7").mock(
            return_value=httpx.Response(200, json={"id": 7, "type": "Document", "bucket": {"id": BUCKET}})
        )
        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=7, event_type="comment.created", recording_type="Document"
        )
        assert route.called
        assert summary["type"] == "Document"

    @pytest.mark.parametrize("pointer", [{"bucket_id": 0, "recording_id": 1}, {"bucket_id": 1, "recording_id": 0}])
    def test_a_pointer_missing_an_id_is_a_usage_error(self, pointer):
        with pytest.raises(UsageError, match="bucket id and recording id"):
            _account().recordings.summarize(event_type="comment.created", **pointer)


class TestProjection:
    @respx.mock
    def test_a_read_from_another_bucket_is_refused(self):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(
                200, json={"id": 1, "type": "Comment", "content": "", "bucket": {"id": 777, "name": "Elsewhere"}}
            )
        )
        with pytest.raises(BucketMismatchError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        assert raised.value.code == "bucket_mismatch"
        assert raised.value.bucket_id == 777
        assert raised.value.requested_bucket_id == BUCKET

    @respx.mock
    def test_a_read_carrying_no_bucket_is_not_a_mismatch(self):
        respx.get(f"{BASE}/comments/1").mock(return_value=httpx.Response(200, json={"id": 1, "type": "Comment"}))
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")
        assert summary["bucket"] is None

    @respx.mock
    def test_absent_fields_project_as_empties_rather_than_missing_keys(self):
        respx.get(f"{BASE}/vaults/9").mock(return_value=httpx.Response(200, json={"id": 9, "type": "Vault"}))
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=9, recording_type="Vault")
        assert summary == {
            "id": 9,
            "status": "",
            "type": "Vault",
            "title": "",
            "app_url": "",
            "parent": None,
            "bucket": None,
            "creator": None,
            "assignees": [],
            "mentioned_person_ids": [],
            "content": "",
            "updated_at": None,
            "campfire_id": None,
        }

    @respx.mock
    @pytest.mark.parametrize(
        ("line_type", "expected"),
        [
            ("Chat::Lines::RichText", [1049715915]),
            ("Chat::Lines::Integration", [1049715915]),
            ("Chat::Lines::Text", []),
            ("Chat::Lines::Code", []),
        ],
    )
    def test_only_a_rich_text_chat_line_can_mention_anyone(self, line_type, expected):
        # A Text line's content is HTML-escaped on the way out and a Code line's
        # is served verbatim, so a literal bc-attachment in either is text BC3
        # never read as markup.
        sgid = (
            "BAh7CEkiCGdpZAY6BkVUSSIrZ2lkOi8vYmMzL1BlcnNvbi8xMDQ5NzE1OTE1P2V4cGlyZXNfaW4GOwBUSSIMcHVycG9zZQY7"
            "AFRJIg9hdHRhY2hhYmxlBjsAVEkiD2V4cGlyZXNfYXQGOwBUMA==--919d2c8b11ff403eefcab9db42dd26846d0c3102"
        )
        content = f'<div><bc-attachment sgid="{sgid}"></bc-attachment> hi</div>'
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1)))
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(
            return_value=httpx.Response(200, json=_line(1, line_type=line_type, content=content))
        )
        summary = _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, recording_type=line_type)
        assert summary["mentioned_person_ids"] == expected


class TestChatLineDiscovery:
    @respx.mock
    def test_the_dock_answers_without_a_listing(self):
        # The listing is the expensive request; a project's own dock names its
        # Campfire, so it is never reached for one.
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        line = respx.get(f"{BASE}/chats/77/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(77)))

        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert project.called and line.called
        assert not listing.called
        assert summary["campfire_id"] == 77

    @respx.mock
    def test_a_non_404_from_a_candidate_stops_the_search_and_is_raised(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1, 2)))
        first = respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(
            return_value=httpx.Response(403, json={"error": "Access denied"})
        )
        second = respx.get(f"{BASE}/chats/2/lines/{LINE_ID}").mock(return_value=_not_found())

        with pytest.raises(ForbiddenError):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert first.called
        assert not second.called, "a permission failure must not be tried past as if it meant 'not here'"

    @respx.mock
    def test_every_candidate_404ing_is_unresolved_not_not_found(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1, 2)))
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))

        with pytest.raises(RecordingUnresolvedError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.code == "recording_unresolved"
        assert raised.value.campfire_ids == [1, 2]
        assert raised.value.bucket_id == BUCKET

    @respx.mock
    def test_a_bucket_that_is_not_a_project_falls_through_to_the_listing(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(200, json=[_campfire(5), _campfire(6, bucket_id=999)])
        )
        elsewhere = respx.get(f"{BASE}/chats/6/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(6)))
        respx.get(f"{BASE}/chats/5/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(5)))

        summary = _account().recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert listing.called
        assert summary["campfire_id"] == 5
        assert not elsewhere.called, "a Campfire in another bucket is never a candidate"

    @respx.mock
    def test_more_candidates_than_the_budget_is_incomplete_never_unresolved(self):
        # Nothing left unsearched is ever reported absent.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(200, json=[_campfire(i) for i in range(1, MAX_CAMPFIRE_CANDIDATES + 11)])
        )
        lines = respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())

        with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.code == "campfire_discovery_incomplete"
        assert str(MAX_CAMPFIRE_CANDIDATES) in raised.value.reason
        assert lines.call_count == MAX_CAMPFIRE_CANDIDATES

    @respx.mock
    def test_a_listing_over_its_cap_is_incomplete_and_is_not_cached(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        overflowing = [_campfire(i) for i in range(1, _campfire_index.MAX_CAMPFIRE_LISTING + 1)]
        listing = respx.get(f"{BASE}/chats.json").mock(
            return_value=httpx.Response(
                200,
                json=overflowing,
                headers={"Link": f'<{BASE}/chats.json?page=2>; rel="next"'},
            )
        )
        account = _account()

        for _ in range(2):
            with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
                account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
            assert "exceeds" in raised.value.reason

        assert listing.call_count == 2, "a truncated listing must not be cached as if it were the whole set"


class TestDiscoveryCache:
    @respx.mock
    def test_a_second_line_in_the_same_bucket_reuses_the_dock(self):
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        respx.get(url__regex=rf"{BASE}/chats/77/lines/\d+").mock(return_value=httpx.Response(200, json=_line(77)))
        account = _account()

        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID + 1, event_type="chat.line.created")

        assert project.call_count == 1

    @respx.mock
    def test_the_cache_expires(self):
        clock = [0.0]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        respx.get(url__regex=rf"{BASE}/chats/77/lines/\d+").mock(return_value=httpx.Response(200, json=_line(77)))
        account = _account(now=lambda: clock[0])

        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        clock[0] = _campfire_index.CAMPFIRE_INDEX_TTL + 1
        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert project.call_count == 2

    @respx.mock
    def test_a_miss_refreshes_the_cached_dock_before_concluding(self):
        # A Campfire created after the cache filled must be tried before the
        # line is called unresolvable.
        clock = [0.0]
        docks = [_project(1), _project(1, 2)]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(
            side_effect=[httpx.Response(200, json=dock) for dock in docks]
        )
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats/2/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(2)))
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = _account(now=lambda: clock[0])

        # Fill the dock cache with the snapshot that does not hold the line.
        with pytest.raises(RecordingUnresolvedError):
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        assert project.call_count == 1, "a freshly loaded source is not re-read in the same call"

        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        summary = account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert project.call_count == 2
        assert summary["campfire_id"] == 2

    @respx.mock
    def test_the_refresh_floor_bounds_a_run_of_unresolvable_lines(self):
        clock = [0.0]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1)))
        respx.get(url__regex=rf"{BASE}/chats/1/lines/\d+").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = _account(now=lambda: clock[0])

        for recording_id in (LINE_ID, LINE_ID + 1, LINE_ID + 2):
            with pytest.raises(RecordingUnresolvedError) as raised:
                account.recordings.summarize(
                    bucket_id=BUCKET, recording_id=recording_id, event_type="chat.line.created"
                )

        assert project.call_count == 1, "the floor keeps a run of misses from becoming a read per line"
        assert listing.call_count == 1
        assert raised.value.refreshed is False

    @respx.mock
    def test_a_candidate_the_refreshed_sources_dropped_is_reported_as_stale(self):
        # BC3 answers 404 for a Campfire the caller may no longer see, exactly
        # as it does for a line that is not there -- so "unresolved" reports
        # which candidates lost visibility rather than pretending to know.
        clock = [0.0]
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            side_effect=[httpx.Response(200, json=_project(1)), httpx.Response(200, json=_project())]
        )
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = _account(now=lambda: clock[0])

        with pytest.raises(RecordingUnresolvedError):
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        with pytest.raises(RecordingUnresolvedError) as raised:
            account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.refreshed is True
        assert raised.value.stale_campfire_ids == [1]

    @respx.mock
    def test_the_cache_does_not_cross_accounts(self):
        other = "https://3.basecampapi.com/999"
        first = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        second = respx.get(f"{other}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(77)))
        respx.get(url__regex=r"https://3\.basecampapi\.com/\d+/chats/77/lines/\d+").mock(
            return_value=httpx.Response(200, json=_line(77))
        )
        client = _client()

        client.for_account(ACCOUNT).recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )
        client.for_account("999").recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert first.call_count == 1 and second.call_count == 1

    @respx.mock
    def test_a_failed_dock_read_is_raised_and_not_cached(self):
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(403, json={"error": "Access denied"})
        )
        account = _account()

        for _ in range(2):
            with pytest.raises(ForbiddenError):
                account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert project.call_count == 2, "a failed load leaves the cache as it was"

    @respx.mock
    def test_a_second_line_answers_from_the_cached_listing(self):
        # The listing is cached per account, so a burst of lines in a
        # non-project bucket costs one listing, not one per line.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[_campfire(5)]))
        respx.get(url__regex=rf"{BASE}/chats/5/lines/\d+").mock(return_value=httpx.Response(200, json=_line(5)))
        account = _account()

        account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        summary = account.recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID + 1, event_type="chat.line.created"
        )

        assert listing.call_count == 1
        assert summary["campfire_id"] == 5

    @respx.mock
    def test_a_dock_over_the_budget_is_incomplete_before_the_listing_is_fetched(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(200, json=_project(*range(1, MAX_CAMPFIRE_CANDIDATES + 6)))
        )
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        listing = respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))

        with pytest.raises(CampfireDiscoveryIncompleteError):
            _account().recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert not listing.called, "a spent budget must not buy a request that cannot help"


class RecordingHooks(BasecampHooks):
    def __init__(self):
        self.operations: list[str] = []

    def on_operation_start(self, info):
        self.operations.append(f"{info.service}.{info.operation}")


class TestHookIdentities:
    @respx.mock
    def test_hooks_see_the_constituent_reads_under_their_own_names(self):
        # SPEC section 18 rule 3: a composite mints no operation identity.
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[_campfire(5)]))
        respx.get(f"{BASE}/chats/5/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(5)))
        hooks = RecordingHooks()

        _account(hooks=hooks).recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert hooks.operations == ["projects.get", "campfires.list", "campfires.get_line"]
        assert not any("summarize" in operation for operation in hooks.operations)

    @respx.mock
    def test_a_typed_read_reaches_hooks_as_that_service(self):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(200, json={"id": 1, "type": "Comment", "bucket": {"id": BUCKET}})
        )
        hooks = RecordingHooks()

        _account(hooks=hooks).recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

        assert hooks.operations == ["comments.get"]


@pytest.mark.asyncio
class TestAsync:
    @respx.mock
    async def test_routes_a_typed_read(self):
        respx.get(f"{BASE}/comments/1").mock(
            return_value=httpx.Response(
                200, json={"id": 1, "type": "Comment", "content": "hi", "bucket": {"id": BUCKET}}
            )
        )
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        summary = await account.recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="comment.created")

        assert summary["type"] == "Comment"
        assert summary["mentioned_person_ids"] == []

    @respx.mock
    async def test_discovers_a_chat_line_and_caches_the_dock(self):
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1, 2)))
        respx.get(url__regex=rf"{BASE}/chats/1/lines/\d+").mock(return_value=_not_found())
        respx.get(url__regex=rf"{BASE}/chats/2/lines/\d+").mock(return_value=httpx.Response(200, json=_line(2)))
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        summary = await account.recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )
        await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID + 1, event_type="chat.line.created")

        assert summary["campfire_id"] == 2
        assert project.call_count == 1

    @respx.mock
    async def test_unresolved_is_its_own_identity(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(return_value=httpx.Response(200, json=_project(1)))
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        with pytest.raises(RecordingUnresolvedError):
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

    async def test_refuses_an_unroutable_pointer(self):
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)
        with pytest.raises(NoRecordingTypeError):
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=1, event_type="boost.created")

    @respx.mock
    async def test_a_dock_over_the_budget_is_incomplete(self):
        respx.get(f"{BASE}/projects/{BUCKET}").mock(
            return_value=httpx.Response(200, json=_project(*range(1, MAX_CAMPFIRE_CANDIDATES + 6)))
        )
        respx.get(url__regex=rf"{BASE}/chats/\d+/lines/{LINE_ID}").mock(return_value=_not_found())
        account = AsyncClient(access_token="test-token").for_account(ACCOUNT)

        with pytest.raises(CampfireDiscoveryIncompleteError) as raised:
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")

        assert raised.value.code == "campfire_discovery_incomplete"

    @respx.mock
    async def test_a_miss_refreshes_the_cached_dock_before_concluding(self):
        clock = [0.0]
        project = respx.get(f"{BASE}/projects/{BUCKET}").mock(
            side_effect=[httpx.Response(200, json=_project(1)), httpx.Response(200, json=_project(1, 2))]
        )
        respx.get(f"{BASE}/chats/1/lines/{LINE_ID}").mock(return_value=_not_found())
        respx.get(f"{BASE}/chats/2/lines/{LINE_ID}").mock(return_value=httpx.Response(200, json=_line(2)))
        respx.get(f"{BASE}/chats.json").mock(return_value=httpx.Response(200, json=[]))
        client = AsyncClient(access_token="test-token")
        client._campfire_index = AsyncCampfireIndex(now=lambda: clock[0])
        account = client.for_account(ACCOUNT)

        with pytest.raises(RecordingUnresolvedError):
            await account.recordings.summarize(bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created")
        clock[0] = _campfire_index.CAMPFIRE_INDEX_MIN_REFRESH + 1
        summary = await account.recordings.summarize(
            bucket_id=BUCKET, recording_id=LINE_ID, event_type="chat.line.created"
        )

        assert project.call_count == 2
        assert summary["campfire_id"] == 2
