from __future__ import annotations

import re
from dataclasses import dataclass
from typing import TypeVar

T = TypeVar("T")


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


def parse_next_link(link_header: str | None) -> str | None:
    """Parse the next page URL from a Link header."""
    if not link_header:
        return None
    for part in link_header.split(","):
        part = part.strip()
        if 'rel="next"' in part:
            match = re.search(r"<([^>]+)>", part)
            if match:
                return match.group(1)
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
