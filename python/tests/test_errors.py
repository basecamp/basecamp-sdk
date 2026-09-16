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

        expected = {
            "NoRecordingTypeError": ("usage", 1),
            "UnknownRecordingTypeError": ("usage", 1),
            "BucketMismatchError": ("usage", 1),
            "RecordingUnresolvedError": ("not_found", 2),
            "CampfireDiscoveryIncompleteError": ("api_error", 7),
            "CampfireIndexLoadAbortedError": ("api_error", 7),
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
            assert (error.code, error.exit_code) == expected[name], name

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


# --- The error-construction inventory -----------------------------------------
#
# One place that knows how to build every `BasecampError` in the package, shared
# by the tests below rather than sampled by each. A sample is how the first
# version of the `code` carve-out test checked 4 of 21 classes: `AuthError`
# could have started absorbing a caller's `code` with every test still green.

#: How to construct each subclass, as (label, args, kwargs) rows. Arguments are
#: bound BY KEYWORD wherever the signature allows, which is load-bearing rather
#: than stylistic: a class given its message positionally would report a
#: *collision* on ``message=`` that is just Python's ordinary duplicate-argument
#: error, and an earlier probe read six such artifacts as the defect. Most need
#: one row. A class whose behaviour is DERIVED from an argument needs one row per
#: value of that argument, or a test pins only the value it happened to pass --
#: which is how a `DeviceFlowError` answering `True` for all five reasons, and a
#: `DiscoverySelectionError` overriding one of the three reasons left out of an
#: earlier draft, would both have passed.
_ERROR_VARIANTS: dict[str, list[tuple[str, tuple[object, ...], dict[str, object]]]] = {
    # The classes that fix their retryability are built at a custom message as
    # well as their default. `error_from_response` builds them with a message
    # from the response body, so the default-only probe was leaving the
    # production construction path unexercised -- and `if message == "Account
    # limit reached":` hid a conditional overwrite behind it.
    "RateLimitError": [("default", (), {}), ("custom message", (), {"message": "slow down"})],
    "NetworkError": [("default", (), {}), ("custom message", (), {"message": "no route"})],
    "LimitExceededError": [("default", (), {}), ("custom message", (), {"message": "out of storage"})],
    "CampfireIndexLoadAbortedError": [("default", (), {}), ("custom message", (), {"message": "abandoned"})],
    "NoRecordingTypeError": [("", (), {"routing_key": "boost.created"})],
    "UnknownRecordingTypeError": [("", (), {"routing_key": "x.y"})],
    "BucketMismatchError": [("", (), {"bucket_id": 1, "recording_id": 2, "requested_bucket_id": 3})],
    "RecordingUnresolvedError": [("", (), {"bucket_id": 1, "recording_id": 2, "campfire_ids": []})],
    "CampfireDiscoveryIncompleteError": [("", (), {"bucket_id": 1, "recording_id": 2, "reason": "r"})],
    "PeopleConfirmationRequiredError": [("", (), {"message": "m", "people": []})],
    "OAuthError": [
        (t, (), {"oauth_type": t, "message": "m"}) for t in ("validation", "auth", "network", "api_error", "usage")
    ],
    # Every member of `DiscoverySelectionReason`, not the three a first draft
    # listed: `capability_unavailable` classifies as `validation` and the other
    # five as `api_error`, so a partial list leaves arms unprobed.
    "DiscoverySelectionError": [
        (r, (), {"reason": r, "message": "m"})
        for r in (
            "ambiguous_issuers",
            "expected_issuer_unavailable",
            "invalid_issuer_origin",
            "as_fetch_failed",
            "issuer_mismatch",
            "capability_unavailable",
        )
    ],
    # Every member of `DeviceFlowReason`, not just the retryable one.
    "DeviceFlowError": [
        (r, (), {"reason": r, "message": "m"})
        for r in ("access_denied", "expired", "transport", "unavailable", "cancelled")
    ],
    "_IssuerBindingError": [("", (), {"message": "m"})],
    "UsageError": [("", (), {"message": "m"})],
    # No __init__ of its own: it inherits the base's, message and all.
    "RecordingRoutingError": [("", (), {"message": "m"})],
}

#: The classes that FIX their retryability, and to what. A bool applies to every
#: variant; a dict gives the answer per variant label, which is what a DERIVED
#: retryability needs. Everything else fixes none, so the caller's value is the
#: value -- `ApiError` being the one that matters, since that is how
#: `error_from_response` tells a 500 from a 418.
_FIXED_RETRYABILITY: dict[str, bool | dict[str, bool]] = {
    "RateLimitError": True,
    "NetworkError": True,
    "LimitExceededError": False,
    "CampfireDiscoveryIncompleteError": False,
    "CampfireIndexLoadAbortedError": True,
    # SPEC section 16: only a transport failure is retryable.
    # `tests/oauth/test_device.py::TestDeviceFlowErrorRetryability` pins this
    # independently and predates this sweep; here it keeps the sweep honest
    # about derived values for the next class that has one.
    "DeviceFlowError": {
        "access_denied": False,
        "expired": False,
        "transport": True,
        "unavailable": False,
        "cancelled": False,
    },
}

#: Every error this package defines. Spelled out rather than counted: a floor
#: only catches removals once, and `>= n` cannot see a class that arrives and
#: another that leaves. Both directions force a decision.
_EXPECTED_ERROR_SUBCLASSES = {
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


def _error_subclasses():
    """Every `BasecampError` descendant this package defines, name-sorted."""
    import importlib
    import pkgutil

    import basecamp
    from basecamp.errors import BasecampError

    for module in pkgutil.walk_packages(basecamp.__path__, "basecamp."):
        importlib.import_module(module.name)

    def descendants(cls):
        found = set()
        for sub in cls.__subclasses__():
            # Only this package's own errors. `__subclasses__` sees every
            # subclass that has been IMPORTED, so without this filter an error
            # class defined at module scope in any other test file joins the
            # walk -- green under `pytest tests/test_errors.py` and red under
            # the whole suite. `== "basecamp"` is not redundant with the prefix:
            # a class defined in the package's own `__init__.py` has exactly
            # that module name, and a sweep that skipped it would skip a public
            # error.
            if sub.__module__ == "basecamp" or sub.__module__.startswith("basecamp."):
                found.add(sub)
            # Known limit, written down rather than papered over:
            # `__subclasses__` sees what has been DEFINED, so a class minted
            # inside a function body is absent until that function first runs.
            # Nothing in the package does that today; if something starts to,
            # this walk becomes order-dependent between running this file alone
            # and running the suite, and `_EXPECTED_ERROR_SUBCLASSES` is what
            # will say so.
            #
            # Recurse unconditionally: a package class can sit under a
            # non-package one.
            found |= descendants(sub)
        return found

    found = sorted(descendants(BasecampError), key=lambda c: c.__name__)
    assert {c.__name__ for c in found} == _EXPECTED_ERROR_SUBCLASSES, (
        "the set of BasecampError subclasses changed. A NEW one needs a name in "
        "`_EXPECTED_ERROR_SUBCLASSES`, a row in `_ERROR_VARIANTS` if its constructor takes "
        "arguments, and a row in `_FIXED_RETRYABILITY` if it fixes its retryability. A REMOVED "
        "one needs its name taken out of all three."
    )
    return found


def _build_rows(cls):
    """The (label, args, kwargs) rows that construct ``cls``."""
    return _ERROR_VARIANTS.get(cls.__name__, [("", (), {})])


def _refusal_of(cls, args, kwargs, keyword, value):
    """How ``cls`` answers ``keyword``: 'collides', 'undeclared' or 'accepts'.

    'collides' is the shape this whole line of work is about -- a fixed keyword
    forwarded beside ``**kwargs``, which Python reports as *multiple values* and
    which escapes the SDK's taxonomy. 'undeclared' is ordinary Python: the
    constructor never advertised the keyword, so mypy refuses the call too.
    """
    try:
        cls(*args, **{**kwargs, keyword: value})
    except TypeError as exc:
        return "collides" if "multiple values" in str(exc) else "undeclared"
    return "accepts"


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

        Three things are pinned, and each was added because the previous version
        of this test did not pin it.

        1. **The flag does not raise.** A constructor advertising ``retryable``
           -- by declaring it, or by accepting ``**kwargs`` and so telling
           callers and mypy alike that the keyword is legal -- must accept it.
        2. **It answers with the right value.** An earlier version asserted only
           ``isinstance(error.retryable, bool)``, which is true of every possible
           answer, so regressing all six overwrite sites to ``kwargs.setdefault``
           -- "honour the caller", the semantics this change rejects on the
           merits -- left it green.
        3. **It answers that way whatever else is passed.** The version after
           that probed exactly one argument shape per class. A *conditional*
           overwrite::

               if "retry_after" not in kwargs:  # instead of unconditionally
                   kwargs["retryable"] = False

           left ``LimitExceededError("x", retryable=True, retry_after=5)``
           retryable -- the invariant this whole change exists to defend -- and
           passed the repo as it stood against the v2 file, all 2218 tests of
           it. (That count and the 2219 quoted in
           :meth:`test_a_fixed_retryability_is_overwritten_unconditionally` are
           not one number drifting by one. They are two different suites: the
           v2 ``test_errors.py`` holds 101 tests to this one's 103, and the
           2219 is this suite with that single test deselected. Both were
           re-run on this head rather than remembered.) So the flag is crossed with
           companion keywords and with a custom message, and classes whose
           retryability is DERIVED are probed at every value of the argument it
           derives from -- which is a property of ``_ERROR_VARIANTS``, a table
           a human writes, so
           :meth:`test_a_fixed_retryability_is_overwritten_unconditionally`
           enforces it rather than trusting it: a constructor deriving the flag
           from a declared argument must carry a per-variant expectation and
           rows that reach more than one value of that argument.

        Item 3 is a NET, not a proof, and saying otherwise is how v2 got
        written. Crossing the flag with a set of companion VALUES cannot see a
        condition on a value outside that set: the first draft of this list
        carried 429 and 503 but not 507 -- the one status
        ``LimitExceededError`` exists for -- and built every fixed class at its
        default message only, so both
        ``if kwargs.get("http_status") != 507:`` and ``if message == "Account
        limit reached":`` still hid, and still passed the whole repo. The
        values below are chosen from what these classes document, and
        :meth:`test_a_fixed_retryability_is_overwritten_unconditionally` closes
        the class of hole structurally instead of by sampling.

        #888's own commit message is a three-iteration post-mortem of a sweep in
        this file that swept nothing. Each numbered item above is the same
        mistake one register further down, which is why the assertions say what
        the right answer is rather than that there is one. A sweep is not tested
        by the code it passes on; it is tested by the regressions it rejects.

        A constructor that advertises the keyword NOWHERE is exempt: a stray one
        is "unexpected keyword argument", refused by mypy as well as at runtime,
        which is ordinary Python rather than an error escaping the taxonomy. The
        exempt set is pinned by name so the sweep cannot be dodged by dropping
        ``**kwargs`` from a constructor.
        """
        import inspect

        # Companion keywords the flag is crossed with, so an overwrite that is
        # conditional on any of them cannot hide. `code` and `message` are
        # deliberately absent: they still refuse, which
        # `test_code_and_message_still_refuse_across_the_whole_hierarchy` pins.
        companions = [
            {},
            {"http_status": 503},
            # 507 is the status `LimitExceededError` exists for, and leaving it
            # out let `if kwargs.get("http_status") != 507:` hide a conditional
            # overwrite. Every status a class in `_FIXED_RETRYABILITY` documents
            # belongs here -- though see the docstring: a value set is a net,
            # not a proof, which is why the AST check below exists.
            {"http_status": 507},
            # 500 is the status `ApiError`'s own reasoning names ("500 is
            # retryable, 418 is not") and the arm `error_from_response` builds
            # it at. It was missing, and `ApiError` is in neither
            # `_FIXED_RETRYABILITY` nor the AST check below -- so for the
            # caller-wins classes this sweep is the ONLY check, and
            # `if kwargs.get("http_status") == 500: retryable = True` sat
            # inside it with the whole suite green.
            {"http_status": 500},
            {"retry_after": 5},
            {"hint": "h", "request_id": "r"},
            {"http_status": 429, "retry_after": 5, "hint": "h", "request_id": "r"},
        ]

        exempt, caller_wins = [], []
        for cls in _error_subclasses():
            parameters = inspect.signature(cls.__init__).parameters
            advertised = "retryable" in parameters or any(
                p.kind is inspect.Parameter.VAR_KEYWORD for p in parameters.values()
            )

            for label, args, kwargs in _build_rows(cls):
                try:
                    # Baseline: the class builds at all, so a failure below is
                    # the flag and not the arguments.
                    cls(*args, **kwargs)
                except TypeError as exc:
                    pytest.fail(
                        f"{cls.__name__} could not be constructed for the sweep ({exc}). "
                        f"Add its constructor arguments to `_ERROR_VARIANTS`."
                    )

                if not advertised:
                    continue

                expected = _FIXED_RETRYABILITY.get(cls.__name__)
                if isinstance(expected, dict):
                    expected = expected[label]

                # The flag is passed on every probe below, which means none of
                # them observes the DEFAULT. Flipping the base constructor's
                # `retryable: bool = False` to `True` made every caller-wins
                # error in the package silently retryable and left this whole
                # file green; the one test in the repo that noticed was an
                # unrelated Campfire cache test, by accident. So each row is
                # also built with no flag at all.
                assert cls(*args, **kwargs).retryable is (False if expected is None else expected), (
                    f"{cls.__name__}({label!r}) built with no `retryable` at all answers "
                    f"{cls(*args, **kwargs).retryable}, not "
                    f"{False if expected is None else expected}. A class that fixes its "
                    "retryability must answer its invariant; one that does not must answer the "
                    "base default, which is False."
                )

                for companion in companions:
                    answers = {}
                    for passed in (True, False):
                        # Must not raise: that is the defect this change fixes.
                        error = cls(*args, **{**kwargs, **companion, "retryable": passed})
                        answers[passed] = error.retryable
                    where = f"{cls.__name__}({label!r}) with {companion}"
                    if expected is None:
                        # NOTE, and it is a real one rather than a shrug: this
                        # branch pins today's Python behaviour, and for the
                        # composite and discovery identities that behaviour
                        # DIVERGES from five of the six other SDKs. Not from
                        # all six, and not "Python is the outlier": an earlier
                        # draft of this note said that, and Rust is the
                        # counter-example it had not read.
                        #
                        # The five put it out of the caller's reach in three
                        # different ways, and they are worth telling apart --
                        # one of them is what Python already does for its fixed
                        # three. TypeScript FORCES the value for the composite
                        # family (`super(code, message, {...options, retryable:
                        # false})`, the same move as `kwargs["retryable"] =
                        # False` here) and REFUSES it at the signature for
                        # `DiscoverySelectionError`, whose options type is
                        # `cause` and `httpStatus` only; Ruby's
                        # `RecordingSummaryError#initialize(kind:, message:,
                        # code:, hint:)` and `DiscoverySelectionError#initialize
                        # (reason, message, http_status:)` take no `retryable:`
                        # keyword; Kotlin's `RecordingSummaryFailure` is an
                        # `internal constructor` with no such parameter, and its
                        # `DiscoverySelection` passes `false` itself. Go and
                        # Swift do not model retryability on these identities
                        # AT ALL, which is a third thing again: Go models
                        # the composite identities as their own structs
                        # (`RecordingRoutingError`, `UnresolvedRecordingError`,
                        # `BucketMismatchError`,
                        # `CampfireDiscoveryIncompleteError`) that unwrap to
                        # sentinels and carry no retryable field at all, the
                        # field living on the unrelated `basecamp.Error`; and
                        # Swift's `RecordingSummaryError` is a plain enum with
                        # no retryability anywhere.
                        #
                        # Rust is NOT on that side. Its `RecordingSummaryError`
                        # and `SelectionFailure` carry no retryability either,
                        # and `From<RecordingSummaryError> for Error` builds at
                        # `Error::new`'s `retryable: false` -- but
                        # `Error::retryable(bool)` is a public builder, so
                        # `Error::from(reason).retryable(true)` is exactly this
                        # hazard, and `RecordingSummaryError::of` still
                        # identifies the result as the composite's verdict.
                        # Python's keyword is the easiest route to a retryable
                        # deterministic refusal; it is not the only one.
                        #
                        # Python lets a caller set
                        # `NoRecordingTypeError(routing_key=..., retryable=True)`
                        # -- a deterministic refusal, decided from the caller's
                        # own arguments before any request, that then invites a
                        # retry loop to re-run the identical refusal forever.
                        #
                        # Pinning it here is not a claim that it is right. It is
                        # test-only ground: changing it is a behaviour change
                        # with its own MIGRATING entry, and it should be decided
                        # across the composites and `DiscoverySelectionError`
                        # together rather than one class at a time. Recorded so
                        # the next reader inherits the question instead of an
                        # assertion that looks settled.
                        assert answers == {True: True, False: False}, (
                            f"{where} fixes no retryability, so the caller's value should be the "
                            f"value, but it answered {answers}. If it should fix one, add it to "
                            "`_FIXED_RETRYABILITY`."
                        )
                    elif answers[True] == answers[False]:
                        assert answers[True] == expected, (
                            f"{where} should fix retryable={expected} but fixes {answers[True]}. "
                            "The caller is not moving it -- the class's own invariant is wrong, or "
                            "`_FIXED_RETRYABILITY` is."
                        )
                    else:
                        assert answers == {True: expected, False: expected}, (
                            f"{where} fixes retryable={expected}, but a caller moved it: {answers}. "
                            "Overwrite the keyword unconditionally -- never setdefault (which honours "
                            "the caller) and never under a condition on the other keywords."
                        )

            if not advertised:
                exempt.append(cls.__name__)
            elif cls.__name__ not in _FIXED_RETRYABILITY:
                caller_wins.append(cls.__name__)

        assert exempt == ["WebhookVerificationError"], (
            f"the set of constructors that advertise no retryable changed: {exempt}. "
            "Dropping `**kwargs` from a constructor removes it from this sweep."
        )
        # Every class lands in exactly one bucket by construction, so what this
        # actually catches is a stale or misspelled key in
        # `_FIXED_RETRYABILITY` -- including one naming a class that advertises
        # the keyword nowhere, whose row would be silently dead because `exempt`
        # classes are never probed.
        assert set(_FIXED_RETRYABILITY) | set(caller_wins) | set(exempt) == _EXPECTED_ERROR_SUBCLASSES, (
            "every subclass must be classified: it fixes retryability, or the caller's value wins, "
            "or it advertises the keyword nowhere."
        )
        assert not set(_FIXED_RETRYABILITY) & set(exempt), (
            "`_FIXED_RETRYABILITY` names a class that advertises `retryable` nowhere, so its row is "
            f"never checked: {set(_FIXED_RETRYABILITY) & set(exempt)}"
        )

    def test_a_fixed_retryability_is_overwritten_unconditionally(self):
        """A fixed retryability is set once, unconditionally, in one canonical form.

        The sweep above crosses the flag with a set of companion VALUES, and a
        value set cannot prove a negative. Three times now a *conditional*
        overwrite slipped past it by branching on something the set did not
        contain::

            if "retry_after" not in kwargs:
                ...  # v2
            if kwargs.get("http_status") != 507:
                ...  # v3
            if message == "Account limit reached":
                ...  # v3

        Each was closed by widening the net, and each time the next condition
        was one argument further out. That is unwinnable: the condition is
        chosen after the net. And widening alone does not even close the
        obvious cases -- an unconditional overwrite followed by a *nested*
        ``kwargs.update({"retryable": True})`` under a condition on an unprobed
        status leaks past the value sweep entirely: with this test deselected,
        the other 2219 pass. It is the only thing that sees it, which is the
        argument for its existence and also the reason not to overstate it --
        "the whole suite stayed green" would be a claim about a suite this test
        is a member of.

        So this asserts the FORM, which is where the property is decidable.
        Each class that fixes its retryability must mention the flag exactly
        once in its constructor, as a top-level::

            kwargs["retryable"] = <expression not reading kwargs>

        It is deliberately a style rule and slightly stricter than
        "unconditional": ``kwargs |= {"retryable": False}``, an aliased
        ``kw = kwargs; kw["retryable"] = False``, and a subclass setting
        ``self.retryable = False`` after ``super().__init__`` -- the form a
        subclass author reaches for first -- are all correct and all rejected.
        The point is to keep the property *checkable* -- one
        statement, one place, no second write for a condition to hide in -- so
        that the value sweep's job is tractable rather than open-ended. Writing
        the canonical form costs nothing; the alternatives cost a test that
        cannot see them.

        This does not replace the value sweep, which catches what a form cannot:
        ``kwargs.setdefault(...)`` and a simply wrong invariant are both
        single-statement and both wrong. The two together are the check.
        """
        import ast
        import inspect
        import textwrap

        def names_the_flag(node):
            """True for any syntax that says ``retryable``, however it is spelled.

            The string in a subscript or a ``setdefault``/``update`` call, the
            attribute in ``self.retryable``, a bare name, a declared parameter.
            Counting ALL of them is the point: a second mention anywhere is a
            second place the value can come from.
            """
            return (
                (isinstance(node, ast.Constant) and node.value == "retryable")
                or (isinstance(node, ast.Attribute) and node.attr == "retryable")
                or (isinstance(node, ast.Name) and node.id == "retryable")
                or (isinstance(node, ast.arg) and node.arg == "retryable")
            )

        def is_the_canonical_target(node):
            return (
                isinstance(node, ast.Subscript)
                and isinstance(node.value, ast.Name)
                and node.value.id == "kwargs"
                and isinstance(node.slice, ast.Constant)
                and node.slice.value == "retryable"
            )

        for name in sorted(_FIXED_RETRYABILITY):
            cls = next(c for c in _error_subclasses() if c.__name__ == name)

            # A `retryable` property or class attribute would answer over
            # whatever the constructor put in `__dict__`, which no amount of
            # reading the constructor can see.
            assert "retryable" not in vars(cls), (
                f"{name} defines its own `retryable` attribute, which overrides what the "
                "constructor sets. The invariant belongs in the constructor's overwrite."
            )

            # `inspect.getsource` calls `inspect.unwrap`, so a `functools.wraps`
            # decorator hands this test the INNER function while the wrapper is
            # what runs. A wrapper doing `self.retryable = True` after the
            # constructor returned put a 507 back to retryable through
            # `error_from_response` with the whole suite green -- the same
            # hazard as the `vars(cls)` guard above, one indirection out.
            assert not hasattr(cls.__init__, "__wrapped__"), (
                f"{name}'s `__init__` is wrapped, so `inspect.getsource` reads the function inside "
                "the decorator and not the one that runs. Whatever the wrapper does to `retryable` "
                "is invisible here: put the invariant in the constructor itself."
            )
            tree = ast.parse(textwrap.dedent(inspect.getsource(cls.__init__)))
            function = tree.body[0]
            assert isinstance(function, ast.FunctionDef), name
            assert not function.decorator_list, (
                f"{name}'s `__init__` carries a decorator, which can rewrite `retryable` after this "
                "test's view of the constructor ends."
            )

            canonical = [
                stmt
                for stmt in function.body
                if isinstance(stmt, ast.Assign) and len(stmt.targets) == 1 and is_the_canonical_target(stmt.targets[0])
            ]
            assert len(canonical) == 1, (
                f"{name} is in `_FIXED_RETRYABILITY`, so its constructor must contain exactly one "
                'top-level `kwargs["retryable"] = ...` statement; it has '
                f"{len(canonical)}. Anything else -- `setdefault`, `update`, `|=`, an alias, a "
                "nested assignment -- is a second place the value can come from, and this test "
                "cannot see into those."
            )

            mentions = [node for node in ast.walk(function) if names_the_flag(node)]
            assert len(mentions) == 1 and mentions[0] is canonical[0].targets[0].slice, (
                f"{name} mentions `retryable` {len(mentions)} times in its constructor (line(s) "
                f"{sorted({m.lineno for m in mentions})}). Exactly one mention, in the canonical "
                "overwrite, is the whole point: a second one is a condition's hiding place. A "
                'nested `kwargs.update({"retryable": True})` beside a correct top-level '
                "overwrite leaked with the entire suite green."
            )

            reads_kwargs = [
                node for node in ast.walk(canonical[0].value) if isinstance(node, ast.Name) and node.id == "kwargs"
            ]
            assert not reads_kwargs, (
                f"{name}'s overwrite reads `kwargs` on its right-hand side, so the caller's value "
                "can reach the invariant. The value assigned must come from the class or from its "
                'own declared arguments -- `DeviceFlowError`\'s `reason == "transport"` is the '
                "shape to follow."
            )

            # ...and if it DOES derive from a declared argument, that argument
            # has to be swept at more than one value, or the derivation is a
            # condition again -- just spelled as an expression rather than an
            # `if`. `kwargs["retryable"] = retry_after is None or retry_after <
            # 3600` and `= "certificate" not in message` are both single
            # top-level statements that read no `kwargs`, and both passed
            # everything else here: `_FIXED_RETRYABILITY` still said `True`
            # unconditionally while the class had quietly become conditional.
            # So a derived value forces the two things that make it visible: a
            # per-variant expectation, and rows that actually reach both arms.
            declared = {
                p.arg
                for p in (function.args.posonlyargs + function.args.args + function.args.kwonlyargs)
                if p.arg != "self"
            }
            derived_from = sorted(
                {node.id for node in ast.walk(canonical[0].value) if isinstance(node, ast.Name) and node.id in declared}
            )
            if derived_from:
                expected = _FIXED_RETRYABILITY[name]
                assert isinstance(expected, dict), (
                    f"{name}'s retryability is derived from {derived_from}, so its row in "
                    "`_FIXED_RETRYABILITY` must be a per-variant dict rather than one bool -- a "
                    "single bool asserts the same answer at every value, which is what a derived "
                    "value is not."
                )
                rows = _ERROR_VARIANTS.get(name, [])
                for argument in derived_from:
                    values = {repr(row_kwargs.get(argument)) for _label, _args, row_kwargs in rows}
                    assert len(values) >= 2, (
                        f"{name} derives its retryability from `{argument}`, and `_ERROR_VARIANTS` "
                        f"builds it at {len(values)} value(s) of it. The sweep can only pin the "
                        "arms it constructs: add a row per value, as `DeviceFlowError` has one per "
                        "`reason`."
                    )

    def test_code_and_message_still_refuse_across_the_whole_hierarchy(self):
        """The two collisions deliberately NOT absorbed, pinned so they stay deliberate.

        ``retryable`` was absorbed because the base class declares it as an
        ordinary knob, ``ApiError`` honours it, and the value a caller asks for
        is not derivable from anything else they passed. The other two are not
        like that, and nothing but this test says so:

        - ``code`` is the class's identity. ``NotFoundError(code="rate_limit")``
          is a contradiction rather than a request, and overwriting it would
          silently swallow a caller's real confusion -- the outcome this change
          calls worse than a refusal.
        - ``message`` on the five composites that compose one is DERIVED from
          the caller's own structured arguments (``routing_key``, ``bucket_id``,
          ``reason``), so the caller already controls it and a refusal defeats
          no intent.

        Without this, the next reader finds three identical-looking collisions,
        one of them "fixed", and no record that the other two were decided.

        Exhaustive, not a sample. The first version of this test built four
        classes by hand, so `AuthError` could have started absorbing a caller's
        ``code`` with every test in the repo still green -- the same
        partial-coverage mistake the sweep above went through three rounds of.
        Every subclass is classified here. For ``code`` all three sets are
        named; for ``message`` the ``collides`` set is named and ``undeclared``
        is asserted empty, so ``accepts`` is pinned as the complement -- the
        same coverage, said exactly.
        """
        from basecamp.errors import ErrorCode

        code, message = {}, {}
        for cls in _error_subclasses():
            for _label, args, kwargs in _build_rows(cls):
                code.setdefault(_refusal_of(cls, args, kwargs, "code", ErrorCode.API), set()).add(cls.__name__)
                message.setdefault(_refusal_of(cls, args, kwargs, "message", "override"), set()).add(cls.__name__)

        # `code`: every class that fixes one collides. The two that do not are
        # named, so absorbing it anywhere else fails here.
        assert code.get("accepts") == {
            # Declares no code of its own -- it inherits the base's constructor,
            # where `code` is an ordinary declared parameter.
            "RecordingRoutingError",
        }, f"a class started accepting `code`: {code.get('accepts')}"
        assert code.get("undeclared") == {
            # Advertises no keywords at all, so this is "unexpected keyword
            # argument" -- refused by mypy too.
            "WebhookVerificationError",
        }, f"the `code`-undeclared set changed: {code.get('undeclared')}"
        assert code.get("collides") == _EXPECTED_ERROR_SUBCLASSES - {
            "RecordingRoutingError",
            "WebhookVerificationError",
        }, "a class stopped refusing `code`"

        # `message`: only the five composites that COMPOSE one collide. Every
        # other class declares `message` and takes the caller's.
        assert message.get("collides") == {
            "BucketMismatchError",
            "CampfireDiscoveryIncompleteError",
            "NoRecordingTypeError",
            "RecordingUnresolvedError",
            "UnknownRecordingTypeError",
        }, f"the set of classes that compose their message changed: {message.get('collides')}"
        assert not message.get("undeclared"), f"a class stopped declaring `message`: {message.get('undeclared')}"
