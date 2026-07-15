# frozen_string_literal: true

module Basecamp
  module Oauth
    # OAuth 2 server configuration from an Authorization Server Metadata document
    # (RFC 8414).
    #
    # As of BC5 resource-first discovery, +authorization_endpoint+ is OPTIONAL:
    # device-only authorization servers omit it, so authorization-code consumers
    # MUST assert its presence before use. +token_endpoint+ stays required.
    #
    # @attr issuer [String] The authorization server's issuer identifier
    # @attr authorization_endpoint [String, nil] URL of the authorization endpoint (optional)
    # @attr token_endpoint [String] URL of the token endpoint
    # @attr device_authorization_endpoint [String, nil] URL of the RFC 8628 device authorization endpoint
    # @attr registration_endpoint [String, nil] URL of the dynamic client registration endpoint
    # @attr scopes_supported [Array<String>, nil] List of OAuth 2 scopes supported
    # @attr grant_types_supported [Array<String>, nil] OAuth 2 grant types the server supports
    Config = Data.define(
      :issuer,
      :authorization_endpoint,
      :token_endpoint,
      :device_authorization_endpoint,
      :registration_endpoint,
      :scopes_supported,
      :grant_types_supported
    ) do
      def initialize(
        issuer:,
        token_endpoint:,
        authorization_endpoint: nil,
        device_authorization_endpoint: nil,
        registration_endpoint: nil,
        scopes_supported: nil,
        grant_types_supported: nil
      )
        super
      end
    end
  end
end
