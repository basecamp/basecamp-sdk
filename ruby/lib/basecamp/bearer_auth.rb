# frozen_string_literal: true

module Basecamp
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
