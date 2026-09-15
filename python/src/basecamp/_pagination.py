from __future__ import annotations

from dataclasses import dataclass
from typing import Any, TypeVar

from basecamp.errors import ApiError

T = TypeVar("T")

# JSON shape names for refusal messages, keyed by exact type so ``bool`` never
# resolves through its ``int`` base class.
_JSON_TYPE_NAMES: dict[type, str] = {
    type(None): "null",
    bool: "boolean",
    int: "number",
    float: "number",
    str: "string",
    list: "array",
    dict: "object",
}


def _json_type_name(value: Any) -> str:
    return _JSON_TYPE_NAMES.get(type(value), type(value).__name__)


def decode_list(value: Any, *, where: str, at: str | None = None) -> list:
    """Read a decoded JSON value as a list of items, the way Go's reference does.

    ``json.Unmarshal`` of ``null`` is a no-op at any depth: it leaves the
    destination at its zero value and returns no error, so a null list — a null
    body, a null value under the item key, or a key that is absent altogether —
    is *no rows*, not a failure. Every other non-array shape fails the decode of
    the whole response, which is why this raises rather than falling back to an
    empty page.

    The distinction is the point. ``value or []`` would read ``0`` and ``false``
    as an empty listing too, turning a loud refusal into a silent wrong answer.

    ``where`` names the read for the refusal message; ``at`` names the envelope
    key when the list came from inside one.
    """
    if value is None:
        return []
    if not isinstance(value, list):
        location = "" if at is None else f" at {at!r}"
        raise ApiError(f"Failed to parse {where}: expected a JSON array{location}, got {_json_type_name(value)}")
    return value


def decode_envelope(value: Any, *, where: str) -> dict:
    """Read a decoded JSON value as a keyed envelope, the way Go's reference does.

    Same rule as :func:`decode_list` one level up: Go unmarshals these bodies
    into a struct, so ``null`` yields the zero struct — every field empty, which
    includes the item key — and every other non-object shape, an array included,
    fails the decode.
    """
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise ApiError(f"Failed to parse {where}: expected a JSON object, got {_json_type_name(value)}")
    return value


@dataclass(frozen=True)
class ListMeta:
    total_count: int = 0
    truncated: bool = False


class ListResult(list[T]):
    """A list with pagination metadata."""

    meta: ListMeta

    def __init__(self, items: list[T], meta: ListMeta | None = None):
        super().__init__(items)
        self.meta = meta or ListMeta()

    def __repr__(self) -> str:
        return f"ListResult({list.__repr__(self)}, meta={self.meta!r})"


def _extract_angle_bracketed(part: str) -> str | None:
    """Return the contents of the first non-empty ``<...>`` pair.

    This is the leftmost-match semantics of ``<([^>]+)>`` in linear time. The
    regex form is quadratic on a header carrying many ``<`` with no reachable
    ``>``, because every ``<`` is retried as a start position and each scans to
    the end. Searching for ``>`` from *after* the ``<`` visits each character
    once instead.

    An empty ``<>`` is skipped rather than returned, because ``[^>]+`` requires
    at least one character — the regex would move on to the next ``<``, and so
    does this.
    """
    cursor = 0
    while True:
        start = part.find("<", cursor)
        if start < 0:
            return None
        end = part.find(">", start + 1)
        if end < 0:
            return None
        if end > start + 1:
            return part[start + 1 : end]
        cursor = start + 1


def parse_next_link(link_header: str | None) -> str | None:
    """Parse the next page URL from a Link header."""
    if not link_header:
        return None
    for part in link_header.split(","):
        part = part.strip()
        if 'rel="next"' in part:
            url = _extract_angle_bracketed(part)
            if url is not None:
                return url
    return None


def selects_single_page(params: dict | None) -> bool:
    """Report whether the outgoing query pins a single page (SPEC section 8).

    A positive ``page`` is a selector, not a starting offset: the operation
    issues exactly one request and never follows ``Link: rel="next"``. The
    query parameters are the authority here — ``page`` reaches the wire only
    when the caller passed it, so reading it back needs no separate plumbing
    through every generated service method.

    ``bool`` is excluded explicitly because it subclasses ``int``: a stray
    ``page=True`` is a caller mistake, not a request for page 1.
    """
    if not params:
        return False
    page = params.get("page")
    return isinstance(page, int) and not isinstance(page, bool) and page > 0


def parse_total_count(headers: dict[str, str]) -> int:
    """Parse X-Total-Count header, returning 0 if missing."""
    value = headers.get("X-Total-Count") or headers.get("x-total-count") or ""
    try:
        return int(value)
    except (ValueError, TypeError):
        return 0
