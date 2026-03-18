# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class PeopleService(BaseService):
    def list_pingable(self) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="people", operation="list_pingable", is_mutation=False), "/circles/people.json"
        )

    def my_profile(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="people", operation="my_profile", is_mutation=False), "GET", "/my/profile.json"
        )

    def update_my_profile(
        self,
        *,
        name: str | None = None,
        email_address: str | None = None,
        title: str | None = None,
        bio: str | None = None,
        location: str | None = None,
        time_zone_name: str | None = None,
        first_week_day: dict | None = None,
        time_format: str | None = None,
    ) -> None:
        self._request_void(
            OperationInfo(service="people", operation="update_my_profile", is_mutation=True),
            "PUT",
            "/my/profile.json",
            json_body=self._compact(
                name=name,
                email_address=email_address,
                title=title,
                bio=bio,
                location=location,
                time_zone_name=time_zone_name,
                first_week_day=first_week_day,
                time_format=time_format,
            ),
            operation="UpdateMyProfile",
        )

    def list(self) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="people", operation="list", is_mutation=False), "/people.json"
        )

    def get(self, *, person_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="people", operation="get", is_mutation=False, resource_id=person_id),
            "GET",
            f"/people/{person_id}",
        )

    def list_for_project(self, *, project_id: int | str) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="people", operation="list_for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/people.json",
        )

    def update_project_access(
        self,
        *,
        project_id: int | str,
        grant: list | None = None,
        revoke: list | None = None,
        create: list | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="people", operation="update_project_access", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/people/users.json",
            json_body=self._compact(grant=grant, revoke=revoke, create=create),
            operation="UpdateProjectAccess",
        )

    def list_assignable(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="people", operation="list_assignable", is_mutation=False),
            "GET",
            "/reports/todos/assigned.json",
        )


class AsyncPeopleService(AsyncBaseService):
    async def list_pingable(self) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="people", operation="list_pingable", is_mutation=False), "/circles/people.json"
        )

    async def my_profile(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="people", operation="my_profile", is_mutation=False), "GET", "/my/profile.json"
        )

    async def update_my_profile(
        self,
        *,
        name: str | None = None,
        email_address: str | None = None,
        title: str | None = None,
        bio: str | None = None,
        location: str | None = None,
        time_zone_name: str | None = None,
        first_week_day: dict | None = None,
        time_format: str | None = None,
    ) -> None:
        await self._request_void(
            OperationInfo(service="people", operation="update_my_profile", is_mutation=True),
            "PUT",
            "/my/profile.json",
            json_body=self._compact(
                name=name,
                email_address=email_address,
                title=title,
                bio=bio,
                location=location,
                time_zone_name=time_zone_name,
                first_week_day=first_week_day,
                time_format=time_format,
            ),
            operation="UpdateMyProfile",
        )

    async def list(self) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="people", operation="list", is_mutation=False), "/people.json"
        )

    async def get(self, *, person_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="people", operation="get", is_mutation=False, resource_id=person_id),
            "GET",
            f"/people/{person_id}",
        )

    async def list_for_project(self, *, project_id: int | str) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="people", operation="list_for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/people.json",
        )

    async def update_project_access(
        self,
        *,
        project_id: int | str,
        grant: list | None = None,
        revoke: list | None = None,
        create: list | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="people", operation="update_project_access", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/people/users.json",
            json_body=self._compact(grant=grant, revoke=revoke, create=create),
            operation="UpdateProjectAccess",
        )

    async def list_assignable(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="people", operation="list_assignable", is_mutation=False),
            "GET",
            "/reports/todos/assigned.json",
        )
