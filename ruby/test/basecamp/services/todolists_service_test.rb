# frozen_string_literal: true

# Tests for the TodolistsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Uses :id instead of :todolist_id for single-resource operations
# - No .json extension for single-resource paths

require "test_helper"

class TodolistsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "name" => "Sprint Tasks", "completed_ratio" => "3/10" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.list(todoset_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Sprint Tasks", result.first["name"]
  end

  def test_list_with_status
    response = [ { "id" => 1, "name" => "Archived List", "status" => "archived" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json\?status=archived})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.list(todoset_id: 2, status: "archived").to_a
    assert_equal "archived", result.first["status"]
  end

  def test_get
    response = { "id" => 2, "name" => "Sprint Tasks" }

    # Generated service uses /todolists/{id} without .json
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.get(id: 2)
    assert_equal "Sprint Tasks", result["name"]
  end

  def test_create
    response = { "id" => 1, "name" => "New List" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.create(todoset_id: 2, name: "New List")
    assert_equal "New List", result["name"]
  end

  def test_update
    response = { "id" => 2, "name" => "Updated List" }

    # Generated service uses /todolists/{id} without .json
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.update(id: 2, name: "Updated List")
    assert_equal "Updated List", result["name"]
  end
end
