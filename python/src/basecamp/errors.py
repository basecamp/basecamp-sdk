from __future__ import annotations

import json
from datetime import UTC
from enum import IntEnum, StrEnum
from typing import Any


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


class AmbiguousError(BasecampError):
    def __init__(self, message: str = "Ambiguous match", *, matches: list[Any] | None = None, **kwargs: Any):
        super().__init__(message, code=ErrorCode.AMBIGUOUS, **kwargs)
        self.matches = matches or []


class ValidationError(BasecampError):
    def __init__(self, message: str = "Validation failed", **kwargs: Any):
        super().__init__(message, code=ErrorCode.VALIDATION, **kwargs)


def parse_error_message(body: str | bytes | None) -> str | None:
    """Extract error message from response body."""
    if not body:
        return None
    try:
        data = json.loads(body)
        if isinstance(data, dict):
            return data.get("error") or data.get("message")
    except (json.JSONDecodeError, TypeError):
        pass
    return None


def error_from_response(status: int, body: str | bytes | None, headers: dict[str, str] | None = None) -> BasecampError:
    """Create an appropriate error from an HTTP response."""
    headers = headers or {}
    retry_after = _parse_retry_after(headers.get("Retry-After") or headers.get("retry-after"))
    request_id = headers.get("X-Request-Id") or headers.get("x-request-id")
    message = parse_error_message(body)

    err: BasecampError
    if status == 401:
        err = AuthError(message or "Authentication failed", http_status=401)
    elif status == 403:
        err = ForbiddenError(message or "Access denied", http_status=403)
    elif status == 404:
        err = NotFoundError(message=_truncate(message or "Not found"), http_status=404)
    elif status == 429:
        err = RateLimitError(_truncate(message or "Rate limited"), retry_after=retry_after, http_status=429)
    elif status in (400, 422):
        err = ValidationError(_truncate(message or "Validation failed"), http_status=status)
    elif status == 500:
        err = ApiError("Server error (500)", retryable=True, http_status=500)
    elif status in (502, 503, 504):
        err = ApiError(f"Gateway error ({status})", retryable=True, http_status=status)
    else:
        err = ApiError(_truncate(message or f"Request failed (HTTP {status})"), http_status=status)

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
