# frozen_string_literal: true

module Basecamp
  # Basecamp refused to start the move or copy (a 4xx), or the SDK refused
  # before sending it: nothing was filed. Its +cause+ is the refusal.
  class FilingRefusedError < FilingError
    KIND = "filing_refused"
  end
end
