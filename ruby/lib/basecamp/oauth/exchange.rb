# frozen_string_literal: true

require "faraday"
require "json"
require "timeout"
require "uri"

module Basecamp
  module Oauth
    # Handles OAuth 2 token exchange and refresh operations.
    #
    # Both operations POST credentials — the authorization code and client
    # secret, or the refresh token — to a token endpoint the caller names,
    # which may be one that discovery's metadata chose. The POST therefore
    # rides the same hardened transport discipline as the device flow
    # (SPEC §16 "Token-Endpoint Transport Policy"): redirects are refused
    # rather than followed, the whole request is wall-clock bounded, and the
    # body reads under the shared streaming cap.
    class Exchange
      # The redirect statuses a token endpoint response is refused for
      # (SPEC §16 "Token-Endpoint Transport Policy") — the same set the signed
      # download hop refuses (SPEC §14). 304 is deliberately absent: it is a
      # cache validator, not a redirect-with-Location, and stays on the
      # generic non-success path.
      REDIRECT_STATUSES = [ 301, 302, 303, 307, 308 ].freeze

      # Default per-request timeout in seconds — the shared credential-POST
      # default every SDK's token and device POSTs converge on (SPEC §16).
      DEFAULT_TIMEOUT = 30

      # Cap on a token response body (1 MiB), matching the device flow's and
      # the other SDKs' token-response bound.
      MAX_BODY_BYTES = 1 * 1024 * 1024

      # @param http_client [Faraday::Connection, nil] HTTP client. Nil selects
      #   the headers-first default transport ({Fetcher.stream_http}); an
      #   injected connection is refused unless its stack is verifiably
      #   redirect-free (adapter-only), and keeps the injected-client fidelity
      #   tier: status classification only after the (bounded) read completes,
      #   deadline enforced wall-clock around the call.
      # @param timeout [Numeric] Request timeout in seconds (default: 30).
      #   Invalid values and values beyond the shared 3600 s ceiling fall back
      #   to the default rather than disabling the bound.
      def initialize(http_client: nil, timeout: DEFAULT_TIMEOUT)
        # An injected connection is the caller's transport, but redirect
        # suppression is not negotiable on a credential POST: refuse a stack
        # that could follow (or rewrite) before any request is issued — the
        # same guard discovery, resource, and the device flow apply.
        Fetcher.ensure_redirects_suppressed!(http_client) if http_client
        @http_client = http_client
        @timeout = Fetcher.normalize_timeout(timeout, default: DEFAULT_TIMEOUT)
      end

      # Exchanges an authorization code for access and refresh tokens.
      #
      # Supports both standard OAuth 2 and Basecamp's Launchpad legacy format.
      # Use `use_legacy_format: true` for Launchpad compatibility.
      #
      # @param request [ExchangeRequest] Exchange request parameters
      # @return [Token] The token response
      # @raise [OauthError] on validation, network, or authentication errors
      #
      # @example Standard OAuth 2
      #   token = exchange.exchange(ExchangeRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     code: "auth_code_from_callback",
      #     redirect_uri: "https://myapp.com/callback",
      #     client_id: "my_client_id",
      #     client_secret: "my_client_secret"
      #   ))
      #
      # @example Launchpad legacy format
      #   token = exchange.exchange(ExchangeRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     code: "auth_code",
      #     redirect_uri: "https://myapp.com/callback",
      #     client_id: "my_client_id",
      #     client_secret: "my_client_secret",
      #     use_legacy_format: true
      #   ))
      def exchange(request)
        validate_exchange_request!(request)

        params = build_exchange_params(request)
        do_token_request(request.token_endpoint, params)
      end

      # Refreshes an access token using a refresh token.
      #
      # Supports both standard OAuth 2 and Basecamp's Launchpad legacy format.
      # Use `use_legacy_format: true` for Launchpad compatibility.
      #
      # @param request [RefreshRequest] Refresh request parameters
      # @return [Token] The new token response
      # @raise [OauthError] on validation, network, or authentication errors
      #
      # @example Standard OAuth 2
      #   new_token = exchange.refresh(RefreshRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     refresh_token: old_token.refresh_token,
      #     client_id: "my_client_id",
      #     client_secret: "my_client_secret"
      #   ))
      #
      # @example Launchpad legacy format
      #   new_token = exchange.refresh(RefreshRequest.new(
      #     token_endpoint: config.token_endpoint,
      #     refresh_token: old_token.refresh_token,
      #     use_legacy_format: true
      #   ))
      def refresh(request)
        validate_refresh_request!(request)

        params = build_refresh_params(request)
        do_token_request(request.token_endpoint, params)
      end

      private

      def validate_exchange_request!(request)
        raise OauthError.new("validation", "Token endpoint is required") if request.token_endpoint.to_s.empty?
        raise OauthError.new("validation", "Authorization code is required") if request.code.to_s.empty?
        raise OauthError.new("validation", "Redirect URI is required") if request.redirect_uri.to_s.empty?
        raise OauthError.new("validation", "Client ID is required") if request.client_id.to_s.empty?
      end

      def validate_refresh_request!(request)
        raise OauthError.new("validation", "Token endpoint is required") if request.token_endpoint.to_s.empty?
        raise OauthError.new("validation", "Refresh token is required") if request.refresh_token.to_s.empty?
      end

      def build_exchange_params(request)
        params = {}

        if request.use_legacy_format
          # Launchpad uses non-standard "type" parameter
          params["type"] = "web_server"
        else
          # Standard OAuth 2
          params["grant_type"] = "authorization_code"
        end

        params["code"] = request.code
        params["redirect_uri"] = request.redirect_uri
        params["client_id"] = request.client_id
        params["client_secret"] = request.client_secret if request.client_secret
        params["code_verifier"] = request.code_verifier if request.code_verifier

        params
      end

      def build_refresh_params(request)
        params = {}

        if request.use_legacy_format
          # Launchpad uses non-standard "type" parameter
          params["type"] = "refresh"
        else
          # Standard OAuth 2
          params["grant_type"] = "refresh_token"
        end

        params["refresh_token"] = request.refresh_token
        params["client_id"] = request.client_id if request.client_id
        params["client_secret"] = request.client_secret if request.client_secret
        # An empty string is truthy in Ruby but an empty resource is not a
        # binding — treat it as unset (omit) per the send-only-when-set
        # contract, matching Go/TS/Kotlin.
        params["resource"] = request.resource unless request.resource.to_s.empty?

        params
      end

      def do_token_request(token_endpoint, params)
        Basecamp::Security.require_https_unless_localhost!(token_endpoint, "token endpoint")

        status, body = post_form(
          token_endpoint, params,
          skip_status: ->(s) { REDIRECT_STATUSES.include?(s) }
        )

        # A refused redirect is a typed verdict classified by status alone —
        # its body (skipped above) is never a token, and the credential POST
        # is never re-issued toward Location (SPEC §16).
        if REDIRECT_STATUSES.include?(status)
          raise OauthError.new(
            "api_error",
            "redirect #{status} on the token endpoint is not followed",
            http_status: status
          )
        end

        parse_token_response(status, body)
      rescue Faraday::TimeoutError
        raise OauthError.new("network", "Token request timed out", retryable: true)
      rescue Faraday::Error => e
        raise OauthError.new("network", "Token request failed: #{e.message}", retryable: true)
      end

      # POSTs the token form and returns +[status, body]+, reading under the
      # same bounded/streaming cap as discovery and the device flow.
      #
      # With no injected client the POST runs on the headers-first
      # {Fetcher.stream_http} primitive: +skip_status+ classifies a redirect
      # by status at HEADER time (its body is never read, even one that
      # stalls forever), redirects are structurally never followed, and a
      # watchdog bounds the whole request — a stalled or byte-dripped header
      # phase included — at the timeout. An INJECTED Faraday connection keeps
      # the Faraday path below.
      def post_form(url, params, skip_status:)
        if @http_client.nil?
          Fetcher.stream_http(
            :post, url,
            headers: { "Content-Type" => "application/x-www-form-urlencoded", "Accept" => "application/json" },
            form: params, timeout: @timeout, max_body_bytes: MAX_BODY_BYTES, skip_status: skip_status
          )
        else
          post_form_injected(url, params, skip_status)
        end
      rescue Fetcher::SkipBody => e
        # The body was intentionally not drained (a redirect's body is never a
        # token) — classify by status upstream.
        [ e.status, "" ]
      rescue Fetcher::BodyTooLarge
        raise OauthError.new("api_error", "Token response exceeds size cap")
      rescue Fetcher::ReadDeadlineExceeded
        # A slow-drip read is a transport timeout, not an api_error — surface
        # as the Faraday timeout the caller's rescue classifies.
        raise Faraday::TimeoutError, "Token request read exceeded the timeout deadline"
      end

      # Injected-client (Faraday) lane — the injected-client fidelity tier
      # (SPEC §16): the same invariants as the default transport (suppressed
      # redirects, bounded body, whole-request wall clock), with buffered
      # classification. +req.options.timeout+ below bounds only each socket
      # read and resets on every +on_data+ chunk, so a slow-drip peer could
      # otherwise hold the credential POST open past the timeout while
      # staying under the cap — the monotonic deadline bounds the WHOLE
      # request, and +Timeout.timeout+ enforces it through a stalled or
      # dripped HEADER phase, where +on_data+ (a body callback) never runs.
      def post_form_injected(url, params, skip_status)
        deadline = Fetcher.monotonic_now + @timeout
        chunks, on_data = Fetcher.bounded_reader(MAX_BODY_BYTES, deadline: deadline, skip_status: skip_status)
        # The window is the REMAINING budget, not a fresh timeout: time spent
        # before dispatch already counts against the deadline, so the request
        # can never run past it.
        remaining = deadline - Fetcher.monotonic_now
        raise Faraday::TimeoutError, "request budget exhausted before dispatch" if remaining <= 0

        response = Timeout.timeout(remaining, Faraday::TimeoutError) do
          @http_client.post(url) do |req|
            req.headers["Content-Type"] = "application/x-www-form-urlencoded"
            req.headers["Accept"] = "application/json"
            req.body = URI.encode_www_form(params)
            req.options.timeout = @timeout
            req.options.open_timeout = @timeout
            req.options.on_data = on_data
          end
        end

        # Status-first backstop on the completed response: the +on_data+
        # SkipBody fast-path only fires when the adapter streams AND passes
        # +env+ (Faraday >= 2.5). A buffered adapter that ignores +on_data+,
        # an older Faraday (2.0–2.4) that omits +env+, or a header-only
        # response reaches here with the redirect body un-skipped — re-apply
        # +skip_status+ to the final status so a redirect is classified by
        # status for every client shape, never buffered into a size-cap error.
        # A definitive completed status outranks the deadline re-check below;
        # everything else completing past the deadline is refused as the same
        # transport-shaped timeout (Timeout.timeout's interrupt can land late).
        if skip_status.call(response.status)
          [ response.status, "" ]
        elsif Fetcher.monotonic_now > deadline
          raise Faraday::TimeoutError, "response completed after the deadline"
        else
          body =
            if chunks.empty?
              raw = response.body.to_s
              raise Fetcher::BodyTooLarge if raw.bytesize > MAX_BODY_BYTES

              raw
            else
              chunks.join
            end

          [ response.status, body.dup.force_encoding(Encoding::UTF_8) ]
        end
      end

      def parse_token_response(status, body)
        data = JSON.parse(body)

        handle_error_response(status, data) unless (200..299).cover?(status)

        unless data["access_token"].is_a?(String) && !data["access_token"].empty?
          raise OauthError.new(
            "api_error", "Token response missing or non-string access_token",
            http_status: status
          )
        end

        # resource: absent and JSON null are unset; when present it must be a
        # non-empty string (SPEC §16) — an empty binding is not a binding.
        resource = data["resource"]
        unless resource.nil? || (resource.is_a?(String) && !resource.empty?)
          raise OauthError.new(
            "api_error",
            "Token response resource must be a non-empty string when present",
            http_status: status
          )
        end

        # token_type: absent/JSON-null defaults to Bearer; present must be a
        # non-empty String ("" is truthy in Ruby, so || alone would admit it) —
        # matching the device-flow parser and SPEC §16.
        token_type = data["token_type"]
        unless token_type.nil? || (token_type.is_a?(String) && !token_type.empty?)
          raise OauthError.new(
            "api_error",
            "Token response token_type must be a non-empty string when present",
            http_status: status
          )
        end

        Token.new(
          access_token: data["access_token"],
          refresh_token: data["refresh_token"],
          token_type: token_type || "Bearer",
          expires_in: data["expires_in"],
          scope: data["scope"],
          resource: resource
        )
      rescue JSON::ParserError
        # A token response that fails to parse may still contain credential
        # material (a syntactically-broken body carrying an access_token) —
        # never echo ANY of it into an error message, where it would reach
        # logs and exception telemetry. The status is diagnosis enough.
        # cause: nil — the parser error's message embeds the offending input,
        # so the implicit cause chain would leak it via full_message.
        raise OauthError.new(
          "api_error",
          "Failed to parse token response",
          http_status: status
        ), cause: nil
      end

      def handle_error_response(status, data)
        error_msg = Basecamp::Security.truncate(data["error_description"] || data["error"] || "Token request failed")

        if status == 401 || data["error"] == "invalid_grant"
          raise OauthError.new(
            "auth",
            error_msg,
            http_status: status,
            hint: "The authorization code or refresh token may be invalid or expired"
          )
        end

        raise OauthError.new("api_error", error_msg, http_status: status)
      end
    end
  end
end
