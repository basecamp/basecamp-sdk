# frozen_string_literal: true

module Basecamp
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

    # @return [String, nil] X-Request-Id from the response
    attr_reader :request_id

    # @return [Exception, nil] original error that caused this error
    attr_reader :cause

    # @param code [String] error category code
    # @param message [String] error message
    # @param hint [String, nil] user-friendly hint
    # @param http_status [Integer, nil] HTTP status code
    # @param retryable [Boolean] whether operation can be retried
    # @param retry_after [Integer, nil] seconds to wait before retry
    # @param request_id [String, nil] X-Request-Id from response
    # @param cause [Exception, nil] underlying cause
    def initialize(code:, message:, hint: nil, http_status: nil, retryable: false, retry_after: nil, request_id: nil, cause: nil)
      super(message)
      @code = code
      @hint = hint
      @http_status = http_status
      @retryable = retryable
      @retry_after = retry_after
      @request_id = request_id
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
      when ErrorCode::VALIDATION then ExitCode::VALIDATION
      when ErrorCode::API_DISABLED then ExitCode::API_DISABLED
      else ExitCode::API
      end
    end
  end
end
