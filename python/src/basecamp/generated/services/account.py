# @generated from OpenAPI spec — do not edit manually

from __future__ import annotations

from typing import Any

from basecamp.generated.services._base import BaseService
from basecamp.generated.services._async_base import AsyncBaseService
from basecamp._pagination import ListResult
from basecamp.hooks import OperationInfo


class AccountService(BaseService):
    def get_account(self) -> dict[str, Any]:
        """Get the account for the current access token."""
        return self._request(
            OperationInfo(service="account", operation="get_account", is_mutation=False),
            "GET",
            "/account.json",
            operation="GetAccount",
        )

    def update_account_logo(self, *, content: bytes, filename: str, content_type: str) -> None:
        """Upload or replace the account logo.
        Accepted formats: PNG, JPEG, GIF, WebP, AVIF, HEIC. Maximum 5 MB.
        Owners and admins only.

        Args:
            content: Raw bytes of the file to upload.
            filename: Filename for the uploaded file.
            content_type: MIME content type of the upload (e.g. "image/png").
        """
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
        """Remove the account logo. Only administrators and account owners can use this endpoint."""
        self._request_void(
            OperationInfo(service="account", operation="remove_account_logo", is_mutation=True),
            "DELETE",
            "/account/logo.json",
            operation="RemoveAccountLogo",
        )

    def update_account_name(self, *, name: str) -> dict[str, Any]:
        """Rename the current account. Only account owners can use this endpoint.

        Args:
            name: The name.
        """
        return self._request(
            OperationInfo(service="account", operation="update_account_name", is_mutation=True),
            "PUT",
            "/account/name.json",
            json_body=self._compact(name=name),
            operation="UpdateAccountName",
        )


class AsyncAccountService(AsyncBaseService):
    async def get_account(self) -> dict[str, Any]:
        """Get the account for the current access token."""
        return await self._request(
            OperationInfo(service="account", operation="get_account", is_mutation=False),
            "GET",
            "/account.json",
            operation="GetAccount",
        )

    async def update_account_logo(self, *, content: bytes, filename: str, content_type: str) -> None:
        """Upload or replace the account logo.
        Accepted formats: PNG, JPEG, GIF, WebP, AVIF, HEIC. Maximum 5 MB.
        Owners and admins only.

        Args:
            content: Raw bytes of the file to upload.
            filename: Filename for the uploaded file.
            content_type: MIME content type of the upload (e.g. "image/png").
        """
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
        """Remove the account logo. Only administrators and account owners can use this endpoint."""
        await self._request_void(
            OperationInfo(service="account", operation="remove_account_logo", is_mutation=True),
            "DELETE",
            "/account/logo.json",
            operation="RemoveAccountLogo",
        )

    async def update_account_name(self, *, name: str) -> dict[str, Any]:
        """Rename the current account. Only account owners can use this endpoint.

        Args:
            name: The name.
        """
        return await self._request(
            OperationInfo(service="account", operation="update_account_name", is_mutation=True),
            "PUT",
            "/account/name.json",
            json_body=self._compact(name=name),
            operation="UpdateAccountName",
        )
