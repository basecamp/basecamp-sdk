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


def _is_localhost_host(host: str) -> bool:
    h = host.lower()
    return h in ("localhost", "127.0.0.1", "::1") or h.endswith(".localhost")


def require_origin_root(raw: str, label: str = "origin") -> str:
    """Parse and enforce the origin-root profile, returning the normalized origin.

    Accepts iff scheme is https (or http on localhost), a host is present, any
    port is valid, the path is empty or exactly ``/``, and there is no query,
    fragment, or userinfo. Parsing uses ``httpx.URL`` — the SAME transport parser
    the client dials with, never a regex or a divergent parser like ``urllib`` —
    so bracketed IPv6 (``http://[::1]:3000``), ports, and IDNA/IPvFuture handling
    agree with the host the client actually dials (no parser-differential bypass).

    Raises :class:`~basecamp.errors.UsageError` on any violation: a bad
    caller-supplied origin is a usage error. Callers validating an *advertised*
    origin catch and reclassify (e.g. ``invalid_issuer_origin``).

    Returns the normalized origin (``scheme://host[:port]``, no trailing slash,
    default port dropped).
    """
    # Parse with httpx.URL — the SAME transport parser the client dials with (see
    # is_localhost below and _http.py) — so validation can never disagree with the
    # request about scheme/host/port. urllib and httpx diverge on IDNA labels and
    # IPvFuture authorities: validating with urllib let a malformed value pass here
    # yet be rewritten or rejected at request time (a parser differential). httpx
    # rejects an invalid IDNA A-label (IDNAError, a UnicodeError) and an IPvFuture
    # authority like "https://[v1.foo]" (InvalidURL) rather than converting them.
    try:
        url = httpx.URL(raw)
        scheme = url.scheme
        # Force IDNA validation: `.host` decodes the A-label and raises IDNAError
        # (a UnicodeError) on an invalid one (e.g. a trailing-hyphen label httpx
        # cannot represent), matching what the transport would reject at dial time.
        _ = url.host
        # Bind against the ASCII host the transport actually dials (raw_host is
        # the IDNA A-label form), NOT the decoded Unicode `.host`: an AS that
        # correctly echoes its ASCII issuer/resource URI must still match by
        # code-point. `.host` would rewrite "xn--..." to Unicode and break that.
        host = url.raw_host.decode("ascii")
    except (httpx.InvalidURL, ValueError) as exc:
        raise UsageError(f"Invalid {label}: not a valid absolute URL: {raw}") from exc

    if not scheme or not host:
        raise UsageError(f"Invalid {label}: not a valid absolute URL: {raw}")

    is_localhost_http = scheme == "http" and _is_localhost_host(host)
    if scheme != "https" and not is_localhost_http:
        raise UsageError(f"{label} must use HTTPS (or http on localhost): {raw}")
    # Reject on the *presence* of userinfo, not its truthiness: httpx drops an
    # empty userinfo (authorities like "@host" report userinfo == b""), so also
    # inspect the raw authority for an "@" delimiter regardless of what surrounds
    # it — an "@" there is always a userinfo delimiter (a host cannot contain one).
    authority = raw.split("://", 1)[-1].split("/", 1)[0].split("?", 1)[0].split("#", 1)[0]
    if url.userinfo or "@" in authority:
        raise UsageError(f"{label} must not contain userinfo: {raw}")
    # httpx collapses a bare trailing "?" to an empty query (b"") and a bare "#"
    # to an empty fragment (""), both falsy, so the parsed fields can't tell a
    # bare delimiter from none. Scan the raw input too: any "?"/"#" past the
    # scheme is a delimiter here (host/port carry neither; path is ""/"/" below).
    if url.query or url.fragment or "?" in raw or "#" in raw:
        raise UsageError(f"{label} must not contain a query or fragment: {raw}")
    if url.path not in ("", "/"):
        raise UsageError(f"{label} must be an origin root (no path): {raw}")

    # A dangling port delimiter ("https://host:") normalizes to port None under
    # httpx, silently accepting a malformed authority. Also reject a signed port
    # token ("+1"/"-1"): httpx parses "+1" to 1, which would pass the range check.
    # Inspect the raw authority's port segment — IPv6 is bracketed ("[::1]"), so a
    # port follows "]:"; otherwise it follows the sole ":".
    if "]:" in authority:
        port_token: str | None = authority.rsplit("]:", 1)[1]
    elif "]" not in authority and ":" in authority:
        port_token = authority.rsplit(":", 1)[1]
    else:
        port_token = None
    if port_token is not None and not (port_token.isascii() and port_token.isdigit()):
        # Empty (dangling ":") or non-digit ("+1") port token — malformed authority.
        raise UsageError(f"{label} has an invalid port: {raw}")

    # httpx does not range-check the port (it accepts :99999), so enforce 1–65535
    # explicitly. httpx already drops an absent/default port to None.
    port = url.port
    if port is not None and not (1 <= port <= 65535):
        raise UsageError(f"{label} has an invalid port: {raw}")

    host_part = f"[{host}]" if ":" in host else host
    if port is None:
        return f"{scheme}://{host_part}"
    return f"{scheme}://{host_part}:{port}"


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
