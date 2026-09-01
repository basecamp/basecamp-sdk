from __future__ import annotations

import threading
from typing import Any, Protocol, runtime_checkable


@runtime_checkable
class TokenProvider(Protocol):
    def access_token(self) -> str: ...
    def refresh(self) -> bool: ...
    @property
    def refreshable(self) -> bool: ...


@runtime_checkable
class AuthStrategy(Protocol):
    def authenticate(self, headers: dict[str, str]) -> None: ...


class StaticTokenProvider:
    """Token provider with a fixed access token."""

    def __init__(self, token: str):
        self._token = token

    def access_token(self) -> str:
        return self._token

    def refresh(self) -> bool:
        return False

    @property
    def refreshable(self) -> bool:
        return False


class OAuthTokenProvider:
    """Token provider that supports OAuth token refresh.

    Thread-safe: uses a lock around refresh operations.

    Launchpad-only: the token URL is fixed to Launchpad's legacy endpoint and
    no RFC 8707 ``resource`` indicator is sent. BC5 multi-account refresh
    tokens require the resource echo — use the discovery +
    :func:`basecamp.oauth.exchange.refresh_token` path (passing
    ``token.resource``) instead of this provider.
    """

    # Public OAuth token endpoint URL, not a credential.
    TOKEN_URL = "https://launchpad.37signals.com/authorization/token"  # noqa: S105

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
        self._lock = threading.Lock()

    def access_token(self) -> str:
        with self._lock:
            if self._expired and self.refreshable:
                self._perform_refresh()
            return self._access_token

    def refresh(self) -> bool:
        with self._lock:
            if not self.refreshable:
                return False
            return self._perform_refresh()

    @property
    def refreshable(self) -> bool:
        return bool(self._refresh_token)

    @property
    def _expired(self) -> bool:
        if self._expires_at is None:
            return False
        import time

        return time.time() >= self._expires_at

    def _perform_refresh(self) -> bool:
        import httpx

        from basecamp.errors import AuthError, NetworkError

        # SPEC §9: the httpx error retains the request it failed on — the form
        # body carrying refresh_token and client_secret — so a transport
        # failure is constructed here and raised outside the handler.
        transport_error: NetworkError | None = None
        try:
            response = httpx.post(
                self.TOKEN_URL,
                data={
                    "type": "refresh",
                    "refresh_token": self._refresh_token,
                    "client_id": self._client_id,
                    "client_secret": self._client_secret,
                },
                headers={"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json"},
                timeout=30.0,
            )
        except httpx.HTTPError as e:
            transport_error = NetworkError(f"Token refresh network error: {e}")
        if transport_error is not None:
            raise transport_error

        if response.status_code >= 400:
            raise AuthError(f"Token refresh failed: {response.status_code}")

        parse_error: AuthError | None = None
        try:
            data = response.json()
        except (ValueError, KeyError):
            # SPEC §9: JSONDecodeError retains the whole token-response body in
            # .doc, so the exception must survive in neither __cause__ nor
            # __context__ — construct here, raise after the handler exits.
            data = None
            parse_error = AuthError("Token refresh returned invalid response")
        if parse_error is not None:
            raise parse_error
        if not isinstance(data, dict) or "access_token" not in data:
            raise AuthError("Token refresh response missing access_token")
        self._access_token = data["access_token"]
        if data.get("refresh_token"):
            self._refresh_token = data["refresh_token"]
        if data.get("expires_in"):
            import time

            self._expires_at = time.time() + int(data["expires_in"])

        if self._on_refresh:
            self._on_refresh(self._access_token, self._refresh_token, self._expires_at)

        return True


class BearerAuth:
    """Auth strategy that adds a Bearer token header."""

    def __init__(self, token_provider: TokenProvider):
        self.token_provider = token_provider

    def authenticate(self, headers: dict[str, str]) -> None:
        token = self.token_provider.access_token()
        headers["Authorization"] = f"Bearer {token}"
