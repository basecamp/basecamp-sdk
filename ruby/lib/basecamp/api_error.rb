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
    # @return [ApiError]
    def self.from_status(status, message = nil)
      message ||= "Request failed (HTTP #{status})"
      retryable = status >= 500 && status < 600
      new(message, http_status: status, retryable: retryable)
    end
  end
end
