# frozen_string_literal: true

require "uri"

module Basecamp
  # Security helpers for URL validation, message truncation, and header redaction.
  # Used across the SDK to enforce HTTPS, prevent SSRF, and protect sensitive data.
  module Security
    MAX_ERROR_MESSAGE_BYTES = 500
    MAX_RESPONSE_BODY_BYTES = 50 * 1024 * 1024 # 50 MB
    MAX_ERROR_BODY_BYTES = 1 * 1024 * 1024      # 1 MB

    # The Launchpad authorization endpoint is on a different origin than the
    # configured API base URL, so it is a sanctioned destination for a
    # credentialed cross-origin request. Resource-first discovery may also
    # authorize one specific discovered-and-validated issuer origin (see
    # Http#get_authorization_document, whose issuer comes from internal discovery
    # of the configured base URL, not a caller argument); every other foreign
    # origin is rejected.
    LAUNCHPAD_AUTHORIZATION_URL = "https://launchpad.37signals.com/authorization.json"

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
        raise Basecamp::ApiError.new(
          "#{label} body too large (#{body.bytesize} bytes, max #{max})"
        )
      end
    end

    def self.localhost?(url)
      uri = URI.parse(url.to_s)
      host = uri.host&.downcase
      return false if host.nil?
      # The carve-out is limited to HTTP(S) so credential guards fail closed
      # on any other scheme (e.g. ws://localhost).
      return false unless %w[http https].include?(uri.scheme&.downcase)

      host == "localhost" ||
        host == "127.0.0.1" ||
        host == "::1" ||
        host == "[::1]" ||
        host.end_with?(".localhost")
    rescue URI::InvalidURIError
      false
    end

    def self.require_https_unless_localhost!(url, label = "URL")
      return if localhost?(url)

      require_https!(url, label)
    end

    # Parses a caller- or metadata-supplied origin and enforces the origin-root
    # profile (SPEC.md §16): https (or http on localhost), host present, optional
    # valid numeric port, path empty or exactly "/", and no query, fragment, or
    # userinfo. Parsing uses Ruby's +URI+ (never a regex) so bracketed IPv6
    # (+http://[::1]:3000+) and ports agree with the host the client dials.
    #
    # A bad *caller* origin is a usage error; callers validating an *advertised*
    # origin rescue {UsageError} and reclassify.
    #
    # @param raw [String] the origin to validate
    # @param label [String] a label for error messages
    # @return [String] the normalized origin (+scheme://host[:port]+, no trailing slash)
    # @raise [UsageError] on any profile violation or parse failure
    def self.require_origin_root!(raw, label = "origin")
      # Reject C0 controls, space, and backslash up front: URL parsers variously
      # strip tabs/newlines/surrounding spaces or convert backslashes, so a
      # malformed spelling could be cleaned and accepted. None is legitimate here.
      if raw.to_s.match?(/[\x00-\x20\\]/)
        raise UsageError.new("#{label} contains invalid characters: #{raw}")
      end

      uri = URI.parse(raw.to_s)
      scheme = uri.scheme&.downcase

      unless scheme == "https" || (scheme == "http" && localhost?(raw))
        raise UsageError.new("#{label} must use HTTPS (or http on localhost): #{raw}")
      end
      raise UsageError.new("#{label} has no host: #{raw}") if uri.host.nil? || uri.host.empty?

      # The raw authority (between "://" and the first "/?#") backs the presence
      # checks the parsed fields miss: URI reports delimiter-only userinfo
      # ("https://@example.com") as an empty (falsy) string, and normalizes a
      # dangling port ("https://example.com:") to the default-port origin.
      authority = raw.to_s.split("://", 2)[1].to_s.split(%r{[/?#]}, 2)[0].to_s

      # Reject on the PRESENCE of userinfo, not truthiness: an "@" in the authority
      # is always a userinfo delimiter (a host cannot contain one).
      if uri.userinfo || authority.include?("@")
        raise UsageError.new("#{label} must not contain userinfo: #{raw}")
      end
      raise UsageError.new("#{label} must not contain a query or fragment: #{raw}") if uri.query || uri.fragment
      unless uri.path.nil? || uri.path.empty? || uri.path == "/"
        raise UsageError.new("#{label} must be an origin root (no path): #{raw}")
      end

      # A dangling port delimiter ("https://example.com:") silently accepts a
      # malformed authority. IPv6 authorities legitimately end with "]" (e.g.
      # "[::1]"), so only a trailing ":" is a dangling port.
      if authority.end_with?(":")
        raise UsageError.new("#{label} has an invalid port: #{raw}")
      end

      # URI.parse rejects a non-numeric port, but it happily accepts a numeric
      # port outside the valid TCP range (e.g. :0 or :99999). Reject anything
      # outside 1–65535 so a structurally-parseable-but-undialable port can never
      # be treated as a trusted origin.
      if uri.port && !uri.port.between?(1, 65_535)
        raise UsageError.new("#{label} has an out-of-range port: #{raw}")
      end

      # A surviving uri now has a structurally valid, in-range (or default) port.
      # Drop the default port.
      if uri.port && uri.port != uri.default_port
        "#{scheme}://#{uri.host}:#{uri.port}"
      else
        "#{scheme}://#{uri.host}"
      end
    rescue URI::InvalidURIError
      raise UsageError.new("Invalid #{label}: not a valid absolute URL: #{raw}")
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
