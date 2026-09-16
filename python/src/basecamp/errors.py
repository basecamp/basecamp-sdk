from __future__ import annotations

import json
import math
import re
from datetime import UTC, datetime
from enum import IntEnum, StrEnum
from typing import Any

from basecamp.generated.types import TemplateLibraryConfirmationPerson


class ErrorCode(StrEnum):
    USAGE = "usage"
    NOT_FOUND = "not_found"
    AUTH = "auth_required"
    FORBIDDEN = "forbidden"
    RATE_LIMIT = "rate_limit"
    NETWORK = "network"
    API = "api_error"
    AMBIGUOUS = "ambiguous"
    VALIDATION = "validation"
    LIMIT_EXCEEDED = "limit_exceeded"


class ExitCode(IntEnum):
    USAGE = 1
    NOT_FOUND = 2
    AUTH = 3
    FORBIDDEN = 4
    RATE_LIMIT = 5
    NETWORK = 6
    API = 7
    AMBIGUOUS = 8
    VALIDATION = 9
    LIMIT_EXCEEDED = 10


_EXIT_CODE_MAP = {
    ErrorCode.USAGE: ExitCode.USAGE,
    ErrorCode.NOT_FOUND: ExitCode.NOT_FOUND,
    ErrorCode.AUTH: ExitCode.AUTH,
    ErrorCode.FORBIDDEN: ExitCode.FORBIDDEN,
    ErrorCode.RATE_LIMIT: ExitCode.RATE_LIMIT,
    ErrorCode.NETWORK: ExitCode.NETWORK,
    ErrorCode.API: ExitCode.API,
    ErrorCode.AMBIGUOUS: ExitCode.AMBIGUOUS,
    ErrorCode.VALIDATION: ExitCode.VALIDATION,
    ErrorCode.LIMIT_EXCEEDED: ExitCode.LIMIT_EXCEEDED,
}


class BasecampError(Exception):
    """Base error class for all Basecamp SDK errors."""

    def __init__(
        self,
        message: str,
        *,
        code: str = ErrorCode.API,
        hint: str | None = None,
        http_status: int | None = None,
        retryable: bool = False,
        retry_after: int | None = None,
        request_id: str | None = None,
    ):
        super().__init__(message)
        self.code = code
        self.hint = hint
        self.http_status = http_status
        self.retryable = retryable
        self.retry_after = retry_after
        self.request_id = request_id

    @property
    def cause(self) -> BaseException | None:
        """The failure this error was raised from, or ``None``.

        The slot the other five SDKs spell ``Cause``/``cause``/``decodeFailure``,
        and the reason it is a property rather than a constructor keyword: the
        refusal sites that have an underlying failure — the page-body decode in
        ``_base``/``_async_base`` — are all inside an ``except`` block and all
        ``raise ... from e``, which is Python's own way of recording it. A
        keyword would be a second place to set the same fact, free to disagree
        with ``__cause__`` and easy to forget at the next site. What was missing
        was the NAME: a caller reading a malformed-body refusal had to know to
        reach for a dunder to get what every other SDK hands over (#750).

        Reading the decoder's own error, rather than the message it was
        interpolated into, is the point. The message says which page failed; the
        exception says whether the body was truncated mid-object or was never
        JSON, and it is not parsed back out of a sentence.
        """
        return self.__cause__

    @property
    def exit_code(self) -> int:
        try:
            return _EXIT_CODE_MAP.get(ErrorCode(self.code), ExitCode.API)
        except ValueError:
            return ExitCode.API


class UsageError(BasecampError):
    def __init__(self, message: str, **kwargs: Any):
        super().__init__(message, code=ErrorCode.USAGE, **kwargs)


class NotFoundError(BasecampError):
    def __init__(self, message: str = "Not found", **kwargs: Any):
        super().__init__(message, code=ErrorCode.NOT_FOUND, **kwargs)


class AuthError(BasecampError):
    def __init__(self, message: str = "Authentication failed", **kwargs: Any):
        super().__init__(message, code=ErrorCode.AUTH, **kwargs)


class ForbiddenError(BasecampError):
    def __init__(self, message: str = "Access denied", **kwargs: Any):
        super().__init__(message, code=ErrorCode.FORBIDDEN, **kwargs)


class RateLimitError(BasecampError):
    def __init__(self, message: str = "Rate limited", *, retry_after: int | None = None, **kwargs: Any):
        super().__init__(message, code=ErrorCode.RATE_LIMIT, retryable=True, retry_after=retry_after, **kwargs)


class NetworkError(BasecampError):
    def __init__(self, message: str = "Connection failed", **kwargs: Any):
        super().__init__(message, code=ErrorCode.NETWORK, retryable=True, **kwargs)


class ApiError(BasecampError):
    def __init__(self, message: str = "API error", *, retryable: bool = False, **kwargs: Any):
        super().__init__(message, code=ErrorCode.API, retryable=retryable, **kwargs)


class LimitExceededError(BasecampError):
    """An account limit blocks the request (HTTP 507).

    File storage exhausted, or a webhook ceiling reached. Never retryable: no
    amount of backoff frees storage or raises a plan limit. That is why this is
    not an ApiError, which a 507 would otherwise become via the 5xx catch-all.
    """

    def __init__(self, message: str = "Account limit reached", **kwargs: Any):
        super().__init__(message, code=ErrorCode.LIMIT_EXCEEDED, retryable=False, **kwargs)


class AmbiguousError(BasecampError):
    def __init__(self, message: str = "Ambiguous match", *, matches: list[Any] | None = None, **kwargs: Any):
        super().__init__(message, code=ErrorCode.AMBIGUOUS, **kwargs)
        self.matches = matches or []


class ValidationError(BasecampError):
    def __init__(
        self,
        message: str = "Validation failed",
        *,
        field_errors: dict[str, list[str]] | None = None,
        **kwargs: Any,
    ):
        super().__init__(message, code=ErrorCode.VALIDATION, **kwargs)
        #: Field-keyed validation messages from a 400/422 body — either
        #: ``{"errors": {"field": ["msg", ...]}}``, the Rails RecordInvalid
        #: rendering, or the same map with no wrapper at all
        #: (``{"field": ["msg", ...]}``), which some controllers emit. ``None``
        #: for every other error shape. The flattened form
        #: is also folded into the message; this slot preserves the raw,
        #: untruncated per-field messages.
        self.field_errors = field_errors


class PeopleConfirmationRequiredError(ValidationError):
    """A template copy requires confirmation before granting project access."""

    def __init__(
        self,
        message: str,
        *,
        people: list[TemplateLibraryConfirmationPerson],
        **kwargs: Any,
    ):
        super().__init__(message, **kwargs)
        self.people = people


# --- Recording summary identities -------------------------------------------
#
# The errors ``RecordingsService.summarize`` raises for its own refusals, as
# opposed to a read's. Identity and CLASSIFICATION are separate here, following
# ``DeviceFlowError`` in ``oauth/errors.py``: that carries the precise outcome
# in ``reason`` and DERIVES the coarse ``code`` from it, overriding retryability
# from the reason rather than from the code. The same split applies, with the
# exception CLASS as the identity a consumer matches -- the shape Go uses too,
# where these are sentinel errors with no code slot at all and its conformance
# runner matches them with ``errors.Is`` before falling through to ``.Code``.
#
# ``code`` therefore stays inside ``ErrorCode``. SPEC section 6 declares that
# enum CLOSED, so a name outside it is a spec violation rather than an
# extension -- and ``exit_code`` maps an unknown code to ``ExitCode.API``, so
# every one of these reported "server-side error" (7), including the two that
# are refused from the caller's own arguments before any request is made.
#: The canonical SPEC section 6 code each composite identity classifies as.
#: Identity is the class; this is the coarse answer a CLI turns into an exit
#: code. Derived, never stored, so the two cannot drift apart.
_COMPOSITE_CODE: dict[str, ErrorCode] = {
    # Refused from the caller's own arguments, before any request.
    "no_recording_type": ErrorCode.USAGE,
    "unknown_recording_type": ErrorCode.USAGE,
    # The pointer names a bucket the recording is not in -- also the caller's.
    # Settled across every port on card 41 after Rust shipped `not_found` here:
    # the read FOUND the recording, in another bucket, and returned it, so
    # nothing is absent and `not_found` would be a false claim.
    # https://app.basecamp.com/2914079/buckets/48699913/card_tables/cards/10308966794
    "bucket_mismatch": ErrorCode.USAGE,
    # Every visible candidate answered 404: the recording is not there.
    "recording_unresolved": ErrorCode.NOT_FOUND,
    # Settled across every port on card 40 after Python and Kotlin shipped
    # different answers: `usage`, non-retryable.
    # https://app.basecamp.com/2914079/buckets/48699913/card_tables/cards/10308122086
    #
    # `usage` is one of only THREE coarse codes no HTTP response can produce:
    # the status mapping yields `auth_required`, `forbidden`, `not_found`,
    # `rate_limit`, `validation`, `limit_exceeded` and `api_error`, and
    # `network` and `ambiguous` are equally unreachable from a status. `usage`
    # is the one of those three that ALSO describes a call the SDK declined to
    # complete, which is why it and not the other two. A verdict the composite
    # reached on its own therefore can never be read back as a constituent
    # read's own answer, which is the property the composite exists to protect.
    # This port previously took the residual `api_error`, which a caller could
    # not tell from a 500 one of those reads returned.
    #
    # Retryability is a SEPARATE field and keeps the answer this port already
    # had: NOT retryable, because both reasons -- too many visible campfires,
    # and a budget spent before the listing -- are deterministic for the same
    # account state, so a retry loop would re-run the same search forever.
    "campfire_discovery_incomplete": ErrorCode.USAGE,
    # The sixth identity, and the one the first pass of this table missed: it
    # is named in the same SPEC row and sits in the same section, so leaving it
    # out left one public error still carrying a name outside the enum and
    # still reaching exit 7 through the `ValueError` fall-through this table
    # exists to close. Retryable, unlike its neighbours -- the load it was
    # waiting on was abandoned, and the next caller loads again.
    "campfire_index_load_aborted": ErrorCode.API,
}


class RecordingRoutingError(BasecampError):
    """A recording pointer ``summarize`` cannot route, refused before any request.

    The base of the two routing refusals, so ``except RecordingRoutingError``
    catches both without naming either.
    """


class NoRecordingTypeError(RecordingRoutingError):
    """An event type that names no recording type.

    ``boost.created``, whose recording is the boost's target and whose type the
    feed row does not carry. A consumer resolves those from its own record of
    what it posted, not through ``summarize``.
    """

    def __init__(self, routing_key: str, **kwargs: Any):
        super().__init__(
            f"event type names no recording type: {routing_key!r}",
            code=_COMPOSITE_CODE["no_recording_type"],
            **kwargs,
        )
        self.routing_key = routing_key


class UnknownRecordingTypeError(RecordingRoutingError):
    """Neither the event type nor the recording type names a routed read."""

    def __init__(self, routing_key: str, **kwargs: Any):
        super().__init__(
            f"no typed read for recording type: {routing_key!r}",
            code=_COMPOSITE_CODE["unknown_recording_type"],
            **kwargs,
        )
        self.routing_key = routing_key


class RecordingUnresolvedError(BasecampError):
    """A chat line found under none of the Campfires the caller can currently see.

    Distinct from a failed read (any non-404 answer is raised as itself) and
    from :class:`CampfireDiscoveryIncompleteError` (candidates were left
    unsearched): every candidate answered 404. It is NOT distinct from lost
    visibility -- BC3 answers 404 for a Campfire the caller may not see, too --
    so a consumer marks the record blocked and retries on its own schedule;
    ``stale_campfire_ids`` says when visibility, rather than existence, is what
    changed.
    """

    def __init__(
        self,
        *,
        bucket_id: int,
        recording_id: int,
        campfire_ids: list[int],
        refreshed: bool = False,
        stale_campfire_ids: list[int] | None = None,
        **kwargs: Any,
    ):
        super().__init__(
            f"chat line found under no visible campfire: line {recording_id} "
            f"in bucket {bucket_id} (tried {len(campfire_ids)} campfires)",
            code=_COMPOSITE_CODE["recording_unresolved"],
            **kwargs,
        )
        self.bucket_id = bucket_id
        self.recording_id = recording_id
        #: The candidates tried, in order; empty when the bucket has no visible
        #: Campfire at all.
        self.campfire_ids = campfire_ids
        #: Whether the cached discovery sources were re-read before concluding.
        #: False when every source had been read within the refresh floor, so a
        #: Campfire created in that window was not seen: the conclusion stands
        #: on data up to that old, and a retry after the floor sees the current
        #: sources.
        self.refreshed = refreshed
        #: Candidates from the cache that the refreshed sources no longer list
        #: -- Campfires the caller could see when the cache filled and cannot
        #: now. Non-empty only when ``refreshed``.
        self.stale_campfire_ids = stale_campfire_ids or []


class CampfireDiscoveryIncompleteError(BasecampError):
    """A chat line's Campfire discovery could not be carried to a conclusion.

    The Campfire listing overflowed its cap, or a bucket has more visible
    Campfires than one call may try. Distinct from
    :class:`RecordingUnresolvedError`: candidates were left unsearched, so
    nothing can be reported absent.
    """

    def __init__(self, *, bucket_id: int, recording_id: int, reason: str, **kwargs: Any):
        # Overwrite -- never setdefault -- so a caller's `retryable` kwarg
        # cannot flip the invariant, exactly as `DeviceFlowError` does it. The
        # coarse code is `usage` (see `_COMPOSITE_CODE`), whose default is
        # already non-retryable; forcing it here is what keeps that true of the
        # IDENTITY rather than of whichever code it currently derives, and it
        # is what the argument on card 40 actually rests on -- both reasons are
        # deterministic for the same account state, so a retry loop would
        # re-run the identical search forever. Claiming that override in a
        # commit message without writing it here is how the invariant would
        # have been lost at the first caller who passed the kwarg through.
        kwargs["retryable"] = False
        super().__init__(
            f"campfire discovery incomplete: line {recording_id} in bucket {bucket_id}: {reason}",
            code=_COMPOSITE_CODE["campfire_discovery_incomplete"],
            **kwargs,
        )
        self.bucket_id = bucket_id
        self.recording_id = recording_id
        self.reason = reason


class CampfireIndexLoadAbortedError(BasecampError):
    """A Campfire discovery load this call waited on was abandoned by its owner.

    Loads are single-flight: the first caller for a key fetches and the rest
    wait on it. When that caller's own task or thread exits -- cancelled,
    interrupted -- the load ends without having learned anything about the
    source, and the failure belongs to the caller that left, not to the ones
    still waiting. Re-raising its ``CancelledError`` (or ``KeyboardInterrupt``)
    into them would tell tasks nobody cancelled that they were, which an
    enclosing ``TaskGroup`` then treats as an orderly exit with the work
    silently missing.

    Retryable, and meant to be retried: nothing was learned, so the next
    attempt loads for itself.
    """

    def __init__(self, message: str = "campfire index load was abandoned by the caller that owned it", **kwargs: Any):
        # Overwrite rather than pass positionally, for the same reason its
        # sibling does: forwarding `**kwargs` alongside a fixed `retryable=`
        # meant `CampfireIndexLoadAbortedError(retryable=False)` raised a bare
        # TypeError about duplicate keyword arguments instead of an error from
        # this SDK's own taxonomy. The invariant is the same either way -- the
        # load was abandoned, so the next caller loads again -- but a caller
        # passing the flag now gets the invariant, not a crash.
        kwargs["retryable"] = True
        super().__init__(message, code=_COMPOSITE_CODE["campfire_index_load_aborted"], **kwargs)


class BucketMismatchError(BasecampError):
    """The recording a read returned lives in a different bucket from the pointer's."""

    def __init__(self, *, bucket_id: int, recording_id: int, requested_bucket_id: int, **kwargs: Any):
        super().__init__(
            f"recording is not in the requested bucket: recording {recording_id} "
            f"is in bucket {bucket_id}, not {requested_bucket_id}",
            code=_COMPOSITE_CODE["bucket_mismatch"],
            **kwargs,
        )
        self.bucket_id = bucket_id
        self.recording_id = recording_id
        self.requested_bucket_id = requested_bucket_id


def _error_body_object(body: str | bytes | None) -> dict[str, Any] | None:
    """The response body as a JSON object, or ``None`` when it is not one."""
    if not body:
        return None
    try:
        data = json.loads(body)
    except (json.JSONDecodeError, TypeError):
        return None
    return data if isinstance(data, dict) else None


def _parse_template_library_confirmation_people(
    data: dict[str, Any] | None,
) -> list[TemplateLibraryConfirmationPerson] | None:
    if data is None or not isinstance(data.get("people"), list) or not data["people"]:
        return None

    people: list[TemplateLibraryConfirmationPerson] = []
    for value in data["people"]:
        if not isinstance(value, dict):
            return None
        person_id = value.get("id")
        name = value.get("name")
        avatar_url = value.get("avatar_url")
        if (
            not isinstance(person_id, int)
            or isinstance(person_id, bool)
            or person_id <= 0
            or not isinstance(name, str)
            or not name
            or not isinstance(avatar_url, str)
            or not avatar_url
        ):
            return None
        people.append({"id": person_id, "name": name, "avatar_url": avatar_url})
    return people


def _message_from(data: dict[str, Any] | None) -> str | None:
    if data is None:
        return None
    for key in ("error", "message"):
        if isinstance(data.get(key), str) and data[key]:
            return data[key]
    return None


def _hint_from(data: dict[str, Any] | None) -> str | None:
    if data is None:
        return None
    hint = data.get("error_description")
    return hint if isinstance(hint, str) and hint else None


def parse_error_message(body: str | bytes | None) -> str | None:
    """Extract error message from response body.

    A key is used only when its value is a string (SPEC section 6), so a
    malformed scalar member cannot leak a non-string into the message or
    prevent field-keyed extraction.
    """
    return _message_from(_error_body_object(body))


def parse_error_hint(body: str | bytes | None) -> str | None:
    """Extract the SPEC section 6 step-3 hint from a response body.

    The ``error_description`` key, used only when its value is a non-empty
    string. Callers truncate it like the message.
    """
    return _hint_from(_error_body_object(body))


def parse_field_errors(body: str | bytes | None) -> dict[str, list[str]] | None:
    """Extract the field-keyed validation errors map from a response body.

    Recognizes the Rails RecordInvalid rendering
    ``{"errors": {"field": ["msg", ...]}}``. Entries whose value is not a list
    are skipped, non-string elements are dropped, and a map with no usable
    entries is treated as absent (``None``).
    """
    if not body:
        return None
    try:
        data = json.loads(body)
    except (json.JSONDecodeError, TypeError):
        return None
    if not isinstance(data, dict):
        return None
    if isinstance(data.get("errors"), list):
        return _parse_row_errors(data["errors"])
    if not isinstance(data.get("errors"), dict):
        return _parse_bare_field_errors(data)
    field_errors: dict[str, list[str]] = {}
    for field, values in data["errors"].items():
        if not isinstance(values, list):
            continue
        messages = [m for m in values if isinstance(m, str)]
        if messages:
            field_errors[str(field)] = messages
    return field_errors or None


def _parse_row_errors(rows: list[object]) -> dict[str, list[str]] | None:
    """Extract a row-keyed errors list -- ``{"errors": [{"email_address": ..., "messages": [...]}]}``.

    This is the batch-invite rendering (SPEC section 6 step 1b), one element per
    rejected row. Rows are keyed by their ``email_address`` when it is a
    non-empty string, else by an integer ``index``, else by their position in
    the list; a repeated key appends. All-or-nothing: one element without a
    usable ``messages`` array means this is some other list, and the slot stays
    absent.
    """
    if not rows:
        return None
    field_errors: dict[str, list[str]] = {}
    for position, row in enumerate(rows):
        if not isinstance(row, dict) or not isinstance(row.get("messages"), list):
            return None
        messages = [m for m in row["messages"] if isinstance(m, str) and m]
        if not messages:
            return None
        email_address = row.get("email_address")
        index = row.get("index")
        if isinstance(email_address, str) and email_address:
            key = email_address
        elif isinstance(index, int) and not isinstance(index, bool):
            key = str(index)
        else:
            key = str(position)
        field_errors.setdefault(key, []).extend(messages)
    return field_errors


def _parse_bare_field_errors(data: dict[str, object]) -> dict[str, list[str]] | None:
    """Extract an unwrapped field map -- the whole body is ``{"field": ["msg"]}``.

    This is the ``render json: @webhook.errors`` rendering. The gate is
    all-or-nothing by design (SPEC section 6 step 2): with no ``errors`` key to
    declare intent, only shape distinguishes a field map from any other JSON
    object, so a single non-conforming member means this is not one.
    """
    # Only "errors" is structurally reserved (it belongs to the wrapped path).
    # "error" and "message" are not excluded by name: a flat body carries them
    # as strings, which the shape gate below already rejects.
    if not data or "errors" in data:
        return None
    field_errors: dict[str, list[str]] = {}
    for field, values in data.items():
        if not isinstance(values, list) or not values:
            return None
        if not all(isinstance(m, str) and m for m in values):
            return None
        field_errors[str(field)] = list(values)  # type: ignore[arg-type]
    return field_errors


def _flatten_field_errors(field_errors: dict[str, list[str]]) -> str:
    """Render a field-keyed errors map as "field: msg1; msg2, other: msg".

    Fields sorted lexicographically, a field's messages joined with "; ",
    fields joined with ", ". This shape is shared by all six SDKs; change it
    everywhere or nowhere.
    """
    return ", ".join(f"{field}: {'; '.join(field_errors[field])}" for field in sorted(field_errors))


def error_from_response(status: int, body: str | bytes | None, headers: dict[str, str] | None = None) -> BasecampError:
    """Create an appropriate error from an HTTP response."""
    headers = headers or {}
    retry_after = _parse_retry_after(headers.get("Retry-After") or headers.get("retry-after"))
    request_id = headers.get("X-Request-Id") or headers.get("x-request-id")
    # One parse serves both the message and the SPEC section 6 step-3 hint
    # (error_description); both are capped per section 9.
    data = _error_body_object(body)
    message = _message_from(data)
    hint = _hint_from(data)
    if hint:
        hint = _truncate(hint)

    err: BasecampError
    if status == 401:
        err = AuthError(_truncate(message or "Authentication failed"), http_status=401, hint=hint)
    elif status == 403:
        err = ForbiddenError(_truncate(message or "Access denied"), http_status=403, hint=hint)
    elif status == 404:
        err = NotFoundError(message=_truncate(message or "Not found"), http_status=404, hint=hint)
    elif status == 429:
        err = RateLimitError(_truncate(message or "Rate limited"), retry_after=retry_after, http_status=429, hint=hint)
    elif status in (400, 422):
        field_errors = parse_field_errors(body)
        if field_errors:
            flat = _flatten_field_errors(field_errors)
            # Appended in parentheses after a top-level message, standing alone
            # otherwise; truncated after flattening so the tail is capped too.
            message = f"{message} ({flat})" if message else flat
        confirmation_people = _parse_template_library_confirmation_people(data) if status == 422 else None
        validation_type = PeopleConfirmationRequiredError if confirmation_people else ValidationError
        validation_kwargs: dict[str, Any] = {
            "http_status": status,
            "field_errors": field_errors,
            "hint": hint,
        }
        if confirmation_people:
            validation_kwargs["people"] = confirmation_people
        err = validation_type(_truncate(message or "Validation failed"), **validation_kwargs)
    elif status == 507:
        # A 5xx status carrying a client fact: the account is out of storage, or
        # at its webhook ceiling. Retrying cannot satisfy it, so this is decided
        # before the 5xx arms below.
        err = LimitExceededError(_truncate(message or "Account limit reached"), http_status=507, hint=hint)
    elif status == 500:
        err = ApiError("Server error (500)", retryable=True, http_status=500, hint=hint)
    elif status in (502, 503, 504):
        err = ApiError(f"Gateway error ({status})", retryable=True, http_status=status, hint=hint)
    else:
        # SPEC section 6 step 12: any other 5xx is retryable; the 507 arm
        # above is the deliberate exception.
        err = ApiError(
            _truncate(message or f"Request failed (HTTP {status})"),
            retryable=status >= 500,
            http_status=status,
            hint=hint,
        )

    err.request_id = request_id
    err.retry_after = err.retry_after or retry_after
    return err


# SPEC section 6 MAX_RETRY_AFTER_SECONDS: the value a parsed Retry-After
# saturates at, in both wire forms. A representability bound pinned once for
# all six SDKs (the narrowest retry_after integer any of them ships, and the
# ceiling section 16 already names), not a policy cap. Python's int is
# arbitrary-precision, so without it the failure was one layer down: float()
# raised OverflowError on the retry path.
MAX_RETRY_AFTER_SECONDS = 2_147_483_647
_DELAY_SECONDS = re.compile(r"[0-9]+")
_MAX_RETRY_AFTER_DIGITS = len(str(MAX_RETRY_AFTER_SECONDS))
# RFC 7231's three HTTP-date shapes, gated BEFORE parsing: parsedate_to_datetime
# is RFC 5322's parser and reads far more than these — a bare date, a numeric
# zone, an unknown zone name — and SPEC section 6's table says anything outside
# the three forms is not a date at all. IMF-fixdate and RFC 850 end in GMT and
# come back aware; asctime carries no zone, comes back naive, and is read as
# UTC (`day-name SP month SP ( 2DIGIT / SP 1DIGIT ) SP time-of-day SP year`).
_HTTP_DATE = re.compile(
    r"[A-Z][a-z]{2}, [0-9]{2} [A-Z][a-z]{2} [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} GMT"
    r"|[A-Z][a-z]+, [0-9]{2}-[A-Z][a-z]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2} GMT"
    r"|[A-Z][a-z]{2} [A-Z][a-z]{2} (?:[0-9]{2}| [0-9]) [0-9]{2}:[0-9]{2}:[0-9]{2} [0-9]{4}"
)


def _parse_retry_after(value: str | None, *, now: datetime | None = None) -> int | None:
    """SPEC section 6 "Retry-After Parsing Algorithm".

    ``now`` is a seam for tests: the HTTP-date branch is one second wide at its
    boundary, so its rounding is only pinnable against a frozen clock.
    """
    if not value:
        return None
    # RFC 9110 spells delay-seconds as 1*DIGIT: no sign, which int() would
    # accept. A value over the ceiling saturates whatever its width, since
    # 1*DIGIT has no upper bound and no digit string is malformed for its
    # length.
    if _DELAY_SECONDS.fullmatch(value):
        # Width before conversion: int() itself refuses a string past the
        # interpreter's digit limit (4300 by default), and a ValueError here
        # would be the exception this branch exists to keep off the retry path.
        digits = value.lstrip("0")
        if len(digits) > _MAX_RETRY_AFTER_DIGITS:
            return MAX_RETRY_AFTER_SECONDS
        seconds = int(digits or "0")
        return min(seconds, MAX_RETRY_AFTER_SECONDS) if seconds > 0 else None
    # Try HTTP-date: one of the three shapes, or nothing.
    if not _HTTP_DATE.fullmatch(value):
        return None
    from email.utils import parsedate_to_datetime

    try:
        date = parsedate_to_datetime(value)
        # asctime carries no zone and comes back naive; RFC 7231 reads every
        # HTTP-date as UTC.
        if date.tzinfo is None:
            date = date.replace(tzinfo=UTC)
        # Rounded UP (SPEC section 6 step 2): truncating a sub-second remainder
        # toward zero turned a date 400ms out into 0, which reads as "no usable
        # value" and drops onto the backoff curve, and retried up to a second
        # before the moment the server named.
        diff = math.ceil((date - (now or datetime.now(UTC))).total_seconds())
        return min(diff, MAX_RETRY_AFTER_SECONDS) if diff > 0 else None
    except (ValueError, TypeError):
        pass
    return None


def _truncate(s: str, max_bytes: int = 500) -> str:
    if len(s.encode()) <= max_bytes:
        return s
    if max_bytes <= 3:
        return s.encode()[:max_bytes].decode(errors="ignore")
    return s.encode()[: max_bytes - 3].decode(errors="ignore") + "..."
