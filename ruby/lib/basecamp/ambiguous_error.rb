# frozen_string_literal: true

module Basecamp
  # Raised when a name/identifier matches multiple resources.
  class AmbiguousError < Error
    # @return [Array<String>] list of matching resources
    attr_reader :matches

    def initialize(resource, matches: [])
      @matches = matches
      hint = if matches.any? && matches.length <= 5
               "Did you mean: #{matches.join(", ")}"
      else
               "Be more specific"
      end
      super(
        code: ErrorCode::AMBIGUOUS,
        message: "Ambiguous #{resource}",
        hint: hint
      )
    end
  end
end
