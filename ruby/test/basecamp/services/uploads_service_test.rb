# frozen_string_literal: true

require "test_helper"

class UploadsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "filename" => "report.pdf", "byte_size" => 1024, "description_attachments" => [] } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/vaults/\d+/uploads\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.list(vault_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "report.pdf", result.first["filename"]
  end

  def test_get
    response = { "id" => 1, "filename" => "report.pdf", "description_attachments" => [] }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/uploads/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.get(upload_id: 2)
    assert_equal "report.pdf", result["filename"]
  end

  def test_create
    response = { "id" => 1, "filename" => "new-report.pdf", "description_attachments" => [] }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/vaults/\d+/uploads\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.create(
      vault_id: 2,
      attachable_sgid: "BAh7CEkiCGdpZAY6BkVUSSIvZ2lk..."
    )
    assert_equal "new-report.pdf", result["filename"]
  end

  def test_create_with_subscriptions
    response = { "id" => 2, "filename" => "report.pdf", "description_attachments" => [] }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/vaults/\d+/uploads\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    @account.uploads.create(
      vault_id: 2,
      attachable_sgid: "BAh7CEkiCGdpZAY6BkVUSSIvZ2lk...",
      subscriptions: [ 111, 222 ]
    )

    assert_requested(:post, %r{https://3\.basecampapi\.com/12345/vaults/\d+/uploads\.json},
      body: hash_including("subscriptions" => [ 111, 222 ]))
  end

  def test_update
    response = { "id" => 1, "description" => "Updated description", "description_attachments" => [] }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/uploads/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.update(upload_id: 2, description: "Updated description")
    assert_equal "Updated description", result["description"]
  end

  # The endpoint returns EVENTS, not Uploads — the retype that closes #649.
  def test_list_versions
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/uploads/\d+/versions\.json})
      .to_return(status: 200, body: load_fixture("uploads/versions.json").to_json,
        headers: { "Content-Type" => "application/json" })

    result = @account.uploads.list_versions(upload_id: 2).to_a

    assert_equal 3, result.length
    assert_equal "blob_changed", result.first["action"]
    assert_equal "company-logo.png", result.first["upload"]["filename"]
    assert_equal 184829, result.first["upload"]["byte_size"]
  end

  def test_list_versions_marks_exactly_one_current
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/uploads/\d+/versions\.json})
      .to_return(status: 200, body: load_fixture("uploads/versions.json").to_json,
        headers: { "Content-Type" => "application/json" })

    result = @account.uploads.list_versions(upload_id: 2).to_a

    assert_equal 1, result.count { |v| v.dig("upload", "current") }
    assert result.first.dig("upload", "current")
  end

  # A version whose recordable no longer resolves omits the upload object
  # entirely; the partial's `if upload = uploads[...]` is false.
  def test_list_versions_tolerates_a_missing_recordable
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/uploads/\d+/versions\.json})
      .to_return(status: 200, body: load_fixture("uploads/versions.json").to_json,
        headers: { "Content-Type" => "application/json" })

    result = @account.uploads.list_versions(upload_id: 2).to_a

    assert_equal "created", result.last["action"]
    assert_nil result.last["upload"]
  end

  def test_create_version_posts_the_attachable_sgid
    stub_request(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json")
      .to_return(status: 201, body: { "id" => 2, "filename" => "company-logo.png" }.to_json,
        headers: { "Content-Type" => "application/json" })

    result = @account.uploads.create_version(upload_id: 2, attachable_sgid: "sgid-abc")

    assert_equal 2, result["id"]
    assert_requested(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json") do |req|
      JSON.parse(req.body)["attachable_sgid"] == "sgid-abc"
    end
  end

  # Presence-aware: omitted carries the previous description forward, "" clears.
  def test_create_version_omits_an_unaddressed_description
    stub_request(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json")
      .to_return(status: 201, body: { "id" => 2 }.to_json, headers: { "Content-Type" => "application/json" })

    @account.uploads.create_version(upload_id: 2, attachable_sgid: "sgid-abc")

    assert_requested(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json") do |req|
      body = JSON.parse(req.body)
      !body.key?("description") && !body.key?("base_name")
    end
  end

  def test_create_version_sends_an_explicit_blank_description_to_clear_it
    stub_request(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json")
      .to_return(status: 201, body: { "id" => 2 }.to_json, headers: { "Content-Type" => "application/json" })

    @account.uploads.create_version(upload_id: 2, attachable_sgid: "sgid-abc", description: "")

    assert_requested(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json") do |req|
      body = JSON.parse(req.body)
      body.key?("description") && body["description"] == ""
    end
  end

  def test_create_version_passes_notify_and_subscriptions
    stub_request(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json")
      .to_return(status: 201, body: { "id" => 2 }.to_json, headers: { "Content-Type" => "application/json" })

    @account.uploads.create_version(upload_id: 2, attachable_sgid: "sgid-abc",
      notify: "custom", subscriptions: [ 1049715915 ])

    assert_requested(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json") do |req|
      body = JSON.parse(req.body)
      body["notify"] == "custom" && body["subscriptions"] == [ 1049715915 ]
    end
  end

  # A replacement copies bytes into a new blob and keeps the old one, so it
  # always grows recorded storage. 507 is a limit, never a transient failure.
  def test_create_version_reports_a_storage_limit_as_limit_exceeded
    stub_request(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json")
      .to_return(status: 507,
        body: { "error" => "The storage limit for this account has been reached." }.to_json,
        headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::LimitExceededError) do
      @account.uploads.create_version(upload_id: 2, attachable_sgid: "sgid-abc")
    end

    assert_equal "limit_exceeded", error.code
    assert_equal 10, error.exit_code
    refute error.retryable?
    assert_match(/storage limit/, error.message)
    assert_requested(:post, "https://3.basecampapi.com/12345/uploads/2/versions.json", times: 1)
  end

  def test_download_delegates_through_download_url
    metadata = {
      "id" => 1069479400,
      "filename" => "report.pdf",
      "download_url" => "https://storage.example/12345/blobs/abc/download/report.pdf"
    }
    stub_request(:get, "#{base_url}/12345/uploads/1069479400")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 200, body: metadata.to_json, headers: { "Content-Type" => "application/json" })

    # Hop 1: auth'd, origin-rewritten; returns 302
    stub_request(:get, "#{base_url}/12345/blobs/abc/download/report.pdf")
      .with(headers: { "Authorization" => "Bearer #{access_token}" })
      .to_return(status: 302, headers: { "Location" => "https://signed.example/bucket/xyz" })

    # Hop 2: signed URL, no auth
    stub_request(:get, "https://signed.example/bucket/xyz")
      .to_return(
        status: 200,
        body: "pdf-bytes",
        headers: { "Content-Type" => "application/pdf", "Content-Length" => "9" }
      )

    result = @account.uploads.download(upload_id: 1069479400)

    assert_equal "pdf-bytes", result.body
    assert_equal "application/pdf", result.content_type
    # filename from upload metadata wins over URL-derived filename
    assert_equal "report.pdf", result.filename
  end

  def test_download_raises_when_metadata_has_no_download_url
    metadata = { "id" => 1069479400, "filename" => "report.pdf", "download_url" => nil }
    stub_request(:get, "#{base_url}/12345/uploads/1069479400")
      .to_return(status: 200, body: metadata.to_json, headers: { "Content-Type" => "application/json" })

    error = assert_raises(Basecamp::UsageError) do
      @account.uploads.download(upload_id: 1069479400)
    end
    assert_match(/1069479400/, error.message)
    assert_match(/download_url/, error.message)
  end
end
