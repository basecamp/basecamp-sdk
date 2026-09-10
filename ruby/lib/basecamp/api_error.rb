# frozen_string_literal: true

module Basecamp
  # Raised for generic API errors.
  class ApiError < Error
    def initialize(message, http_status: nil, hint: nil, retryable: false, retry_after: nil, cause: nil)
      super(
        code: ErrorCode::API,
        message: message,
        hint: hint,
        http_status: http_status,
        retryable: retryable,
        retry_after: retry_after,
        cause: cause
      )
    end

    # Creates an ApiError from an HTTP status code.
    # @param status [Integer] HTTP status code
    # @param message [String, nil] optional error message
    # @param hint [String, nil] optional hint (SPEC section 6 step 3)
    # @param retry_after [Integer, nil] seconds from a parsed Retry-After header
    # @return [ApiError]
    def self.from_status(status, message = nil, hint: nil, retry_after: nil)
      message ||= "Request failed (HTTP #{status})"
      retryable = status >= 500 && status < 600
      new(message, http_status: status, hint: hint, retryable: retryable, retry_after: retry_after)
    end
  end
end
