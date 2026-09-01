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
from basecamp.errors import ApiError, AuthError, NetworkError, UsageError
from basecamp.hooks import BasecampHooks


class _RefreshableProvider(StaticTokenProvider):
    """Counts refreshes so a test can tell "declined to refresh" from "refreshed"."""

    refreshable = True

    def __init__(self):
        super().__init__("test-token")
        self.refreshes = 0

    def refresh(self):
        self.refreshes += 1
        return True


class _AsyncRefreshableProvider(AsyncStaticTokenProvider):
    refreshable = True

    def __init__(self):
        super().__init__("test-token")
        self.refreshes = 0

    async def refresh(self):
        self.refreshes += 1
        return True


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

    # SPEC §14 "Hop-2 Redirect Policy": the signed URL is the one destination the
    # API host named. A redirect from it surfaces with its status, and the Location it
    # names is never dialled (#805). Before hop 2 passed follow_redirects=False,
    # httpx followed this chain and the caller received b"SECRET" as the file.
    @respx.mock
    def test_hop2_redirect_is_refused_not_followed(self):
        respx.get("https://3.basecampapi.com/files/doc.pdf").mock(
            return_value=httpx.Response(302, headers={"Location": "https://signed.storage.com/doc.pdf?sig=xyz"})
        )
        respx.get("https://signed.storage.com/doc.pdf?sig=xyz").mock(
            return_value=httpx.Response(302, headers={"Location": "https://elsewhere.example.com/final/doc.pdf"})
        )
        third = respx.get("https://elsewhere.example.com/final/doc.pdf").mock(
            return_value=httpx.Response(200, content=b"SECRET")
        )

        with pytest.raises(ApiError, match="not followed") as excinfo:
            download_sync("https://original.com/files/doc.pdf", http_client=make_http(), config=make_config())

        assert excinfo.value.http_status == 302
        assert not third.called

    @respx.mock
    @pytest.mark.asyncio
    async def test_hop2_redirect_is_refused_not_followed_async(self):
        respx.get("https://3.basecampapi.com/files/doc.pdf").mock(
            return_value=httpx.Response(302, headers={"Location": "https://signed.storage.com/doc.pdf?sig=xyz"})
        )
        respx.get("https://signed.storage.com/doc.pdf?sig=xyz").mock(
            return_value=httpx.Response(302, headers={"Location": "https://elsewhere.example.com/final/doc.pdf"})
        )
        third = respx.get("https://elsewhere.example.com/final/doc.pdf").mock(
            return_value=httpx.Response(200, content=b"SECRET")
        )
        config = make_config()
        http = AsyncHttpClient(config, AsyncBearerAuth(AsyncStaticTokenProvider("test-token")))

        with pytest.raises(ApiError, match="not followed") as excinfo:
            await download_async("https://original.com/files/doc.pdf", http_client=http, config=config)

        assert excinfo.value.http_status == 302
        assert not third.called

    # Hop 2 used to accept anything below 400, so a 3xx the client no longer
    # follows — or a 304, which is outside the redirect set — would have
    # returned as a success with an empty body. Any non-2xx is the download
    # failing.
    @respx.mock
    def test_hop2_non_2xx_below_400_is_a_failure(self):
        respx.get("https://3.basecampapi.com/files/doc.pdf").mock(
            return_value=httpx.Response(302, headers={"Location": "https://signed.storage.com/doc.pdf?sig=xyz"})
        )
        respx.get("https://signed.storage.com/doc.pdf?sig=xyz").mock(return_value=httpx.Response(304))

        with pytest.raises(ApiError, match="download failed with status 304") as excinfo:
            download_sync("https://original.com/files/doc.pdf", http_client=make_http(), config=make_config())

        assert excinfo.value.http_status == 304


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
    def test_refresh_replay_is_not_attempted_without_budget(self):
        """SPEC §4/§14: with retry disabled, ONE request goes out — refresh included.

        The replay is a request on the wire, so it spends an attempt from the
        same total-attempt budget as a transient retry. A cap of one means one
        request no matter what would have caused the second (#565, #461).

        The refresh itself must not fire either: §4 checks the budget BEFORE
        calling refresh(), because rotating a token the SDK has no attempt left
        to use burns it for nothing and still surfaces the stale 401.
        """
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(401))

        provider = _RefreshableProvider()
        config = make_fast_config(max_retries=0)
        http = HttpClient(config, BearerAuth(provider))
        with pytest.raises(AuthError):
            download_sync(self.RAW, http_client=http, config=config)

        assert hop1.call_count == 1
        assert provider.refreshes == 0

    @respx.mock
    def test_refresh_replay_spends_an_attempt_from_the_budget(self):
        """A budget of two allows the refresh replay, and it consumes attempt 2."""
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(401))

        provider = _RefreshableProvider()
        config = make_fast_config(max_retries=2)
        http = HttpClient(config, BearerAuth(provider))
        with pytest.raises(AuthError):
            download_sync(self.RAW, http_client=http, config=config)

        # Two requests, not three: the replay drew from the same budget, and
        # §4 allows only one refresh per request regardless.
        assert hop1.call_count == 2
        assert provider.refreshes == 1

    @respx.mock
    def test_refresh_replay_succeeds_when_the_new_token_works(self):
        """The refreshed request is actually SENT, not refreshed-then-discarded."""
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(401),
                httpx.Response(302, headers={"Location": self.SIGNED}),
            ]
        )
        hop2 = respx.get(self.SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "application/pdf"})
        )

        provider = _RefreshableProvider()
        config = make_fast_config(max_retries=2)
        http = HttpClient(config, BearerAuth(provider))
        result = download_sync(self.RAW, http_client=http, config=config)

        assert result.body == b"data"
        assert hop1.call_count == 2
        assert hop2.call_count == 1
        assert provider.refreshes == 1

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
    async def test_honors_retry_after_on_429(self, monkeypatch):
        """The async transport's backoff is pinned too, not just the sync one."""
        delays: list[float] = []

        async def record(d):
            delays.append(d)

        monkeypatch.setattr("basecamp._async_http.asyncio.sleep", record)

        respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(429, headers={"Retry-After": "7"}),
                httpx.Response(200, content=b"content", headers={"content-type": "text/plain"}),
            ]
        )

        config = make_fast_config()
        await download_async(self.RAW, http_client=self.make_async_http(config), config=config)

        assert delays == [7.0]

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

    @respx.mock
    @pytest.mark.asyncio
    async def test_refresh_replay_is_not_attempted_without_budget(self):
        """SPEC §4/§14 budget gate, async transport (see the sync twin)."""
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(401))

        provider = _AsyncRefreshableProvider()
        config = make_fast_config(max_retries=0)
        http = AsyncHttpClient(config, AsyncBearerAuth(provider))
        with pytest.raises(AuthError):
            await download_async(self.RAW, http_client=http, config=config)

        assert hop1.call_count == 1
        assert provider.refreshes == 0

    @respx.mock
    @pytest.mark.asyncio
    async def test_refresh_replay_spends_an_attempt_from_the_budget(self):
        hop1 = respx.get(self.HOP1).mock(return_value=httpx.Response(401))

        provider = _AsyncRefreshableProvider()
        config = make_fast_config(max_retries=2)
        http = AsyncHttpClient(config, AsyncBearerAuth(provider))
        with pytest.raises(AuthError):
            await download_async(self.RAW, http_client=http, config=config)

        assert hop1.call_count == 2
        assert provider.refreshes == 1

    @respx.mock
    @pytest.mark.asyncio
    async def test_refresh_replay_succeeds_when_the_new_token_works(self):
        """The refreshed request is actually SENT, not awaited-then-discarded.

        Pins the await too: a refresh coroutine truthy-tested without awaiting
        would pass the gate and then never rotate the token (#563's de8c9255d).
        """
        hop1 = respx.get(self.HOP1).mock(
            side_effect=[
                httpx.Response(401),
                httpx.Response(302, headers={"Location": self.SIGNED}),
            ]
        )
        hop2 = respx.get(self.SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "application/pdf"})
        )

        provider = _AsyncRefreshableProvider()
        config = make_fast_config(max_retries=2)
        http = AsyncHttpClient(config, AsyncBearerAuth(provider))
        result = await download_async(self.RAW, http_client=http, config=config)

        assert result.body == b"data"
        assert hop1.call_count == 2
        assert hop2.call_count == 1
        assert provider.refreshes == 1


class TestSpec9CredentialRendering:
    """SPEC §9: credential-bearing values are never rendered."""

    HOP1 = "https://3.basecampapi.com/files/doc.pdf"
    SIGNED = "https://signed.storage.com/doc.pdf?X-Amz-Signature=SECRET"
    RAW = "https://original.com/files/doc.pdf"
    HOP1_SIGNED = "https://3.basecampapi.com/files/doc.pdf?verifier=SECRET"
    RAW_SIGNED = "https://original.com/files/doc.pdf?verifier=SECRET"

    @respx.mock
    def test_hop2_network_error_is_fixed_and_retains_nothing(self):
        respx.get(self.HOP1).mock(return_value=httpx.Response(302, headers={"Location": self.SIGNED}))
        respx.get(self.SIGNED).mock(side_effect=httpx.ConnectError(f"dial {self.SIGNED} refused"))

        with pytest.raises(NetworkError) as exc_info:
            download_sync(self.RAW, http_client=make_http(), config=make_config())

        err = exc_info.value
        # Fixed message, and the raising boundary retains nothing on the
        # RAISED exception: not __cause__, and not the __context__ that
        # `raise ... from None` would still leave populated.
        assert str(err) == "Download failed"
        assert err.__cause__ is None
        assert err.__context__ is None

    @respx.mock
    @pytest.mark.asyncio
    async def test_hop2_network_error_is_fixed_and_retains_nothing_async(self):
        respx.get(self.HOP1).mock(return_value=httpx.Response(302, headers={"Location": self.SIGNED}))
        respx.get(self.SIGNED).mock(side_effect=httpx.ConnectError(f"dial {self.SIGNED} refused"))

        config = make_config()
        http = AsyncHttpClient(config, AsyncBearerAuth(AsyncStaticTokenProvider("test-token")))
        with pytest.raises(NetworkError) as exc_info:
            await download_async(self.RAW, http_client=http, config=config)

        err = exc_info.value
        assert str(err) == "Download failed"
        assert err.__cause__ is None
        assert err.__context__ is None

    @respx.mock
    def test_hop1_hook_urls_omit_query_while_wire_keeps_it(self):
        # The caller's URL can smuggle a signed query through the origin
        # rewrite into hop 1, so hooks see origin+path only; the wire request
        # keeps the query (respx matches it exactly).
        hop1 = respx.get(self.HOP1_SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "text/plain"})
        )

        class UrlHooks(BasecampHooks):
            def __init__(self):
                self.urls: list[str] = []

            def on_request_start(self, info):
                self.urls.append(info.url)

            def on_request_end(self, info, result):
                self.urls.append(info.url)

        config = make_config()
        hooks = UrlHooks()
        http = HttpClient(config, BearerAuth(StaticTokenProvider("test-token")), hooks)
        download_sync(self.RAW_SIGNED, http_client=http, config=config)

        assert hop1.called
        assert hooks.urls == ["https://3.basecampapi.com/files/doc.pdf"] * 2

    @respx.mock
    def test_on_retry_url_omits_query(self):
        respx.get(self.HOP1_SIGNED).mock(
            side_effect=[
                httpx.Response(503),
                httpx.Response(200, content=b"data", headers={"content-type": "text/plain"}),
            ]
        )

        class RetryUrlHooks(BasecampHooks):
            def __init__(self):
                self.urls: list[str] = []

            def on_retry(self, info, attempt, error, delay):
                self.urls.append(info.url)

        config = make_fast_config()
        hooks = RetryUrlHooks()
        download_sync(self.RAW_SIGNED, http_client=make_fast_http(config, hooks), config=config)

        assert hooks.urls == ["https://3.basecampapi.com/files/doc.pdf"]

    @respx.mock
    @pytest.mark.asyncio
    async def test_hop1_hook_urls_omit_query_async(self):
        hop1 = respx.get(self.HOP1_SIGNED).mock(
            return_value=httpx.Response(200, content=b"data", headers={"content-type": "text/plain"})
        )

        class UrlHooks(BasecampHooks):
            def __init__(self):
                self.urls: list[str] = []

            def on_request_start(self, info):
                self.urls.append(info.url)

            def on_request_end(self, info, result):
                self.urls.append(info.url)

        config = make_config()
        hooks = UrlHooks()
        http = AsyncHttpClient(config, AsyncBearerAuth(AsyncStaticTokenProvider("test-token")), hooks)
        await download_async(self.RAW_SIGNED, http_client=http, config=config)

        assert hop1.called
        assert hooks.urls == ["https://3.basecampapi.com/files/doc.pdf"] * 2

    @respx.mock
    def test_hop1_network_error_is_severed_for_caller_and_hooks(self):
        # The httpx error retains the request it failed on (and any signed
        # query in its URL), so the caller's error and the hook's are the
        # fixed NetworkError with neither __cause__ nor __context__.
        respx.get(self.HOP1_SIGNED).mock(side_effect=httpx.ConnectError("dial refused"))

        class ErrorHooks(BasecampHooks):
            def __init__(self):
                self.errors: list[BaseException | None] = []

            def on_request_end(self, info, result):
                self.errors.append(result.error)

        config = make_fast_config(max_retries=1)
        hooks = ErrorHooks()
        with pytest.raises(NetworkError) as exc_info:
            download_sync(self.RAW_SIGNED, http_client=make_fast_http(config, hooks), config=config)

        assert len(hooks.errors) == 1
        for err in (exc_info.value, hooks.errors[0]):
            assert isinstance(err, NetworkError)
            assert str(err) == "Network error"
            assert err.__cause__ is None
            assert err.__context__ is None

    @respx.mock
    @pytest.mark.asyncio
    async def test_hop1_network_error_is_severed_async(self):
        respx.get(self.HOP1_SIGNED).mock(side_effect=httpx.ConnectError("dial refused"))

        config = make_fast_config(max_retries=1)
        http = AsyncHttpClient(config, AsyncBearerAuth(AsyncStaticTokenProvider("test-token")))
        with pytest.raises(NetworkError) as exc_info:
            await download_async(self.RAW_SIGNED, http_client=http, config=config)

        assert str(exc_info.value) == "Network error"
        assert exc_info.value.__cause__ is None
        assert exc_info.value.__context__ is None

    @respx.mock
    def test_api_network_error_keeps_its_cause(self):
        # The projection is gated on the download flow; ordinary API requests
        # keep their transport diagnostic.
        respx.get("https://3.basecampapi.com/test.json").mock(side_effect=httpx.ConnectError("refused"))

        config = make_fast_config(max_retries=1)
        with pytest.raises(NetworkError) as exc_info:
            make_fast_http(config).get("/test.json")

        assert isinstance(exc_info.value.__cause__, httpx.ConnectError)


class TestDisplayUrl:
    def test_drops_userinfo_query_and_fragment_and_keeps_port(self):
        from basecamp._security import display_url

        assert display_url("https://user:pw@host.example:8443/a/b?sig=SECRET#frag") == "https://host.example:8443/a/b"
        assert display_url("https://host.example/a") == "https://host.example/a"
        assert display_url("http://[::1]:3000/x?y=1") == "http://[::1]:3000/x"

    def test_unparsable_input_renders_fixed_token(self):
        from basecamp._security import display_url

        assert display_url("http://[::1:3000/x") == "unparsable"
        assert display_url("not a url") == "unparsable"


class TestUndialableSignedLocation:
    HOP1 = "https://3.basecampapi.com/files/doc.pdf"
    RAW = "https://original.com/files/doc.pdf"
    BAD = "https://signed.example:SECRET/file?sig=SECRET"

    @respx.mock
    def test_malformed_location_is_projected_at_hop_1(self):
        # httpx builds the redirect request even with follow_redirects=False,
        # so a malformed Location fails hop 1 itself — inside the download
        # boundary, which renders the fixed message and chains nothing.
        respx.get(self.HOP1).mock(return_value=httpx.Response(302, headers={"Location": self.BAD}))

        with pytest.raises(NetworkError) as exc_info:
            download_sync(self.RAW, http_client=make_http(), config=make_config())

        err = exc_info.value
        assert str(err) == "Network error"
        assert err.__cause__ is None
        assert err.__context__ is None

    def test_hop2_invalid_url_renders_fixed_token(self):
        # The hop-2 boundary itself: httpx.InvalidURL sits outside HTTPError
        # and renders the offending component ("Invalid port: 'SECRET'").
        from basecamp.download import _fetch_signed

        with pytest.raises(ApiError) as exc_info:
            _fetch_signed(self.BAD, timeout=1.0)

        err = exc_info.value
        assert str(err) == "redirect to undialable download URL: unparsable"
        assert err.__cause__ is None
        assert err.__context__ is None

    @pytest.mark.asyncio
    async def test_hop2_invalid_url_renders_fixed_token_async(self):
        from basecamp.download import _fetch_signed_async

        with pytest.raises(ApiError) as exc_info:
            await _fetch_signed_async(self.BAD, timeout=1.0)

        assert str(exc_info.value) == "redirect to undialable download URL: unparsable"
        assert exc_info.value.__context__ is None
