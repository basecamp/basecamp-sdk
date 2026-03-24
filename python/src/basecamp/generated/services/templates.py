# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TemplatesService(BaseService):
    def list(self, *, status: str | None = None) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="templates", operation="list", is_mutation=False),
            "/templates.json",
            params=self._compact(status=status),
        )

    def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="templates", operation="create", is_mutation=True),
            "POST",
            "/templates.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateTemplate",
        )

    def get(self, *, template_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="templates", operation="get", is_mutation=False, resource_id=template_id),
            "GET",
            f"/templates/{template_id}",
        )

    def update(self, *, template_id: int, name: str | None = None, description: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="templates", operation="update", is_mutation=True, resource_id=template_id),
            "PUT",
            f"/templates/{template_id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTemplate",
        )

    def delete(self, *, template_id: int) -> None:
        self._request_void(
            OperationInfo(service="templates", operation="delete", is_mutation=True, resource_id=template_id),
            "DELETE",
            f"/templates/{template_id}",
            operation="DeleteTemplate",
        )

    def create_project(self, *, template_id: int, name: str, description: str | None = None) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="templates", operation="create_project", is_mutation=True, resource_id=template_id),
            "POST",
            f"/templates/{template_id}/project_constructions.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateProjectFromTemplate",
        )

    def get_construction(self, *, template_id: int, construction_id: int) -> dict[str, Any]:
        return self._request(
            OperationInfo(
                service="templates", operation="get_construction", is_mutation=False, resource_id=construction_id
            ),
            "GET",
            f"/templates/{template_id}/project_constructions/{construction_id}",
        )


class AsyncTemplatesService(AsyncBaseService):
    async def list(self, *, status: str | None = None) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="templates", operation="list", is_mutation=False),
            "/templates.json",
            params=self._compact(status=status),
        )

    async def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="templates", operation="create", is_mutation=True),
            "POST",
            "/templates.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateTemplate",
        )

    async def get(self, *, template_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="templates", operation="get", is_mutation=False, resource_id=template_id),
            "GET",
            f"/templates/{template_id}",
        )

    async def update(
        self, *, template_id: int, name: str | None = None, description: str | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="templates", operation="update", is_mutation=True, resource_id=template_id),
            "PUT",
            f"/templates/{template_id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTemplate",
        )

    async def delete(self, *, template_id: int) -> None:
        await self._request_void(
            OperationInfo(service="templates", operation="delete", is_mutation=True, resource_id=template_id),
            "DELETE",
            f"/templates/{template_id}",
            operation="DeleteTemplate",
        )

    async def create_project(self, *, template_id: int, name: str, description: str | None = None) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="templates", operation="create_project", is_mutation=True, resource_id=template_id),
            "POST",
            f"/templates/{template_id}/project_constructions.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateProjectFromTemplate",
        )

    async def get_construction(self, *, template_id: int, construction_id: int) -> dict[str, Any]:
        return await self._request(
            OperationInfo(
                service="templates", operation="get_construction", is_mutation=False, resource_id=construction_id
            ),
            "GET",
            f"/templates/{template_id}/project_constructions/{construction_id}",
        )
