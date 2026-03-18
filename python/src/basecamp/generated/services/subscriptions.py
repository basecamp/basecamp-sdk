# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class SubscriptionsService(BaseService):
    def get(self, *, recording_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="subscriptions", operation="get", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}/subscription.json",
        )

    def subscribe(self, *, recording_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="subscriptions", operation="subscribe", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/subscription.json",
            operation="Subscribe",
        )

    def update(
        self, *, recording_id: int | str, subscriptions: list | None = None, unsubscriptions: list | None = None
    ) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="subscriptions", operation="update", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/subscription.json",
            json_body=self._compact(subscriptions=subscriptions, unsubscriptions=unsubscriptions),
            operation="UpdateSubscription",
        )

    def unsubscribe(self, *, recording_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="subscriptions", operation="unsubscribe", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/subscription.json",
            operation="Unsubscribe",
        )


class AsyncSubscriptionsService(AsyncBaseService):
    async def get(self, *, recording_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="subscriptions", operation="get", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}/subscription.json",
        )

    async def subscribe(self, *, recording_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="subscriptions", operation="subscribe", is_mutation=True, resource_id=recording_id),
            "POST",
            f"/recordings/{recording_id}/subscription.json",
            operation="Subscribe",
        )

    async def update(
        self, *, recording_id: int | str, subscriptions: list | None = None, unsubscriptions: list | None = None
    ) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="subscriptions", operation="update", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/subscription.json",
            json_body=self._compact(subscriptions=subscriptions, unsubscriptions=unsubscriptions),
            operation="UpdateSubscription",
        )

    async def unsubscribe(self, *, recording_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="subscriptions", operation="unsubscribe", is_mutation=True, resource_id=recording_id),
            "DELETE",
            f"/recordings/{recording_id}/subscription.json",
            operation="Unsubscribe",
        )
