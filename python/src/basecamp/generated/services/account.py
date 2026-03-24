# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class AccountService(BaseService):
    def get_account(self) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="account", operation="get_account", is_mutation=False), "GET", "/account.json"
        )

    def update_account_logo(self, *, content: bytes, filename: str, content_type: str) -> None:
        self._request_multipart_void(
            OperationInfo(service="account", operation="update_account_logo", is_mutation=True),
            "PUT",
            "/account/logo.json",
            field="logo",
            content=content,
            filename=filename,
            content_type=content_type,
            operation="UpdateAccountLogo",
        )

    def remove_account_logo(self) -> None:
        self._request_void(
            OperationInfo(service="account", operation="remove_account_logo", is_mutation=True),
            "DELETE",
            "/account/logo.json",
            operation="RemoveAccountLogo",
        )

    def update_account_name(self, *, name: str) -> dict[str, Any]:
        return self._request(
            OperationInfo(service="account", operation="update_account_name", is_mutation=True),
            "PUT",
            "/account/name.json",
            json_body=self._compact(name=name),
            operation="UpdateAccountName",
        )


class AsyncAccountService(AsyncBaseService):
    async def get_account(self) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="account", operation="get_account", is_mutation=False), "GET", "/account.json"
        )

    async def update_account_logo(self, *, content: bytes, filename: str, content_type: str) -> None:
        await self._request_multipart_void(
            OperationInfo(service="account", operation="update_account_logo", is_mutation=True),
            "PUT",
            "/account/logo.json",
            field="logo",
            content=content,
            filename=filename,
            content_type=content_type,
            operation="UpdateAccountLogo",
        )

    async def remove_account_logo(self) -> None:
        await self._request_void(
            OperationInfo(service="account", operation="remove_account_logo", is_mutation=True),
            "DELETE",
            "/account/logo.json",
            operation="RemoveAccountLogo",
        )

    async def update_account_name(self, *, name: str) -> dict[str, Any]:
        return await self._request(
            OperationInfo(service="account", operation="update_account_name", is_mutation=True),
            "PUT",
            "/account/name.json",
            json_body=self._compact(name=name),
            operation="UpdateAccountName",
        )
