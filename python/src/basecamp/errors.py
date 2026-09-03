from __future__ import annotations

import json
from datetime import UTC
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


def _parse_retry_after(value: str | None) -> int | None:
    if not value:
        return None
    try:
        seconds = int(value)
        return seconds if seconds > 0 else None
    except ValueError:
        pass
    # Try HTTP-date
    from datetime import datetime
    from email.utils import parsedate_to_datetime

    try:
        date = parsedate_to_datetime(value)
        diff = int((date - datetime.now(UTC)).total_seconds())
        return diff if diff > 0 else None
    except (ValueError, TypeError):
        pass
    return None


def _truncate(s: str, max_bytes: int = 500) -> str:
    if len(s.encode()) <= max_bytes:
        return s
    if max_bytes <= 3:
        return s.encode()[:max_bytes].decode(errors="ignore")
    return s.encode()[: max_bytes - 3].decode(errors="ignore") + "..."
