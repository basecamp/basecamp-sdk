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

    def test_only_the_three_http_date_shapes_parse(self):
        # parsedate_to_datetime is RFC 5322's parser and reads far more than
        # RFC 7231's three forms: a bare date, a numeric or unknown zone. Each
        # of those used to become a saturated delay (naive ones once the
        # asctime branch read naive as UTC; aware ones outright); SPEC section
        # 6's table says they are not dates, so the shape is gated before the
        # parser sees the value and they fall through to backoff.
        now = datetime(2021, 6, 9, 10, 18, 14, tzinfo=UTC)
        assert _parse_retry_after("Wed, 09 Jun 2021 10:18:17 GMT", now=now) == 3
        assert _parse_retry_after("Wednesday, 09-Jun-21 10:18:17 GMT", now=now) == 3
        assert _parse_retry_after("Wed Jun  9 10:18:17 2021", now=now) == 3
        for value in (
            "1 Jan 2099 00:00:00",
            "Wed, 09 Jun 2021 10:18:17 XYZ",
            "Wed, 09 Jun 2021 10:18:17 -0000",
            "Thu, 31 Dec 2099 23:59:59 +0000",
            "Thu, 31 Dec 2099 23:59:59 UTC",
            "Wed, 9 Jun 2021 10:18:17 GMT",
        ):
            assert _parse_retry_after(value, now=now) is None, value

    def test_none(self):
        assert _parse_retry_after(None) is None

    def test_http_date_in_future(self):
        from email.utils import format_datetime

        future = datetime.now(UTC) + timedelta(seconds=30)
        value = format_datetime(future, usegmt=True)  # IMF-fixdate ends in GMT
        result = _parse_retry_after(value)
        assert result is not None
        assert 25 <= result <= 35  # allow some clock drift

    def test_http_date_in_past_returns_none(self):
        from email.utils import format_datetime

        past = datetime.now(UTC) - timedelta(seconds=30)
        value = format_datetime(past, usegmt=True)
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


class TestCompositeIdentities:
    """Identity is the class; `code` is the canonical SPEC section 6 answer."""

    def test_no_error_in_the_module_carries_a_code_outside_the_enum(self):
        # Third attempt at this test, and the first two both failed to catch a
        # new member breaking the rule -- which is the ONLY thing it exists for.
        #
        #   v1 filtered `vars(errors)` to six hard-coded names, so it could
        #      only fail on a rename.
        #   v2 walked the AST for `code=` STRING LITERALS. The module contains
        #      none: every site is `ErrorCode.X` (10) or `_COMPOSITE_CODE[...]`
        #      (6), so it asserted `not {}` and passed no matter what was added.
        #
        # So the rule is inverted here. Rather than recognising the safe forms
        # and checking them, it requires EVERY `code=` site to be one of the
        # forms this test knows how to verify, and verifies each. A new site in
        # any other shape -- a bare literal, an f-string, a call -- fails as
        # unrecognised rather than passing unexamined.
        import ast
        import pathlib

        import basecamp.errors as _errors

        canonical = {member.value for member in _errors.ErrorCode}
        source = pathlib.Path(_errors.__file__).read_text()
        unverifiable: list[str] = []
        outside: list[str] = []
        sites = 0
        for node in ast.walk(ast.parse(source)):
            if not isinstance(node, ast.Call):
                continue
            for keyword in node.keywords:
                if keyword.arg != "code":
                    continue
                sites += 1
                value = keyword.value
                where = f"line {value.lineno}"
                if isinstance(value, ast.Constant):
                    if value.value not in canonical:
                        outside.append(f"{value.value!r} at {where}")
                elif isinstance(value, ast.Attribute) and getattr(value.value, "id", None) == "ErrorCode":
                    if value.attr not in _errors.ErrorCode.__members__:
                        outside.append(f"ErrorCode.{value.attr} at {where}")
                elif (
                    isinstance(value, ast.Subscript)
                    and getattr(value.value, "id", None) == "_COMPOSITE_CODE"
                    and isinstance(value.slice, ast.Constant)
                ):
                    if value.slice.value not in _errors._COMPOSITE_CODE:
                        outside.append(f"_COMPOSITE_CODE[{value.slice.value!r}] missing at {where}")
                else:
                    unverifiable.append(f"{ast.dump(value)[:60]} at {where}")
        assert sites >= 16, f"the walk found only {sites} `code=` sites; it has stopped finding them"
        assert not unverifiable, f"a `code=` site this test cannot verify: {unverifiable}"
        assert not outside, f"a code outside the closed ErrorCode enum: {outside}"
        # ...and the table every composite reads from is itself in-enum, which
        # is what a seventh identity added the way the six existing ones are
        # written would violate.
        rogue = {key: code for key, code in _errors._COMPOSITE_CODE.items() if code not in canonical}
        assert not rogue, f"_COMPOSITE_CODE maps an identity outside the enum: {rogue}"

    def test_each_composite_exit_code_is_the_one_its_identity_should_get(self):
        # The previous version asserted `{exit codes} == {1, 2, 7}` -- a SET
        # over six errors, so four of the six could be misclassified without
        # changing it, and `campfire_index_load_aborted` was pinned by nothing
        # in the entire repo: flipping it API->NOT_FOUND left all 2080 tests
        # and all 274 conformance cases green. Its exit code is a public,
        # documented answer, so it is spelled out per identity here.
        import basecamp.errors as _errors

        # `retryable` is pinned alongside, because it is the other half of what
        # card 40 settled for `campfire_discovery_incomplete` and the half a
        # code change could silently carry away with it.
        expected = {
            "NoRecordingTypeError": ("usage", 1, False),
            "UnknownRecordingTypeError": ("usage", 1, False),
            "BucketMismatchError": ("usage", 1, False),
            "RecordingUnresolvedError": ("not_found", 2, False),
            "CampfireDiscoveryIncompleteError": ("usage", 1, False),
            "CampfireIndexLoadAbortedError": ("api_error", 7, True),
        }
        built = [
            _errors.NoRecordingTypeError(routing_key="boost.created"),
            _errors.UnknownRecordingTypeError(routing_key="x.y"),
            _errors.BucketMismatchError(bucket_id=1, recording_id=2, requested_bucket_id=3),
            _errors.RecordingUnresolvedError(bucket_id=1, recording_id=2, campfire_ids=[], refreshed=False),
            _errors.CampfireDiscoveryIncompleteError(bucket_id=1, recording_id=2, reason="r"),
            _errors.CampfireIndexLoadAbortedError(),
        ]
        assert {type(e).__name__ for e in built} == set(expected), "every identity must be built here"
        for error in built:
            name = type(error).__name__
            assert (error.code, error.exit_code, error.retryable) == expected[name], name

    def test_discovery_incomplete_cannot_be_told_it_is_retryable(self):
        # `DeviceFlowError` overwrites rather than setdefaults so a caller's
        # kwarg cannot flip the invariant. Claiming that in prose without
        # writing it is how it would be lost at the first caller who passes it.
        from basecamp.errors import CampfireDiscoveryIncompleteError

        error = CampfireDiscoveryIncompleteError(bucket_id=1, recording_id=2, reason="r", retryable=True)
        assert error.retryable is False

    def test_neither_composite_raises_a_bare_type_error_on_the_retryable_kwarg(self):
        # The sibling was left forwarding `**kwargs` beside a fixed
        # `retryable=`, so passing the flag raised TypeError about duplicate
        # keyword arguments -- outside this SDK's taxonomy entirely.
        from basecamp.errors import CampfireDiscoveryIncompleteError, CampfireIndexLoadAbortedError

        assert CampfireIndexLoadAbortedError(retryable=False).retryable is True
        assert CampfireIndexLoadAbortedError(retryable=True).retryable is True
        assert (
            CampfireDiscoveryIncompleteError(bucket_id=1, recording_id=2, reason="r", retryable=False).retryable
            is False
        )


class TestRetryableKeywordDoesNotCollide:
    """A fixed ``retryable`` must CONSUME the caller's keyword, not collide with it.

    ``RateLimitError(retryable=False)``, ``NetworkError(retryable=False)`` and
    ``LimitExceededError(retryable=True)`` each raised a bare ``TypeError``
    about duplicate keyword arguments: the constructor forwarded ``**kwargs``
    beside a fixed ``retryable=``. That is an error from outside this SDK's
    taxonomy -- ``except BasecampError`` does not catch it, and ``except
    Exception`` cannot tell it from a bug -- raised on a call the ``**kwargs:
    Any`` signature, and mypy reading that signature, both said was legal.

    The answer is the one the composite's two errors and ``DeviceFlowError``
    already use: overwrite, so the caller gets the class invariant. ``ApiError``
    honours the caller instead, and is not an exception to the rule but the
    other half of it -- it fixes no retryability (500 is retryable, 418 is not),
    so there is nothing for a caller's value to contradict.
    """

    def test_the_three_accept_the_flag_and_yield_their_invariant(self):
        from basecamp.errors import LimitExceededError

        for passed in (True, False):
            assert RateLimitError(retryable=passed).retryable is True
            assert NetworkError(retryable=passed).retryable is True
            # Never retryable: no amount of backoff frees storage. A caller who
            # could flip this would put a retry loop into a spin against a full
            # disk.
            assert LimitExceededError(retryable=passed).retryable is False

    def test_everything_previously_accepted_is_unchanged(self):
        # The other direction, which is what makes this a widening rather than a
        # change: every call that worked before still works, with the same
        # values. Each of these is a call `error_from_response` itself makes.
        from basecamp.errors import LimitExceededError

        rate = RateLimitError("slow down", retry_after=7, http_status=429, hint="h", request_id="r")
        assert (rate.code, rate.retryable, rate.retry_after) == (ErrorCode.RATE_LIMIT, True, 7)
        assert (rate.http_status, rate.hint, rate.request_id) == (429, "h", "r")
        assert str(rate) == "slow down"

        network = NetworkError("no route", hint="check the network")
        assert (network.code, network.retryable, network.hint) == (ErrorCode.NETWORK, True, "check the network")
        assert str(network) == "no route"

        limit = LimitExceededError("out of storage", http_status=507, hint="h")
        assert (limit.code, limit.retryable, limit.http_status) == (ErrorCode.LIMIT_EXCEEDED, False, 507)
        assert str(limit) == "out of storage"

        # Defaults, too -- the no-argument form each class documents.
        assert (RateLimitError().retryable, NetworkError().retryable, LimitExceededError().retryable) == (
            True,
            True,
            False,
        )

        # And the classification `error_from_response` derives, which is where
        # a flipped invariant would actually be felt.
        assert error_from_response(429, b"").retryable is True
        assert error_from_response(507, b"").retryable is False

    def test_api_error_still_honours_a_caller_that_sets_the_flag(self):
        # The half of the rule that is NOT overwrite. ApiError fixes no
        # retryability, so the caller's value is the value -- which is how
        # `error_from_response` tells a 500 from a 418.
        assert ApiError(retryable=True).retryable is True
        assert ApiError(retryable=False).retryable is False

    def test_every_subclass_answers_the_flag_with_the_value_it_should(self):
        """The sweep, so a new subclass can reintroduce neither the crash nor the wrong answer.

        The card that filed this named three classes because that is where its
        author looked. This walks the whole package instead: every
        ``BasecampError`` descendant, not a list someone remembered to extend.

        Two things are pinned, and the first version of this test pinned only
        the first. **That the flag does not raise**: a constructor advertising
        ``retryable`` -- by declaring it, or by accepting ``**kwargs`` and so
        telling callers and mypy alike that the keyword is legal -- must accept
        it. **And that it answers with the right value**: an earlier version
        asserted only ``isinstance(error.retryable, bool)``, which is true of
        every possible answer, so regressing all six overwrite sites to
        ``kwargs.setdefault`` -- "honour the caller", the semantics this change
        rejects on the merits -- left it green. #888's own commit message is a
        three-iteration post-mortem of a sweep in this file that swept nothing;
        this is the same mistake one register down, and ``_FIXED_RETRYABILITY``
        below is the fix: a per-class expected value, and every class must be
        in it or in the caller-wins set.

        A constructor that advertises the keyword NOWHERE is exempt: a stray
        one is "unexpected keyword argument", refused by mypy as well as at
        runtime, which is ordinary Python rather than an error escaping the
        taxonomy. The exempt set is pinned by name so the sweep cannot be
        dodged by dropping ``**kwargs`` from a constructor.
        """
        import importlib
        import inspect
        import pkgutil

        import basecamp
        from basecamp.errors import BasecampError

        for module in pkgutil.walk_packages(basecamp.__path__, "basecamp."):
            importlib.import_module(module.name)

        def descendants(cls):
            found = set()
            for sub in cls.__subclasses__():
                # Only this package's own errors. `__subclasses__` sees every
                # subclass that has been IMPORTED, so without this filter an
                # error class defined at module scope in any other test file
                # joins the walk -- green under `pytest tests/test_errors.py`
                # and red under the whole suite.
                if sub.__module__.startswith("basecamp."):
                    found.add(sub)
                found |= descendants(sub)
            return found

        # Constructor arguments for the subclasses that require them.
        required = {
            "NoRecordingTypeError": ((), {"routing_key": "boost.created"}),
            "UnknownRecordingTypeError": ((), {"routing_key": "x.y"}),
            "BucketMismatchError": ((), {"bucket_id": 1, "recording_id": 2, "requested_bucket_id": 3}),
            "RecordingUnresolvedError": ((), {"bucket_id": 1, "recording_id": 2, "campfire_ids": []}),
            "CampfireDiscoveryIncompleteError": ((), {"bucket_id": 1, "recording_id": 2, "reason": "r"}),
            "PeopleConfirmationRequiredError": (("m",), {"people": []}),
            "OAuthError": (("auth", "m"), {}),
            "DiscoverySelectionError": (("issuer_mismatch", "m"), {}),
            "DeviceFlowError": (("transport", "m"), {}),
            "_IssuerBindingError": (("m",), {}),
            "UsageError": (("m",), {}),
            # No __init__ of its own: it inherits the base's, message and all.
            "RecordingRoutingError": (("m",), {}),
        }

        # The classes that FIX their retryability, and to what. Everything else
        # fixes none, so the caller's value is the value -- `ApiError` being the
        # one that matters, since that is how `error_from_response` tells a 500
        # from a 418. `DeviceFlowError` derives it from `reason`, and the
        # arguments above give it `transport`, the one retryable reason.
        fixed = {
            "RateLimitError": True,
            "NetworkError": True,
            "LimitExceededError": False,
            "CampfireDiscoveryIncompleteError": False,
            "CampfireIndexLoadAbortedError": True,
            "DeviceFlowError": True,
        }

        # Every error this package defines. Spelled out rather than counted: a
        # floor only catches removals once, and `>= n` cannot see a class that
        # arrives and another that leaves. Both directions force a decision.
        expected_subclasses = {
            "AmbiguousError",
            "ApiError",
            "AuthError",
            "BucketMismatchError",
            "CampfireDiscoveryIncompleteError",
            "CampfireIndexLoadAbortedError",
            "DeviceFlowError",
            "DiscoverySelectionError",
            "ForbiddenError",
            "LimitExceededError",
            "NetworkError",
            "NoRecordingTypeError",
            "NotFoundError",
            "OAuthError",
            "PeopleConfirmationRequiredError",
            "RateLimitError",
            "RecordingRoutingError",
            "RecordingUnresolvedError",
            "UnknownRecordingTypeError",
            "UsageError",
            "ValidationError",
            "WebhookVerificationError",
            "_IssuerBindingError",
        }

        subclasses = sorted(descendants(BasecampError), key=lambda c: c.__name__)
        assert {c.__name__ for c in subclasses} == expected_subclasses, (
            "the set of BasecampError subclasses changed. A new one needs a row in "
            "`fixed` (or a deliberate place in the caller-wins set) and a name here."
        )

        exempt, caller_wins = [], []
        for cls in subclasses:
            args, kwargs = required.get(cls.__name__, ((), {}))
            try:
                # Baseline: the class builds at all, so a failure below is the
                # flag and not the arguments.
                cls(*args, **kwargs)
            except TypeError as exc:
                pytest.fail(
                    f"{cls.__name__} could not be constructed for the sweep ({exc}). "
                    f"Add its constructor arguments to the `required` map in this test."
                )

            parameters = inspect.signature(cls.__init__).parameters
            advertised = "retryable" in parameters or any(
                p.kind is inspect.Parameter.VAR_KEYWORD for p in parameters.values()
            )
            if not advertised:
                exempt.append(cls.__name__)
                continue

            answers = {}
            for passed in (True, False):
                # Must not raise: that is the defect this change fixes.
                answers[passed] = cls(*args, **{**kwargs, "retryable": passed}).retryable

            if cls.__name__ in fixed:
                invariant = fixed[cls.__name__]
                assert answers == {True: invariant, False: invariant}, (
                    f"{cls.__name__} fixes retryable={invariant}, but a caller moved it: {answers}. "
                    "Overwrite the keyword -- never setdefault, which honours the caller instead."
                )
            else:
                assert answers == {True: True, False: False}, (
                    f"{cls.__name__} fixes no retryability, so the caller's value should be the "
                    f"value, but it answered {answers}. If it should fix one, add it to `fixed`."
                )
                caller_wins.append(cls.__name__)

        assert exempt == ["WebhookVerificationError"], (
            f"the set of constructors that advertise no retryable changed: {exempt}. "
            "Dropping `**kwargs` from a constructor removes it from this sweep."
        )
        assert set(fixed) | set(caller_wins) | set(exempt) == expected_subclasses, (
            "every subclass must be classified: it fixes retryability, or the caller's value wins, "
            "or it advertises the keyword nowhere."
        )
