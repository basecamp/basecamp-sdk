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
    USER_AGENT = "basecamp-sdk-ruby/#{VERSION}".freeze

    # @param config [Config] configuration settings
    # @param token_provider [TokenProvider] OAuth token provider
    # @param hooks [Hooks] observability hooks
    def initialize(config:, token_provider:, hooks: nil)
      @config = config
      @token_provider = token_provider
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
    # @return [Response]
    def get(path, params: {})
      request(:get, path, params: params)
    end

    # Performs a GET request to an absolute URL.
    # Used for endpoints not on the base API (e.g., Launchpad).
    # @param url [String] absolute URL
    # @param params [Hash] query parameters
    # @return [Response]
    def get_absolute(url, params: {})
      request(:get, url, params: params)
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

    # Fetches all pages of a paginated resource.
    # @param path [String] initial URL path
    # @param params [Hash] query parameters
    # @yield [Hash] each item from the response
    # @return [Enumerator] if no block given
    def paginate(path, params: {}, &block)
      return to_enum(:paginate, path, params: params) unless block

      url = build_url(path)
      page = 0

      loop do
        page += 1
        break if page > @config.max_pages

        @hooks.on_paginate(url, page)
        response = get(url, params: page == 1 ? params : {})

        items = JSON.parse(response.body)
        items.each(&block)

        next_url = parse_next_link(response.headers["Link"])
        break if next_url.nil?

        url = next_url
      end
    end

    # Fetches all pages of a paginated resource, extracting items from a key.
    # Use this for endpoints that return objects like { "events": [...] }.
    # @param path [String] initial URL path
    # @param key [String] the key containing the array of items
    # @param params [Hash] query parameters
    # @yield [Hash] each item from the response
    # @return [Enumerator] if no block given
    def paginate_key(path, key:, params: {}, &block)
      return to_enum(:paginate_key, path, key: key, params: params) unless block

      url = build_url(path)
      page = 0

      loop do
        page += 1
        break if page > @config.max_pages

        @hooks.on_paginate(url, page)
        response = get(url, params: page == 1 ? params : {})

        data = JSON.parse(response.body)
        unless data.key?(key)
          warn "[Basecamp SDK] paginate_key: expected key '#{key}' not found in response (page #{page})"
        end
        items = data[key] || []
        items.each(&block)

        next_url = parse_next_link(response.headers["Link"])
        break if next_url.nil?

        url = next_url
      end
    end

    private

    def build_faraday_client
      Faraday.new(url: @config.base_url) do |f|
        f.options.timeout = @config.timeout
        f.options.open_timeout = 10
        f.request :json
        f.response :raise_error
        f.adapter Faraday.default_adapter
      end
    end

    def request(method, path, params: {}, body: nil)
      url = build_url(path)

      # Mutations don't retry on 429/5xx to avoid duplicating data
      if method == :get
        request_with_retry(method, url, params: params)
      else
        single_request(method, url, params: params, body: body, attempt: 1)
      end
    end

    def request_with_retry(method, url, params: {})
      attempt = 0
      last_error = nil

      loop do
        attempt += 1
        break if attempt > @config.max_retries

        begin
          return single_request(method, url, params: params, body: nil, attempt: attempt)
        rescue Basecamp::RateLimitError, Basecamp::NetworkError, Basecamp::APIError => e
          raise e unless e.retryable?

          last_error = e

          # Don't sleep if this was the last attempt
          break if attempt >= @config.max_retries

          delay = calculate_delay(attempt, e.retry_after)

          @hooks.on_retry(RequestInfo.new(method: method.to_s.upcase, url: url, attempt: attempt), attempt + 1, e,
                          delay)
          sleep(delay)
        end
      end

      raise last_error || Basecamp::APIError.new("Request failed after #{@config.max_retries} retries")
    end

    def single_request(method, url, params:, body:, attempt:)
      info = RequestInfo.new(method: method.to_s.upcase, url: url, attempt: attempt)
      @hooks.on_request_start(info)

      start_time = Process.clock_gettime(Process::CLOCK_MONOTONIC)

      begin
        response = @faraday.run_request(method, url, body, request_headers) do |req|
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
      rescue Faraday::ClientError => e
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

    def request_headers
      {
        "Authorization" => "Bearer #{@token_provider.access_token}",
        "User-Agent" => USER_AGENT,
        "Accept" => "application/json"
      }
    end

    def single_request_raw(method, url, body:, content_type:, attempt:)
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
      rescue Faraday::ClientError => e
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

    def handle_error(error)
      status = error.response&.dig(:status)
      body = error.response&.dig(:body)
      headers = error.response&.dig(:headers) || {}

      retry_after = parse_retry_after(headers["Retry-After"] || headers["retry-after"])

      case status
      when 401
        # Try token refresh
        if @token_provider.refreshable? && @token_provider.refresh
          raise Basecamp::AuthError.new("Token refreshed", hint: "Retry the request")
        end

        Basecamp::AuthError.new("Authentication failed")
      when 403
        Basecamp::ForbiddenError.new("Access denied")
      when 404
        Basecamp::NotFoundError.new("Resource", "unknown")
      when 429
        Basecamp::RateLimitError.new(retry_after: retry_after)
      when 400, 422
        message = Basecamp.parse_error_message(body) || "Validation failed"
        Basecamp::ValidationError.new(message)
      when 500
        Basecamp::APIError.new("Server error (500)", http_status: 500)
      when 502, 503, 504
        Basecamp::APIError.new("Gateway error (#{status})", http_status: status, retryable: true)
      else
        message = Basecamp.parse_error_message(body) || "Request failed (HTTP #{status})"
        Basecamp::APIError.from_status(status || 0, message)
      end
    end

    def build_url(path)
      return path if path.start_with?("http://", "https://")

      path = "/#{path}" unless path.start_with?("/")
      "#{@config.base_url}#{path}"
    end

    def calculate_delay(attempt, server_retry_after)
      return server_retry_after if server_retry_after&.positive?

      # Exponential backoff: base_delay * 2^(attempt-1) + jitter
      base = @config.base_delay * (2**(attempt - 1))
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

    def parse_next_link(link_header)
      return nil if link_header.nil? || link_header.empty?

      link_header.split(",").each do |part|
        part = part.strip
        next unless part.include?('rel="next"')

        match = part.match(/<([^>]+)>/)
        return match[1] if match
      end

      nil
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

    # Parses the response body as JSON.
    # @return [Hash, Array]
    def json
      @json ||= JSON.parse(@body)
    end

    # Returns whether the response was successful (2xx).
    # @return [Boolean]
    def success?
      status >= 200 && status < 300
    end
  end
end
