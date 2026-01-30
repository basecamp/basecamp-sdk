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

    # OAuth 2 access token response.
    #
    # @attr access_token [String] The access token string
    # @attr token_type [String] Token type (usually "Bearer")
    # @attr refresh_token [String, nil] The refresh token string
    # @attr expires_in [Integer, nil] Lifetime of the access token in seconds
    # @attr expires_at [Time, nil] Calculated expiration time
    # @attr scope [String, nil] OAuth scope granted
    Token = Data.define(
      :access_token,
      :token_type,
      :refresh_token,
      :expires_in,
      :expires_at,
      :scope
    ) do
      def initialize(
        access_token:,
        token_type: "Bearer",
        refresh_token: nil,
        expires_in: nil,
        expires_at: nil,
        scope: nil
      )
        # Calculate expires_at from expires_in if not provided
        calculated_expires_at = expires_at || (expires_in ? Time.now + expires_in : nil)
        super(
          access_token: access_token,
          token_type: token_type,
          refresh_token: refresh_token,
          expires_in: expires_in,
          expires_at: calculated_expires_at,
          scope: scope
        )
      end

      # Checks if the token is expired or about to expire.
      #
      # @param buffer_seconds [Integer] Buffer time before actual expiration (default: 60)
      # @return [Boolean] true if expired or will expire within buffer time
      def expired?(buffer_seconds = 60)
        return false unless expires_at

        Time.now + buffer_seconds >= expires_at
      end
    end

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

    # Parameters for refreshing an access token.
    #
    # @attr token_endpoint [String] URL of the token endpoint
    # @attr refresh_token [String] The refresh token
    # @attr client_id [String, nil] The client identifier (optional)
    # @attr client_secret [String, nil] The client secret (optional)
    # @attr use_legacy_format [Boolean] Use Launchpad's non-standard token format
    RefreshRequest = Data.define(
      :token_endpoint,
      :refresh_token,
      :client_id,
      :client_secret,
      :use_legacy_format
    ) do
      def initialize(
        token_endpoint:,
        refresh_token:,
        client_id: nil,
        client_secret: nil,
        use_legacy_format: false
      )
        super
      end
    end
  end
end
