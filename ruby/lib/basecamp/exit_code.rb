# frozen_string_literal: true

module Basecamp
  # Exit codes for CLI tools
  module ExitCode
    OK = 0
    USAGE = 1
    NOT_FOUND = 2
    AUTH = 3
    FORBIDDEN = 4
    RATE_LIMIT = 5
    NETWORK = 6
    API = 7
    AMBIGUOUS = 8
    VALIDATION = 9
  end
end
