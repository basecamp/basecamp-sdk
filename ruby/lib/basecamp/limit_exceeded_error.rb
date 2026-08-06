# frozen_string_literal: true

module Basecamp
  # Raised when an account limit blocks the request (HTTP 507) — file storage
  # exhausted, or a webhook ceiling reached.
  #
  # Never retryable: no amount of backoff frees storage or raises a plan limit.
  # That is the whole reason this is not an ApiError, which a 507 would
  # otherwise become through the 5xx catch-all.
  class LimitExceededError < Error
    def initialize(message = "Account limit reached", hint: nil, cause: nil)
      super(
        code: ErrorCode::LIMIT_EXCEEDED,
        message: message,
        hint: hint,
        http_status: 507,
        retryable: false,
        cause: cause
      )
    end
  end
end
