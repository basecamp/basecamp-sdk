# frozen_string_literal: true

# Tests for the TodolistGroupsService (generated from OpenAPI spec)
#
# Note: Generated services are spec-conformant:
# - Only list(), create(), reposition() available
# - No get(), update(), trash() - use recordings.trash() for deletion
# - No client-side validation (API validates)

require "test_helper"

class TodolistGroupsServiceTest < Minitest::Test
  include TestHelper

  def setup
    @account = create_account_client(account_id: "12345")
  end

  def test_list
    response = [ { "id" => 1, "name" => "Phase 1" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+/groups\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.list(todolist_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Phase 1", result.first["name"]
  end

  def test_create
    response = { "id" => 1, "name" => "New Group" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/todolists/\d+/groups\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.create(todolist_id: 2, name: "New Group")
    assert_equal "New Group", result["name"]
  end

  def test_reposition
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todolists/\d+/position\.json})
      .to_return(status: 204)

    result = @account.todolist_groups.reposition(group_id: 2, position: 1)
    assert_nil result
  end

  # Note: get(), update() not available in generated service (spec-conformant)
  # Note: trash() is on RecordingsService - use recordings.trash(project_id:, recording_id:)
  # Note: No client-side validation - API validates
end
