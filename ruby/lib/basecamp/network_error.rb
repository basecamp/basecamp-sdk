# frozen_string_literal: true

module Basecamp
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
end
