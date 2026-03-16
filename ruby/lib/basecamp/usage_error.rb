# frozen_string_literal: true

module Basecamp
  # Raised when there's a usage error (invalid arguments, missing config).
  class UsageError < Error
    def initialize(message, hint: nil)
      super(code: ErrorCode::USAGE, message: message, hint: hint)
    end
  end
end
