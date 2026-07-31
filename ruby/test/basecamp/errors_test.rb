# frozen_string_literal: true

require "test_helper"

class ErrorsTest < Minitest::Test
  def test_error_has_code_and_message
    error = Basecamp::Error.new(
      code: Basecamp::ErrorCode::NOT_FOUND,
      message: "Resource not found"
    )

    assert_equal Basecamp::ErrorCode::NOT_FOUND, error.code
    assert_equal "Resource not found", error.message
  end

  def test_error_has_exit_code
    error = Basecamp::NotFoundError.new("Project", "123")

    assert_equal Basecamp::ExitCode::NOT_FOUND, error.exit_code
  end

  def test_not_found_error
    error = Basecamp::NotFoundError.new("Project", "123")

    assert_equal "Project not found: 123", error.message
    assert_equal 404, error.http_status
  end

  def test_auth_error
    error = Basecamp::AuthError.new

    assert_equal "Authentication required", error.message
    assert_equal 401, error.http_status
    assert_not error.retryable?
  end

  def test_forbidden_error
    error = Basecamp::ForbiddenError.new

    assert_equal "Access denied", error.message
    assert_equal 403, error.http_status
  end

  def test_forbidden_scope_error
    error = Basecamp::ForbiddenError.insufficient_scope

    assert_equal "Access denied: insufficient scope", error.message
    assert_includes error.hint, "Re-authenticate"
  end

  def test_rate_limit_error
    error = Basecamp::RateLimitError.new(retry_after: 30)

    assert_equal "Rate limit exceeded", error.message
    assert_equal 429, error.http_status
    assert error.retryable?
    assert_equal 30, error.retry_after
  end

  def test_network_error_is_retryable
    error = Basecamp::NetworkError.new("Connection timeout")

    assert error.retryable?
  end

  def test_api_error_from_status
    error = Basecamp::ApiError.from_status(500)

    assert_equal "Request failed (HTTP 500)", error.message
    assert_equal 500, error.http_status
    assert error.retryable?
  end

  def test_api_error_4xx_not_retryable
    error = Basecamp::ApiError.from_status(400)

    assert_not error.retryable?
  end

  def test_validation_error
    error = Basecamp::ValidationError.new("Name is required")

    assert_equal "Name is required", error.message
    assert_equal 400, error.http_status
  end

  def test_validation_error_preserves_422_status
    error = Basecamp::ValidationError.new("Unprocessable", http_status: 422)

    assert_equal "Unprocessable", error.message
    assert_equal 422, error.http_status
  end

  def test_error_from_response_422
    error = Basecamp.error_from_response(422, '{"error": "Invalid data"}')

    assert_instance_of Basecamp::ValidationError, error
    assert_equal 422, error.http_status
    assert_equal "Invalid data", error.message
  end

  def test_error_from_response_422_field_keyed_flattens_into_message
    error = Basecamp.error_from_response(422, '{"errors": {"color": ["is not a valid color"]}}')

    assert_instance_of Basecamp::ValidationError, error
    assert_equal "color: is not a valid color", error.message
    assert_equal({ "color" => [ "is not a valid color" ] }, error.field_errors)
  end

  def test_error_from_response_422_field_keyed_sorts_and_joins
    body = '{"errors": {"name": ["can\'t be blank", "is too short"], "color": ["is not a valid color"]}}'
    error = Basecamp.error_from_response(422, body)

    assert_equal "color: is not a valid color, name: can't be blank; is too short", error.message
    assert_equal(
      { "color" => [ "is not a valid color" ], "name" => [ "can't be blank", "is too short" ] },
      error.field_errors
    )
  end

  def test_error_from_response_422_field_keyed_appends_to_top_level_error
    body = '{"error": "Validation failed", "errors": {"color": ["is not a valid color"]}}'
    error = Basecamp.error_from_response(422, body)

    assert_equal "Validation failed (color: is not a valid color)", error.message
  end

  def test_error_from_response_400_field_keyed_extracts_too
    error = Basecamp.error_from_response(400, '{"errors": {"color": ["is not a valid color"]}}')

    assert_instance_of Basecamp::ValidationError, error
    assert_equal "color: is not a valid color", error.message
    assert_equal({ "color" => [ "is not a valid color" ] }, error.field_errors)
  end

  def test_error_from_response_403_does_not_extract_field_errors
    error = Basecamp.error_from_response(403, '{"errors": {"color": ["is not a valid color"]}}')

    assert_not error.respond_to?(:field_errors)
  end

  def test_error_from_response_422_field_keyed_skips_malformed_entries
    body = '{"errors": {"color": "not an array", "name": ["can\'t be blank"], "empty": [], "mixed": [42, "is invalid"]}}'
    error = Basecamp.error_from_response(422, body)

    assert_equal "mixed: is invalid, name: can't be blank", error.message
    assert_equal({ "mixed" => [ "is invalid" ], "name" => [ "can't be blank" ] }, error.field_errors)
  end

  def test_error_from_response_422_unusable_errors_shape_falls_back
    [ '{"errors": {"color": "not an array"}}', '{"errors": []}', '{"errors": "nope"}', '{"errors": {}}' ].each do |body|
      error = Basecamp.error_from_response(422, body)

      assert_nil error.field_errors, "expected nil field_errors for #{body}"
      assert_equal "Request failed", error.message, "expected fallback message for #{body}"
    end
  end

  def test_error_from_response_422_truncates_after_flattening_keeps_raw_slot
    long = "x" * 600
    error = Basecamp.error_from_response(422, %({"errors": {"color": ["#{long}"]}}))

    assert_equal 500, error.message.bytesize
    assert error.message.start_with?("color: xxx")
    assert error.message.end_with?("...")
    assert_equal({ "color" => [ long ] }, error.field_errors)
  end

  def test_error_from_response_422_plain_error_body_unchanged
    error = Basecamp.error_from_response(422, '{"error": "Name can\'t be blank"}')

    assert_equal "Name can't be blank", error.message
    assert_nil error.field_errors
  end

  def test_validation_error_field_errors_default_nil
    assert_nil Basecamp::ValidationError.new("nope").field_errors
  end

  def test_validation_error_exit_code
    error = Basecamp::ValidationError.new("Name is required")

    assert_equal 9, error.exit_code
  end

  def test_ambiguous_error_exit_code
    error = Basecamp::AmbiguousError.new("project", matches: %w[A B])

    assert_equal 8, error.exit_code
  end

  def test_ambiguous_error_with_matches
    error = Basecamp::AmbiguousError.new("project", matches: %w[Project1 Project2])

    assert_includes error.hint, "Project1"
    assert_includes error.hint, "Project2"
    assert_equal %w[Project1 Project2], error.matches
  end

  def test_error_from_response_401
    error = Basecamp.error_from_response(401, nil)

    assert_instance_of Basecamp::AuthError, error
  end

  def test_error_from_response_404
    error = Basecamp.error_from_response(404, nil)

    assert_instance_of Basecamp::NotFoundError, error
  end

  def test_error_from_response_429
    error = Basecamp.error_from_response(429, nil, retry_after: 60)

    assert_instance_of Basecamp::RateLimitError, error
    assert_equal 60, error.retry_after
  end

  def test_error_from_response_500
    error = Basecamp.error_from_response(500, nil)

    assert_instance_of Basecamp::ApiError, error
    assert error.retryable?
  end

  def test_parse_error_message_from_json
    body = '{"error": "Invalid request"}'
    message = Basecamp.parse_error_message(body)

    assert_equal "Invalid request", message
  end

  def test_parse_error_message_returns_nil_for_invalid_json
    message = Basecamp.parse_error_message("not json")

    assert_nil message
  end
end
