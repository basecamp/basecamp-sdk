"""Reading a decoded JSON value the way the Go reference's ``json.Unmarshal`` does.

``json.Unmarshal`` of ``null`` is a no-op at any depth: it leaves the destination
at its zero value and returns no error. So a null object is the empty struct, a
null slice is no rows, a null string is ``""`` — none of them a failure. Anything
else of the wrong type is a decode error that fails the WHOLE response, never a
skipped field and never a coerced one.

Both halves matter and they are not symmetric. Collapsing them into "falsy
becomes empty" — ``value or []`` — reads ``0`` and ``false`` as an empty listing,
which turns a refusal the caller could act on into a silent wrong answer.

Python's generated services return plain ``dict``, so nothing typed sits between
the SDK and the wire and every reader has to do this decode itself. These
functions are that decode, in one place, so the rule cannot drift between the
readers that apply it. ``what`` names the thing being read, for the refusal
message.
"""

from __future__ import annotations

from typing import Any

from basecamp.errors import ApiError


def decoded_object(value: Any, what: str) -> dict[str, Any]:
    """An object field: ``{}`` for null, the dict itself, else a decode error."""
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise ApiError(f"{what} was not an object: {type(value).__name__}")
    return value


def decoded_optional_object(value: Any, what: str) -> dict[str, Any] | None:
    """A pointer-to-struct field, where null stays ``None`` rather than ``{}``.

    Go tells `*Bucket` nil from `&Bucket{}`, and a nil check on such a pointer is
    a real branch, so the two cannot be collapsed here either.
    """
    if value is None:
        return None
    if not isinstance(value, dict):
        raise ApiError(f"{what} was not an object: {type(value).__name__}")
    return value


def decoded_array(value: Any, what: str) -> list[Any]:
    """A slice field: ``[]`` for null, the list itself, else a decode error."""
    if value is None:
        return []
    if not isinstance(value, list):
        raise ApiError(f"{what} was not an array: {type(value).__name__}")
    return value


def decoded_string(value: Any, what: str) -> str:
    """A string field: ``""`` for null, the str itself, else a decode error."""
    if value is None:
        return ""
    if not isinstance(value, str):
        raise ApiError(f"{what} was not a string: {type(value).__name__}")
    return value
