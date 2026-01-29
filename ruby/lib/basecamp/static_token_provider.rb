# frozen_string_literal: true

module Basecamp
  # A simple token provider that returns a static access token.
  # Useful for testing or when you manage token refresh externally.
  #
  # @example
  #   provider = Basecamp::StaticTokenProvider.new(ENV["BASECAMP_ACCESS_TOKEN"])
  class StaticTokenProvider
    include TokenProvider

    # @param token [String] the static access token
    def initialize(token)
      raise ArgumentError, "token cannot be nil or empty" if token.nil? || token.empty?

      @token = token
    end

    # @return [String] the access token
    def access_token
      @token
    end
  end
end
