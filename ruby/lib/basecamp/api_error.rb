# frozen_string_literal: true

module Basecamp
  # Raised for generic API errors.
  class ApiError < Error
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

    # Creates an ApiError from an HTTP status code.
    # @param status [Integer] HTTP status code
    # @param message [String, nil] optional error message
    # @param hint [String, nil] optional hint (SPEC section 6 step 3)
    # @return [ApiError]
    def self.from_status(status, message = nil, hint: nil)
      message ||= "Request failed (HTTP #{status})"
      retryable = status >= 500 && status < 600
      new(message, http_status: status, hint: hint, retryable: retryable)
    end
  end
end
