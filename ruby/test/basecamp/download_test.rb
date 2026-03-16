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

    account = create_account_client(hooks: hooks_impl)

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

  # -- No retry on 429 --

  def test_download_url_no_retry_on_429
    stub_request(:get, "#{base_url}/12345/attachments/abc/download/file.txt")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 429, body: '{"error":"Rate limited"}', headers: { "Content-Type" => "application/json", "Retry-After" => "30" })

    error = assert_raises(Basecamp::RateLimitError) do
      @account.download_url("https://3.basecampapi.com/12345/attachments/abc/download/file.txt")
    end

    assert_equal "rate_limit", error.code

    # Should have been called exactly once (no retry)
    assert_requested(:get, "#{base_url}/12345/attachments/abc/download/file.txt", times: 1)
  end
end
