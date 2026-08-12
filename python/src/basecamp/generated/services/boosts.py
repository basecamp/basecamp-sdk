# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class BoostsService(BaseService):
    def get_boost(self, *, boost_id: int) -> dict[str, Any]:
        """Get a single boost.

        Args:
            boost_id: The boost id.
        """
        return self._request(
            OperationInfo(service="boosts", operation="get_boost", is_mutation=False, resource_id=boost_id),
            "GET",
            f"/boosts/{boost_id}",
            operation="GetBoost",
        )

    def delete_boost(self, *, boost_id: int) -> None:
        """Delete a boost.

        Args:
            boost_id: The boost id.
        """
        self._request_void(
            OperationInfo(service="boosts", operation="delete_boost", is_mutation=True, resource_id=boost_id),
            "DELETE",
            f"/boosts/{boost_id}",
            operation="DeleteBoost",
        )

    def list_recording_boosts(
        self, *, recording_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List boosts on a recording.

        Args:
            recording_id: The recording id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(
                service="boosts", operation="list_recording_boosts", is_mutation=False, resource_id=recording_id
            ),
            f"/recordings/{recording_id}/boosts.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListRecordingBoosts",
        )

    def create_recording_boost(self, *, recording_id: int, content: str) -> dict[str, Any]:
        """Create a boost on a recording.

        Args:
            recording_id: The recording id.
            content: The content.
        """
        return self._request(
            OperationInfo(
                service="boosts", operation="create_recording_boost", is_mutation=True, resource_id=recording_id
            ),
            "POST",
            f"/recordings/{recording_id}/boosts.json",
            json_body=self._compact(content=content),
            operation="CreateRecordingBoost",
        )

    def list_event_boosts(
        self, *, recording_id: int, event_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List boosts on a specific event within a recording.

        Args:
            recording_id: The recording id.
            event_id: The event id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return self._request_paginated(
            OperationInfo(service="boosts", operation="list_event_boosts", is_mutation=False, resource_id=event_id),
            f"/recordings/{recording_id}/events/{event_id}/boosts.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListEventBoosts",
        )

    def create_event_boost(self, *, recording_id: int, event_id: int, content: str) -> dict[str, Any]:
        """Create a boost on a specific event within a recording.

        Args:
            recording_id: The recording id.
            event_id: The event id.
            content: The content.
        """
        return self._request(
            OperationInfo(service="boosts", operation="create_event_boost", is_mutation=True, resource_id=event_id),
            "POST",
            f"/recordings/{recording_id}/events/{event_id}/boosts.json",
            json_body=self._compact(content=content),
            operation="CreateEventBoost",
        )


class AsyncBoostsService(AsyncBaseService):
    async def get_boost(self, *, boost_id: int) -> dict[str, Any]:
        """Get a single boost.

        Args:
            boost_id: The boost id.
        """
        return await self._request(
            OperationInfo(service="boosts", operation="get_boost", is_mutation=False, resource_id=boost_id),
            "GET",
            f"/boosts/{boost_id}",
            operation="GetBoost",
        )

    async def delete_boost(self, *, boost_id: int) -> None:
        """Delete a boost.

        Args:
            boost_id: The boost id.
        """
        await self._request_void(
            OperationInfo(service="boosts", operation="delete_boost", is_mutation=True, resource_id=boost_id),
            "DELETE",
            f"/boosts/{boost_id}",
            operation="DeleteBoost",
        )

    async def list_recording_boosts(
        self, *, recording_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List boosts on a recording.

        Args:
            recording_id: The recording id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(
                service="boosts", operation="list_recording_boosts", is_mutation=False, resource_id=recording_id
            ),
            f"/recordings/{recording_id}/boosts.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListRecordingBoosts",
        )

    async def create_recording_boost(self, *, recording_id: int, content: str) -> dict[str, Any]:
        """Create a boost on a recording.

        Args:
            recording_id: The recording id.
            content: The content.
        """
        return await self._request(
            OperationInfo(
                service="boosts", operation="create_recording_boost", is_mutation=True, resource_id=recording_id
            ),
            "POST",
            f"/recordings/{recording_id}/boosts.json",
            json_body=self._compact(content=content),
            operation="CreateRecordingBoost",
        )

    async def list_event_boosts(
        self, *, recording_id: int, event_id: int, page: int | None = None, max_items: int | None = None
    ) -> ListResult:
        """List boosts on a specific event within a recording.

        Args:
            recording_id: The recording id.
            event_id: The event id.
            page: Page number for paginating through results. Defaults to 1. A positive value
                selects exactly that page, not a starting offset; see SPEC section 8.
            max_items: Client-side cap on the total number of items collected across pages; None
                collects every page.
        """
        return await self._request_paginated(
            OperationInfo(service="boosts", operation="list_event_boosts", is_mutation=False, resource_id=event_id),
            f"/recordings/{recording_id}/events/{event_id}/boosts.json",
            params=self._compact(page=page),
            max_items=max_items,
            operation="ListEventBoosts",
        )

    async def create_event_boost(self, *, recording_id: int, event_id: int, content: str) -> dict[str, Any]:
        """Create a boost on a specific event within a recording.

        Args:
            recording_id: The recording id.
            event_id: The event id.
            content: The content.
        """
        return await self._request(
            OperationInfo(service="boosts", operation="create_event_boost", is_mutation=True, resource_id=event_id),
            "POST",
            f"/recordings/{recording_id}/events/{event_id}/boosts.json",
            json_body=self._compact(content=content),
            operation="CreateEventBoost",
        )
