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


def decoded_envelope_array(envelope: dict[str, Any], key: str, what: str) -> list[Any]:
    """The ITEMS member of a wrapped-pagination envelope, read as required.

    Stricter than :func:`decoded_array`, and deliberately. SPEC §6 "Statusless
    ``api_error`` for a malformed 2xx body" settles this shape for every SDK:
    *an absent or wrong-typed member of the envelope is a malformed body and not
    an empty result*, because BC3 writes these envelopes unconditionally.

    So the null-is-empty rule stops at the envelope's door. It governs a bare
    array body, where Go's ``json.Unmarshal`` and the wire contract agree that a
    null list is no rows; it does not govern a member the server always writes,
    where absence means the body did not arrive intact. Conflating the two hands
    the caller a 2xx-shaped result with silently empty items — the
    loud-crash-for-a-silent-wrong-answer trade this module exists to refuse.

    **Scope, stated rather than implied.** §6 describes the decode in two halves,
    the items array on every page *and the first page's remaining members*. This
    reader is the first half only. The sibling members — ``person`` on
    ``GetPersonProgress``, the one operation §6 names — are still copied through
    unvalidated by the paginator above, so an envelope carrying items but no
    ``person`` is accepted here and surfaces as a ``KeyError`` at the caller.
    That half needs the required-member names, which live in the OpenAPI schema
    and would have to reach the primitive through the generator the way Swift's
    and Kotlin's do; it is a separate change and is not made here. Do not read
    this function as closing it.

    **On null.** §6 says "absent or wrong-typed" and is silent on ``null``; this
    reads a null member as wrong-typed. That is a deliberate divergence from Go,
    whose ``json.Unmarshal`` leaves a null member at its zero value without
    error. It follows the SDKs that already decode this shape through types:
    Rust's ``events`` is a bare ``Vec<TimelineEvent>`` with no ``serde(default)``,
    Kotlin's ``requiredMember`` passes ``JsonNull`` on to a decode that throws,
    and Swift's ``guard let`` clears an ``NSNull`` that then fails the cast. No
    SDK reads a null envelope member as an empty listing.
    """
    if key not in envelope:
        raise ApiError(f"{what} is absent from the response envelope")
    value = envelope[key]
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
