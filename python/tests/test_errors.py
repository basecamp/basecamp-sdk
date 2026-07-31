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


class TestFieldKeyed422:
    def test_flattens_field_errors_into_message(self):
        err = error_from_response(422, b'{"errors": {"color": ["is not a valid color"]}}')
        assert isinstance(err, ValidationError)
        assert str(err) == "color: is not a valid color"
        assert err.field_errors == {"color": ["is not a valid color"]}

    def test_sorts_fields_and_joins_multi_message_fields(self):
        body = b'{"errors": {"name": ["can\'t be blank", "is too short"], "color": ["is not a valid color"]}}'
        err = error_from_response(422, body)
        assert str(err) == "color: is not a valid color, name: can't be blank; is too short"
        assert err.field_errors == {
            "color": ["is not a valid color"],
            "name": ["can't be blank", "is too short"],
        }

    def test_appends_after_top_level_error_message(self):
        body = b'{"error": "Validation failed", "errors": {"color": ["is not a valid color"]}}'
        err = error_from_response(422, body)
        assert str(err) == "Validation failed (color: is not a valid color)"

    def test_extracts_on_400_too(self):
        err = error_from_response(400, b'{"errors": {"color": ["is not a valid color"]}}')
        assert isinstance(err, ValidationError)
        assert str(err) == "color: is not a valid color"
        assert err.field_errors == {"color": ["is not a valid color"]}

    def test_not_extracted_outside_validation_statuses(self):
        err = error_from_response(403, b'{"errors": {"color": ["is not a valid color"]}}')
        assert not hasattr(err, "field_errors")
        assert str(err) == "Access denied"

    def test_skips_malformed_entries(self):
        body = (
            b'{"errors": {"color": "not an array", "name": ["can\'t be blank"],'
            b' "empty": [], "mixed": [42, "is invalid"]}}'
        )
        err = error_from_response(422, body)
        assert str(err) == "mixed: is invalid, name: can't be blank"
        assert err.field_errors == {"mixed": ["is invalid"], "name": ["can't be blank"]}

    @pytest.mark.parametrize("errors", ['{"color": "not an array"}', "[]", '"nope"', "{}"])
    def test_unusable_errors_shape_falls_back(self, errors):
        err = error_from_response(422, f'{{"errors": {errors}}}'.encode())
        assert err.field_errors is None
        assert str(err) == "Validation failed"

    def test_truncates_after_flattening_but_keeps_raw_slot(self):
        long = "x" * 600
        err = error_from_response(422, f'{{"errors": {{"color": ["{long}"]}}}}'.encode())
        assert len(str(err).encode()) == 500
        assert str(err).startswith("color: xxx")
        assert str(err).endswith("...")
        assert err.field_errors == {"color": [long]}

    def test_plain_422_unchanged(self):
        err = error_from_response(422, b'{"error": "Name can\'t be blank"}')
        assert str(err) == "Name can't be blank"
        assert err.field_errors is None

    def test_validation_error_field_errors_default_none(self):
        assert ValidationError("nope").field_errors is None


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
