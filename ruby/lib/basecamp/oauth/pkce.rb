# frozen_string_literal: true

require "securerandom"
require "digest"
require "base64"

module Basecamp
  module Oauth
    # PKCE (Proof Key for Code Exchange) utilities for OAuth 2.0.
    #
    # Provides cryptographically secure code verifier and challenge generation
    # to protect against authorization code interception attacks.
    module Pkce
      # Generates a cryptographically secure PKCE code verifier and challenge.
      #
      # The verifier is 43 characters (32 random bytes, base64url-encoded).
      # The challenge is the base64url-encoded SHA256 hash of the verifier.
      #
      # Use code_challenge_method=S256 with the challenge in the authorization request.
      #
      # @return [Hash] containing :verifier and :challenge keys
      #
      # @example
      #   pkce = Basecamp::Oauth::Pkce.generate
      #
      #   # In authorization request:
      #   auth_url = "#{auth_endpoint}?code_challenge=#{pkce[:challenge]}&code_challenge_method=S256"
      #
      #   # Later, in token exchange:
      #   token = exchange_code(code: code, code_verifier: pkce[:verifier])
      #
      def self.generate
        # Generate 32 random bytes, base64url-encoded without padding
        # Note: Use Base64.urlsafe_encode64 directly for consistent behavior across Ruby versions
        verifier = Base64.urlsafe_encode64(SecureRandom.random_bytes(32), padding: false)

        # Compute SHA256 hash and base64url-encode without padding
        hash = Digest::SHA256.digest(verifier)
        challenge = Base64.urlsafe_encode64(hash, padding: false)

        { verifier: verifier, challenge: challenge }
      end

      # Generates a cryptographically secure OAuth state parameter.
      #
      # The state is 22 characters (16 random bytes, base64url-encoded).
      # Use this to prevent CSRF attacks on the OAuth flow.
      #
      # @return [String] the state parameter
      #
      # @example
      #   state = Basecamp::Oauth::Pkce.generate_state
      #
      #   # Store state in session before redirect:
      #   session[:oauth_state] = state
      #
      #   # In callback handler:
      #   if params[:state] != session[:oauth_state]
      #     raise "State mismatch - possible CSRF attack"
      #   end
      #
      def self.generate_state
        # Note: Use Base64.urlsafe_encode64 directly for consistent behavior across Ruby versions
        Base64.urlsafe_encode64(SecureRandom.random_bytes(16), padding: false)
      end
    end
  end
end
