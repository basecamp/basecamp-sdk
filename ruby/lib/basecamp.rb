# frozen_string_literal: true

require "zeitwerk"

loader = Zeitwerk::Loader.for_gem
loader.collapse("#{__dir__}/basecamp/generated")
loader.setup

# Load generated types if available
begin
  require_relative "basecamp/generated/types"
rescue LoadError
  # Generated types not available yet
end

# Main entry point for the Basecamp SDK.
#
# The SDK follows a Client -> AccountClient pattern:
# - Client: Holds shared resources (HTTP client, token provider, hooks)
# - AccountClient: Bound to a specific account ID, provides service accessors
#
# @example Basic usage
#   config = Basecamp::Config.new(base_url: "https://3.basecampapi.com")
#   token = Basecamp::StaticTokenProvider.new(ENV["BASECAMP_TOKEN"])
#
#   client = Basecamp::Client.new(config: config, token_provider: token)
#   account = client.for_account("12345")
#
#   # Use services (returns lazy Enumerator)
#   projects = account.projects.list.to_a
#
# @example With hooks for logging
#   class MyHooks
#     include Basecamp::Hooks
#
#     def on_request_start(info)
#       puts "Starting #{info.method} #{info.url}"
#     end
#
#     def on_request_end(info, result)
#       puts "Completed in #{result.duration}s"
#     end
#   end
#
#   client = Basecamp::Client.new(config: config, token_provider: token, hooks: MyHooks.new)
module Basecamp
  # Creates a new Basecamp client.
  #
  # This is a convenience method that creates a Client with the given options.
  #
  # @param access_token [String, nil] OAuth access token
  # @param auth [AuthStrategy, nil] custom authentication strategy
  # @param account_id [String, nil] Basecamp account ID (optional)
  # @param base_url [String] Base URL for API requests
  # @param hooks [Hooks, nil] Observability hooks
  # @return [Client, AccountClient] Client if no account_id, AccountClient if account_id provided
  #
  # @example With access token
  #   client = Basecamp.client(access_token: "abc123", account_id: "12345")
  #   projects = client.projects.list.to_a
  #
  # @example With custom auth strategy
  #   client = Basecamp.client(auth: MyCustomAuth.new, account_id: "12345")
  def self.client(
    access_token: nil,
    auth: nil,
    account_id: nil,
    base_url: Config::DEFAULT_BASE_URL,
    hooks: nil
  )
    raise ArgumentError, "provide either access_token or auth, not both" if access_token && auth
    raise ArgumentError, "provide access_token or auth" if !access_token && !auth

    config = Config.new(base_url: base_url)

    client = if auth
      Client.new(config: config, auth_strategy: auth, hooks: hooks)
    else
      token_provider = StaticTokenProvider.new(access_token)
      Client.new(config: config, token_provider: token_provider, hooks: hooks)
    end

    account_id ? client.for_account(account_id) : client
  end

  # Maps an HTTP response to the appropriate error class.
  #
  # @param status [Integer] HTTP status code
  # @param body [String, nil] response body (will attempt JSON parse)
  # @param retry_after [Integer, nil] Retry-After header value
  # @return [Error]
  def self.error_from_response(status, body = nil, retry_after: nil)
    message = parse_error_message(body) || "Request failed"

    case status
    when 400, 422
      ValidationError.new(message, http_status: status)
    when 401
      AuthError.new(message)
    when 403
      ForbiddenError.new(message)
    when 404
      NotFoundError.new(message: message)
    when 429
      RateLimitError.new(retry_after: retry_after)
    when 500
      ApiError.new("Server error (500)", http_status: 500, retryable: true)
    when 502, 503, 504
      ApiError.new("Gateway error (#{status})", http_status: status, retryable: true)
    else
      ApiError.from_status(status, message)
    end
  end

  # Extracts a filename from the last path segment of a URL.
  # Falls back to "download" if the URL is unparseable or has no path segments.
  def self.filename_from_url(raw_url)
    uri = URI.parse(raw_url)
    path = uri.path
    return "download" if path.nil? || path.empty? || path == "/" || path.end_with?("/")

    segments = path.split("/").reject(&:empty?)
    return "download" if segments.empty?

    last = segments.last
    return "download" if last.nil? || last.empty? || last == "." || last == "/"

    URI::RFC2396_PARSER.unescape(last)
  rescue URI::InvalidURIError
    "download"
  end

  # Parses error message from response body.
  # @param body [String, nil]
  # @return [String, nil]
  def self.parse_error_message(body)
    return nil if body.nil? || body.empty?

    Security.check_body_size!(body, Security::MAX_ERROR_BODY_BYTES, "Error")

    data = JSON.parse(body)
    msg = data["error"] || data["message"]
    msg ? Security.truncate(msg) : nil
  rescue JSON::ParserError, ApiError
    nil
  end
end
