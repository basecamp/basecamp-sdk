# frozen_string_literal: true

module Basecamp
  # Raised when API access has been disabled by an account administrator.
  # The Basecamp API returns 404 with a "Reason: API Disabled" header.
  class ApiDisabledError < Error
    def initialize(message: "API access is disabled for this account", hint: nil)
      super(
        code: ErrorCode::API_DISABLED,
        message: message,
        hint: hint || "An administrator can re-enable it in Adminland under Manage API access",
        http_status: 404
      )
    end
  end
end
