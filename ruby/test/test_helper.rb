# frozen_string_literal: true

require "simplecov"
SimpleCov.start do
  add_filter "/test/"
  add_filter "/generated/"
  # Build-time code generators, not shipped library code. test/scripts/ loads
  # them to unit-test their pure helpers; their emit paths run under `make
  # rb-generate` and are covered by the regenerate-and-diff freshness gate.
  add_filter "/scripts/"
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

  # Stub resource-first discovery hop 1 to fail (404 on the protected-resource
  # well-known), yielding a soft resource_discovery_failed → Launchpad fallback.
  def stub_discovery_failure
    stub_request(:get, "#{BASE_URL}/.well-known/oauth-protected-resource")
      .to_return(status: 404, body: "Not Found")
  end

  # Stub resource-first discovery hop 1 to advertise only Launchpad, yielding a
  # soft no_as_advertised → Launchpad fallback.
  def stub_discovery_success
    resource_metadata = {
      resource: BASE_URL,
      authorization_servers: [ LAUNCHPAD_URL ]
    }
    stub_request(:get, "#{BASE_URL}/.well-known/oauth-protected-resource")
      .to_return(
        status: 200,
        body: resource_metadata.to_json,
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

  # Load a shared JSON fixture (the validated source of truth) as string-keyed
  # hashes. test_helper.rb lives at ruby/test/, so "../../spec/fixtures" reaches
  # the repo-root spec/fixtures directory. Read as explicit UTF-8 so a non-UTF-8
  # process locale (LC_ALL=C) doesn't choke on the emoji/non-ASCII bytes several
  # fixtures carry.
  def load_fixture(relative_path)
    path = File.expand_path("../../spec/fixtures/#{relative_path}", __dir__)
    JSON.parse(File.read(path, encoding: "UTF-8"))
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

  # Sample authorization data as *Launchpad* serves it.
  #
  # Use this only for tests whose issuer is Launchpad. A BC5 issuer serves a
  # different document; see {#sample_bc5_authorization}.
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

  # Sample authorization data as a *BC5* issuer serves it, per bc3's
  # app/views/api/authorizations/show.json.jbuilder.
  #
  # Feeding Launchpad's body to a test that proves discovery reached a BC5 issuer
  # makes the test agree with itself and with nothing else: it would pass just as
  # well if the SDK could not read a BC5 document at all. The differences are the
  # point — identity id only, no product or app_href, an RFC 8707 resource
  # indicator, and a top-level scope. (expires_at was integer epoch seconds
  # before bc3 #12646 converged it on ISO 8601; Ruby passes it through verbatim
  # either way.)
  def sample_bc5_authorization
    {
      "identity" => { "id" => 1 },
      "accounts" => [
        {
          "id" => 12_345,
          "name" => "Test Account",
          "href" => "https://bc5.example.test/12345",
          "resource" => "urn:bc:account:12345"
        }
      ],
      "scope" => "read write",
      "expires_at" => "2036-01-29T09:55:56Z"
    }
  end
end

# Also expose as TestHelper for compatibility
TestHelper = TestHelpers

module Minitest
  # Assertion aliases for readable tests (like ActiveSupport provides).
  # rubocop:disable Rails/RefuteMethods
  module AssertNotAliases
    def assert_not(object, message = nil)
      refute(object, message)
    end

    def assert_not_nil(object, message = nil)
      refute_nil(object, message)
    end

    def assert_not_equal(expected, actual, message = nil)
      refute_equal(expected, actual, message)
    end

    def assert_not_empty(object, message = nil)
      refute_empty(object, message)
    end

    def assert_not_includes(collection, object, message = nil)
      refute_includes(collection, object, message)
    end

    def assert_no_match(pattern, string, message = nil)
      refute_match(pattern, string, message)
    end
  end
  # rubocop:enable Rails/RefuteMethods

  class Test
    include TestHelpers
    include AssertNotAliases
  end
end
