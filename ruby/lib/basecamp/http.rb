# frozen_string_literal: true

require "faraday"
require "json"
require "time"
require "uri"

module Basecamp
  # HTTP client layer with retry, backoff, and caching support.
  # This is an internal class used by Client; you typically don't use it directly.
  class Http
    # Default User-Agent header
    USER_AGENT = "basecamp-sdk-ruby/#{VERSION} (api:#{API_VERSION})".freeze

    # Normalizes Person-shaped objects in parsed JSON.
    # For objects with personable_type and a string id:
    # - Numeric strings: coerced to Integer, no system_label
    # - Non-numeric sentinels (e.g. "basecamp"): id becomes 0, system_label preserves original
    def self.normalize_person_ids(obj)
      case obj
      when Hash
        if obj.key?("personable_type") && obj["id"].is_a?(String)
          raw_id = obj["id"]
          numeric = Integer(raw_id, exception: false)
          if numeric
            obj["id"] = numeric
          else
            obj["system_label"] = raw_id
            obj["id"] = 0
          end
        end
        obj.each_value { |v| normalize_person_ids(v) }
      when Array
        obj.each { |item| normalize_person_ids(item) }
      end
    end

    # @param config [Config] configuration settings
    # @param token_provider [TokenProvider, nil] OAuth token provider (deprecated, use auth_strategy)
    # @param auth_strategy [AuthStrategy, nil] authentication strategy
    # @param hooks [Hooks] observability hooks
    def initialize(config:, token_provider: nil, auth_strategy: nil, hooks: nil)
      @config = config
      @auth_strategy = auth_strategy || BearerAuth.new(token_provider)
      @token_provider = token_provider || (@auth_strategy.is_a?(BearerAuth) ? @auth_strategy.token_provider : nil)
      @hooks = hooks || NoopHooks.new
      @faraday = build_faraday_client
    end

    # @return [String] the configured base URL
    def base_url
      @config.base_url
    end

    # Performs a GET request.
    # @param path [String] URL path
    # @param params [Hash] query parameters
    # @param operation [String, nil] canonical operation ID; when given, the
    #   operation's declared retry block governs attempts (max as a ceiling on
    #   the configured cap) and retryable statuses (the declared retryOn set)
    # @return [Response]
    def get(path, params: {}, operation: nil)
      request(:get, path, params: params, operation: operation)
    end

    # Performs a GET request to an absolute URL.
    # Used for endpoints not on the base API.
    #
    # This is the PUBLIC, general path and it credentials cross-origin for ONE
    # destination only: the exact Launchpad authorization URL
    # ({Basecamp::Security::LAUNCHPAD_AUTHORIZATION_URL}). Every other foreign
    # origin — including an endpoint-shaped URL such as
    # +https://evil.example/authorization.json+ — trips the same-origin guard, so
    # the bearer token only ever reaches Launchpad, the configured base URL, or
    # localhost. There is deliberately NO raw-string trusted-origin parameter: a
    # syntactically valid origin does not prove discovery provenance, so the ONE
    # legitimate cross-origin discovery destination goes through the narrow
    # {#get_authorization_document}, which derives its issuer from internal
    # discovery of the configured base URL rather than any caller argument.
    #
    # @param url [String] absolute URL
    # @param params [Hash] query parameters
    # @return [Response]
    def get_absolute(url, params: {})
      Security.require_https_unless_localhost!(url, "absolute URL")

      allow_cross_origin = url == Security::LAUNCHPAD_AUTHORIZATION_URL
      request(:get, url, params: params, allow_cross_origin: allow_cross_origin)
    end

    # Fetches the credentialed authorization document (the fixed +authorization.json+
    # path). This is the ONE sanctioned cross-origin credential path besides
    # Launchpad, and the origin that receives the bearer token is NOT
    # caller-supplied.
    #
    # The issuer is derived HERE by running resource-first discovery (SPEC.md §16)
    # against this client's OWN configured base URL, then binding to whatever
    # issuer discovery selects and validates (RFC 8414 issuer binding). A soft
    # fallback fetches Launchpad's fixed URL; a hard discovery failure raises. The
    # request URL is CONSTRUCTED from the discovered issuer origin + the fixed path
    # (string concatenation, never URL re-parsing). Because no caller-supplied
    # config, origin, or path reaches this method, there is no public API through
    # which a forged issuer could redirect the credential to a foreign host —
    # discovery provenance is structural, not a claim about a passed-in object.
    #
    # @return [Response]
    # @raise [Oauth::DiscoverySelectionError] on a hard discovery failure
    # @raise [Basecamp::UsageError] when the discovered issuer is not an origin root
    def get_authorization_document
      result = Oauth.discover_from_resource(@config.base_url)
      if result.selected?
        issuer_origin = Security.require_origin_root!(result.issuer, "selected issuer origin")
        request(:get, "#{issuer_origin}/authorization.json", allow_cross_origin: true)
      else
        get_absolute(Security::LAUNCHPAD_AUTHORIZATION_URL)
      end
    end

    # Performs a POST request.
    # @param path [String] URL path
    # @param body [Hash, nil] request body
    # @return [Response]
    def post(path, body: nil)
      request(:post, path, body: body)
    end

    # Performs a PUT request.
    # @param path [String] URL path
    # @param body [Hash, nil] request body
    # @return [Response]
    def put(path, body: nil)
      request(:put, path, body: body)
    end

    # Performs a DELETE request.
    # @param path [String] URL path
    # @return [Response]
    def delete(path)
      request(:delete, path)
    end

    # Performs a POST request with raw binary data.
    # Used for file uploads (attachments).
    # @param path [String] URL path
    # @param body [String, IO] raw binary data
    # @param content_type [String] MIME content type
    # @return [Response]
    def post_raw(path, body:, content_type:)
      url = build_url(path)
      single_request_raw(:post, url, body: body, content_type: content_type, attempt: 1)
    end

    # Performs a PUT request with raw binary data.
    # Used for multipart uploads (e.g., account logo).
    # @param path [String] URL path
    # @param body [String, IO] raw binary data
    # @param content_type [String] MIME content type
    # @return [Response]
    def put_raw(path, body:, content_type:)
      url = build_url(path)
      single_request_raw(:put, url, body: body, content_type: content_type, attempt: 1)
    end

    # SPEC §14's declared hop-1 retry set for downloads: a carve-out from the
    # ungoverned GET taxonomy (which retries all retryable 5xx, including 500).
    # Authoritative in BOTH directions, like an operation's declared retryOn.
    DOWNLOAD_RETRY_ON = [ 429, 502, 503, 504 ].freeze

    # Performs the authenticated hop-1 GET for the download flow (SPEC §14).
    #
    # Retries network errors plus the declared {DOWNLOAD_RETRY_ON} statuses —
    # never 500 — under the public max_retries total-attempt cap, which is
    # floored at one attempt on every path, not just this one (+max_retries: 0+
    # still sends one request). DownloadURL has no behavior-model entry, so the
    # policy is passed directly rather than looked up by operation.
    # @param url [String] absolute URL
    # @return [Response]
    def get_download(url)
      request_with_retry(:get, url, retry_on: DOWNLOAD_RETRY_ON, accept: nil)
    end

    # Fetches all pages of a paginated resource.
    # The first page is fetched eagerly, so pagination metadata (and any
    # page-1 error) surfaces at call time; later pages are fetched lazily
    # as enumeration demands them.
    # @param path [String] initial URL path
    # @param params [Hash] query parameters
    # @param max_items [Integer, nil] cap on items yielded across pages;
    #   nil or non-positive means no cap
    # @yield [Hash] each item from the response
    # @return [ListEnumerator] metadata-carrying lazy enumerator
    def paginate(path, params: {}, operation: nil, max_items: nil, &block)
      enum = paginated_enumerator(path, params: params, operation: operation, max_items: max_items)
      block ? enum.each(&block) : enum
    end

    # Fetches all pages of a paginated resource, extracting items from a key.
    # Use this for endpoints that return objects like { "events": [...] }.
    # The first page is fetched eagerly; later pages are fetched lazily.
    # @param path [String] initial URL path
    # @param key [String] the key containing the array of items
    # @param params [Hash] query parameters
    # @param max_items [Integer, nil] cap on items yielded across pages;
    #   nil or non-positive means no cap
    # @yield [Hash] each item from the response
    # @return [ListEnumerator] metadata-carrying lazy enumerator
    def paginate_key(path, key:, params: {}, operation: nil, max_items: nil, &block)
      enum = paginated_enumerator(path, key: key, params: params, operation: operation, max_items: max_items)
      block ? enum.each(&block) : enum
    end

    # Fetches a wrapped paginated resource, returning wrapper fields + lazy paginated items.
    # Use this for endpoints that return {wrapper_field: ..., key: [items]} on every page.
    # @param path [String] initial URL path
    # @param key [String] the key containing the array of paginated items
    # @param params [Hash] query parameters
    # @param max_items [Integer, nil] cap on items yielded across pages;
    #   nil or non-positive means no cap
    # @return [Hash] wrapper fields merged with key => ListEnumerator of all items
    def paginate_wrapped(path, key:, params: {}, operation: nil, max_items: nil)
      wrapper = nil
      events = paginated_enumerator(path, key: key, params: params, operation: operation, \
        max_items: max_items) do |first_data|
        wrapper = first_data.reject { |k, _| k == key }
      end
      wrapper.merge(key => events)
    end

    private

    # Shared paginator core behind paginate/paginate_key/paginate_wrapped.
    # Eagerly fetches page 1 (so ListMeta#total_count is available
    # immediately), then returns a ListEnumerator that yields the captured
    # first page and follows Link headers lazily. When key is nil each page
    # body is a bare item array; otherwise items live under key. Yields the
    # parsed first-page body when a block is given (paginate_wrapped uses it
    # to capture the wrapper fields).
    #
    # meta.truncated is finalized during enumeration: set when max_items
    # drops items or leaves a next Link unfetched, or when the max_pages
    # safety cap stops with a next Link still present. A cap landing exactly
    # on the final item of the last page is not truncation.
    #
    # Re-enumerating restarts pagination from the base URL: the eagerly
    # fetched first page is served from memory on the first pass only, and
    # later passes refetch every page, so each pass is a consistent
    # snapshot rather than a hybrid of captured and current data.
    def paginated_enumerator(path, params:, operation:, max_items:, key: nil)
      max_items = nil if max_items && max_items <= 0
      single_page = single_page_selected?(params)
      base_url = build_url(path)

      @hooks.on_paginate(base_url, 1)
      first_response = get(base_url, params: params, operation: operation)
      first_data = parse_page(first_response, page: 1)
      yield first_data if block_given?
      first_items = extract_page_items(first_data, key: key, page: 1)

      meta = ListMeta.new(total_count: parse_total_count(first_response.headers))
      first_pass = true

      ListEnumerator.new(meta) do |yielder|
        if first_pass
          first_pass = false
          response = first_response
          items = first_items
        else
          @hooks.on_paginate(base_url, 1)
          response = get(base_url, params: params, operation: operation)
          items = extract_page_items(parse_page(response, page: 1), key: key, page: 1)
          meta.restart!(total_count: parse_total_count(response.headers))
        end

        yielded = 0
        page = 1
        url = base_url

        loop do
          next_link = parse_next_link(response.headers["Link"])

          # A pinned page is the whole answer (SPEC §8): this pass yields that
          # page and stops, so a next link we deliberately do not follow is
          # what makes the result truncated. Recorded before the yields for the
          # same reason as the capping case below.
          meta.mark_truncated! if single_page && next_link

          capped = false
          items.each_with_index do |item, index|
            yielded += 1
            capped = max_items && yielded >= max_items
            # Truncation is recorded before the capping yield: consumers like
            # first/take cancel the producer at that yield, and the metadata
            # must already be accurate once the capped item is delivered.
            meta.mark_truncated! if capped && (index < items.size - 1 || next_link)
            yielder << item
            break if capped
          end
          break if capped
          break if single_page
          break if next_link.nil?

          next_url = Security.resolve_url(url, next_link)
          unless Security.same_origin?(next_url, base_url)
            raise Basecamp::ApiError.new(
              "Pagination Link header points to different origin: #{Security.truncate(next_url)}"
            )
          end

          if page >= @config.max_pages
            meta.mark_truncated!
            break
          end

          page += 1
          @hooks.on_paginate(next_url, page)
          response = get(next_url, operation: operation)
          items = extract_page_items(parse_page(response, page: page), key: key, page: page)
          url = next_url
        end
      end
    end

    # Reports whether the outgoing query pins a single page (SPEC §8).
    #
    # The query parameters are the authority: `page` reaches the wire only when
    # the caller passed it, so reading it back here needs no extra plumbing
    # through every generated service method. `to_i` rather than an Integer
    # check so a string-typed "3" selects page 3 instead of silently walking
    # the collection from there.
    # @param params [Hash, nil] the outgoing query parameters
    # @return [Boolean]
    def single_page_selected?(params)
      page = params && (params[:page] || params["page"])
      page.respond_to?(:to_i) && page.to_i.positive?
    end

    # Parses a pagination page body: size check, JSON parse, and person-ID
    # normalization, with page-numbered error context.
    def parse_page(response, page:)
      Security.check_body_size!(response.body, Security::MAX_RESPONSE_BODY_BYTES)
      data = JSON.parse(response.body)
      Http.normalize_person_ids(data)
      data
    rescue JSON::ParserError => e
      raise Basecamp::ApiError.new("Failed to parse paginated response (page #{page}): #{Security.truncate(e.message)}")
    end

    # Extracts the item array from a parsed page body: the body itself for
    # bare-array pagination, or the named key's array otherwise.
    def extract_page_items(data, key:, page:)
      if key.nil?
        data
      else
        unless data.key?(key)
          warn "[Basecamp SDK] paginate: expected key '#{key}' not found in response (page #{page})"
        end
        data[key] || []
      end
    end

    # Parses the X-Total-Count header, returning 0 when absent or malformed.
    def parse_total_count(headers)
      Integer(headers["X-Total-Count"] || headers["x-total-count"], 10)
    rescue ArgumentError, TypeError
      0
    end

    def build_faraday_client
      Faraday.new(url: @config.base_url) do |f|
        f.options.timeout = @config.timeout
        f.options.open_timeout = 10
        f.request :json
        f.response :raise_error
        f.adapter Faraday.default_adapter
      end
    end

    def request(method, path, params: {}, body: nil, allow_cross_origin: false, operation: nil)
      url = build_url(path, allow_cross_origin: allow_cross_origin)

      # Mutations don't retry on 429/5xx to avoid duplicating data
      if method == :get
        request_with_retry(method, url, params: params, allow_cross_origin: allow_cross_origin, operation: operation)
      else
        single_request(method, url, params: params, body: body, attempt: 1, allow_cross_origin: allow_cross_origin)
      end
    end

    def request_with_retry(method, url, params: {}, allow_cross_origin: false, operation: nil, retry_on: nil,
      accept: "application/json")
      op_retry = operation && Http.operation_retry(operation)
      # The cap is floored at one attempt on every path: whether a request
      # reaches the wire at all must not depend on whether the operation
      # carries a declared retry block (#532). A declared operation ceiling
      # still clamps the floored cap downward.
      caller_cap = [ @config.max_retries, 1 ].max
      max_attempts = op_retry ? [ caller_cap, op_retry.fetch("maxAttempts") ].min : caller_cap
      attempt = 0
      refreshed_once = false
      last_error = nil

      loop do
        attempt += 1
        break if attempt > max_attempts

        # Each rescue only CLASSIFIES the attempt; the retry side effects run
        # below, outside every handler. An exception raised inside a rescue
        # clause bypasses that begin's sibling rescues, so refreshing there
        # would let a NetworkError from the token endpoint escape the loop
        # with budget still on the table.
        error = nil
        stale_auth = nil

        begin
          return single_request(method, url, params: params, body: nil, attempt: attempt,
            allow_cross_origin: allow_cross_origin, accept: accept, refresh_replay: false)
        rescue Basecamp::AuthError => e
          # SPEC §4: the refresh replay is a request on the wire, so it spends
          # an attempt from THIS budget rather than an uncounted one inside
          # single_request. max_retries is a total attempt count (#461), and a
          # cap of one means one request whatever would have caused the second.
          #
          # The budget is checked BEFORE refresh so a rotation is never burned
          # on an attempt the loop has no room to make.
          raise e if refreshed_once || e.http_status != 401 || attempt >= max_attempts

          stale_auth = e
        rescue Basecamp::RateLimitError, Basecamp::NetworkError, Basecamp::ApiError => e
          error = e
        end

        if stale_auth
          # SPEC §4 tracks refresh with an "attempted" boolean, not a
          # "succeeded" one, so mark it BEFORE invoking the provider: a refresh
          # that raises still counts as the one attempt this request gets.
          # Otherwise a transient token-endpoint failure lets the NEXT 401 in
          # the same request call refresh again — and if the first call reached
          # the server and rotated the token before its response was lost, the
          # second spends a refresh token that is already dead.
          refreshed_once = true

          begin
            refreshed = @token_provider&.refreshable? && @token_provider.refresh
          rescue Basecamp::RateLimitError, Basecamp::NetworkError, Basecamp::ApiError => e
            # A token endpoint that times out is a transient failure of this
            # attempt, not a terminal auth fault: it retries under the same
            # budget, exactly as it did when the replay lived in single_request.
            error = e
          else
            raise stale_auth unless refreshed

            # No backoff: the token is fresh, and the server never asked us to
            # wait. The replay still costs the attempt counted above.
            next
          end
        end

        if error
          raise error unless retry_eligible?(error, op_retry, retry_on)

          last_error = error

          # Don't sleep if this was the last attempt
          break if attempt >= max_attempts

          delay = calculate_delay(attempt, error.retry_after)

          @hooks.on_retry(RequestInfo.new(method: method.to_s.upcase, url: url, attempt: attempt), attempt + 1, error,
                          delay)
          sleep(delay)
        end
      end

      noun = max_attempts == 1 ? "attempt" : "attempts"
      raise last_error || Basecamp::ApiError.new("Request failed after #{max_attempts} #{noun}")
    end

    # For a governed request — an operation's declared retry block, or an
    # explicit declared set such as the download flow's — a status-bearing
    # error retries exactly when the declared retryOn set says so: the error
    # taxonomy's retryable flag neither widens the set (500 is retryable in
    # errors.rb but not declared) nor vetoes it. Status-less errors (network
    # failures) and all ungoverned traffic keep the taxonomy's judgment.
    def retry_eligible?(error, op_retry, retry_on)
      declared = retry_on || op_retry&.fetch("retryOn")
      if declared && error.http_status
        declared.include?(error.http_status)
      else
        error.retryable?
      end
    end

    # Memoized per-operation retry metadata, keyed by canonical operation ID.
    # Benign-race memoization: concurrent first loads compute identical values.
    def self.operation_retry(operation)
      @operation_metadata ||= JSON.parse(
        File.read(File.join(__dir__, "generated", "metadata.json"))
      ).fetch("operations").freeze
      @operation_metadata.dig(operation, "retry")
    end

    def single_request(method, url, params:, body:, attempt:, retry_count: 0, allow_cross_origin: false,
      accept: "application/json", refresh_replay: true)
      assert_credential_origin!(url, allow_cross_origin)
      info = RequestInfo.new(method: method.to_s.upcase, url: url, attempt: attempt)
      @hooks.on_request_start(info)

      start_time = Process.clock_gettime(Process::CLOCK_MONOTONIC)

      begin
        response = @faraday.run_request(method, url, body, request_headers(accept: accept)) do |req|
          req.params.merge!(params) if params.any?
        end

        duration = Process.clock_gettime(Process::CLOCK_MONOTONIC) - start_time
        result = RequestResult.new(status_code: response.status, duration: duration)
        @hooks.on_request_end(info, result)

        Response.new(
          body: response.body,
          status: response.status,
          headers: response.headers
        )
      rescue Faraday::ServerError, Faraday::ClientError => e
        duration = Process.clock_gettime(Process::CLOCK_MONOTONIC) - start_time
        error = handle_error(e, refresh_on_401: refresh_replay)
        result = RequestResult.new(
          status_code: e.response&.dig(:status),
          duration: duration,
          error: error,
          retry_after: error.respond_to?(:retry_after) ? error.retry_after : nil
        )
        @hooks.on_request_end(info, result)

        # 401 replay for callers that come here directly (mutations), which
        # have no retry loop to own it. request_with_retry passes
        # refresh_replay: false and replays from the loop so the extra request
        # draws from the attempt budget (SPEC §4).
        if refresh_replay && error.is_a?(Basecamp::AuthError) && error.http_status == 401 && retry_count < 1 \
            && @token_refreshed
          @token_refreshed = false
          return single_request(method, url, params: params, body: body, attempt: attempt, retry_count: retry_count + 1,
            allow_cross_origin: allow_cross_origin, accept: accept, refresh_replay: refresh_replay)
        end

        raise error
      rescue Faraday::Error => e
        duration = Process.clock_gettime(Process::CLOCK_MONOTONIC) - start_time
        error = Basecamp::NetworkError.new("Connection failed", cause: e)
        result = RequestResult.new(duration: duration, error: error)
        @hooks.on_request_end(info, result)
        raise error
      end
    end

    # accept: nil is the binary-download carve-out (SPEC §14): hop 1 sends
    # Authorization and User-Agent only, because it is not a JSON API call.
    # Every other caller keeps the JSON Accept.
    def request_headers(accept: "application/json")
      headers = { "User-Agent" => USER_AGENT }
      headers["Accept"] = accept if accept
      @auth_strategy.authenticate(headers)
      headers
    end

    def single_request_raw(method, url, body:, content_type:, attempt:)
      assert_credential_origin!(url, false)
      info = RequestInfo.new(method: method.to_s.upcase, url: url, attempt: attempt)
      @hooks.on_request_start(info)

      start_time = Process.clock_gettime(Process::CLOCK_MONOTONIC)

      begin
        headers = request_headers.merge("Content-Type" => content_type)
        response = @faraday.run_request(method, url, body, headers)

        duration = Process.clock_gettime(Process::CLOCK_MONOTONIC) - start_time
        result = RequestResult.new(status_code: response.status, duration: duration)
        @hooks.on_request_end(info, result)

        Response.new(
          body: response.body,
          status: response.status,
          headers: response.headers
        )
      rescue Faraday::ServerError, Faraday::ClientError => e
        duration = Process.clock_gettime(Process::CLOCK_MONOTONIC) - start_time
        error = handle_error(e)
        result = RequestResult.new(
          status_code: e.response&.dig(:status),
          duration: duration,
          error: error,
          retry_after: error.respond_to?(:retry_after) ? error.retry_after : nil
        )
        @hooks.on_request_end(info, result)
        raise error
      rescue Faraday::Error => e
        duration = Process.clock_gettime(Process::CLOCK_MONOTONIC) - start_time
        error = Basecamp::NetworkError.new("Connection failed", cause: e)
        result = RequestResult.new(duration: duration, error: error)
        @hooks.on_request_end(info, result)
        raise error
      end
    end

    # refresh_on_401: false leaves the token alone — the caller (request_with_retry)
    # owns the refresh so it can gate it on the attempt budget before spending a
    # rotation. Classifying an error should not rotate credentials as a side effect.
    def handle_error(error, refresh_on_401: true)
      status = error.response&.dig(:status)
      body = error.response&.dig(:body)
      headers = error.response&.dig(:headers) || {}

      retry_after = parse_retry_after(headers["Retry-After"] || headers["retry-after"])
      request_id = headers["X-Request-Id"] || headers["x-request-id"]

      err = case status
      when 401
        # Try token refresh; flag for caller to retry
        @token_refreshed = refresh_on_401 && @token_provider&.refreshable? && @token_provider.refresh
        Basecamp::AuthError.new("Authentication failed")
      when 403
        Basecamp::ForbiddenError.new("Access denied")
      when 404
        message = Security.truncate(Basecamp.parse_error_message(body) || "Not found")
        Basecamp::NotFoundError.new(message: message)
      when 429
        Basecamp::RateLimitError.new(retry_after: retry_after)
      when 400, 422
        field_errors = Basecamp.parse_field_errors(body)
        message = Security.truncate(
          Basecamp.compose_validation_message(Basecamp.parse_error_message(body), field_errors) || "Validation failed"
        )
        Basecamp::ValidationError.new(message, http_status: status, field_errors: field_errors)
      when 500
        Basecamp::ApiError.new("Server error (500)", http_status: 500, retryable: true)
      when 502, 503, 504
        Basecamp::ApiError.new("Gateway error (#{status})", http_status: status, retryable: true)
      else
        message = Security.truncate(Basecamp.parse_error_message(body) || "Request failed (HTTP #{status})")
        Basecamp::ApiError.from_status(status || 0, message)
      end

      err.instance_variable_set(:@request_id, request_id) if request_id
      err
    end

    def build_url(path, allow_cross_origin: false)
      # Schemes are case-insensitive (RFC 3986): detect absolute URLs on a
      # lowercased copy so HTTPS://... is not mis-joined onto the base URL.
      lower_path = path.downcase
      if lower_path.start_with?("https://")
        return path if allow_cross_origin
        return path if Security.localhost?(path) || Security.same_origin?(path, @config.base_url)

        raise Basecamp::UsageError.new("URL origin does not match configured base URL: #{Security.truncate(path)}")
      elsif lower_path.start_with?("http://")
        # Localhost may use plain HTTP for local development; every other host
        # must use HTTPS.
        return path if Security.localhost?(path)

        raise Basecamp::UsageError.new("URL must use HTTPS: #{Security.truncate(path)}")
      end

      path = "/#{path}" unless path.start_with?("/")
      "#{@config.base_url}#{path}"
    end

    # Attach-point backstop: refuse to attach credentials to a foreign origin
    # before the auth strategy adds the bearer token. Localhost is carved out
    # for dev/test. allow_cross_origin is granted only by get_absolute (for the
    # trusted Launchpad authorization endpoint) or get_authorization_document (for
    # a URL constructed against an issuer that internal discovery selected and
    # validated); every other absolute URL must be same-origin with the base URL.
    def assert_credential_origin!(url, allow_cross_origin)
      return if allow_cross_origin
      return if Security.localhost?(url) || Security.same_origin?(url, @config.base_url)

      raise Basecamp::UsageError.new(
        "Refusing to send credentials to a different origin than base URL: #{Security.truncate(url)}"
      )
    end

    def calculate_delay(attempt, server_retry_after)
      # Retry-After is server-directed and exempt from the ceiling (SPEC §7);
      # only the locally-computed term saturates.
      return server_retry_after if server_retry_after&.positive?

      # Exponential backoff: min(base_delay * 2^(attempt-1), ceiling) + jitter
      base = Config.saturating_backoff(@config.base_delay, attempt)
      jitter = rand * @config.max_jitter
      base + jitter
    end

    def parse_retry_after(value)
      return nil if value.nil? || value.empty?

      # Try parsing as seconds (integer)
      seconds = Integer(value, exception: false)
      return seconds if seconds&.positive?

      # Try parsing as HTTP-date
      begin
        date = Time.httpdate(value)
        diff = (date - Time.now).to_i
        return diff if diff.positive?
      rescue ArgumentError
        # Not a valid HTTP-date
      end

      nil
    end

    # Returns the contents of the first non-empty <...> pair.
    #
    # This is the leftmost-match semantics of /<([^>]+)>/ without the regex.
    # CodeQL flags that pattern as polynomial-redos (alert 48): every "<" is
    # retried as a start position, and each scan runs to the end of the string.
    # Onigmo does not actually realize the blowup — measured linear to 3.2M
    # characters — so this is a consistency and correctness change here rather
    # than a remediation. Searching for ">" from *after* the "<" is linear by
    # construction, and it is what the other five SDKs now do.
    #
    # The scan runs over a binary view, and that is the whole point in Ruby.
    # Neither of the obvious spellings is both fast and total:
    #
    #   String#index(str, offset) takes a CHARACTER offset, and on a string
    #   whose coderange is not CR_7BIT Ruby walks from the start to convert it —
    #   O(cursor) per call. The skip loop below advances the cursor once per
    #   empty <>, so character indexing is quadratic on any header carrying a
    #   non-ASCII byte — seconds, against 5ms for the regex it replaces, on the
    #   input measured below.
    #
    #   String#byteindex(str, offset) is O(1), but it RAISES IndexError when the
    #   offset does not land on a character boundary — and on malformed UTF-8 it
    #   does. In "\xC2<\x80>" the "\xC2" is a two-byte lead, so byte 2, the
    #   offset just past the "<", is mid-character by Ruby's reckoning and
    #   byteindex(">", 2) blows up. The header is attacker-influenced, so that
    #   is a reachable crash, not a curiosity.
    #
    # ASCII-8BIT has no multi-byte characters and therefore no boundaries to
    # violate: every offset into part.b is O(1) AND legal, whatever the bytes
    # say. force_encoding hands the caller's encoding back on the way out, so
    # UTF-8 in gives UTF-8 out and binary in gives binary out. Measured on
    # "é" + "<>" * n: 2.7ms / 8.2ms / 31.7ms at n = 10k / 40k / 160k, against
    # 64.7ms / 988.1ms / 15,459.7ms for the character version — still linear.
    # Parity with the character version is exact: 0 mismatches and 0 raises over
    # 400,000 random byte strings drawn from <, >, a, /, \xC2, \x80, \xFF, \xC3,
    # \xA9 and force-encoded to UTF-8. It has to be, because "<" and ">" are
    # ASCII and UTF-8 is self-synchronising, so neither can match inside a
    # multi-byte sequence.
    #
    # The other five SDKs index by byte (Go), UTF-16 code unit
    # (TypeScript/Kotlin), flat code point (Python) or an O(1) native
    # String.Index (Swift) — all already linear; only Ruby had to ask for it.
    #
    # An empty <> is skipped rather than returned, because [^>]+ requires at
    # least one character: the regex would move on to the next "<", and so does
    # this.
    def extract_angle_bracketed(part)
      bytes = part.b
      result = nil
      cursor = 0

      while cursor && result.nil?
        start = bytes.index("<", cursor)
        finish = start && bytes.index(">", start + 1)

        if finish.nil?
          cursor = nil
        elsif finish > start + 1
          result = bytes[(start + 1)...finish]
        else
          cursor = start + 1
        end
      end

      result && result.force_encoding(part.encoding)
    end

    def parse_next_link(link_header)
      next_url = nil

      unless link_header.nil? || link_header.empty?
        # Split a binary view for the same reason extract_angle_bracketed scans
        # one: String#split and String#strip raise ArgumentError on a broken
        # coderange, so a header carrying malformed UTF-8 crashed here, one
        # frame above the extractor — fixing only the extractor would leave the
        # header just as fatal. ASCII-8BIT has no invalid sequences, so the
        # split is total, and strip and include? behave identically against the
        # ASCII-only literals used below. The extracted URL is retagged with the
        # caller's encoding so the .b round trip is invisible to well-formed
        # input.
        link_header.b.split(",").each do |part|
          part = part.strip
          next_url = extract_angle_bracketed(part) if part.include?('rel="next"')
          break unless next_url.nil?
        end

        next_url = next_url&.force_encoding(link_header.encoding)
      end

      next_url
    end
  end

  # Wraps an HTTP response.
  class Response
    # @return [String] response body
    attr_reader :body

    # @return [Integer] HTTP status code
    attr_reader :status

    # @return [Hash] response headers
    attr_reader :headers

    def initialize(body:, status:, headers:)
      @body = body
      @status = status
      @headers = headers
    end

    # Parses the response body as JSON, normalizing Person-shaped objects.
    # @return [Hash, Array]
    def json
      @json ||= begin
        Security.check_body_size!(@body, Security::MAX_RESPONSE_BODY_BYTES)
        result = JSON.parse(@body)
        Http.normalize_person_ids(result)
        result
      end
    end

    # Returns whether the response was successful (2xx).
    # @return [Boolean]
    def success?
      status >= 200 && status < 300
    end
  end
end
