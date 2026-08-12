# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class ProjectsService(BaseService):
    def list(self, *, status: str | None = None, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List projects (active by default; optionally archived/trashed).

        Args:
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="projects", operation="list", is_mutation=False),
            "/projects.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListProjects",
        )

    def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        """Create a new project.

        Args:
            name: The name.
            description: The description.
        """
        return self._request(
            OperationInfo(service="projects", operation="create", is_mutation=True),
            "POST",
            "/projects.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateProject",
        )

    def get(self, *, project_id: int) -> dict[str, Any]:
        """Get a single project by id.

        Args:
            project_id: The project id.
        """
        return self._request(
            OperationInfo(service="projects", operation="get", is_mutation=False, project_id=project_id),
            "GET",
            f"/projects/{project_id}",
            operation="GetProject",
        )

    def update(
        self,
        *,
        project_id: int,
        name: str,
        description: str | None = None,
        admissions: str | None = None,
        schedule_attributes: dict | None = None,
    ) -> dict[str, Any]:
        """Update an existing project.

        Args:
            project_id: The project id.
            name: The name.
            description: The description.
            admissions: invite|employee|team
            schedule_attributes: The schedule attributes.
        """
        return self._request(
            OperationInfo(service="projects", operation="update", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}",
            json_body=self._compact(
                name=name, description=description, admissions=admissions, schedule_attributes=schedule_attributes
            ),
            operation="UpdateProject",
        )

    def trash(self, *, project_id: int) -> None:
        """Trash a project (returns 204 No Content).

        Args:
            project_id: The project id.
        """
        self._request_void(
            OperationInfo(service="projects", operation="trash", is_mutation=True, project_id=project_id),
            "DELETE",
            f"/projects/{project_id}",
            operation="TrashProject",
        )

    def unarchive(self, *, project_id: int) -> None:
        """Restore a project to active status from trash as well as from the archive (returns 204 No Content).
        This is the inverse of both ArchiveProject and TrashProject. Restoring counts against
        the account's project limit, so it answers 507 when that limit is already reached.

        Args:
            project_id: The project id.
        """
        self._request_void(
            OperationInfo(service="projects", operation="unarchive", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/status/active.json",
            operation="UnarchiveProject",
        )

    def archive(self, *, project_id: int) -> None:
        """Archive a project, removing it from the active project list (returns 204 No Content).
        Accounts on the admin pro pack may restrict archiving to admins and the project's
        creator, which answers 403.

        Args:
            project_id: The project id.
        """
        self._request_void(
            OperationInfo(service="projects", operation="archive", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/status/archived.json",
            operation="ArchiveProject",
        )


class AsyncProjectsService(AsyncBaseService):
    async def list(
        self, *, status: str | None = None, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List projects (active by default; optionally archived/trashed).

        Args:
            status: active|archived|trashed
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="projects", operation="list", is_mutation=False),
            "/projects.json",
            params=self._compact(status=status, page=page),
            max_items=max_items,
            operation="ListProjects",
        )

    async def create(self, *, name: str, description: str | None = None) -> dict[str, Any]:
        """Create a new project.

        Args:
            name: The name.
            description: The description.
        """
        return await self._request(
            OperationInfo(service="projects", operation="create", is_mutation=True),
            "POST",
            "/projects.json",
            json_body=self._compact(name=name, description=description),
            operation="CreateProject",
        )

    async def get(self, *, project_id: int) -> dict[str, Any]:
        """Get a single project by id.

        Args:
            project_id: The project id.
        """
        return await self._request(
            OperationInfo(service="projects", operation="get", is_mutation=False, project_id=project_id),
            "GET",
            f"/projects/{project_id}",
            operation="GetProject",
        )

    async def update(
        self,
        *,
        project_id: int,
        name: str,
        description: str | None = None,
        admissions: str | None = None,
        schedule_attributes: dict | None = None,
    ) -> dict[str, Any]:
        """Update an existing project.

        Args:
            project_id: The project id.
            name: The name.
            description: The description.
            admissions: invite|employee|team
            schedule_attributes: The schedule attributes.
        """
        return await self._request(
            OperationInfo(service="projects", operation="update", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}",
            json_body=self._compact(
                name=name, description=description, admissions=admissions, schedule_attributes=schedule_attributes
            ),
            operation="UpdateProject",
        )

    async def trash(self, *, project_id: int) -> None:
        """Trash a project (returns 204 No Content).

        Args:
            project_id: The project id.
        """
        await self._request_void(
            OperationInfo(service="projects", operation="trash", is_mutation=True, project_id=project_id),
            "DELETE",
            f"/projects/{project_id}",
            operation="TrashProject",
        )

    async def unarchive(self, *, project_id: int) -> None:
        """Restore a project to active status from trash as well as from the archive (returns 204 No Content).
        This is the inverse of both ArchiveProject and TrashProject. Restoring counts against
        the account's project limit, so it answers 507 when that limit is already reached.

        Args:
            project_id: The project id.
        """
        await self._request_void(
            OperationInfo(service="projects", operation="unarchive", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/status/active.json",
            operation="UnarchiveProject",
        )

    async def archive(self, *, project_id: int) -> None:
        """Archive a project, removing it from the active project list (returns 204 No Content).
        Accounts on the admin pro pack may restrict archiving to admins and the project's
        creator, which answers 403.

        Args:
            project_id: The project id.
        """
        await self._request_void(
            OperationInfo(service="projects", operation="archive", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/status/archived.json",
            operation="ArchiveProject",
        )
