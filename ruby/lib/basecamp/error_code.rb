# frozen_string_literal: true

module Basecamp
  # Error codes for API responses
  module ErrorCode
    USAGE = "usage"
    NOT_FOUND = "not_found"
    AUTH = "auth_required"
    FORBIDDEN = "forbidden"
    RATE_LIMIT = "rate_limit"
    NETWORK = "network"
    API = "api_error"
    AMBIGUOUS = "ambiguous"
    VALIDATION = "validation"
  end
end
