# frozen_string_literal: true

module Basecamp
  module Oauth
    # OAuth 2 server configuration from discovery endpoint.
    #
    # @attr issuer [String] The authorization server's issuer identifier
    # @attr authorization_endpoint [String] URL of the authorization endpoint
    # @attr token_endpoint [String] URL of the token endpoint
    # @attr registration_endpoint [String, nil] URL of the dynamic client registration endpoint
    # @attr scopes_supported [Array<String>, nil] List of OAuth 2 scopes supported
    Config = Data.define(
      :issuer,
      :authorization_endpoint,
      :token_endpoint,
      :registration_endpoint,
      :scopes_supported
    ) do
      def initialize(
        issuer:,
        authorization_endpoint:,
        token_endpoint:,
        registration_endpoint: nil,
        scopes_supported: nil
      )
        super
      end
    end
  end
end
