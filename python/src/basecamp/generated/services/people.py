# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class PeopleService(BaseService):
    def list_pingable(self, *, max_items: int | None = None) -> ListResult:
        """List all account users who can be pinged.

        Args:
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages.
        """
        return self._request_paginated(
            OperationInfo(service="people", operation="list_pingable", is_mutation=False),
            "/circles/people.json",
            max_items=max_items,
            operation="ListPingablePeople",
        )

    def get_my_preferences(self) -> dict[str, Any]:
        """Get the current user's preferences."""
        return self._request(
            OperationInfo(service="people", operation="get_my_preferences", is_mutation=False),
            "GET",
            "/my/preferences.json",
            operation="GetMyPreferences",
        )

    def update_my_preferences(self, *, person: dict) -> dict[str, Any]:
        """Update the current user's preferences.
        Rejections arrive as a field-keyed 422
        ({"errors": {"time_zone_name": ["is not included in the list"]}}), not the
        flat {error} body.

        Args:
            person: The person.
        """
        return self._request(
            OperationInfo(service="people", operation="update_my_preferences", is_mutation=True),
            "PUT",
            "/my/preferences.json",
            json_body=self._compact(person=person),
            operation="UpdateMyPreferences",
        )

    def my_profile(self) -> dict[str, Any]:
        """Get the current authenticated user's profile."""
        return self._request(
            OperationInfo(service="people", operation="my_profile", is_mutation=False),
            "GET",
            "/my/profile.json",
            operation="GetMyProfile",
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
        """Update the current authenticated user's profile (returns 204 No Content).

        Args:
            name: The name.
            email_address: The email address.
            title: The title.
            bio: The bio.
            location: The location.
            time_zone_name: The time zone name.
            first_week_day: The first week day.
            time_format: The time format.
        """
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

    def list(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all people visible to the current user.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="people", operation="list", is_mutation=False),
            "/people.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListPeople",
        )

    def get(self, *, person_id: int) -> dict[str, Any]:
        """Get a person by ID.

        Args:
            person_id: The person id.
        """
        return self._request(
            OperationInfo(service="people", operation="get", is_mutation=False, resource_id=person_id),
            "GET",
            f"/people/{person_id}",
            operation="GetPerson",
        )

    def get_out_of_office(self, *, person_id: int) -> dict[str, Any]:
        """Get the out of office status for a person.

        Args:
            person_id: The person id.
        """
        return self._request(
            OperationInfo(service="people", operation="get_out_of_office", is_mutation=False, resource_id=person_id),
            "GET",
            f"/people/{person_id}/out_of_office.json",
            operation="GetOutOfOffice",
        )

    def enable_out_of_office(self, *, person_id: int, out_of_office: dict) -> dict[str, Any]:
        """Enable or replace out of office for a person.
        Admins on Pro Pack accounts can manage others; otherwise self only.

        Args:
            person_id: The person id.
            out_of_office: The out of office.
        """
        return self._request(
            OperationInfo(service="people", operation="enable_out_of_office", is_mutation=True, resource_id=person_id),
            "POST",
            f"/people/{person_id}/out_of_office.json",
            json_body=self._compact(out_of_office=out_of_office),
            operation="EnableOutOfOffice",
        )

    def disable_out_of_office(self, *, person_id: int) -> None:
        """Disable out of office for a person.
        Admins on Pro Pack accounts can manage others; otherwise self only.

        Args:
            person_id: The person id.
        """
        self._request_void(
            OperationInfo(service="people", operation="disable_out_of_office", is_mutation=True, resource_id=person_id),
            "DELETE",
            f"/people/{person_id}/out_of_office.json",
            operation="DisableOutOfOffice",
        )

    def enable_project_clients(self, *, project_id: int) -> dict[str, Any]:
        """Enable clients on a project so client users can be added to it.

        A deliberate step separate from adding clients: it turns on the project's
        client-facing surface and applies the default client visibility (the
        timeline and most docked tools become client-visible; the card table,
        Campfire, and Doors stay private). UpdateProjectClientAccess never enables
        clients implicitly — enable first, then add. 403 unless the project can have
        clients (the account supports clients and the project is a standard
        project). Naturally idempotent: enabling an enabled project re-answers
        `{"clients_enabled": true}`.

        Args:
            project_id: The project id.
        """
        return self._request(
            OperationInfo(
                service="people", operation="enable_project_clients", is_mutation=True, project_id=project_id
            ),
            "POST",
            f"/projects/{project_id}/client_enablement.json",
            operation="EnableProjectClients",
        )

    def disable_project_clients(self, *, project_id: int) -> dict[str, Any]:
        """Disable clients on a project.

        403 while the project still has any client users — revoke them first with
        UpdateProjectClientAccess. Naturally idempotent: disabling a project with
        clients already off re-answers `{"clients_enabled": false}`.

        Args:
            project_id: The project id.
        """
        return self._request(
            OperationInfo(
                service="people", operation="disable_project_clients", is_mutation=True, project_id=project_id
            ),
            "DELETE",
            f"/projects/{project_id}/client_enablement.json",
            operation="DisableProjectClients",
        )

    def list_for_project(self, *, project_id: int, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all active people on a project.

        Args:
            project_id: The project id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return self._request_paginated(
            OperationInfo(service="people", operation="list_for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/people.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListProjectPeople",
        )

    def update_project_client_access(
        self,
        *,
        project_id: int,
        grant: list[int] | None = None,
        revoke: list[int] | None = None,
        create: list[dict] | None = None,
    ) -> dict[str, Any]:
        """Update project client access (grant/revoke/create client users).

        The client-side counterpart to UpdateProjectAccess: `grant` adds existing
        client users by id, `revoke` removes client users, and `create` invites
        brand-new clients by email (`name` optional, defaulting to the address).
        Only client users are eligible — a `grant` id belonging to a team member is
        rejected (omitted from `granted`) rather than cross-graded, and `revoke` never removes a team
        member. The response mirrors UpdateProjectAccess: `granted` and `revoked`
        people, each with `client: true`.

        Requires clients to be enabled on the project (EnableProjectClients);
        otherwise 403. Invitations are all-or-nothing: an invalid `create` row
        (including one with no email address) answers 422 with the rejected
        addresses and nobody is invited; new addresses that would exceed the
        account's user limit answer 429 and nobody is invited. Addresses already on
        the account take no seat and a repeated address counts once.

        The seat-limit 429 is a verdict, not throttling: it carries no Retry-After
        and re-asking cannot change the answer, so this operation declares
        `retryOn: [503]` and a 429 surfaces on the first attempt (status-mapped to
        `rate_limit`, since the wire status is the only signal). Every other
        operation retries on 429.

        Args:
            project_id: The project id.
            grant: Existing client people IDs to add to the project.
            revoke: Client people IDs to remove from the project.
            create: New clients to invite by email.
        """
        return self._request(
            OperationInfo(
                service="people", operation="update_project_client_access", is_mutation=True, project_id=project_id
            ),
            "PUT",
            f"/projects/{project_id}/people/client_users.json",
            json_body=self._compact(grant=grant, revoke=revoke, create=create),
            operation="UpdateProjectClientAccess",
        )

    def update_project_access(
        self,
        *,
        project_id: int,
        grant: list[int] | None = None,
        revoke: list[int] | None = None,
        create: list[dict] | None = None,
    ) -> dict[str, Any]:
        """Update project access (grant/revoke/create people).

        Args:
            project_id: The project id.
            grant: The grant.
            revoke: The revoke.
            create: The create.
        """
        return self._request(
            OperationInfo(service="people", operation="update_project_access", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/people/users.json",
            json_body=self._compact(grant=grant, revoke=revoke, create=create),
            operation="UpdateProjectAccess",
        )

    def list_assignable(self) -> ListResult:
        """List people who can be assigned todos."""
        return self._request_list(
            OperationInfo(service="people", operation="list_assignable", is_mutation=False),
            "/reports/todos/assigned.json",
            operation="ListAssignablePeople",
        )


class AsyncPeopleService(AsyncBaseService):
    async def list_pingable(self, *, max_items: int | None = None) -> ListResult:
        """List all account users who can be pinged.

        Args:
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages.
        """
        return await self._request_paginated(
            OperationInfo(service="people", operation="list_pingable", is_mutation=False),
            "/circles/people.json",
            max_items=max_items,
            operation="ListPingablePeople",
        )

    async def get_my_preferences(self) -> dict[str, Any]:
        """Get the current user's preferences."""
        return await self._request(
            OperationInfo(service="people", operation="get_my_preferences", is_mutation=False),
            "GET",
            "/my/preferences.json",
            operation="GetMyPreferences",
        )

    async def update_my_preferences(self, *, person: dict) -> dict[str, Any]:
        """Update the current user's preferences.
        Rejections arrive as a field-keyed 422
        ({"errors": {"time_zone_name": ["is not included in the list"]}}), not the
        flat {error} body.

        Args:
            person: The person.
        """
        return await self._request(
            OperationInfo(service="people", operation="update_my_preferences", is_mutation=True),
            "PUT",
            "/my/preferences.json",
            json_body=self._compact(person=person),
            operation="UpdateMyPreferences",
        )

    async def my_profile(self) -> dict[str, Any]:
        """Get the current authenticated user's profile."""
        return await self._request(
            OperationInfo(service="people", operation="my_profile", is_mutation=False),
            "GET",
            "/my/profile.json",
            operation="GetMyProfile",
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
        """Update the current authenticated user's profile (returns 204 No Content).

        Args:
            name: The name.
            email_address: The email address.
            title: The title.
            bio: The bio.
            location: The location.
            time_zone_name: The time zone name.
            first_week_day: The first week day.
            time_format: The time format.
        """
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

    async def list(self, *, page: int | None = None, max_items: int | None = None) -> ListResult:
        """List all people visible to the current user.

        Args:
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="people", operation="list", is_mutation=False),
            "/people.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListPeople",
        )

    async def get(self, *, person_id: int) -> dict[str, Any]:
        """Get a person by ID.

        Args:
            person_id: The person id.
        """
        return await self._request(
            OperationInfo(service="people", operation="get", is_mutation=False, resource_id=person_id),
            "GET",
            f"/people/{person_id}",
            operation="GetPerson",
        )

    async def get_out_of_office(self, *, person_id: int) -> dict[str, Any]:
        """Get the out of office status for a person.

        Args:
            person_id: The person id.
        """
        return await self._request(
            OperationInfo(service="people", operation="get_out_of_office", is_mutation=False, resource_id=person_id),
            "GET",
            f"/people/{person_id}/out_of_office.json",
            operation="GetOutOfOffice",
        )

    async def enable_out_of_office(self, *, person_id: int, out_of_office: dict) -> dict[str, Any]:
        """Enable or replace out of office for a person.
        Admins on Pro Pack accounts can manage others; otherwise self only.

        Args:
            person_id: The person id.
            out_of_office: The out of office.
        """
        return await self._request(
            OperationInfo(service="people", operation="enable_out_of_office", is_mutation=True, resource_id=person_id),
            "POST",
            f"/people/{person_id}/out_of_office.json",
            json_body=self._compact(out_of_office=out_of_office),
            operation="EnableOutOfOffice",
        )

    async def disable_out_of_office(self, *, person_id: int) -> None:
        """Disable out of office for a person.
        Admins on Pro Pack accounts can manage others; otherwise self only.

        Args:
            person_id: The person id.
        """
        await self._request_void(
            OperationInfo(service="people", operation="disable_out_of_office", is_mutation=True, resource_id=person_id),
            "DELETE",
            f"/people/{person_id}/out_of_office.json",
            operation="DisableOutOfOffice",
        )

    async def enable_project_clients(self, *, project_id: int) -> dict[str, Any]:
        """Enable clients on a project so client users can be added to it.

        A deliberate step separate from adding clients: it turns on the project's
        client-facing surface and applies the default client visibility (the
        timeline and most docked tools become client-visible; the card table,
        Campfire, and Doors stay private). UpdateProjectClientAccess never enables
        clients implicitly — enable first, then add. 403 unless the project can have
        clients (the account supports clients and the project is a standard
        project). Naturally idempotent: enabling an enabled project re-answers
        `{"clients_enabled": true}`.

        Args:
            project_id: The project id.
        """
        return await self._request(
            OperationInfo(
                service="people", operation="enable_project_clients", is_mutation=True, project_id=project_id
            ),
            "POST",
            f"/projects/{project_id}/client_enablement.json",
            operation="EnableProjectClients",
        )

    async def disable_project_clients(self, *, project_id: int) -> dict[str, Any]:
        """Disable clients on a project.

        403 while the project still has any client users — revoke them first with
        UpdateProjectClientAccess. Naturally idempotent: disabling a project with
        clients already off re-answers `{"clients_enabled": false}`.

        Args:
            project_id: The project id.
        """
        return await self._request(
            OperationInfo(
                service="people", operation="disable_project_clients", is_mutation=True, project_id=project_id
            ),
            "DELETE",
            f"/projects/{project_id}/client_enablement.json",
            operation="DisableProjectClients",
        )

    async def list_for_project(
        self, *, project_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List all active people on a project.

        Args:
            project_id: The project id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the number of items collected across pages; None or a
                non-positive value means no item cap. Collection is always bounded by
                config.max_pages. A positive page argument fetches exactly that one page.
        """
        return await self._request_paginated(
            OperationInfo(service="people", operation="list_for_project", is_mutation=False, project_id=project_id),
            f"/projects/{project_id}/people.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListProjectPeople",
        )

    async def update_project_client_access(
        self,
        *,
        project_id: int,
        grant: list[int] | None = None,
        revoke: list[int] | None = None,
        create: list[dict] | None = None,
    ) -> dict[str, Any]:
        """Update project client access (grant/revoke/create client users).

        The client-side counterpart to UpdateProjectAccess: `grant` adds existing
        client users by id, `revoke` removes client users, and `create` invites
        brand-new clients by email (`name` optional, defaulting to the address).
        Only client users are eligible — a `grant` id belonging to a team member is
        rejected (omitted from `granted`) rather than cross-graded, and `revoke` never removes a team
        member. The response mirrors UpdateProjectAccess: `granted` and `revoked`
        people, each with `client: true`.

        Requires clients to be enabled on the project (EnableProjectClients);
        otherwise 403. Invitations are all-or-nothing: an invalid `create` row
        (including one with no email address) answers 422 with the rejected
        addresses and nobody is invited; new addresses that would exceed the
        account's user limit answer 429 and nobody is invited. Addresses already on
        the account take no seat and a repeated address counts once.

        The seat-limit 429 is a verdict, not throttling: it carries no Retry-After
        and re-asking cannot change the answer, so this operation declares
        `retryOn: [503]` and a 429 surfaces on the first attempt (status-mapped to
        `rate_limit`, since the wire status is the only signal). Every other
        operation retries on 429.

        Args:
            project_id: The project id.
            grant: Existing client people IDs to add to the project.
            revoke: Client people IDs to remove from the project.
            create: New clients to invite by email.
        """
        return await self._request(
            OperationInfo(
                service="people", operation="update_project_client_access", is_mutation=True, project_id=project_id
            ),
            "PUT",
            f"/projects/{project_id}/people/client_users.json",
            json_body=self._compact(grant=grant, revoke=revoke, create=create),
            operation="UpdateProjectClientAccess",
        )

    async def update_project_access(
        self,
        *,
        project_id: int,
        grant: list[int] | None = None,
        revoke: list[int] | None = None,
        create: list[dict] | None = None,
    ) -> dict[str, Any]:
        """Update project access (grant/revoke/create people).

        Args:
            project_id: The project id.
            grant: The grant.
            revoke: The revoke.
            create: The create.
        """
        return await self._request(
            OperationInfo(service="people", operation="update_project_access", is_mutation=True, project_id=project_id),
            "PUT",
            f"/projects/{project_id}/people/users.json",
            json_body=self._compact(grant=grant, revoke=revoke, create=create),
            operation="UpdateProjectAccess",
        )

    async def list_assignable(self) -> ListResult:
        """List people who can be assigned todos."""
        return await self._request_list(
            OperationInfo(service="people", operation="list_assignable", is_mutation=False),
            "/reports/todos/assigned.json",
            operation="ListAssignablePeople",
        )
