# frozen_string_literal: true

module Basecamp
  module Services
    # Service for authorization operations.
    # This is the only service that doesn't require an account context.
    #
    # @example Get authorization info
    #   auth = client.authorization.get
    #   puts "Identity: #{auth["identity"]["email_address"]}"
    #   auth["accounts"].each do |account|
    #     puts "Account: #{account["name"]} (#{account["id"]})"
    #   end
    class AuthorizationService < BaseService
      # Fallback Launchpad endpoint for authorization
      LAUNCHPAD_AUTHORIZATION_URL = "https://launchpad.37signals.com/authorization.json"

      # Gets authorization information for the current user.
      #
      # Attempts to use the authorization endpoint discovered via OAuth discovery
      # on the configured base URL. Falls back to Launchpad if discovery fails.
      #
      # Returns the authenticated user's identity and list of accounts
      # they have access to.
      #
      # @return [Hash] authorization info with :identity and :accounts
      # @see https://github.com/basecamp/bc3-api/blob/master/sections/authentication.md
      def get
        url = discover_authorization_url
        response = http.get_absolute(url)
        response.json
      end

      private

        def discover_authorization_url
          # Try OAuth discovery on the configured base URL
          config = Oauth.discover(http.base_url)
          # Use issuer as base for authorization.json
          "#{config.issuer.chomp('/')}/authorization.json"
        rescue Oauth::OAuthError
          # Fall back to Launchpad
          LAUNCHPAD_AUTHORIZATION_URL
        end
    end
  end
end
