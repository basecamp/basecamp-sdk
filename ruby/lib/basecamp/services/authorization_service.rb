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
      # Gets authorization information for the current user.
      #
      # Delegates to {Basecamp::Http#get_authorization_document}, which resolves
      # the issuer via resource-first OAuth discovery (SPEC.md §16) on the client's
      # own configured base URL and fetches the fixed +authorization.json+ path.
      # Only the two *soft* fallback outcomes (+resource_discovery_failed+,
      # +no_as_advertised+) fall back to Launchpad; every *hard* selection failure
      # propagates as a {Oauth::DiscoverySelectionError} — a hard failure is never
      # silently converted into a Launchpad request. This service passes NO issuer,
      # config, or origin to the credentialed fetch, so there is no caller-supplied
      # path through which the bearer token could be sent to a foreign host.
      #
      # Returns the authenticated user's identity and list of accounts
      # they have access to.
      #
      # @return [Hash] authorization info with :identity and :accounts
      # @raise [Oauth::DiscoverySelectionError] on a hard discovery failure after
      #   a BC5 issuer was advertised and selected
      # @see https://github.com/basecamp/bc3-api/blob/master/sections/authentication.md
      def get
        http.get_authorization_document.json
      end
    end
  end
end
