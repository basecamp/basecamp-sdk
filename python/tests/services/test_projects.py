from __future__ import annotations

import json

import httpx
import pytest
import respx

from basecamp import AsyncClient
from basecamp.client import Client
from basecamp.errors import ErrorCode, ForbiddenError, LimitExceededError, NotFoundError, ValidationError

BASE = "https://3.basecampapi.com/12345"


def make_account():
    client = Client(access_token="test-token")
    return client, client.for_account("12345")


class TestProjects:
    @respx.mock
    def test_get_project_includes_schedule_dates(self):
        respx.get("https://3.basecampapi.com/12345/projects/42").mock(
            return_value=httpx.Response(
                200,
                json={
                    "id": 42,
                    "name": "My Project",
                    "status": "active",
                    "start_date": "2024-01-01",
                    "end_date": "2024-03-31",
                    "created_at": "2024-01-15T10:00:00Z",
                    "updated_at": "2024-01-15T10:00:00Z",
                    "url": "https://3.basecampapi.com/12345/projects/42.json",
                    "app_url": "https://3.basecamp.com/12345/projects/42",
                    "star_url": "https://3.basecampapi.com/12345/buckets/42/stars.json",
                    "bookmarked": True,
                    "starred": False,
                },
            )
        )

        client, account = make_account()
        project = account.projects.get(project_id=42)
        client.close()

        assert project["start_date"] == "2024-01-01"
        assert project["end_date"] == "2024-03-31"
        assert project["star_url"] == "https://3.basecampapi.com/12345/buckets/42/stars.json"
        # starred implies bookmarked, never the reverse: pinned but unstarred is the discriminating case.
        assert project["bookmarked"] is True
        assert project["starred"] is False

    @respx.mock
    def test_list_projects_includes_schedule_dates(self):
        respx.get("https://3.basecampapi.com/12345/projects.json").mock(
            return_value=httpx.Response(
                200,
                json=[
                    {
                        "id": 1,
                        "name": "Project A",
                        "status": "active",
                        "start_date": "2024-01-01",
                        "end_date": "2024-03-31",
                        "created_at": "2024-01-15T10:00:00Z",
                        "updated_at": "2024-01-15T10:00:00Z",
                        "url": "https://3.basecampapi.com/12345/projects/1.json",
                        "app_url": "https://3.basecamp.com/12345/projects/1",
                    }
                ],
            )
        )

        client, account = make_account()
        projects = account.projects.list()
        client.close()

        assert projects[0]["start_date"] == "2024-01-01"
        assert projects[0]["end_date"] == "2024-03-31"

    @respx.mock
    def test_list_recent_projects(self):
        # The recently-visited list is the standard projection plus bookmarked
        # only — the wire omits starred here (BC3 #13043).
        def recent(project_id, bookmarked):
            return {
                "id": project_id,
                "name": f"Project {project_id}",
                "status": "active",
                "created_at": "2024-01-15T10:00:00Z",
                "updated_at": "2024-01-15T10:00:00Z",
                "url": f"https://3.basecampapi.com/12345/projects/{project_id}.json",
                "app_url": f"https://3.basecamp.com/12345/projects/{project_id}",
                "bookmarked": bookmarked,
            }

        respx.get(f"{BASE}/my/recent_projects.json").mock(
            return_value=httpx.Response(200, json=[recent(2, True), recent(1, False)])
        )

        client, account = make_account()
        projects = account.projects.list_recent_projects()
        client.close()

        assert [p["id"] for p in projects] == [2, 1]
        assert [p["bookmarked"] for p in projects] == [True, False]
        assert all("starred" not in p for p in projects)

    @respx.mock
    def test_list_recent_projects_forbidden(self):
        respx.get(f"{BASE}/my/recent_projects.json").mock(return_value=httpx.Response(403, json={"error": "Forbidden"}))

        client, account = make_account()
        with pytest.raises(ForbiddenError) as excinfo:
            account.projects.list_recent_projects()
        client.close()

        assert excinfo.value.http_status == 403

    @respx.mock
    def test_record_project_visit(self):
        route = respx.post(f"{BASE}/projects/42/recent_visit.json").mock(return_value=httpx.Response(204))

        client, account = make_account()
        assert account.projects.record_project_visit(project_id=42) is None
        client.close()

        assert route.call_count == 1

    @respx.mock
    def test_record_project_visit_not_found(self):
        # An inaccessible project answers 404; archived/trashed ones still 204.
        respx.post(f"{BASE}/projects/999/recent_visit.json").mock(return_value=httpx.Response(404))

        client, account = make_account()
        with pytest.raises(NotFoundError) as excinfo:
            account.projects.record_project_visit(project_id=999)
        client.close()

        assert excinfo.value.http_status == 404

    @respx.mock
    def test_join_project(self):
        route = respx.post(f"{BASE}/buckets/42/admissions.json").mock(return_value=httpx.Response(201))

        client, account = make_account()
        assert account.projects.join_project(project_id=42) is None
        client.close()

        assert route.call_count == 1
        assert route.calls.last.request.content == b""

    @respx.mock
    def test_join_project_already_on_project_302_is_success(self):
        # A person who already has access gets a 302 to the project. The client
        # follows no redirect and a 3xx is not an error, so the join returns.
        route = respx.post(f"{BASE}/buckets/42/admissions.json").mock(
            return_value=httpx.Response(302, headers={"Location": f"{BASE}/projects/42"})
        )
        followed = respx.get(f"{BASE}/projects/42").mock(return_value=httpx.Response(200, json={"id": 42}))

        client, account = make_account()
        assert account.projects.join_project(project_id=42) is None
        client.close()

        assert route.call_count == 1
        assert followed.call_count == 0

    @respx.mock
    def test_join_project_not_found(self):
        # A missing project, or one whose policy doesn't admit the person.
        respx.post(f"{BASE}/buckets/999/admissions.json").mock(return_value=httpx.Response(404))

        client, account = make_account()
        with pytest.raises(NotFoundError) as excinfo:
            account.projects.join_project(project_id=999)
        client.close()

        assert excinfo.value.http_status == 404

    @respx.mock
    def test_list_projects_forwards_archived_status_filter(self):
        route = respx.get(f"{BASE}/projects.json", params={"status": "archived"}).mock(
            return_value=httpx.Response(200, json=[])
        )

        client, account = make_account()
        account.projects.list(status="archived")
        client.close()

        assert route.call_count == 1

    @respx.mock
    def test_list_projects_forwards_explicit_active_status(self):
        # active is a server-accepted alias of the unfiltered default.
        route = respx.get(f"{BASE}/projects.json", params={"status": "active"}).mock(
            return_value=httpx.Response(200, json=[])
        )

        client, account = make_account()
        account.projects.list(status="active")
        client.close()

        assert route.call_count == 1

    @respx.mock
    def test_list_projects_default_sends_no_status_param(self):
        route = respx.get(f"{BASE}/projects.json").mock(return_value=httpx.Response(200, json=[]))

        client, account = make_account()
        account.projects.list()
        client.close()

        assert route.call_count == 1
        assert "status" not in route.calls.last.request.url.params


PROJECT_LIMIT_BODY = {"error": "The project limit for this account has been reached."}


class TestSyncStatusTransitions:
    @respx.mock
    def test_archive_project(self):
        route = respx.put(f"{BASE}/projects/42/status/archived.json").mock(return_value=httpx.Response(204))

        client, account = make_account()
        assert account.projects.archive(project_id=42) is None
        client.close()

        assert route.call_count == 1

    @respx.mock
    def test_unarchive_project(self):
        route = respx.put(f"{BASE}/projects/42/status/active.json").mock(return_value=httpx.Response(204))

        client, account = make_account()
        assert account.projects.unarchive(project_id=42) is None
        client.close()

        assert route.call_count == 1

    # The admin pro pack can limit archiving to admins and the project's creator,
    # which bc3 answers with `head :forbidden`.
    @respx.mock
    def test_archive_project_forbidden(self):
        respx.put(f"{BASE}/projects/42/status/archived.json").mock(return_value=httpx.Response(403))

        client, account = make_account()
        with pytest.raises(ForbiddenError) as excinfo:
            account.projects.archive(project_id=42)
        client.close()

        assert excinfo.value.http_status == 403

    # The only behavioural evidence for ProjectLimitError. A 507 is an account
    # limit, so it maps to limit_exceeded and is NOT retryable — no backoff frees
    # a project slot (SPEC.md §6, step 11).
    #
    # The divergence this comment used to record is gone. Python's fallback arm
    # produced retryable=False here while the other five marked every
    # unclassified 5xx retryable; now all six classify 507 the same way, and
    # False is the agreed answer rather than an accident of which arm caught it.
    @respx.mock
    def test_unarchive_project_at_project_limit(self):
        respx.put(f"{BASE}/projects/42/status/active.json").mock(
            return_value=httpx.Response(507, json=PROJECT_LIMIT_BODY)
        )

        client, account = make_account()
        with pytest.raises(LimitExceededError) as excinfo:
            account.projects.unarchive(project_id=42)
        client.close()

        assert excinfo.value.http_status == 507
        assert excinfo.value.code == ErrorCode.LIMIT_EXCEEDED
        assert excinfo.value.retryable is False


class TestAsyncStatusTransitions:
    @pytest.mark.asyncio
    @respx.mock
    async def test_archive_project(self):
        route = respx.put(f"{BASE}/projects/42/status/archived.json").mock(return_value=httpx.Response(204))

        account = AsyncClient(access_token="test-token").for_account("12345")
        assert await account.projects.archive(project_id=42) is None

        assert route.call_count == 1

    @pytest.mark.asyncio
    @respx.mock
    async def test_unarchive_project(self):
        route = respx.put(f"{BASE}/projects/42/status/active.json").mock(return_value=httpx.Response(204))

        account = AsyncClient(access_token="test-token").for_account("12345")
        assert await account.projects.unarchive(project_id=42) is None

        assert route.call_count == 1

    @pytest.mark.asyncio
    @respx.mock
    async def test_archive_project_forbidden(self):
        respx.put(f"{BASE}/projects/42/status/archived.json").mock(return_value=httpx.Response(403))

        account = AsyncClient(access_token="test-token").for_account("12345")
        with pytest.raises(ForbiddenError) as excinfo:
            await account.projects.archive(project_id=42)

        assert excinfo.value.http_status == 403

    @pytest.mark.asyncio
    @respx.mock
    async def test_unarchive_project_at_project_limit(self):
        respx.put(f"{BASE}/projects/42/status/active.json").mock(
            return_value=httpx.Response(507, json=PROJECT_LIMIT_BODY)
        )

        account = AsyncClient(access_token="test-token").for_account("12345")
        with pytest.raises(LimitExceededError) as excinfo:
            await account.projects.unarchive(project_id=42)

        assert excinfo.value.http_status == 507
        assert excinfo.value.code == ErrorCode.LIMIT_EXCEEDED
        assert excinfo.value.retryable is False


class TestProjectWallpaper:
    @respx.mock
    def test_update_project_wallpaper_sends_multipart_with_mode(self):
        route = respx.put(f"{BASE}/projects/42/wallpaper.json").mock(return_value=httpx.Response(204))

        _, account = make_account()
        result = account.projects.update_project_wallpaper(
            project_id=42, content=b"fake-webp-data", filename="beach.webp", content_type="image/webp", mode="tile"
        )

        assert result is None
        request = route.calls[0].request
        assert request.url.params["mode"] == "tile"
        assert request.headers["content-type"].startswith("multipart/form-data")
        assert b'name="wallpaper"; filename="beach.webp"' in request.content
        assert b"fake-webp-data" in request.content

    @respx.mock
    def test_update_project_wallpaper_omits_mode_when_absent(self):
        route = respx.put(f"{BASE}/projects/42/wallpaper.json").mock(return_value=httpx.Response(204))

        _, account = make_account()
        account.projects.update_project_wallpaper(
            project_id=42, content=b"data", filename="beach.png", content_type="image/png"
        )

        assert "mode" not in route.calls[0].request.url.params

    @respx.mock
    def test_update_project_wallpaper_refused_format(self):
        respx.put(f"{BASE}/projects/42/wallpaper.json").mock(
            return_value=httpx.Response(422, json={"error": "Wallpaper is not an allowed image type"})
        )

        _, account = make_account()
        with pytest.raises(ValidationError) as excinfo:
            account.projects.update_project_wallpaper(
                project_id=42, content=b"not an image", filename="a.txt", content_type="text/plain"
            )

        assert excinfo.value.http_status == 422

    @respx.mock
    def test_remove_project_wallpaper(self):
        route = respx.delete(f"{BASE}/projects/42/wallpaper.json").mock(return_value=httpx.Response(204))

        _, account = make_account()
        assert account.projects.remove_project_wallpaper(project_id=42) is None
        assert route.call_count == 1

    @respx.mock
    def test_remove_project_wallpaper_forbidden(self):
        respx.delete(f"{BASE}/projects/42/wallpaper.json").mock(return_value=httpx.Response(403))

        _, account = make_account()
        with pytest.raises(ForbiddenError):
            account.projects.remove_project_wallpaper(project_id=42)

    @respx.mock
    def test_get_project_includes_wallpaper(self):
        wallpaper = {"url": f"{BASE}/projects/42/wallpaper?v=1669139787", "mode": "fill"}
        respx.get(f"{BASE}/projects/42").mock(
            return_value=httpx.Response(
                200,
                json={
                    "id": 42,
                    "name": "My Project",
                    "status": "active",
                    "created_at": "2024-01-15T10:00:00Z",
                    "updated_at": "2024-01-15T10:00:00Z",
                    "url": f"{BASE}/projects/42.json",
                    "app_url": "https://3.basecamp.com/12345/projects/42",
                    "wallpaper": wallpaper,
                },
            )
        )

        _, account = make_account()
        assert account.projects.get(project_id=42)["wallpaper"] == wallpaper

    @respx.mock
    def test_update_project_sends_wallpaper_mode(self):
        route = respx.put(f"{BASE}/projects/42").mock(
            return_value=httpx.Response(
                200,
                json={
                    "id": 42,
                    "name": "My Project",
                    "status": "active",
                    "created_at": "2024-01-15T10:00:00Z",
                    "updated_at": "2024-01-15T10:00:00Z",
                    "url": f"{BASE}/projects/42.json",
                    "app_url": "https://3.basecamp.com/12345/projects/42",
                },
            )
        )

        _, account = make_account()
        account.projects.update(project_id=42, name="My Project", wallpaper_mode="tile")

        assert json.loads(route.calls[0].request.content)["wallpaper_mode"] == "tile"

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_update_project_wallpaper_sends_mode(self):
        route = respx.put(f"{BASE}/projects/42/wallpaper.json").mock(return_value=httpx.Response(204))

        account = AsyncClient(access_token="test-token").for_account("12345")
        await account.projects.update_project_wallpaper(
            project_id=42, content=b"data", filename="beach.avif", content_type="image/avif", mode="fill"
        )

        assert route.calls[0].request.url.params["mode"] == "fill"
        assert b'name="wallpaper"' in route.calls[0].request.content

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_remove_project_wallpaper(self):
        route = respx.delete(f"{BASE}/projects/42/wallpaper.json").mock(return_value=httpx.Response(204))

        account = AsyncClient(access_token="test-token").for_account("12345")
        assert await account.projects.remove_project_wallpaper(project_id=42) is None
        assert route.call_count == 1


class TestProjectLogo:
    @respx.mock
    def test_update_project_logo_sends_multipart(self):
        route = respx.put(f"{BASE}/projects/42/logo.json").mock(return_value=httpx.Response(204))

        _, account = make_account()
        result = account.projects.update_project_logo(
            project_id=42, content=b"fake-webp-data", filename="beach.webp", content_type="image/webp"
        )

        assert result is None
        request = route.calls[0].request
        assert request.headers["content-type"].startswith("multipart/form-data")
        assert b'name="logo"; filename="beach.webp"' in request.content
        assert b"fake-webp-data" in request.content

    @respx.mock
    def test_update_project_logo_omits_mode_when_absent(self):
        route = respx.put(f"{BASE}/projects/42/logo.json").mock(return_value=httpx.Response(204))

        _, account = make_account()
        account.projects.update_project_logo(
            project_id=42, content=b"data", filename="beach.png", content_type="image/png"
        )

        assert "mode" not in route.calls[0].request.url.params

    @respx.mock
    def test_update_project_logo_refused_format(self):
        respx.put(f"{BASE}/projects/42/logo.json").mock(
            return_value=httpx.Response(422, json={"error": "Logo is not an allowed image type"})
        )

        _, account = make_account()
        with pytest.raises(ValidationError) as excinfo:
            account.projects.update_project_logo(
                project_id=42, content=b"not an image", filename="a.txt", content_type="text/plain"
            )

        assert excinfo.value.http_status == 422

    @respx.mock
    def test_remove_project_logo(self):
        route = respx.delete(f"{BASE}/projects/42/logo.json").mock(return_value=httpx.Response(204))

        _, account = make_account()
        assert account.projects.remove_project_logo(project_id=42) is None
        assert route.call_count == 1

    @respx.mock
    def test_remove_project_logo_forbidden(self):
        respx.delete(f"{BASE}/projects/42/logo.json").mock(return_value=httpx.Response(403))

        _, account = make_account()
        with pytest.raises(ForbiddenError):
            account.projects.remove_project_logo(project_id=42)

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_update_project_logo(self):
        route = respx.put(f"{BASE}/projects/42/logo.json").mock(return_value=httpx.Response(204))

        account = AsyncClient(access_token="test-token").for_account("12345")
        await account.projects.update_project_logo(
            project_id=42, content=b"data", filename="beach.avif", content_type="image/avif"
        )

        assert b'name="logo"' in route.calls[0].request.content

    @pytest.mark.asyncio
    @respx.mock
    async def test_async_remove_project_logo(self):
        route = respx.delete(f"{BASE}/projects/42/logo.json").mock(return_value=httpx.Response(204))

        account = AsyncClient(access_token="test-token").for_account("12345")
        assert await account.projects.remove_project_logo(project_id=42) is None
        assert route.call_count == 1
