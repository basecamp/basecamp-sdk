from __future__ import annotations

from urllib.parse import urljoin, urlparse

from basecamp.errors import ApiError, UsageError

MAX_ERROR_MESSAGE_BYTES = 500
MAX_RESPONSE_BODY_BYTES = 50 * 1024 * 1024  # 50 MB
MAX_ERROR_BODY_BYTES = 1 * 1024 * 1024  # 1 MB

SENSITIVE_HEADERS = frozenset({"authorization", "cookie", "set-cookie", "x-csrf-token"})


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


def is_localhost(url: str) -> bool:
    try:
        parsed = urlparse(url)
        host = (parsed.hostname or "").lower()
    except ValueError:
        return False
    return host in ("localhost", "127.0.0.1", "::1") or host.endswith(".localhost")


def same_origin(a: str, b: str) -> bool:
    try:
        ua = urlparse(a)
        ub = urlparse(b)
    except ValueError:
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


def _normalize_host(parsed) -> str:
    host = (parsed.hostname or "").lower()
    port = parsed.port
    if port is None:
        return host
    if parsed.scheme.lower() == "https" and port == 443:
        return host
    if parsed.scheme.lower() == "http" and port == 80:
        return host
    return f"{host}:{port}"
