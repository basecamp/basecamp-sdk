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
    response = [ { "id" => 1, "name" => "Sprint Tasks", "completed_ratio" => "3/10", "description_attachments" => [] } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.list(todoset_id: 2).to_a
    assert_kind_of Array, result
    assert_equal "Sprint Tasks", result.first["name"]
  end

  def test_list_with_status
    response = [ { "id" => 1, "name" => "Archived List", "status" => "archived", "description_attachments" => [] } ]

    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json\?status=archived})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.list(todoset_id: 2, status: "archived").to_a
    assert_equal "archived", result.first["status"]
  end

  # Uses the canonical fixture rather than a hand-rolled hash: Todolist has a
  # dozen required members (bubble_up_url among them) and a minimal stub is a
  # payload BC3 cannot produce. spec/fixtures is validated by
  # `make check-fixture-coverage`, so this stub cannot drift from the contract.
  def test_get
    response = load_fixture("todolists/get.json")

    # Generated service uses /todolists/{id} without .json
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.get(id: 2)
    assert_equal response["name"], result["name"]
    # bubble_up_url is @required on Todolist: todolists/_todolist.json.jbuilder
    # renders the shared recording partial with bubbleupable: true
    # unconditionally, so every projection of this shape carries it.
    assert result.key?("bubble_up_url"), "required bubble_up_url must be present"
  end

  def test_create
    response = { "id" => 1, "name" => "New List", "description_attachments" => [] }

    stub_request(:post, %r{https://3\.basecampapi\.com/12345/todosets/\d+/todolists\.json})
      .to_return(status: 201, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.create(todoset_id: 2, name: "New List")
    assert_equal "New List", result["name"]
  end

  def test_update
    response = { "id" => 2, "name" => "Updated List", "description_attachments" => [] }

    # Generated service uses /todolists/{id} without .json
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    result = @account.todolists.update(id: 2, name: "Updated List")
    assert_equal "Updated List", result["name"]
  end

  def test_reposition
    captured = {}
    stub_request(:put, "https://3.basecampapi.com/12345/todosets/todolists/2/position.json")
      .with { |req| captured[:body] = JSON.parse(req.body) }
      .to_return(status: 204)

    assert_nil @account.todolists.reposition(todolist_id: 2, position: 3)
    assert_equal({ "position" => 3 }, captured[:body])
  end

  def test_reposition_not_found
    stub_request(:put, "https://3.basecampapi.com/12345/todosets/todolists/999/position.json")
      .to_return(status: 404, body: "")

    assert_raises(Basecamp::NotFoundError) do
      @account.todolists.reposition(todolist_id: 999, position: 1)
    end
  end

  # The get/update path label is the unsuffixed `{id}`. resource_id must still
  # carry it (predicate is `end_with?("Id") || == "id"`); a suffix-only
  # regression would silently drop resource_id here.
  def test_get_operation_metadata
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))

    response = { "id" => 2, "name" => "Sprint Tasks", "description_attachments" => [] }
    stub_request(:get, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    account.todolists.get(id: 2)

    event = events.find { |e| e[:event] == :on_operation_start }
    assert event, "Expected on_operation_start to fire"
    info = event[:info]
    assert_equal "todolists", info.service
    assert_equal "get", info.operation
    assert_equal 2, info.resource_id
  end

  def test_update_operation_metadata
    events = []
    account = create_account_client(account_id: "12345", hooks: CapturingHooks.new(events))

    response = { "id" => 2, "name" => "Updated List", "description_attachments" => [] }
    stub_request(:put, %r{https://3\.basecampapi\.com/12345/todolists/\d+$})
      .to_return(status: 200, body: response.to_json, headers: { "Content-Type" => "application/json" })

    account.todolists.update(id: 2, name: "Updated List")

    event = events.find { |e| e[:event] == :on_operation_start }
    assert event, "Expected on_operation_start to fire"
    info = event[:info]
    assert_equal "todolists", info.service
    assert_equal "update", info.operation
    assert_equal 2, info.resource_id
  end

  private

  # Captures the OperationInfo passed to on_operation_start so tests can
  # assert the metadata emitted by the real generated service call.
  class CapturingHooks
    include Basecamp::Hooks

    def initialize(events)
      @events = events
    end

    def on_operation_start(info)
      @events << { event: :on_operation_start, info: info }
    end
  end
end
