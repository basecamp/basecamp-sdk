# frozen_string_literal: true

require "zeitwerk"

loader = Zeitwerk::Loader.for_gem
loader.collapse("#{__dir__}/basecamp/generated")
# The generated class owns the Basecamp::Services::TodosService constant
# (generated/ is collapsed), so the hand-written merge-safe update/edit
# surface is prepended onto it as soon as zeitwerk loads it.
loader.on_load("Basecamp::Services::TodosService") do |klass, _abspath|
  klass.prepend(Basecamp::Services::TodosExtensions)
end
# Same shape for cards: the generated class owns the constant and the
# tri-state `due_on` update is prepended over the generated update_verbatim.
loader.on_load("Basecamp::Services::CardsService") do |klass, _abspath|
  klass.prepend(Basecamp::Services::CardsExtensions)
end
# And for todolists: PUT /todolists/{id} is a full replace, so the generated
# class owns `replace` and the merge-safe update/edit surface is prepended.
loader.on_load("Basecamp::Services::TodolistsService") do |klass, _abspath|
  klass.prepend(Basecamp::Services::TodolistsExtensions)
end
# And for documents: PUT /documents/{id} is a full replace, so the generated
# class owns `replace` and the merge-safe update/edit surface is prepended.
loader.on_load("Basecamp::Services::DocumentsService") do |klass, _abspath|
  klass.prepend(Basecamp::Services::DocumentsExtensions)
end
# And for schedule entries: PUT /schedule_entries/{id} is a full replace, so
# the generated class owns `replace_entry` and the merge-safe
# `update_entry`/`edit_entry` surface is prepended.
loader.on_load("Basecamp::Services::SchedulesService") do |klass, _abspath|
  klass.prepend(Basecamp::Services::SchedulesExtensions)
end
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
    # SPEC §6 step 3: a body's error_description becomes the hint. Step 5:
    # with no body message, the else arm falls back (via from_status) to the
    # fixed code-bearing phrase, never a reason phrase.
    hint = parse_error_hint(body)
    server_message = parse_error_message(body)
    message = server_message || "Request failed"

    case status
    when 400, 422
      field_errors = parse_field_errors(body)
      message = Security.truncate(compose_validation_message(server_message, field_errors) || "Request failed")
      ValidationError.new(message, hint: hint, http_status: status, field_errors: field_errors)
    when 401
      AuthError.new(message, hint: hint)
    when 403
      ForbiddenError.new(message, hint: hint)
    when 404
      NotFoundError.new(message: message, hint: hint)
    when 429
      RateLimitError.new(retry_after: retry_after, hint: hint)
    when 507
      # Decided before the 5xx arms: a 507 is an account limit, not a
      # transient server failure, and no retry can satisfy it.
      LimitExceededError.new(Security.truncate(message), hint: hint)
    when 500
      ApiError.new("Server error (500)", http_status: 500, retryable: true, hint: hint)
    when 502, 503, 504
      ApiError.new("Gateway error (#{status})", http_status: status, retryable: true, hint: hint)
    else
      ApiError.from_status(status, server_message, hint: hint)
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

  # Parses error message from response body. A key is used only when its
  # value is a String (SPEC section 6), so a malformed scalar member such as
  # {"error": {}} cannot raise or leak a non-string into the message.
  # @param body [String, nil]
  # @return [String, nil]
  def self.parse_error_message(body)
    return nil if body.nil? || body.empty?

    Security.check_body_size!(body, Security::MAX_ERROR_BODY_BYTES, "Error")

    data = JSON.parse(body)
    msg = data.is_a?(Hash) ? [ data["error"], data["message"] ].find { |value| value.is_a?(String) } : nil
    msg ? Security.truncate(msg) : nil
  rescue JSON::ParserError, ApiError
    nil
  end

  # Parses the SPEC section 6 step-3 hint from a response body: the
  # "error_description" key, used only when its value is a non-empty String,
  # truncated like the message.
  # @param body [String, nil]
  # @return [String, nil]
  def self.parse_error_hint(body)
    return nil if body.nil? || body.empty?

    Security.check_body_size!(body, Security::MAX_ERROR_BODY_BYTES, "Error")

    data = JSON.parse(body)
    hint = data.is_a?(Hash) ? data["error_description"] : nil
    hint.is_a?(String) && !hint.empty? ? Security.truncate(hint) : nil
  rescue JSON::ParserError, ApiError
    nil
  end

  # Extracts the field-keyed validation errors map from a response body — the
  # Rails RecordInvalid rendering {"errors" => {"field" => ["msg", ...]}}.
  # Entries whose value is not an array are skipped, non-string elements are
  # dropped, and a map with no usable entries is treated as absent (nil).
  # @param body [String, nil]
  # @return [Hash{String => Array<String>}, nil]
  def self.parse_field_errors(body)
    return nil if body.nil? || body.empty?

    Security.check_body_size!(body, Security::MAX_ERROR_BODY_BYTES, "Error")

    data = JSON.parse(body)
    errors = data.is_a?(Hash) ? data["errors"] : nil
    if errors.is_a?(Hash)
      field_errors = errors.each_with_object({}) do |(field, values), result|
        next unless values.is_a?(Array)

        messages = values.grep(String)
        result[field.to_s] = messages unless messages.empty?
      end
      field_errors.empty? ? nil : field_errors
    else
      parse_bare_field_errors(data)
    end
  rescue JSON::ParserError, ApiError
    nil
  end

  # Extracts an unwrapped field map — the `render json: @webhook.errors`
  # rendering, where the whole body is {"field" => ["msg", ...]}. The gate is
  # all-or-nothing by design (SPEC section 6 step 2): with no "errors" key to
  # declare intent, only shape distinguishes a field map from any other JSON
  # object, so a single non-conforming member means this is not one.
  # @param data [Object] the parsed body
  # @return [Hash{String => Array<String>}, nil]
  def self.parse_bare_field_errors(data)
    return nil unless data.is_a?(Hash) && !data.empty?
    # Only "errors" is structurally reserved (it belongs to the wrapped path).
    # "error" and "message" are not excluded by name: a flat body carries them
    # as strings, which the shape gate below already rejects.
    return nil if data.key?("errors")

    data.each_with_object({}) do |(field, values), result|
      return nil unless values.is_a?(Array) && !values.empty?
      return nil unless values.all? { |message| message.is_a?(String) && !message.empty? }

      result[field.to_s] = values
    end
  end

  # Merges the top-level error message with the flattened field-keyed errors:
  # appended in parentheses when both are present, standing alone when only the
  # field errors are. The flattened shape — fields sorted lexicographically, a
  # field's messages joined with "; ", fields joined with ", " — is shared by
  # all six SDKs; change it everywhere or nowhere. Callers truncate the
  # composed result so the appended tail is capped too.
  # @param message [String, nil]
  # @param field_errors [Hash{String => Array<String>}, nil]
  # @return [String, nil]
  def self.compose_validation_message(message, field_errors)
    if field_errors.nil?
      message
    else
      flat = field_errors.keys.sort \
        .map { |field| "#{field}: #{field_errors[field].join("; ")}" } \
        .join(", ")
      message ? "#{message} (#{flat})" : flat
    end
  end
end
