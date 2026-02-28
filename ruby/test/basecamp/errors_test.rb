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
    error = Basecamp::APIError.from_status(500)

    assert_equal "Request failed (HTTP 500)", error.message
    assert_equal 500, error.http_status
    assert error.retryable?
  end

  def test_api_error_4xx_not_retryable
    error = Basecamp::APIError.from_status(400)

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

    assert_instance_of Basecamp::APIError, error
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
