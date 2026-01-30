# frozen_string_literal: true

module Basecamp
  # Error codes for API responses
  module ErrorCode
    USAGE = "usage"
    NOT_FOUND = "not_found"
    AUTH = "auth_required"
    FORBIDDEN = "forbidden"
    RATE_LIMIT = "rate_limit"
    NETWORK = "network"
    API = "api_error"
    AMBIGUOUS = "ambiguous"
    VALIDATION = "validation"
  end

  # Exit codes for CLI tools
  module ExitCode
    OK = 0
    USAGE = 1
    NOT_FOUND = 2
    AUTH = 3
    FORBIDDEN = 4
    RATE_LIMIT = 5
    NETWORK = 6
    API = 7
    AMBIGUOUS = 8
  end

  # Base error class for all Basecamp SDK errors.
  # Provides structured error handling with codes, hints, and CLI exit codes.
  #
  # @example Catching errors
  #   begin
  #     client.projects.list
  #   rescue Basecamp::Error => e
  #     puts "#{e.code}: #{e.message}"
  #     puts "Hint: #{e.hint}" if e.hint
  #     exit e.exit_code
  #   end
  class Error < StandardError
    # @return [String] error category code
    attr_reader :code

    # @return [String, nil] user-friendly hint for resolving the error
    attr_reader :hint

    # @return [Integer, nil] HTTP status code that caused the error
    attr_reader :http_status

    # @return [Boolean] whether the operation can be retried
    attr_reader :retryable

    # @return [Integer, nil] seconds to wait before retrying (for rate limits)
    attr_reader :retry_after

    # @return [Exception, nil] original error that caused this error
    attr_reader :cause

    # @param code [String] error category code
    # @param message [String] error message
    # @param hint [String, nil] user-friendly hint
    # @param http_status [Integer, nil] HTTP status code
    # @param retryable [Boolean] whether operation can be retried
    # @param retry_after [Integer, nil] seconds to wait before retry
    # @param cause [Exception, nil] underlying cause
    def initialize(code:, message:, hint: nil, http_status: nil, retryable: false, retry_after: nil, cause: nil)
      super(message)
      @code = code
      @hint = hint
      @http_status = http_status
      @retryable = retryable
      @retry_after = retry_after
      @cause = cause
    end

    # Returns the exit code for CLI applications.
    # @return [Integer]
    def exit_code
      self.class.exit_code_for(@code)
    end

    # Returns whether this error can be retried.
    # @return [Boolean]
    def retryable?
      @retryable
    end

    # Maps error codes to exit codes.
    # @param code [String]
    # @return [Integer]
    def self.exit_code_for(code)
      case code
      when ErrorCode::USAGE then ExitCode::USAGE
      when ErrorCode::NOT_FOUND then ExitCode::NOT_FOUND
      when ErrorCode::AUTH then ExitCode::AUTH
      when ErrorCode::FORBIDDEN then ExitCode::FORBIDDEN
      when ErrorCode::RATE_LIMIT then ExitCode::RATE_LIMIT
      when ErrorCode::NETWORK then ExitCode::NETWORK
      when ErrorCode::API then ExitCode::API
      when ErrorCode::AMBIGUOUS then ExitCode::AMBIGUOUS
      else ExitCode::API
      end
    end
  end

  # Raised when there's a usage error (invalid arguments, missing config).
  class UsageError < Error
    def initialize(message, hint: nil)
      super(code: ErrorCode::USAGE, message: message, hint: hint)
    end
  end

  # Raised when a resource is not found (404).
  class NotFoundError < Error
    def initialize(resource, identifier, hint: nil)
      super(
        code: ErrorCode::NOT_FOUND,
        message: "#{resource} not found: #{identifier}",
        hint: hint,
        http_status: 404
      )
    end
  end

  # Raised when authentication fails (401).
  class AuthError < Error
    def initialize(message = "Authentication required", hint: nil, cause: nil)
      super(
        code: ErrorCode::AUTH,
        message: message,
        hint: hint || "Check your access token or refresh it if expired",
        http_status: 401,
        cause: cause
      )
    end
  end

  # Raised when access is denied (403).
  class ForbiddenError < Error
    def initialize(message = "Access denied", hint: nil)
      super(
        code: ErrorCode::FORBIDDEN,
        message: message,
        hint: hint || "You do not have permission to access this resource",
        http_status: 403
      )
    end

    # Creates a forbidden error due to insufficient OAuth scope.
    def self.insufficient_scope
      new("Access denied: insufficient scope", hint: "Re-authenticate with full scope")
    end
  end

  # Raised when rate limited (429).
  class RateLimitError < Error
    def initialize(retry_after: nil, cause: nil)
      hint = retry_after ? "Try again in #{retry_after} seconds" : "Please slow down requests"
      super(
        code: ErrorCode::RATE_LIMIT,
        message: "Rate limit exceeded",
        hint: hint,
        http_status: 429,
        retryable: true,
        retry_after: retry_after,
        cause: cause
      )
    end
  end

  # Raised when there's a network error (connection, timeout, DNS).
  class NetworkError < Error
    def initialize(message = "Network error", cause: nil)
      super(
        code: ErrorCode::NETWORK,
        message: message,
        hint: cause&.message || "Check your network connection",
        retryable: true,
        cause: cause
      )
    end
  end

  # Raised for generic API errors.
  class APIError < Error
    def initialize(message, http_status: nil, hint: nil, retryable: false, cause: nil)
      super(
        code: ErrorCode::API,
        message: message,
        hint: hint,
        http_status: http_status,
        retryable: retryable,
        cause: cause
      )
    end

    # Creates an APIError from an HTTP status code.
    # @param status [Integer] HTTP status code
    # @param message [String, nil] optional error message
    # @return [APIError]
    def self.from_status(status, message = nil)
      message ||= "Request failed (HTTP #{status})"
      retryable = status >= 500 && status < 600
      new(message, http_status: status, retryable: retryable)
    end
  end

  # Raised when a name/identifier matches multiple resources.
  class AmbiguousError < Error
    # @return [Array<String>] list of matching resources
    attr_reader :matches

    def initialize(resource, matches: [])
      @matches = matches
      hint = if matches.any? && matches.length <= 5
               "Did you mean: #{matches.join(", ")}"
      else
               "Be more specific"
      end
      super(
        code: ErrorCode::AMBIGUOUS,
        message: "Ambiguous #{resource}",
        hint: hint
      )
    end
  end

  # Raised for validation errors (400, 422).
  class ValidationError < Error
    def initialize(message, hint: nil)
      super(
        code: ErrorCode::VALIDATION,
        message: message,
        hint: hint,
        http_status: 400
      )
    end
  end

  # Maps an HTTP response to the appropriate error class.
  #
  # @param status [Integer] HTTP status code
  # @param body [String, nil] response body (will attempt JSON parse)
  # @param retry_after [Integer, nil] Retry-After header value
  # @return [Error]
  def self.error_from_response(status, body = nil, retry_after: nil)
    message = parse_error_message(body) || "Request failed"

    case status
    when 400, 422
      ValidationError.new(message)
    when 401
      AuthError.new(message)
    when 403
      ForbiddenError.new(message)
    when 404
      NotFoundError.new("Resource", "unknown")
    when 429
      RateLimitError.new(retry_after: retry_after)
    when 500
      APIError.new("Server error (500)", http_status: 500, retryable: true)
    when 502, 503, 504
      APIError.new("Gateway error (#{status})", http_status: status, retryable: true)
    else
      APIError.from_status(status, message)
    end
  end

  # Parses error message from response body.
  # @param body [String, nil]
  # @return [String, nil]
  def self.parse_error_message(body)
    return nil if body.nil? || body.empty?

    data = JSON.parse(body)
    data["error"] || data["message"]
  rescue JSON::ParserError
    nil
  end
end
