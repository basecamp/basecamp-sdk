# frozen_string_literal: true

require "uri"

module Basecamp
  # Security helpers for URL validation, message truncation, and header redaction.
  # Used across the SDK to enforce HTTPS, prevent SSRF, and protect sensitive data.
  module Security
    MAX_ERROR_MESSAGE_BYTES = 500
    MAX_RESPONSE_BODY_BYTES = 50 * 1024 * 1024 # 50 MB
    MAX_ERROR_BODY_BYTES = 1 * 1024 * 1024      # 1 MB

    def self.truncate(str, max = MAX_ERROR_MESSAGE_BYTES)
      return str if str.nil? || str.bytesize <= max

      max <= 3 ? str.byteslice(0, max) : str.byteslice(0, max - 3) + "..."
    end

    def self.require_https!(url, label = "URL")
      uri = URI.parse(url.to_s)
      raise UsageError.new("#{label} must use HTTPS: #{url}") unless uri.scheme&.downcase == "https"
    rescue URI::InvalidURIError
      raise UsageError.new("Invalid #{label}: #{url}")
    end

    def self.same_origin?(a, b)
      ua = URI.parse(a)
      ub = URI.parse(b)
      return false if ua.scheme.nil? || ub.scheme.nil?

      ua.scheme.downcase == ub.scheme.downcase &&
        normalize_host(ua) == normalize_host(ub)
    rescue URI::InvalidURIError
      false
    end

    def self.resolve_url(base, target)
      URI.join(base, target).to_s
    rescue URI::InvalidURIError
      target
    end

    def self.normalize_host(uri)
      host = uri.host&.downcase
      port = uri.port
      return host if port.nil?
      return host if uri.scheme&.downcase == "https" && port == 443
      return host if uri.scheme&.downcase == "http" && port == 80

      "#{host}:#{port}"
    end

    def self.check_body_size!(body, max, label = "Response")
      return if body.nil?

      if body.bytesize > max
        raise Basecamp::APIError.new(
          "#{label} body too large (#{body.bytesize} bytes, max #{max})"
        )
      end
    end

    def self.localhost?(url)
      uri = URI.parse(url.to_s)
      host = uri.host&.downcase
      host == "localhost" || host == "127.0.0.1" || host == "::1"
    rescue URI::InvalidURIError
      false
    end

    def self.require_https_unless_localhost!(url, label = "URL")
      return if localhost?(url)

      require_https!(url, label)
    end

    # Headers that contain sensitive values and should be redacted.
    SENSITIVE_HEADERS = %w[
      authorization
      cookie
      set-cookie
      x-csrf-token
    ].freeze

    # Returns a copy of the headers with sensitive values replaced by "[REDACTED]".
    #
    # This is useful for safely logging HTTP requests and responses without
    # exposing tokens, cookies, or other credentials.
    #
    # @param headers [Hash] the headers hash (case-insensitive keys)
    # @return [Hash] a new hash with sensitive values redacted
    #
    # @example
    #   headers = { "Authorization" => "Bearer token", "Content-Type" => "application/json" }
    #   safe = Basecamp::Security.redact_headers(headers)
    #   # => { "Authorization" => "[REDACTED]", "Content-Type" => "application/json" }
    #
    def self.redact_headers(headers)
      result = {}
      headers.each do |key, value|
        result[key] = SENSITIVE_HEADERS.include?(key.to_s.downcase) ? "[REDACTED]" : value
      end
      result
    end
  end
end
