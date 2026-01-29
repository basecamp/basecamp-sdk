# frozen_string_literal: true

require "test_helper"

class VaultsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get
    response = { "id" => 1, "title" => "Files & Documents" }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/vaults/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.get(project_id: 1, vault_id: 2)
    assert_equal "Files & Documents", result["title"]
  end

  def test_list
    response = [ { "id" => 1, "title" => "Subfolder 1" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/vaults/\d+/vaults\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.list(project_id: 1, vault_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Subfolder 1", result.first["title"]
  end

  def test_create
    response = { "id" => 1, "title" => "New Folder" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/\d+/vaults/\d+/vaults\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.create(project_id: 1, vault_id: 2, title: "New Folder")
    assert_equal "New Folder", result["title"]
  end

  def test_update
    response = { "id" => 1, "title" => "Updated Folder" }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/buckets/\d+/vaults/\d+})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.update(project_id: 1, vault_id: 2, title: "Updated Folder")
    assert_equal "Updated Folder", result["title"]
  end

  def test_list_documents
    response = [ { "id" => 1, "title" => "Doc 1" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/vaults/\d+/documents\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.list_documents(project_id: 1, vault_id: 2).to_a
    assert_kind_of Array, result
  end

  def test_list_uploads
    response = [ { "id" => 1, "filename" => "file.pdf" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/vaults/\d+/uploads\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.list_uploads(project_id: 1, vault_id: 2).to_a
    assert_kind_of Array, result
  end
end
