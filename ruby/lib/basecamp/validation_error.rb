# frozen_string_literal: true

module Basecamp
  # Raised for validation errors (400, 422).
  class ValidationError < Error
    def initialize(message, hint: nil, http_status: 400)
      super(
        code: ErrorCode::VALIDATION,
        message: message,
        hint: hint,
        http_status: http_status
      )
    end
  end
end
