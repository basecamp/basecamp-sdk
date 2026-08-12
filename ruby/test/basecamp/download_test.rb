# frozen_string_literal: true

require "test_helper"

class DownloadTest < Minitest::Test
  def setup
    @account = create_account_client
  end

  # -- filenameFromURL tests --

  def test_filename_from_url_simple
    assert_equal "report.pdf", Basecamp.filename_from_url("https://example.com/files/report.pdf")
  end

  def test_filename_from_url_encoded
    assert_equal "my report.pdf", Basecamp.filename_from_url("https://example.com/files/my%20report.pdf")
  end

  def test_filename_from_url_trailing_slash
    assert_equal "download", Basecamp.filename_from_url("https://example.com/files/")
  end

  def test_filename_from_url_no_path
    assert_equal "download", Basecamp.filename_from_url("https://example.com")
  end

  def test_filename_from_url_empty
    assert_equal "download", Basecamp.filename_from_url("")
  end

  def test_filename_from_url_deep_path
    assert_equal "notes.txt", Basecamp.filename_from_url("https://example.com/a/b/c/notes.txt")
  end

  def test_filename_from_url_with_query
    assert_equal "image.png", Basecamp.filename_from_url("https://example.com/image.png?size=large")
  end

  def test_filename_from_url_root_path
    assert_equal "download", Basecamp.filename_from_url("https://example.com/")
  end

  # -- Validation tests --

  def test_download_url_empty_raises_usage_error
    error = assert_raises(Basecamp::UsageError) { @account.download_url("") }
    assert_equal "usage", error.code
  end

  def test_download_url_nil_raises_usage_error
    error = assert_raises(Basecamp::UsageError) { @account.download_url(nil) }
    assert_equal "usage", error.code
  end

  def test_download_url_relative_raises_usage_error
    error = assert_raises(Basecamp::UsageError) { @account.download_url("/just/a/path") }
    assert_equal "usage", error.code
  end

  # -- URL rewriting tests --

  def test_download_url_rewrites_origin
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/report.pdf")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 200,
        body: "file-content",
        headers: { "Content-Type" => "application/pdf", "Content-Length" => "12" }
      )

    result = @account.download_url("https://other-host.example.com/12345/attachments/abc/download/report.pdf")
    assert_equal "file-content", result.body
    assert_equal "application/pdf", result.content_type
  end

  def test_download_url_host_agnostic
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 200, body: "content", headers: { "Content-Type" => "text/plain" })

    # Works regardless of the incoming origin
    result = @account.download_url("https://completely-different.com/12345/attachments/abc/download/file.txt")
    assert_equal "content", result.body
  end

  def test_download_url_preserves_query_params
    stub_request(:get, "#{base_url}/12345/download?token=abc&v=2")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "application/octet-stream" })

    result = @account.download_url("https://any-host.com/12345/download?token=abc&v=2")
    assert_equal "data", result.body
  end

  # -- Redirect flow tests --

  def test_download_url_redirect_flow
    # Hop 1: API returns 302 redirect
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/report.pdf")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 302,
        headers: { "Location" => "https://s3.amazonaws.com/bucket/signed-file?sig=xyz" }
      )

    # Hop 2: S3 returns the file
    stub_request(:get, "https://s3.amazonaws.com/bucket/signed-file?sig=xyz")
      .to_return(
        status: 200,
        body: "pdf-content",
        headers: { "Content-Type" => "application/pdf", "Content-Length" => "11" }
      )

    result = @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/report.pdf")
    assert_equal "pdf-content", result.body
    assert_equal "application/pdf", result.content_type
    assert_equal 11, result.content_length
    assert_equal "report.pdf", result.filename
  end

  def test_download_url_direct_download
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 200,
        body: "direct-content",
        headers: { "Content-Type" => "text/plain", "Content-Length" => "14" }
      )

    result = @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    assert_equal "direct-content", result.body
    assert_equal "text/plain", result.content_type
    assert_equal 14, result.content_length
    assert_equal "file.txt", result.filename
  end

  def test_download_url_relative_location
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 302,
        headers: { "Location" => "/signed/file.txt" }
      )

    stub_request(:get, "#{base_url}/signed/file.txt")
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "text/plain" })

    result = @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    assert_equal "data", result.body
  end

  def test_download_url_redirect_no_location
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 302, headers: {})

    assert_raises(Basecamp::ApiError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end
  end

  # The redirect Location is server-supplied, and hop 2 is deliberately
  # cross-origin (signed storage), so no same_origin? gate stands between the
  # resolved target and the dial. A scheme-only "mailto:" — which
  # Security.resolve_url returns verbatim for exactly this refusal — escaped
  # as a raw URI::InvalidComponentError crash from fetch_signed_download's
  # URI.parse before it gained its own guards.
  def test_download_url_redirect_to_unparseable_location_is_refused
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 302, headers: { "Location" => "mailto:" })

    error = assert_raises(Basecamp::ApiError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end
    assert_match(/invalid download URL/, error.message)
  end

  def test_download_url_redirect_to_non_http_scheme_is_refused
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 302, headers: { "Location" => "ftp://storage.example/file" })

    error = assert_raises(Basecamp::ApiError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end
    assert_match(/non-HTTP\(S\) download URL/, error.message)
  end

  # -- Error tests --

  def test_download_url_api_404
    stub_request(:get, "#{base_url}/12345/attachments/missing/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 404, body: '{"error": "Not found"}', headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/missing/download/file.txt")
    end
  end

  def test_download_url_api_403
    stub_request(:get, "#{base_url}/12345/attachments/secret/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 403, body: '{"error": "Forbidden"}', headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ForbiddenError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/secret/download/file.txt")
    end
  end

  def test_download_url_api_500
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 500, body: '{"error": "Server error"}', headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ApiError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end
  end

  def test_download_url_s3_error
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 302,
        headers: { "Location" => "https://s3.amazonaws.com/bucket/file" }
      )

    stub_request(:get, "https://s3.amazonaws.com/bucket/file")
      .to_return(status: 403, body: "AccessDenied")

    assert_raises(Basecamp::ApiError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end
  end

  # -- Auth header tests --

  def test_download_url_auth_on_api_not_on_s3
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 302,
        headers: { "Location" => "https://s3.amazonaws.com/bucket/file" }
      )

    # S3 stub must NOT have an Authorization header
    s3_stub = stub_request(:get, "https://s3.amazonaws.com/bucket/file")
      .with { |req| req.headers["Authorization"].nil? }
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "application/octet-stream" })

    @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")

    # API leg had auth (verified by .with(headers:) on the stub)
    assert_requested(:get, "#{base_url}/12345/attachments/abc/download/file.txt", times: 1)
    # S3 leg had no auth header (verified by .with block on s3_stub)
    assert_requested(s3_stub)
  end

  # -- Hook tests --

  def test_download_url_operation_hooks
    ops_started = []
    ops_ended = []

    hooks_impl = Class.new do
      include Basecamp::Hooks
      define_method(:on_operation_start) { |info| ops_started << info }
      define_method(:on_operation_end) { |info, result| ops_ended << [ info, result ] }
    end.new

    account = create_account_client(hooks: hooks_impl)

    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "text/plain" })

    account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")

    assert_equal 1, ops_started.length
    assert_equal "Account", ops_started[0].service
    assert_equal "DownloadURL", ops_started[0].operation

    assert_equal 1, ops_ended.length
    assert_nil ops_ended[0][1].error
  end

  def test_download_url_request_hooks_api_only
    requests_started = []
    requests_ended = []

    hooks_impl = Class.new do
      include Basecamp::Hooks
      define_method(:on_request_start) { |info| requests_started << info }
      define_method(:on_request_end) { |info, result| requests_ended << [ info, result ] }
    end.new

    account = create_account_client(hooks: hooks_impl)

    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 302,
        headers: { "Location" => "https://s3.amazonaws.com/bucket/file" }
      )

    stub_request(:get, "https://s3.amazonaws.com/bucket/file")
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "text/plain" })

    account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")

    # Request hooks fire for hop 1 only
    assert_equal 1, requests_started.length
    assert_equal 1, requests_ended.length
    assert_equal "GET", requests_started[0].method
  end

  # -- Network failure tests --

  def test_download_url_hop1_network_failure
    requests_ended = []

    hooks_impl = Class.new do
      include Basecamp::Hooks
      define_method(:on_request_end) { |info, result| requests_ended << [ info, result ] }
    end.new

    # max_retries: 1 pins the per-attempt hook contract without backoff; the
    # retry-enabled hook shape is pinned by the balanced-hooks test below.
    account = create_account_client(config: fast_download_config(max_retries: 1), hooks: hooks_impl)

    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .to_timeout

    error = assert_raises(Basecamp::NetworkError) do
      account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end

    assert_equal "network", error.code

    # on_request_end fires with status_code nil (network failure)
    assert_equal 1, requests_ended.length
    assert_nil requests_ended[0][1].status_code
  end

  def test_download_url_hop2_network_failure
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(
        status: 302,
        headers: { "Location" => "https://s3.amazonaws.com/bucket/file" }
      )

    stub_request(:get, "https://s3.amazonaws.com/bucket/file")
      .to_timeout

    error = assert_raises(Basecamp::NetworkError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end

    assert_equal "network", error.code
  end

  # -- Hop-1 retry policy (SPEC §14) --

  HOP1_URL = "https://3.basecampapi.com/12345/attachments/abc/download/file.txt"
  HOP1_PATH = "/12345/attachments/abc/download/file.txt"
  SIGNED_URL = "https://s3.amazonaws.com/bucket/signed-file"

  # Millisecond backoff so the retry tables run without real one-second sleeps.
  def fast_download_config(**overrides)
    Basecamp::Config.new(
      **{ base_url: base_url, timeout: 5, max_retries: 3, base_delay: 0.001, max_jitter: 0.0 }.merge(overrides)
    )
  end

  # The COMPLETE declared retry set, pinned status by status (the shared
  # conformance fixtures cover 429/503). Hop 1 retries {429, 502, 503, 504}
  # under the public max_retries total-attempt cap, floored at one.
  [ 429, 502, 503, 504 ].each do |status|
    define_method("test_download_url_retries_#{status}_then_follows_redirect") do
      stub_request(:get, "#{base_url}#{HOP1_PATH}")
        .with(headers: { "Authorization" => "Bearer #{access_token}" })
        .to_return(status: status, body: "{}", headers: { "Content-Type" => "application/json" })
        .then.to_return(status: 302, headers: { "Location" => SIGNED_URL })

      stub_request(:get, SIGNED_URL)
        .to_return(status: 200, body: "data", headers: { "Content-Type" => "application/octet-stream" })

      account = create_account_client(config: fast_download_config)
      result = account.download_url(HOP1_URL)

      assert_equal "data", result.body
      assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 2)
    end
  end

  def test_download_url_never_retries_500
    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 500, body: '{"error":"Server error"}', headers: { "Content-Type" => "application/json" })

    account = create_account_client(config: fast_download_config)
    assert_raises(Basecamp::ApiError) { account.download_url(HOP1_URL) }

    # 500 is deliberately outside the declared set {429, 502, 503, 504}
    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 1)
  end

  def test_download_url_retries_network_error_then_succeeds
    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_timeout
      .then.to_return(status: 200, body: "content", headers: { "Content-Type" => "text/plain" })

    account = create_account_client(config: fast_download_config)
    result = account.download_url(HOP1_URL)

    assert_equal "content", result.body
    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 2)
  end

  def test_download_url_exhausts_cap_then_surfaces_error
    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 503, body: "{}", headers: { "Content-Type" => "application/json" })

    account = create_account_client(config: fast_download_config(max_retries: 3))
    assert_raises(Basecamp::ApiError) { account.download_url(HOP1_URL) }

    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 3)
  end

  def test_download_url_zero_max_retries_still_sends_one_attempt
    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 503, body: "{}", headers: { "Content-Type" => "application/json" })

    # The download attempt budget is floored at one: max_retries: 0 still
    # sends exactly one request. The same floor now applies on every Ruby
    # request path, governed or ungoverned (#532).
    account = create_account_client(config: fast_download_config(max_retries: 0))
    assert_raises(Basecamp::ApiError) { account.download_url(HOP1_URL) }

    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 1)
  end

  def test_download_url_auth_on_every_hop1_attempt_never_on_hop2
    # The .with(headers:) matcher applies to every attempt: an unauthenticated
    # retry would not match this stub and would raise as unstubbed.
    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 503, body: "{}", headers: { "Content-Type" => "application/json" })
      .then.to_return(status: 302, headers: { "Location" => SIGNED_URL })

    s3_stub = stub_request(:get, SIGNED_URL)
      .with { |req| req.headers["Authorization"].nil? }
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "application/octet-stream" })

    account = create_account_client(config: fast_download_config)
    account.download_url(HOP1_URL)

    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 2)
    assert_requested(s3_stub)
  end

  def refreshable_provider
    Class.new do
      attr_reader :refreshes

      def initialize = @refreshes = 0
      def access_token = "test-token"
      def refreshable? = true

      def refresh
        @refreshes += 1
        true
      end
    end.new
  end

  # SPEC §4/§14: with retry disabled ONE request goes out — refresh included.
  # The replay is a request on the wire, so it spends an attempt from the same
  # total-attempt budget as a transient retry (#565, #461).
  #
  # The refresh itself must not fire either: §4 checks the budget BEFORE
  # refreshing, because rotating a token the SDK has no attempt left to use
  # burns it for nothing and still surfaces the stale 401.
  def test_download_url_refresh_replay_is_not_attempted_without_budget
    provider = refreshable_provider
    stub_request(:get, "#{base_url}#{HOP1_PATH}").to_return(status: 401)

    account = create_account_client(config: fast_download_config(max_retries: 0), token_provider: provider)
    assert_raises(Basecamp::AuthError) { account.download_url(HOP1_URL) }

    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 1)
    assert_equal 0, provider.refreshes
  end

  # A budget of two allows the replay, and it consumes attempt 2.
  def test_download_url_refresh_replay_spends_an_attempt_from_the_budget
    provider = refreshable_provider
    stub_request(:get, "#{base_url}#{HOP1_PATH}").to_return(status: 401)

    account = create_account_client(config: fast_download_config(max_retries: 2), token_provider: provider)
    assert_raises(Basecamp::AuthError) { account.download_url(HOP1_URL) }

    # Two requests, not three: the replay drew from the same budget, and §4
    # allows only one refresh per request regardless.
    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 2)
    assert_equal 1, provider.refreshes
  end

  # The refreshed request is actually SENT, not refreshed-then-discarded.
  def test_download_url_refresh_replay_succeeds_when_the_new_token_works
    provider = refreshable_provider
    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .to_return(status: 401)
      .then.to_return(status: 302, headers: { "Location" => SIGNED_URL })
    s3_stub = stub_request(:get, SIGNED_URL)
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "application/octet-stream" })

    account = create_account_client(config: fast_download_config(max_retries: 2), token_provider: provider)
    result = account.download_url(HOP1_URL)

    assert_equal "data", result.body
    assert_requested(:get, "#{base_url}#{HOP1_PATH}", times: 2)
    assert_requested(s3_stub)
    assert_equal 1, provider.refreshes
  end

  # SPEC §14: hop 1 carries Authorization and User-Agent only. A binary
  # download is not a JSON API call, so the generic request path's
  # "Accept: application/json" must not ride along — on the first attempt or
  # on a retry.
  def test_download_url_hop1_sends_no_json_accept_on_any_attempt
    hop1_accepts = []

    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with { |req| hop1_accepts << req.headers["Accept"]; true }
      .to_return(status: 503, body: "{}", headers: { "Content-Type" => "application/json" })
      .then.to_return(status: 302, headers: { "Location" => SIGNED_URL })

    stub_request(:get, SIGNED_URL)
      .to_return(status: 200, body: "data", headers: { "Content-Type" => "application/octet-stream" })

    account = create_account_client(config: fast_download_config)
    account.download_url(HOP1_URL)

    # Faraday supplies its own "*/*" default once the SDK stops setting an
    # Accept, so the header cannot be absent outright without overriding the
    # HTTP library. Pin the exact value rather than merely "not JSON": that
    # catches both a reintroduced application/json and any other Accept the
    # SDK might start attaching.
    assert_equal 2, hop1_accepts.length
    hop1_accepts.each do |accept|
      assert_equal "*/*", accept,
        "hop 1 must carry only Faraday's default Accept, never one the SDK sets (SPEC §14)"
    end
  end

  def test_download_url_balanced_hooks_across_retries
    starts = []
    ends = []
    retries = []

    hooks_impl = Class.new do
      include Basecamp::Hooks
      define_method(:on_request_start) { |info| starts << info.attempt }
      define_method(:on_request_end) { |info, _result| ends << info.attempt }
      define_method(:on_retry) { |_info, attempt, _error, _delay| retries << attempt }
    end.new

    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 503, body: "{}", headers: { "Content-Type" => "application/json" })
      .then.to_return(status: 503, body: "{}", headers: { "Content-Type" => "application/json" })
      .then.to_return(status: 200, body: "content", headers: { "Content-Type" => "text/plain" })

    account = create_account_client(config: fast_download_config, hooks: hooks_impl)
    account.download_url(HOP1_URL)

    assert_equal [ 1, 2, 3 ], starts
    assert_equal [ 1, 2, 3 ], ends
    # on_retry receives the UPCOMING attempt (SPEC §7 attempt semantics).
    assert_equal [ 2, 3 ], retries
  end

  def test_download_url_honors_retry_after_on_429
    delays = []

    stub_request(:get, "#{base_url}#{HOP1_PATH}")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 429, body: "{}", headers: { "Content-Type" => "application/json", "Retry-After" => "7" })
      .then.to_return(status: 200, body: "content", headers: { "Content-Type" => "text/plain" })

    http = Basecamp::Http.new(config: fast_download_config, token_provider: token_provider)
    http.define_singleton_method(:sleep) { |delay| delays << delay }

    response = http.get_download("#{base_url}#{HOP1_PATH}")

    assert_equal 200, response.status
    assert_equal [ 7 ], delays
  end
end
