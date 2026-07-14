from __future__ import annotations

from urllib.parse import urljoin, urlparse

import httpx

from basecamp.errors import ApiError, UsageError

MAX_ERROR_MESSAGE_BYTES = 500
MAX_RESPONSE_BODY_BYTES = 50 * 1024 * 1024  # 50 MB
MAX_ERROR_BODY_BYTES = 1 * 1024 * 1024  # 1 MB

SENSITIVE_HEADERS = frozenset({"authorization", "cookie", "set-cookie", "x-csrf-token"})

# The Launchpad authorization endpoint lives on a different origin than the
# configured API base URL, so it is the one sanctioned destination for a
# credentialed cross-origin request. Any other foreign origin must be rejected.
LAUNCHPAD_AUTHORIZATION_URL = "https://launchpad.37signals.com/authorization.json"


def truncate(s: str | None, max_bytes: int = MAX_ERROR_MESSAGE_BYTES) -> str:
    if s is None:
        return ""
    encoded = s.encode()
    if len(encoded) <= max_bytes:
        return s
    if max_bytes <= 3:
        return encoded[:max_bytes].decode(errors="ignore")
    return encoded[: max_bytes - 3].decode(errors="ignore") + "..."


def require_https(url: str, label: str = "URL") -> None:
    try:
        parsed = urlparse(url)
    except ValueError as e:
        raise UsageError(f"Invalid {label}: {url}") from e
    if parsed.scheme.lower() != "https":
        raise UsageError(f"{label} must use HTTPS: {url}")
    if not parsed.hostname:
        raise UsageError(f"{label} must include a hostname: {url}")


def is_localhost(url: str) -> bool:
    # Decide with the SAME parser the transport dials with (httpx.URL, see
    # _http.py) so the guard can never disagree with the client about which
    # host a URL targets. A guard that extracts the host with a different
    # parser than the transport invites parser-differential bypasses.
    try:
        parsed = httpx.URL(url)
    except httpx.InvalidURL:
        return False
    # The carve-out is limited to HTTP(S) so credential guards fail closed on
    # any other scheme (e.g. ws://localhost).
    if parsed.scheme.lower() not in ("http", "https"):
        return False
    host = parsed.host.lower()
    return host in ("localhost", "127.0.0.1", "::1") or host.endswith(".localhost")


def same_origin(a: str, b: str) -> bool:
    # Same parser as the transport (httpx.URL) — see is_localhost.
    try:
        ua = httpx.URL(a)
        ub = httpx.URL(b)
    except httpx.InvalidURL:
        return False
    if not ua.scheme or not ub.scheme:
        return False
    return ua.scheme.lower() == ub.scheme.lower() and _normalize_host(ua) == _normalize_host(ub)


def resolve_url(base: str, target: str) -> str:
    try:
        return urljoin(base, target)
    except ValueError:
        return target


def check_body_size(
    body: bytes | str | None, max_bytes: int = MAX_RESPONSE_BODY_BYTES, label: str = "Response"
) -> None:
    if body is None:
        return
    size = len(body) if isinstance(body, bytes) else len(body.encode())
    if size > max_bytes:
        raise ApiError(f"{label} body too large ({size} bytes, max {max_bytes})")


def redact_headers(headers: dict[str, str]) -> dict[str, str]:
    return {k: "[REDACTED]" if k.lower() in SENSITIVE_HEADERS else v for k, v in headers.items()}


def _normalize_host(parsed: httpx.URL) -> str:
    host = parsed.host.lower()
    # httpx already drops an explicit default port (:443/:80) at parse time;
    # keep the normalization anyway so this cannot silently regress.
    port = parsed.port
    if port is None:
        return host
    if parsed.scheme.lower() == "https" and port == 443:
        return host
    if parsed.scheme.lower() == "http" and port == 80:
        return host
    return f"{host}:{port}"
