# frozen_string_literal: true

require "test_helper"

class TodolistGroupsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "name" => "Phase 1" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/todolists/\d+/groups\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.list(project_id: 1, todolist_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Phase 1", result.first["name"]
  end

  def test_get
    response = { "id" => 1, "name" => "Phase 1" }

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/buckets/\d+/todolists/\d+\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.get(project_id: 1, group_id: 2)
    assert_equal "Phase 1", result["name"]
  end

  def test_create
    response = { "id" => 1, "name" => "New Group" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/buckets/\d+/todolists/\d+/groups\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.create(project_id: 1, todolist_id: 2, name: "New Group")
    assert_equal "New Group", result["name"]
  end

  def test_create_requires_name
    assert_raises ArgumentError do
      @account.todolist_groups.create(project_id: 1, todolist_id: 2, name: "")
    end
  end

  def test_update
    response = { "id" => 1, "name" => "Updated Group" }

    stub_request(:put, %r{https://3\.basecampapi\.com/12345/buckets/\d+/todolists/\d+\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.update(project_id: 1, group_id: 2, name: "Updated Group")
    assert_equal "Updated Group", result["name"]
  end

  def test_reposition
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/buckets/\d+/todolists/\d+/position\.json})
      .to_return(status: 204)

    result = @account.todolist_groups.reposition(project_id: 1, group_id: 2, position: 1)
    assert_nil result
  end

  def test_reposition_requires_positive_position
    assert_raises ArgumentError do
      @account.todolist_groups.reposition(project_id: 1, group_id: 2, position: 0)
    end
  end

  def test_trash
    stub_request(:delete, %r{https://3\.basecampapi\.com/12345/buckets/\d+/recordings/\d+/status/trashed\.json})
      .to_return(status: 204)

    result = @account.todolist_groups.trash(project_id: 1, group_id: 2)
    assert_nil result
  end
end
