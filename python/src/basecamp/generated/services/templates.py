# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TemplatesService(BaseService):
    def list(self, *, status: str | None = None, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all templates visible to the current user.

        Args:
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="templates", operation="list", is_mutation=False),
            "/templates.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListTemplates",
        )

    def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        """Create a new template.

        Args:
            name: The name.
            description: The description.
        """
        return self._request(
            OperationInfo(service="templates", operation="create", is_mutation=True),
            "POST",
            "/templates.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateTemplate",
        )

    def get(self, *, template_id: int) -> dict[str, Any]:
        """Get a single template by id.

        Args:
            template_id: The template id.
        """
        return self._request(
            OperationInfo(service="templates", operation="get", is_mutation=False, resource_id=template_id),
            "GET",
            f"/templates/{template_id}",
            operation="GetTemplate",
        )

    def update(self, *, template_id: int, name: str | None = None, description: str | None = None) -> dict[str, Any]:
        """Update an existing template.

        Args:
            template_id: The template id.
            name: The name.
            description: The description.
        """
        return self._request(
            OperationInfo(service="templates", operation="update", is_mutation=True, resource_id=template_id),
            "PUT",
            f"/templates/{template_id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTemplate",
        )

    def delete(self, *, template_id: int) -> None:
        """Delete a template (trash it).

        Args:
            template_id: The template id.
        """
        self._request_void(
            OperationInfo(service="templates", operation="delete", is_mutation=True, resource_id=template_id),
            "DELETE",
            f"/templates/{template_id}",
            operation="DeleteTemplate",
        )

    def create_project(self, *, template_id: int, project: dict) -> dict[str, Any]:
        """Create a project from a template (asynchronous).

        Args:
            template_id: The template id.
            project: The project.
        """
        return self._request(
            OperationInfo(service="templates", operation="create_project", is_mutation=True, resource_id=template_id),
            "POST",
            f"/templates/{template_id}/project_constructions.json",
            json_body=self._compact(project=project),
            operation="CreateProjectFromTemplate",
        )

    def get_construction(self, *, template_id: int, construction_id: int) -> dict[str, Any]:
        """Get the status of a project construction.

        Args:
            template_id: The template id.
            construction_id: The construction id.
        """
        return self._request(
            OperationInfo(
                service="templates", operation="get_construction", is_mutation=False, resource_id=construction_id
            ),
            "GET",
            f"/templates/{template_id}/project_constructions/{construction_id}",
            operation="GetProjectConstruction",
        )


class AsyncTemplatesService(AsyncBaseService):
    async def list(
        self, *, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List all templates visible to the current user.

        Args:
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="templates", operation="list", is_mutation=False),
            "/templates.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListTemplates",
        )

    async def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        """Create a new template.

        Args:
            name: The name.
            description: The description.
        """
        return await self._request(
            OperationInfo(service="templates", operation="create", is_mutation=True),
            "POST",
            "/templates.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateTemplate",
        )

    async def get(self, *, template_id: int) -> dict[str, Any]:
        """Get a single template by id.

        Args:
            template_id: The template id.
        """
        return await self._request(
            OperationInfo(service="templates", operation="get", is_mutation=False, resource_id=template_id),
            "GET",
            f"/templates/{template_id}",
            operation="GetTemplate",
        )

    async def update(
        self, *, template_id: int, name: str | None = None, description: str | None = None
    ) -> dict[str, Any]:
        """Update an existing template.

        Args:
            template_id: The template id.
            name: The name.
            description: The description.
        """
        return await self._request(
            OperationInfo(service="templates", operation="update", is_mutation=True, resource_id=template_id),
            "PUT",
            f"/templates/{template_id}",
            json_body=self._compact(name=name, description=description),
            operation="UpdateTemplate",
        )

    async def delete(self, *, template_id: int) -> None:
        """Delete a template (trash it).

        Args:
            template_id: The template id.
        """
        await self._request_void(
            OperationInfo(service="templates", operation="delete", is_mutation=True, resource_id=template_id),
            "DELETE",
            f"/templates/{template_id}",
            operation="DeleteTemplate",
        )

    async def create_project(self, *, template_id: int, project: dict) -> dict[str, Any]:
        """Create a project from a template (asynchronous).

        Args:
            template_id: The template id.
            project: The project.
        """
        return await self._request(
            OperationInfo(service="templates", operation="create_project", is_mutation=True, resource_id=template_id),
            "POST",
            f"/templates/{template_id}/project_constructions.json",
            json_body=self._compact(project=project),
            operation="CreateProjectFromTemplate",
        )

    async def get_construction(self, *, template_id: int, construction_id: int) -> dict[str, Any]:
        """Get the status of a project construction.

        Args:
            template_id: The template id.
            construction_id: The construction id.
        """
        return await self._request(
            OperationInfo(
                service="templates", operation="get_construction", is_mutation=False, resource_id=construction_id
            ),
            "GET",
            f"/templates/{template_id}/project_constructions/{construction_id}",
            operation="GetProjectConstruction",
        )
