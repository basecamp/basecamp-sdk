# frozen_string_literal: true

module Basecamp
  module Oauth
    # Parameters for refreshing an access token.
    #
    # @attr token_endpoint [String] URL of the token endpoint
    # @attr refresh_token [String] The refresh token
    # @attr client_id [String, nil] The client identifier (optional)
    # @attr client_secret [String, nil] The client secret (optional)
    # @attr use_legacy_format [Boolean] Use Launchpad's non-standard token format
    # @attr resource [String, nil] RFC 8707 resource indicator to bind the
    #   refreshed token to, sent only when set. Echo the stored token's
    #   resource: BC5 multi-account refresh tokens hard-require it (SPEC §16).
    #   Appended last so earlier members keep their positions.
    RefreshRequest = Data.define(
      :token_endpoint,
      :refresh_token,
      :client_id,
      :client_secret,
      :use_legacy_format,
      :resource
    ) do
      def initialize(
        token_endpoint:,
        refresh_token:,
        client_id: nil,
        client_secret: nil,
        use_legacy_format: false,
        resource: nil
      )
        super
      end
    end
  end
end
