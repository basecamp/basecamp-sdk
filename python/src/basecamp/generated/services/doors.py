# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class DoorsService(BaseService):
    def list(self, *, bucket_id: int) -> ListResult:
        """List the active external links in a project
        They come in the order they appear in the project's External links section.
        Not paginated: a project has a handful at most. Clients see only the
        external links visible to them. Listing needs only access to the project,
        not permission to edit it.

        Args:
            bucket_id: The bucket id.
        """
        return self._request_list(
            OperationInfo(service="doors", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/dock/doors.json",
            operation="ListDoors",
        )

    def create(self, *, bucket_id: int, door: dict, visible_to_clients: bool | None = None) -> dict[str, Any]:
        """Create an external link in a project
        It goes at the end of the project's External links section. The link's
        fields go under the `door` key and `visible_to_clients` sits beside it.
        Omitting `visible_to_clients` keeps the link private to the team, whatever
        the visibility of the project's other tools; a client caller always creates
        a client-visible link. An unusable `url` (e.g. a `javascript:` address) is a
        field-keyed 422. Needs permission to edit the project. The `door[image]`
        thumbnail is multipart-only and not modelled.

        Args:
            bucket_id: The bucket id.
            door: The door.
            visible_to_clients: Whether the project's clients can see the external link. Omitted
                means false: the link is private to the team. A client caller always creates a
                client-visible link regardless of what is sent.
        """
        return self._request(
            OperationInfo(service="doors", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/dock/doors.json",
            json_body=self._compact(door=door, visible_to_clients=visible_to_clients),
            operation="CreateDoor",
        )

    def get(self, *, door_id: int) -> dict[str, Any]:
        """Get an external link by id
        A door hidden from a client, an unknown id, or the id of a recording that is
        not a door is a 404.

        Args:
            door_id: The door id.
        """
        return self._request(
            OperationInfo(service="doors", operation="get", is_mutation=False, resource_id=door_id),
            "GET",
            f"/dock/doors/{door_id}",
            operation="GetDoor",
        )

    def update(
        self, *, door_id: int, door: dict | None = None, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Update an external link
        A partial update: every field is optional, and those left out keep their
        values. Omit `door` entirely to change only `visible_to_clients`; omit
        `visible_to_clients` to keep the current visibility. An unusable `url` is a
        field-keyed 422. Needs permission to edit the project.

        Args:
            door_id: The door id.
            door: The door.
            visible_to_clients: Whether the project's clients can see the external link. Omitted
                keeps the current visibility.
        """
        return self._request(
            OperationInfo(service="doors", operation="update", is_mutation=True, resource_id=door_id),
            "PUT",
            f"/dock/doors/{door_id}",
            json_body=self._compact(door=door, visible_to_clients=visible_to_clients),
            operation="UpdateDoor",
        )

    def delete(self, *, door_id: int) -> None:
        """Delete an external link (returns 204 No Content)
        Its status becomes "deleted" and it leaves the project's External links section. It does not
        go to the trash: Basecamp permanently destroys it about five days later.
        Needs permission to edit the project.

        Args:
            door_id: The door id.
        """
        self._request_void(
            OperationInfo(service="doors", operation="delete", is_mutation=True, resource_id=door_id),
            "DELETE",
            f"/dock/doors/{door_id}",
            operation="DeleteDoor",
        )


class AsyncDoorsService(AsyncBaseService):
    async def list(self, *, bucket_id: int) -> ListResult:
        """List the active external links in a project
        They come in the order they appear in the project's External links section.
        Not paginated: a project has a handful at most. Clients see only the
        external links visible to them. Listing needs only access to the project,
        not permission to edit it.

        Args:
            bucket_id: The bucket id.
        """
        return await self._request_list(
            OperationInfo(service="doors", operation="list", is_mutation=False, project_id=bucket_id),
            f"/buckets/{bucket_id}/dock/doors.json",
            operation="ListDoors",
        )

    async def create(self, *, bucket_id: int, door: dict, visible_to_clients: bool | None = None) -> dict[str, Any]:
        """Create an external link in a project
        It goes at the end of the project's External links section. The link's
        fields go under the `door` key and `visible_to_clients` sits beside it.
        Omitting `visible_to_clients` keeps the link private to the team, whatever
        the visibility of the project's other tools; a client caller always creates
        a client-visible link. An unusable `url` (e.g. a `javascript:` address) is a
        field-keyed 422. Needs permission to edit the project. The `door[image]`
        thumbnail is multipart-only and not modelled.

        Args:
            bucket_id: The bucket id.
            door: The door.
            visible_to_clients: Whether the project's clients can see the external link. Omitted
                means false: the link is private to the team. A client caller always creates a
                client-visible link regardless of what is sent.
        """
        return await self._request(
            OperationInfo(service="doors", operation="create", is_mutation=True, project_id=bucket_id),
            "POST",
            f"/buckets/{bucket_id}/dock/doors.json",
            json_body=self._compact(door=door, visible_to_clients=visible_to_clients),
            operation="CreateDoor",
        )

    async def get(self, *, door_id: int) -> dict[str, Any]:
        """Get an external link by id
        A door hidden from a client, an unknown id, or the id of a recording that is
        not a door is a 404.

        Args:
            door_id: The door id.
        """
        return await self._request(
            OperationInfo(service="doors", operation="get", is_mutation=False, resource_id=door_id),
            "GET",
            f"/dock/doors/{door_id}",
            operation="GetDoor",
        )

    async def update(
        self, *, door_id: int, door: dict | None = None, visible_to_clients: bool | None = None
    ) -> dict[str, Any]:
        """Update an external link
        A partial update: every field is optional, and those left out keep their
        values. Omit `door` entirely to change only `visible_to_clients`; omit
        `visible_to_clients` to keep the current visibility. An unusable `url` is a
        field-keyed 422. Needs permission to edit the project.

        Args:
            door_id: The door id.
            door: The door.
            visible_to_clients: Whether the project's clients can see the external link. Omitted
                keeps the current visibility.
        """
        return await self._request(
            OperationInfo(service="doors", operation="update", is_mutation=True, resource_id=door_id),
            "PUT",
            f"/dock/doors/{door_id}",
            json_body=self._compact(door=door, visible_to_clients=visible_to_clients),
            operation="UpdateDoor",
        )

    async def delete(self, *, door_id: int) -> None:
        """Delete an external link (returns 204 No Content)
        Its status becomes "deleted" and it leaves the project's External links section. It does not
        go to the trash: Basecamp permanently destroys it about five days later.
        Needs permission to edit the project.

        Args:
            door_id: The door id.
        """
        await self._request_void(
            OperationInfo(service="doors", operation="delete", is_mutation=True, resource_id=door_id),
            "DELETE",
            f"/dock/doors/{door_id}",
            operation="DeleteDoor",
        )
