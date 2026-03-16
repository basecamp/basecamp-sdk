# frozen_string_literal: true

module Basecamp
  # Raised when access is denied (403).
  class ForbiddenError < Error
    def initialize(message = "Access denied", hint: nil)
      super(
        code: ErrorCode::FORBIDDEN,
        message: message,
        hint: hint || "You do not have permission to access this resource",
        http_status: 403
      )
    end

    # Creates a forbidden error due to insufficient OAuth scope.
    def self.insufficient_scope
      new("Access denied: insufficient scope", hint: "Re-authenticate with full scope")
    end
  end
end
