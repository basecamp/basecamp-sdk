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

    # SPEC section 6 step 3: a body's error_description becomes the hint,
    # truncated like the message.
    def test_error_description_becomes_hint(self):
        err = error_from_response(403, b'{"error": "denied", "error_description": "You need the admin scope"}')
        assert err.hint == "You need the admin scope"

    def test_error_description_truncated(self):
        long = "x" * 600
        err = error_from_response(403, ('{"error": "denied", "error_description": "' + long + '"}').encode())
        assert err.hint is not None
        assert len(err.hint.encode()) == 500
        assert err.hint.endswith("...")

    def test_non_string_error_description_ignored(self):
        err = error_from_response(403, b'{"error": "denied", "error_description": {"nested": true}}')
        assert err.hint is None

    # SPEC section 6 step 5: an empty body on an unmapped status renders the
    # fixed code-bearing phrase — 599 has no registered reason phrase at all.
    def test_599_empty_body_renders_fixed_phrase(self):
        err = error_from_response(599, b"")
        assert isinstance(err, ApiError)
        assert str(err) == "Request failed (HTTP 599)"


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

    def test_survives_non_string_error_sibling(self):
        body = b'{"error": {"base": 1}, "errors": {"color": ["is not a valid color"]}}'
        err = error_from_response(422, body)
        assert str(err) == "color: is not a valid color"
        assert err.field_errors == {"color": ["is not a valid color"]}

    def test_non_string_error_does_not_crash_other_statuses(self):
        err = error_from_response(404, b'{"error": {"base": 1}}')
        assert str(err) == "Not found"

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


class TestBareFieldMap:
    """SPEC section 6 step 2.

    webhooks_controller and chats/integrations_controller render
    ``json: @webhook.errors`` at 400, lineup markers at 422 -- the field map
    arrives as the whole body, with no ``errors`` wrapper.
    """

    @pytest.mark.parametrize(
        ("status", "body", "message", "field_errors"),
        [
            (
                400,
                b'{"payload_url": ["is not a valid URL"]}',
                "payload_url: is not a valid URL",
                {"payload_url": ["is not a valid URL"]},
            ),
            (
                400,
                b'{"types": ["is invalid"], "payload_url": ["is not a valid URL", "is too long"]}',
                "payload_url: is not a valid URL; is too long, types: is invalid",
                {
                    "payload_url": ["is not a valid URL", "is too long"],
                    "types": ["is invalid"],
                },
            ),
            (
                422,
                b'{"name": ["can\'t be blank"]}',
                "name: can't be blank",
                {"name": ["can't be blank"]},
            ),
        ],
    )
    def test_flattens_bare_field_map(self, status, body, message, field_errors):
        err = error_from_response(status, body)
        assert isinstance(err, ValidationError)
        assert str(err) == message
        assert err.field_errors == field_errors

    @pytest.mark.parametrize(
        "body",
        [
            b'{"id": 1}',
            b'{"color": ["is invalid"], "count": 3}',
            b'{"color": []}',
            b'{"color": ["", "is invalid"]}',
            b'{"color": ["is invalid", 42]}',
            b'{"color": [null]}',
            b"{}",
            b"[1, 2]",
            b'"nope"',
        ],
    )
    def test_strict_gate_rejects_non_field_maps(self, body):
        err = error_from_response(400, body)
        assert err.field_errors is None
        assert str(err) == "Validation failed"

    @pytest.mark.parametrize(
        ("body", "message"),
        [
            (b'{"error": "Webhook is invalid", "payload_url": ["is bad"]}', "Webhook is invalid"),
            (b'{"message": "Webhook is invalid", "payload_url": ["is bad"]}', "Webhook is invalid"),
            (b'{"errors": {}, "payload_url": ["is bad"]}', "Validation failed"),
        ],
    )
    def test_stays_flat_for_flat_bodies(self, body, message):
        # Only "errors" is excluded by name; a flat body's "error"/"message" is
        # a str, and the shape gate rejects a str-valued member — so these
        # bodies stay flat on shape, not on the key's name. The test above
        # covers the other half: list-valued keys ARE recognized as fields.
        err = error_from_response(400, body)
        assert err.field_errors is None
        assert str(err) == message

    # Only "errors" is reserved by name. A record whose validated attribute is
    # called "message" or "error" still gets its field map recognized: the flat
    # shape carries those keys as strings, which the gate rejects on shape alone.
    @pytest.mark.parametrize(
        ("body", "message", "field_errors"),
        [
            (
                b'{"message": ["can\'t be blank"]}',
                "message: can't be blank",
                {"message": ["can't be blank"]},
            ),
            (
                b'{"error": ["is invalid"], "name": ["can\'t be blank"]}',
                "error: is invalid, name: can't be blank",
                {"error": ["is invalid"], "name": ["can't be blank"]},
            ),
        ],
    )
    def test_allows_reserved_field_names(self, body, message, field_errors):
        err = error_from_response(400, body)
        assert str(err) == message
        assert err.field_errors == field_errors

    def test_not_extracted_outside_validation_statuses(self):
        err = error_from_response(500, b'{"payload_url": ["is not a valid URL"]}')
        assert not hasattr(err, "field_errors")
