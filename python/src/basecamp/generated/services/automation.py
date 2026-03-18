# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class AutomationService(BaseService):
    def list_lineup_markers(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="automation", operation="list_lineup_markers", is_mutation=False),
            "GET",
            "/lineup/markers.json",
        )


class AsyncAutomationService(AsyncBaseService):
    async def list_lineup_markers(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="automation", operation="list_lineup_markers", is_mutation=False),
            "GET",
            "/lineup/markers.json",
        )
