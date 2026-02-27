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
end
