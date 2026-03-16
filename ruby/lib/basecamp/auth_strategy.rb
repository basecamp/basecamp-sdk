# frozen_string_literal: true

module Basecamp
  # AuthStrategy controls how authentication is applied to HTTP requests.
  # The default strategy is BearerAuth, which uses a TokenProvider to set
  # the Authorization header with a Bearer token.
  #
  # Custom strategies can implement alternative auth schemes such as
  # cookie-based auth, API keys, or mutual TLS.
  #
  # To implement a custom strategy, create a class that responds to
  # #authenticate(headers), where headers is a Hash that you can modify.
  module AuthStrategy
    # Apply authentication to the given headers hash.
    # @param headers [Hash] the request headers to modify
    def authenticate(headers)
      raise NotImplementedError, "#{self.class} must implement #authenticate"
    end
  end
end
