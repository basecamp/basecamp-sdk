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

# The person-id corpus, every row a MEASURED verdict of the reference's own
# reader rather than a reading of its documentation: a probe linked against
# go/pkg/types.FlexibleInt64 and against normalizeEmbeddedPeopleJSON produced
# all 74. Both Ruby sites that read a person id off the wire are pinned to it —
# Basecamp::Http.normalize_person_ids and Basecamp::Ids.person_from_wire — so
# the table lives here rather than in either test file.
#
# The three outcomes are strconv.ParseInt's three, which the sites spell
# differently and mean identically:
#
#   [ :value, n ]  ParseInt returned n. The normalizer writes the number and no
#                  system_label; the reader returns n.
#   :label         ErrSyntax. The normalizer writes id 0 and system_label = the
#                  raw string (the reference's non-numeric sentinel — the
#                  "basecamp" system actor); the reader returns 0.
#   :refuse        ErrRange. The normalizer LEAVES THE STRING alone so the
#                  reader refuses it; the reader fails the read (nil).
#
# Rows that exist to discriminate and must not be pruned as redundant: "+7" and
# "+007" (the sign a ^-?\d+$ regex refuses); "007", "010" and
# "0009223372036854775807" (leading zeros — "010" is TEN, and Ruby's Integer()
# read it as eight); the Unicode digit rows ("１２３", "٠١٢", "৭", "۷", "７",
# "৭7", "7৭"), which Ruby refuses today — both its /\d/ and Integer() are
# ASCII-only — but which a \p{Nd}-aware rewrite would turn into ids the
# reference reads as sentinels; the whitespace and underscore rows, which
# Integer() DOES accept (" 7" is 7 there, "1_2" is 12); "9007199254740992" and
# "9007199254740993" (past JS's safe-integer range, real int64 ids); and the
# scan-order pair "18446744073709551615x" against "18446744073709551616x" —
# one digit apart and opposite refusals, because ParseUint checks the magnitude
# inside the scan and never reaches the "x" once it has overflowed.
module GoPersonIds
  CORPUS = [
    [ "7", [ :value, 7 ] ],
    [ "0", [ :value, 0 ] ],
    [ "-0", [ :value, 0 ] ],
    [ "+0", [ :value, 0 ] ],
    [ "+7", [ :value, 7 ] ],
    [ "-7", [ :value, -7 ] ],
    [ "007", [ :value, 7 ] ],
    [ "+007", [ :value, 7 ] ],
    [ "-007", [ :value, -7 ] ],
    [ "0009223372036854775807", [ :value, 9223372036854775807 ] ],
    [ "0000000000000000000000009", [ :value, 9 ] ],
    [ "", :label ],
    [ " ", :label ],
    [ "+", :label ],
    [ "-", :label ],
    [ " 7", :label ],
    [ "7 ", :label ],
    [ " 7 ", :label ],
    [ "\n7", :label ],
    [ "7\n", :label ],
    [ "\t7", :label ],
    [ "7\t", :label ],
    [ "1_0", :label ],
    [ "1_2", :label ],
    [ "0x10", :label ],
    [ "0b11", :label ],
    [ "0o17", :label ],
    [ "010", [ :value, 10 ] ],
    [ "0X1F", :label ],
    [ "7x", :label ],
    [ "x7", :label ],
    [ "12.0", :label ],
    [ "1e3", :label ],
    [ "12,3", :label ],
    [ "basecamp", :label ],
    [ "campfire", :label ],
    [ "LocalPerson", :label ],
    [ "\uff11\uff12\uff13", :label ],
    [ "\uff17", :label ],
    [ "\u0660\u0661\u0662", :label ],
    [ "\u09ed", :label ],
    [ "\u06f7", :label ],
    [ "9223372036854775806", [ :value, 9223372036854775806 ] ],
    [ "9223372036854775807", [ :value, 9223372036854775807 ] ],
    [ "9223372036854775808", :refuse ],
    [ "9223372036854775809", :refuse ],
    [ "-9223372036854775807", [ :value, -9223372036854775807 ] ],
    [ "-9223372036854775808", [ :value, -9223372036854775808 ] ],
    [ "-9223372036854775809", :refuse ],
    [ "18446744073709551614", :refuse ],
    [ "18446744073709551615", :refuse ],
    [ "18446744073709551616", :refuse ],
    [ "18446744073709551615x", :label ],
    [ "18446744073709551616x", :refuse ],
    [ "1844674407370955161x", :label ],
    [ "-18446744073709551615x", :label ],
    [ "-18446744073709551616x", :refuse ],
    [ "99999999999999999999999", :refuse ],
    [ "99999999999999999999999x", :refuse ],
    [ "00000000000000000000018446744073709551616", :refuse ],
    [ "0000000000000000000009223372036854775807", [ :value, 9223372036854775807 ] ],
    [ "9007199254740991", [ :value, 9007199254740991 ] ],
    [ "9007199254740992", [ :value, 9007199254740992 ] ],
    [ "9007199254740993", [ :value, 9007199254740993 ] ],
    [ "90071992547409931", [ :value, 90071992547409931 ] ],
    [ "-9007199254740993", [ :value, -9007199254740993 ] ],
    [ "+9223372036854775807", [ :value, 9223372036854775807 ] ],
    [ "+9223372036854775808", :refuse ],
    [ "00", [ :value, 0 ] ],
    [ "0000", [ :value, 0 ] ],
    [ "-00", [ :value, 0 ] ],
    [ "\u0660", :label ],
    [ "\u09ed7", :label ],
    [ "7\u09ed", :label ]
  ].freeze
end

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
