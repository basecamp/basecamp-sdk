# frozen_string_literal: true

module Basecamp
  # Basecamp started the move or copy and had not finished when the wait ended:
  # the timeout, or a GetFiling that failed (its +cause+). {#filing} is its last
  # known state; poll +get_filing(filing_id: filing["id"])+ rather than starting
  # it again.
  class FilingUnsettledError < FilingError
    KIND = "filing_unsettled"

    # @return [Hash] the filing's last known state
    attr_reader :filing

    def initialize(filing)
      @filing = filing
      super("filing #{filing["id"]} is still #{filing["status"]}")
    end
  end
end
