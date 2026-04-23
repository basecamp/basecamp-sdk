# frozen_string_literal: true

require "test_helper"

class UploadsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "filename" => "report.pdf", "byte_size" => 1024 } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/vaults/\d+/uploads\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.list(vault_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "report.pdf", result.first["filename"]
  end

  def test_get
    response = { "id" => 1, "filename" => "report.pdf" }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/uploads/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.get(upload_id: 2)
    assert_equal "report.pdf", result["filename"]
  end

  def test_create
    response = { "id" => 1, "filename" => "new-report.pdf" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/vaults/\d+/uploads\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.create(
      vault_id: 2,
      attachable_sgid: "BAh7CEkiCGdpZAY6BkVUSSIvZ2lk..."
    )
    assert_equal "new-report.pdf", result["filename"]
  end

  def test_create_with_subscriptions
    response = { "id" => 2, "filename" => "report.pdf" }

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
    response = { "id" => 1, "description" => "Updated description" }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/uploads/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.update(upload_id: 2, description: "Updated description")
    assert_equal "Updated description", result["description"]
  end

  def test_list_versions
    response = [ { "id" => 1, "version" => 1 }, { "id" => 2, "version" => 2 } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/uploads/\d+/versions\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.uploads.list_versions(upload_id: 2).to_a
    assert_equal 2, result.length
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
