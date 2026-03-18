from __future__ import annotations

import httpx
import pytest
import respx

from basecamp._http import HttpClient
from basecamp.auth import BearerAuth, StaticTokenProvider
from basecamp.config import Config
from basecamp.download import _rewrite_url, download_sync, filename_from_url
from basecamp.errors import UsageError


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
