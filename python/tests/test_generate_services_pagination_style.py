"""The pagination trait's `style` is load-bearing in this generator.

`has_pagination` decides whether the emitted method follows `Link: rel="next"`
and flattens the walk. It used to key off the trait's mere presence, so the
`cursor` style the trait documents would have produced exactly the walk it
exists to avoid. It now keys off `style`, which makes an unrecognised value
dangerous in a new way: read as "not paginated" it would silently ship a method
that never walks. So the parse refuses anything that is not `link` or `cursor`,
and these cases pin both halves.
"""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent.parent / "scripts"))
from generate_services import parse_operation  # noqa: E402


def _operation(style):
    operation = {
        "operationId": "PollWidgets",
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}},
    }
    if style is not None:
        operation["x-basecamp-pagination"] = {"style": style, "key": "events"}
    return operation


def _parse(style):
    return parse_operation("/{accountId}/widgets.json", "get", _operation(style), {})


def test_link_style_auto_paginates():
    assert _parse("link")["has_pagination"] is True


def test_cursor_style_does_not_auto_paginate():
    # One call is one page, carrying its own position; flattening would swallow
    # every position a consumer could resume from.
    assert _parse("cursor")["has_pagination"] is False


def test_no_trait_at_all_is_simply_unpaginated():
    assert _parse(None)["has_pagination"] is False


def test_the_key_is_withheld_from_a_cursor_operation():
    # The key drives envelope unwrapping in type resolution: set, it turns
    # `{events: [...], position}` into the item type. A cursor operation is
    # typed as its envelope, so the key must not reach those consumers at all.
    assert _parse("link")["pagination_key"] == "events"
    assert _parse("cursor")["pagination_key"] is None


@pytest.mark.parametrize("style", ["page", "linkk", "Link", "", None])
def test_an_unrecognised_style_is_refused_rather_than_read_as_unpaginated(style):
    operation = _operation("link")
    if style is None:
        del operation["x-basecamp-pagination"]["style"]
    else:
        operation["x-basecamp-pagination"]["style"] = style

    with pytest.raises(ValueError, match="unsupported pagination style"):
        parse_operation("/{accountId}/widgets.json", "get", operation, {})
