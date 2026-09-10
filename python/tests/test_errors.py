from __future__ import annotations

from datetime import UTC, datetime, timedelta

import pytest

from basecamp.errors import (
    MAX_RETRY_AFTER_SECONDS,
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

    def test_sign_is_not_a_delay(self):
        # RFC 9110's 1*DIGIT has no sign; int() would have read +5 as 5.
        assert _parse_retry_after("+5") is None

    def test_over_range_saturates_at_the_ceiling(self):
        # No digit string is malformed for its width; before the ceiling the
        # arbitrary-precision int reached float() on the retry path and raised.
        assert _parse_retry_after("0120") == 120
        assert _parse_retry_after("2147483647") == MAX_RETRY_AFTER_SECONDS
        assert _parse_retry_after("2147483648") == MAX_RETRY_AFTER_SECONDS
        assert _parse_retry_after("9" * 400) == MAX_RETRY_AFTER_SECONDS
        # Past int()'s own digit limit (sys.int_info.str_digits_check_threshold and up).
        assert _parse_retry_after("9" * 5000) == MAX_RETRY_AFTER_SECONDS
        assert _parse_retry_after("0" * 5000) is None
        assert _parse_retry_after("Fri, 31 Dec 9999 23:59:59 GMT") == MAX_RETRY_AFTER_SECONDS

    def test_asctime_form_is_read_as_utc(self):
        # parsedate_to_datetime hands the zoneless asctime form back naive;
        # subtracting an aware now used to raise TypeError, swallowed into None.
        now = datetime(2021, 6, 9, 10, 18, 14, tzinfo=UTC)
        assert _parse_retry_after("Wed Jun  9 10:18:17 2021", now=now) == 3
        assert _parse_retry_after("Wed Jun 19 10:18:17 2021", now=now) == 864003

    def test_other_zoneless_spellings_are_not_http_dates(self):
        # parsedate_to_datetime is RFC 5322's parser and hands back a naive
        # datetime for more than asctime: a bare date, or an IMF-fixdate whose
        # zone it does not know. Reading those as UTC turned a value outside
        # SPEC section 6's table into a saturated delay; only asctime earns the
        # zone, the rest fall through to backoff.
        now = datetime(2021, 6, 9, 10, 18, 14, tzinfo=UTC)
        assert _parse_retry_after("1 Jan 2099 00:00:00", now=now) is None
        assert _parse_retry_after("Wed, 09 Jun 2021 10:18:17 XYZ", now=now) is None
        assert _parse_retry_after("Wed, 09 Jun 2021 10:18:17 -0000", now=now) is None
        assert _parse_retry_after("Wed, 09 Jun 2021 10:18:17 GMT", now=now) == 3

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

    def test_http_date_sub_second_remainder_rounds_up(self):
        # SPEC section 6 step 2: 2.75s out is 3 seconds, never 2. Truncation
        # retried up to a second before the moment the server named, and turned
        # a remainder under a second into 0 -- read as "no usable value".
        now = datetime(2021, 6, 9, 10, 18, 14, 250_000, tzinfo=UTC)
        assert _parse_retry_after("Wed, 09 Jun 2021 10:18:17 GMT", now=now) == 3
        assert _parse_retry_after("Wed, 09 Jun 2021 10:18:15 GMT", now=now) == 1
        assert _parse_retry_after("Wed, 09 Jun 2021 10:18:14 GMT", now=now) is None

    def test_retry_after_carried_on_every_status(self):
        # SPEC section 6 "HTTP Status Mapping Algorithm": one parse feeds both
        # the retry loop's sleep and the error's field, at 503 as at 429.
        err = error_from_response(503, None, {"Retry-After": "7"})
        assert isinstance(err, ApiError)
        assert err.retry_after == 7


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


class TestSpec6Caps:
    def test_401_and_403_messages_are_truncated(self):
        long = "x" * 600
        for status in (401, 403):
            err = error_from_response(status, ('{"error": "' + long + '"}').encode())
            assert len(str(err).encode()) == 500
            assert str(err).endswith("...")

    def test_unmapped_5xx_is_retryable(self):
        # SPEC section 6 step 12: any 5xx outside the mapped arms retries; the
        # 507 arm is the deliberate exception.
        assert error_from_response(599, b"").retryable is True
        assert error_from_response(418, b"").retryable is False
        assert error_from_response(507, b"").retryable is False


class TestRowKeyedErrors:
    """SPEC section 6 step 1b: {"errors": [{"email_address", "messages"}]}, the literal bc3 batch-invite body."""

    @pytest.mark.parametrize(
        "body, message, field_errors",
        [
            (
                b'{"errors":[{"email_address":"not-an-address","messages":["Email address must be valid"]}]}',
                "not-an-address: Email address must be valid",
                {"not-an-address": ["Email address must be valid"]},
            ),
            (
                b'{"errors":[{"email_address":"not-an-address","messages":["Email address must be valid"]},'
                b'{"email_address":null,"messages":["Email address can\'t be blank"]}]}',
                "1: Email address can't be blank, not-an-address: Email address must be valid",
                {"not-an-address": ["Email address must be valid"], "1": ["Email address can't be blank"]},
            ),
            (
                b'{"errors":[{"index":1,"messages":["email_address is invalid"]}]}',
                "1: email_address is invalid",
                {"1": ["email_address is invalid"]},
            ),
            (
                b'{"errors":[{"email_address":"annie@example.com","messages":["Name is too long"]},'
                b'{"email_address":"annie@example.com","messages":["Email address is duplicated"]}]}',
                "annie@example.com: Name is too long; Email address is duplicated",
                {"annie@example.com": ["Name is too long", "Email address is duplicated"]},
            ),
            (
                b'{"errors":[{"email_address":42,"messages":["Email address must be valid"]}]}',
                "0: Email address must be valid",
                {"0": ["Email address must be valid"]},
            ),
            (
                b'{"errors":[{"email_address":"annie@example.com","index":"1","messages":["Name is too long"]}]}',
                "annie@example.com: Name is too long",
                {"annie@example.com": ["Name is too long"]},
            ),
            (
                b'{"errors":[{"email_address":null,"index":true,"messages":["Email address can\'t be blank"]}]}',
                "0: Email address can't be blank",
                {"0": ["Email address can't be blank"]},
            ),
        ],
    )
    def test_keys_rows_by_address_index_or_position(self, body, message, field_errors):
        err = error_from_response(422, body)
        assert isinstance(err, ValidationError)
        assert str(err) == message
        assert err.field_errors == field_errors

    @pytest.mark.parametrize(
        "body",
        [
            b'{"errors": ["nope"]}',
            b'{"errors": []}',
            b'{"errors": [{"email_address": "x"}]}',
            b'{"errors": [{"email_address": "x", "messages": []}]}',
            b'{"errors": [{"email_address": "x", "messages": ["bad"]}, 42]}',
        ],
    )
    def test_strict_gate_leaves_slot_absent(self, body):
        err = error_from_response(422, body)
        assert isinstance(err, ValidationError)
        assert err.field_errors is None
