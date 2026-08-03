# frozen_string_literal: true

require "test_helper"

class FoldersServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  # The list shape: base folder fields, no "projects". The wire "type" is
  # "Stack", not "Folder" — the product was renamed, the payload was not.
  def sample_folder(id:, name: "Client work")
    {
      "id" => id,
      "name" => name,
      "type" => "Stack",
      "created_at" => "2026-07-27T10:16:49.312Z",
      "updated_at" => "2026-07-27T10:16:49.325Z",
      "bucket_ids" => [ 201, 202 ],
      "is_emoji_only_name" => false,
      "star_url" => "https://3.basecampapi.com/12345/buckets/#{id}/stars.json",
      "gauges_url" => nil,
      "color" => nil,
      "image_url" => nil,
      "url" => "https://3.basecampapi.com/12345/stacks/#{id}.json"
    }
  end

  def sample_project(id:, name:)
    {
      "id" => id,
      "status" => "active",
      "created_at" => "2026-06-01T00:00:00Z",
      "updated_at" => "2026-06-02T00:00:00Z",
      "name" => name,
      "description" => "",
      "purpose" => "topic",
      "clients_enabled" => false,
      "bookmark_url" => "https://3.basecampapi.com/12345/my/bookmarks/abc.json",
      "url" => "https://3.basecampapi.com/12345/projects/#{id}.json",
      "app_url" => "https://3.basecamp.com/12345/projects/#{id}"
    }
  end

  # The get/create/update shape: the base folder plus the expanded projects.
  def sample_folder_with_projects(id:, name: "Client work")
    sample_folder(id: id, name: name).merge(
      "projects" => [
        sample_project(id: 201, name: "Refresh"),
        sample_project(id: 202, name: "Nike promotion")
      ]
    )
  end

  def test_list_folders_returns_a_bare_array_without_projects
    stub_get("/12345/stacks.json",
             response_body: [ sample_folder(id: 1), sample_folder(id: 2, name: "Personal") ])

    result = @account.folders.list_folders

    assert_equal 2, result.length
    assert_equal 1, result[0]["id"]
    assert_equal "Stack", result[0]["type"]
    assert_equal [ 201, 202 ], result[0]["bucket_ids"]
    assert_not result[0].key?("projects")
  end

  def test_list_folders_decodes_the_always_present_nullable_fields
    stub_get("/12345/stacks.json", response_body: [ sample_folder(id: 1) ])

    folder = @account.folders.list_folders.first

    assert folder.key?("gauges_url")
    assert_nil folder["gauges_url"]
    assert_nil folder["color"]
    assert_nil folder["image_url"]
  end

  def test_list_folders_raises_on_401
    stub_request(:get, "https://3.basecampapi.com/12345/stacks.json")
      .to_return(status: 401, body: { "error" => "Unauthorized" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::AuthError) do
      @account.folders.list_folders
    end
  end

  def test_get_folder_expands_its_projects
    stub_get("/12345/stacks/1", response_body: sample_folder_with_projects(id: 1))

    folder = @account.folders.get_folder(folder_id: 1)

    assert_equal 1, folder["id"]
    assert_equal 2, folder["projects"].length
    assert_equal "Refresh", folder["projects"][0]["name"]
    assert_equal folder["bucket_ids"], folder["projects"].map { |p| p["id"] }
  end

  def test_get_folder_raises_not_found
    stub_request(:get, "https://3.basecampapi.com/12345/stacks/999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.folders.get_folder(folder_id: 999)
    end
  end

  def test_create_folder_sends_project_ids_and_returns_the_expanded_folder
    stub_request(:post, "https://3.basecampapi.com/12345/stacks.json")
      .with(body: { name: "Client work", project_ids: [ 201, 202 ] }.to_json)
      .to_return(status: 201, body: sample_folder_with_projects(id: 7).to_json,
                 headers: { "Content-Type" => "application/json" })

    folder = @account.folders.create_folder(name: "Client work", project_ids: [ 201, 202 ])

    assert_equal 7, folder["id"]
    assert_equal 2, folder["projects"].length
  end

  # An unreachable project id fails the whole request and writes nothing.
  def test_create_folder_raises_not_found_for_an_unreachable_project
    stub_request(:post, "https://3.basecampapi.com/12345/stacks.json")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.folders.create_folder(name: "Mixed", project_ids: [ 201, 999_999_999 ])
    end
  end

  def test_update_folder_renames_and_returns_projects
    stub_request(:put, "https://3.basecampapi.com/12345/stacks/1")
      .with(body: { name: "Active client work" }.to_json)
      .to_return(status: 200,
                 body: sample_folder_with_projects(id: 1, name: "Active client work").to_json,
                 headers: { "Content-Type" => "application/json" })

    folder = @account.folders.update_folder(folder_id: 1, name: "Active client work")

    assert_equal "Active client work", folder["name"]
    assert_equal 2, folder["projects"].length
  end

  def test_update_folder_raises_validation_error_for_a_blank_name
    stub_request(:put, "https://3.basecampapi.com/12345/stacks/1")
      .to_return(status: 422, body: { "errors" => { "name" => [ "can't be blank" ] } }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::ValidationError) do
      @account.folders.update_folder(folder_id: 1, name: "   ")
    end
  end

  def test_delete_folder_returns_no_content
    stub_request(:delete, "https://3.basecampapi.com/12345/stacks/1")
      .to_return(status: 204, body: "")

    assert_nil @account.folders.delete_folder(folder_id: 1)

    assert_requested(:delete, "https://3.basecampapi.com/12345/stacks/1")
  end

  def test_delete_folder_raises_not_found
    stub_request(:delete, "https://3.basecampapi.com/12345/stacks/999")
      .to_return(status: 404, body: { "error" => "Not found" }.to_json,
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Basecamp::NotFoundError) do
      @account.folders.delete_folder(folder_id: 999)
    end
  end
end
