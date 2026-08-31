from __future__ import annotations

import httpx
import pytest
import respx

from basecamp import AsyncClient
from basecamp.client import Client
from basecamp.errors import ErrorCode, ForbiddenError, LimitExceededError

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
                },
            )
        )

        client, account = make_account()
        project = account.projects.get(project_id=42)
        client.close()

        assert project["start_date"] == "2024-01-01"
        assert project["end_date"] == "2024-03-31"

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
