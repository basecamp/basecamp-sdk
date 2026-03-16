# frozen_string_literal: true

module Basecamp
  # Raised when a resource is not found (404).
  class NotFoundError < Error
    def initialize(resource, identifier, hint: nil)
      super(
        code: ErrorCode::NOT_FOUND,
        message: "#{resource} not found: #{identifier}",
        hint: hint,
        http_status: 404
      )
    end
  end
end
