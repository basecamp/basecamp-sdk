# frozen_string_literal: true

module Basecamp
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
end
