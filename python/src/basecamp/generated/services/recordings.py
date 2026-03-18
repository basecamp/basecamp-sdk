# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class RecordingsService(BaseService):
    def list(
        self,
        *,
        type: str,
        bucket: str | None = None,
        status: str | None = None,
        sort: str | None = None,
        direction: str | None = None,
    ) -> ListResult:
        return self._request_paginated(
            OperationInfo(service="recordings", operation="list", is_mutation=False),
            "/projects/recordings.json",
            params=self._compact(type=type, bucket=bucket, status=status, sort=sort, direction=direction),
        )

    def get(self, *, recording_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="recordings", operation="get", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}",
        )

    def unarchive(self, *, recording_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="recordings", operation="unarchive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/active.json",
            operation="UnarchiveRecording",
        )

    def archive(self, *, recording_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="recordings", operation="archive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/archived.json",
            operation="ArchiveRecording",
        )

    def trash(self, *, recording_id: int | str) -> None:
        self._request_void(
            OperationInfo(service="recordings", operation="trash", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/trashed.json",
            operation="TrashRecording",
        )


class AsyncRecordingsService(AsyncBaseService):
    async def list(
        self,
        *,
        type: str,
        bucket: str | None = None,
        status: str | None = None,
        sort: str | None = None,
        direction: str | None = None,
    ) -> ListResult:
        return await self._request_paginated(
            OperationInfo(service="recordings", operation="list", is_mutation=False),
            "/projects/recordings.json",
            params=self._compact(type=type, bucket=bucket, status=status, sort=sort, direction=direction),
        )

    async def get(self, *, recording_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="recordings", operation="get", is_mutation=False, resource_id=recording_id),
            "GET",
            f"/recordings/{recording_id}",
        )

    async def unarchive(self, *, recording_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="recordings", operation="unarchive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/active.json",
            operation="UnarchiveRecording",
        )

    async def archive(self, *, recording_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="recordings", operation="archive", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/archived.json",
            operation="ArchiveRecording",
        )

    async def trash(self, *, recording_id: int | str) -> None:
        await self._request_void(
            OperationInfo(service="recordings", operation="trash", is_mutation=True, resource_id=recording_id),
            "PUT",
            f"/recordings/{recording_id}/status/trashed.json",
            operation="TrashRecording",
        )
