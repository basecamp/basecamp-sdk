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


def parse_total_count(headers: dict[str, str]) -> int:
    """Parse X-Total-Count header, returning 0 if missing."""
    value = headers.get("X-Total-Count") or headers.get("x-total-count") or ""
    try:
        return int(value)
    except (ValueError, TypeError):
        return 0
