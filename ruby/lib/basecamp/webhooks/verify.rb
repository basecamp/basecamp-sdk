# frozen_string_literal: true

require "openssl"

module Basecamp
  module Webhooks
    # HMAC-SHA256 signature verification for webhook payloads.
    module Verify
      # Verifies an HMAC-SHA256 signature for a webhook payload.
      # Returns false if secret or signature is empty/nil.
      def self.valid?(payload:, signature:, secret:)
        return false if secret.nil? || secret.empty?
        return false if signature.nil? || signature.empty?

        expected = compute_signature(payload: payload, secret: secret)
        secure_compare(expected, signature)
      end

      # Computes the HMAC-SHA256 signature for a webhook payload.
      def self.compute_signature(payload:, secret:)
        OpenSSL::HMAC.hexdigest("SHA256", secret, payload)
      end

      # Timing-safe string comparison
      def self.secure_compare(a, b)
        return false if a.nil? || b.nil?
        return false if a.bytesize != b.bytesize

        # Use OpenSSL's constant-time comparison via HMAC
        OpenSSL.fixed_length_secure_compare(a, b)
      end

      private_class_method :secure_compare
    end
  end
end
