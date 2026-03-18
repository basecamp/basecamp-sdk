from __future__ import annotations

from dataclasses import dataclass
from urllib.parse import unquote, urlparse, urlunparse

import httpx

from basecamp import _security
from basecamp.errors import ApiError, NetworkError, UsageError


@dataclass(frozen=True)
class DownloadResult:
    body: bytes
    content_type: str
    content_length: int
    filename: str


def filename_from_url(url: str) -> str:
    """Extract filename from URL path, URL-decoded."""
    path = urlparse(url).path
    name = unquote(path.rsplit("/", 1)[-1]) if "/" in path else ""
    return name or "download"


def _rewrite_url(raw_url: str, base_url: str) -> str:
    """Replace scheme+host with base_url origin, preserving base_url path prefix and download path+query+fragment."""
    parsed = urlparse(raw_url)
    base = urlparse(base_url)
    base_path = base.path.rstrip("/")
    new_path = base_path + parsed.path if base_path else parsed.path
    rewritten = parsed._replace(scheme=base.scheme, netloc=base.netloc, path=new_path)
    return urlunparse(rewritten)


def _parse_content_length(value: str | None) -> int:
    if not value:
        return -1
    try:
        parsed = int(value)
        return parsed if parsed >= 0 else -1
    except (ValueError, TypeError):
        return -1


def download_sync(raw_url: str, *, http_client, config) -> DownloadResult:
    """Perform sync download: URL rewrite → authenticated hop 1 → redirect → unauthenticated hop 2."""
    _validate_url(raw_url)
    rewritten_url = _rewrite_url(raw_url, config.base_url)

    response = http_client.get_no_retry(rewritten_url)

    if response.status_code in {301, 302, 303, 307, 308}:
        location = response.headers.get("Location") or response.headers.get("location")
        if not location:
            raise ApiError(f"redirect {response.status_code} with no Location header")
        resolved_url = _security.resolve_url(rewritten_url, location)
        signed_response = _fetch_signed(resolved_url, timeout=config.timeout)
        return DownloadResult(
            body=signed_response.content,
            content_type=signed_response.headers.get("content-type", ""),
            content_length=_parse_content_length(signed_response.headers.get("content-length")),
            filename=filename_from_url(raw_url),
        )

    if 200 <= response.status_code < 300:
        return DownloadResult(
            body=response.content,
            content_type=response.headers.get("content-type", ""),
            content_length=_parse_content_length(response.headers.get("content-length")),
            filename=filename_from_url(raw_url),
        )

    raise ApiError(f"Download failed with status {response.status_code}", http_status=response.status_code)


async def download_async(raw_url: str, *, http_client, config) -> DownloadResult:
    """Perform async download: URL rewrite → authenticated hop 1 → redirect → unauthenticated hop 2."""
    _validate_url(raw_url)
    rewritten_url = _rewrite_url(raw_url, config.base_url)

    response = await http_client.get_no_retry(rewritten_url)

    if response.status_code in {301, 302, 303, 307, 308}:
        location = response.headers.get("Location") or response.headers.get("location")
        if not location:
            raise ApiError(f"redirect {response.status_code} with no Location header")
        resolved_url = _security.resolve_url(rewritten_url, location)
        signed_response = await _fetch_signed_async(resolved_url, timeout=config.timeout)
        return DownloadResult(
            body=signed_response.content,
            content_type=signed_response.headers.get("content-type", ""),
            content_length=_parse_content_length(signed_response.headers.get("content-length")),
            filename=filename_from_url(raw_url),
        )

    if 200 <= response.status_code < 300:
        return DownloadResult(
            body=response.content,
            content_type=response.headers.get("content-type", ""),
            content_length=_parse_content_length(response.headers.get("content-length")),
            filename=filename_from_url(raw_url),
        )

    raise ApiError(f"Download failed with status {response.status_code}", http_status=response.status_code)


def _validate_url(raw_url: str) -> None:
    if not raw_url:
        raise UsageError("download URL is required")
    parsed = urlparse(raw_url)
    if not parsed.scheme or not parsed.netloc:
        raise UsageError("download URL must be an absolute URL")
    if parsed.scheme not in ("http", "https"):
        raise UsageError("download URL scheme must be http or https")


def _fetch_signed(url: str, *, timeout: float) -> httpx.Response:
    """Unauthenticated GET for signed download URL."""
    try:
        with httpx.Client(timeout=timeout, follow_redirects=True) as client:
            response = client.get(url)
        if response.status_code >= 400:
            raise ApiError(f"download failed with status {response.status_code}", http_status=response.status_code)
        return response
    except httpx.HTTPError as e:
        raise NetworkError(f"Download failed: {e}") from e


async def _fetch_signed_async(url: str, *, timeout: float) -> httpx.Response:
    """Async unauthenticated GET for signed download URL."""
    try:
        async with httpx.AsyncClient(timeout=timeout, follow_redirects=True) as client:
            response = await client.get(url)
        if response.status_code >= 400:
            raise ApiError(f"download failed with status {response.status_code}", http_status=response.status_code)
        return response
    except httpx.HTTPError as e:
        raise NetworkError(f"Download failed: {e}") from e
