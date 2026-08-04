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
    # color and comments_app_url are @required on Todolist (#630), and a group
    # IS a Todolist. color is null here because an uncolored group is the
    # ordinary case — the key is still always emitted.
    response = [ { "id" => 1, "name" => "Phase 1", "color" => nil,
                   "comments_app_url" => "https://3.basecamp.com/12345/buckets/1/recordings/1/comments" } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+/groups\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.list(todolist_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Phase 1", result.first["name"]
  end

  def test_create
    response = { "id" => 1, "name" => "New Group", "color" => nil,
                 "comments_app_url" => "https://3.basecamp.com/12345/buckets/1/recordings/1/comments" }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/todolists/\d+/groups\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.create(todolist_id: 2, name: "New Group")
    assert_equal "New Group", result["name"]
  end

  def test_reposition
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todolists/groups/\d+/position\.json})
      .to_return(status: 204)

    result = @account.todolist_groups.reposition(group_id: 2, position: 1)
    assert_nil result
  end

  # ---------------------------------------------------------------------
  # #544: a group is a Todolist.
  #
  # BC3 has no group model — todolists/groups/{index,show}.json.jbuilder render
  # todolists/_todolist.json.jbuilder — so a group carries description and
  # description_attachments like any list, and reports "type": "Todolist".
  # The spec used to model a group-only projection with NO description member,
  # and the shared fixtures matched it; both of the reads below would have had
  # nothing to assert. Discrimination is structural — group_position_url XOR
  # groups_url — and never the type string, which is "Todolist" either way.
  #
  # These use the shared spec/fixtures bodies deliberately: they are validated
  # by `make check-fixture-coverage`, so the contract these tests pin cannot
  # drift away from the one the spec ships.
  # ---------------------------------------------------------------------

  # A group is fetched on the TODOLIST route: the groups service has no get(),
  # because there is nothing group-shaped to get.
  def test_group_get_returns_the_flat_todolist_shape_with_its_description
    response = load_fixture("todolist_groups/get.json")

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.get(id: response["id"])

    assert_equal "Phase 1", result["name"]
    assert_equal "<div>Phase one hardware work</div>", result["description"]
    assert_equal [], result["description_attachments"]
    assert_equal "Todolist", result["type"]
    assert_equal "Todolist", result.dig("parent", "type")
    assert result.key?("group_position_url"), "a group's parent is a Todolist, so it carries group_position_url"
    assert_not result.key?("groups_url"), "only a todoset-parented list carries groups_url"
  end

  def test_list_returns_the_flat_todolist_shape_with_descriptions
    response = load_fixture("todolist_groups/list.json")

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+/groups\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolist_groups.list(todolist_id: 2).to_a

    assert_equal 2, result.length
    first = result.first
    assert_equal "Phase 1", first["name"]
    assert_equal "<div>Phase one hardware work</div>", first["description"]
    assert first.key?("group_position_url"), "a group's parent is a Todolist, so it carries group_position_url"
    assert_not first.key?("groups_url"), "only a todoset-parented list carries groups_url"
    # Present-and-empty is the shape for a group with no description; absent is
    # not. description is @required on Todolist, so BC3 renders it either way.
    assert result.last.key?("description"), "description is rendered even when blank"
    assert_equal "", result.last["description"]
  end

  # Note: get(), update() not available in generated service (spec-conformant)
  # Note: trash() is on RecordingsService - use recordings.trash(project_id:, recording_id:)
  # Note: No client-side validation - API validates
end
