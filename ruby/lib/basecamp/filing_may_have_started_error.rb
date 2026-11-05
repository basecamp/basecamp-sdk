# frozen_string_literal: true

module Basecamp
  # The start of a move or copy failed without Basecamp refusing it, such as a
  # 5xx or a dropped connection. It may have been filed, and each copy request
  # makes another copy, so look before trying again.
  class FilingMayHaveStartedError < FilingError
    KIND = "filing_may_have_started"
  end
end
