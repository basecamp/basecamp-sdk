# frozen_string_literal: true

module Basecamp
  # Raised when a template copy requires confirmation before granting project access.
  class PeopleConfirmationRequiredError < ValidationError
    # @return [Array<Basecamp::Types::TemplateLibraryConfirmationPerson>]
    attr_reader :people

    def initialize(message, people:, hint: nil, http_status: 422, field_errors: nil)
      super(message, hint: hint, http_status: http_status, field_errors: field_errors)
      @people = people
    end
  end
end
