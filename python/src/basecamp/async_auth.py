from __future__ import annotations

import asyncio
import inspect
from typing import Any, Protocol, runtime_checkable


@runtime_checkable
class AsyncTokenProvider(Protocol):
    async def access_token(self) -> str: ...
    async def refresh(self) -> bool: ...
    @property
    def refreshable(self) -> bool: ...


@runtime_checkable
class AsyncAuthStrategy(Protocol):
    async def authenticate(self, headers: dict[str, str]) -> None: ...


class AsyncStaticTokenProvider:
    """Async token provider with a fixed access token."""

    def __init__(self, token: str):
        self._token = token

    async def access_token(self) -> str:
        return self._token

    async def refresh(self) -> bool:
        return False

    @property
    def refreshable(self) -> bool:
        return False


class AsyncOAuthTokenProvider:
    """Async token provider that supports OAuth token refresh.

    Uses an asyncio lock for concurrency safety.
    """

    TOKEN_URL = "https://launchpad.37signals.com/authorization/token"

    def __init__(
        self,
        access_token: str,
        client_id: str,
        client_secret: str,
        *,
        refresh_token: str | None = None,
        expires_at: float | None = None,
        on_refresh: Any = None,
    ):
        self._access_token = access_token
        self._refresh_token = refresh_token
        self._client_id = client_id
        self._client_secret = client_secret
        self._expires_at = expires_at
        self._on_refresh = on_refresh
        self._lock = asyncio.Lock()

    async def access_token(self) -> str:
        async with self._lock:
            if self._expired and self.refreshable:
                await self._perform_refresh()
            return self._access_token

    async def refresh(self) -> bool:
        async with self._lock:
            if not self.refreshable:
                return False
            return await self._perform_refresh()

    @property
    def refreshable(self) -> bool:
        return bool(self._refresh_token)

    @property
    def _expired(self) -> bool:
        if self._expires_at is None:
            return False
        import time

        return time.time() >= self._expires_at

    async def _perform_refresh(self) -> bool:
        import httpx

        from basecamp.errors import AuthError, NetworkError

        try:
            async with httpx.AsyncClient(timeout=30.0) as client:
                response = await client.post(
                    self.TOKEN_URL,
                    data={
                        "type": "refresh",
                        "refresh_token": self._refresh_token,
                        "client_id": self._client_id,
                        "client_secret": self._client_secret,
                    },
                    headers={"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json"},
                )
        except httpx.HTTPError as e:
            raise NetworkError(f"Token refresh network error: {e}") from e

        if response.status_code >= 400:
            raise AuthError(f"Token refresh failed: {response.status_code}")

        try:
            data = response.json()
        except (ValueError, KeyError) as e:
            raise AuthError(f"Token refresh returned invalid response: {e}") from e
        if not isinstance(data, dict) or "access_token" not in data:
            raise AuthError("Token refresh response missing access_token")
        self._access_token = data["access_token"]
        if data.get("refresh_token"):
            self._refresh_token = data["refresh_token"]
        if data.get("expires_in"):
            import time

            self._expires_at = time.time() + int(data["expires_in"])

        if self._on_refresh:
            if inspect.iscoroutinefunction(self._on_refresh):
                await self._on_refresh(self._access_token, self._refresh_token, self._expires_at)
            else:
                self._on_refresh(self._access_token, self._refresh_token, self._expires_at)

        return True


class AsyncBearerAuth:
    """Async auth strategy that adds a Bearer token header."""

    def __init__(self, token_provider: AsyncTokenProvider):
        self.token_provider = token_provider

    async def authenticate(self, headers: dict[str, str]) -> None:
        token = await self.token_provider.access_token()
        headers["Authorization"] = f"Bearer {token}"
