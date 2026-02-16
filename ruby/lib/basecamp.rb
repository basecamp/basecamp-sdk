# frozen_string_literal: true

require "zeitwerk"

# Set up Zeitwerk loader
loader = Zeitwerk::Loader.for_gem
# No custom inflections - use standard Ruby camelcase (Http, Oauth, etc.)

# Ignore hand-written services - we use generated services instead (spec-conformant)
# EXCEPT: base_service.rb (infrastructure) and authorization_service.rb (OAuth, not in spec)
loader.ignore("#{__dir__}/basecamp/services")

# Collapse the generated directory so Basecamp::Generated::Services becomes Basecamp::Services
loader.collapse("#{__dir__}/basecamp/generated")

# Ignore errors.rb - it defines multiple classes, loaded explicitly below
loader.ignore("#{__dir__}/basecamp/errors.rb")
# Ignore auth_strategy.rb - defines both AuthStrategy and BearerAuth
loader.ignore("#{__dir__}/basecamp/auth_strategy.rb")
# Ignore operation_info.rb - defines both OperationInfo and OperationResult
loader.ignore("#{__dir__}/basecamp/operation_info.rb")
loader.setup

# Load infrastructure that generated services depend on
require_relative "basecamp/errors"
require_relative "basecamp/auth_strategy"
require_relative "basecamp/operation_info"
require_relative "basecamp/services/base_service"
require_relative "basecamp/services/authorization_service"

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
end
