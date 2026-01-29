# frozen_string_literal: true

module Basecamp
  # Information about an HTTP request for observability hooks.
  RequestInfo = Data.define(:method, :url, :attempt) do
    def initialize(method:, url:, attempt: 1)
      super
    end
  end
end
