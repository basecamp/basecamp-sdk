from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest

from basecamp.errors import (
    AmbiguousError,
    ApiError,
    AuthError,
    BasecampError,
    ErrorCode,
    ExitCode,
    ForbiddenError,
    NetworkError,
    NotFoundError,
    RateLimitError,
    UsageError,
    ValidationError,
    _parse_retry_after,
    error_from_response,
    parse_error_message,
)


class TestErrorHierarchy:
    @pytest.mark.parametrize(
        "cls,code,exit_code",
        [
            (UsageError, ErrorCode.USAGE, ExitCode.USAGE),
            (NotFoundError, ErrorCode.NOT_FOUND, ExitCode.NOT_FOUND),
            (AuthError, ErrorCode.AUTH, ExitCode.AUTH),
            (ForbiddenError, ErrorCode.FORBIDDEN, ExitCode.FORBIDDEN),
            (RateLimitError, ErrorCode.RATE_LIMIT, ExitCode.RATE_LIMIT),
            (NetworkError, ErrorCode.NETWORK, ExitCode.NETWORK),
            (ApiError, ErrorCode.API, ExitCode.API),
            (AmbiguousError, ErrorCode.AMBIGUOUS, ExitCode.AMBIGUOUS),
            (ValidationError, ErrorCode.VALIDATION, ExitCode.VALIDATION),
        ],
    )
    def test_code_and_exit_code(self, cls, code, exit_code):
        err = cls("test")
        assert err.code == code
        assert err.exit_code == exit_code
        assert isinstance(err, BasecampError)

    def test_rate_limit_is_retryable(self):
        err = RateLimitError()
        assert err.retryable is True

    def test_network_error_is_retryable(self):
        err = NetworkError()
        assert err.retryable is True

    def test_api_error_default_not_retryable(self):
        err = ApiError()
        assert err.retryable is False

    def test_api_error_retryable_when_set(self):
        err = ApiError(retryable=True)
        assert err.retryable is True

    def test_ambiguous_error_stores_matches(self):
        err = AmbiguousError(matches=[1, 2, 3])
        assert err.matches == [1, 2, 3]


class TestErrorFromResponse:
    def test_401_auth_error(self):
        err = error_from_response(401, None)
        assert isinstance(err, AuthError)
        assert err.http_status == 401

    def test_403_forbidden(self):
        err = error_from_response(403, None)
        assert isinstance(err, ForbiddenError)
        assert err.http_status == 403

    def test_404_not_found(self):
        err = error_from_response(404, None)
        assert isinstance(err, NotFoundError)
        assert err.http_status == 404

    def test_429_rate_limit(self):
        err = error_from_response(429, None, {"Retry-After": "5"})
        assert isinstance(err, RateLimitError)
        assert err.http_status == 429
        assert err.retry_after == 5

    def test_422_validation(self):
        err = error_from_response(422, b'{"error": "invalid"}')
        assert isinstance(err, ValidationError)
        assert err.http_status == 422

    def test_400_validation(self):
        err = error_from_response(400, None)
        assert isinstance(err, ValidationError)
        assert err.http_status == 400

    def test_500_retryable(self):
        err = error_from_response(500, None)
        assert isinstance(err, ApiError)
        assert err.retryable is True
        assert err.http_status == 500

    @pytest.mark.parametrize("status", [502, 503, 504])
    def test_gateway_errors_retryable(self, status):
        err = error_from_response(status, None)
        assert isinstance(err, ApiError)
        assert err.retryable is True
        assert err.http_status == status

    def test_request_id_extracted(self):
        err = error_from_response(500, None, {"X-Request-Id": "abc-123"})
        assert err.request_id == "abc-123"

    def test_json_error_message_extracted(self):
        err = error_from_response(422, b'{"error": "Name is required"}')
        assert "Name is required" in str(err)


class TestParseErrorMessage:
    def test_json_error_field(self):
        assert parse_error_message(b'{"error": "bad"}') == "bad"

    def test_json_message_field(self):
        assert parse_error_message(b'{"message": "oops"}') == "oops"

    def test_empty_body(self):
        assert parse_error_message(None) is None
        assert parse_error_message(b"") is None

    def test_invalid_json(self):
        assert parse_error_message(b"not json") is None


class TestParseRetryAfter:
    def test_integer(self):
        assert _parse_retry_after("10") == 10

    def test_zero_returns_none(self):
        assert _parse_retry_after("0") is None

    def test_negative_returns_none(self):
        assert _parse_retry_after("-5") is None

    def test_none(self):
        assert _parse_retry_after(None) is None

    def test_http_date_in_future(self):
        from email.utils import format_datetime

        future = datetime.now(UTC) + timedelta(seconds=30)
        value = format_datetime(future)
        result = _parse_retry_after(value)
        assert result is not None
        assert 25 <= result <= 35  # allow some clock drift

    def test_http_date_in_past_returns_none(self):
        from email.utils import format_datetime

        past = datetime.now(UTC) - timedelta(seconds=30)
        value = format_datetime(past)
        assert _parse_retry_after(value) is None
