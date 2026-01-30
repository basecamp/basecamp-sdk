# frozen_string_literal: true

require "simplecov"
SimpleCov.start do
  add_filter "/test/"
  add_filter "/generated/"
  enable_coverage :branch
  minimum_coverage line: 90, branch: 60
end

$LOAD_PATH.unshift File.expand_path("../lib", __dir__)

require "basecamp"
require "minitest/autorun"
require "webmock/minitest"
require "json"

# Disable external connections during tests
WebMock.disable_net_connect!

# Test helpers and fixtures
module TestHelpers
  BASE_URL = "https://3.basecampapi.com"
  LAUNCHPAD_URL = "https://launchpad.37signals.com"
  ACCOUNT_ID = "12345"
  ACCESS_TOKEN = "test-access-token"

  def base_url
    BASE_URL
  end

  def account_id
    ACCOUNT_ID
  end

  def access_token
    ACCESS_TOKEN
  end

  def config
    @config ||= Basecamp::Config.new(
      base_url: BASE_URL,
      timeout: 5,
      max_retries: 3
    )
  end

  # Alias for compatibility with nested tests
  alias default_config config

  def token_provider
    @token_provider ||= Basecamp::StaticTokenProvider.new(ACCESS_TOKEN)
  end

  # Alias for compatibility with nested tests
  alias test_token_provider token_provider

  def http
    @http ||= Basecamp::Http.new(
      config: config,
      token_provider: token_provider
    )
  end

  # Creates a test client
  def create_client(config: nil, token_provider: nil, hooks: nil)
    Basecamp::Client.new(
      config: config || self.config,
      token_provider: token_provider || self.token_provider,
      hooks: hooks
    )
  end

  # Creates a test AccountClient
  def create_account_client(account_id: ACCOUNT_ID, **kwargs)
    create_client(**kwargs).for_account(account_id)
  end

  def stub_api_get(path, body:, status: 200, headers: {})
    stub_request(:get, "#{BASE_URL}#{path}")
      .with(headers: { "Authorization" => "Bearer #{ACCESS_TOKEN}" })
      .to_return(
        status: status,
        body: body.is_a?(String) ? body : body.to_json,
        headers: { "Content-Type" => "application/json" }.merge(headers)
      )
  end

  # Alias for compatibility with nested tests
  def stub_get(path, response_body:, status: 200, headers: {})
    stub_request(:get, "#{BASE_URL}#{path}")
      .to_return(
        status: status,
        body: response_body.is_a?(String) ? response_body : response_body.to_json,
        headers: { "Content-Type" => "application/json" }.merge(headers)
      )
  end

  # Stub requests to launchpad (authorization endpoint)
  def stub_launchpad_get(path, response_body:, status: 200, headers: {})
    stub_request(:get, "#{LAUNCHPAD_URL}#{path}")
      .to_return(
        status: status,
        body: response_body.is_a?(String) ? response_body : response_body.to_json,
        headers: { "Content-Type" => "application/json" }.merge(headers)
      )
  end

  # Stub OAuth discovery to fail (triggers fallback to launchpad)
  def stub_discovery_failure
    stub_request(:get, "#{BASE_URL}/.well-known/oauth-authorization-server")
      .to_return(status: 404, body: "Not Found")
  end

  # Stub OAuth discovery to succeed with launchpad config
  def stub_discovery_success
    discovery_response = {
      issuer: LAUNCHPAD_URL,
      authorization_endpoint: "#{LAUNCHPAD_URL}/authorization/new",
      token_endpoint: "#{LAUNCHPAD_URL}/authorization/token"
    }
    stub_request(:get, "#{BASE_URL}/.well-known/oauth-authorization-server")
      .to_return(
        status: 200,
        body: discovery_response.to_json,
        headers: { "Content-Type" => "application/json" }
      )
  end

  def stub_api_post(path, body:, status: 201, headers: {})
    stub_request(:post, "#{BASE_URL}#{path}")
      .with(headers: { "Authorization" => "Bearer #{ACCESS_TOKEN}" })
      .to_return(
        status: status,
        body: body.is_a?(String) ? body : body.to_json,
        headers: { "Content-Type" => "application/json" }.merge(headers)
      )
  end

  # Alias for compatibility with nested tests
  def stub_post(path, response_body:, status: 201, headers: {})
    stub_request(:post, "#{BASE_URL}#{path}")
      .to_return(
        status: status,
        body: response_body.is_a?(String) ? response_body : response_body.to_json,
        headers: { "Content-Type" => "application/json" }.merge(headers)
      )
  end

  def stub_api_put(path, body:, status: 200, headers: {})
    stub_request(:put, "#{BASE_URL}#{path}")
      .with(headers: { "Authorization" => "Bearer #{ACCESS_TOKEN}" })
      .to_return(
        status: status,
        body: body.is_a?(String) ? body : body.to_json,
        headers: { "Content-Type" => "application/json" }.merge(headers)
      )
  end

  # Alias for compatibility with nested tests
  def stub_put(path, response_body:, status: 200, headers: {})
    stub_request(:put, "#{BASE_URL}#{path}")
      .to_return(
        status: status,
        body: response_body.is_a?(String) ? response_body : response_body.to_json,
        headers: { "Content-Type" => "application/json" }.merge(headers)
      )
  end

  def stub_api_delete(path, status: 204, body: nil, headers: {})
    stub_request(:delete, "#{BASE_URL}#{path}")
      .with(headers: { "Authorization" => "Bearer #{ACCESS_TOKEN}" })
      .to_return(
        status: status,
        body: body,
        headers: headers
      )
  end

  # Alias for compatibility with nested tests
  def stub_delete(path, status: 204)
    stub_request(:delete, "#{BASE_URL}#{path}")
      .to_return(status: status, body: "")
  end

  # Sample project data
  def sample_project(id: 123, name: "Test Project")
    {
      "id" => id,
      "name" => name,
      "description" => "A test project",
      "status" => "active",
      "created_at" => "2024-01-01T00:00:00Z",
      "updated_at" => "2024-01-01T00:00:00Z"
    }
  end

  # Sample todo data
  def sample_todo(id: 456, content: "Test todo")
    {
      "id" => id,
      "content" => content,
      "description" => "",
      "completed" => false,
      "created_at" => "2024-01-01T00:00:00Z",
      "updated_at" => "2024-01-01T00:00:00Z"
    }
  end

  # Sample authorization data
  def sample_authorization
    {
      "expires_at" => "2025-01-01T00:00:00Z",
      "identity" => {
        "id" => 1,
        "first_name" => "Test",
        "last_name" => "User",
        "email_address" => "test@example.com"
      },
      "accounts" => [
        {
          "id" => 12_345,
          "name" => "Test Account",
          "product" => "bc3",
          "href" => "https://3.basecampapi.com/12345"
        }
      ]
    }
  end
end

# Also expose as TestHelper for compatibility
TestHelper = TestHelpers

module Minitest
  class Test
    include TestHelpers
  end
end
