# frozen_string_literal: true

module Basecamp
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
end
