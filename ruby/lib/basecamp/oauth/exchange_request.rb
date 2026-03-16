# frozen_string_literal: true

module Basecamp
  module Oauth
    # Parameters for exchanging an authorization code for tokens.
    #
    # @attr token_endpoint [String] URL of the token endpoint
    # @attr code [String] The authorization code received from the authorization server
    # @attr redirect_uri [String] The redirect URI used in the authorization request
    # @attr client_id [String] The client identifier
    # @attr client_secret [String, nil] The client secret (optional for public clients)
    # @attr code_verifier [String, nil] PKCE code verifier (optional)
    # @attr use_legacy_format [Boolean] Use Launchpad's non-standard token format
    ExchangeRequest = Data.define(
      :token_endpoint,
      :code,
      :redirect_uri,
      :client_id,
      :client_secret,
      :code_verifier,
      :use_legacy_format
    ) do
      def initialize(
        token_endpoint:,
        code:,
        redirect_uri:,
        client_id:,
        client_secret: nil,
        code_verifier: nil,
        use_legacy_format: false
      )
        super
      end
    end
  end
end
