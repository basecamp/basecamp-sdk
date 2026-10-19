# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class PingsService(BaseService):
    def list(self) -> ListResult:
        """Get Pings: the caller's active Pings that have a line in them, most
        recently updated first, in one unpaginated response. Archived Pings, and
        Pings no one has posted in yet, are not listed; GetPing reads either by id.

        Answers `403 Forbidden` (`{"error": "Pings have been disabled"}`) when the
        caller cannot ping.
        """
        return self._request_list(
            OperationInfo(service="pings", operation="list", is_mutation=False), "/circles.json", operation="ListPings"
        )

    def create(self, *, user_ids: list[int], new_group: bool | None = None) -> dict[str, Any]:
        """Create a Ping with people, or return the one that already holds exactly
        them.

        The caller is always included. Without `new_group`, an existing active
        Ping with exactly these people (the oldest, if several) is returned rather
        than a second one created; with `new_group: true` and more than one other
        person, a new group is created every time. Either way the answer is
        `201 Created` with the Ping.

        Answers `403 Forbidden` (`{"error": "Pings have been disabled"}`) when the
        caller cannot ping, and a bodiless `422` when nobody in the Ping would be a
        person who can ping.

        Args:
            user_ids: Person ids of the people to ping. People the caller cannot reach are dropped.
            new_group: Create a new group even when a Ping with exactly these people exists.
        """
        return self._request(
            OperationInfo(service="pings", operation="create", is_mutation=True),
            "POST",
            "/circles.json",
            json_body=self._compact(user_ids=user_ids, new_group=new_group),
            operation="CreatePing",
        )

    def get(self, *, ping_id: int) -> dict[str, Any]:
        """Get a Ping by its id, which is its bucket's id.

        Answers `403 Forbidden` (`{"error": "Pings have been disabled"}`) when the
        caller cannot ping, and `404 Not Found` for a bucket that is not a Ping the
        caller is in.

        Args:
            ping_id: A Ping's id is the id of the bucket that holds it (the bucket type is "Circle",
                which is also the wire path), so the `bucket.id` a Ping's recordings carry is the id
                GetPing takes.
        """
        return self._request(
            OperationInfo(service="pings", operation="get", is_mutation=False, resource_id=ping_id),
            "GET",
            f"/circles/{ping_id}",
            operation="GetPing",
        )


class AsyncPingsService(AsyncBaseService):
    async def list(self) -> ListResult:
        """Get Pings: the caller's active Pings that have a line in them, most
        recently updated first, in one unpaginated response. Archived Pings, and
        Pings no one has posted in yet, are not listed; GetPing reads either by id.

        Answers `403 Forbidden` (`{"error": "Pings have been disabled"}`) when the
        caller cannot ping.
        """
        return await self._request_list(
            OperationInfo(service="pings", operation="list", is_mutation=False), "/circles.json", operation="ListPings"
        )

    async def create(self, *, user_ids: list[int], new_group: bool | None = None) -> dict[str, Any]:
        """Create a Ping with people, or return the one that already holds exactly
        them.

        The caller is always included. Without `new_group`, an existing active
        Ping with exactly these people (the oldest, if several) is returned rather
        than a second one created; with `new_group: true` and more than one other
        person, a new group is created every time. Either way the answer is
        `201 Created` with the Ping.

        Answers `403 Forbidden` (`{"error": "Pings have been disabled"}`) when the
        caller cannot ping, and a bodiless `422` when nobody in the Ping would be a
        person who can ping.

        Args:
            user_ids: Person ids of the people to ping. People the caller cannot reach are dropped.
            new_group: Create a new group even when a Ping with exactly these people exists.
        """
        return await self._request(
            OperationInfo(service="pings", operation="create", is_mutation=True),
            "POST",
            "/circles.json",
            json_body=self._compact(user_ids=user_ids, new_group=new_group),
            operation="CreatePing",
        )

    async def get(self, *, ping_id: int) -> dict[str, Any]:
        """Get a Ping by its id, which is its bucket's id.

        Answers `403 Forbidden` (`{"error": "Pings have been disabled"}`) when the
        caller cannot ping, and `404 Not Found` for a bucket that is not a Ping the
        caller is in.

        Args:
            ping_id: A Ping's id is the id of the bucket that holds it (the bucket type is "Circle",
                which is also the wire path), so the `bucket.id` a Ping's recordings carry is the id
                GetPing takes.
        """
        return await self._request(
            OperationInfo(service="pings", operation="get", is_mutation=False, resource_id=ping_id),
            "GET",
            f"/circles/{ping_id}",
            operation="GetPing",
        )
