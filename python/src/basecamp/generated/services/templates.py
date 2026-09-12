# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class TemplatesService(BaseService):
    def get_library_card_tables(self) -> dict[str, Any]:
        """Get the account's card table templates."""
        return self._request(
            OperationInfo(service="templates", operation="get_library_card_tables", is_mutation=False),
            "GET",
            "/template_library/card_tables.json",
            operation="GetTemplateLibraryCardTables",
        )

    def create_library_card_table(self, *, name: str) -> dict[str, Any]:
        """Create a card table template with the default columns.

        Args:
            name: The template's name. Write-only: the response carries it as `title`.
        """
        return self._request(
            OperationInfo(service="templates", operation="create_library_card_table", is_mutation=True),
            "POST",
            "/template_library/card_tables.json",
            json_body=self._compact(name=name),
            operation="CreateTemplateLibraryCardTable",
        )

    def create_library_copy(
        self,
        *,
        template_recording_id: int,
        destination_project_id: int | None = None,
        destination_parent_id: int | None = None,
        adding_people_confirmed: bool | None = None,
    ) -> dict[str, Any]:
        """Start copying a to-do list or card table template into a project.

        Args:
            template_recording_id: The to-do list or card table in the library to copy.
            destination_project_id: The destination project. Basecamp resolves the container from
                the template's kind, so a caller naming a project needs to know nothing about docks
                or to-do sets. Supply this or destination_parent_id; if both are sent,
                destination_parent_id wins.
            destination_parent_id: The container to copy into, for a caller that already holds one:
                the project's to-do set for a to-do list template, or its dock for a card table. A
                caller who may not edit it gets 404, not 403.
            adding_people_confirmed: Confirm granting destination-project access to people
                referenced by the template.
        """
        return self._request(
            OperationInfo(service="templates", operation="create_library_copy", is_mutation=True),
            "POST",
            "/template_library/copies.json",
            json_body=self._compact(
                template_recording_id=template_recording_id,
                destination_project_id=destination_project_id,
                destination_parent_id=destination_parent_id,
                adding_people_confirmed=adding_people_confirmed,
            ),
            operation="CreateTemplateLibraryCopy",
        )

    def get_library_copy(self, *, copy_id: int) -> dict[str, Any]:
        """Get the current status of a to-do list template copy.

        Args:
            copy_id: The copy id.
        """
        return self._request(
            OperationInfo(service="templates", operation="get_library_copy", is_mutation=False, resource_id=copy_id),
            "GET",
            f"/template_library/copies/{copy_id}",
            operation="GetTemplateLibraryCopy",
        )

    def get_library_todolists(self) -> dict[str, Any]:
        """Get the account's to-do list templates."""
        return self._request(
            OperationInfo(service="templates", operation="get_library_todolists", is_mutation=False),
            "GET",
            "/template_library/todolists.json",
            operation="GetTemplateLibraryTodolists",
        )

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
    async def get_library_card_tables(self) -> dict[str, Any]:
        """Get the account's card table templates."""
        return await self._request(
            OperationInfo(service="templates", operation="get_library_card_tables", is_mutation=False),
            "GET",
            "/template_library/card_tables.json",
            operation="GetTemplateLibraryCardTables",
        )

    async def create_library_card_table(self, *, name: str) -> dict[str, Any]:
        """Create a card table template with the default columns.

        Args:
            name: The template's name. Write-only: the response carries it as `title`.
        """
        return await self._request(
            OperationInfo(service="templates", operation="create_library_card_table", is_mutation=True),
            "POST",
            "/template_library/card_tables.json",
            json_body=self._compact(name=name),
            operation="CreateTemplateLibraryCardTable",
        )

    async def create_library_copy(
        self,
        *,
        template_recording_id: int,
        destination_project_id: int | None = None,
        destination_parent_id: int | None = None,
        adding_people_confirmed: bool | None = None,
    ) -> dict[str, Any]:
        """Start copying a to-do list or card table template into a project.

        Args:
            template_recording_id: The to-do list or card table in the library to copy.
            destination_project_id: The destination project. Basecamp resolves the container from
                the template's kind, so a caller naming a project needs to know nothing about docks
                or to-do sets. Supply this or destination_parent_id; if both are sent,
                destination_parent_id wins.
            destination_parent_id: The container to copy into, for a caller that already holds one:
                the project's to-do set for a to-do list template, or its dock for a card table. A
                caller who may not edit it gets 404, not 403.
            adding_people_confirmed: Confirm granting destination-project access to people
                referenced by the template.
        """
        return await self._request(
            OperationInfo(service="templates", operation="create_library_copy", is_mutation=True),
            "POST",
            "/template_library/copies.json",
            json_body=self._compact(
                template_recording_id=template_recording_id,
                destination_project_id=destination_project_id,
                destination_parent_id=destination_parent_id,
                adding_people_confirmed=adding_people_confirmed,
            ),
            operation="CreateTemplateLibraryCopy",
        )

    async def get_library_copy(self, *, copy_id: int) -> dict[str, Any]:
        """Get the current status of a to-do list template copy.

        Args:
            copy_id: The copy id.
        """
        return await self._request(
            OperationInfo(service="templates", operation="get_library_copy", is_mutation=False, resource_id=copy_id),
            "GET",
            f"/template_library/copies/{copy_id}",
            operation="GetTemplateLibraryCopy",
        )

    async def get_library_todolists(self) -> dict[str, Any]:
        """Get the account's to-do list templates."""
        return await self._request(
            OperationInfo(service="templates", operation="get_library_todolists", is_mutation=False),
            "GET",
            "/template_library/todolists.json",
            operation="GetTemplateLibraryTodolists",
        )

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
