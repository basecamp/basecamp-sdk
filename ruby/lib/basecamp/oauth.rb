# frozen_string_literal: true

module Basecamp
  # OAuth 2 module for Basecamp SDK.
  #
  # Provides OAuth discovery, token exchange, and token refresh functionality.
  # Supports both standard OAuth 2 and Basecamp's Launchpad legacy format.
  #
  # @example Complete OAuth flow
  #   # 1. Discover OAuth configuration
  #   config = Basecamp::Oauth.discover_launchpad
  #
  #   # 2. Build authorization URL (redirect user here)
  #   auth_url = "#{config.authorization_endpoint}?" + URI.encode_www_form(
  #     type: "web_server",
  #     client_id: ENV["BASECAMP_CLIENT_ID"],
  #     redirect_uri: "https://myapp.com/callback"
  #   )
  #
  #   # 3. Exchange authorization code for tokens (in callback handler)
  #   token = Basecamp::Oauth.exchange_code(
  #     token_endpoint: config.token_endpoint,
  #     code: params[:code],
  #     redirect_uri: "https://myapp.com/callback",
  #     client_id: ENV["BASECAMP_CLIENT_ID"],
  #     client_secret: ENV["BASECAMP_CLIENT_SECRET"],
  #     use_legacy_format: true  # Required for Launchpad
  #   )
  #
  #   # 4. Use the token
  #   client = Basecamp.client(
  #     access_token: token.access_token,
  #     account_id: "12345"
  #   )
  #
  #   # 5. Refresh when needed
  #   if token.expired?
  #     token = Basecamp::Oauth.refresh_token(
  #       token_endpoint: config.token_endpoint,
  #       refresh_token: token.refresh_token,
  #       use_legacy_format: true
  #     )
  #   end
  #
  # @see https://github.com/basecamp/api/blob/master/sections/authentication.md
  module Oauth
    LAUNCHPAD_BASE_URL = "https://launchpad.37signals.com"

    # Soft fallback reasons — the ONLY two outcomes under which
    # {discover_from_resource} yields a fallback (Launchpad) rather than a
    # selected config. Every other failure raises {DiscoverySelectionError}.
    FALLBACK_RESOURCE_DISCOVERY_FAILED = "resource_discovery_failed"
    FALLBACK_NO_AS_ADVERTISED = "no_as_advertised"

    # Discovers RFC 8414 Authorization Server Metadata and binds +issuer+ to
    # +base_url+ by code-point.
    #
    # @param base_url [String] the OAuth server's issuer origin
    # @param timeout [Integer] request timeout in seconds
    # @return [Config]
    def self.discover(base_url, timeout: 10)
      Discovery.new(timeout: timeout).discover(base_url)
    end

    def self.discover_launchpad(timeout: 10)
      discover(LAUNCHPAD_BASE_URL, timeout: timeout)
    end

    # Discovers RFC 9728 protected-resource metadata for a resource origin.
    #
    # @param resource_origin [String] the API/resource host origin
    # @param timeout [Integer] request timeout in seconds
    # @return [ProtectedResourceMetadata]
    def self.discover_protected_resource(resource_origin, timeout: 10)
      Resource.new(timeout: timeout).discover(resource_origin)
    end

    # Resource-first discovery orchestrator (SPEC.md §16). Composes RFC 9728
    # (resource metadata) and RFC 8414 (AS metadata) and applies the
    # stage-sensitive fallback state machine.
    #
    # Returns a {DiscoveryResult} that is either +selected+ (a bound AS config)
    # or a soft +fallback+ whose reason is +resource_discovery_failed+ or
    # +no_as_advertised+ ONLY. Every hard failure raises
    # {DiscoverySelectionError} — callers MUST NOT convert a raise into a
    # Launchpad request.
    #
    # @param resource_origin [String] the API/resource host origin
    # @param expected_issuer [String, nil] explicit, authoritative issuer
    #   selection. When provided, the advertised member equal by code-point is
    #   selected; if none matches, +expected_issuer_unavailable+ is raised (never
    #   a fallback). Omit to use the Basecamp-profile exclusion heuristic.
    # @param timeout [Integer] request timeout in seconds
    # @return [DiscoveryResult]
    # @raise [Basecamp::UsageError] on a malformed caller +resource_origin+
    # @raise [DiscoverySelectionError] on any hard selection/validation failure
    def self.discover_from_resource(resource_origin, expected_issuer: nil, timeout: 10)
      # Origin-root validation of the *caller's* input is a usage error — let it
      # propagate as-is (not a soft fallback).
      origin = Basecamp::Security.require_origin_root!(resource_origin, "resource origin")

      # Hop 1: resource metadata. Any failure here is soft (before selection).
      resource = begin
        discover_protected_resource(origin, timeout: timeout)
      rescue Basecamp::UsageError
        raise
      rescue OauthError
        nil
      end

      if resource.nil?
        DiscoveryResult.fallback(FALLBACK_RESOURCE_DISCOVERY_FAILED)
      else
        select_and_bind(resource.authorization_servers || [], expected_issuer, timeout)
      end
    end

    def self.exchange_code(
      token_endpoint:, code:, redirect_uri:, client_id:,
      client_secret: nil, code_verifier: nil,
      use_legacy_format: false, timeout: 30
    )
      request = ExchangeRequest.new(
        token_endpoint: token_endpoint, code: code,
        redirect_uri: redirect_uri, client_id: client_id,
        client_secret: client_secret, code_verifier: code_verifier,
        use_legacy_format: use_legacy_format
      )
      Exchange.new(timeout: timeout).exchange(request)
    end

    def self.refresh_token(
      token_endpoint:, refresh_token:,
      client_id: nil, client_secret: nil,
      use_legacy_format: false, timeout: 30
    )
      request = RefreshRequest.new(
        token_endpoint: token_endpoint, refresh_token: refresh_token,
        client_id: client_id, client_secret: client_secret,
        use_legacy_format: use_legacy_format
      )
      Exchange.new(timeout: timeout).refresh(request)
    end

    def self.token_expired?(token, buffer_seconds = 60)
      token.expired?(buffer_seconds)
    end

    # Selects an issuer from the advertised set and — once a BC5 issuer is
    # committed — binds its AS metadata. From this point on, every failure is
    # fatal: no Launchpad request may be issued.
    #
    # @return [DiscoveryResult] +selected+, or +fallback(no_as_advertised)+ when
    #   the advertised set carries no non-Launchpad issuer.
    def self.select_and_bind(advertised, expected_issuer, timeout)
      selected_issuer =
        if expected_issuer
          select_expected(advertised, expected_issuer)
        else
          select_by_exclusion(advertised)
        end

      if selected_issuer.nil?
        # Valid resource metadata omits BC5 — soft fallback (before selection).
        DiscoveryResult.fallback(FALLBACK_NO_AS_ADVERTISED)
      else
        bind_issuer(selected_issuer, timeout)
      end
    end

    # Explicit, authoritative selection: the advertised member equal by
    # code-point, else a hard +expected_issuer_unavailable+.
    def self.select_expected(advertised, expected_issuer)
      match = advertised.find { |server| server == expected_issuer }
      if match.nil?
        raise DiscoverySelectionError.new(
          "expected_issuer_unavailable",
          "Expected issuer #{expected_issuer.inspect} is not advertised by the resource"
        )
      end

      match
    end

    # Basecamp-profile heuristic: identification by exclusion. Exactly one
    # non-Launchpad issuer selects it; two or more is a hard +ambiguous_issuers+
    # (never guess); zero returns +nil+ (caller yields the soft fallback).
    def self.select_by_exclusion(advertised)
      # Dedupe by code-point: the same non-Launchpad issuer advertised twice
      # (e.g. [BC5, BC5, Launchpad]) is ONE candidate, not an ambiguity.
      non_launchpad = advertised.reject { |server| launchpad_issuer?(server) }.uniq
      if non_launchpad.length >= 2
        raise DiscoverySelectionError.new(
          "ambiguous_issuers",
          "Multiple non-Launchpad issuers advertised; pass expected_issuer to disambiguate: #{non_launchpad.join(", ")}"
        )
      end

      non_launchpad.first
    end

    # BC5 is committed: validate the advertised issuer origin then fetch + bind
    # its AS metadata. Every failure here is a hard {DiscoverySelectionError}.
    def self.bind_issuer(selected_issuer, timeout)
      issuer_origin = begin
        Basecamp::Security.require_origin_root!(selected_issuer, "advertised issuer")
      rescue Basecamp::UsageError => e
        raise DiscoverySelectionError.new(
          "invalid_issuer_origin",
          "Advertised issuer #{selected_issuer.inspect} is not a valid origin root: #{e.message}"
        )
      end

      config = begin
        # Fetch from the normalized origin, but bind the AS metadata issuer to the
        # RAW advertised issuer by code-point: an AS whose issuer equals what the
        # resource advertised must not be rejected merely because normalization
        # dropped a trailing slash / default port before the bind. Uses the
        # internal binding path so the public Oauth.discover exposes no override.
        Discovery.new(timeout: timeout).discover_and_bind(issuer_origin, selected_issuer)
      rescue OauthError => e
        raise as_failure_error(issuer_origin, e)
      end

      DiscoveryResult.selected(config)
    end

    # Distinguishes an issuer-binding mismatch from a generic AS fetch failure by
    # the error's CLASS — a structured marker ({Discovery::IssuerBindingError})
    # raised by the binding check — not by matching its message text.
    def self.as_failure_error(issuer_origin, error)
      case error
      when Discovery::IssuerBindingError
        DiscoverySelectionError.new("issuer_mismatch", error.message)
      else
        DiscoverySelectionError.new(
          "as_fetch_failed",
          "AS metadata fetch failed for committed issuer #{issuer_origin.inspect}: #{error.message}"
        )
      end
    end

    # True when an advertised issuer string denotes the Launchpad origin. A
    # non-origin-root advertised value is (correctly) treated as non-Launchpad;
    # its later origin-root validation raises +invalid_issuer_origin+. Comparison
    # is origin-aware (scheme + normalized host + port) via {Security.same_origin?}
    # so a case-variant host (e.g. +Launchpad.37signals.COM+) still classifies as
    # Launchpad rather than slipping through as a BC5 issuer.
    def self.launchpad_issuer?(issuer)
      origin = Basecamp::Security.require_origin_root!(issuer, "issuer")
      Basecamp::Security.same_origin?(origin, launchpad_origin)
    rescue Basecamp::UsageError
      false
    end

    def self.launchpad_origin
      @launchpad_origin ||= Basecamp::Security.require_origin_root!(LAUNCHPAD_BASE_URL, "Launchpad base URL")
    end

    private_class_method :select_and_bind, :select_expected, :select_by_exclusion,
      :bind_issuer, :as_failure_error, :launchpad_issuer?, :launchpad_origin
  end
end
