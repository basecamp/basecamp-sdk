"""Tests for the generated gauges service routes (all seven operations)."""

from __future__ import annotations

import json
from pathlib import Path

import httpx
import pytest
import respx

from basecamp import AsyncClient, Client
from basecamp.errors import AuthError, ForbiddenError, NotFoundError, ValidationError
from basecamp.generated.services.gauges import AsyncGaugesService, GaugesService

BASE = "https://3.basecampapi.com/12345"

_FIXTURES = Path(__file__).resolve().parents[3] / "spec" / "fixtures"

_GAUGES_URL = f"{BASE}/reports/gauges.json"
_NEEDLES_URL = f"{BASE}/projects/7/gauge/needles.json"
# GET/PUT/DELETE of a single needle carry NO `.json` suffix. That is deliberate
# in the Smithy spec, so these tests stub the bare path and additionally assert
# the request path verbatim — a silently re-suffixed route would be a contract
# change, not a cosmetic one.
_NEEDLE_PATH = "/gauge_needles/1069479850"
_NEEDLE_URL = f"{BASE}{_NEEDLE_PATH}"
_TOGGLE_URL = f"{BASE}/projects/7/gauge.json"


def _fixture(name: str) -> dict:
    """One of the shared gauge fixtures `make check-fixture-coverage` validates."""
    return json.loads((_FIXTURES / "gauges" / f"{name}.json").read_text(encoding="utf-8"))


def _gauge_stub(gauge_id: int) -> dict:
    """A trimmed gauge, for the pagination tests that only count and identify items."""
    return {"id": gauge_id, "type": "Gauge", "enabled": True, "last_needle_color": "green"}


def _needle_stub(needle_id: int) -> dict:
    return {"id": needle_id, "type": "Gauge::Needle", "color": "green", "position": 50}


def _link_to(url: str) -> dict[str, str]:
    return {"Link": f'<{url}>; rel="next"'}


def _gauges() -> GaugesService:
    return Client(access_token="test-token").for_account("12345").gauges


def _async_gauges() -> AsyncGaugesService:
    return AsyncClient(access_token="test-token").for_account("12345").gauges


class TestListGauges:
    @respx.mock
    def test_lists_a_bare_array_from_the_account_scoped_reports_path(self):
        route = respx.get(_GAUGES_URL).mock(return_value=httpx.Response(200, json=[_fixture("get")]))

        result = _gauges().list_gauges()

        assert route.called
        assert route.calls[0].request.url.path == "/12345/reports/gauges.json"
        assert len(result) == 1
        gauge = result[0]
        assert gauge["id"] == 1069479800
        assert gauge["type"] == "Gauge"
        assert gauge["enabled"] is True
        assert gauge["last_needle_color"] == "green"
        assert gauge["last_needle_position"] == 72
        assert gauge["previous_needle_position"] == 45
        assert gauge["bucket"]["id"] == 2085958500
        # A gauge is a SINGLETON under its project: Gauge#route is
        # [:project_gauge, bucket], so the gauge's own id never appears in its
        # url, and the /buckets/{id}/gauges/{id}.json shape these fixtures used
        # to carry is a route bc3 does not draw. Asserted here because
        # check-fixture-coverage validates fixture shapes, never their values
        # (see issue #733), so nothing else stops that drifting back.
        assert gauge["url"] == "https://3.basecampapi.com/999999999/projects/2085958500/gauge.json"
        assert gauge["app_url"] == "https://3.basecamp.com/999999999/projects/2085958500/gauge"
        assert str(gauge["id"]) not in gauge["url"]
        assert gauge["title"] == "How far along are we?"  # Gauge#title, hard-coded in bc3

    @respx.mock
    def test_sends_no_query_when_neither_bucket_ids_nor_page_is_given(self):
        route = respx.get(_GAUGES_URL).mock(return_value=httpx.Response(200, json=[]))

        _gauges().list_gauges()

        assert dict(route.calls[0].request.url.params) == {}

    @respx.mock
    def test_bucket_ids_reaches_the_wire_under_that_exact_param_name(self):
        route = respx.get(_GAUGES_URL, params={"bucket_ids": "2085958500,2085958501"}).mock(
            return_value=httpx.Response(200, json=[_gauge_stub(1), _gauge_stub(2)])
        )

        result = _gauges().list_gauges(bucket_ids="2085958500,2085958501")

        assert route.call_count == 1
        assert dict(route.calls[0].request.url.params) == {"bucket_ids": "2085958500,2085958501"}
        assert [g["id"] for g in result] == [1, 2]

    @respx.mock
    def test_positive_page_selects_exactly_that_page_and_reports_the_unfollowed_link(self):
        # SPEC section 8: `page` is a selector, not a starting offset. One request
        # is issued, the rel="next" link is NOT followed, and truncated reports it.
        # Page 4 is deliberately unmounted: following the link would fail the test.
        route = respx.get(_GAUGES_URL, params={"page": "3"}).mock(
            return_value=httpx.Response(
                200,
                json=[_gauge_stub(5), _gauge_stub(6)],
                headers={**_link_to(f"{_GAUGES_URL}?page=4"), "X-Total-Count": "9"},
            )
        )

        result = _gauges().list_gauges(page=3)

        assert route.call_count == 1
        assert dict(route.calls[0].request.url.params) == {"page": "3"}
        assert [g["id"] for g in result] == [5, 6]
        assert result.meta.total_count == 9
        assert result.meta.truncated is True

    @respx.mock
    def test_no_page_follows_links_across_pages_until_the_last(self):
        page2 = respx.get(_GAUGES_URL, params={"page": "2"}).mock(
            return_value=httpx.Response(200, json=[_gauge_stub(3)])
        )
        page1 = respx.get(_GAUGES_URL).mock(
            return_value=httpx.Response(
                200, json=[_gauge_stub(1), _gauge_stub(2)], headers=_link_to(f"{_GAUGES_URL}?page=2")
            )
        )

        result = _gauges().list_gauges()

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [g["id"] for g in result] == [1, 2, 3]
        assert result.meta.truncated is False

    @respx.mock
    def test_401_surfaces_as_an_auth_error_carrying_the_status_and_body_message(self):
        route = respx.get(_GAUGES_URL).mock(return_value=httpx.Response(401, json={"error": "Unauthorized"}))

        with pytest.raises(AuthError) as excinfo:
            _gauges().list_gauges()

        assert route.call_count == 1
        assert excinfo.value.http_status == 401
        assert excinfo.value.code == "auth_required"
        assert str(excinfo.value) == "Unauthorized"


class TestListGaugeNeedles:
    @respx.mock
    def test_lists_needles_under_the_project_scoped_gauge_path(self):
        route = respx.get(_NEEDLES_URL).mock(return_value=httpx.Response(200, json=[_fixture("needle_get")]))

        result = _gauges().list_gauge_needles(project_id=7)

        assert route.called
        assert route.calls[0].request.url.path == "/12345/projects/7/gauge/needles.json"
        assert len(result) == 1
        needle = result[0]
        assert needle["id"] == 1069479850
        assert needle["type"] == "Gauge::Needle"
        assert needle["color"] == "green"
        assert needle["position"] == 72
        assert needle["parent"]["id"] == 1069479800

    @respx.mock
    def test_needle_description_attachments_flow_through_including_the_nullable_width(self):
        # bc3 renders blob widths as floats ("1024.0") and omits them as null for
        # non-previewable blobs; both must survive decoding untouched.
        respx.get(_NEEDLES_URL).mock(return_value=httpx.Response(200, json=[_fixture("needle_get")]))

        attachments = _gauges().list_gauge_needles(project_id=7)[0]["description_attachments"]

        assert len(attachments) == 2
        assert attachments[0]["width"] == 1024.0
        # The equality above cannot fail on its own: 1024 == 1024.0 in Python, so
        # an integer-decoding regression would slip straight past it. The type
        # assertion is what actually bites, as in test_uploads.py / test_todos.py.
        assert isinstance(attachments[0]["width"], float)
        assert attachments[1]["width"] is None

    @respx.mock
    def test_positive_page_selects_exactly_that_page_and_reports_the_unfollowed_link(self):
        route = respx.get(_NEEDLES_URL, params={"page": "2"}).mock(
            return_value=httpx.Response(
                200,
                json=[_needle_stub(11), _needle_stub(12)],
                headers={**_link_to(f"{_NEEDLES_URL}?page=3"), "X-Total-Count": "6"},
            )
        )

        result = _gauges().list_gauge_needles(project_id=7, page=2)

        assert route.call_count == 1
        assert dict(route.calls[0].request.url.params) == {"page": "2"}
        assert [n["id"] for n in result] == [11, 12]
        assert result.meta.total_count == 6
        assert result.meta.truncated is True

    @respx.mock
    def test_a_pinned_final_page_without_a_next_link_is_not_truncated(self):
        respx.get(_NEEDLES_URL, params={"page": "9"}).mock(
            return_value=httpx.Response(200, json=[_needle_stub(90)], headers={"X-Total-Count": "81"})
        )

        result = _gauges().list_gauge_needles(project_id=7, page=9)

        assert len(result) == 1
        assert result.meta.total_count == 81
        assert result.meta.truncated is False

    @respx.mock
    def test_no_page_follows_links_across_pages_until_the_last(self):
        page2 = respx.get(_NEEDLES_URL, params={"page": "2"}).mock(
            return_value=httpx.Response(200, json=[_needle_stub(3)])
        )
        page1 = respx.get(_NEEDLES_URL).mock(
            return_value=httpx.Response(
                200, json=[_needle_stub(1), _needle_stub(2)], headers=_link_to(f"{_NEEDLES_URL}?page=2")
            )
        )

        result = _gauges().list_gauge_needles(project_id=7)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [n["id"] for n in result] == [1, 2, 3]
        assert result.meta.truncated is False

    @respx.mock
    def test_404_on_an_unreachable_project_surfaces_as_not_found(self):
        route = respx.get(f"{BASE}/projects/999/gauge/needles.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            _gauges().list_gauge_needles(project_id=999)

        assert route.call_count == 1
        assert excinfo.value.http_status == 404
        assert excinfo.value.code == "not_found"
        assert str(excinfo.value) == "Not found"


class TestGetGaugeNeedle:
    @respx.mock
    def test_gets_a_needle_from_the_suffixless_account_scoped_path(self):
        route = respx.get(_NEEDLE_URL).mock(return_value=httpx.Response(200, json=_fixture("needle_get")))

        needle = _gauges().get_gauge_needle(needle_id=1069479850)

        assert route.call_count == 1
        request = route.calls[0].request
        assert request.method == "GET"
        # No `.json`: the spec models this route without a suffix.
        assert request.url.path == f"/12345{_NEEDLE_PATH}"
        assert not request.url.path.endswith(".json")
        assert needle["id"] == 1069479850
        assert needle["type"] == "Gauge::Needle"
        assert needle["color"] == "green"
        assert needle["position"] == 72
        assert needle["parent"]["id"] == 1069479800
        assert len(needle["description_attachments"]) == 2
        # Gauge::Needle#route is [:project_gauge_needle, bucket, recording], and
        # the parent gauge is the singleton route again. Same reasoning as the
        # gauge url assertion in TestListGauges.
        assert needle["url"] == "https://3.basecampapi.com/999999999/projects/2085958500/gauge/needles/1069479850.json"
        assert needle["title"] == "Moved the needle"  # Gauge::Needle#title, hard-coded in bc3
        assert needle["parent"]["url"] == "https://3.basecampapi.com/999999999/projects/2085958500/gauge.json"
        assert needle["parent"]["title"] == "How far along are we?"
        # The envelope's own URLs stay bucket_recording_*-scoped — those helpers
        # are not the recordable route and must NOT move with it.
        assert "/buckets/2085958500/recordings/" in needle["subscription_url"]
        assert "/buckets/2085958500/recordings/" in needle["comments_url"]

    @respx.mock
    def test_404_surfaces_as_not_found_with_the_status_and_body_message(self):
        route = respx.get(f"{BASE}/gauge_needles/999").mock(
            return_value=httpx.Response(404, json={"error": "Gauge needle not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            _gauges().get_gauge_needle(needle_id=999)

        assert route.call_count == 1
        assert excinfo.value.http_status == 404
        assert excinfo.value.code == "not_found"
        assert str(excinfo.value) == "Gauge needle not found"


class TestCreateGaugeNeedle:
    @respx.mock
    def test_posts_the_needle_under_a_gauge_needle_envelope_and_returns_201(self):
        route = respx.post(_NEEDLES_URL).mock(return_value=httpx.Response(201, json=_fixture("needle_get")))

        needle = _gauges().create_gauge_needle(
            project_id=7,
            gauge_needle={"position": 72, "color": "green", "description": "<div>Shipped it.</div>"},
        )

        assert route.call_count == 1
        request = route.calls[0].request
        assert request.method == "POST"
        assert request.url.path == "/12345/projects/7/gauge/needles.json"
        assert json.loads(request.content) == {
            "gauge_needle": {"position": 72, "color": "green", "description": "<div>Shipped it.</div>"}
        }
        assert needle["id"] == 1069479850
        assert needle["position"] == 72

    @respx.mock
    def test_omitted_notify_and_subscriptions_are_dropped_from_the_body(self):
        # `_compact` drops None members: an omitted notify must not reach the wire
        # as an explicit null, which bc3 would read as a notification target.
        route = respx.post(_NEEDLES_URL).mock(return_value=httpx.Response(201, json=_fixture("needle_get")))

        _gauges().create_gauge_needle(project_id=7, gauge_needle={"position": 72})

        body = json.loads(route.calls[0].request.content)
        assert "notify" not in body
        assert "subscriptions" not in body
        assert body == {"gauge_needle": {"position": 72}}

    @respx.mock
    def test_supplied_notify_and_subscriptions_are_sent_alongside_the_needle(self):
        route = respx.post(_NEEDLES_URL).mock(return_value=httpx.Response(201, json=_fixture("needle_get")))

        _gauges().create_gauge_needle(
            project_id=7,
            gauge_needle={"position": 72},
            notify="custom",
            subscriptions=[1049715915, 1049715916],
        )

        assert json.loads(route.calls[0].request.content) == {
            "gauge_needle": {"position": 72},
            "notify": "custom",
            "subscriptions": [1049715915, 1049715916],
        }

    @respx.mock
    def test_422_surfaces_as_a_validation_error_carrying_the_field_errors(self):
        route = respx.post(_NEEDLES_URL).mock(
            return_value=httpx.Response(422, json={"errors": {"position": ["must be between 0 and 100"]}})
        )

        with pytest.raises(ValidationError) as excinfo:
            _gauges().create_gauge_needle(project_id=7, gauge_needle={"position": 4000})

        assert route.call_count == 1
        assert excinfo.value.http_status == 422
        assert excinfo.value.code == "validation"
        assert excinfo.value.field_errors == {"position": ["must be between 0 and 100"]}
        assert str(excinfo.value) == "position: must be between 0 and 100"


class TestUpdateGaugeNeedle:
    @respx.mock
    def test_puts_the_description_under_a_gauge_needle_envelope_to_the_suffixless_path(self):
        route = respx.put(_NEEDLE_URL).mock(return_value=httpx.Response(200, json=_fixture("needle_get")))

        needle = _gauges().update_gauge_needle(
            needle_id=1069479850, gauge_needle={"description": "<div>Revised.</div>"}
        )

        assert route.call_count == 1
        request = route.calls[0].request
        assert request.method == "PUT"
        assert request.url.path == f"/12345{_NEEDLE_PATH}"
        assert not request.url.path.endswith(".json")
        assert json.loads(request.content) == {"gauge_needle": {"description": "<div>Revised.</div>"}}
        assert needle["id"] == 1069479850

    @respx.mock
    def test_404_surfaces_as_not_found_with_the_status_and_body_message(self):
        route = respx.put(f"{BASE}/gauge_needles/999").mock(
            return_value=httpx.Response(404, json={"error": "Gauge needle not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            _gauges().update_gauge_needle(needle_id=999, gauge_needle={"description": "<div>Revised.</div>"})

        assert route.call_count == 1
        assert excinfo.value.http_status == 404
        assert excinfo.value.code == "not_found"
        assert str(excinfo.value) == "Gauge needle not found"


class TestDestroyGaugeNeedle:
    @respx.mock
    def test_deletes_the_suffixless_path_with_204_and_returns_none(self):
        route = respx.delete(_NEEDLE_URL).mock(return_value=httpx.Response(204))

        assert _gauges().destroy_gauge_needle(needle_id=1069479850) is None

        assert route.call_count == 1
        request = route.calls[0].request
        assert request.method == "DELETE"
        assert request.url.path == f"/12345{_NEEDLE_PATH}"
        assert not request.url.path.endswith(".json")
        assert request.content == b""

    @respx.mock
    def test_404_surfaces_as_not_found_with_the_status_and_body_message(self):
        route = respx.delete(f"{BASE}/gauge_needles/999").mock(
            return_value=httpx.Response(404, json={"error": "Gauge needle not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            _gauges().destroy_gauge_needle(needle_id=999)

        assert route.call_count == 1
        assert excinfo.value.http_status == 404
        assert excinfo.value.code == "not_found"
        assert str(excinfo.value) == "Gauge needle not found"


class TestToggleGauge:
    # bc3's Projects::GaugesController#update answers ``head :ok`` — a 200 with an
    # EMPTY body, not a 204. These stubs say 200 deliberately: an empty 200 is where
    # a void decode can trip over zero-length input, and a 204 (defined to carry no
    # body) would never exercise that path. destroy_gauge_needle really is a 204 —
    # bc3 answers that one with ``head :no_content``.
    @respx.mock
    def test_puts_the_enabled_flag_under_a_gauge_envelope_and_returns_none(self):
        route = respx.put(_TOGGLE_URL).mock(return_value=httpx.Response(200))

        assert _gauges().toggle_gauge(project_id=7, gauge={"enabled": True}) is None

        assert route.call_count == 1
        request = route.calls[0].request
        assert request.method == "PUT"
        assert request.url.path == "/12345/projects/7/gauge.json"
        assert json.loads(request.content) == {"gauge": {"enabled": True}}

    @respx.mock
    def test_an_explicit_false_reaches_the_wire(self):
        # `enabled: False` is a real disable instruction, not an omission: it must
        # survive `_compact` (which only drops None) as a literal false.
        route = respx.put(_TOGGLE_URL).mock(return_value=httpx.Response(200))

        _gauges().toggle_gauge(project_id=7, gauge={"enabled": False})

        body = json.loads(route.calls[0].request.content)
        assert body == {"gauge": {"enabled": False}}
        assert body["gauge"]["enabled"] is False

    @respx.mock
    def test_403_surfaces_as_forbidden_for_a_non_admin(self):
        # Only project admins may toggle a gauge; everyone else gets a 403.
        route = respx.put(_TOGGLE_URL).mock(
            return_value=httpx.Response(403, json={"error": "Only administrators can toggle the gauge"})
        )

        with pytest.raises(ForbiddenError) as excinfo:
            _gauges().toggle_gauge(project_id=7, gauge={"enabled": True})

        assert route.call_count == 1
        assert excinfo.value.http_status == 403
        assert excinfo.value.code == "forbidden"
        assert str(excinfo.value) == "Only administrators can toggle the gauge"


class TestAsyncGauges:
    """The async service has its own base, paginator and `_compact`, so every
    operation is pinned again over `AsyncGaugesService`."""

    @pytest.mark.asyncio
    @respx.mock
    async def test_list_gauges_returns_the_bare_array(self):
        route = respx.get(_GAUGES_URL).mock(return_value=httpx.Response(200, json=[_fixture("get")]))

        result = await _async_gauges().list_gauges()

        assert route.calls[0].request.url.path == "/12345/reports/gauges.json"
        gauge = result[0]
        assert gauge["id"] == 1069479800
        assert gauge["type"] == "Gauge"
        assert gauge["enabled"] is True
        assert gauge["last_needle_color"] == "green"
        assert gauge["last_needle_position"] == 72
        assert gauge["previous_needle_position"] == 45
        assert gauge["bucket"]["id"] == 2085958500

    @pytest.mark.asyncio
    @respx.mock
    async def test_list_gauges_sends_bucket_ids_under_that_exact_param_name(self):
        route = respx.get(_GAUGES_URL, params={"bucket_ids": "2085958500"}).mock(
            return_value=httpx.Response(200, json=[_gauge_stub(1)])
        )

        await _async_gauges().list_gauges(bucket_ids="2085958500")

        assert route.call_count == 1
        assert dict(route.calls[0].request.url.params) == {"bucket_ids": "2085958500"}

    @pytest.mark.asyncio
    @respx.mock
    async def test_list_gauges_401_surfaces_as_an_auth_error(self):
        respx.get(_GAUGES_URL).mock(return_value=httpx.Response(401, json={"error": "Unauthorized"}))

        with pytest.raises(AuthError) as excinfo:
            await _async_gauges().list_gauges()

        assert excinfo.value.http_status == 401
        assert excinfo.value.code == "auth_required"
        assert str(excinfo.value) == "Unauthorized"

    @pytest.mark.asyncio
    @respx.mock
    async def test_list_gauge_needles_positive_page_selects_exactly_that_page(self):
        route = respx.get(_NEEDLES_URL, params={"page": "2"}).mock(
            return_value=httpx.Response(
                200,
                json=[_needle_stub(11), _needle_stub(12)],
                headers={**_link_to(f"{_NEEDLES_URL}?page=3"), "X-Total-Count": "6"},
            )
        )

        result = await _async_gauges().list_gauge_needles(project_id=7, page=2)

        assert route.call_count == 1
        assert [n["id"] for n in result] == [11, 12]
        assert result.meta.total_count == 6
        assert result.meta.truncated is True

    @pytest.mark.asyncio
    @respx.mock
    async def test_list_gauge_needles_without_page_follows_links(self):
        page2 = respx.get(_NEEDLES_URL, params={"page": "2"}).mock(
            return_value=httpx.Response(200, json=[_needle_stub(3)])
        )
        page1 = respx.get(_NEEDLES_URL).mock(
            return_value=httpx.Response(
                200, json=[_needle_stub(1), _needle_stub(2)], headers=_link_to(f"{_NEEDLES_URL}?page=2")
            )
        )

        result = await _async_gauges().list_gauge_needles(project_id=7)

        assert page1.call_count == 1
        assert page2.call_count == 1
        assert [n["id"] for n in result] == [1, 2, 3]
        assert result.meta.truncated is False

    @pytest.mark.asyncio
    @respx.mock
    async def test_list_gauge_needles_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/projects/999/gauge/needles.json").mock(
            return_value=httpx.Response(404, json={"error": "Not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            await _async_gauges().list_gauge_needles(project_id=999)

        assert excinfo.value.http_status == 404
        assert str(excinfo.value) == "Not found"

    @pytest.mark.asyncio
    @respx.mock
    async def test_get_gauge_needle_uses_the_suffixless_path(self):
        route = respx.get(_NEEDLE_URL).mock(return_value=httpx.Response(200, json=_fixture("needle_get")))

        needle = await _async_gauges().get_gauge_needle(needle_id=1069479850)

        assert route.calls[0].request.url.path == f"/12345{_NEEDLE_PATH}"
        assert not route.calls[0].request.url.path.endswith(".json")
        assert needle["id"] == 1069479850
        assert needle["color"] == "green"
        assert needle["position"] == 72
        assert needle["parent"]["id"] == 1069479800

    @pytest.mark.asyncio
    @respx.mock
    async def test_get_gauge_needle_404_surfaces_as_not_found(self):
        respx.get(f"{BASE}/gauge_needles/999").mock(
            return_value=httpx.Response(404, json={"error": "Gauge needle not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            await _async_gauges().get_gauge_needle(needle_id=999)

        assert excinfo.value.http_status == 404
        assert excinfo.value.code == "not_found"
        assert str(excinfo.value) == "Gauge needle not found"

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_gauge_needle_envelopes_the_needle_and_drops_omitted_members(self):
        route = respx.post(_NEEDLES_URL).mock(return_value=httpx.Response(201, json=_fixture("needle_get")))

        needle = await _async_gauges().create_gauge_needle(project_id=7, gauge_needle={"position": 72})

        body = json.loads(route.calls[0].request.content)
        assert body == {"gauge_needle": {"position": 72}}
        assert "notify" not in body
        assert "subscriptions" not in body
        assert needle["id"] == 1069479850

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_gauge_needle_sends_notify_and_subscriptions_when_supplied(self):
        route = respx.post(_NEEDLES_URL).mock(return_value=httpx.Response(201, json=_fixture("needle_get")))

        await _async_gauges().create_gauge_needle(
            project_id=7, gauge_needle={"position": 72}, notify="custom", subscriptions=[1049715915]
        )

        assert json.loads(route.calls[0].request.content) == {
            "gauge_needle": {"position": 72},
            "notify": "custom",
            "subscriptions": [1049715915],
        }

    @pytest.mark.asyncio
    @respx.mock
    async def test_create_gauge_needle_422_surfaces_as_a_validation_error(self):
        respx.post(_NEEDLES_URL).mock(
            return_value=httpx.Response(422, json={"errors": {"position": ["must be between 0 and 100"]}})
        )

        with pytest.raises(ValidationError) as excinfo:
            await _async_gauges().create_gauge_needle(project_id=7, gauge_needle={"position": 4000})

        assert excinfo.value.http_status == 422
        assert excinfo.value.field_errors == {"position": ["must be between 0 and 100"]}
        assert str(excinfo.value) == "position: must be between 0 and 100"

    @pytest.mark.asyncio
    @respx.mock
    async def test_update_gauge_needle_puts_the_envelope_to_the_suffixless_path(self):
        route = respx.put(_NEEDLE_URL).mock(return_value=httpx.Response(200, json=_fixture("needle_get")))

        needle = await _async_gauges().update_gauge_needle(
            needle_id=1069479850, gauge_needle={"description": "<div>Revised.</div>"}
        )

        request = route.calls[0].request
        assert request.url.path == f"/12345{_NEEDLE_PATH}"
        assert not request.url.path.endswith(".json")
        assert json.loads(request.content) == {"gauge_needle": {"description": "<div>Revised.</div>"}}
        assert needle["id"] == 1069479850

    @pytest.mark.asyncio
    @respx.mock
    async def test_update_gauge_needle_404_surfaces_as_not_found(self):
        respx.put(f"{BASE}/gauge_needles/999").mock(
            return_value=httpx.Response(404, json={"error": "Gauge needle not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            await _async_gauges().update_gauge_needle(needle_id=999, gauge_needle={"description": "<div>x</div>"})

        assert excinfo.value.http_status == 404
        assert str(excinfo.value) == "Gauge needle not found"

    @pytest.mark.asyncio
    @respx.mock
    async def test_destroy_gauge_needle_deletes_the_suffixless_path_with_204(self):
        route = respx.delete(_NEEDLE_URL).mock(return_value=httpx.Response(204))

        assert await _async_gauges().destroy_gauge_needle(needle_id=1069479850) is None

        request = route.calls[0].request
        assert request.method == "DELETE"
        assert request.url.path == f"/12345{_NEEDLE_PATH}"
        assert not request.url.path.endswith(".json")

    @pytest.mark.asyncio
    @respx.mock
    async def test_destroy_gauge_needle_404_surfaces_as_not_found(self):
        respx.delete(f"{BASE}/gauge_needles/999").mock(
            return_value=httpx.Response(404, json={"error": "Gauge needle not found"})
        )

        with pytest.raises(NotFoundError) as excinfo:
            await _async_gauges().destroy_gauge_needle(needle_id=999)

        assert excinfo.value.http_status == 404
        assert str(excinfo.value) == "Gauge needle not found"

    @pytest.mark.asyncio
    @respx.mock
    async def test_toggle_gauge_puts_the_enabled_flag_under_a_gauge_envelope(self):
        route = respx.put(_TOGGLE_URL).mock(return_value=httpx.Response(200))

        assert await _async_gauges().toggle_gauge(project_id=7, gauge={"enabled": False}) is None

        request = route.calls[0].request
        assert request.method == "PUT"
        assert request.url.path == "/12345/projects/7/gauge.json"
        body = json.loads(request.content)
        assert body == {"gauge": {"enabled": False}}
        assert body["gauge"]["enabled"] is False

    @pytest.mark.asyncio
    @respx.mock
    async def test_toggle_gauge_403_surfaces_as_forbidden(self):
        respx.put(_TOGGLE_URL).mock(
            return_value=httpx.Response(403, json={"error": "Only administrators can toggle the gauge"})
        )

        with pytest.raises(ForbiddenError) as excinfo:
            await _async_gauges().toggle_gauge(project_id=7, gauge={"enabled": True})

        assert excinfo.value.http_status == 403
        assert excinfo.value.code == "forbidden"
        assert str(excinfo.value) == "Only administrators can toggle the gauge"
