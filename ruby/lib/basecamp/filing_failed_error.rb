# frozen_string_literal: true

module Basecamp
  # Basecamp started the move or copy and reported it failed.
  class FilingFailedError < FilingError
    KIND = "filing_failed"

    # @return [Hash] the failed filing
    attr_reader :filing

    def initialize(filing)
      @filing = filing
      super("filing #{filing["id"]} failed")
    end
  end
end
