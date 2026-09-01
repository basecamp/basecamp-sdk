# frozen_string_literal: true

require "test_helper"

# A token provider that supports refresh for testing 401 retry behavior.
class RefreshableTokenProvider
  include Basecamp::TokenProvider

  attr_reader :refresh_count

  def initialize(token, refresh_result: true)
    @token = token
    @refresh_result = refresh_result
    @refresh_count = 0
  end

  def access_token
    @token
  end

  def refreshable?
    true
  end

  def refresh
    @refresh_count += 1
    @refresh_result
  end
end

class HTTPTest < Minitest::Test
  include TestHelper

  def setup
    @config = default_config
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_get_request
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: '{"result": "ok"}', headers: { "Content-Type" => "application/json" })

    response = @http.get("/test.json")

    assert_equal 200, response.status
    assert_equal({ "result" => "ok" }, response.json)
  end

  def test_get_with_params
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .with(query: { status: "active" })
      .to_return(status: 200, body: "[]", headers: { "Content-Type" => "application/json" })

    response = @http.get("/test.json", params: { status: "active" })

    assert_equal 200, response.status
  end

  def test_post_request
    stub_request(:post, "https://3.basecampapi.com/test.json")
      .with(body: { name: "Test" }.to_json)
      .to_return(status: 201, body: '{"id": 1}', headers: { "Content-Type" => "application/json" })

    response = @http.post("/test.json", body: { name: "Test" })

    assert_equal 201, response.status
    assert_equal({ "id" => 1 }, response.json)
  end

  def test_put_request
    stub_request(:put, "https://3.basecampapi.com/test/1.json")
      .to_return(status: 200, body: '{"updated": true}', headers: { "Content-Type" => "application/json" })

    response = @http.put("/test/1.json", body: { name: "Updated" })

    assert_equal 200, response.status
  end

  def test_delete_request
    stub_request(:delete, "https://3.basecampapi.com/test/1.json")
      .to_return(status: 204, body: "")

    response = @http.delete("/test/1.json")

    assert_equal 204, response.status
  end

  def test_authorization_header
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .with(headers: { "Authorization" => "Bearer test-access-token" })
      .to_return(status: 200, body: "{}")

    @http.get("/test.json")

    assert_requested(:get, "https://3.basecampapi.com/test.json",
                     headers: { "Authorization" => "Bearer test-access-token" })
  end

  def test_user_agent_header
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: "{}")

    @http.get("/test.json")

    assert_requested(:get, "https://3.basecampapi.com/test.json",
                     headers: { "User-Agent" => /basecamp-sdk-ruby/ })
  end

  def test_rejects_cross_origin_absolute_url
    error = assert_raises(Basecamp::UsageError) do
      @http.get("https://other.api.com/path.json")
    end
    assert_match(/origin/, error.message)
    assert_not_requested(:get, "https://other.api.com/path.json")
  end

  def test_401_raises_auth_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')

    assert_raises(Basecamp::AuthError) do
      @http.get("/test.json")
    end
  end

  def test_401_refresh_and_retry_succeeds
    provider = RefreshableTokenProvider.new("old-token", refresh_result: true)
    http = Basecamp::Http.new(config: @config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')
      .then.to_return(status: 200, body: '{"ok": true}')

    response = http.get("/test.json")

    assert_equal 200, response.status
    assert_equal({ "ok" => true }, response.json)
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
  end

  def test_401_refresh_and_retry_no_infinite_loop
    provider = RefreshableTokenProvider.new("old-token", refresh_result: true)
    http = Basecamp::Http.new(config: @config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')

    assert_raises(Basecamp::AuthError) do
      http.get("/test.json")
    end

    # First 401 triggers refresh+retry, second 401 raises (no infinite loop)
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
  end

  # SPEC §4: the refresh replay is a request on the wire, so it spends an
  # attempt from the same total-attempt budget as a transient retry (#565,
  # #461). These pin the PLAIN GET path — the "every GET in Python and Ruby"
  # case that an earlier, ungated attempt at this change regressed.
  def test_401_refresh_replay_is_not_attempted_without_budget
    provider = RefreshableTokenProvider.new("old-token", refresh_result: true)
    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com", timeout: 5, max_retries: 1)
    http = Basecamp::Http.new(config: config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')

    assert_raises(Basecamp::AuthError) { http.get("/test.json") }

    # One request, and no rotation burned on an attempt the loop cannot make.
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 1)
    assert_equal 0, provider.refresh_count
  end

  def test_401_refresh_replay_shares_the_budget_with_transient_retries
    provider = RefreshableTokenProvider.new("old-token", refresh_result: true)
    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com", timeout: 5, max_retries: 2,
      base_delay: 0.001, max_jitter: 0.0)
    http = Basecamp::Http.new(config: config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')
      .then.to_return(status: 503, body: '{"error": "Unavailable"}')

    assert_raises(Basecamp::ApiError) { http.get("/test.json") }

    # 401 (attempt 1, refresh) then 503 (attempt 2) exhausts the cap. Three
    # requests would mean the replay rode outside it.
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
    assert_equal 1, provider.refresh_count
  end

  def test_401_refresh_replay_is_actually_sent_not_discarded
    # The regression an ungated version caused: refresh, then rethrow the stale
    # 401 without ever sending the refreshed request. A 200 body proves it went.
    provider = RefreshableTokenProvider.new("old-token", refresh_result: true)
    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com", timeout: 5, max_retries: 2)
    http = Basecamp::Http.new(config: config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')
      .then.to_return(status: 200, body: '{"ok": true}')

    response = http.get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
    assert_equal 1, provider.refresh_count
  end

  # A token endpoint that fails is a transient failure of THIS attempt.
  # Performing the refresh inside the `rescue Basecamp::AuthError` clause would
  # let this NetworkError bypass the sibling transient rescue entirely — an
  # exception raised inside a rescue is not offered to that begin's other
  # rescues — ending the request with budget still unspent.
  def test_401_refresh_network_failure_still_retries_under_the_budget
    provider = Class.new do
      attr_reader :refresh_count

      def initialize = @refresh_count = 0
      def access_token = "old-token"
      def refreshable? = true

      def refresh
        @refresh_count += 1
        raise Basecamp::NetworkError.new("token endpoint timed out")
      end
    end.new

    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com", timeout: 5, max_retries: 3,
      base_delay: 0.001, max_jitter: 0.0)
    http = Basecamp::Http.new(config: config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')
      .then.to_return(status: 200, body: '{"ok": true}')

    response = http.get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
    assert_equal 1, provider.refresh_count
  end

  # SPEC §4 counts refresh ATTEMPTS, not successes. The 401 recurs because the
  # token never changed, so a flag set only on success would let the same
  # request call refresh a second time — and if the first call reached the
  # server and rotated the token before its response was lost, the second
  # spends a refresh token that is already dead.
  def test_401_throwing_refresh_still_spends_the_one_allowed_refresh
    provider = Class.new do
      attr_reader :refresh_count

      def initialize = @refresh_count = 0
      def access_token = "old-token"
      def refreshable? = true

      def refresh
        @refresh_count += 1
        raise Basecamp::NetworkError.new("token endpoint timed out")
      end
    end.new

    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com", timeout: 5, max_retries: 3,
      base_delay: 0.001, max_jitter: 0.0)
    http = Basecamp::Http.new(config: config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')

    assert_raises(Basecamp::AuthError) { http.get("/test.json") }

    assert_equal 1, provider.refresh_count
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
  end

  def test_401_no_retry_when_refresh_fails
    provider = RefreshableTokenProvider.new("old-token", refresh_result: false)
    http = Basecamp::Http.new(config: @config, token_provider: provider)

    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')

    assert_raises(Basecamp::AuthError) do
      http.get("/test.json")
    end

    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 1)
  end

  def test_401_refresh_and_retry_works_for_post
    provider = RefreshableTokenProvider.new("old-token", refresh_result: true)
    http = Basecamp::Http.new(config: @config, token_provider: provider)

    stub_request(:post, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "Unauthorized"}')
      .then.to_return(status: 201, body: '{"id": 1}')

    response = http.post("/test.json", body: { name: "Test" })

    assert_equal 201, response.status
    assert_requested(:post, "https://3.basecampapi.com/test.json", times: 2)
  end

  def test_403_raises_forbidden_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 403, body: '{"error": "Forbidden"}')

    assert_raises(Basecamp::ForbiddenError) do
      @http.get("/test.json")
    end
  end

  def test_422_raises_validation_error_with_correct_status
    stub_request(:post, "https://3.basecampapi.com/test.json")
      .to_return(status: 422, body: '{"error": "Name is required"}')

    error = assert_raises(Basecamp::ValidationError) do
      @http.post("/test.json", body: { name: "" })
    end

    assert_equal 422, error.http_status
    assert_equal "Name is required", error.message
  end

  def test_422_field_keyed_errors_flatten_into_message
    stub_request(:put, "https://3.basecampapi.com/test.json")
      .to_return(status: 422, body: '{"errors": {"color": ["is not a valid color"]}}')

    error = assert_raises(Basecamp::ValidationError) do
      @http.put("/test.json", body: { calendar: { color: "chartreuse" } })
    end

    assert_equal 422, error.http_status
    assert_equal "color: is not a valid color", error.message
    assert_equal({ "color" => [ "is not a valid color" ] }, error.field_errors)
  end

  def test_422_field_keyed_errors_append_to_top_level_error
    body = '{"error": "Validation failed", "errors": {"name": ["can\'t be blank", "is too short"], "color": ["is not a valid color"]}}'
    stub_request(:put, "https://3.basecampapi.com/test.json")
      .to_return(status: 422, body: body)

    error = assert_raises(Basecamp::ValidationError) do
      @http.put("/test.json", body: { calendar: { color: "chartreuse" } })
    end

    assert_equal "Validation failed (color: is not a valid color, name: can't be blank; is too short)", error.message
    assert_equal(
      { "color" => [ "is not a valid color" ], "name" => [ "can't be blank", "is too short" ] },
      error.field_errors
    )
  end

  # SPEC §6 step 3: a body's error_description becomes the hint on the runtime
  # error path.
  def test_error_description_becomes_hint
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(
        status: 403,
        body: '{"error": "denied", "error_description": "You need the admin scope"}',
        headers: { "Content-Type" => "application/json" }
      )

    error = assert_raises(Basecamp::ForbiddenError) { @http.get("/test.json") }
    assert_equal "You need the admin scope", error.hint
  end

  # A class-constant hint fills in only when the body carries no
  # error_description.
  def test_class_default_hint_survives_body_without_description
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 401, body: '{"error": "nope"}', headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::AuthError) { @http.get("/test.json") }
    assert_equal "Check your access token or refresh it if expired", error.hint
  end

  # SPEC §6 step 5: an empty body on an unmapped status renders the fixed
  # code-bearing phrase — 599 has no registered reason phrase at all.
  def test_unregistered_status_with_empty_body_renders_fixed_phrase
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 599, body: "")

    error = assert_raises(Basecamp::ApiError) { @http.get("/test.json") }
    assert_equal "Request failed (HTTP 599)", error.message
  end

  def test_404_raises_not_found_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 404, body: '{"error": "Not found"}')

    assert_raises(Basecamp::NotFoundError) do
      @http.get("/test.json")
    end
  end

  def test_429_raises_rate_limit_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 429, body: "{}", headers: { "Retry-After" => "30" })

    # Use single-attempt config to test error classification without sleeping
    config = Basecamp::Config.new(base_url: "https://3.basecampapi.com", max_retries: 1)
    http = Basecamp::Http.new(config: config, token_provider: @token_provider)

    error = assert_raises(Basecamp::RateLimitError) do
      http.get("/test.json")
    end

    assert_equal 30, error.retry_after
    assert error.retryable?
  end

  def test_500_raises_error
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 500, body: '{"error": "Server error"}')

    # 5xx errors may raise ApiError or NetworkError depending on Faraday error classification
    assert_raises(Basecamp::Error) do
      @http.get("/test.json")
    end
  end

  def test_response_json_parsing
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: '{"name": "Test", "count": 42}')

    response = @http.get("/test.json")
    json = response.json

    assert_equal "Test", json["name"]
    assert_equal 42, json["count"]
  end

  def test_response_success_predicate
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: "{}")

    response = @http.get("/test.json")

    assert response.success?
  end
end

class HTTPRetryTest < Minitest::Test
  include TestHelper

  def setup
    @config = Basecamp::Config.new(
      base_url: "https://3.basecampapi.com",
      timeout: 5,
      max_retries: 3,
      base_delay: 0.01, # Short delay for tests
      max_jitter: 0.001
    )
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_retries_on_5xx_for_get
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")
      .then.to_return(status: 200, body: '{"ok": true}')

    response = @http.get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 2)
  end

  def test_does_not_retry_post_on_5xx
    stub_request(:post, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")

    # Mutations should not retry, error type depends on Faraday classification
    assert_raises(Basecamp::Error) do
      @http.post("/test.json", body: { data: "test" })
    end

    assert_requested(:post, "https://3.basecampapi.com/test.json", times: 1)
  end

  def test_respects_retry_after_header
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 429, body: "{}", headers: { "Retry-After" => "1" })

    error = assert_raises(Basecamp::RateLimitError) do
      @http.get("/test.json")
    end

    assert_equal 1, error.retry_after
  end

  def test_max_retries_exceeded
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")

    # After max retries, error type depends on Faraday classification
    assert_raises(Basecamp::Error) do
      @http.get("/test.json")
    end

    # Should have retried max_retries times
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 3)
  end
end

# Governed retries: a GET carrying its canonical operation ID is bounded by
# min(caller cap, operation maxAttempts) and status-gated on the operation's
# declared retryOn set — the error taxonomy neither widens nor vetoes it.
# Ungoverned traffic (no operation keyword: get_absolute, OAuth discovery)
# keeps the pre-metadata contract.
class HTTPGovernedRetryTest < Minitest::Test
  include TestHelper

  def http_with_max_retries(max_retries)
    config = Basecamp::Config.new(
      base_url: "https://3.basecampapi.com",
      timeout: 5,
      max_retries: max_retries,
      base_delay: 0.01,
      max_jitter: 0.001
    )
    Basecamp::Http.new(config: config, token_provider: test_token_provider)
  end

  def test_operation_ceiling_bounds_raised_cap
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")

    # GetProject declares maxAttempts 3; a caller cap of 5 must clamp to it.
    assert_raises(Basecamp::Error) do
      http_with_max_retries(5).get("/test.json", operation: "GetProject")
    end

    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 3)
  end

  def test_caller_lower_cap_wins
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 503, body: "{}")

    assert_raises(Basecamp::Error) do
      http_with_max_retries(1).get("/test.json", operation: "GetProject")
    end

    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 1)
  end

  # #532: max_retries: 0 must still put one request on the wire. The count is
  # the assertion that matters — the un-fixed path raises the same error class
  # from the same method, so only "did a request happen" separates them.
  def test_ungoverned_get_with_zero_max_retries_still_makes_one_request
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: '{"id": 1}')

    response = http_with_max_retries(0).get("/test.json")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 1)
  end

  # The governed branch already floored the cap at one attempt; pinning it here
  # keeps the two branches from drifting apart again.
  def test_governed_get_with_zero_max_retries_still_makes_one_request
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 200, body: '{"id": 1}')

    response = http_with_max_retries(0).get("/test.json", operation: "GetProject")

    assert_equal 200, response.status
    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 1)
  end

  def test_governed_get_does_not_retry_500
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 500, body: "{}")

    # 500 is retryable in the error taxonomy but absent from the declared
    # retryOn [429, 503]; the declared set wins for governed traffic.
    assert_raises(Basecamp::Error) do
      http_with_max_retries(3).get("/test.json", operation: "GetProject")
    end

    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 1)
  end

  def test_ungoverned_get_still_retries_500
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(status: 500, body: "{}")

    assert_raises(Basecamp::Error) do
      http_with_max_retries(3).get("/test.json")
    end

    assert_requested(:get, "https://3.basecampapi.com/test.json", times: 3)
  end

  def test_paginate_follow_up_pages_stay_governed
    stub_request(:get, "https://3.basecampapi.com/test.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: {
          "Content-Type" => "application/json",
          "Link" => '<https://3.basecampapi.com/test.json?page=2>; rel="next"'
        }
      )
    stub_request(:get, "https://3.basecampapi.com/test.json?page=2")
      .to_return(status: 500, body: "{}")

    # The governance must survive onto follow-up pages: page 2's 500 is not
    # in the declared retryOn set, so it must not be retried mid-stream.
    assert_raises(Basecamp::Error) do
      http_with_max_retries(3).paginate("/test.json", operation: "ListProjects").to_a
    end

    assert_requested(:get, "https://3.basecampapi.com/test.json?page=2", times: 1)
  end
end

class HTTPPaginationTest < Minitest::Test
  include TestHelper

  def setup
    @config = default_config
    @token_provider = test_token_provider
    @http = Basecamp::Http.new(config: @config, token_provider: @token_provider)
  end

  def test_paginate_single_page
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(status: 200, body: '[{"id": 1}, {"id": 2}]')

    items = @http.paginate("/items.json").to_a

    assert_equal 2, items.length
    assert_equal 1, items[0]["id"]
    assert_equal 2, items[1]["id"]
  end

  def test_paginate_multiple_pages
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next"' }
      )

    stub_request(:get, "https://3.basecampapi.com/items.json?page=2")
      .to_return(status: 200, body: '[{"id": 2}]')

    items = @http.paginate("/items.json").to_a

    assert_equal 2, items.length
    assert_equal 1, items[0]["id"]
    assert_equal 2, items[1]["id"]
  end

  def test_paginate_returns_enumerator
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(status: 200, body: '[{"id": 1}, {"id": 2}, {"id": 3}]')

    enum = @http.paginate("/items.json")

    assert_kind_of Enumerator, enum
  end

  def test_paginate_lazy_evaluation
    # Only stub first page - second should not be called if we only take 1
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next"' }
      )

    items = @http.paginate("/items.json").take(1).to_a

    assert_equal 1, items.length
    assert_requested(:get, "https://3.basecampapi.com/items.json", times: 1)
  end

  # A page body that is not JSON at all is the "the server sent something I
  # could not decode" refusal, and the parser's own error must stay REACHABLE
  # rather than only quoted into the message (#750). Reading which page failed,
  # or whether the body was truncated mid-object, out of a sentence is the
  # substring mechanism this SDK family is getting rid of.
  def test_paginate_parse_failure_keeps_the_parser_error
    stub_request(:get, "https://3.basecampapi.com/items.json")
      .to_return(
        status: 200,
        body: '[{"id": 1}]',
        headers: { "Link" => '<https://3.basecampapi.com/items.json?page=2>; rel="next"' }
      )

    stub_request(:get, "https://3.basecampapi.com/items.json?page=2")
      .to_return(status: 200, body: '[{"id": 2')

    error = assert_raises(Basecamp::ApiError) { @http.paginate("/items.json").to_a }

    assert_kind_of JSON::ParserError, error.cause
    assert_match(/page 2/, error.message)
  end
end
