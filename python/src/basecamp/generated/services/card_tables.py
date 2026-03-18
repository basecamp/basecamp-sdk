# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class CardTablesService(BaseService):
    def get(self, *, card_table_id: int | str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="cardtables", operation="get", is_mutation=False, resource_id=card_table_id),
            "GET",
            f"/card_tables/{card_table_id}",
        )


class AsyncCardTablesService(AsyncBaseService):
    async def get(self, *, card_table_id: int | str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="cardtables", operation="get", is_mutation=False, resource_id=card_table_id),
            "GET",
            f"/card_tables/{card_table_id}",
        )
