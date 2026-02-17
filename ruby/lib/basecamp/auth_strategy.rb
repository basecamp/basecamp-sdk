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

  # Bearer token authentication strategy (default).
  # Sets the Authorization header with "Bearer {token}".
  class BearerAuth
    include AuthStrategy

    # @param token_provider [TokenProvider] provides access tokens
    def initialize(token_provider)
      @token_provider = token_provider
    end

    # @return [TokenProvider] the underlying token provider
    attr_reader :token_provider

    def authenticate(headers)
      headers["Authorization"] = "Bearer #{@token_provider.access_token}"
    end
  end
end
