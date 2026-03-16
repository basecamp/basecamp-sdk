# frozen_string_literal: true

module Basecamp
  # Raised when a resource is not found (404).
  class NotFoundError < Error
    def initialize(resource = nil, identifier = nil, message: nil, hint: nil)
      message ||= if resource
        "#{resource} not found: #{identifier}"
      else
        "Not found"
      end
      super(
        code: ErrorCode::NOT_FOUND,
        message: message,
        hint: hint,
        http_status: 404
      )
    end
  end
end
