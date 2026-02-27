# frozen_string_literal: true

# Tests for the VaultsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - No list_documents(), list_uploads() - use documents/uploads service instead
# - No client-side validation (API validates)
# - Single-resource paths without .json (get, update)

require "test_helper"

class VaultsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_get
    # Generated service: /vaults/{id} without .json
    response = { "id" => 1, "title" => "Files & Documents" }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/vaults/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.get(vault_id: 2)
    assert_equal "Files & Documents", result["title"]
  end

  def test_list
    response = [ { "id" => 1, "title" => "Subfolder 1" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/vaults/\d+/vaults\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.list(vault_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Subfolder 1", result.first["title"]
  end

  def test_create
    response = { "id" => 1, "title" => "New Folder" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/vaults/\d+/vaults\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.create(vault_id: 2, title: "New Folder")
    assert_equal "New Folder", result["title"]
  end

  def test_update
    # Generated service: /vaults/{id} without .json
    response = { "id" => 1, "title" => "Updated Folder" }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/vaults/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.vaults.update(vault_id: 2, title: "Updated Folder")
    assert_equal "Updated Folder", result["title"]
  end

  # Note: list_documents() and list_uploads() not available in generated service (spec-conformant)
  # Use the DocumentsService and UploadsService to work with documents/uploads
end
