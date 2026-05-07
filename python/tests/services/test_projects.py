from __future__ import annotations

import httpx
import respx

from basecamp.client import Client


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
