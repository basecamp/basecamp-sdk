# frozen_string_literal: true

module Basecamp
  # Raised for validation errors (400, 422).
  class ValidationError < Error
    # @return [Hash{String => Array<String>}, nil] field-keyed validation
    #   messages from a 422 body of the form {"errors" => {"field" => ["msg"]}}
    #   — the Rails RecordInvalid rendering. Nil for every other error shape.
    #   The flattened form is also folded into the message; this slot preserves
    #   the raw, untruncated per-field messages.
    attr_reader :field_errors

    def initialize(message, hint: nil, http_status: 400, field_errors: nil)
      super(
        code: ErrorCode::VALIDATION,
        message: message,
        hint: hint,
        http_status: http_status
      )
      @field_errors = field_errors
    end
  end
end
