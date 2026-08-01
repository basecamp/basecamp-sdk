from __future__ import annotations

import asyncio

import httpx
import pytest
import respx

from basecamp._async_http import AsyncHttpClient
from basecamp._http import HttpClient
from basecamp.async_auth import AsyncBearerAuth, AsyncStaticTokenProvider
from basecamp.auth import BearerAuth, StaticTokenProvider
from basecamp.config import Config
from basecamp.download import _rewrite_url, download_async, download_sync, filename_from_url
from basecamp.errors import ApiError, UsageError
from basecamp.hooks import BasecampHooks


def make_config():
    return Config(base_url="https://3.basecampapi.com")


def make_http():
    config = make_config()
    auth = BearerAuth(StaticTokenProvider("test-token"))
    return HttpClient(config, auth)


class TestRewriteUrl:
    def test_replaces_host_preserves_path(self):
        result = _rewrite_url(
            "https://other.host.com/123/things/456.pdf?sig=abc",
            "https://3.basecampapi.com",
        )
        assert result.startswith("https://3.basecampapi.com/")
        assert "/123/things/456.pdf" in result
        assert "sig=abc" in result

    def test_preserves_path_and_query(self):
        result = _rewrite_url(
            "https://original.com/a/b?x=1&y=2",
            "https://new.com",
        )
        assert result == "https://new.com/a/b?x=1&y=2"


class TestFilenameFromUrl:
    def test_extracts_filename(self):
        assert filename_from_url("https://example.com/files/report.pdf") == "report.pdf"

    def test_no_path_returns_download(self):
        assert filename_from_url("https://example.com") == "download"

    def test_url_decodes_filename(self):
        assert filename_from_url("https://example.com/files/my%20report.pdf") == "my report.pdf"

    def test_root_path_returns_download(self):
        assert filename_from_url("https://example.com/") == "download"


class TestRedirectHandling:
    @respx.mock
    def test_302_follows_to_signed_url(self):
        # Hop 1: authenticated request to rewritten URL returns redirect
        respx.get("https://3.basecampapi.com/files/doc.pdf").mock(
            return_value=httpx.Response(
                302,
                headers={"Location": "https://signed.storage.com/doc.pdf?sig=xyz"},
            )
        )
        # Hop 2: unauthenticated fetch of the signed URL
        respx.get("https://signed.storage.com/doc.pdf?sig=xyz").mock(
            return_value=httpx.Response(
                200,
                content=b"file-content",
                headers={"content-type": "application/pdf", "content-length": "12"},
            )
        )

        config = make_config()
        http = make_http()
        result = download_sync("https://original.com/files/doc.pdf", http_client=http, config=config)
        assert result.body == b"file-content"
        assert result.content_type == "application/pdf"
        assert result.filename == "doc.pdf"


class TestDirectDownload:
    @respx.mock
    def test_200_direct_response(self):
        respx.get("https://3.basecampapi.com/files/image.png").mock(
            return_value=httpx.Response(
                200,
                content=b"png-data",
                headers={"content-type": "image/png", "content-length": "8"},
            )
        )

        config = make_config()
        http = make_http()
        result = download_sync("https://original.com/files/image.png", http_client=http, config=config)
        assert result.body == b"png-data"
        assert result.content_type == "image/png"
        assert result.content_length == 8


class TestInvalidUrl:
    def test_empty_url_raises(self):
        with pytest.raises(UsageError, match="URL is required"):
            download_sync("", http_client=make_http(), config=make_config())

    def test_relative_url_raises(self):
        with pytest.raises(UsageError, match="absolute URL"):
            download_sync("/just/a/path", http_client=make_http(), config=make_config())


def make_fast_config(**overrides):
    """Millisecond backoff so the retry tables run without real sleeps."""
    defaults = dict(base_url="https://3.basecampapi.com", base_delay=0.001, max_jitter=0.0)
    defaults.update(overrides)
    return Config(**defaults)


def make_fast_http(config, hooks=None):
    auth = BearerAuth(StaticTokenProvider("test-token"))
    return HttpClient(config, auth, hooks)


class RecordingHooks(BasecampHooks):
    def __init__(self):
        self.starts: list[int] = []
        self.ends: list[int] = []
        self.retries: list[int] = []

    def on_request_start(self, info):
        self.starts.append(info.attempt)

    def on_request_end(self, info, result):
        self.ends.append(info.attempt)

    def on_retry(self, info, attempt, error, delay):
        self.retries.append(attempt)


class TestHop1Retry:
    """SPEC §14 hop-1 retry policy: network errors plus {429, 502, 503, 504},
    never 500, under the public max_retries cap floored at one."""

    HOP1 = "https://3.basecampapi.com/files/doc.pdf"
    SIGNED = "https://signed.storage.com/doc.pdf"
    RAW = "https://original.com/files/doc.pdf"

    # The COMPLETE declared retry set, pinned status by status (the shared
    # conformance fixtures cover 429/503).
    @respx.mock
    @pytest.mark.parametrize("status", [429, 502, 503, 504])
    def test_retries_declared_status_then_succeeds(self, status):
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(status),
                httpx.Response(302, headers={"Location": self.SIGNED}),
            ]
        )
        respx.get(self.SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "application/pdf"})
        )

        config = make_fast_config()
        result = download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        assert hop1.call_count == 2
        assert result.body == b"data"

    @respx.mock
    def test_never_retries_500(self):
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(500))

        config = make_fast_config()
        with pytest.raises(ApiError):
            download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        assert hop1.call_count == 1

    @respx.mock
    def test_retries_network_error_then_succeeds(self):
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.ConnectError("simulated network error"),
                httpx.Response(200, content=b"content", headers={"content-type": "text/plain"}),
            ]
        )

        config = make_fast_config()
        result = download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        assert hop1.call_count == 2
        assert result.body == b"content"

    @respx.mock
    def test_exhausts_cap_then_surfaces_error(self):
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(503))

        config = make_fast_config(max_retries=3)
        with pytest.raises(ApiError):
            download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        assert hop1.call_count == 3

    @respx.mock
    def test_zero_max_retries_still_sends_one_attempt(self):
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(503))

        config = make_fast_config(max_retries=0)
        with pytest.raises(ApiError):
            download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        assert hop1.call_count == 1

    @respx.mock
    def test_auth_on_every_hop1_attempt_never_on_hop2(self):
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(503),
                httpx.Response(302, headers={"Location": self.SIGNED}),
            ]
        )
        hop2 = respx.get(self.SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "application/pdf"})
        )

        config = make_fast_config()
        download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        for call in hop1.calls:
            assert call.request.headers.get("Authorization") == "Bearer test-token"
        assert "Authorization" not in hop2.calls[0].request.headers

    @respx.mock
    def test_hop1_sends_no_accept_header_on_any_attempt(self):
        """SPEC section 14: hop 1 carries Authorization and User-Agent only.

        A binary download is not a JSON API call, so the generic path's
        ``Accept: application/json`` must not ride along — on the first
        attempt or on a retry.
        """
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(503),
                httpx.Response(302, headers={"Location": self.SIGNED}),
            ]
        )
        hop2 = respx.get(self.SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "application/pdf"})
        )

        config = make_fast_config()
        download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        # httpx supplies its own transport-level "*/*" default when no Accept
        # is set, exactly as it does for the bare hop-2 client. Pin hop 1
        # against that baseline: identical to hop 2, and never the JSON Accept.
        hop2_accept = hop2.calls[0].request.headers.get("Accept")
        assert hop1.call_count == 2
        for call in hop1.calls:
            assert call.request.headers.get("Accept") == hop2_accept
            assert call.request.headers.get("Accept") != "application/json"
            assert "Content-Type" not in call.request.headers
            assert call.request.headers.get("User-Agent") is not None

    @respx.mock
    def test_balanced_hooks_across_retries(self):
        respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(503),
                httpx.Response(503),
                httpx.Response(200, content=b"content", headers={"content-type": "text/plain"}),
            ]
        )

        config = make_fast_config()
        hooks = RecordingHooks()
        download_sync(self.RAW, http_client=make_fast_http(config, hooks), config=config)

        assert hooks.starts == [1, 2, 3]
        assert hooks.ends == [1, 2, 3]
        # on_retry receives the UPCOMING attempt (SPEC §7 attempt semantics).
        assert hooks.retries == [2, 3]

    @respx.mock
    def test_honors_retry_after_on_429(self, monkeypatch):
        delays: list[float] = []
        monkeypatch.setattr("basecamp._http.time.sleep", lambda d: delays.append(d))

        respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(429, headers={"Retry-After": "7"}),
                httpx.Response(200, content=b"content", headers={"content-type": "text/plain"}),
            ]
        )

        config = make_fast_config()
        download_sync(self.RAW, http_client=make_fast_http(config), config=config)

        assert delays == [7.0]


class TestHop1RetryAsync:
    HOP1 = "https://3.basecampapi.com/files/doc.pdf"
    SIGNED = "https://signed.storage.com/doc.pdf"
    RAW = "https://original.com/files/doc.pdf"

    def make_async_http(self, config):
        auth = AsyncBearerAuth(AsyncStaticTokenProvider("test-token"))
        return AsyncHttpClient(config, auth)

    @respx.mock
    @pytest.mark.asyncio
    @pytest.mark.parametrize("status", [429, 502, 503, 504])
    async def test_retries_declared_status_then_succeeds(self, status):
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(status),
                httpx.Response(302, headers={"Location": self.SIGNED}),
            ]
        )
        respx.get(self.SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "application/pdf"})
        )

        config = make_fast_config()
        result = await download_async(self.RAW, http_client=self.make_async_http(config), config=config)

        assert hop1.call_count == 2
        assert result.body == b"data"

    @respx.mock
    @pytest.mark.asyncio
    async def test_never_retries_500(self):
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(500))

        config = make_fast_config()
        with pytest.raises(ApiError):
            await download_async(self.RAW, http_client=self.make_async_http(config), config=config)

        assert hop1.call_count == 1

    @respx.mock
    @pytest.mark.asyncio
    async def test_retries_network_error_then_succeeds(self):
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.ConnectError("simulated network error"),
                httpx.Response(200, content=b"content", headers={"content-type": "text/plain"}),
            ]
        )

        config = make_fast_config()
        result = await download_async(self.RAW, http_client=self.make_async_http(config), config=config)

        assert hop1.call_count == 2
        assert result.body == b"content"

    @respx.mock
    @pytest.mark.asyncio
    async def test_zero_max_retries_still_sends_one_attempt(self):
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(503))

        config = make_fast_config(max_retries=0)
        with pytest.raises(ApiError):
            await download_async(self.RAW, http_client=self.make_async_http(config), config=config)

        assert hop1.call_count == 1

    @respx.mock
    @pytest.mark.asyncio
    async def test_cancellation_is_terminal(self):
        # Cancellation is not a transport failure: it must propagate raw with
        # NO further attempts, even though a 503 would have been retried.
        # (A counting callable side effect: respx refuses to raise BaseException
        # types itself — CancelledError is a BaseException since Python 3.8 —
        # and does not record calls whose side effect raises one.)
        attempts = []

        def cancel(request):
            attempts.append(request)
            raise asyncio.CancelledError()

        respx.get(self.HOP1).mock(side_effect=cancel)

        config = make_fast_config()
        with pytest.raises(asyncio.CancelledError):
            await download_async(self.RAW, http_client=self.make_async_http(config), config=config)

        assert len(attempts) == 1

    @respx.mock
    @pytest.mark.asyncio
    async def test_hop1_sends_no_accept_header_on_any_attempt(self):
        """SPEC section 14 header scope, async transport (see the sync twin)."""
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(503),
                httpx.Response(302, headers={"Location": self.SIGNED}),
            ]
        )
        hop2 = respx.get(self.SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "application/pdf"})
        )

        config = make_fast_config()
        await download_async(self.RAW, http_client=self.make_async_http(config), config=config)

        # httpx supplies its own transport-level "*/*" default when no Accept
        # is set, exactly as it does for the bare hop-2 client. Pin hop 1
        # against that baseline: identical to hop 2, and never the JSON Accept.
        hop2_accept = hop2.calls[0].request.headers.get("Accept")
        assert hop1.call_count == 2
        for call in hop1.calls:
            assert call.request.headers.get("Accept") == hop2_accept
            assert call.request.headers.get("Accept") != "application/json"
            assert "Content-Type" not in call.request.headers
            assert call.request.headers.get("User-Agent") is not None
