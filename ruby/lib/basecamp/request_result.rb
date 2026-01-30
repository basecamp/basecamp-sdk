# frozen_string_literal: true

module Basecamp
  # Result information for completed HTTP requests.
  RequestResult = Data.define(:status_code, :duration, :error, :retry_after, :from_cache) do
    def initialize(status_code: nil, duration: 0.0, error: nil, retry_after: nil, from_cache: false)
      super
    end

    def success?
      status_code && status_code >= 200 && status_code < 300
    end
  end
end
