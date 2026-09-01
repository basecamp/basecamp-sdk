from __future__ import annotations

import asyncio
import random
import time

import httpx

from basecamp import _security
from basecamp._version import API_VERSION, VERSION
from basecamp.config import saturating_backoff
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


class AsyncHttpClient:
    """Async HTTP client with retry, auth injection, and error mapping."""

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
        self._client = httpx.AsyncClient(
            timeout=httpx.Timeout(config.timeout, connect=10.0),
            follow_redirects=False,
        )

    @property
    def base_url(self) -> str:
        return self._config.base_url

    async def get(self, url: str, *, params: dict | None = None, operation: str | None = None) -> httpx.Response:
        url = self._build_url(url)
        return await self._request_with_retry("GET", url, params=params, operation=operation)

    async def get_absolute(self, url: str, *, params: dict | None = None) -> httpx.Response:
        if not _security.is_localhost(url):
            _security.require_https(url, "URL")
        # Cross-origin is permitted only for the trusted Launchpad authorization
        # endpoint; any other foreign origin still trips the same-origin guard so
        # the bearer token never leaks off the configured host.
        allow_cross_origin = url == _security.LAUNCHPAD_AUTHORIZATION_URL
        return await self._request_with_retry("GET", url, params=params, allow_cross_origin=allow_cross_origin)

    async def post(self, url: str, *, json_body: dict | None = None, operation: str | None = None) -> httpx.Response:
        url = self._build_url(url)
        return await self._mutation("POST", url, json_body=json_body, operation=operation)

    async def put(self, url: str, *, json_body: dict | None = None, operation: str | None = None) -> httpx.Response:
        url = self._build_url(url)
        return await self._mutation("PUT", url, json_body=json_body, operation=operation)

    async def delete(self, url: str, *, operation: str | None = None) -> httpx.Response:
        url = self._build_url(url)
        return await self._mutation("DELETE", url, operation=operation)

    async def post_raw(
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
            return await self._request_with_retry(
                "POST",
                url,
                params=params,
                content=content,
                content_type=content_type,
                operation=operation,
            )
        return await self._single_request(
            "POST",
            url,
            params=params,
            content=content,
            content_type=content_type,
        )

    async def request_multipart(
        self,
        method: str,
        url: str,
        *,
        field: str,
        content: bytes,
        filename: str,
        content_type: str,
        operation: str | None = None,
    ) -> httpx.Response:
        url = self._build_url(url)
        safe_filename = filename.replace("\r", "").replace("\n", "")
        safe_content_type = content_type.replace("\r", "").replace("\n", "")
        files = {field: (safe_filename, content, safe_content_type)}
        if operation and self._is_retryable_operation(operation):
            return await self._request_with_retry(method, url, files=files, operation=operation)
        return await self._single_request(method, url, files=files)

    async def get_download(self, url: str) -> httpx.Response:
        """Authenticated hop-1 GET for the download flow (SPEC section 14).

        Retries network errors plus the declared DOWNLOAD_RETRY_ON statuses —
        never 500 — under the public max_retries total-attempt cap, floored at
        one. DownloadURL has no behavior-model entry, so the policy is passed
        directly rather than looked up by operation.
        """
        url = self._build_url(url)
        # download=True projects this flow's hooks and transport errors (SPEC
        # section 9): the caller's URL can smuggle a signed query through the
        # rewrite into hop 1, and an httpx error retains the request it failed
        # on. The wire request keeps the query; only the renderings are
        # projected.
        return await self._request_with_retry("GET", url, retry_on=self.DOWNLOAD_RETRY_ON, accept=None, download=True)

    async def close(self) -> None:
        await self._client.aclose()

    # -- internal --

    async def _mutation(
        self, method: str, url: str, *, json_body: dict | None = None, operation: str | None = None
    ) -> httpx.Response:
        if operation and self._is_retryable_operation(operation):
            return await self._request_with_retry(method, url, json_body=json_body, operation=operation)
        return await self._single_request(method, url, json_body=json_body)

    async def _request_with_retry(
        self,
        method: str,
        url: str,
        *,
        params: dict | None = None,
        json_body: dict | None = None,
        content: bytes | None = None,
        content_type: str | None = None,
        files: dict | None = None,
        allow_cross_origin: bool = False,
        operation: str | None = None,
        retry_on: frozenset[int] | None = None,
        accept: str | None = "application/json",
        download: bool = False,
    ) -> httpx.Response:
        # max_retries is a TOTAL attempt count (config validation guarantees it
        # is >= 0). 0 is accepted as a compatibility exception and means a single
        # attempt with no retry.
        max_attempts = self._config.max_retries if self._config.max_retries > 0 else 1
        max_attempts = self._apply_operation_retry_max(operation, max_attempts)
        attempt = 0
        refreshed_once = False
        last_error: BasecampError | None = None

        while True:
            attempt += 1
            if attempt > max_attempts:
                break

            # Each except suite only CLASSIFIES the attempt; the retry side
            # effects run below, outside every handler. An exception raised
            # inside an except suite bypasses that try's sibling handlers, so
            # refreshing there would let a NetworkError from the token endpoint
            # escape the loop with budget still on the table.
            error: BasecampError | None = None
            stale_auth: AuthError | None = None
            try:
                return await self._single_request(
                    method,
                    url,
                    params=params,
                    json_body=json_body,
                    content=content,
                    content_type=content_type,
                    files=files,
                    attempt=attempt,
                    allow_cross_origin=allow_cross_origin,
                    accept=accept,
                    refresh_replay=False,
                    download=download,
                )
            except AuthError as e:
                # SPEC §4: the refresh replay is a request on the wire, so it
                # spends an attempt from THIS budget rather than an uncounted
                # one inside the single-request primitive. max_retries is a
                # total attempt count (#461), and a cap of one means one
                # request whatever would have caused the second.
                #
                # The budget is checked BEFORE refresh() so a rotation is never
                # burned on an attempt the loop has no room to make. Await the
                # refresh rather than truthy-testing the coroutine, which would
                # pass the gate without ever rotating the token.
                if refreshed_once or e.http_status != 401 or attempt >= max_attempts:
                    raise
                stale_auth = e
            except (RateLimitError, NetworkError, ApiError) as e:
                error = e

            if stale_auth is not None:
                # SPEC §4 tracks refresh with an "attempted" boolean, not a
                # "succeeded" one, so mark it BEFORE invoking the provider: a
                # refresh that throws still counts as the one attempt this
                # request gets. Otherwise a transient token-endpoint failure
                # lets the NEXT 401 in the same request call refresh() again —
                # and if the first call reached the server and rotated the
                # token before its response was lost, the second spends a
                # refresh token that is already dead.
                refreshed_once = True
                try:
                    tp = getattr(self._auth, "token_provider", None)
                    refreshed = bool(tp and getattr(tp, "refreshable", False) and await tp.refresh())
                except (RateLimitError, NetworkError, ApiError) as e:
                    # A token endpoint that times out is a transient failure of
                    # this attempt, not a terminal auth fault: it retries under
                    # the same budget, exactly as it did when the replay lived
                    # inside _single_request.
                    error = e
                else:
                    if not refreshed:
                        raise stale_auth
                    # No backoff: the token is fresh, and the server never asked
                    # us to wait. The replay still costs the attempt counted above.
                    continue

            if error is not None:
                if not self._is_retryable_error(error, operation, retry_on=retry_on):
                    raise error
                last_error = error
                if attempt >= max_attempts:
                    break
                delay = self._calculate_delay(attempt, error.retry_after)
                safe_hook(
                    self._hooks.on_retry,
                    RequestInfo(method=method, url=_security.display_url(url) if download else url, attempt=attempt),
                    attempt + 1,
                    error,
                    delay,
                )
                await asyncio.sleep(delay)

        if last_error:
            raise last_error
        noun = "attempt" if max_attempts == 1 else "attempts"
        raise ApiError(f"Request failed after {max_attempts} {noun}")

    async def _single_request(
        self,
        method: str,
        url: str,
        *,
        params: dict | None = None,
        json_body: dict | None = None,
        content: bytes | None = None,
        content_type: str | None = None,
        files: dict | None = None,
        attempt: int = 1,
        _retry_count: int = 0,
        allow_cross_origin: bool = False,
        accept: str | None = "application/json",
        refresh_replay: bool = True,
        download: bool = False,
    ) -> httpx.Response:
        if not allow_cross_origin and not (
            _security.is_localhost(url) or _security.same_origin(url, self._config.base_url)
        ):
            raise UsageError(
                f"Refusing to send credentials to a different origin than base URL: {_security.truncate(url)}"
            )
        # download: the SPEC section 9 projection for a URL whose query can
        # carry a credential (download hop 1) — hooks see origin+path, and a
        # transport error is severed below; the wire request keeps url.
        info = RequestInfo(method=method, url=_security.display_url(url) if download else url, attempt=attempt)
        safe_hook(self._hooks.on_request_start, info)
        start = time.monotonic()

        severed: NetworkError
        try:
            headers = self._request_headers(accept)
            if content_type:
                headers["Content-Type"] = content_type
            await self._auth.authenticate(headers)

            response = await self._client.request(
                method,
                url,
                headers=headers,
                params=params,
                json=json_body,
                content=content,
                files=files,
            )

            if response.status_code >= 400:
                error = self._handle_error(response)
                # 401 replay for callers that come here directly (mutations),
                # which have no retry loop to own it. _request_with_retry
                # passes refresh_replay=False and replays from the loop so the
                # extra request draws from the attempt budget (SPEC §4).
                if refresh_replay and isinstance(error, AuthError) and error.http_status == 401 and _retry_count < 1:
                    tp = getattr(self._auth, "token_provider", None)
                    if tp and getattr(tp, "refreshable", False) and await tp.refresh():
                        return await self._single_request(
                            method,
                            url,
                            params=params,
                            json_body=json_body,
                            content=content,
                            content_type=content_type,
                            files=files,
                            attempt=attempt,
                            _retry_count=_retry_count + 1,
                            allow_cross_origin=allow_cross_origin,
                            accept=accept,
                            refresh_replay=refresh_replay,
                            download=download,
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
            # SPEC section 9: same raising boundary as the sync client — on a
            # download hop 1 the httpx error is fixed, unchained, and raised
            # below, outside this handler.
            error = NetworkError("Network error") if download else NetworkError(f"Connection failed: {e}")
            safe_hook(self._hooks.on_request_end, info, RequestResult(duration=duration, error=error))
            if not download:
                raise error from e
            severed = error

        raise severed

    def _handle_error(self, response: httpx.Response) -> BasecampError:
        body = response.content[: _security.MAX_ERROR_BODY_BYTES] if response.content else None
        return error_from_response(
            response.status_code,
            body,
            dict(response.headers),
        )

    def _request_headers(self, accept: str | None = "application/json") -> dict[str, str]:
        # accept=None is the binary-download carve-out (SPEC section 14): hop 1
        # sends Authorization and User-Agent only, because it is not a JSON API
        # call. Every other caller keeps the JSON Accept.
        headers = {"User-Agent": self._user_agent}
        if accept is not None:
            headers["Accept"] = accept
        return headers

    def _build_url(self, path: str) -> str:
        # Schemes are case-insensitive (RFC 3986): detect absolute URLs on a
        # lowercased copy so HTTPS://... is not mis-joined onto the base URL.
        lower_path = path.lower()
        if lower_path.startswith("https://"):
            if _security.is_localhost(path) or _security.same_origin(path, self._config.base_url):
                return path
            raise UsageError(f"URL origin does not match configured base URL: {_security.truncate(path)}")
        if lower_path.startswith("http://"):
            if not _security.is_localhost(path):
                raise UsageError(f"URL must use HTTPS: {_security.truncate(path)}")
            return path
        if not path.startswith("/"):
            path = f"/{path}"
        return f"{self._config.base_url}{path}"

    def _calculate_delay(self, attempt: int, server_retry_after: int | None = None) -> float:
        # Retry-After is server-directed and exempt from the ceiling (SPEC
        # section 7); only the locally-computed term saturates.
        if server_retry_after and server_retry_after > 0:
            return float(server_retry_after)
        base = saturating_backoff(self._config.base_delay, attempt)
        # Backoff jitter spreads retries; it is a fairness device, not a security primitive.
        jitter = random.random() * self._config.max_jitter  # noqa: S311
        return base + jitter

    def _is_retryable_operation(self, operation: str) -> bool:
        op_meta = self._metadata.get(operation, {})
        return op_meta.get("idempotent", False)

    # behavior-model.json's declared default, used when an operation carries no
    # retry_on of its own.
    DEFAULT_RETRY_ON = frozenset({429, 503})

    # SPEC section 14's declared hop-1 set for downloads: a carve-out from the
    # ungoverned GET taxonomy (which retries 500). Authoritative in BOTH
    # directions, like an operation's declared retry_on.
    DOWNLOAD_RETRY_ON = frozenset({429, 502, 503, 504})

    def _is_retryable_error(
        self, error: BasecampError, operation: str | None, *, retry_on: frozenset[int] | None = None
    ) -> bool:
        # A network error carries no HTTP status, so the declared status set does
        # not apply; SPEC section 7's network-error rule governs, and errors.py's
        # classification is the right signal there.
        if error.http_status is None:
            return error.retryable
        # An explicit declared set (the download flow) is authoritative in both
        # directions, exactly like an operation's declared retry_on below.
        if retry_on is not None:
            return error.http_status in retry_on
        # No operation id means no behavior-model metadata — today only the
        # Launchpad authorization GET issued by get_absolute(). Non-Smithy
        # traffic keeps its pre-Smithy retry contract; applying the generated
        # gate here would impose API policy on OAuth authorization traffic.
        if operation is None:
            return error.retryable
        # For a governed operation the DECLARED set is authoritative in BOTH
        # directions. It must not be widened by errors.py (which marks
        # 500/502/503/504 retryable for the caller's benefit), and it must not be
        # vetoed by it either: if an operation declares a status retryable, a
        # future errors.py classifying that status non-retryable must not
        # silently disable the declared retry.
        return error.http_status in self._operation_retry_on(operation)

    def _operation_retry_on(self, operation: str) -> frozenset[int]:
        retry = (self._metadata.get(operation) or {}).get("retry") or {}
        statuses = retry.get("retry_on")
        # `is None`, not truthiness: an operation that declares `retry_on: []`
        # means "never retry on any status", which is not the same as declaring
        # nothing. Collapsing the two would silently re-enable 429/503 retries on
        # an operation that opted out.
        if statuses is None:
            return self.DEFAULT_RETRY_ON
        return frozenset(statuses)

    def _apply_operation_retry_max(self, operation: str | None, max_attempts: int) -> int:
        # Apply the per-operation retry ceiling from metadata as an upper bound on
        # attempts: effective = min(client cap, operation's retry.max). The
        # ceiling can only reduce attempts below the client-configured cap, never
        # raise them, so a client that lowered its cap is still honored. An
        # operation with no metadata (or no retry.max) leaves the cap unchanged.
        if not operation:
            return max_attempts
        op_max = self._metadata.get(operation, {}).get("retry", {}).get("max")
        if isinstance(op_max, int) and 0 < op_max < max_attempts:
            return op_max
        return max_attempts
