# frozen_string_literal: true

module Basecamp
  # Result information for completed service operations.
  OperationResult = Data.define(:duration_ms, :error) do
    def initialize(duration_ms: 0, error: nil)
      super
    end
  end
end
