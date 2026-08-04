"""Response guards shared by the merge-safe composites.

A merge-safe ``update``/``edit`` GETs a record, reads each writable field, and
PUTs the **full** representation back. The endpoint is full-replace, so every
value read here is written — including one the caller never mentioned. If the
read step coerces or forwards a malformed value instead of refusing it, that
value lands on the record.

There are two failure modes and they are the same defect wearing different
clothes:

* **erasure** — a falsey non-string coalesced to ``""`` by a plain ``or ""``
  (``False``, ``0``, ``[]``, ``{}`` all become ``""``), wiping the field;
* **corruption** — a truthy non-string forwarded verbatim (``42``, ``True``,
  ``["x"]``), writing a number, boolean, array or object where a string
  belongs.

Testing only the first is what let this class survive five review passes, so
both are refused here.

**The rule: a composite is safe exactly when a typed decoder sits between the
GET and the field read.** Go (``json.Unmarshal``), Swift (``Codable``) and
Kotlin (kotlinx.serialization) get one for free from their models. Python does
not — the generated services return ``dict[str, Any]``, so nothing rejects a
wrong-typed field and the check has to be explicit. That is why these guards
exist in Python, Ruby and TypeScript and nowhere else (#576).

Todolists carries its own copy of these guards (#574). #544 flattened the shape
those guards read - dropping the envelope-arm rung, not the guards - but did not
unify them here. A generated validating layer (#578) is the intended end state
for all of them.
"""

from __future__ import annotations

from typing import Any

from basecamp._security import truncate as _truncate
from basecamp.errors import ApiError

_RESEND_HINT = (
    "The merge-safe update/edit resend this field verbatim, so a coerced or empty value "
    "would overwrite the current one. Use {escape} to write the record deliberately."
)


def describe(value: object) -> str:
    """Render a value for an error message without ever throwing.

    The guard's own error path must not fail while explaining a failure:
    ``repr`` is arbitrary user code and can raise. The type name is always
    available; the rendering is a bonus, capped per SPEC section 9 and dropped
    if it fails.
    """
    kind = type(value).__name__
    try:
        return f"{kind} {_truncate(repr(value))}"
    except Exception:
        return kind


def malformed(message: str, hint: str) -> ApiError:
    """Build the malformed-response error.

    ``ApiError``, not ``UsageError``: the value arrived in a successful API
    response, so nothing the caller passed is at fault. Non-retryable, because
    re-requesting cannot repair a malformed body.
    """
    return ApiError(_truncate(message), hint=hint, retryable=False)


def require_mapping(body: object, *, record: str, operation: str, escape: str) -> dict[str, Any]:
    """The response must be a JSON object before any field is read.

    One level up from the malformed-*field* guards: a successful GET can return
    a scalar, a list, or null. ``body.get(key)`` raises ``AttributeError`` on a
    scalar or ``None`` and is absent entirely on a list, so a malformed envelope
    would surface as a native ``TypeError``/``AttributeError`` instead of the
    documented statusless ``api_error``.
    """
    if not isinstance(body, dict):
        raise malformed(
            f"{operation} returned {describe(body)} where a {record.lower()} object was expected",
            "The merge-safe update/edit read this record's fields before rewriting them, so a "
            f"non-object body cannot be used. Use {escape} to write the record deliberately.",
        )
    return body


def writable_string(body: dict[str, Any], key: str, *, record: str, escape: str) -> str:
    """Read a writable string field, refusing to coerce a malformed one.

    An absent key or an explicit ``None`` is genuinely empty — there is nothing
    to preserve and ``""`` is what the server already holds. An actual string
    passes verbatim. Anything else is a malformed response and is refused
    **before** the PUT, naming the offending field.
    """
    value = body.get(key)
    if value is None:
        return ""
    if not isinstance(value, str):
        raise malformed(
            f"{record} field {key!r} is not a string: {describe(value)}",
            _RESEND_HINT.format(escape=escape),
        )
    return value


def required_writable_string(body: dict[str, Any], key: str, *, record: str, escape: str) -> str:
    """Read a writable string the record is *required* to carry.

    :func:`writable_string` treats an absent key or an explicit ``None`` as
    genuinely empty, which is right for an optional field — ``""`` is what the
    server already holds. It is wrong for a required one. Where the spec marks a
    response member ``@required`` and BC3 can never render it blank, an absent,
    null or blank value in a 2xx body is a **malformed response**, not an empty
    field. Coalescing it to ``""`` and sending that in the full-replace PUT
    would blank the real value on a call that never mentioned it — #576's defect
    exactly.

    Two records rely on this today and for the same reason: ``Document.title``
    is ``super.presence || "Untitled"`` and ``ScheduleEntry.summary`` is
    ``super.presence || "Untitled"``, so neither can come back blank from a
    healthy server.

    The wrong-type branch is delegated to :func:`writable_string`, so a required
    field and an optional one report a non-string identically.
    """
    value = body.get(key)
    if value is None or (isinstance(value, str) and not value.strip()):
        raise malformed(
            f'{record} field "{key}" is required but the response carried {describe(value)}',
            "The merge-safe update/edit resend this field verbatim, so a missing or blank value "
            f"would blank the current one. Use {escape} to write the record deliberately.",
        )
    return writable_string(body, key, record=record, escape=escape)


def required_writable_boolean(body: dict[str, Any], key: str, *, record: str, escape: str) -> bool:
    """Read a writable boolean the record is *required* to carry.

    The boolean analogue of :func:`required_writable_string`, and it cannot be
    expressed with a truthiness test: the value this guard most needs to admit
    is ``False``, which every ``or``/``if not`` idiom would treat as missing and
    replace with a default. ``ScheduleEntry.all_day`` is ``NOT NULL`` with a
    ``false`` default in BC3 and every partial emits it, so absent or null is a
    malformed response — and defaulting it to ``False`` would silently convert
    an all-day event into a midnight-to-midnight timed one on a call that only
    changed the summary.

    ``0``/``1`` are refused rather than coerced, for the same reason
    :func:`writable_string` refuses ``42``: JSON has a boolean type and the
    server uses it.
    """
    value = body.get(key)
    if value is None:
        raise malformed(
            f'{record} field "{key}" is required but the response carried {describe(value)}',
            "The merge-safe update/edit resend this field verbatim, so a missing value would "
            f"replace the current one with a default. Use {escape} to write the record deliberately.",
        )
    if not isinstance(value, bool):
        raise malformed(
            f"{record} field {key!r} is not a boolean: {describe(value)}",
            _RESEND_HINT.format(escape=escape),
        )
    return value


def writable_boolean(body: dict[str, Any], key: str, *, record: str, escape: str) -> bool:
    """Read an *optional* writable boolean, refusing to coerce a malformed one.

    :func:`writable_string`'s boolean sibling, and it stands in the same
    relation to :func:`required_writable_boolean` that ``writable_string`` does
    to ``required_writable_string``: an absent key or an explicit ``None`` is
    genuinely "not set" and returns ``False``, because that is what the server
    already holds.

    ``ScheduleEntry.highlighted`` is the case it exists for. The entry partial
    emits it unconditionally, but the reduced calendar partial behind
    ``GetUpcomingSchedule`` does not, and both render through the same schema —
    so the member is optional and absence is legitimate rather than malformed.

    What still cannot be tolerated is the *wrong type*: a ``"yes"`` or a ``1``
    must be refused, not coerced, because a caller who assigns the seeded value
    straight back sends whatever it was seeded with. That branch is delegated to
    :func:`required_writable_boolean`, so an optional boolean and a required one
    report a non-boolean identically.
    """
    if body.get(key) is None:
        return False
    return required_writable_boolean(body, key, record=record, escape=escape)


def writable_id_list(body: dict[str, Any], key: str, *, record: str, escape: str) -> list[int]:
    """Read a list of person records and project it to their integer IDs.

    The analogue of :func:`writable_string` for the ID-list fields. The list
    comprehension it replaces (``[p["id"] for p in body.get(key) or []]``) has
    three ways to go wrong on malformed data: a non-list iterates as something
    else (a string yields characters, a dict yields its keys), a non-mapping
    element raises ``TypeError``, and a non-integer ``id`` rides through
    verbatim into the full-replace PUT — the same corruption as a wrong-typed
    string, one level down.

    ``bool`` is excluded explicitly: it subclasses ``int`` in Python, so
    ``isinstance(True, int)`` is true and ``True`` would otherwise pass as a
    person ID.
    """
    value = body.get(key)
    if value is None:
        return []
    if not isinstance(value, list):
        raise malformed(
            f"{record} field {key!r} is not an array: {describe(value)}",
            _RESEND_HINT.format(escape=escape),
        )
    ids: list[int] = []
    for index, element in enumerate(value):
        if not isinstance(element, dict):
            raise malformed(
                f"{record} field {key!r}[{index}] is not an object: {describe(element)}",
                _RESEND_HINT.format(escape=escape),
            )
        person_id = element.get("id")
        if person_id is None:
            raise malformed(
                f"{record} field {key!r}[{index}] has no 'id'",
                _RESEND_HINT.format(escape=escape),
            )
        if isinstance(person_id, bool) or not isinstance(person_id, int):
            raise malformed(
                f"{record} field {key!r}[{index}].id is not an integer: {describe(person_id)}",
                _RESEND_HINT.format(escape=escape),
            )
        ids.append(person_id)
    return ids
