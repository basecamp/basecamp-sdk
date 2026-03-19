# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ProjectsService(BaseService):
    def list(self, *, status: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="projects", operation="list", is_mutation=False),
            "/projects.json",
            params=self._compact(status=status),
        )

    def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="projects", operation="create", is_mutation=True),
            "POST",
            "/projects.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateProject",
        )

    def get(self, *, project_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="projects", operation="get", is_mutation=False, project_id=project_id),
            "GET",
            f"/projects/{project_id}",
        )

    def update(
        self,
        *,
        project_id: int | str,
        name: str,
        description: str | None = None,
        admissions: str | None = None,
        schedule_attributes: dict | None = None,
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="projects", operation="update", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}",
            json_body=self._compact(
                name=name, description=description, admissions=admissions, schedule_attributes=schedule_attributes
            ),
            operation="UpdateProject",
        )

    def trash(self, *, project_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="projects", operation="trash", is_mutation=True, project_id=project_id),
            "DELETE",
            f"/projects/{project_id}",
            operation="TrashProject",
        )


class AsyncProjectsService(AsyncBaseService):
    async def list(self, *, status: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="projects", operation="list", is_mutation=False),
            "/projects.json",
            params=self._compact(status=status),
        )

    async def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="projects", operation="create", is_mutation=True),
            "POST",
            "/projects.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateProject",
        )

    async def get(self, *, project_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="projects", operation="get", is_mutation=False, project_id=project_id),
            "GET",
            f"/projects/{project_id}",
        )

    async def update(
        self,
        *,
        project_id: int | str,
        name: str,
        description: str | None = None,
        admissions: str | None = None,
        schedule_attributes: dict | None = None,
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="projects", operation="update", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}",
            json_body=self._compact(
                name=name, description=description, admissions=admissions, schedule_attributes=schedule_attributes
            ),
            operation="UpdateProject",
        )

    async def trash(self, *, project_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="projects", operation="trash", is_mutation=True, project_id=project_id),
            "DELETE",
            f"/projects/{project_id}",
            operation="TrashProject",
        )
