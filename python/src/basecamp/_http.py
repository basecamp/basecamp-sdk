from __future__ import annotations

import random
import time

import httpx

from basecamp import _security
from basecamp._version import API_VERSION, VERSION
from basecamp.errors import (
    ApiError,
    AuthError,
    BasecampError,
    NetworkError,
    RateLimitError,
    UsageError,
    error_from_response,
)
from basecamp.hooks import BasecampHooks, RequestInfo, RequestResult, safe_hook


class HttpClient:
    """Sync HTTP client with retry, auth injection, and error mapping."""

    USER_AGENT = f"basecamp-sdk-python/{VERSION} (api:{API_VERSION})"

    def __init__(
        self,
        config,
        auth,
        hooks: BasecampHooks | None = None,
        *,
        user_agent: str | None = None,
        metadata: dict | None = None,
    ):
        self._config = config
        self._auth = auth
        self._hooks = hooks or BasecampHooks()
        self._metadata = metadata or {}
        self._user_agent = user_agent or self.USER_AGENT
        self._client = httpx.Client(
            timeout=httpx.Timeout(config.timeout, connect=10.0),
            follow_redirects=False,
        )

    @property
    def base_url(self) -> str:
        return self._config.base_url

    def get(self, url: str, *, params: dict | None = None) -> httpx.Response:
        url = self._build_url(url)
        return self._request_with_retry("GET", url, params=params)

    def get_absolute(self, url: str, *, params: dict | None = None) -> httpx.Response:
        if not _security.is_localhost(url):
            _security.require_https(url, "URL")
        return self._request_with_retry("GET", url, params=params)

    def post(self, url: str, *, json_body: dict | None = None, operation: str | None = None) -> httpx.Response:
        url = self._build_url(url)
        return self._mutation("POST", url, json_body=json_body, operation=operation)

    def put(self, url: str, *, json_body: dict | None = None, operation: str | None = None) -> httpx.Response:
        url = self._build_url(url)
        return self._mutation("PUT", url, json_body=json_body, operation=operation)

    def delete(self, url: str, *, operation: str | None = None) -> httpx.Response:
        url = self._build_url(url)
        return self._mutation("DELETE", url, operation=operation)

    def post_raw(
        self,
        url: str,
        *,
        content: bytes,
        content_type: str,
        params: dict | None = None,
        operation: str | None = None,
    ) -> httpx.Response:
        url = self._build_url(url)
        if operation and self._is_retryable_operation(operation):
            return self._request_with_retry(
                "POST",
                url,
                params=params,
                content=content,
                content_type=content_type,
            )
        return self._single_request(
            "POST",
            url,
            params=params,
            content=content,
            content_type=content_type,
        )

    def get_no_retry(self, url: str) -> httpx.Response:
        url = self._build_url(url)
        return self._single_request("GET", url)

    def close(self) -> None:
        self._client.close()

    # -- internal --

    def _mutation(
        self, method: str, url: str, *, json_body: dict | None = None, operation: str | None = None
    ) -> httpx.Response:
        if operation and self._is_retryable_operation(operation):
            return self._request_with_retry(method, url, json_body=json_body)
        return self._single_request(method, url, json_body=json_body)

    def _request_with_retry(
        self,
        method: str,
        url: str,
        *,
        params: dict | None = None,
        json_body: dict | None = None,
        content: bytes | None = None,
        content_type: str | None = None,
    ) -> httpx.Response:
        attempt = 0
        last_error: BasecampError | None = None

        while True:
            attempt += 1
            if attempt > self._config.max_retries + 1:
                break

            try:
                return self._single_request(
                    method,
                    url,
                    params=params,
                    json_body=json_body,
                    content=content,
                    content_type=content_type,
                    attempt=attempt,
                )
            except (RateLimitError, NetworkError, ApiError) as e:
                if not e.retryable:
                    raise
                last_error = e
                if attempt > self._config.max_retries:
                    break
                delay = self._calculate_delay(attempt, e.retry_after)
                safe_hook(
                    self._hooks.on_retry,
                    RequestInfo(method=method, url=url, attempt=attempt),
                    attempt + 1,
                    e,
                    delay,
                )
                time.sleep(delay)

        if last_error:
            raise last_error
        raise ApiError(f"Request failed after {self._config.max_retries} retries")

    def _single_request(
        self,
        method: str,
        url: str,
        *,
        params: dict | None = None,
        json_body: dict | None = None,
        content: bytes | None = None,
        content_type: str | None = None,
        attempt: int = 1,
        _retry_count: int = 0,
    ) -> httpx.Response:
        info = RequestInfo(method=method, url=url, attempt=attempt)
        safe_hook(self._hooks.on_request_start, info)
        start = time.monotonic()

        try:
            headers = self._request_headers()
            if content_type:
                headers["Content-Type"] = content_type
            self._auth.authenticate(headers)

            response = self._client.request(
                method,
                url,
                headers=headers,
                params=params,
                json=json_body,
                content=content,
            )

            if response.status_code >= 400:
                error = self._handle_error(response)
                # 401 retry with token refresh
                if isinstance(error, AuthError) and error.http_status == 401 and _retry_count < 1:
                    tp = getattr(self._auth, "token_provider", None)
                    if tp and getattr(tp, "refreshable", False) and tp.refresh():
                        return self._single_request(
                            method,
                            url,
                            params=params,
                            json_body=json_body,
                            content=content,
                            content_type=content_type,
                            attempt=attempt,
                            _retry_count=_retry_count + 1,
                        )
                raise error

            duration = time.monotonic() - start
            safe_hook(
                self._hooks.on_request_end, info, RequestResult(status_code=response.status_code, duration=duration)
            )
            return response

        except BasecampError as e:
            duration = time.monotonic() - start
            safe_hook(self._hooks.on_request_end, info, RequestResult(duration=duration, error=e))
            raise
        except httpx.HTTPError as e:
            duration = time.monotonic() - start
            error = NetworkError(f"Connection failed: {e}")
            safe_hook(self._hooks.on_request_end, info, RequestResult(duration=duration, error=error))
            raise error from e

    def _handle_error(self, response: httpx.Response) -> BasecampError:
        body = response.content[: _security.MAX_ERROR_BODY_BYTES] if response.content else None
        return error_from_response(
            response.status_code,
            body,
            dict(response.headers),
        )

    def _request_headers(self) -> dict[str, str]:
        return {
            "User-Agent": self._user_agent,
            "Accept": "application/json",
        }

    def _build_url(self, path: str) -> str:
        if path.startswith("https://"):
            return path
        if path.startswith("http://"):
            if not _security.is_localhost(path):
                raise UsageError(f"URL must use HTTPS: {path}")
            return path
        if not path.startswith("/"):
            path = f"/{path}"
        return f"{self._config.base_url}{path}"

    def _calculate_delay(self, attempt: int, server_retry_after: int | None = None) -> float:
        if server_retry_after and server_retry_after > 0:
            return float(server_retry_after)
        base = self._config.base_delay * (2 ** (attempt - 1))
        jitter = random.random() * self._config.max_jitter
        return base + jitter

    def _is_retryable_operation(self, operation: str) -> bool:
        op_meta = self._metadata.get(operation, {})
        return op_meta.get("idempotent", False)
